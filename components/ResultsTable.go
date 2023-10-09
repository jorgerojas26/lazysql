package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.design/x/clipboard"

	"lazysql/app"
	"lazysql/drivers"
)

type ResultsTableState struct {
	dbReference string
	currentSort string
	error       string
	records     [][]string
	columns     [][]string
	constraints [][]string
	foreignKeys [][]string
	indexes     [][]string
	isEditing   bool
	isLoading   bool
}

type ResultsTable struct {
	*tview.Table
	state      *ResultsTableState
	Page       *tview.Pages
	Wrapper    *tview.Flex
	Menu       *ResultsTableMenu
	Filter     *ResultsTableFilter
	Error      *tview.Modal
	Loading    *tview.Modal
	Pagination *Pagination
}

var ErrorModal = tview.NewModal()

func NewResultsTable() *ResultsTable {
	state := &ResultsTableState{
		records:     [][]string{},
		columns:     [][]string{},
		constraints: [][]string{},
		foreignKeys: [][]string{},
		indexes:     [][]string{},
		isEditing:   false,
		isLoading:   false,
	}

	menu := NewResultsTableMenu()

	filter := NewResultsFilter()

	wrapper := tview.NewFlex()
	wrapper.SetDirection(tview.FlexColumnCSS)
	wrapper.AddItem(menu.Flex, 3, 0, false)
	wrapper.AddItem(filter.Flex, 3, 0, false)

	errorModal := tview.NewModal()
	errorModal.AddButtons([]string{"Ok"})
	errorModal.SetText("An error occurred")
	errorModal.SetBackgroundColor(tcell.ColorRed)
	errorModal.SetTextColor(tcell.ColorBlack)
	errorModal.SetFocus(0)

	loadingModal := tview.NewModal()
	loadingModal.SetText("Loading...")
	loadingModal.SetBackgroundColor(tcell.ColorBlack)
	loadingModal.SetTextColor(tcell.ColorWhite.TrueColor())

	pages := tview.NewPages()
	pages.AddPage("table", wrapper, true, true)
	pages.AddPage("error", errorModal, true, false)
	pages.AddPage("loading", loadingModal, false, false)

	pagination := NewPagination()

	table := &ResultsTable{
		Table:      tview.NewTable(),
		state:      state,
		Menu:       menu,
		Filter:     filter,
		Page:       pages,
		Wrapper:    wrapper,
		Error:      errorModal,
		Loading:    loadingModal,
		Pagination: pagination,
	}

	table.SetSelectable(true, true)
	table.SetBorders(true)
	// table.SetBorder(true)
	table.SetFixed(1, 0)
	table.SetInputCapture(table.tableInputCapture)

	go table.subscribeToFilterChanges()

	wrapper.AddItem(table, 0, 1, true)
	wrapper.AddItem(pagination, 3, 0, false)

	return table
}

func (table *ResultsTable) AddRows(rows [][]string) {
	for i, row := range rows {
		for j, cell := range row {
			tableCell := tview.NewTableCell(cell)
			tableCell.SetSelectable(i > 0)
			tableCell.SetExpansion(1)

			if i == 0 {
				tableCell.SetTextColor(app.ActiveTextColor)
			} else {
				tableCell.SetTextColor(app.FocusTextColor)
			}

			table.SetCell(i, j, tableCell)
		}
	}
}

func (table *ResultsTable) tableInputCapture(event *tcell.EventKey) *tcell.EventKey {
	selectedRowIndex, selectedColumnIndex := table.GetSelection()
	colCount := table.GetColumnCount()
	rowCount := table.GetRowCount()

	if event.Rune() == 49 || event.Rune() == 50 || event.Rune() == 51 || event.Rune() == 52 || event.Rune() == 53 {
		table.Select(1, 0)
	}

	if event.Rune() == 49 { // 1 Key
		table.Menu.SetSelectedOption(1)
		table.UpdateRows(table.GetRecords())
		table.Select(1, 0)
	} else if event.Rune() == 50 { // 2 Key
		table.Menu.SetSelectedOption(2)
		table.UpdateRows(table.GetColumns())
		table.Select(1, 0)
	} else if event.Rune() == 51 { // 3 Key
		table.Menu.SetSelectedOption(3)
		table.UpdateRows(table.GetConstraints())
		table.Select(1, 0)
	} else if event.Rune() == 52 { // 4 Key
		table.Menu.SetSelectedOption(4)
		table.UpdateRows(table.GetForeignKeys())
		table.Select(1, 0)
	} else if event.Rune() == 53 { // 5 Key
		table.Menu.SetSelectedOption(5)
		table.UpdateRows(table.GetIndexes())
		table.Select(1, 0)
	} else if event.Rune() == 47 { // / Key
		app.App.SetFocus(table.Filter.Input)
		table.RemoveHighlightTable()
		table.Filter.HighlightLocal()
		table.Filter.SetIsFiltering(true)

		if table.Filter.Input.GetText() == "/" {
			go table.Filter.Input.SetText("")
		}

		table.SetInputCapture(nil)
	} else if event.Rune() == 99 { // c Key
		table.SetIsEditing(true)
		go func() {
			table.SetInputCapture(nil)
			cell := table.GetCell(selectedRowIndex, selectedColumnIndex)
			inputField := tview.NewInputField()
			inputField.SetText(cell.Text)
			inputField.SetFieldBackgroundColor(app.ActiveTextColor)
			inputField.SetFieldTextColor(tcell.ColorBlack)
			oldBgColor := cell.BackgroundColor
			oldTextColor := cell.Color
			selectedRowId := table.GetCell(selectedRowIndex, 0).Text

			selectedColumnName := table.GetColumnNameByIndex(selectedColumnIndex)

			inputField.SetDoneFunc(func(key tcell.Key) {
				table.SetIsEditing(false)
				currentValue := cell.Text
				newValue := inputField.GetText()
				if key == tcell.KeyEnter {
					if currentValue != newValue {
						cell.SetBackgroundColor(tcell.ColorYellow)
						cell.SetTextColor(tcell.ColorBlack)

						err := drivers.MySQL.UpdateRecord(table.GetDBReference(), selectedColumnName, newValue, selectedRowId)
						if err != nil {
							panic(err)
						}
						cell.SetBackgroundColor(oldBgColor)
						cell.SetTextColor(oldTextColor)

						cell.SetText(inputField.GetText())
					}
				}
				table.SetInputCapture(table.tableInputCapture)
				table.Page.RemovePage("edit")
				app.App.SetFocus(table)
			})

			x, y, width := cell.GetLastPosition()
			inputField.SetRect(x, y, width+1, 1)
			table.Page.AddPage("edit", inputField, false, true)
			app.App.SetFocus(inputField)
			app.App.Draw()
		}()
	} else if event.Rune() == 119 { // w key
		if selectedColumnIndex+1 < colCount {
			table.Select(selectedRowIndex, selectedColumnIndex+1)
		}
	} else if event.Rune() == 98 { // b key
		if selectedColumnIndex > 0 {
			table.Select(selectedRowIndex, selectedColumnIndex-1)
		}
	} else if event.Rune() == 36 { // $ Key
		table.Select(selectedRowIndex, colCount-1)
	} else if event.Rune() == 48 { // 0 Key
		table.Select(selectedRowIndex, 0)
	} else if event.Rune() == 103 { // g Key
		go table.Select(1, selectedColumnIndex)
	} else if event.Rune() == 71 { // G Key
		go table.Select(rowCount-1, selectedColumnIndex)
	} else if event.Rune() == 100 { // d Key
		if selectedRowIndex+7 > rowCount-1 {
			go table.Select(rowCount-1, selectedColumnIndex)
		} else {
			go table.Select(selectedRowIndex+7, selectedColumnIndex)
		}
	} else if event.Rune() == 117 { // u Key
		if selectedRowIndex-7 < 1 {
			go table.Select(1, selectedColumnIndex)
		} else {
			go table.Select(selectedRowIndex-7, selectedColumnIndex)
		}
	} else if event.Rune() == 74 { // J Key
		currentColumnName := table.GetColumnNameByIndex(selectedColumnIndex)
		table.Pagination.SetOffset(0)
		table.SetSortedBy(currentColumnName, "DESC")

	} else if event.Rune() == 75 {
		currentColumnName := table.GetColumnNameByIndex(selectedColumnIndex)
		table.Pagination.SetOffset(0)
		table.SetSortedBy(currentColumnName, "ASC")
	} else if event.Rune() == 121 { // y Key
		selectedCell := table.Table.GetCell(selectedRowIndex, selectedColumnIndex)

		if selectedCell != nil {
			err := clipboard.Init()

			if err == nil {
				text := []byte(selectedCell.Text)

				if text != nil {
					clipboard.Write(clipboard.FmtText, text)
				}
			}
		}
	}

	return event
}

func (table *ResultsTable) UpdateRows(rows [][]string) {
	table.Clear()
	table.AddRows(rows)
	table.Select(0, 0)
	app.App.ForceDraw()
}

func (table *ResultsTable) UpdateRowsColor(headerColor tcell.Color, rowColor tcell.Color) {
	for i := 0; i < table.GetRowCount(); i++ {
		for j := 0; j < table.GetColumnCount(); j++ {
			cell := table.GetCell(i, j)
			if i == 0 && headerColor != 0 {
				cell.SetTextColor(headerColor)
			} else {
				cell.SetTextColor(rowColor)
			}
		}
	}
}

func (table *ResultsTable) RemoveHighlightTable() {
	table.SetBorderColor(app.BlurTextColor)
	table.SetBordersColor(app.BlurTextColor)
	table.SetTitleColor(app.BlurTextColor)
	table.UpdateRowsColor(app.BlurTextColor, app.BlurTextColor)
}

func (table *ResultsTable) RemoveHighlightAll() {
	table.RemoveHighlightTable()
	table.Menu.SetBlur()
	table.Filter.RemoveHighlight()
}

func (table *ResultsTable) HighlightTable() {
	table.SetBorderColor(app.FocusTextColor)
	table.SetBordersColor(app.FocusTextColor)
	table.SetTitleColor(app.FocusTextColor)
	table.UpdateRowsColor(app.ActiveTextColor, app.FocusTextColor)
}

func (table *ResultsTable) HighlightAll() {
	table.HighlightTable()
	table.Menu.SetFocus()
	table.Filter.Highlight()
}

func (table *ResultsTable) subscribeToFilterChanges() {
	ch := table.Filter.Subscribe()

	for stateChange := range ch {
		switch stateChange.Key {
		case "Filter":
			if stateChange.Value != "" {
				rows := table.FetchRecords(table.GetDBReference())

				if len(rows) > 1 {
					table.Menu.SetSelectedOption(1)
					app.App.SetFocus(table)
					table.HighlightTable()
					table.Filter.HighlightLocal()
					table.SetInputCapture(table.tableInputCapture)
					app.App.ForceDraw()
				}

			} else {
				table.FetchRecords(table.GetDBReference())

				table.SetInputCapture(table.tableInputCapture)
				app.App.SetFocus(table)
				table.HighlightTable()
				table.Filter.HighlightLocal()
				app.App.ForceDraw()

			}
		}
	}
}

// Getters

func (table *ResultsTable) GetRecords() [][]string {
	return table.state.records
}

func (table *ResultsTable) GetIndexes() [][]string {
	return table.state.indexes
}

func (table *ResultsTable) GetColumns() [][]string {
	return table.state.columns
}

func (table *ResultsTable) GetConstraints() [][]string {
	return table.state.constraints
}

func (table *ResultsTable) GetForeignKeys() [][]string {
	return table.state.foreignKeys
}

func (table *ResultsTable) GetDBReference() string {
	return table.state.dbReference
}

func (table *ResultsTable) GetIsEditing() bool {
	return table.state.isEditing
}

func (table *ResultsTable) GetCurrentSort() string {
	return table.state.currentSort
}

func (table *ResultsTable) GetColumnNameByIndex(index int) string {
	columns := table.GetColumns()

	for i, col := range columns {
		if i > 0 && i == index+1 {
			return col[0]
		}
	}

	return ""
}

func (table *ResultsTable) GetIsLoading() bool {
	return table.state.isLoading
}

// Setters

func (table *ResultsTable) SetRecords(rows [][]string) {
	table.state.records = rows
	table.UpdateRows(rows)
}

func (table *ResultsTable) SetColumns(columns [][]string) {
	table.state.columns = columns
}

func (table *ResultsTable) SetConstraints(constraints [][]string) {
	table.state.constraints = constraints
}

func (table *ResultsTable) SetForeignKeys(foreignKeys [][]string) {
	table.state.foreignKeys = foreignKeys
}

func (table *ResultsTable) SetIndexes(indexes [][]string) {
	table.state.indexes = indexes
}

func (table *ResultsTable) SetDBReference(dbReference string) {
	table.state.dbReference = dbReference
}

func (table *ResultsTable) SetError(err string, done func()) {
	table.state.error = err

	table.Error.SetText(err)
	table.Error.SetDoneFunc(func(_ int, _ string) {
		table.state.error = ""
		table.Page.HidePage("error")
		if table.Filter.GetIsFiltering() {
			app.App.SetFocus(table.Filter.Input)
		} else {
			app.App.SetFocus(table)
		}
		if done != nil {
			done()
		}
	})
	table.Page.ShowPage("error")
	app.App.SetFocus(table.Error)
	table.Error.SetFocus(0)
	app.App.ForceDraw()
}

func (table *ResultsTable) SetLoading(show bool) {
	table.state.isLoading = show
	if show {
		table.Page.ShowPage("loading")
		app.App.SetFocus(table.Loading)
		app.App.ForceDraw()
	} else {
		table.Page.HidePage("loading")
		if table.state.error != "" {
			app.App.SetFocus(table.Error)
		} else {
			app.App.SetFocus(table)
		}
		app.App.ForceDraw()
	}
}

func (table *ResultsTable) SetIsEditing(editing bool) {
	table.state.isEditing = editing
}

func (table *ResultsTable) SetCurrentSort(sort string) {
	table.state.currentSort = sort
}

func (table *ResultsTable) SetSortedBy(column string, direction string) {
	sort := fmt.Sprintf("%s %s", column, direction)

	if table.GetCurrentSort() != sort {
		where := table.Filter.GetCurrentFilter()
		table.SetLoading(true)
		records, err := drivers.MySQL.GetRecords(table.GetDBReference(), where, sort, table.Pagination.GetOffset(), table.Pagination.GetLimit(), true)
		table.SetLoading(false)

		if err != nil {
			table.SetError(err.Error(), nil)
		} else {
			table.SetRecords(records)
			app.App.ForceDraw()
		}

		table.SetCurrentSort(sort)

		columns := table.GetColumns()
		iconDirection := "▲"

		if direction == "DESC" {
			iconDirection = "▼"
		}

		for i, col := range columns {
			if i > 0 {
				tableCell := tview.NewTableCell(col[0])
				tableCell.SetSelectable(false)
				tableCell.SetExpansion(1)
				tableCell.SetTextColor(app.ActiveTextColor)

				if col[0] == column {
					tableCell.SetText(fmt.Sprintf("%s %s", col[0], iconDirection))
					table.SetCell(0, i-1, tableCell)
				} else {
					table.SetCell(0, i-1, tableCell)
				}
			}
		}
	}
}

func (table *ResultsTable) FetchRecords(tableName string) [][]string {
	table.SetLoading(true)

	where := table.Filter.GetCurrentFilter()
	sort := table.GetCurrentSort()

	records, totalRecords, err := drivers.MySQL.GetPaginatedRecords(tableName, where, sort, table.Pagination.GetOffset(), table.Pagination.GetLimit(), true)

	if err != nil {
		table.SetError(err.Error(), nil)
	} else {
		if table.Filter.GetIsFiltering() {
			table.Filter.SetIsFiltering(false)
		}
	}

	columns := drivers.MySQL.DescribeTable(tableName)
	constraints := drivers.MySQL.GetTableConstraints(tableName)
	foreignKeys := drivers.MySQL.GetTableForeignKeys(tableName)
	indexes := drivers.MySQL.GetTableIndexes(tableName)

	table.SetRecords(records)
	table.SetColumns(columns)
	table.SetConstraints(constraints)
	table.SetForeignKeys(foreignKeys)
	table.SetIndexes(indexes)
	table.SetDBReference(tableName)
	table.Select(1, 0)

	table.Pagination.SetTotalRecords(totalRecords)

	table.SetLoading(false)

	return records
}
