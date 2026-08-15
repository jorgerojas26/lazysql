package components

import (
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

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

func (m *readOnlyRoutingMock) ExecuteQuery(query string) ([][]string, int, error) {
	m.record(query)
	return [][]string{{"col"}}, 0, nil
}

func (m *readOnlyRoutingMock) ExecuteDMLStatement(query string) (string, error) {
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
