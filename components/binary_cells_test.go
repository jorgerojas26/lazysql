package components

import (
	"testing"

	"github.com/jorgerojas26/lazysql/drivers"
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
	if got := table.getRawCellValue(1, 1); got != "\xde\xad\xbe\xef" {
		t.Fatalf("getRawCellValue after a hex edit = %q, want the decoded bytes", got)
	}

	if got := table.editedCellValue(1, 1, "plain text"); got != "plain text" {
		t.Fatalf("editedCellValue(text) = %q, want it unchanged", got)
	}
	if _, ok := table.state.rawCellValues[table.foreignKeyCellMapKey(1, 1)]; ok {
		t.Fatal("a text edit should drop the cell's binary value")
	}

	// Hex typed into a text column is just text.
	if got := table.editedCellValue(1, 2, "0x41"); got != "0x41" {
		t.Fatalf("editedCellValue on a text cell = %q, want %q", got, "0x41")
	}
}

func TestBinaryCellEditKeepsRawPrimaryKey(t *testing.T) {
	table := newBinaryCellsTable()

	value := table.editedCellValue(1, 1, "0xCAFE")
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
	original := table.editedCellValue(1, 1, "0x0001FF")
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
