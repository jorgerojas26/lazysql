package components

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/models"
)

func TestColumnVisibilityTableIntegration(t *testing.T) {
	previousApplication := App.Application
	previousPages := mainPages
	App.Application = tview.NewApplication()
	mainPages = tview.NewPages()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(100, 24)
	App.SetScreen(screen)

	table := newRecordsFetchTestTable(&pendingForeignKeyDriver{})
	table.connectionIdentifier = t.Name()
	table.Menu = NewResultsTableMenu()
	table.Page = tview.NewPages()
	table.SetSelectable(true, true).SetFixed(1, 0).SetBorders(true)
	table.SetInputCapture(table.tableInputCapture)
	table.Page.AddPage("table", tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, true).AddItem(table.Pagination, 3, 0, false), true, true)
	mainPages.AddPage("records", table.Page, true, true)
	path := filepath.Join(t.TempDir(), "config.toml")
	done := make(chan error, 1)
	go func() { done <- App.Run(mainPages, path) }()
	App.QueueUpdate(func() {})
	t.Cleanup(func() {
		App.Application.Stop()
		if err := <-done; err != nil {
			t.Error(err)
		}
		App.Application = previousApplication
		mainPages = previousPages
	})

	rows := [][]string{{"id", "secret", "name", "team_id"}, {"row_pk", "secret_value", "Alice", "42"}}
	var setupError error
	var sibling *ResultsTable
	App.QueueUpdateDraw(func() {
		setupError = App.SaveHiddenColumns(table.connectionIdentifier, "database", "table", []string{"id", "secret"})
		table.SetPrimaryKeyColumnNames([]string{"id"})
		table.state.foreignKeyColumns["team_id"] = true
		table.state.foreignKeyJumpTargets["team_id"] = foreignKeyJumpTarget{ReferencedTable: "teams", ReferencedColumn: "id"}
		table.SetRecords(rows)
		sibling = newRecordsFetchTestTable(&schemaProgrammingMock{})
		sibling.connectionIdentifier = table.connectionIdentifier
		sibling.Menu = NewResultsTableMenu()
		sibling.SetRecords(rows)
		table.Home = &Home{TabbedPane: &TabbedPane{state: &TabbedPaneState{FirstTab: &Tab{Content: table, NextTab: &Tab{Content: sibling}}}}}
		App.SetFocus(table)
	})
	if setupError != nil {
		t.Fatal(setupError)
	}
	text := simulationText(screen)
	if strings.Contains(text, "secret_value") || strings.Contains(text, "row_pk") || !strings.Contains(text, "Alice") || !strings.Contains(text, "Columns [V]") || !strings.Contains(text, "2 hidden") {
		t.Fatalf("unexpected visible grid:\n%s", text)
	}

	key := func(code tcell.Key, character rune) {
		App.QueueUpdateDraw(func() {
			mainPages.InputHandler()(tcell.NewEventKey(code, character, tcell.ModNone), func(p tview.Primitive) { App.SetFocus(p) })
		})
	}
	checkSelection := func(want int) {
		App.QueueUpdate(func() {
			_, column := table.GetSelection()
			if column != want {
				t.Errorf("selected raw column = %d, want %d", column, want)
			}
		})
	}
	checkSelection(2)
	key(tcell.KeyRune, 'w')
	checkSelection(3)
	key(tcell.KeyRune, 'b')
	checkSelection(2)
	key(tcell.KeyRight, 0)
	checkSelection(3)
	key(tcell.KeyLeft, 0)
	checkSelection(2)
	key(tcell.KeyRune, '$')
	checkSelection(3)
	key(tcell.KeyRune, '0')
	checkSelection(2)

	App.QueueUpdateDraw(func() {
		x, y, _ := table.GetCell(1, 3).GetLastPosition()
		mainPages.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone), func(p tview.Primitive) { App.SetFocus(p) })
		if !reflect.DeepEqual(table.GetPrimaryKeyValue(1), []models.PrimaryKeyInfo{{Name: "id", Value: "row_pk"}}) {
			t.Error("hidden primary key is unavailable")
		}
		if table.GetColumnCount() != 4 || table.Table.GetColumnCount() != 2 || !reflect.DeepEqual(table.GetRecords(), rows) {
			t.Error("display projection changed backing data")
		}
		if table.GetColumnNameByIndex(3) != "team_id" || table.getRawCellValue(1, 3) != "42" {
			t.Error("selected cell column identity changed")
		}
		_, _, attributes := table.Table.GetCell(1, 1).Style.Decompose()
		if attributes&tcell.AttrUnderline == 0 {
			t.Error("foreign key markers should follow projected columns")
		}
	})
	checkSelection(3)

	App.QueueUpdateDraw(func() {
		if err := App.SaveHiddenColumns(table.connectionIdentifier, "database", "table", []string{"id", "name"}); err != nil {
			t.Error(err)
		}
		table.refreshColumnVisibility()
		table.Select(1, 1)
	})
	key(tcell.KeyRune, 'c')
	key(tcell.KeyTab, 0)
	checkSelection(3)
	key(tcell.KeyBacktab, 0)
	checkSelection(1)
	key(tcell.KeyEscape, 0)
	App.QueueUpdateDraw(func() {
		if table.GetIsEditing() || App.GetFocus() != table {
			t.Error("inline editing did not close or restore focus")
		}
		if err := App.SaveHiddenColumns(table.connectionIdentifier, "database", "table", []string{"id", "secret"}); err != nil {
			t.Error(err)
		}
		table.refreshColumnVisibility()
		table.Select(1, 3)
		table.ReadOnly = true
	})

	key(tcell.KeyRune, 'V')
	App.QueueUpdate(func() {
		name, _ := mainPages.GetFrontPage()
		if name != "column-visibility" {
			t.Error("V did not open column picker")
		}
	})
	key(tcell.KeyRune, 'A')
	key(tcell.KeyEscape, 0)
	App.QueueUpdate(func() {
		if table.Table.GetColumnCount() != 2 || !reflect.DeepEqual(App.HiddenColumns(table.connectionIdentifier, "database", "table"), []string{"id", "secret"}) {
			t.Error("cancel changed saved visibility")
		}
	})
	checkSelection(3)

	var changedCell *tview.TableCell
	App.QueueUpdateDraw(func() {
		changedCell = table.GetCell(1, 2)
		changedCell.SetText("Changed")
		if err := table.AppendNewChange(models.DMLUpdateType, 1, 2, models.CellValue{Type: models.String, Column: "name", Value: "Changed", TableRowIndex: 1, TableColumnIndex: 2}); err != nil {
			t.Error(err)
		}
		table.toggleRowMark(1)
		x, y, width, _ := table.Pagination.GetRect()
		mainPages.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x+width-3, y, tcell.Button1, tcell.ModNone), func(p tview.Primitive) { App.SetFocus(p) })
	})
	key(tcell.KeyRune, 'A')
	key(tcell.KeyEnter, 0)
	App.QueueUpdateDraw(func() {
		if sibling.Table.GetColumnCount() != 4 {
			t.Error("applying visibility should update other tabs for the same table")
		}
		if table.Table.GetColumnCount() != 4 || table.GetCell(1, 2) != changedCell || changedCell.Text != "Changed" || len(table.GetMarkedRowIndexes()) != 1 {
			t.Error("applying visibility discarded cell presentation, pending edits, or row marks")
		}
		if len(*table.state.listOfDBChanges) != 1 || (*table.state.listOfDBChanges)[0].Values[0].Column != "name" {
			t.Error("visibility changed pending DML")
		}
		if err := table.AppendNewChange(models.DMLUpdateType, 1, 2, models.CellValue{Type: models.String, Column: "name", Value: "Alice", TableRowIndex: 1, TableColumnIndex: 2}); err != nil {
			t.Error(err)
		}
		if len(*table.state.listOfDBChanges) != 0 {
			t.Error("edit revert compared against the wrong raw column")
		}
	})

	App.QueueUpdateDraw(func() {
		if err := App.SaveHiddenColumns(table.connectionIdentifier, "database", "table", []string{"id", "secret"}); err != nil {
			t.Error(err)
		}
		table.SetRecords(rows)
		table.Menu.SetSelectedOption(2)
		table.UpdateRows([][]string{{"Field", "Type"}, {"id", "text"}})
		if table.Table.GetColumnCount() != 2 || table.Pagination.GetTitle() != "" || table.GetCell(1, 0).Text != "id" {
			t.Error("metadata surface should not hide columns")
		}
		table.Menu.SetSelectedOption(1)
		table.SetRecords(rows)
		if table.Table.GetColumnCount() != 2 {
			t.Error("visibility did not survive returning to Records")
		}
		if got, err := table.formatCopyRows(1, "JSON"); err != nil || !strings.Contains(got, "secret_value") {
			t.Error("row copy should retain hidden columns")
		}
		csv := filepath.Join(t.TempDir(), "export.csv")
		if _, err := table.exportCurrentPage(csv); err != nil {
			t.Error(err)
		} else if data, err := os.ReadFile(csv); err != nil || !strings.Contains(string(data), "secret_value") {
			t.Error("CSV export should retain hidden columns")
		}
		table.SetRecords([][]string{{"id", "secret", "name", "team_id", "new_column"}})
		if table.Table.GetColumnCount() != 3 {
			t.Error("new columns should be visible even on an empty table")
		}
		table.SetRecords([][]string{{"id", "secret"}})
		if table.Table.GetColumnCount() != 1 || table.Table.GetCell(0, 0).Text != "id" {
			t.Error("stale preferences must leave at least one column visible")
		}
		table.Editor = &SQLEditor{}
		table.SetRecords(rows)
		if table.Table.GetColumnCount() != 4 || table.Pagination.GetTitle() != "" {
			t.Error("SQL editor results should not use table visibility preferences")
		}
		table.Editor = nil
		table.ReadOnly = false
		table.SetColumns([][]string{{"Field", "Type"}, {"id", "text"}, {"secret", "text"}, {"name", "text"}, {"team_id", "text"}})
		table.SetRecords(rows)
	})
	key(tcell.KeyRune, 'O')
	checkSelection(2)
	App.QueueUpdate(func() {
		if table.GetCell(2, 0).Text != "row_pk" || table.GetCell(2, 1).Text != "secret_value" || table.GetCell(2, 2).Text != "Alice" {
			t.Error("duplicated rows should retain hidden cell values")
		}
	})
	key(tcell.KeyEscape, 0)
	key(tcell.KeyRune, 'o')
	checkSelection(2)
	App.QueueUpdate(func() {
		if input, ok := App.GetFocus().(*tview.InputField); ok {
			input.SetText("New name")
		} else {
			t.Error("inserting a row should edit the first visible column")
		}
	})
	key(tcell.KeyEnter, 0)
	App.QueueUpdate(func() {
		if table.GetCell(3, 2).Text != "New name" || len(*table.state.listOfDBChanges) != 2 || len((*table.state.listOfDBChanges)[1].Values) != 4 {
			t.Error("new row editing should preserve complete inserts with correct column identities")
		}
	})
}
