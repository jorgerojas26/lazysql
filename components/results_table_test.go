package components

import (
	"testing"
	"time"

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
