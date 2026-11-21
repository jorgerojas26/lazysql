package components

import (
	"strings"
	"testing"
)

func newSortedTestTable(direction string) *ResultsTable {
	rows := [][]string{{"id", "name"}, {"1", "alice"}, {"2", "bob"}}
	table := newMarkTestTable(rows)
	table.state.records = rows
	table.updateSortHeader("name", direction)
	return table
}

func TestSortHeaderShowsIndicatorButKeepsRawName(t *testing.T) {
	for direction, icon := range map[string]string{"ASC": "▲", "DESC": "▼"} {
		table := newSortedTestTable(direction)

		if got := table.GetCell(0, 1).Text; got != "name "+icon {
			t.Fatalf("%s: header text = %q, want indicator shown", direction, got)
		}
		if got := table.columnHeaderName(1); got != "name" {
			t.Fatalf("%s: columnHeaderName(1) = %q, want %q", direction, got, "name")
		}
		if got := table.columnHeaderName(0); got != "id" {
			t.Fatalf("%s: columnHeaderName(0) = %q, want %q", direction, got, "id")
		}
	}
}

func TestColumnHeaderNameWithoutSort(t *testing.T) {
	table := newMarkTestTable([][]string{{"id", "name"}, {"1", "alice"}})

	if got := table.columnHeaderName(1); got != "name" {
		t.Fatalf("columnHeaderName(1) = %q, want %q", got, "name")
	}
	if got := table.columnHeaderName(5); got != "" {
		t.Fatalf("columnHeaderName(out of range) = %q, want empty", got)
	}
}

func TestCopyRowsIgnoresSortIndicator(t *testing.T) {
	table := newSortedTestTable("DESC")

	for _, format := range []string{"CSV", "Markdown", "JSON", "SQL"} {
		out, err := table.formatCopyRows(1, format)
		if err != nil {
			t.Fatalf("%s: %v", format, err)
		}
		if strings.ContainsAny(out, "▲▼") {
			t.Fatalf("%s output contains sort indicator:\n%s", format, out)
		}
	}

	out, _ := table.formatCopyRows(1, "JSON")
	if !strings.Contains(out, `"name": "alice"`) {
		t.Fatalf("JSON should key by raw column name:\n%s", out)
	}
}
