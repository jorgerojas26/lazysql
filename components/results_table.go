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
	"github.com/jorgerojas26/lazysql/internal/history"
	"github.com/jorgerojas26/lazysql/lib"
	"github.com/jorgerojas26/lazysql/models"
)

type ResultsTableState struct {
	listOfDBChanges       *[]models.DBDMLChange
	error                 string
	currentSort           string
	databaseName          string
	tableName             string
	primaryKeyColumnNames []string
	columns               [][]string
	constraints           [][]string
	foreignKeys           [][]string
	indexes               [][]string
	records               [][]string
	isEditing             bool
	isFiltering           bool
	isLoading             bool
	showSidebar           bool
}

type ResultsTable struct {
	*tview.Table
	state                *ResultsTableState
	Page                 *tview.Pages
	Wrapper              *tview.Flex
	Menu                 *ResultsTableMenu
	Filter               *ResultsTableFilter
	Error                *tview.Modal
	Loading              *tview.Modal
	Pagination           *Pagination
	Editor               *SQLEditor
	EditorPages          *tview.Pages
	ResultsInfo          *tview.TextView
	Tree                 *Tree
	Sidebar              *Sidebar
	SidebarContainer     *tview.Flex
	DBDriver             drivers.Driver
	connectionIdentifier string
}

func NewResultsTable(listOfDBChanges *[]models.DBDMLChange, tree *Tree, dbdriver drivers.Driver, connectionIdentifier string) *ResultsTable {
	state := &ResultsTableState{
		records:         [][]string{},
		columns:         [][]string{},
		constraints:     [][]string{},
		foreignKeys:     [][]string{},
		indexes:         [][]string{},
		isEditing:       false,
		isLoading:       false,
		listOfDBChanges: listOfDBChanges,
		showSidebar:     false,
	}

	wrapper := tview.NewFlex()
	wrapper.SetDirection(tview.FlexColumnCSS)

	errorModal := tview.NewModal()
	errorModal.AddButtons([]string{"Ok"})
	errorModal.SetText("An error occurred")
	errorModal.SetBackgroundColor(tcell.ColorRed)
	errorModal.SetTextColor(app.Styles.PrimaryTextColor)
	errorModal.SetButtonStyle(tcell.StyleDefault.Foreground(app.Styles.PrimaryTextColor))
	errorModal.SetFocus(0)

	loadingModal := tview.NewModal()
	loadingModal.SetText("Loading...")
	loadingModal.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	loadingModal.SetBorderStyle(tcell.StyleDefault.Background(app.Styles.PrimitiveBackgroundColor))
	loadingModal.SetTextColor(app.Styles.SecondaryTextColor)

	pages := tview.NewPages()
	pages.AddPage(pageNameTable, wrapper, true, true)
	pages.AddPage(pageNameTableError, errorModal, true, false)
	pages.AddPage(pageNameTableLoading, loadingModal, false, false)

	pagination := NewPagination()

	sidebar := NewSidebar(dbdriver.GetProvider())

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
		Sidebar:    sidebar,
		// SidebarContainer is only used when AppConfig.SidebarOverlay is false.
		SidebarContainer:     tview.NewFlex(),
		connectionIdentifier: connectionIdentifier,
	}

	// When AppConfig.SidebarOverlay is true, the sidebar is added as a page to the table.Page.
	// When AppConfig.SidebarOverlay is false, the sidebar is added to the table.SidebarContainer.
	table.Page.AddPage(pageNameSidebar, table.Sidebar, false, false)

	table.SetSelectable(true, true)
	table.SetBorders(true)
	table.SetFixed(1, 0)
	table.SetInputCapture(table.tableInputCapture)
	table.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))

	table.SetSelectionChangedFunc(func(_, _ int) {
		if table.GetShowSidebar() {
			go table.UpdateSidebar()
		}
	})

	go table.subscribeToTreeChanges()
	go table.subscribeToSidebarChanges()

	return table
}

func (table *ResultsTable) WithFilter() *ResultsTable {
	menu := NewResultsTableMenu()
	filter := NewResultsFilter()

	table.Menu = menu
	table.Filter = filter

	if App.Config().SidebarOverlay {
		table.Wrapper.AddItem(menu, 3, 0, false)
		table.Wrapper.AddItem(filter, 3, 0, false)
		table.Wrapper.AddItem(table, 0, 1, true)
		table.Wrapper.AddItem(table.Pagination, 3, 0, false)
	} else {
		tableContainer := tview.NewFlex().SetDirection(tview.FlexColumnCSS)
		tableContainer.AddItem(menu, 3, 0, false)
		tableContainer.AddItem(filter, 3, 0, false)
		tableContainer.AddItem(table, 0, 1, true)
		tableContainer.AddItem(table.Pagination, 3, 0, false)
		tableContainer.SetBorder(true)

		table.SidebarContainer.AddItem(tableContainer, 0, 4, true)

		table.Wrapper.AddItem(table.SidebarContainer, 0, 1, true)
	}

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
	resultsInfoText.SetBorderColor(app.Styles.PrimaryTextColor)
	resultsInfoText.SetTextColor(app.Styles.PrimaryTextColor)
	resultsInfoWrapper.AddItem(resultsInfoText, 3, 0, false)

	editorPages.AddPage(pageNameTableEditorTable, tableWrapper, true, false)
	editorPages.AddPage(pageNameTableEditorResultsInfo, resultsInfoWrapper, true, true)

	table.EditorPages = editorPages
	table.ResultsInfo = resultsInfoText

	table.Wrapper.AddItem(editorPages, 0, 1, true)

	go table.subscribeToEditorChanges()

	return table
}

func (table *ResultsTable) subscribeToTreeChanges() {
	ch := table.Tree.Subscribe()

	for stateChange := range ch {
		if stateChange.Key == eventTreeSelectedDatabase {
			table.SetDatabaseName(stateChange.Value.(string))
		}
	}
}

func (table *ResultsTable) subscribeToSidebarChanges() {
	ch := table.Sidebar.Subscribe()

	for stateChange := range ch {
		switch stateChange.Key {
		case eventSidebarEditing:
			editing := stateChange.Value.(bool)
			table.SetIsEditing(editing)
		case eventSidebarUnfocusing:
			App.SetFocus(table)
			App.ForceDraw()
		case eventSidebarToggling:
			table.ShowSidebar(false)
			App.ForceDraw()
		case eventSidebarCommitEditing:
			params := stateChange.Value.(models.SidebarEditingCommitParams)

			table.SetInputCapture(table.tableInputCapture)
			table.SetIsEditing(false)

			row, _ := table.GetSelection()
			changedColumnIndex := table.GetColumnIndexByName(params.ColumnName)
			tableCell := table.GetCell(row, changedColumnIndex)

			tableCell.SetText(params.NewValue)

			cellValue := models.CellValue{
				Type:             params.Type,
				Column:           params.ColumnName,
				Value:            params.NewValue,
				TableColumnIndex: changedColumnIndex,
				TableRowIndex:    row,
			}

			logger.Info("eventSidebarCommitEditing", map[string]any{"cellValue": cellValue, "params": params, "rowIndex": row, "changedColumnIndex": changedColumnIndex})
			table.AppendNewChange(models.DMLUpdateType, row, changedColumnIndex, cellValue)

			App.ForceDraw()
		case eventSidebarError:
			errorMessage := stateChange.Value.(string)
			table.SetError(errorMessage, nil)
		}
	}
}

func (table *ResultsTable) AddRows(rows [][]string) {
	for i, row := range rows {
		for j, cell := range row {
			tableCell := tview.NewTableCell(cell)
			tableCell.SetTextColor(app.Styles.PrimaryTextColor)

			if cell == "EMPTY&" || cell == "NULL&" || cell == "DEFAULT&" {
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
	inserts := make([]models.DBDMLChange, 0)

	for _, change := range *table.state.listOfDBChanges {
		if change.Type == models.DMLInsertType {
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
			tableCell.SetReference(inserts[i].PrimaryKeyInfo[0].Value)

			tableCell.SetTextColor(app.Styles.PrimaryTextColor)
			tableCell.SetBackgroundColor(colorTableInsert)

			table.SetCell(rowIndex, j, tableCell)
		}
	}
}

func (table *ResultsTable) AppendNewRow(cells []models.CellValue, index int, UUID string) {
	for i, cell := range cells {
		tableCell := tview.NewTableCell(cell.Value.(string))
		tableCell.SetExpansion(1)
		// Appended rows have a reference to the row UUID so we can identify them later
		// Also, rows that have columns marked to be UPDATED will have a reference to the type of the new value (NULL, EMPTY, DEFAULT)
		// So, the cell reference will be used to determine if the row/column is an inserted row or if it's an UPDATED row
		// there might be a better way to do this, but it works for now
		tableCell.SetReference(UUID)
		tableCell.SetTextColor(app.Styles.PrimaryTextColor)
		tableCell.SetBackgroundColor(tcell.ColorDarkGreen)

		switch cell.Type {
		case models.Null, models.Empty, models.Default:
			tableCell.SetText(strings.Replace(cell.Value.(string), "&", "", 1))
			tableCell.SetStyle(table.GetItalicStyle())
			// tableCell.SetText("")

			tableCell.SetTextColor(app.Styles.InverseTextColor)
		}

		tableCell.SetBackgroundColor(colorTableInsert)
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

	menuCommands := []commands.Command{commands.RecordsMenu, commands.ColumnsMenu, commands.ConstraintsMenu, commands.ForeignKeysMenu, commands.IndexesMenu, commands.Refresh}

	if helpers.ContainsCommand(menuCommands, command) {
		table.Select(1, 0)
	}

	if table.Menu != nil {
		switch command {
		case commands.RecordsMenu:
			table.Menu.SetSelectedOption(1)
			table.UpdateRows(table.GetRecords())
			table.colorChangedCells()
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
		case commands.Refresh:
			if table.Loading != nil {
				app.App.SetFocus(table.Loading)
			}
			table.Menu.SetSelectedOption(1)
			if err := table.FetchRecords(nil); err != nil {
				return event
			}
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
		table.StartEditingCell(selectedRowIndex, selectedColumnIndex, func(_ string, _, _ int) {
			if table.GetShowSidebar() {
				table.UpdateSidebar()
			}
		})
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

			isAnInsertedRow, indexOfInsertedRow := table.isAnInsertedRow(selectedRowIndex)

			if isAnInsertedRow {
				*table.state.listOfDBChanges = append((*table.state.listOfDBChanges)[:indexOfInsertedRow], (*table.state.listOfDBChanges)[indexOfInsertedRow+1:]...)
				table.RemoveRow(selectedRowIndex)
				if selectedRowIndex-1 != 0 {
					table.Select(selectedRowIndex-1, 0)
				} else {
					if selectedRowIndex+1 < rowCount {
						table.Select(selectedRowIndex+1, 0)
					}
				}
			} else {
				table.AppendNewChange(models.DMLDeleteType, selectedRowIndex, -1, models.CellValue{TableColumnIndex: -1, TableRowIndex: selectedRowIndex, Column: table.GetColumnNameByIndex(selectedColumnIndex)})
			}

		}
	} else if command == commands.SetValue {
		table.SetIsEditing(true)
		table.SetInputCapture(nil)

		cell := table.GetCell(selectedRowIndex, selectedColumnIndex)
		x, y, _ := cell.GetLastPosition()

		list := NewSetValueList(table.DBDriver.GetProvider())

		list.OnFinish(func(selection models.CellValueType, value string) {
			table.FinishSettingValue()

			if selection >= 0 {
				table.AppendNewChange(models.DMLUpdateType, selectedRowIndex, selectedColumnIndex, models.CellValue{Type: selection, Value: value, Column: table.GetColumnNameByIndex(selectedColumnIndex)})
			}
		})

		list.Show(x, y, 30)
	} else if command == commands.ToggleSidebar {
		table.ShowSidebar(!table.GetShowSidebar())
	} else if command == commands.FocusSidebar {
		if table.GetShowSidebar() {
			App.SetFocus(table.Sidebar)
		}
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

				if cellReference != nil && (cellReference == "EMPTY&" || cellReference == "NULL&" || cellReference == "DEFAULT&") && (cell.BackgroundColor != colorTableDelete && cell.BackgroundColor != colorTableChange && cell.BackgroundColor != colorTableInsert) {
					cell.SetStyle(table.GetItalicStyle())
				} else {
					cell.SetTextColor(rowColor)
				}
			}
		}
	}
}

func (table *ResultsTable) RemoveHighlightTable() {
	table.SetBorderColor(app.Styles.InverseTextColor)
	table.SetBordersColor(app.Styles.InverseTextColor)
	table.SetTitleColor(app.Styles.InverseTextColor)
	table.UpdateRowsColor(app.Styles.InverseTextColor, tview.Styles.InverseTextColor)
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
	table.SetBorderColor(app.Styles.PrimaryTextColor)
	table.SetBordersColor(app.Styles.PrimaryTextColor)
	table.SetTitleColor(app.Styles.PrimaryTextColor)
	table.UpdateRowsColor(app.Styles.PrimaryTextColor, tview.Styles.PrimaryTextColor)
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
		case eventResultsTableFiltering:
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
		case eventSQLEditorQuery:
			query := stateChange.Value.(string)
			if query != "" {
				queryLower := strings.ToLower(query)

				if strings.Contains(queryLower, "select") {
					table.SetLoading(true)
					App.Draw()

					rows, records, err := table.DBDriver.ExecuteQuery(query)
					table.Pagination.SetTotalRecords(records)
					table.Pagination.SetLimit(records)

					if err != nil {
						table.SetLoading(false)
						table.SetError(err.Error(), nil)
						App.Draw()
					} else {
						table.UpdateRows(rows)
						table.SetLoading(false)
						table.SetIsFiltering(false)
						table.HighlightTable()
						table.Editor.SetBlur()
						table.SetInputCapture(table.tableInputCapture)
						table.EditorPages.SwitchToPage(pageNameTableEditorTable)
						App.SetFocus(table)
						// Add successful SELECT query to history
						if err := history.AddQueryToHistory(table.connectionIdentifier, query); err != nil {
							logger.Error("Failed to add SELECT query to history", map[string]any{"error": err, "query": query, "connection": table.connectionIdentifier})
						}
						App.Draw()
					}
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
						table.EditorPages.SwitchToPage(pageNameTableEditorResultsInfo)
						App.SetFocus(table.Editor)
						// Add successful DML query to history
						if err := history.AddQueryToHistory(table.connectionIdentifier, query); err != nil {
							logger.Error("Failed to add DML query to history", map[string]any{"error": err, "query": query, "connection": table.connectionIdentifier})
						}
						App.Draw()
					}
				}
			}
		case eventSQLEditorEscape:
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

func (table *ResultsTable) GetColumnIndexByName(columnName string) int {
	cols := table.GetColumns()
	index := -1

	for i, col := range cols {
		if i > 0 && col[0] == columnName {
			index = i - 1 // Because the first column is the column names
			break
		}
	}

	return index
}

func (table *ResultsTable) GetIsLoading() bool {
	return table.state.isLoading
}

func (table *ResultsTable) GetIsFiltering() bool {
	return table.state.isFiltering
}

func (table *ResultsTable) GetShowSidebar() bool {
	return table.state.showSidebar
}

func (table *ResultsTable) GetPrimaryKeyColumnNames() []string {
	return table.state.primaryKeyColumnNames
}

// Setters

func (table *ResultsTable) SetRecords(rows [][]string) {
	table.state.records = rows
	table.UpdateRows(rows)
	table.colorChangedCells()
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
		table.Page.HidePage(pageNameTableError)
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
	table.Page.ShowPage(pageNameTableError)
	App.SetFocus(table.Error)
	App.ForceDraw()
}

func (table *ResultsTable) SetResultsInfo(text string) {
	table.ResultsInfo.SetText(text)
}

func (table *ResultsTable) SetLoading(show bool) {
	table.state.isLoading = show

	if show {
		table.Page.ShowPage(pageNameTableLoading)
		App.SetFocus(table.Loading)
	} else {
		table.Page.HidePage(pageNameTableLoading)
		if table.state.error != "" {
			App.SetFocus(table.Error)
		} else {
			App.SetFocus(table)
		}
	}

	App.ForceDraw()
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
		records, _, _, err := table.DBDriver.GetRecords(table.GetDatabaseName(), table.GetTableName(), where, sort, table.Pagination.GetOffset(), table.Pagination.GetLimit())
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
				tableCell.SetTextColor(app.Styles.PrimaryTextColor)

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

func (table *ResultsTable) SetPrimaryKeyColumnNames(primaryKeyColumnNames []string) {
	table.state.primaryKeyColumnNames = primaryKeyColumnNames
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

	records, totalRecords, executedQuery, err := table.DBDriver.GetRecords(databaseName, tableName, where, sort, table.Pagination.GetOffset(), table.Pagination.GetLimit())

	if err != nil {
		table.SetError(err.Error(), onError)
		table.SetLoading(false)
	} else {
		// Add filter query to history if a filter was applied and a query was executed
		if where != "" && executedQuery != "" {
			if err := history.AddQueryToHistory(table.connectionIdentifier, executedQuery); err != nil {
				logger.Error("Failed to add filter query to history", map[string]any{"error": err, "query": executedQuery, "connection": table.connectionIdentifier})
			}
		}

		if table.GetIsFiltering() {
			table.SetIsFiltering(false)
		}

		columns, err := table.DBDriver.GetTableColumns(databaseName, tableName)
		if err != nil {
			table.SetError(err.Error(), nil)
		}

		constraints, err := table.DBDriver.GetConstraints(databaseName, tableName)
		if err != nil {
			table.SetError(err.Error(), nil)
		}

		foreignKeys, err := table.DBDriver.GetForeignKeys(databaseName, tableName)
		if err != nil {
			table.SetError(err.Error(), nil)
		}

		indexes, err := table.DBDriver.GetIndexes(databaseName, tableName)
		if err != nil {
			table.SetError(err.Error(), nil)
		}

		primaryKeyColumnNames, err := table.DBDriver.GetPrimaryKeyColumnNames(databaseName, tableName)
		if err != nil {
			table.SetError(err.Error(), nil)
		}

		if len(records) > 0 {
			table.SetRecords(records)
		}

		table.SetColumns(columns)
		table.SetConstraints(constraints)
		table.SetForeignKeys(foreignKeys)
		table.SetIndexes(indexes)
		table.SetPrimaryKeyColumnNames(primaryKeyColumnNames)
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
	inputField.SetFieldBackgroundColor(app.Styles.PrimaryTextColor)
	inputField.SetFieldTextColor(app.Styles.PrimitiveBackgroundColor)
	inputField.SetBorder(true)

	inputField.SetDoneFunc(func(key tcell.Key) {
		table.SetIsEditing(false)
		currentValue := cell.Text
		newValue := inputField.GetText()
		columnName := table.GetCell(0, col).Text

		if key != tcell.KeyEscape {
			cell.SetText(newValue)

			if currentValue != newValue {
				table.AppendNewChange(models.DMLUpdateType, row, col, models.CellValue{Type: models.String, Value: newValue, Column: columnName, TableColumnIndex: col, TableRowIndex: row})
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
			table.Page.RemovePage(pageNameTableEditCell)
			App.SetFocus(table)
		}

		if callback != nil {
			callback(newValue, row, col)
		}
	})

	x, y, width := cell.GetLastPosition()
	inputField.SetRect(x-1, y-1, width+3, 3)
	table.Page.AddPage(pageNameTableEditCell, inputField, false, true)
	App.SetFocus(inputField)
}

func (table *ResultsTable) CheckIfRowIsInserted(rowID string) bool {
	for _, dmlChange := range *table.state.listOfDBChanges {
		if dmlChange.Type == models.DMLInsertType && dmlChange.PrimaryKeyInfo[0].Value == rowID {
			return true
		}
	}

	return false
}

func (table *ResultsTable) MutateInsertedRowCell(rowID string, newValue models.CellValue) {
	for i, dmlChange := range *table.state.listOfDBChanges {
		if dmlChange.PrimaryKeyInfo[0].Value == rowID && dmlChange.Type == models.DMLInsertType {
			for j, v := range dmlChange.Values {
				if v.Column == newValue.Column {
					(*table.state.listOfDBChanges)[i].Values[j] = newValue
					break
				}
			}
		}
	}
}

func (table *ResultsTable) AppendNewChange(changeType models.DMLType, rowIndex int, colIndex int, value models.CellValue) {
	databaseName := table.GetDatabaseName()
	tableName := table.GetTableName()

	dmlChangeAlreadyExists := false

	// If the column has a reference, it means it's an inserted rowIndex
	// There is maybe a better way to detect it is an inserted row
	tableCell := table.GetCell(rowIndex, colIndex)
	tableCellReference := tableCell.GetReference()

	isAnInsertedRow, _ := table.isAnInsertedRow(rowIndex)

	if isAnInsertedRow {
		table.MutateInsertedRowCell(tableCellReference.(string), value)
		return
	}

	rowPrimaryKeyInfo := table.GetPrimaryKeyValue(rowIndex)

	if len(rowPrimaryKeyInfo) == 0 {
		table.SetError(fmt.Sprintf("Primary key not found for row %d", rowIndex), nil)
		return
	}

	if changeType == models.DMLUpdateType {
		switch value.Type {
		case models.Null, models.Empty, models.Default:
			tableCell.SetText(value.Value.(string))
			tableCell.SetStyle(tcell.StyleDefault.Italic(true))
			tableCell.SetReference(value.Value.(string) + "&")
		}
	}

	for i, dmlChange := range *table.state.listOfDBChanges {
		changeExistOnSameCell := false

		for _, value := range dmlChange.Values {
			if value.TableRowIndex == rowIndex && value.TableColumnIndex == colIndex {
				changeExistOnSameCell = true
				break
			}
		}

		if dmlChange.Table == tableName && dmlChange.Type == changeType && changeExistOnSameCell {
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
			case models.DMLUpdateType:
				originalValue := table.GetRecords()[rowIndex][colIndex]

				if changeForColExists {
					if originalValue == value.Value {
						if len((*table.state.listOfDBChanges)[i].Values) == 1 {
							*table.state.listOfDBChanges = append((*table.state.listOfDBChanges)[:i], (*table.state.listOfDBChanges)[i+1:]...)
						} else {
							(*table.state.listOfDBChanges)[i].Values = append((*table.state.listOfDBChanges)[i].Values[:valueIndex], (*table.state.listOfDBChanges)[i].Values[valueIndex+1:]...)
						}
						table.SetCellColor(rowIndex, colIndex, app.Styles.PrimitiveBackgroundColor)
					} else {
						(*table.state.listOfDBChanges)[i].Values[valueIndex] = value
					}
				} else {
					(*table.state.listOfDBChanges)[i].Values = append((*table.state.listOfDBChanges)[i].Values, value)
					table.SetCellColor(rowIndex, colIndex, colorTableChange)
				}

			case models.DMLDeleteType:
				*table.state.listOfDBChanges = append((*table.state.listOfDBChanges)[:i], (*table.state.listOfDBChanges)[i+1:]...)
				table.SetRowColor(rowIndex, app.Styles.PrimitiveBackgroundColor)
			}
		}
	}

	if !dmlChangeAlreadyExists {
		switch changeType {
		case models.DMLDeleteType:
			table.SetRowColor(rowIndex, colorTableDelete)
		case models.DMLUpdateType:
			tableCell.SetStyle(tcell.StyleDefault.Background(colorTableChange))
			table.SetCellColor(rowIndex, colIndex, colorTableChange)
		}

		newDMLChange := models.DBDMLChange{
			Type:           changeType,
			Database:       databaseName,
			Table:          tableName,
			Values:         []models.CellValue{value},
			PrimaryKeyInfo: rowPrimaryKeyInfo,
		}

		*table.state.listOfDBChanges = append(*table.state.listOfDBChanges, newDMLChange)
	}
}

func (table *ResultsTable) GetPrimaryKeyValue(rowIndex int) []models.PrimaryKeyInfo {
	primaryKeyColumnNames := table.GetPrimaryKeyColumnNames()

	info := []models.PrimaryKeyInfo{}

	for _, primaryKeyColumnName := range primaryKeyColumnNames {
		primaryKeyValue := table.GetCell(rowIndex, table.GetColumnIndexByName(primaryKeyColumnName)).Text
		info = append(info, models.PrimaryKeyInfo{Name: primaryKeyColumnName, Value: primaryKeyValue})
	}

	return info
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
			newRow[i-1] = models.CellValue{Type: models.Default, Column: column[0], Value: "DEFAULT", TableRowIndex: newRowTableIndex, TableColumnIndex: i}
		}
	}

	newInsert := models.DBDMLChange{
		Type:           models.DMLInsertType,
		Database:       table.GetDatabaseName(),
		Table:          table.GetTableName(),
		Values:         newRow,
		PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: "", Value: newRowUUID}},
	}

	*table.state.listOfDBChanges = append(*table.state.listOfDBChanges, newInsert)

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
		comparators := []string{
			"=", "!=",
			">", "<",
			">=", "<=",
			"between", "not between",
			"ilike", "not ilike",
			"in", "not in",
			"is", "is not",
			"like", "not like",
			"regexp", "not regexp",
		}

		matches := []string{}

		switch len(split) {
		case 1:
			for _, col := range table.GetColumns()[1:] {
				matches = append(matches, col[0])
			}

		case 2:
			for _, comparator := range comparators {
				if strings.HasPrefix(comparator, split[1]) {
					matches = append(matches, fmt.Sprintf("%s %s", split[0], comparator))
				}
			}

		case 3:
			switch split[1] {
			case "not":
				comparators = []string{"between", "ilike", "in", "like", "regexp"}
			case "is":
				comparators = []string{"not", "null"}
			default:
				return matches
			}
			for _, comparator := range comparators {
				if strings.HasPrefix(comparator, split[2]) {
					matches = append(matches, fmt.Sprintf("%s %s %s", split[0], split[1], comparator))
				}
			}

		case 4:
			switch split[2] {
			case "not":
				comparators = []string{"null"}
			case "is":
				comparators = []string{"not", "null"}
			default:
				return matches
			}
			for _, comparator := range comparators {
				if strings.HasPrefix(comparator, split[3]) {
					matches = append(matches,
						fmt.Sprintf("%s %s %s %s", split[0], split[1], split[2], comparator),
					)
				}
			}
		}

		return matches
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

func (table *ResultsTable) ShowSidebar(show bool) {
	table.state.showSidebar = show

	if show {
		table.UpdateSidebar()

		if App.Config().SidebarOverlay {
			table.Page.SendToFront(pageNameSidebar)
			table.Page.ShowPage(pageNameSidebar)
		} else {
			table.SidebarContainer.AddItem(table.Sidebar, 0, 1, true)
		}
	} else {
		if App.Config().SidebarOverlay {
			table.Page.HidePage(pageNameSidebar)
		} else {
			table.SidebarContainer.RemoveItem(table.Sidebar)
		}
		App.SetFocus(table)
	}
}

func (table *ResultsTable) UpdateSidebar() {
	columns := table.GetColumns()
	selectedRow, _ := table.GetSelection()

	if selectedRow > 0 {

		if App.Config().SidebarOverlay {
			table.recomputeSidebarPosition()
		}

		table.Sidebar.Clear()

		for i := 1; i < len(columns); i++ {
			name := columns[i][0]
			colType := columns[i][1]

			sidebarWidth := table.getSidebarWidth()

			text := table.GetCell(selectedRow, i-1).Text
			title := name

			repeatCount := sidebarWidth - len(name) - len(colType) - 4 // idk why 4 is needed, but it works.

			if repeatCount <= 0 {
				repeatCount = 1
			}

			title += fmt.Sprintf("[%s]", app.Styles.SidebarTitleBorderColor) + strings.Repeat("-", repeatCount)
			title += colType

			pendingEditExist := false

			for _, dmlChange := range *table.state.listOfDBChanges {
				if dmlChange.Type == models.DMLUpdateType {
					for _, v := range dmlChange.Values {
						if v.Column == name && v.TableRowIndex == selectedRow && v.TableColumnIndex == i-1 {
							pendingEditExist = true
							break
						}
					}
				}
			}

			table.Sidebar.AddField(title, text, sidebarWidth, pendingEditExist)
		}

	}
}

func (table *ResultsTable) getSidebarWidth() int {
	if App.Config().SidebarOverlay {
		_, _, tableInnerWidth, _ := table.GetInnerRect()
		return (tableInnerWidth / 4)
	}

	_, _, width, _ := table.SidebarContainer.GetInnerRect()

	return width
}

// Only used when AppConfig.SidebarOverlay is true.
func (table *ResultsTable) recomputeSidebarPosition() {
	tableX, _, _, tableHeight := table.GetRect()
	_, _, tableInnerWidth, _ := table.GetInnerRect()
	_, tableMenuY, _, tableMenuHeight := table.Menu.GetRect()
	_, _, _, tableFilterHeight := table.Filter.GetRect()
	_, _, _, tablePaginationHeight := table.Pagination.GetRect()

	sidebarWidth := (tableInnerWidth / 4)
	sidebarHeight := tableHeight + tableMenuHeight + tableFilterHeight + tablePaginationHeight + 1

	table.Sidebar.SetRect(tableX+tableInnerWidth-sidebarWidth, tableMenuY, sidebarWidth, sidebarHeight)
}

func (table *ResultsTable) isAnInsertedRow(rowIndex int) (isAnInsertedRow bool, DBChangeIndex int) {
	for i, dmlChange := range *table.state.listOfDBChanges {
		for _, value := range dmlChange.Values {
			if value.TableRowIndex != rowIndex {
				continue
			}
			cellReference := table.GetCell(rowIndex, 0).GetReference()
			if cellReference == nil {
				break
			}
			switch cellReference.(string) {
			case "NULL&", "EMPTY&", "DEFAULT&":
			default:
				return true, i
			}
			break
		}
	}
	return false, -1
}

func (table *ResultsTable) colorChangedCells() {
	for _, dmlChange := range *table.state.listOfDBChanges {
		switch dmlChange.Type {
		case models.DMLDeleteType:
			table.SetRowColor(dmlChange.Values[0].TableRowIndex, colorTableDelete)
		case models.DMLUpdateType:
			for _, value := range dmlChange.Values {
				table.SetCellColor(value.TableRowIndex, value.TableColumnIndex, colorTableChange)
			}
		}
	}
}
