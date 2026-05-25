package components

import (
	"testing"
	"time"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/models"
)

// TestSetLoadingIsSynchronous verifies that SetLoading does not use
// QueueUpdateDraw or any other blocking mechanism.
//
// Regression test for commit d5b0c4b which wrapped SetLoading in
// App.QueueUpdateDraw(), causing a deadlock when called from the
// main tview event loop goroutine (e.g., via tableInputCapture ->
// SetSortedBy when pressing J/K to sort).
//
// QueueUpdateDraw is blocking — it sends to the app's updates channel
// and waits for the event loop to execute the function. When called
// from the main goroutine, the event loop can't process both the
// current event handler and the queued update, causing a deadlock.
func TestSetLoadingIsSynchronous(t *testing.T) {
	changes := []models.DBDMLChange{}

	loadingModal := tview.NewModal()
	errorModal := tview.NewModal()

	pages := tview.NewPages()
	pages.AddPage(pageNameTable, tview.NewFlex(), true, true)
	pages.AddPage(pageNameTableLoading, loadingModal, false, false)
	pages.AddPage(pageNameTableError, errorModal, false, false)

	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			records:         [][]string{},
			isLoading:       false,
			listOfDBChanges: &changes,
		},
		Page:    pages,
		Loading: loadingModal,
		Error:   errorModal,
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
