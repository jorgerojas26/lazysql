package components

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/drivers"
)

func TestCopyRowRangeAndSelection(t *testing.T) {
	table := newMarkTestTable([][]string{{"id"}, {"1"}, {"2"}, {"3"}, {"4"}})
	table.SetSelectionChangedFunc(func(row, _ int) { table.previewRowRange(row) })
	key := func(r rune) { table.tableInputCapture(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)) }
	color := func(row int) tcell.Color { return table.GetCell(row, 0).BackgroundColor }

	table.Select(4, 0)
	key('v')
	table.Select(2, 0)
	if len(table.GetMarkedRowIndexes()) != 0 || !reflect.DeepEqual(table.copyRowIndexes(2), []int{2, 3, 4}) {
		t.Fatalf("live reverse range: marks=%v copy=%v", table.GetMarkedRowIndexes(), table.copyRowIndexes(2))
	}
	for _, row := range []int{2, 3, 4} {
		if color(row) != app.Styles.TableMarkedColor {
			t.Fatalf("row %d not highlighted in visual range", row)
		}
	}
	table.Select(3, 0)
	if color(2) == app.Styles.TableMarkedColor {
		t.Fatal("row leaving range should lose preview highlight")
	}
	table.Select(2, 0)
	key(' ')
	if got := table.GetMarkedRowIndexes(); !reflect.DeepEqual(got, []int{2, 3, 4}) {
		t.Fatalf("committed range marks = %v", got)
	}
	table.Select(1, 0)
	key('v')
	if got := table.copyRowIndexes(1); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("visual range should override existing marks: %v", got)
	}
	key('v') // Toggle off without committing row 1.
	if color(1) == app.Styles.TableMarkedColor || table.state.rangeAnchor != 0 {
		t.Fatal("v should cancel visual selection and restore colors")
	}
	key('v')
	table.tableInputCapture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	if table.state.rangeAnchor != 0 || len(table.GetMarkedRowIndexes()) != 3 {
		t.Fatal("Esc should cancel visual selection without changing marks")
	}
	key(' ')
	if got := table.GetMarkedRowIndexes(); !reflect.DeepEqual(got, []int{1, 2, 3, 4}) {
		t.Fatalf("individual mark after range = %v", got)
	}
	table.Select(2, 0)
	key('v')
	table.clearRowMarks()
	if table.state.rangeAnchor != 0 || len(table.GetMarkedRowIndexes()) != 0 {
		t.Fatal("rebuild should discard range preview and marks")
	}
}

func TestFormatCopyRows(t *testing.T) {
	table := newMarkTestTable([][]string{
		{"id", "name", "notes"},
		{"001", "Alice", "a|b"},
		{"002", "O'Brien", "NULL"},
	})
	table.GetCell(2, 2).SetReference("NULL&")
	table.state.tableName = "people"
	table.Menu = NewResultsTableMenu()
	table.DBDriver = &drivers.SQLite{}

	for _, tc := range []struct{ format, want string }{
		{"SQL", "INSERT INTO `people` (`id`, `name`, `notes`) VALUES\n('001', 'Alice', 'a|b');"},
		{"Markdown", "| id | name | notes |\n| --- | --- | --- |\n| 001 | Alice | a\\|b |"},
		{"JSON", "[\n  {\n    \"id\": \"001\",\n    \"name\": \"Alice\",\n    \"notes\": \"a|b\"\n  }\n]"},
		{"CSV", "id,name,notes\n001,Alice,a|b\n"},
	} {
		got, err := table.formatCopyRows(1, tc.format)
		if err != nil || got != tc.want {
			t.Errorf("%s = %q, %v; want %q", tc.format, got, err, tc.want)
		}
	}

	table.toggleRowMark(2)
	table.toggleRowMark(1)
	got, err := table.formatCopyRows(1, "SQL")
	if err != nil || got != "INSERT INTO `people` (`id`, `name`, `notes`) VALUES\n('001', 'Alice', 'a|b'),\n('002', 'O''Brien', NULL);" {
		t.Fatalf("marked SQL = %q, %v", got, err)
	}
	got, err = table.formatCopyRows(1, "JSON")
	if err != nil || !strings.Contains(got, `"notes": null`) || !strings.Contains(got, `"id": "002"`) {
		t.Fatalf("marked JSON = %q, %v", got, err)
	}
	table.state.markedRows[99] = true
	got, err = table.formatCopyRows(1, "CSV")
	if err != nil || got != "id,name,notes\n001,Alice,a|b\n002,O'Brien,NULL\n" {
		t.Fatalf("marked CSV = %q, %v", got, err)
	}
}

func TestCopyFormatsEscapeSpecialValues(t *testing.T) {
	table := newMarkTestTable([][]string{{`a"b`, "multiline"}, {`[red]|\\`, "first\nsecond,\"third\""}})
	table.GetCell(1, 0).SetText(tview.Escape(`[red]|\\`))

	csvText, err := table.formatCopyRows(1, "CSV")
	if err != nil || !strings.Contains(csvText, `"first`+"\n"+`second,""third"""`) {
		t.Fatalf("CSV escaping = %q, %v", csvText, err)
	}
	markdown, err := table.formatCopyRows(1, "Markdown")
	if err != nil || !strings.Contains(markdown, `\|`) || !strings.Contains(markdown, "first<br>second") {
		t.Fatalf("Markdown escaping = %q, %v", markdown, err)
	}
	jsonText, err := table.formatCopyRows(1, "JSON")
	var decoded []map[string]string
	if err != nil || json.Unmarshal([]byte(jsonText), &decoded) != nil || len(decoded) != 1 || decoded[0][`a"b`] != `[red]|\\` || decoded[0]["multiline"] != "first\nsecond,\"third\"" {
		t.Fatalf("JSON escaping = %q, %v", jsonText, err)
	}
}

func TestSQLCopyFromEditorResults(t *testing.T) {
	table := newMarkTestTable([][]string{{"x", "y"}, {"a", "b"}, {"c", "d"}, {"e", "f"}})
	// An SQL editor result is also a ResultsTable, but has no records menu
	// or target table. Its SQL representation is a standalone VALUES clause.
	got, err := table.formatCopyRows(2, "SQL")
	if err != nil || got != "VALUES\n('c', 'd');" {
		t.Fatalf("editor result SQL = %q, %v", got, err)
	}
	table.toggleRowMark(3)
	table.Select(1, 0)
	table.tableInputCapture(tcell.NewEventKey(tcell.KeyRune, 'v', tcell.ModNone))
	table.previewRowRange(2) // The selection callback does this in real tables.
	got, err = table.formatCopyRows(2, "SQL")
	if err != nil || got != "VALUES\n('a', 'b'),\n('c', 'd');" {
		t.Fatalf("editor visual SQL (without previously marked row) = %q, %v", got, err)
	}
	if got := table.markedRowsToText(); got != "e\tf" {
		t.Fatalf("lowercase y must still use only committed marks, got %q", got)
	}
	table.cancelRowRange()
	if got := table.copyRowIndexes(2); !reflect.DeepEqual(got, []int{3}) {
		t.Fatalf("cancelling visual range should restore individual marks: %v", got)
	}
}

func TestVisualRangeRestoresExistingRowBackground(t *testing.T) {
	table := newMarkTestTable([][]string{{"x", "y"}, {"one", "1"}, {"two", "2"}, {"three", "3"}})
	table.GetCell(2, 1).SetBackgroundColor(app.Styles.TableDeleteColor)
	assertOriginal := func(row int, originals []rowCellPresentation) {
		t.Helper()
		for col, want := range originals {
			cell := table.GetCell(row, col)
			if cell.Style != want.style || cell.BackgroundColor != want.background || cell.Transparent != want.transparent {
				t.Errorf("row %d col %d presentation after preview = %v/%v/%v, want %+v", row, col, cell.Style, cell.BackgroundColor, cell.Transparent, want)
			}
		}
	}
	originals := make([]rowCellPresentation, 2)
	for col := range originals {
		cell := table.GetCell(2, col)
		originals[col] = rowCellPresentation{cell.Style, cell.BackgroundColor, cell.Transparent}
	}
	table.Select(1, 0)
	table.tableInputCapture(tcell.NewEventKey(tcell.KeyRune, 'v', tcell.ModNone))
	table.previewRowRange(2)
	table.previewRowRange(1)
	assertOriginal(2, originals)
	table.previewRowRange(2)
	table.tableInputCapture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	assertOriginal(2, originals)

	// A pre-existing Space mark must survive a cancelled preview too.
	table.toggleRowMark(2)
	for col := range originals {
		cell := table.GetCell(2, col)
		originals[col] = rowCellPresentation{cell.Style, cell.BackgroundColor, cell.Transparent}
	}
	table.Select(2, 0)
	table.tableInputCapture(tcell.NewEventKey(tcell.KeyRune, 'v', tcell.ModNone))
	table.previewRowRange(3)
	table.tableInputCapture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
	assertOriginal(2, originals)
}

func TestCopyRowsAsKeyOpensPicker(t *testing.T) {
	table := newMarkTestTable([][]string{{"id"}, {"1"}})
	table.Page = tview.NewPages()
	table.Select(1, 0)
	table.tableInputCapture(tcell.NewEventKey(tcell.KeyRune, 'Y', tcell.ModNone))
	if !table.Page.HasPage(pageNameCopyRowsAs) {
		t.Fatal("Y should show format picker on editor result table")
	}
}
