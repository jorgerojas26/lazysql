package components

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/models"
)

// binaryID is a BINARY(16) UUID whose bytes include a tab and a newline, which
// used to break the table layout when shown as text.
const binaryID = "\x11\xee\x8c\x5b\x09\x42\x0a\xbc\xde\xf0\x12\x34\x56\x78\x9a\xbc"

func newBinaryCellsTable() *ResultsTable {
	rows := [][]string{
		{"id", "token", "note"},
		{binaryID, "\x00\x01\xff", "line one\nline two"},
	}

	table := newMarkTestTable(nil)
	table.DBDriver = &drivers.MySQL{}
	table.state.databaseName = "app"
	table.state.tableName = "sessions"
	table.state.primaryKeyColumnNames = []string{"id"}
	table.state.records = rows
	table.AddRows(rows)
	return table
}

func TestAddRowsShowsBinaryValuesAsHex(t *testing.T) {
	table := newBinaryCellsTable()

	tests := []struct {
		col  int
		text string
		raw  string
	}{
		{col: 0, text: "0x11EE8C5B09420ABCDEF0123456789ABC", raw: binaryID},
		{col: 1, text: "0x0001FF", raw: "\x00\x01\xff"},
		{col: 2, text: "line one\nline two", raw: "line one\nline two"},
	}

	for _, tt := range tests {
		if got := cellText(table.GetCell(1, tt.col)); got != tt.text {
			t.Errorf("cellText(1, %d) = %q, want %q", tt.col, got, tt.text)
		}
		// Foreign key jumps and copies keep using the record value.
		if got := table.getRawCellValue(1, tt.col); got != tt.raw {
			t.Errorf("getRawCellValue(1, %d) = %q, want %q", tt.col, got, tt.raw)
		}
	}
}

func TestEditedCellValueStoresHexAsBytes(t *testing.T) {
	table := newBinaryCellsTable()

	if got := table.editedCellValue(1, 1, "0xDEADBEEF"); got != "\xde\xad\xbe\xef" {
		t.Fatalf("editedCellValue(hex) = %q, want the decoded bytes", got)
	}
	if got := table.getRawCellValue(1, 1); got != "\x00\x01\xff" {
		t.Fatalf("conversion should not mutate raw data, got %q", got)
	}

	if got := table.editedCellValue(1, 1, "plain text"); got != "plain text" {
		t.Fatalf("editedCellValue(text) = %q, want it unchanged", got)
	}
	if got := table.editedCellValue(1, 1, "0x0001FF"); got != "\x00\x01\xff" {
		t.Fatalf("binary identity should survive text conversion, got %q", got)
	}

	// Hex typed into a text column is just text.
	if got := table.editedCellValue(1, 2, "0x41"); got != "0x41" {
		t.Fatalf("editedCellValue on a text cell = %q, want %q", got, "0x41")
	}
}

func TestBinaryCellEditKeepsRawPrimaryKey(t *testing.T) {
	table := newBinaryCellsTable()

	value := "0xCAFE"
	err := table.AppendNewChange(models.DMLUpdateType, 1, 1, models.CellValue{
		Type:             models.String,
		Value:            value,
		Column:           "token",
		TableColumnIndex: 1,
		TableRowIndex:    1,
	})
	if err != nil {
		t.Fatal(err)
	}

	changes := *table.state.listOfDBChanges
	if len(changes) != 1 {
		t.Fatalf("expected one pending change, got %d", len(changes))
	}
	change := changes[0]
	if len(change.PrimaryKeyInfo) != 1 || change.PrimaryKeyInfo[0].Value != binaryID {
		t.Fatalf("primary key = %+v, want the raw binary id", change.PrimaryKeyInfo)
	}
	if len(change.Values) != 1 || change.Values[0].Value != "\xca\xfe" {
		t.Fatalf("values = %+v, want the decoded bytes", change.Values)
	}

	// Typing the original hex back reverts the pending change.
	original := "0x0001FF"
	err = table.AppendNewChange(models.DMLUpdateType, 1, 1, models.CellValue{
		Type:             models.String,
		Value:            original,
		Column:           "token",
		TableColumnIndex: 1,
		TableRowIndex:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(*table.state.listOfDBChanges) != 0 {
		t.Fatalf("expected the change to be reverted, got %+v", *table.state.listOfDBChanges)
	}
}

func newBinaryEditingTable(t *testing.T) *ResultsTable {
	t.Helper()
	original := App.Application
	App.Application = tview.NewApplication()
	t.Cleanup(func() { App.Application = original })
	table := newBinaryCellsTable()
	table.Page = tview.NewPages()
	table.state.columns = [][]string{{"Field", "Type"}, {"id", "binary(16)"}, {"token", "varbinary(32)"}, {"note", "text"}}
	return table
}

func editBinaryCell(t *testing.T, table *ResultsTable, row, col int, text string) {
	t.Helper()
	table.StartEditingCell(row, col, nil)
	_, primitive := table.Page.GetFrontPage()
	input := primitive.(*tview.InputField)
	input.SetText(text)
	input.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
}

func TestBinaryInlineEditRevertsAfterText(t *testing.T) {
	table := newBinaryEditingTable(t)
	table.state.columns[2][1] = "text"
	editBinaryCell(t, table, 1, 1, "plain text")
	if got := table.getRawCellValue(1, 1); got != "plain text" {
		t.Fatalf("raw text edit = %q", got)
	}
	editBinaryCell(t, table, 1, 1, "0x0001FF")
	if got := len(*table.state.listOfDBChanges); got != 0 {
		t.Fatalf("original hex should revert, got %d pending changes", got)
	}
	if got := table.getRawCellValue(1, 1); got != "\x00\x01\xff" {
		t.Fatalf("raw reverted value = %q", got)
	}
}

func TestBinarySpecialValuesReplaceCachedBytes(t *testing.T) {
	for _, value := range []struct {
		kind models.CellValueType
		text string
	}{{models.Null, "NULL"}, {models.Default, "DEFAULT"}, {models.Empty, "EMPTY"}} {
		t.Run(value.text, func(t *testing.T) {
			table := newBinaryEditingTable(t)
			err := table.AppendNewChange(models.DMLUpdateType, 1, 1, models.CellValue{Type: value.kind, Value: value.text, Column: "token", TableRowIndex: 1, TableColumnIndex: 1})
			if err != nil {
				t.Fatal(err)
			}
			if got := table.getRawCellValue(1, 1); got != value.text {
				t.Fatalf("special raw value = %q, want %q", got, value.text)
			}
			if isNavigableForeignKeyValue(table.getRawCellValue(1, 1)) {
				t.Fatal("special value must not allow a Foreign Key Jump")
			}
			editBinaryCell(t, table, 1, 1, "0x0001FF")
			if len(*table.state.listOfDBChanges) != 0 {
				t.Fatal("original hex should revert the special value")
			}
		})
	}
}

func TestDuplicateBinaryRowKeepsRawValuesAndCellIdentity(t *testing.T) {
	table := newBinaryEditingTable(t)
	table.state.records = append(table.state.records, []string{"second", "\x00\x02", "second note"})
	table.AddRows(table.state.records)
	table.Select(1, 0)
	table.duplicateRow()
	changes := *table.state.listOfDBChanges
	if len(changes) != 1 || changes[0].Values[1].Value != "\x00\x01\xff" {
		t.Fatalf("duplicated token = %+v", changes)
	}
	if changes[0].Values[0].Value != binaryID {
		t.Fatalf("duplicated id = %q", changes[0].Values[0].Value)
	}
	for _, row := range []struct {
		index int
		raw   string
	}{{1, "\x00\x01\xff"}, {2, "\x00\x01\xff"}, {3, "\x00\x02"}} {
		if got := table.getRawCellValue(row.index, 1); got != row.raw {
			t.Fatalf("raw row %d = %q, want %q", row.index, got, row.raw)
		}
	}
	editBinaryCell(t, table, 2, 1, "0xCAFE")
	if got := (*table.state.listOfDBChanges)[0].Values[1].Value; got != "\xca\xfe" {
		t.Fatalf("edited inserted token = %q", got)
	}
	table.RemoveRow(2)
	if got := table.getRawCellValue(2, 1); got != "\x00\x02" {
		t.Fatalf("raw after removing inserted row = %q", got)
	}
}

func TestBinaryInsertedRowsRenderAsHexAfterRebuild(t *testing.T) {
	table := newBinaryEditingTable(t)
	table.Select(1, 0)
	table.duplicateRow()
	table.state.rawCellValues = nil
	table.Clear()
	table.AddRows(table.state.records)
	table.AddInsertedRows()
	if got := cellText(table.GetCell(2, 1)); got != "0x0001FF" {
		t.Fatalf("rebuilt inserted display = %q", got)
	}
	if got := table.getRawCellValue(2, 1); got != "\x00\x01\xff" {
		t.Fatalf("rebuilt inserted raw = %q", got)
	}
}

func TestBinaryColumnMetadataFormatsPrintableAndWhitespaceBytes(t *testing.T) {
	for _, raw := range []string{"AB", "\t\n\r", ""} {
		t.Run(raw, func(t *testing.T) {
			table := newBinaryEditingTable(t)
			columns := table.state.columns
			table.state.columns = nil
			table.state.records[1][1] = raw
			table.state.rawCellValues = nil
			table.Clear()
			table.AddRows(table.state.records)
			table.SetColumns(columns)
			if got := cellText(table.GetCell(1, 1)); got != helpers.EncodeBinaryDisplayValue(raw) {
				t.Fatalf("metadata display = %q", got)
			}
			editBinaryCell(t, table, 1, 1, "0xCAFE")
			if got := (*table.state.listOfDBChanges)[0].Values[0].Value; got != "\xca\xfe" {
				t.Fatalf("hex edit after printable value = %q", got)
			}
		})
	}
}

func TestBinaryHexIsDecodedOnlyOnce(t *testing.T) {
	table := newBinaryEditingTable(t)
	editBinaryCell(t, table, 1, 1, "0x30783431")
	if got := table.getRawCellValue(1, 1); got != "0x41" {
		t.Fatalf("hex should decode to literal bytes 0x41, got %q", got)
	}
	if got := (*table.state.listOfDBChanges)[0].Values[0].Value; got != "0x41" {
		t.Fatalf("stored value = %q", got)
	}
}

func TestBinaryExternalEditorDecodesHex(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("external cell editor is supported on Unix")
	}
	table := newBinaryEditingTable(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	App.SetScreen(screen)
	path := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(path, []byte("printf '0xCAFE\\n' > \"$1\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", "sh "+path)
	table.Select(1, 1)
	table.tableInputCapture(tcell.NewEventKey(tcell.KeyRune, 'e', tcell.ModNone))
	changes := *table.state.listOfDBChanges
	if len(changes) != 1 || changes[0].Values[0].Value != "\xca\xfe" {
		t.Fatalf("external editor change = %+v", changes)
	}
	if got := table.getRawCellValue(1, 1); got != "\xca\xfe" {
		t.Fatalf("external editor raw = %q", got)
	}
}

func TestBinarySidebarReversionPublishesOriginalHex(t *testing.T) {
	table := newBinaryEditingTable(t)
	table.Select(1, 1)
	sidebar := NewSidebar(drivers.DriverMySQL, false)
	sidebar.AddField("token", "0x0001FF", 40, false)
	sidebar.RawCellValue = func(col int) string { return table.getRawCellValue(1, col+1) }
	events := make(chan models.StateChange, 8)
	sidebar.subscribers = []chan models.StateChange{events}
	for _, text := range []string{"plain text", "0x0001FF"} {
		sidebar.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 'c', tcell.ModNone))
		item := sidebar.Flex.GetItem(0).(*tview.TextArea)
		item.SetText(text, true)
		sidebar.GetInputCapture()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
		<-events
		event := <-events
		if event.Key != eventSidebarCommitEditing {
			t.Fatalf("expected commit for %q, got %+v", text, event)
		}
		params := event.Value.(models.SidebarEditingCommitParams)
		if err := table.AppendNewChange(models.DMLUpdateType, 1, 1, models.CellValue{Type: params.Type, Value: params.NewValue, Column: "token", TableRowIndex: 1, TableColumnIndex: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if len(*table.state.listOfDBChanges) != 0 {
		t.Fatal("sidebar original hex should revert the change")
	}
	if got := sidebar.currentFieldValue(); got != "\x00\x01\xff" {
		t.Fatalf("sidebar copy value = %q", got)
	}
}

func TestBinaryCopyUsesCurrentRawSelection(t *testing.T) {
	table := newBinaryEditingTable(t)
	table.Select(1, 1)
	if got := table.copySelectionValue(); got != "\x00\x01\xff" {
		t.Fatalf("single cell copy = %q", got)
	}
	editBinaryCell(t, table, 1, 1, "0xCAFE")
	if got := table.copySelectionValue(); got != "\xca\xfe" {
		t.Fatalf("edited cell copy = %q", got)
	}
	table.toggleRowMark(1)
	if got := table.copySelectionValue(); got != binaryID+"\t\xca\xfe\tline one\nline two" {
		t.Fatalf("marked row copy = %q", got)
	}
}

func TestBinaryMetadataKeepsNullAndEmptySentinels(t *testing.T) {
	table := newBinaryEditingTable(t)
	columns := table.state.columns
	table.state.columns = nil
	table.state.records = [][]string{{"id", "token", "note"}, {binaryID, "NULL&", "null"}, {"second", "EMPTY&", "empty"}}
	table.Clear()
	table.AddRows(table.state.records)
	table.SetColumns(columns)
	for _, row := range []struct {
		index int
		want  string
	}{{1, "NULL"}, {2, "EMPTY"}} {
		if got := cellText(table.GetCell(row.index, 1)); got != row.want {
			t.Fatalf("special display = %q, want %q", got, row.want)
		}
	}
}

func TestRejectedBinaryEditLeavesRawAndDisplayUnchanged(t *testing.T) {
	table := newBinaryEditingTable(t)
	table.state.records = nil
	before := table.GetCell(1, 1).Text
	err := table.AppendNewChange(models.DMLUpdateType, 1, 1, models.CellValue{Type: models.String, Value: "0xCAFE", Column: "token", TableRowIndex: 1, TableColumnIndex: 1})
	if err == nil {
		t.Fatal("expected missing primary key error")
	}
	if table.GetCell(1, 1).Text != before || table.getRawCellValue(1, 1) != "\x00\x01\xff" {
		t.Fatal("rejected edit changed display or cached raw bytes")
	}
}
