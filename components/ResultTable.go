package components

import (
	"lazysql/drivers"
	"lazysql/utils"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func RenderColumns(tableName string, table *tview.Table) {
	columns := drivers.Database.DescribeTable(tableName)
	table.Clear()
	utils.AddTableRows(table, columns)
}

func RenderConstraints(tableName string, table *tview.Table) {
	constraints := drivers.Database.GetTableConstraints(tableName)
	table.Clear()
	utils.AddTableRows(table, constraints)
}

func RenderForeignKeys(tableName string, table *tview.Table) {
	foreignKeys := drivers.Database.GetTableForeignKeys(tableName)
	table.Clear()
	utils.AddTableRows(table, foreignKeys)
}

func RenderIndexes(tableName string, table *tview.Table) {
	indexes := drivers.Database.GetTableIndexes(tableName)
	table.Clear()
	utils.AddTableRows(table, indexes)
}

func AddVimKeyBindings(table *tview.Table, tableTree *tview.Flex, App *tview.Application, dbTablePages *tview.Pages) {

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		selectedRow, selectedColumn := table.GetSelection()
		rowCount := table.GetRowCount()

		if event.Rune() == 99 { // c Key
			cell := table.GetCell(selectedRow, selectedColumn)
			inputField := tview.NewInputField()
			inputField.SetText(cell.Text)
			inputField.SetDoneFunc(func(key tcell.Key) {
				cell.SetText(inputField.GetText())
				dbTablePages.RemovePage("edit")
				App.SetFocus(table)
			})
			x, y, width := cell.GetLastPosition()
			inputField.SetRect(x, y, width+1, 1)
			dbTablePages.AddPage("edit", inputField, false, true)
			App.SetFocus(inputField)

		} else if event.Rune() == 36 { // $ Key
			colCount := table.GetColumnCount()
			table.Select(selectedRow, colCount-1)
		} else if event.Rune() == 48 { // 0 Key
			table.Select(selectedRow, 0)
		} else if event.Rune() == 103 { // g Key
			go table.Select(1, selectedColumn)
		} else if event.Rune() == 71 { // G Key
			go table.Select(rowCount-1, selectedColumn)
		} else if event.Rune() == 100 { // d Key
			if selectedRow+7 > rowCount-1 {
				go table.Select(rowCount-1, selectedColumn)
			} else {
				go table.Select(selectedRow+7, selectedColumn)
			}
		} else if event.Rune() == 117 { // u Key
			if selectedRow-7 < 1 {
				go table.Select(1, selectedColumn)
			} else {
				go table.Select(selectedRow-7, selectedColumn)
			}
		} else if event.Rune() == 72 { // H Key
			App.SetFocus(tableTree)
		}

		return event
	})
}
