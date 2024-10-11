package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/lib"
	"github.com/jorgerojas26/lazysql/models"
)

type ResultsTableState struct {
	listOfDbChanges *[]models.DbDmlChange
	error           string
	currentSort     string
	databaseName    string
	tableName       string
	records         [][]string
	columns         [][]string
	constraints     [][]string
	foreignKeys     [][]string
	indexes         [][]string
	isEditing       bool
	isFiltering     bool
	isLoading       bool
}

type ResultsTable struct {
	*tview.Table
	state       *ResultsTableState
	Page        *tview.Pages
	Wrapper     *tview.Flex
	Menu        *ResultsTableMenu
	Filter      *ResultsTableFilter
	Error       *tview.Modal
	Loading     *tview.Modal
	Pagination  *Pagination
	Editor      *SQLEditor
	EditorPages *tview.Pages
	ResultsInfo *tview.TextView
	Tree        *Tree
	DBDriver    drivers.Driver
}

var (
	ErrorModal  = tview.NewModal()
	ChangeColor = tcell.ColorDarkOrange
	InsertColor = tcell.ColorDarkGreen
	DeleteColor = tcell.ColorRed
)

func NewResultsTable(listOfDbChanges *[]models.DbDmlChange, tree *Tree, dbdriver drivers.Driver) *ResultsTable {
	state := &ResultsTableState{
		records:         [][]string{},
		columns:         [][]string{},
		constraints:     [][]string{},
		foreignKeys:     [][]string{},
		indexes:         [][]string{},
		isEditing:       false,
		isLoading:       false,
		listOfDbChanges: listOfDbChanges,
	}

	wrapper := tview.NewFlex()
	wrapper.SetDirection(tview.FlexColumnCSS)

	errorModal := tview.NewModal()
	errorModal.AddButtons([]string{"Ok"})
	errorModal.SetText("An error occurred")
	errorModal.SetBackgroundColor(tcell.ColorRed)
	errorModal.SetTextColor(tview.Styles.PrimaryTextColor)
	errorModal.SetButtonStyle(tcell.StyleDefault.Foreground(tview.Styles.PrimaryTextColor))
	errorModal.SetFocus(0)

	loadingModal := tview.NewModal()
	loadingModal.SetText("Loading...")
	loadingModal.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	loadingModal.SetTextColor(tview.Styles.SecondaryTextColor)

	pages := tview.NewPages()
	pages.AddPage("table", wrapper, true, true)
	pages.AddPage("error", errorModal, true, false)
	pages.AddPage("loading", loadingModal, false, false)

	pagination := NewPagination()

	table := &ResultsTable{
		Table:      tview.NewTable(),
		state:      state,
		Page:       pages,
		Wrapper:    wrapper,
		Error:      errorModal,
		Loading:    loadingModal,
		Pagination: pagination,
		Editor:     nil,
		Tree:       tree,
		DBDriver:   dbdriver,
	}

	table.SetSelectable(true, true)
	table.SetBorders(true)
	table.SetFixed(1, 0)
	table.SetInputCapture(table.tableInputCapture)
	table.SetSelectedStyle(tcell.StyleDefault.Background(tview.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))

	go table.subscribeToTreeChanges()

	return table
}

func (table *ResultsTable) WithFilter() *ResultsTable {
	menu := NewResultsTableMenu()
	filter := NewResultsFilter()

	table.Menu = menu
	table.Filter = filter

	table.Wrapper.AddItem(menu.Flex, 3, 0, false)
	table.Wrapper.AddItem(filter.Flex, 3, 0, false)
	table.Wrapper.AddItem(table, 0, 1, true)
	table.Wrapper.AddItem(table.Pagination, 3, 0, false)

	go table.subscribeToFilterChanges()

	return table
}

func (table *ResultsTable) WithEditor() *ResultsTable {
	editor := NewSQLEditor()
	editorPages := tview.NewPages()

	editor.SetFocusFunc(func() {
		table.SetIsEditing(true)
	})

	editor.SetBlurFunc(func() {
		table.SetIsEditing(false)
	})

	table.Editor = editor

	table.Wrapper.Clear()

	table.Wrapper.AddItem(editor, 12, 0, true)
	table.SetBorder(true)

	tableWrapper := tview.NewFlex().SetDirection(tview.FlexColumnCSS)
	tableWrapper.AddItem(table, 0, 1, false)
	tableWrapper.AddItem(table.Pagination, 3, 0, false)

	resultsInfoWrapper := tview.NewFlex().SetDirection(tview.FlexColumnCSS)
	resultsInfoText := tview.NewTextView()
	resultsInfoText.SetBorder(true)
	resultsInfoText.SetBorderColor(tview.Styles.PrimaryTextColor)
	resultsInfoText.SetTextColor(tview.Styles.PrimaryTextColor)
	resultsInfoWrapper.AddItem(resultsInfoText, 3, 0, false)

	editorPages.AddPage("Table", tableWrapper, true, false)
	editorPages.AddPage("ResultsInfo", resultsInfoWrapper, true, true)

	table.EditorPages = editorPages
	table.ResultsInfo = resultsInfoText

	table.Wrapper.AddItem(editorPages, 0, 1, true)

	go table.subscribeToEditorChanges()

	return table
}

func (table *ResultsTable) subscribeToTreeChanges() {
	ch := table.Tree.Subscribe()

	for stateChange := range ch {
		if stateChange.Key == "SelectedDatabase" {
			table.SetDatabaseName(stateChange.Value.(string))
		}
	}
}

func (table *ResultsTable) AddRows(rows [][]string) {
	for i, row := range rows {
		for j, cell := range row {
			tableCell := tview.NewTableCell(cell)
			tableCell.SetTextColor(tview.Styles.PrimaryTextColor)

			if cell == "EMPTY&" || cell == "NULL&" {
				tableCell.SetText(strings.Replace(cell, "&", "", 1))
				tableCell.SetStyle(table.GetItalicStyle())
				tableCell.SetReference(cell)
			}

			tableCell.SetSelectable(i > 0)
			tableCell.SetExpansion(1)

			table.SetCell(i, j, tableCell)
		}
	}
}

func (table *ResultsTable) AddInsertedRows() {
	inserts := make([]models.DbDmlChange, 0)

	for _, change := range *table.state.listOfDbChanges {
		if change.Type == models.DmlInsertType {
			inserts = append(inserts, change)
		}
	}

	rows := make([][]models.CellValue, len(inserts))

	if len(inserts) > 0 {
		for i, insert := range inserts {
			if insert.Table == table.GetTableName() {
				rows[i] = insert.Values
			}
		}
	}

	rowCount := table.GetRowCount()
	for i, row := range rows {
		rowIndex := rowCount + i

		for j, cell := range row {
			tableCell := tview.NewTableCell(cell.Value.(string))
			tableCell.SetExpansion(1)
			tableCell.SetReference(inserts[i].PrimaryKeyValue)

			tableCell.SetTextColor(tview.Styles.PrimaryTextColor)
			tableCell.SetBackgroundColor(InsertColor)

			table.SetCell(rowIndex, j, tableCell)
		}
	}
}

func (table *ResultsTable) AppendNewRow(cells []models.CellValue, index int, UUID string) {
	for i, cell := range cells {
		tableCell := tview.NewTableCell(cell.Value.(string))
		tableCell.SetExpansion(1)
		tableCell.SetReference(UUID)
		tableCell.SetTextColor(tview.Styles.PrimaryTextColor)
		tableCell.SetBackgroundColor(InsertColor)

		switch cell.Type {
		case models.Null:
		case models.Default:
		case models.String:
			tableCell.SetText("")
			tableCell.SetTextColor(tview.Styles.InverseTextColor)
		}

		table.SetCell(index, i, tableCell)
	}

	table.Select(index, 0)
	App.ForceDraw()
}

func (table *ResultsTable) tableInputCapture(event *tcell.EventKey) *tcell.EventKey {
	selectedRowIndex, selectedColumnIndex := table.GetSelection()
	colCount := table.GetColumnCount()
	rowCount := table.GetRowCount()

	eventKey := event.Rune()

	command := app.Keymaps.Group(app.TableGroup).Resolve(event)

	menuCommands := []commands.Command{commands.RecordsMenu, commands.ColumnsMenu, commands.ConstraintsMenu, commands.ForeignKeysMenu, commands.IndexesMenu}

	if helpers.ContainsCommand(menuCommands, command) {
		table.Select(1, 0)
	}

	if table.Menu != nil {
		switch command {
		case commands.RecordsMenu:
			table.Menu.SetSelectedOption(1)
			table.UpdateRows(table.GetRecords())
			table.AddInsertedRows()
		case commands.ColumnsMenu:
			table.Menu.SetSelectedOption(2)
			table.UpdateRows(table.GetColumns())
		case commands.ConstraintsMenu:
			table.Menu.SetSelectedOption(3)
			table.UpdateRows(table.GetConstraints())
		case commands.ForeignKeysMenu:
			table.Menu.SetSelectedOption(4)
			table.UpdateRows(table.GetForeignKeys())
		case commands.IndexesMenu:
			table.Menu.SetSelectedOption(5)
			table.UpdateRows(table.GetIndexes())
		}
	}

	switch command {
	case commands.AppendNewRow:
		if table.Menu.GetSelectedOption() == 1 {
			table.appendNewRow()
		}
	case commands.Search:
		table.search()
	}

	if rowCount == 1 || colCount == 0 {
		return nil
	}

	if command == commands.Edit {
		table.StartEditingCell(selectedRowIndex, selectedColumnIndex, nil)
	} else if command == commands.GotoNext {
		if selectedColumnIndex+1 < colCount {
			table.Select(selectedRowIndex, selectedColumnIndex+1)
		}
	} else if command == commands.GotoPrev {
		if selectedColumnIndex > 0 {
			table.Select(selectedRowIndex, selectedColumnIndex-1)
		}
	} else if command == commands.GotoEnd {
		table.Select(selectedRowIndex, colCount-1)
	} else if command == commands.GotoStart {
		table.Select(selectedRowIndex, 0)
	} else if command == commands.GotoBottom {
		go table.Select(1, selectedColumnIndex)
	} else if command == commands.GotoTop {
		go table.Select(rowCount-1, selectedColumnIndex)
	} else if eventKey == 4 { // Ctrl + D
		if selectedRowIndex+7 > rowCount-1 {
			go table.Select(rowCount-1, selectedColumnIndex)
		} else {
			go table.Select(selectedRowIndex+7, selectedColumnIndex)
		}
	} else if eventKey == 21 { // Ctrl + U
		if selectedRowIndex-7 < 1 {
			go table.Select(1, selectedColumnIndex)
		} else {
			go table.Select(selectedRowIndex-7, selectedColumnIndex)
		}
	} else if command == commands.Delete {
		if table.Menu.GetSelectedOption() == 1 {
			isAnInsertedRow := false
			indexOfInsertedRow := -1

			for i, insertedRow := range *table.state.listOfDbChanges {
				cellReference := table.GetCell(selectedRowIndex, 0).GetReference()

				if cellReference != nil && insertedRow.PrimaryKeyValue == cellReference {
					isAnInsertedRow = true
					indexOfInsertedRow = i
				}
			}

			if isAnInsertedRow {
				*table.state.listOfDbChanges = append((*table.state.listOfDbChanges)[:indexOfInsertedRow], (*table.state.listOfDbChanges)[indexOfInsertedRow+1:]...)
				table.RemoveRow(selectedRowIndex)
				if selectedRowIndex-1 != 0 {
					table.Select(selectedRowIndex-1, 0)
				} else {
					if selectedRowIndex+1 < rowCount {
						table.Select(selectedRowIndex+1, 0)
					}
				}
			} else {
				table.AppendNewChange(models.DmlDeleteType, table.GetDatabaseName(), table.GetTableName(), selectedRowIndex, -1, models.CellValue{})
			}

		}
	} else if command == commands.SetValue {
		table.SetIsEditing(true)
		table.SetInputCapture(nil)

		cell := table.GetCell(selectedRowIndex, selectedColumnIndex)
		x, y, width := cell.GetLastPosition()

		list := NewSetValueList()
		list.SetRect(x, y, width, 7)

		list.OnFinish(func(selection models.CellValueType, value string) {
			table.FinishSettingValue()

			if selection >= 0 {
				table.AppendNewChange(models.DmlUpdateType, table.Tree.GetSelectedDatabase(), table.Tree.GetSelectedTable(), selectedRowIndex, selectedColumnIndex, models.CellValue{Type: selection, Value: value, Column: table.GetColumnNameByIndex(selectedColumnIndex)})
			}
		})

		list.Show(x, y, width)
	}

	if len(table.GetRecords()) > 0 {
		switch command {
		case commands.SortDesc:
			currentColumnName := table.GetColumnNameByIndex(selectedColumnIndex)
			table.Pagination.SetOffset(0)
			table.SetSortedBy(currentColumnName, "DESC")
		case commands.SortAsc:
			currentColumnName := table.GetColumnNameByIndex(selectedColumnIndex)
			table.Pagination.SetOffset(0)
			table.SetSortedBy(currentColumnName, "ASC")
		case commands.Copy:
			selectedCell := table.GetCell(selectedRowIndex, selectedColumnIndex)

			if selectedCell != nil {

				clipboard := lib.NewClipboard()

				err := clipboard.Write(selectedCell.Text)
				if err != nil {
					table.SetError(err.Error(), nil)
				}
			}
		}
	}

	return event
}

func (table *ResultsTable) UpdateRows(rows [][]string) {
	table.Clear()
	table.AddRows(rows)
	App.ForceDraw()
	table.Select(1, 0)
}

func (table *ResultsTable) UpdateRowsColor(headerColor tcell.Color, rowColor tcell.Color) {
	for i := 0; i < table.GetRowCount(); i++ {
		for j := 0; j < table.GetColumnCount(); j++ {
			cell := table.GetCell(i, j)
			if i == 0 && headerColor != 0 {
				cell.SetTextColor(headerColor)
			} else {
				cellReference := cell.GetReference()

				if cellReference != nil && cellReference == "EMPTY&" || cellReference == "NULL&" || cellReference == "DEFAULT&" {
					cell.SetStyle(table.GetItalicStyle())
				} else {
					cell.SetTextColor(rowColor)
				}
			}
		}
	}
}

func (table *ResultsTable) RemoveHighlightTable() {
	table.SetBorderColor(tview.Styles.InverseTextColor)
	table.SetBordersColor(tview.Styles.InverseTextColor)
	table.SetTitleColor(tview.Styles.InverseTextColor)
	table.UpdateRowsColor(tview.Styles.InverseTextColor, tview.Styles.InverseTextColor)
}

func (table *ResultsTable) RemoveHighlightAll() {
	table.RemoveHighlightTable()
	if table.Menu != nil {
		table.Menu.SetBlur()
	}
	if table.Filter != nil {
		table.Filter.RemoveHighlight()
	}
}

func (table *ResultsTable) HighlightTable() {
	table.SetBorderColor(tview.Styles.PrimaryTextColor)
	table.SetBordersColor(tview.Styles.PrimaryTextColor)
	table.SetTitleColor(tview.Styles.PrimaryTextColor)
	table.UpdateRowsColor(tview.Styles.PrimaryTextColor, tview.Styles.PrimaryTextColor)
}

func (table *ResultsTable) HighlightAll() {
	table.HighlightTable()
	if table.Menu != nil {
		table.Menu.SetFocus()
	}
	if table.Filter != nil {
		table.Filter.Highlight()
	}
}

func (table *ResultsTable) subscribeToFilterChanges() {
	ch := table.Filter.Subscribe()

	for stateChange := range ch {
		switch stateChange.Key {
		case "Filter":
			if stateChange.Value != "" {
				rows := table.FetchRecords(nil)

				if len(rows) > 1 {
					table.Menu.SetSelectedOption(1)
					App.SetFocus(table)
					table.HighlightTable()
					table.Filter.HighlightLocal()
					table.SetInputCapture(table.tableInputCapture)
					App.ForceDraw()
				} else if len(rows) == 1 {
					table.SetInputCapture(nil)
					App.SetFocus(table.Filter.Input)
					table.RemoveHighlightTable()
					table.Filter.HighlightLocal()
					table.SetIsFiltering(true)
					App.ForceDraw()
				}

			} else {
				table.FetchRecords(nil)

				table.SetInputCapture(table.tableInputCapture)
				App.SetFocus(table)
				table.HighlightTable()
				table.Filter.HighlightLocal()
				App.ForceDraw()

			}
		}
	}
}

func (table *ResultsTable) subscribeToEditorChanges() {
	ch := table.Editor.Subscribe()

	for stateChange := range ch {
		switch stateChange.Key {
		case "Query":
			query := stateChange.Value.(string)
			if query != "" {
				queryLower := strings.ToLower(query)

				if strings.Contains(queryLower, "select") {
					table.SetLoading(true)
					App.Draw()

					rows, err := table.DBDriver.ExecuteQuery(query)
					table.Pagination.SetTotalRecords(len(rows))
					table.Pagination.SetLimit(len(rows))

					if err != nil {
						table.SetLoading(false)
						App.Draw()
						table.SetError(err.Error(), nil)
					} else {
						table.UpdateRows(rows)
						table.SetIsFiltering(false)

						if len(rows) > 1 {
							App.SetFocus(table)
							table.HighlightTable()
							table.Editor.SetBlur()
							table.SetInputCapture(table.tableInputCapture)
							App.Draw()
						} else if len(rows) == 1 {
							table.SetInputCapture(nil)
							App.SetFocus(table.Editor)
							table.Editor.Highlight()
							table.RemoveHighlightTable()
							table.SetIsFiltering(true)
							App.Draw()
						}
						table.SetLoading(false)
					}
					table.EditorPages.SwitchToPage("Table")
					App.Draw()
				} else {
					table.SetRecords([][]string{})
					table.SetLoading(true)
					App.Draw()

					result, err := table.DBDriver.ExecuteDMLStatement(query)

					if err != nil {
						table.SetLoading(false)
						App.Draw()
						table.SetError(err.Error(), nil)
					} else {
						table.SetResultsInfo(result)
						table.SetLoading(false)
						table.EditorPages.SwitchToPage("ResultsInfo")
						App.SetFocus(table.Editor)
						App.Draw()
					}
				}
			}
		case "Escape":
			table.SetIsFiltering(false)
			App.SetFocus(table)
			table.HighlightTable()
			table.Editor.SetBlur()
			table.SetInputCapture(table.tableInputCapture)
			App.Draw()
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

func (table *ResultsTable) GetTableName() string {
	return table.state.tableName
}

func (table *ResultsTable) GetDatabaseName() string {
	return table.state.databaseName
}

func (table *ResultsTable) GetDatabaseAndTableName() string {
	return fmt.Sprintf("%s.%s", table.GetDatabaseName(), table.GetTableName())
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

func (table *ResultsTable) GetIsFiltering() bool {
	return table.state.isFiltering
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

func (table *ResultsTable) SetDatabaseName(databaseName string) {
	table.state.databaseName = databaseName
}

func (table *ResultsTable) SetTableName(tableName string) {
	table.state.tableName = tableName
}

func (table *ResultsTable) SetError(err string, done func()) {
	table.state.error = err

	table.Error.SetText(err)
	table.Error.SetDoneFunc(func(_ int, _ string) {
		table.state.error = ""
		table.Page.HidePage("error")
		if table.GetIsFiltering() {
			if table.Editor != nil {
				App.SetFocus(table.Editor)
			} else {
				App.SetFocus(table.Filter.Input)
			}
		} else {
			App.SetFocus(table)
		}
		if done != nil {
			done()
		}
	})
	table.Page.ShowPage("error")
	App.SetFocus(table.Error)
	App.ForceDraw()
}

func (table *ResultsTable) SetResultsInfo(text string) {
	table.ResultsInfo.SetText(text)
}

func (table *ResultsTable) SetLoading(show bool) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("ResultsTable.go:800 => Recovered from panic", map[string]any{"error": r})
			_ = table.Page.HidePage("loading")
			if table.state.error != "" {
				App.SetFocus(table.Error)
			} else {
				App.SetFocus(table)
			}
		}
	}()

	table.state.isLoading = show
	if show {
		table.Page.ShowPage("loading")
		App.SetFocus(table.Loading)
		App.ForceDraw()
	} else {
		table.Page.HidePage("loading")
		if table.state.error != "" {
			App.SetFocus(table.Error)
		} else {
			App.SetFocus(table)
		}
		App.ForceDraw()
	}
}

func (table *ResultsTable) SetIsEditing(editing bool) {
	table.state.isEditing = editing
}

func (table *ResultsTable) SetIsFiltering(filtering bool) {
	table.state.isFiltering = filtering
}

func (table *ResultsTable) SetCurrentSort(sort string) {
	table.state.currentSort = sort
}

func (table *ResultsTable) SetSortedBy(column string, direction string) {
	sort := fmt.Sprintf("%s %s", column, direction)

	if table.GetCurrentSort() != sort {
		where := ""
		if table.Filter != nil {
			where = table.Filter.GetCurrentFilter()
		}
		table.SetLoading(true)
		records, _, err := table.DBDriver.GetRecords(table.GetDatabaseName(), table.GetTableName(), where, sort, table.Pagination.GetOffset(), table.Pagination.GetLimit())
		table.SetLoading(false)

		if err != nil {
			table.SetError(err.Error(), nil)
		} else {
			table.SetRecords(records)
			App.ForceDraw()
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
				tableCell.SetTextColor(tview.Styles.PrimaryTextColor)

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

func (table *ResultsTable) FetchRecords(onError func()) [][]string {
	tableName := table.GetTableName()
	databaseName := table.GetDatabaseName()

	table.SetLoading(true)

	where := ""
	if table.Filter != nil {
		where = table.Filter.GetCurrentFilter()
	}
	sort := table.GetCurrentSort()

	records, totalRecords, err := table.DBDriver.GetRecords(databaseName, tableName, where, sort, table.Pagination.GetOffset(), table.Pagination.GetLimit())

	if err != nil {
		table.SetError(err.Error(), onError)
		table.SetLoading(false)
	} else {
		if table.GetIsFiltering() {
			table.SetIsFiltering(false)
		}

		columns, _ := table.DBDriver.GetTableColumns(databaseName, tableName)
		constraints, _ := table.DBDriver.GetConstraints(databaseName, tableName)
		foreignKeys, _ := table.DBDriver.GetForeignKeys(databaseName, tableName)
		indexes, _ := table.DBDriver.GetIndexes(databaseName, tableName)

		if len(records) > 0 {
			table.SetRecords(records)
		}

		table.SetColumns(columns)
		table.SetConstraints(constraints)
		table.SetForeignKeys(foreignKeys)
		table.SetIndexes(indexes)
		table.Select(1, 0)

		table.Pagination.SetTotalRecords(totalRecords)

		table.SetLoading(false)

		return records
	}

	return [][]string{}
}

func (table *ResultsTable) StartEditingCell(row int, col int, callback func(newValue string, row, col int)) {
	table.SetIsEditing(true)
	table.SetInputCapture(nil)

	cell := table.GetCell(row, col)
	inputField := tview.NewInputField()
	inputField.SetText(cell.Text)
	inputField.SetFieldBackgroundColor(tview.Styles.PrimaryTextColor)
	inputField.SetFieldTextColor(tview.Styles.PrimitiveBackgroundColor)

	inputField.SetDoneFunc(func(key tcell.Key) {
		table.SetIsEditing(false)
		currentValue := cell.Text
		newValue := inputField.GetText()
		columnName := table.GetCell(0, col).Text

		if key != tcell.KeyEscape {
			cell.SetText(newValue)

			if currentValue != newValue {
				table.AppendNewChange(models.DmlUpdateType, table.GetDatabaseName(), table.GetTableName(), row, col, models.CellValue{Type: models.String, Value: newValue, Column: columnName})
			}

			switch key {
			case tcell.KeyTab:
				nextEditableColumnIndex := col + 1

				if nextEditableColumnIndex <= table.GetColumnCount()-1 {
					table.Select(row, nextEditableColumnIndex)

					table.StartEditingCell(row, nextEditableColumnIndex, callback)

				}
			case tcell.KeyBacktab:
				nextEditableColumnIndex := col - 1

				if nextEditableColumnIndex >= 0 {
					table.Select(row, nextEditableColumnIndex)

					table.StartEditingCell(row, nextEditableColumnIndex, callback)

				}
			}

		}

		if key == tcell.KeyEnter || key == tcell.KeyEscape {
			table.SetInputCapture(table.tableInputCapture)
			table.Page.RemovePage("edit")
			App.SetFocus(table)
		}

		if callback != nil {
			callback(newValue, row, col)
		}
	})

	x, y, width := cell.GetLastPosition()
	inputField.SetRect(x, y, width+1, 1)
	table.Page.AddPage("edit", inputField, false, true)
	App.SetFocus(inputField)
}

func (table *ResultsTable) CheckIfRowIsInserted(rowID string) bool {
	for _, dmlChange := range *table.state.listOfDbChanges {
		if dmlChange.Type == models.DmlInsertType && dmlChange.PrimaryKeyValue == rowID {
			return true
		}
	}

	return false
}

func (table *ResultsTable) MutateInsertedRowCell(rowID string, newValue models.CellValue) {
	for i, dmlChange := range *table.state.listOfDbChanges {
		if dmlChange.PrimaryKeyValue == rowID && dmlChange.Type == models.DmlInsertType {
			for j, v := range dmlChange.Values {
				if v.Column == newValue.Column {
					(*table.state.listOfDbChanges)[i].Values[j] = newValue
					break
				}
			}
		}
	}
}

func (table *ResultsTable) AppendNewChange(changeType models.DmlType, databaseName, tableName string, rowIndex int, colIndex int, value models.CellValue) {
	dmlChangeAlreadyExists := false

	// If the column has a reference, it means it's an inserted rowIndex
	// These is maybe a better way to detect it is an inserted row
	tableCell := table.GetCell(rowIndex, colIndex)
	tableCellReference := tableCell.GetReference()

	isAnInsertedRow := tableCellReference != nil && tableCellReference.(string) != "NULL&" && tableCellReference.(string) != "EMPTY&" && tableCellReference.(string) != "DEFAULT&"

	if isAnInsertedRow {
		table.MutateInsertedRowCell(tableCellReference.(string), value)
		return
	}

	primaryKeyValue, primaryKeyColumnName := table.GetPrimaryKeyValue(rowIndex)

	if changeType == models.DmlUpdateType {
		switch value.Type {
		case models.Null, models.Empty, models.Default:
			tableCell.SetText(value.Value.(string))
			tableCell.SetStyle(tcell.StyleDefault.Italic(true))
			tableCell.SetReference(value.Value.(string) + "&")
		}
	}

	for i, dmlChange := range *table.state.listOfDbChanges {
		if dmlChange.Table == tableName && dmlChange.Type == changeType && dmlChange.PrimaryKeyValue == primaryKeyValue {
			dmlChangeAlreadyExists = true

			changeForColExists := false
			valueIndex := -1

			for j, v := range dmlChange.Values {
				if v.Column == value.Column {
					changeForColExists = true
					valueIndex = j
					break
				}
			}

			switch changeType {
			case models.DmlUpdateType:
				originalValue := table.GetRecords()[rowIndex][colIndex]

				if changeForColExists {
					if originalValue == value.Value {
						if len((*table.state.listOfDbChanges)[i].Values) == 1 {
							*table.state.listOfDbChanges = append((*table.state.listOfDbChanges)[:i], (*table.state.listOfDbChanges)[i+1:]...)
						} else {
							(*table.state.listOfDbChanges)[i].Values = append((*table.state.listOfDbChanges)[i].Values[:valueIndex], (*table.state.listOfDbChanges)[i].Values[valueIndex+1:]...)
						}
						table.SetCellColor(rowIndex, colIndex, tview.Styles.PrimitiveBackgroundColor)
					} else {
						(*table.state.listOfDbChanges)[i].Values[valueIndex] = value
					}
				} else {
					(*table.state.listOfDbChanges)[i].Values = append((*table.state.listOfDbChanges)[i].Values, value)
					table.SetCellColor(rowIndex, colIndex, ChangeColor)
				}

			case models.DmlDeleteType:
				*table.state.listOfDbChanges = append((*table.state.listOfDbChanges)[:i], (*table.state.listOfDbChanges)[i+1:]...)
				table.SetRowColor(rowIndex, tview.Styles.PrimitiveBackgroundColor)
			}
		}
	}

	if !dmlChangeAlreadyExists {

		switch changeType {
		case models.DmlDeleteType:
			table.SetRowColor(rowIndex, DeleteColor)
		case models.DmlUpdateType:
			tableCell.SetStyle(tcell.StyleDefault.Background(ChangeColor))
			table.SetCellColor(rowIndex, colIndex, ChangeColor)
		}

		newDmlChange := models.DbDmlChange{
			Type:                 changeType,
			Database:             databaseName,
			Table:                tableName,
			Values:               []models.CellValue{value},
			PrimaryKeyColumnName: primaryKeyColumnName,
			PrimaryKeyValue:      primaryKeyValue,
		}

		*table.state.listOfDbChanges = append(*table.state.listOfDbChanges, newDmlChange)

	}
}

func (table *ResultsTable) GetPrimaryKeyValue(rowIndex int) (string, string) {
	provider := table.DBDriver.GetProvider()
	columns := table.GetColumns()
	constraints := table.GetConstraints()

	primaryKeyColumnName := ""
	primaryKeyValue := ""

	switch provider {
	case "mysql":
		keyColumnIndex := -1
		primaryKeyColumnIndex := -1

		for i, col := range columns[0] {
			if col == "Key" {
				keyColumnIndex = i
			}
		}

		for i, col := range columns {
			if col[keyColumnIndex] == "PRI" {
				primaryKeyColumnIndex = i - 1
				primaryKeyColumnName = col[0]
			}
		}

		if primaryKeyColumnIndex != -1 {
			primaryKeyValue = table.GetRecords()[rowIndex][primaryKeyColumnIndex]
		}

	case "postgres":
		keyColumnIndex := -1
		constraintTypeColumnIndex := -1
		constraintNameColumnIndex := -1
		pKeyName := ""
		primaryKeyColumnIndex := -1

		for i, constraint := range constraints[0] {
			if constraint == "constraint_type" {
				constraintTypeColumnIndex = i
			}
			if constraint == "column_name" {
				constraintNameColumnIndex = i
			}
		}

		for _, col := range constraints {
			if col[constraintTypeColumnIndex] == "PRIMARY KEY" {
				pKeyName = col[constraintNameColumnIndex]
				break
			}
		}

		primaryKeyColumnName = pKeyName
		for i, col := range columns[0] {
			if col == "column_name" {
				keyColumnIndex = i
				break
			}
		}

		for i, col := range columns {
			if col[keyColumnIndex] == pKeyName {
				primaryKeyColumnIndex = i - 1
				break
			}
		}

		if primaryKeyColumnIndex != -1 {
			primaryKeyValue = table.GetRecords()[rowIndex][primaryKeyColumnIndex]
		}

	case "sqlite3":
		keyColumnIndex := -1
		primaryKeyColumnIndex := -1

		for i, col := range columns[0] {
			if col == "pk" {
				keyColumnIndex = i
			}
		}

		for i, col := range columns {
			if col[keyColumnIndex] == "1" {
				primaryKeyColumnIndex = i - 1
				primaryKeyColumnName = col[0]
			}
		}

		if primaryKeyColumnIndex != -1 {
			primaryKeyValue = table.GetRecords()[rowIndex][primaryKeyColumnIndex]
		}
	}

	return primaryKeyValue, primaryKeyColumnName
}

func (table *ResultsTable) SetRowColor(rowIndex int, color tcell.Color) {
	for i := 0; i < table.GetColumnCount(); i++ {
		table.GetCell(rowIndex, i).SetBackgroundColor(color)
	}
}

func (table *ResultsTable) SetCellColor(rowIndex int, colIndex int, color tcell.Color) {
	table.GetCell(rowIndex, colIndex).SetBackgroundColor(color)
}

func (table *ResultsTable) appendNewRow() {
	dbColumns := table.GetColumns()
	newRowTableIndex := table.GetRowCount()
	newRowUUID := uuid.New().String()
	newRow := make([]models.CellValue, len(dbColumns)-1)

	for i, column := range dbColumns {
		if i != 0 { // Skip the first row because they are the column names (e.x "Field", "Type", "Null", "Key", "Default", "Extra")
			newRow[i-1] = models.CellValue{Type: models.Default, Column: column[0], Value: "DEFAULT"}
		}
	}

	newInsert := models.DbDmlChange{
		Type:                 models.DmlInsertType,
		Database:             table.GetDatabaseName(),
		Table:                table.GetTableName(),
		Values:               newRow,
		PrimaryKeyColumnName: "",
		PrimaryKeyValue:      newRowUUID,
	}

	*table.state.listOfDbChanges = append(*table.state.listOfDbChanges, newInsert)

	table.AppendNewRow(newRow, newRowTableIndex, newRowUUID)

	table.StartEditingCell(newRowTableIndex, 0, nil)
}

func (table *ResultsTable) search() {
	if table.Editor != nil {
		App.SetFocus(table.Editor)
		table.Editor.Highlight()
		table.RemoveHighlightTable()
		table.SetIsFiltering(true)
		return
	}

	App.SetFocus(table.Filter.Input)
	table.RemoveHighlightTable()
	table.Filter.HighlightLocal()
	table.SetIsFiltering(true)

	if table.Filter.Input.GetText() == "/" {
		go table.Filter.Input.SetText("")
	}

	table.Filter.Input.SetAutocompleteFunc(func(currentText string) []string {
		split := strings.Split(currentText, " ")
		comparators := []string{"=", "!=", ">", "<", ">=", "<=", "LIKE", "NOT LIKE", "IN", "NOT IN", "IS", "IS NOT", "BETWEEN", "NOT BETWEEN"}

		if len(split) == 1 {
			columns := table.GetColumns()
			columnNames := []string{}

			for i, col := range columns {
				if i > 0 {
					columnNames = append(columnNames, col[0])
				}
			}

			return columnNames
		} else if len(split) == 2 {

			for i, comparator := range comparators {
				comparators[i] = fmt.Sprintf("%s %s", split[0], strings.ToLower(comparator))
			}

			return comparators
		} else if len(split) == 3 {

			ret := true

			switch split[1] {
			case "not":
				comparators = []string{"between", "in", "like"}
			case "is":
				comparators = []string{"not", "null"}
			default:
				ret = false
			}

			if ret {
				for i, comparator := range comparators {
					comparators[i] = fmt.Sprintf("%s %s %s", split[0], split[1], strings.ToLower(comparator))
				}
				return comparators
			}

		} else if len(split) == 4 {
			ret := true

			switch split[2] {
			case "not":
				comparators = []string{"null"}
			case "is":
				comparators = []string{"not", "null"}
			default:
				ret = false
			}

			if ret {
				for i, comparator := range comparators {
					comparators[i] = fmt.Sprintf("%s %s %s %s", split[0], split[1], split[2], strings.ToLower(comparator))
				}

				return comparators
			}
		}

		return []string{}
	})

	table.Filter.Input.SetAutocompletedFunc(func(text string, _ int, source int) bool {
		if source != tview.AutocompletedNavigate {
			inputText := strings.Split(table.Filter.Input.GetText(), " ")

			if len(inputText) == 1 {
				table.Filter.Input.SetText(fmt.Sprintf("%s =", text))
			} else if len(inputText) == 2 {
				table.Filter.Input.SetText(fmt.Sprintf("%s %s", inputText[0], text))
			}

			table.Filter.Input.SetText(text)
		}
		return source == tview.AutocompletedEnter || source == tview.AutocompletedClick
	})

	table.SetInputCapture(nil)
}

func (table *ResultsTable) FinishSettingValue() {
	table.SetIsEditing(false)
	table.SetInputCapture(table.tableInputCapture)
	App.SetFocus(table)
}

func (table *ResultsTable) GetItalicStyle() tcell.Style {
	return tcell.StyleDefault.Foreground(tview.Styles.InverseTextColor).Italic(true)
}
