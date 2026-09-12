package components

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

func newMarkTestTable(rows [][]string) *ResultsTable {
	changes := []models.DBDMLChange{}

	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			listOfDBChanges: &changes,
			markedRows:      map[int]bool{},
			fkRawCellValues: map[string]string{},
		},
	}

	for rowIndex, row := range rows {
		for columnIndex, cell := range row {
			table.SetCell(rowIndex, columnIndex, tview.NewTableCell(cell))
		}
	}

	return table
}

func TestToggleRowMarkNeverMarksHeader(t *testing.T) {
	table := newMarkTestTable([][]string{
		{"id", "name"},
		{"1", "alice"},
	})

	table.toggleRowMark(0)

	if len(table.state.markedRows) != 0 {
		t.Fatalf("expected header row to be unmarkable, got %d marked rows", len(table.state.markedRows))
	}
}

func TestToggleRowMarkAddsAndRemoves(t *testing.T) {
	table := newMarkTestTable([][]string{
		{"id", "name"},
		{"1", "alice"},
		{"2", "bob"},
	})

	table.toggleRowMark(1)
	table.toggleRowMark(2)

	if got := table.GetMarkedRowIndexes(); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("expected marked rows [1 2], got %v", got)
	}

	table.toggleRowMark(1)

	if got := table.GetMarkedRowIndexes(); len(got) != 1 || got[0] != 2 {
		t.Fatalf("expected marked rows [2] after unmark, got %v", got)
	}
}

func TestClearRowMarks(t *testing.T) {
	table := newMarkTestTable([][]string{
		{"id"},
		{"1"},
		{"2"},
	})

	table.toggleRowMark(1)
	table.toggleRowMark(2)
	table.clearRowMarks()

	if len(table.state.markedRows) != 0 {
		t.Fatalf("expected no marked rows after clear, got %d", len(table.state.markedRows))
	}
}

func TestMarkedRowsToTextIsOrderedTSV(t *testing.T) {
	table := newMarkTestTable([][]string{
		{"id", "name"},
		{"1", "alice"},
		{"2", "bob"},
		{"3", "carol"},
	})

	// Mark out of order to prove the output is sorted top to bottom.
	table.toggleRowMark(3)
	table.toggleRowMark(1)

	want := "1\talice\n3\tcarol"
	if got := table.markedRowsToText(); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestMarkedRowsToTextSkipsStaleIndexes(t *testing.T) {
	table := newMarkTestTable([][]string{
		{"id"},
		{"1"},
	})

	// Simulate a mark left over from a larger result set.
	table.state.markedRows[5] = true
	table.toggleRowMark(1)

	if got := table.markedRowsToText(); got != "1" {
		t.Fatalf("expected only the in-range row, got %q", got)
	}
}

// TestSetLoadingIsSynchronous verifies that SetLoading does not use
// QueueUpdateDraw or any other blocking mechanism.
func TestSetLoadingIsSynchronous(t *testing.T) {
	changes := []models.DBDMLChange{}
	errorModal := tview.NewModal()

	pages := tview.NewPages()
	pages.AddPage(pageNameTable, tview.NewFlex(), true, true)
	pages.AddPage(pageNameTableError, errorModal, false, false)

	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			records:         [][]string{},
			isLoading:       false,
			listOfDBChanges: &changes,
		},
		Page:       pages,
		Error:      errorModal,
		Pagination: NewPagination(),
	}

	// Verify initial state
	if table.GetIsLoading() {
		t.Error("Expected isLoading to be false initially")
	}

	// SetLoading(true) must return immediately (not block)
	done := make(chan struct{}, 1)
	go func() {
		table.SetLoading(true)
		done <- struct{}{}
	}()

	select {
	case <-done:
		// Success — SetLoading returned synchronously
	case <-time.After(3 * time.Second):
		t.Fatal("SetLoading(true) did not return — likely deadlock from QueueUpdateDraw")
	}

	if !table.GetIsLoading() {
		t.Error("Expected isLoading to be true after SetLoading(true)")
	}

	// SetLoading(false) must also return immediately
	go func() {
		table.SetLoading(false)
		done <- struct{}{}
	}()

	select {
	case <-done:
		// Success — SetLoading returned synchronously
	case <-time.After(3 * time.Second):
		t.Fatal("SetLoading(false) did not return — likely deadlock from QueueUpdateDraw")
	}

	if table.GetIsLoading() {
		t.Error("Expected isLoading to be false after SetLoading(false)")
	}
}

func TestStartingRecordsLoadCancelsPreviousContext(t *testing.T) {
	table := &ResultsTable{
		Table:      tview.NewTable(),
		state:      &ResultsTableState{},
		Pagination: NewPagination(),
	}

	firstContext, firstGeneration := table.startLoad()
	secondContext, secondGeneration := table.startLoad()
	defer table.CancelLoading()

	select {
	case <-firstContext.Done():
	default:
		t.Fatal("starting a newer load did not cancel the previous context")
	}

	if table.isCurrentLoad(firstContext, firstGeneration) {
		t.Fatal("cancelled load is still considered current")
	}
	if !table.isCurrentLoad(secondContext, secondGeneration) {
		t.Fatal("newer load is not considered current")
	}
}

func TestPaginationUsesLookaheadForNavigation(t *testing.T) {
	pagination := NewPagination()
	pagination.SetLimit(2)
	pagination.SetPageInfo(2, true)

	if !pagination.GetIsFirstPage() {
		t.Fatal("expected offset zero to be the first page")
	}
	if pagination.GetIsLastPage() {
		t.Fatal("expected lookahead row to enable the next page")
	}
	if !pagination.GetHasNextPage() {
		t.Fatal("expected HasNextPage to be exposed")
	}

	pagination.SetOffset(2)
	pagination.SetPageInfo(1, false)

	if pagination.GetIsFirstPage() {
		t.Fatal("expected non-zero offset not to be the first page")
	}
	if !pagination.GetIsLastPage() {
		t.Fatal("expected missing lookahead row to mark the last page")
	}
	if pagination.GetTotalRecords() != 3 {
		t.Fatalf("expected inferred total 3, got %d", pagination.GetTotalRecords())
	}
}

func TestTrimRecordsToPage(t *testing.T) {
	rows := [][]string{{"id"}, {"1"}, {"2"}, {"3"}}
	got := trimRecordsToPage(rows, 2)
	want := [][]string{{"id"}, {"1"}, {"2"}}

	if len(got) != len(want) {
		t.Fatalf("expected %d rows, got %d", len(want), len(got))
	}
	for i := range want {
		if len(got[i]) != len(want[i]) || got[i][0] != want[i][0] {
			t.Fatalf("expected rows %v, got %v", want, got)
		}
	}
}

func TestRebuildForeignKeyJumpMetadataPostgresSkipsComposite(t *testing.T) {
	changes := []models.DBDMLChange{}

	db := &drivers.Postgres{}
	db.SetProvider(drivers.DriverPostgres)

	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			listOfDBChanges:       &changes,
			foreignKeyColumns:     map[string]bool{},
			foreignKeyJumpTargets: map[string]foreignKeyJumpTarget{},
			fkRawCellValues:       map[string]string{},
			tableName:             "public.orders",
		},
		DBDriver: db,
	}

	table.SetForeignKeys([][]string{
		{"constraint_name", "column_name", "foreign_table_name", "foreign_column_name"},
		{"fk_orders_user", "user_id", "users", "id"},
		{"fk_orders_loc", "country_code", "locations", "country_code"},
		{"fk_orders_loc", "city_code", "locations", "city_code"},
	})

	target, ok := table.getForeignKeyJumpTarget("user_id")
	if !ok {
		t.Fatal("expected single-column fk jump target for user_id")
	}

	if target.ReferencedTable != "public.users" {
		t.Fatalf("expected referenced table public.users, got %q", target.ReferencedTable)
	}

	if target.ReferencedColumn != "id" {
		t.Fatalf("expected referenced column id, got %q", target.ReferencedColumn)
	}

	if table.isForeignKeyColumn("country_code") {
		t.Fatal("expected composite FK column country_code to be excluded from jump metadata")
	}
}

func TestRebuildForeignKeyJumpMetadataUnsupportedProvider(t *testing.T) {
	changes := []models.DBDMLChange{}

	db := &drivers.MySQL{}
	db.SetProvider(drivers.DriverMySQL)

	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			listOfDBChanges:       &changes,
			foreignKeyColumns:     map[string]bool{},
			foreignKeyJumpTargets: map[string]foreignKeyJumpTarget{},
			fkRawCellValues:       map[string]string{},
		},
		DBDriver: db,
	}

	table.SetForeignKeys([][]string{
		{"TABLE_NAME", "COLUMN_NAME", "CONSTRAINT_NAME", "REFERENCED_COLUMN_NAME", "REFERENCED_TABLE_NAME"},
		{"orders", "user_id", "fk_user", "id", "users"},
	})

	if len(table.state.foreignKeyJumpTargets) != 0 {
		t.Fatalf("expected no fk jump targets for unsupported provider, got %d", len(table.state.foreignKeyJumpTargets))
	}
}

func TestHandleForeignKeyEnterConsumesOnNullValues(t *testing.T) {
	changes := []models.DBDMLChange{}

	db := &drivers.Postgres{}
	db.SetProvider(drivers.DriverPostgres)

	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			listOfDBChanges:       &changes,
			columns:               [][]string{{"Field"}, {"user_id"}},
			foreignKeyColumns:     map[string]bool{"user_id": true},
			foreignKeyJumpTargets: map[string]foreignKeyJumpTarget{"user_id": {ReferencedTable: "public.users", ReferencedColumn: "id"}},
			fkRawCellValues:       map[string]string{},
		},
		DBDriver: db,
	}

	table.SetCell(1, 0, tview.NewTableCell("NULL"))

	if consumed := table.handleForeignKeyEnter(1, 0); !consumed {
		t.Fatal("expected Enter to be consumed on FK column with NULL value")
	}
}

func TestShouldShowForeignKeyMarker(t *testing.T) {
	changes := []models.DBDMLChange{}

	db := &drivers.Postgres{}
	db.SetProvider(drivers.DriverPostgres)

	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			listOfDBChanges:       &changes,
			columns:               [][]string{{"Field"}, {"user_id"}},
			foreignKeyColumns:     map[string]bool{"user_id": true},
			foreignKeyJumpTargets: map[string]foreignKeyJumpTarget{"user_id": {ReferencedTable: "public.users", ReferencedColumn: "id"}},
			fkRawCellValues:       map[string]string{},
		},
		DBDriver: db,
	}

	table.SetCell(1, 0, tview.NewTableCell("7"))

	if !table.shouldShowForeignKeyMarker(1, 0, "7") {
		t.Fatal("expected FK marker for navigable FK value")
	}

	if table.shouldShowForeignKeyMarker(1, 0, "NULL") {
		t.Fatal("expected no FK marker for NULL FK value")
	}
}

func TestRebuildForeignKeyJumpMetadataPostgresUsesForeignTableSchemaColumn(t *testing.T) {
	changes := []models.DBDMLChange{}

	db := &drivers.Postgres{}
	db.SetProvider(drivers.DriverPostgres)

	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			listOfDBChanges:       &changes,
			foreignKeyColumns:     map[string]bool{},
			foreignKeyJumpTargets: map[string]foreignKeyJumpTarget{},
			fkRawCellValues:       map[string]string{},
			tableName:             "public.orders",
		},
		DBDriver: db,
	}

	table.SetForeignKeys([][]string{
		{"constraint_name", "column_name", "foreign_table_schema", "foreign_table_name", "foreign_column_name"},
		{"fk_orders_user", "user_id", "auth", "users", "id"},
	})

	target, ok := table.getForeignKeyJumpTarget("user_id")
	if !ok {
		t.Fatal("expected fk jump target for user_id")
	}

	if target.ReferencedTable != "auth.users" {
		t.Fatalf("expected referenced table auth.users, got %q", target.ReferencedTable)
	}
}

// ── Records loading ────────────────────────────────────────────────────────────

type recordsFirstPaintMock struct {
	schemaProgrammingMock

	mu            sync.Mutex
	pageCalls     int
	metadataCalls int
}

func (m *recordsFirstPaintMock) GetRecords(context.Context, string, string, string, string, int, int) (drivers.PageResult, error) {
	m.mu.Lock()
	m.pageCalls++
	m.mu.Unlock()

	return drivers.PageResult{
		Rows: [][]string{{"id"}, {"1"}, {"2"}},
	}, nil
}

func (m *recordsFirstPaintMock) GetTableColumns(context.Context, string, string) ([][]string, error) {
	m.mu.Lock()
	m.metadataCalls++
	m.mu.Unlock()
	return nil, errors.New("metadata intentionally deferred")
}

func (m *recordsFirstPaintMock) counts() (pageCalls, metadataCalls int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pageCalls, m.metadataCalls
}

func newRecordsFetchTestTable(driver drivers.Driver) *ResultsTable {
	changes := []models.DBDMLChange{}
	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			records:               [][]string{},
			columns:               [][]string{},
			constraints:           [][]string{},
			foreignKeys:           [][]string{},
			indexes:               [][]string{},
			foreignKeyColumns:     map[string]bool{},
			foreignKeyJumpTargets: map[string]foreignKeyJumpTarget{},
			fkRawCellValues:       map[string]string{},
			markedRows:            map[int]bool{},
			listOfDBChanges:       &changes,
		},
		Pagination: NewPagination(),
		DBDriver:   driver,
	}
	table.SetDatabaseName("database")
	table.SetTableName("table")
	return table
}

func TestFetchRecordsRendersBeforeMetadata(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init simulation screen: %v", err)
	}
	App.SetScreen(screen)

	driver := &recordsFirstPaintMock{}
	table := newRecordsFetchTestTable(driver)
	root := tview.NewPages()
	root.AddPage(pageNameTable, table, true, true)
	appDone := make(chan struct{})
	go func() {
		defer close(appDone)
		_ = App.Run(root, "")
	}()
	App.QueueUpdate(func() {})

	rendered := make(chan struct{})
	table.FetchRecords(nil, func() {
		pageCalls, metadataCalls := driver.counts()
		if pageCalls != 1 {
			t.Errorf("expected one page fetch before first paint, got %d", pageCalls)
		}
		if metadataCalls != 0 {
			t.Errorf("metadata ran before first paint: %d calls", metadataCalls)
		}
		close(rendered)
	})
	<-rendered

	table.CancelLoading()
	App.Application.Stop()
	<-appDone
}

type slowForeignKeyFirstPaintMock struct {
	recordsFirstPaintMock
	started chan struct{}
	once    sync.Once
}

func (m *slowForeignKeyFirstPaintMock) GetForeignKeys(ctx context.Context, _, _ string) ([][]string, error) {
	m.once.Do(func() { close(m.started) })
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestFetchRecordsRendersBeforeSlowForeignKeys(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init simulation screen: %v", err)
	}
	App.SetScreen(screen)

	driver := &slowForeignKeyFirstPaintMock{started: make(chan struct{})}
	table := newRecordsFetchTestTable(driver)
	root := tview.NewPages()
	root.AddPage(pageNameTable, table, true, true)
	appDone := make(chan struct{})
	go func() {
		defer close(appDone)
		_ = App.Run(root, "")
	}()
	App.QueueUpdate(func() {})

	rendered := make(chan struct{})
	table.FetchRecords(nil, func() {
		pageCalls, _ := driver.counts()
		if pageCalls != 1 {
			t.Errorf("expected one page fetch before first paint, got %d", pageCalls)
		}
		select {
		case <-driver.started:
			t.Errorf("slow Foreign Keys lookup started before Records first paint")
		default:
		}
		close(rendered)
	})

	select {
	case <-rendered:
	case <-time.After(3 * time.Second):
		t.Fatal("Records first paint was blocked by Foreign Keys metadata")
	}
	select {
	case <-driver.started:
	case <-time.After(3 * time.Second):
		t.Fatal("Foreign Keys metadata lookup did not start in the background")
	}

	table.CancelLoading()
	App.Application.Stop()
	<-appDone
}

type staleRecordsLoadMock struct {
	schemaProgrammingMock

	firstStarted  chan struct{}
	firstCanceled chan struct{}
	mu            sync.Mutex
	calls         int
}

func newStaleRecordsLoadMock() *staleRecordsLoadMock {
	return &staleRecordsLoadMock{
		firstStarted:  make(chan struct{}),
		firstCanceled: make(chan struct{}),
	}
}

func (m *staleRecordsLoadMock) GetRecords(ctx context.Context, _ string, _ string, _ string, _ string, _ int, _ int) (drivers.PageResult, error) {
	m.mu.Lock()
	m.calls++
	call := m.calls
	m.mu.Unlock()

	if call == 1 {
		close(m.firstStarted)
		<-ctx.Done()
		close(m.firstCanceled)
		return drivers.PageResult{}, ctx.Err()
	}

	return drivers.PageResult{Rows: [][]string{{"id"}, {"new"}}}, nil
}

func (m *staleRecordsLoadMock) GetTableColumns(context.Context, string, string) ([][]string, error) {
	return nil, errors.New("metadata intentionally deferred")
}

func TestStaleRecordsLoadCannotOverwriteNewerPage(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init simulation screen: %v", err)
	}
	App.SetScreen(screen)

	driver := newStaleRecordsLoadMock()
	table := newRecordsFetchTestTable(driver)
	root := tview.NewPages()
	root.AddPage(pageNameTable, table, true, true)
	appDone := make(chan struct{})
	go func() {
		defer close(appDone)
		_ = App.Run(root, "")
	}()
	App.QueueUpdate(func() {})

	table.FetchRecords(nil, nil)
	<-driver.firstStarted

	newerRendered := make(chan struct{})
	table.FetchRecords(nil, func() {
		close(newerRendered)
	})

	<-driver.firstCanceled
	<-newerRendered

	records := table.GetRecords()
	if len(records) != 2 || records[1][0] != "new" {
		t.Fatalf("stale page overwrote newer page: %v", records)
	}

	table.CancelLoading()
	App.Application.Stop()
	<-appDone
}

// ── read-only routing ──────────────────────────────────────────────────────────

// readOnlyRoutingMock records every query the editor pipeline sends to the
// driver, so a test can assert that a mutation never reaches the database.
type readOnlyRoutingMock struct {
	schemaProgrammingMock

	mu       sync.Mutex
	executed []string
}

func (m *readOnlyRoutingMock) record(query string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executed = append(m.executed, query)
}

func (m *readOnlyRoutingMock) queries() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.executed...)
}

func (m *readOnlyRoutingMock) ExecuteQuery(_ context.Context, query string) ([][]string, int, error) {
	m.record(query)
	return [][]string{{"col"}}, 0, nil
}

func (m *readOnlyRoutingMock) ExecuteDMLStatement(_ context.Context, query string) (string, error) {
	m.record(query)
	return "", nil
}

// runEditorQuery drives subscribeToEditorChanges for a single query and returns
// the queries that actually reached the driver.
func runEditorQuery(t *testing.T, readOnly bool, query string) []string {
	t.Helper()

	// A simulation screen lets the queued UI updates actually run, so the
	// editor pipeline behaves as it does in the real application.
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init simulation screen: %v", err)
	}
	App.SetScreen(screen)

	driver := &readOnlyRoutingMock{}
	changes := []models.DBDMLChange{}
	errorModal := tview.NewModal()

	pages := tview.NewPages()
	pages.AddPage(pageNameTable, tview.NewFlex(), true, true)
	pages.AddPage(pageNameTableError, errorModal, true, false)

	editor := NewSQLEditor("")

	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			records:         [][]string{},
			listOfDBChanges: &changes,
		},
		Page:        pages,
		Wrapper:     tview.NewFlex(),
		Error:       errorModal,
		Pagination:  NewPagination(),
		ResultsInfo: tview.NewTextView(),
		EditorPages: tview.NewPages(),
		Editor:      editor,
		DBDriver:    driver,
		ReadOnly:    readOnly,
	}
	table.EditorPages.AddPage(pageNameTableEditorTable, tview.NewFlex(), true, true)

	go table.subscribeToEditorChanges()

	appDone := make(chan struct{})
	go func() {
		defer close(appDone)
		_ = App.Run(pages, "")
	}()

	// Wait for the application loop to start draining queued updates.
	time.Sleep(100 * time.Millisecond)
	editor.Publish(eventSQLEditorQuery, query)
	time.Sleep(400 * time.Millisecond)

	App.Application.Stop()
	<-appDone

	return driver.queries()
}

func TestSchemaMutatingQueryClassification(t *testing.T) {
	for _, query := range []string{
		"CREATE TABLE users (id INTEGER)",
		"-- alter a table\nALTER TABLE users ADD COLUMN name TEXT",
		"/* drop is intentional */ DROP VIEW users_view",
		"TRUNCATE TABLE users",
	} {
		if !isSchemaMutatingQuery(query) {
			t.Errorf("isSchemaMutatingQuery(%q) = false, want true", query)
		}
	}

	for _, query := range []string{
		"INSERT INTO users (id) VALUES (1)",
		"UPDATE users SET name = 'new'",
		"DELETE FROM users WHERE id = 1",
		"WITH changed AS (SELECT 1) UPDATE users SET id = 1",
		"SELECT * FROM users",
	} {
		if isSchemaMutatingQuery(query) {
			t.Errorf("isSchemaMutatingQuery(%q) = true, want false", query)
		}
	}
}

// TestReadOnlyBlocksMultiLineCTEMutation covers the routing bug: a mutation
// wrapped in a CTE starts with "with", so the prefix check classified it as a
// SELECT and it executed without ever reaching the read-only validator.
func TestReadOnlyBlocksMultiLineCTEMutation(t *testing.T) {
	query := "WITH cte AS (\n  SELECT 1\n)\nINSERT INTO users SELECT * FROM cte"

	if executed := runEditorQuery(t, true, query); len(executed) != 0 {
		t.Fatalf("read-only connection executed a mutation, got %q", executed)
	}
}

// TestReadOnlyAllowsCTESelect is the contract that must hold: a read-only CTE
// is still a plain read and must keep running.
func TestReadOnlyAllowsCTESelect(t *testing.T) {
	query := "WITH cte AS (\n  SELECT 1\n)\nSELECT * FROM cte"

	executed := runEditorQuery(t, true, query)
	if len(executed) != 1 || executed[0] != query {
		t.Fatalf("read-only WITH ... SELECT should still execute, got %q", executed)
	}
}
