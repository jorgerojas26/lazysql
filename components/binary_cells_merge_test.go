package components

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

func TestBinarySidebarCopyKeepsHiddenColumnValues(t *testing.T) {
	previous := App.Application
	App.Application = tview.NewApplication()
	t.Cleanup(func() { App.Application = previous })
	changes := []models.DBDMLChange{}
	table := NewResultsTable(&changes, &Tree{}, &drivers.MySQL{}, nil, t.Name(), "", false)
	if table.columnView == nil {
		t.Fatal("column visibility must be initialized alongside binary sidebar copying")
	}
	table.Menu = NewResultsTableMenu()
	table.Filter = NewResultsFilter()
	table.SetRect(0, 0, 100, 24)
	table.SidebarContainer.SetRect(0, 0, 80, 24)
	table.state.databaseName = "app"
	table.state.tableName = "sessions"
	table.SetPrimaryKeyColumnNames([]string{"id"})
	table.SetRecords([][]string{{"id", "token", "note"}, {binaryID, "\x00\x01\xff", "first"}, {"abcdefghijklmnop", "\x00\x02", "second"}})
	table.SetColumns([][]string{{"Field", "Type"}, {"id", "binary(16)"}, {"token", "varbinary(32)"}, {"note", "text"}})
	table.columnView.columns = []int{1, 2}
	table.Select(1, 1)
	table.UpdateSidebar()
	if table.Table.GetColumnCount() != 2 || table.Sidebar.Flex.GetItemCount() != 3 {
		t.Fatal("the grid must hide the primary key while the sidebar retains every field")
	}
	for col, want := range []string{binaryID, "\x00\x01\xff", "first"} {
		table.Sidebar.SetCurrentFieldIndex(col)
		if got := table.Sidebar.currentFieldValue(); got != want {
			t.Fatalf("sidebar raw column %d = %q, want %q", col, got, want)
		}
	}
	editBinaryCell(t, table, 1, 1, "0xCAFE")
	table.Sidebar.SetCurrentFieldIndex(1)
	if got := table.Sidebar.currentFieldValue(); got != "\xca\xfe" {
		t.Fatalf("edited sidebar copy = %q, want current raw bytes", got)
	}
	if len(changes) != 1 || changes[0].PrimaryKeyInfo[0].Value != binaryID {
		t.Fatal("editing a projected binary cell must preserve the hidden raw primary key")
	}
	table.Select(2, 1)
	table.UpdateSidebar()
	table.Sidebar.SetCurrentFieldIndex(1)
	if got := table.Sidebar.currentFieldValue(); got != "\x00\x02" {
		t.Fatalf("sidebar copy after selecting another row = %q", got)
	}
}

func TestAddRowsKeepsSpecialValueTextStyleAndReferences(t *testing.T) {
	table := newMarkTestTable(nil)
	table.DBDriver = &drivers.MySQL{}
	sentinels := []string{"NULL&", "EMPTY&", "DEFAULT&"}
	table.AddRows([][]string{sentinels, sentinels})
	for row := 0; row < 2; row++ {
		for col, want := range []string{"NULL", "EMPTY", "DEFAULT"} {
			cell := table.GetCell(row, col)
			_, _, attrs := cell.Style.Decompose()
			if cellText(cell) != want || cell.GetReference() != sentinels[col] || attrs&tcell.AttrItalic == 0 {
				t.Fatalf("special cell (%d,%d) lost its text, reference or italic style", row, col)
			}
		}
	}
}

func TestQueryResultsShowUnicodeControlsAsHex(t *testing.T) {
	table := newMarkTestTable(nil)
	table.DBDriver = &drivers.MySQL{}
	table.Editor = &SQLEditor{}
	table.state.records = [][]string{{"value"}, {"\u0085"}, {"\u009b"}}
	table.AddRows(table.state.records)
	for row, want := range []string{"0xC285", "0xC29B"} {
		if got := cellText(table.GetCell(row+1, 0)); got != want {
			t.Fatalf("query control display = %q, want %q", got, want)
		}
		if got := table.getRawCellValue(row+1, 0); got != table.state.records[row+1][0] {
			t.Fatalf("query control raw value changed to %q", got)
		}
	}
}
