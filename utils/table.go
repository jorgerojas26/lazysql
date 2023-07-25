package utils

import "github.com/rivo/tview"

func AddTableRows(table *tview.Table, data [][]string) {
	rowCount := table.GetRowCount()

	for x, row := range data {
		for y, column := range row {
			cell := tview.NewTableCell(column)
			cell.SetReference(column)
			cell.SetSelectable(x > 0)
			cell.SetExpansion(1)

			table.SetCell(x+rowCount, y, cell)
		}
	}
}
