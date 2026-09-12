package components

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

type csvExportDriver struct {
	schemaProgrammingMock

	mu            sync.Mutex
	pages         map[int]drivers.PageResult
	pageCalls     []csvExportPageCall
	exactCount    int
	streamCalls   int
	streamMaxRows int
	streamQuery   string
	streamBatches []drivers.QueryBatch
	streamErr     error
	blockStream   bool
	streamStarted chan struct{}
	cancelSeen    bool
}

type csvExportPageCall struct {
	where  string
	sort   string
	offset int
	limit  int
}

func (driver *csvExportDriver) GetRecords(ctx context.Context, _, _ string, where, sort string, offset, limit int) (drivers.PageResult, error) {
	if err := ctx.Err(); err != nil {
		return drivers.PageResult{}, err
	}
	driver.mu.Lock()
	driver.pageCalls = append(driver.pageCalls, csvExportPageCall{where: where, sort: sort, offset: offset, limit: limit})
	page := driver.pages[offset]
	driver.mu.Unlock()
	return page, nil
}

func (driver *csvExportDriver) GetExactRowCount(context.Context, string, string, string) (int64, error) {
	driver.mu.Lock()
	driver.exactCount++
	driver.mu.Unlock()
	return 0, errors.New("exact count must not be requested")
}

func (driver *csvExportDriver) StreamQuery(ctx context.Context, query string, maxRows int, onBatch func(drivers.QueryBatch) error) (drivers.QueryStreamResult, error) {
	driver.mu.Lock()
	driver.streamCalls++
	driver.streamMaxRows = maxRows
	driver.streamQuery = query
	batches := append([]drivers.QueryBatch(nil), driver.streamBatches...)
	streamErr := driver.streamErr
	driver.mu.Unlock()

	result := drivers.QueryStreamResult{Columns: []string{"id"}}
	if driver.blockStream {
		if driver.streamStarted != nil {
			close(driver.streamStarted)
		}
		<-ctx.Done()
		driver.mu.Lock()
		driver.cancelSeen = true
		driver.mu.Unlock()
		return result, ctx.Err()
	}
	for _, batch := range batches {
		result.Rows += len(batch.Rows)
		result.Columns = batch.Columns
		if err := onBatch(batch); err != nil {
			driver.mu.Lock()
			if ctx.Err() != nil {
				driver.cancelSeen = true
			}
			driver.mu.Unlock()
			return result, err
		}
	}
	if streamErr != nil {
		return result, streamErr
	}
	if err := ctx.Err(); err != nil {
		driver.mu.Lock()
		driver.cancelSeen = true
		driver.mu.Unlock()
		return result, err
	}
	return result, nil
}

func (driver *csvExportDriver) pageCallsSnapshot() []csvExportPageCall {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	return append([]csvExportPageCall(nil), driver.pageCalls...)
}

func (driver *csvExportDriver) streamSnapshot() (calls, maxRows int, query string, cancelSeen bool) {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	return driver.streamCalls, driver.streamMaxRows, driver.streamQuery, driver.cancelSeen
}

func newCSVExportTestTable(driver drivers.Driver) *ResultsTable {
	changes := []models.DBDMLChange{}
	return &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			records:         [][]string{{"id", "name"}, {"visible", "row"}},
			listOfDBChanges: &changes,
			markedRows:      map[int]bool{},
			fkRawCellValues: map[string]string{},
		},
		Pagination: NewPagination(),
		DBDriver:   driver,
	}
}

func readCSVExportFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", path, err)
	}
	return string(content)
}

func assertNoCSVExportTempFiles(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%q) failed: %v", dir, err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".lazysql_export_") {
			t.Fatalf("temporary export remained: %s", entry.Name())
		}
	}
}

func TestReplaySafeQueryClassification(t *testing.T) {
	tests := []struct {
		query string
		safe  bool
	}{
		{query: "SELECT * FROM users", safe: true},
		{query: "SELECT COUNT(*) FROM users", safe: true},
		{query: "SELECT dangerous_user_function(id) FROM users", safe: false},
		{query: "-- DELETE is only a comment\nSELECT 'UPDATE' FROM users;", safe: true},
		{query: "WITH recent AS (SELECT * FROM users) SELECT * FROM recent", safe: true},
		{query: "SHOW TABLES", safe: true},
		{query: "DESCRIBE users", safe: true},
		{query: "SELECT * FROM users; DELETE FROM users", safe: false},
		{query: "WITH recent AS (SELECT 1) UPDATE users SET id = 1", safe: false},
		{query: "INSERT INTO users VALUES (1)", safe: false},
		{query: "CALL refresh_users()", safe: false},
		{query: "PRAGMA user_version = 1", safe: false},
		{query: "EXPLAIN ANALYZE SELECT * FROM users", safe: false},
		{query: "VACUUM", safe: false},
		{query: "DELETE FROM users -- SELECT", safe: false},
		{query: "WITH only_cte AS (SELECT 1)", safe: false},
	}

	for _, test := range tests {
		t.Run(strings.ReplaceAll(strings.ToLower(test.query), " ", "_"), func(t *testing.T) {
			if got := isReplaySafeQuery(test.query); got != test.safe {
				t.Fatalf("isReplaySafeQuery(%q) = %v, want %v", test.query, got, test.safe)
			}
		})
	}
}

func TestCSVExportModalScopes(t *testing.T) {
	tableModal := NewCSVExportModal(CSVExportOptions{HasPagination: true}, nil)
	if got := tableModal.form.GetButtonCount(); got != 2 {
		t.Fatalf("table button count = %d, want 2", got)
	}
	if got := tableModal.form.GetButton(0).GetLabel(); got != "Export Current Page" {
		t.Fatalf("first table button = %q", got)
	}
	if got := tableModal.form.GetButton(1).GetLabel(); got != "Export All Records" {
		t.Fatalf("second table button = %q", got)
	}

	queryModal := NewCSVExportModal(CSVExportOptions{IsQueryResult: true, CanExportAll: true}, nil)
	if got := queryModal.form.GetButtonCount(); got != 2 {
		t.Fatalf("safe query button count = %d, want 2", got)
	}
	if got := queryModal.form.GetButton(0).GetLabel(); got != "Export Visible Results" {
		t.Fatalf("first query button = %q", got)
	}
	if got := queryModal.form.GetButton(1).GetLabel(); got != "Export All Results" {
		t.Fatalf("second query button = %q", got)
	}

	unsafeModal := NewCSVExportModal(CSVExportOptions{IsQueryResult: true}, nil)
	if got := unsafeModal.form.GetButtonCount(); got != 1 {
		t.Fatalf("unsafe query button count = %d, want 1", got)
	}
	if got := unsafeModal.form.GetButton(0).GetLabel(); got != "Export Visible Results" {
		t.Fatalf("unsafe query button = %q", got)
	}
}

func TestExportAllRecordsInBatchesUsesFilterSortAndUnknownTotal(t *testing.T) {
	driver := &csvExportDriver{pages: map[int]drivers.PageResult{
		0: {Rows: [][]string{{"id", "name"}, {"1", "Alice"}, {"2", "Bob"}}, HasNextPage: true},
		2: {Rows: [][]string{{"id", "name"}, {"3", "Carol"}}},
	}}
	table := newCSVExportTestTable(driver)
	dir := t.TempDir()
	path := filepath.Join(dir, "all.csv")
	var progress []int

	got, err := table.exportAllRecordsInBatchesWithProgress(
		context.Background(), path, "db", "users", "WHERE active = 1", "name DESC", 2,
		func(rows int) { progress = append(progress, rows) },
	)
	if err != nil {
		t.Fatalf("exportAllRecordsInBatchesWithProgress() error = %v", err)
	}
	if got != 3 {
		t.Fatalf("exported rows = %d, want 3", got)
	}
	if want := []int{2, 3}; !reflect.DeepEqual(progress, want) {
		t.Fatalf("progress = %v, want %v", progress, want)
	}
	if want := []csvExportPageCall{
		{where: "WHERE active = 1", sort: "name DESC", offset: 0, limit: 2},
		{where: "WHERE active = 1", sort: "name DESC", offset: 2, limit: 2},
	}; !reflect.DeepEqual(driver.pageCallsSnapshot(), want) {
		t.Fatalf("page calls = %+v, want %+v", driver.pageCallsSnapshot(), want)
	}
	driver.mu.Lock()
	exactCount := driver.exactCount
	driver.mu.Unlock()
	if exactCount != 0 {
		t.Fatalf("exact count calls = %d, want 0", exactCount)
	}
	if got, want := readCSVExportFile(t, path), "id,name\n1,Alice\n2,Bob\n3,Carol\n"; got != want {
		t.Fatalf("CSV = %q, want %q", got, want)
	}
}

func TestExportVisibleQueryResultsUsesShownRows(t *testing.T) {
	driver := &csvExportDriver{streamBatches: []drivers.QueryBatch{{
		Columns: []string{"id"},
		Rows:    [][]string{{"database-row"}},
	}}}
	table := newCSVExportTestTable(driver)
	table.Editor = NewSQLEditor("")
	dir := t.TempDir()
	path := filepath.Join(dir, "visible.csv")

	got, err := table.exportCurrentPage(path)
	if err != nil {
		t.Fatalf("exportCurrentPage() error = %v", err)
	}
	if got != 1 {
		t.Fatalf("exported rows = %d, want 1", got)
	}
	if streamCalls, _, _, _ := driver.streamSnapshot(); streamCalls != 0 {
		t.Fatalf("visible export reexecuted the query %d times", streamCalls)
	}
	if got, want := readCSVExportFile(t, path), "id,name\nvisible,row\n"; got != want {
		t.Fatalf("CSV = %q, want %q", got, want)
	}
}

func TestExportAllQueryResultsStreamsWithoutInteractiveCap(t *testing.T) {
	query := "WITH selected AS (SELECT id FROM users) SELECT id FROM selected"
	driver := &csvExportDriver{streamBatches: []drivers.QueryBatch{
		{Columns: []string{"id", "name"}, Rows: [][]string{{"1", "Alice"}, {"2", "Bob"}}},
		{Columns: []string{"id", "name"}, Rows: [][]string{{"3", "Carol"}}},
	}}
	table := newCSVExportTestTable(driver)
	dir := t.TempDir()
	path := filepath.Join(dir, "query.csv")
	var progress []int

	got, err := table.exportAllQueryResults(context.Background(), path, query, func(rows int) {
		progress = append(progress, rows)
	})
	if err != nil {
		t.Fatalf("exportAllQueryResults() error = %v", err)
	}
	if got != 3 {
		t.Fatalf("exported rows = %d, want 3", got)
	}
	calls, maxRows, gotQuery, _ := driver.streamSnapshot()
	if calls != 1 || maxRows != 0 || gotQuery != query {
		t.Fatalf("stream call = (%d, %d, %q), want (1, 0, %q)", calls, maxRows, gotQuery, query)
	}
	if want := []int{2, 3}; !reflect.DeepEqual(progress, want) {
		t.Fatalf("progress = %v, want %v", progress, want)
	}
	if got, want := readCSVExportFile(t, path), "id,name\n1,Alice\n2,Bob\n3,Carol\n"; got != want {
		t.Fatalf("CSV = %q, want %q", got, want)
	}
}

func TestExportAllQueryResultsRefusesUnsafeAndNonStreamingQueries(t *testing.T) {
	dir := t.TempDir()

	unsafeDriver := &csvExportDriver{streamBatches: []drivers.QueryBatch{{Columns: []string{"id"}, Rows: [][]string{{"1"}}}}}
	unsafeTable := newCSVExportTestTable(unsafeDriver)
	unsafePath := filepath.Join(dir, "unsafe.csv")
	if _, err := unsafeTable.exportAllQueryResults(context.Background(), "INSERT INTO users VALUES (1)", unsafePath, nil); err == nil {
		t.Fatal("unsafe query export succeeded")
	}
	if calls, _, _, _ := unsafeDriver.streamSnapshot(); calls != 0 {
		t.Fatalf("unsafe query invoked stream %d times", calls)
	}
	if _, err := os.Stat(unsafePath); !os.IsNotExist(err) {
		t.Fatalf("unsafe query created final file: %v", err)
	}

	nonStreamingTable := newCSVExportTestTable(&schemaProgrammingMock{})
	nonStreamingPath := filepath.Join(dir, "non-streaming.csv")
	if _, err := nonStreamingTable.exportAllQueryResults(context.Background(), "SELECT 1", nonStreamingPath, nil); err == nil {
		t.Fatal("non-streaming query export succeeded")
	}
	if _, err := os.Stat(nonStreamingPath); !os.IsNotExist(err) {
		t.Fatalf("non-streaming query created final file: %v", err)
	}
}

func TestExportAllQueryResultsCancellationAbortsAtomicFile(t *testing.T) {
	driver := &csvExportDriver{streamBatches: []drivers.QueryBatch{
		{Columns: []string{"id"}, Rows: [][]string{{"1"}, {"2"}}},
		{Columns: []string{"id"}, Rows: [][]string{{"3"}}},
	}}
	table := newCSVExportTestTable(driver)
	dir := t.TempDir()
	path := filepath.Join(dir, "cancelled.csv")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := table.exportAllQueryResults(ctx, path, "SELECT id FROM users", func(rows int) {
		if rows == 2 {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("export error = %v, want context.Canceled", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("cancelled export left final file: %v", statErr)
	}
	_, _, _, cancelSeen := driver.streamSnapshot()
	if !cancelSeen {
		t.Fatal("database stream did not observe cancellation")
	}
	assertNoCSVExportTempFiles(t, dir)
}

func TestCancelExportCancelsTheActiveStreamingOperation(t *testing.T) {
	driver := &csvExportDriver{blockStream: true, streamStarted: make(chan struct{})}
	table := newCSVExportTestTable(driver)
	run := table.beginCSVExport()

	done := make(chan error, 1)
	path := filepath.Join(t.TempDir(), "cancelled.csv")
	go func() {
		_, err := table.exportAllQueryResults(run.ctx, path, "SELECT id FROM users", nil)
		done <- err
	}()
	select {
	case <-driver.streamStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("streaming export did not start")
	}

	if !table.CancelExport() {
		t.Fatal("CancelExport() did not report an active export")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("export error = %v, want context.Canceled", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("streaming export did not stop after cancellation")
	}
	_, _, _, cancelSeen := driver.streamSnapshot()
	if !cancelSeen {
		t.Fatal("active database operation did not observe cancellation")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cancelled export left final file: %v", err)
	}
	assertNoCSVExportTempFiles(t, filepath.Dir(path))
}

func TestExportAllQueryResultsFailurePreservesExistingFinalFile(t *testing.T) {
	driver := &csvExportDriver{
		streamBatches: []drivers.QueryBatch{{Columns: []string{"id"}, Rows: [][]string{{"1"}}}},
		streamErr:     errors.New("connection lost"),
	}
	table := newCSVExportTestTable(driver)
	dir := t.TempDir()
	path := filepath.Join(dir, "failed.csv")
	if err := os.WriteFile(path, []byte("old file\n"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	_, err := table.exportAllQueryResults(context.Background(), path, "SELECT id FROM users", nil)
	if err == nil || err.Error() != "connection lost" {
		t.Fatalf("export error = %v, want connection lost", err)
	}
	if got := readCSVExportFile(t, path); got != "old file\n" {
		t.Fatalf("existing final file = %q, want old file", got)
	}
	assertNoCSVExportTempFiles(t, dir)
}

func TestCanExportAllQueryResultsRequiresShownSafeResultAndStreamingDriver(t *testing.T) {
	driver := &csvExportDriver{}
	table := newCSVExportTestTable(driver)
	table.Editor = NewSQLEditor("")
	table.state.records = nil
	table.state.lastEditorQuery = "SELECT 1"
	table.state.lastEditorQueryReplaySafe = true

	if table.canExportAllQueryResults() {
		t.Fatal("query without shown result was offered Export All")
	}
	table.state.editorResultAvailable = true
	if !table.canExportAllQueryResults() {
		t.Fatal("safe shown query was not offered Export All")
	}

	nonStreaming := newCSVExportTestTable(&schemaProgrammingMock{})
	nonStreaming.Editor = NewSQLEditor("")
	nonStreaming.state.lastEditorQuery = "SELECT 1"
	nonStreaming.state.lastEditorQueryReplaySafe = true
	nonStreaming.state.editorResultAvailable = true
	if nonStreaming.canExportAllQueryResults() {
		t.Fatal("non-streaming driver was offered Export All")
	}
}
