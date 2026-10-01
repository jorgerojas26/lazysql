package components

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/lib"
)

const pageNameCopyRowsAs = "CopyRowsAs"

// copyRowIndexes prefers the live visual range, then marked rows, then the cursor.
func (table *ResultsTable) copyRowIndexes(current int) []int {
	var indexes []int
	if table.state.rangeAnchor != 0 {
		start, end := table.state.rangeAnchor, table.state.rangeEnd
		if start > end {
			start, end = end, start
		}
		for row := start; row <= end; row++ {
			indexes = append(indexes, row)
		}
	} else if len(table.state.markedRows) > 0 {
		indexes = table.GetMarkedRowIndexes()
	} else {
		indexes = []int{current}
	}
	valid := make([]int, 0, len(indexes))
	for _, row := range indexes {
		if row > 0 && row < table.GetRowCount() {
			valid = append(valid, row)
		}
	}
	return valid
}

func (table *ResultsTable) showCopyRowsAs(current int) {
	if table.Page == nil || len(table.copyRowIndexes(current)) == 0 {
		return
	}
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(" Copy rows as ")
	list.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	list.SetBorderColor(app.Styles.PrimaryTextColor)
	closePicker := func() {
		table.Page.RemovePage(pageNameCopyRowsAs)
		App.SetFocus(table.Table)
	}
	for _, option := range []struct {
		name string
		key  rune
	}{
		{"SQL", 's'}, {"Markdown", 'm'}, {"JSON", 'j'}, {"CSV", 'c'},
	} {
		format := option.name
		list.AddItem(option.name, "", option.key, func() {
			text, err := table.formatCopyRows(current, format)
			closePicker()
			if err == nil {
				err = lib.NewClipboard().Write(text)
			}
			if err != nil {
				table.SetError(err.Error(), nil)
			} else {
				table.cancelRowRange()
			}
		})
	}
	list.SetDoneFunc(closePicker)
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			closePicker()
			return nil
		}
		return event
	})
	modal := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(list, 34, 0, true).
			AddItem(nil, 0, 1, false), 6, 0, true).
		AddItem(nil, 0, 1, false)
	table.Page.AddPage(pageNameCopyRowsAs, modal, true, true)
	App.SetFocus(list)
}

// formatCopyRows formats visible result values, not DB-native typed values.
// Only the database's NULL placeholder is treated specially; all other values
// remain strings so values like "001" are never silently converted to numbers.
func (table *ResultsTable) formatCopyRows(current int, format string) (string, error) {
	indexes := table.copyRowIndexes(current)
	if len(indexes) == 0 {
		return "", fmt.Errorf("no rows to copy")
	}
	columns := make([]string, table.GetColumnCount())
	for col := range columns {
		columns[col] = table.getRawCellValue(0, col)
	}
	values := make([][]string, 0, len(indexes))
	for _, row := range indexes {
		cells := make([]string, len(columns))
		for col := range columns {
			cells[col] = table.getRawCellValue(row, col)
		}
		values = append(values, cells)
	}

	switch format {
	case "CSV":
		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)
		if err := writer.Write(columns); err != nil {
			return "", err
		}
		if err := writer.WriteAll(values); err != nil {
			return "", err
		}
		return buf.String(), nil
	case "Markdown":
		lines := []string{markdownCopyRow(columns)}
		separators := make([]string, len(columns))
		for col := range separators {
			separators[col] = "---"
		}
		lines = append(lines, markdownCopyRow(separators))
		for _, row := range values {
			lines = append(lines, markdownCopyRow(row))
		}
		return strings.Join(lines, "\n"), nil
	case "JSON":
		// Build each object in column order (maps do not preserve column order).
		var buf bytes.Buffer
		buf.WriteString("[\n")
		for i, row := range values {
			if i > 0 {
				buf.WriteString(",\n")
			}
			buf.WriteString("  {")
			for col, value := range row {
				if col > 0 {
					buf.WriteByte(',')
				}
				key, _ := json.Marshal(columns[col])
				buf.WriteString("\n    ")
				buf.Write(key)
				buf.WriteString(": ")
				if table.isCopyNull(indexes[i], col) {
					buf.WriteString("null")
				} else {
					encoded, _ := json.Marshal(value)
					buf.Write(encoded)
				}
			}
			buf.WriteString("\n  }")
		}
		buf.WriteString("\n]")
		return buf.String(), nil
	case "SQL":
		quoted := make([]string, len(columns))
		for col, name := range columns {
			quoted[col] = table.copySQLIdentifier(name)
		}
		rows := make([]string, len(values))
		for i, row := range values {
			literals := make([]string, len(row))
			for col, value := range row {
				if table.isCopyNull(indexes[i], col) {
					literals[col] = "NULL"
				} else {
					literals[col] = "'" + strings.ReplaceAll(value, "'", "''") + "'"
				}
			}
			rows[i] = "(" + strings.Join(literals, ", ") + ")"
		}
		if table.Menu != nil && table.Menu.GetSelectedOption() == 1 && table.GetTableName() != "" {
			parts := strings.Split(table.GetTableName(), ".")
			for i := range parts {
				parts[i] = table.copySQLIdentifier(parts[i])
			}
			return fmt.Sprintf("INSERT INTO %s (%s) VALUES\n%s;", strings.Join(parts, "."), strings.Join(quoted, ", "), strings.Join(rows, ",\n")), nil
		}
		// Ad-hoc editor queries and metadata views have no safe INSERT target.
		return "VALUES\n" + strings.Join(rows, ",\n") + ";", nil
	default:
		return "", fmt.Errorf("unknown copy format: %s", format)
	}
}

func (table *ResultsTable) isCopyNull(row, col int) bool {
	cell := table.GetCell(row, col)
	return cell != nil && cell.GetReference() == "NULL&"
}

func (table *ResultsTable) copySQLIdentifier(name string) string {
	if table.DBDriver != nil {
		switch table.DBDriver.FormatReference("x") {
		case "`x`":
			return "`" + strings.ReplaceAll(name, "`", "``") + "`"
		case "[x]":
			return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
		}
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func markdownCopyRow(cells []string) string {
	values := make([]string, len(cells))
	for i, cell := range cells {
		cell = strings.ReplaceAll(cell, `\`, `\\`)
		cell = strings.ReplaceAll(cell, "|", `\|`)
		cell = strings.ReplaceAll(cell, "\r\n", "<br>")
		cell = strings.ReplaceAll(cell, "\n", "<br>")
		values[i] = cell
	}
	return "| " + strings.Join(values, " | ") + " |"
}
