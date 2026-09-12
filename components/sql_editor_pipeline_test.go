package components

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/internal/history"
	"github.com/jorgerojas26/lazysql/models"
)

type blockingEditorStreamDriver struct {
	schemaProgrammingMock
	started       chan struct{}
	batchReturned chan struct{}
	released      chan struct{}
	cancelled     chan struct{}
	done          chan struct{}
	startOnce     sync.Once
	cancelOnce    sync.Once
}

func newBlockingEditorStreamDriver() *blockingEditorStreamDriver {
	return &blockingEditorStreamDriver{
		started:       make(chan struct{}),
		batchReturned: make(chan struct{}),
		released:      make(chan struct{}),
		cancelled:     make(chan struct{}),
		done:          make(chan struct{}),
	}
}

func (driver *blockingEditorStreamDriver) StreamQuery(ctx context.Context, _ string, _ int, onBatch func(drivers.QueryBatch) error) (drivers.QueryStreamResult, error) {
	defer close(driver.done)
	driver.startOnce.Do(func() { close(driver.started) })
	result := drivers.QueryStreamResult{Columns: []string{"id"}, Rows: 1}
	if err := onBatch(drivers.QueryBatch{Columns: result.Columns, Rows: [][]string{{"1"}}}); err != nil {
		return result, err
	}
	close(driver.batchReturned)

	select {
	case <-driver.released:
		return result, nil
	case <-ctx.Done():
		driver.cancelOnce.Do(func() { close(driver.cancelled) })
		return result, ctx.Err()
	}
}

type errorEditorStreamDriver struct {
	schemaProgrammingMock
	called chan struct{}
}

type truncatedEditorStreamDriver struct {
	schemaProgrammingMock
	called  chan struct{}
	maxRows chan int
}

func (driver *errorEditorStreamDriver) StreamQuery(_ context.Context, _ string, _ int, onBatch func(drivers.QueryBatch) error) (drivers.QueryStreamResult, error) {
	close(driver.called)
	result := drivers.QueryStreamResult{Columns: []string{"id"}, Rows: 1}
	if err := onBatch(drivers.QueryBatch{Columns: result.Columns, Rows: [][]string{{"1"}}}); err != nil {
		return result, err
	}
	return result, errors.New("connection lost")
}

func (driver *truncatedEditorStreamDriver) StreamQuery(_ context.Context, _ string, maxRows int, onBatch func(drivers.QueryBatch) error) (drivers.QueryStreamResult, error) {
	close(driver.called)
	driver.maxRows <- maxRows
	result := drivers.QueryStreamResult{Columns: []string{"id"}, Rows: 2, Truncated: true}
	if err := onBatch(drivers.QueryBatch{Columns: result.Columns, Rows: [][]string{{"1"}, {"2"}}}); err != nil {
		return result, err
	}
	return result, nil
}

func newEditorPipelineTable(driver drivers.Driver) (*ResultsTable, *SQLEditor, *tview.Pages) {
	changes := []models.DBDMLChange{}
	errorModal := tview.NewModal()
	pages := tview.NewPages()
	pages.AddPage(pageNameTable, tview.NewFlex(), true, true)
	pages.AddPage(pageNameTableError, errorModal, true, false)

	editor := NewSQLEditor("")
	table := &ResultsTable{
		Table:                tview.NewTable(),
		state:                &ResultsTableState{records: [][]string{}, listOfDBChanges: &changes},
		Page:                 pages,
		Wrapper:              tview.NewFlex(),
		Error:                errorModal,
		Pagination:           NewPagination(),
		Editor:               editor,
		EditorPages:          tview.NewPages(),
		DBDriver:             driver,
		connectionIdentifier: "stream-test",
	}
	table.EditorPages.AddPage(pageNameTableEditorTable, tview.NewFlex(), true, true)
	editor.SetQueryCancelFunc(table.CancelActiveQuery)
	return table, editor, pages
}

func startEditorPipeline(t *testing.T, table *ResultsTable, editor *SQLEditor, pages *tview.Pages) chan struct{} {
	t.Helper()
	// tview applications cannot reliably be restarted after Stop; each
	// pipeline test owns a fresh UI loop while sharing the application config.
	App.Application = tview.NewApplication()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init simulation screen: %v", err)
	}
	App.SetScreen(screen)

	go table.subscribeToEditorChanges()
	appDone := make(chan struct{})
	go func() {
		defer close(appDone)
		_ = App.Run(pages, "")
	}()
	App.QueueUpdate(func() {})
	// Let the subscription goroutine register before publishing the test query.
	time.Sleep(50 * time.Millisecond)
	return appDone
}

func stopEditorPipeline(t *testing.T, appDone chan struct{}) {
	t.Helper()
	App.Application.Stop()
	select {
	case <-appDone:
	case <-time.After(3 * time.Second):
		t.Fatal("application did not stop")
	}
}

func TestEditorStreamRendersFirstBatchBeforeStreamCompletes(t *testing.T) {
	driver := newBlockingEditorStreamDriver()
	table, editor, pages := newEditorPipelineTable(driver)
	appDone := startEditorPipeline(t, table, editor, pages)
	defer func() {
		close(driver.released)
		stopEditorPipeline(t, appDone)
	}()

	editor.Publish(eventSQLEditorQuery, "SELECT id")
	select {
	case <-driver.started:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not start")
	}
	select {
	case <-driver.batchReturned:
	case <-time.After(3 * time.Second):
		t.Fatal("first batch was not rendered")
	}

	select {
	case <-driver.done:
		t.Fatal("stream completed before the first batch was observed")
	default:
	}
	rows := table.GetRecords()
	if len(rows) != 2 || rows[1][0] != "1" {
		t.Fatalf("rendered records = %v, want header and first row", rows)
	}
}

func TestEditorStreamCancellationKeepsPartialRows(t *testing.T) {
	driver := newBlockingEditorStreamDriver()
	table, editor, pages := newEditorPipelineTable(driver)
	appDone := startEditorPipeline(t, table, editor, pages)
	defer stopEditorPipeline(t, appDone)

	editor.Publish(eventSQLEditorQuery, "SELECT id")
	select {
	case <-driver.batchReturned:
	case <-time.After(3 * time.Second):
		t.Fatal("first batch was not rendered")
	}
	if !table.CancelActiveQuery() {
		t.Fatal("CancelActiveQuery() did not report an active stream")
	}
	select {
	case <-driver.cancelled:
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not reach the stream driver")
	}

	rows := table.GetRecords()
	if len(rows) != 2 || rows[1][0] != "1" {
		t.Fatalf("cancelled stream discarded rows = %v", rows)
	}
	if !containsEditorText(table.GetQueryStatus(), "partial result: query cancelled") {
		t.Fatalf("query status = %q, want partial cancellation label", table.GetQueryStatus())
	}
}

func TestEditorStreamShowsConfiguredTruncation(t *testing.T) {
	config := App.Config()
	oldMaxRows := config.MaxQueryRows
	config.MaxQueryRows = 2
	t.Cleanup(func() { config.MaxQueryRows = oldMaxRows })

	driver := &truncatedEditorStreamDriver{called: make(chan struct{}), maxRows: make(chan int, 1)}
	table, editor, pages := newEditorPipelineTable(driver)
	appDone := startEditorPipeline(t, table, editor, pages)
	defer stopEditorPipeline(t, appDone)

	editor.Publish(eventSQLEditorQuery, "SELECT id")
	select {
	case <-driver.called:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not start")
	}
	select {
	case maxRows := <-driver.maxRows:
		if maxRows != 2 {
			t.Fatalf("stream max rows = %d, want configured value 2", maxRows)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not receive max rows")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !containsEditorText(table.GetQueryStatus(), "result truncated (maximum 2)") {
		time.Sleep(time.Millisecond)
	}
	if !containsEditorText(table.GetQueryStatus(), "result truncated (maximum 2)") {
		t.Fatalf("query status = %q, want truncation label", table.GetQueryStatus())
	}
	rows := table.GetRecords()
	if len(rows) != 3 || rows[2][0] != "2" {
		t.Fatalf("truncated records = %v, want two displayed rows", rows)
	}
}

func TestEditorStreamErrorKeepsPartialRows(t *testing.T) {
	driver := &errorEditorStreamDriver{called: make(chan struct{})}
	table, editor, pages := newEditorPipelineTable(driver)
	appDone := startEditorPipeline(t, table, editor, pages)
	defer stopEditorPipeline(t, appDone)

	editor.Publish(eventSQLEditorQuery, "SELECT id")
	select {
	case <-driver.called:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not start")
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !containsEditorText(table.GetQueryStatus(), "partial result: connection lost") {
		time.Sleep(time.Millisecond)
	}
	if !containsEditorText(table.GetQueryStatus(), "partial result: connection lost") {
		t.Fatalf("query status = %q, want partial error label", table.GetQueryStatus())
	}
}

func TestReadOnlyRejectedQueryIsNotRecordedInHistory(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	query := "WITH cte AS (SELECT 1) INSERT INTO users SELECT * FROM cte"
	if executed := runEditorQuery(t, true, query); len(executed) != 0 {
		t.Fatalf("read-only query reached driver: %q", executed)
	}

	path, err := history.GetHistoryFilePath("")
	if err != nil {
		t.Fatalf("history path: %v", err)
	}
	items, err := history.ReadHistory(path, 0)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("history = %v, want no pre-dispatch validation failure", items)
	}
}

func TestEditorStreamHistoryRecordsPostDispatchError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	driver := &errorEditorStreamDriver{called: make(chan struct{})}
	table, editor, pages := newEditorPipelineTable(driver)
	appDone := startEditorPipeline(t, table, editor, pages)
	defer stopEditorPipeline(t, appDone)

	query := "SELECT id"
	editor.Publish(eventSQLEditorQuery, query)
	select {
	case <-driver.called:
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not start")
	}

	path, err := history.GetHistoryFilePath("stream-test")
	if err != nil {
		t.Fatalf("history path: %v", err)
	}
	items, err := history.ReadHistory(path, 0)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	if len(items) != 1 || items[0].QueryText != query {
		t.Fatalf("history = %v, want dispatched query %q", items, query)
	}
}

func containsEditorText(value, want string) bool {
	return len(value) >= len(want) && strings.Contains(value, want)
}
