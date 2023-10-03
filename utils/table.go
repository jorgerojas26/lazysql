package utils

import (
	"github.com/rivo/tview"
)

func AddTableRows(table *tview.Table, data [][]string) {
	rowCount := table.GetRowCount()

	columnaNames := data[0]

	for x, row := range data {
		for y, columnValue := range row {
			cell := tview.NewTableCell(columnValue)
			columnName := columnaNames[y]
			cell.SetReference(columnName)
			cell.SetSelectable(x > 0)
			cell.SetExpansion(1)

			table.SetCell(x+rowCount, y, cell)
		}
	}
}
