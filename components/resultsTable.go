package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
	"github.com/rivo/tview"
	"golang.design/x/clipboard"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

type ResultsTableState struct {
	listOfDbChanges *[]models.DbDmlChange
	listOfDbInserts *[]models.DbInsert
	dbReference     string
	currentSort     string
	error           string
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
	DBDriver    *drivers.MySQL
}

var (
	ErrorModal  = tview.NewModal()
	ChangeColor = tcell.ColorDarkOrange.TrueColor()
	InsertColor = tcell.ColorDarkGreen.TrueColor()
	DeleteColor = tcell.ColorRed
)

func NewResultsTable(
	listOfDbChanges *[]models.DbDmlChange,
	listOfDbInserts *[]models.DbInsert,
	tree *Tree,
	dbdriver *drivers.MySQL,
) *ResultsTable {
	state := &ResultsTableState{
		records:         [][]string{},
		columns:         [][]string{},
		constraints:     [][]string{},
		foreignKeys:     [][]string{},
		indexes:         [][]string{},
		isEditing:       false,
		isLoading:       false,
		listOfDbChanges: listOfDbChanges,
		listOfDbInserts: listOfDbInserts,
	}

	wrapper := tview.NewFlex()
	wrapper.SetDirection(tview.FlexColumnCSS)

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
	// table.SetSelectedStyle(tcell.StyleDefault.Background(tcell.ColorLightGray).Foreground(tcell.ColorBlack.TrueColor()))

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
	resultsInfoText.SetBorderColor(app.FocusTextColor)
	resultsInfoText.SetTextColor(app.FocusTextColor)
	resultsInfoWrapper.AddItem(resultsInfoText, 3, 0, false)

	editorPages.AddPage("Table", tableWrapper, true, false)
	editorPages.AddPage("ResultsInfo", resultsInfoWrapper, true, true)

	table.EditorPages = editorPages
	table.ResultsInfo = resultsInfoText

	table.Wrapper.AddItem(editorPages, 0, 1, true)

	go table.subscribeToEditorChanges()

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

func (table *ResultsTable) AddInsertedRows() {
	inserts := *table.state.listOfDbInserts
	rows := make([][]string, len(inserts))

	if len(inserts) > 0 {
		for i, insert := range inserts {
			if insert.Table == table.GetDBReference() && insert.Option == table.Menu.GetSelectedOption() {
				rows[i] = insert.Values
			}
		}
	}

	rowCount := table.GetRowCount()
	for i, row := range rows {
		rowIndex := rowCount + i

		for j, cell := range row {
			tableCell := tview.NewTableCell(cell)
			tableCell.SetExpansion(1)
			tableCell.SetReference(inserts[i].RowID)

			tableCell.SetTextColor(app.FocusTextColor)
			tableCell.SetBackgroundColor(InsertColor)

			table.SetCell(rowIndex, j, tableCell)
		}
	}
}

func (table *ResultsTable) InsertRow(cols []string, index int, id uuid.UUID) {
	for i, cell := range cols {
		tableCell := tview.NewTableCell(cell)
		tableCell.SetExpansion(1)

		if i == 0 {
			tableCell.SetReference(id)
		}

		tableCell.SetTextColor(app.FocusTextColor)
		table.SetCell(index, i, tableCell)
	}
}

func (table *ResultsTable) tableInputCapture(event *tcell.EventKey) *tcell.EventKey {
	selectedRowIndex, selectedColumnIndex := table.GetSelection()
	colCount := table.GetColumnCount()
	rowCount := table.GetRowCount()

	eventRune := event.Rune()

	if eventRune == '1' || eventRune == '2' || eventRune == '3' || eventRune == '4' || eventRune == '5' {
		table.Select(1, 0)
	}

	if table.Menu != nil {
		switch eventRune {
		case '1':
			table.Menu.SetSelectedOption(1)
			table.UpdateRows(table.GetRecords())
		case '2':
			table.Menu.SetSelectedOption(2)
			table.UpdateRows(table.GetColumns())
		case '3':
			table.Menu.SetSelectedOption(3)
			table.UpdateRows(table.GetConstraints())
		case '4':
			table.Menu.SetSelectedOption(4)
			table.UpdateRows(table.GetForeignKeys())
		case '5':
			table.Menu.SetSelectedOption(5)
			table.UpdateRows(table.GetIndexes())
		}
	}

	switch eventRune {
	case '/':
		if table.Editor != nil {
			App.SetFocus(table.Editor)
			table.Editor.Highlight()
			table.RemoveHighlightTable()
			table.SetIsFiltering(true)

			return nil
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

			switch len(split) {
			case 1:
				columns := table.GetColumns()
				columnNames := []string{}

				for i, col := range columns {
					if i > 0 {
						columnNames = append(columnNames, col[0])
					}
				}

				return columnNames
			case 2:

				for i, comparator := range comparators {
					comparators[i] = fmt.Sprintf("%s %s", split[0], strings.ToLower(comparator))
				}

				return comparators
			case 3:

				ret := true

				if split[1] == "not" {
					comparators = []string{"between", "in", "like"}
				} else if split[1] == "is" {
					comparators = []string{"not", "null"}
				} else {
					ret = false
				}

				if ret {
					for i, comparator := range comparators {
						comparators[i] = fmt.Sprintf("%s %s %s", split[0], split[1], strings.ToLower(comparator))
					}
					return comparators
				}
			case 4:
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

				switch len(inputText) {
				case 1:
					table.Filter.Input.SetText(fmt.Sprintf("%s =", text))
				case 2:
					table.Filter.Input.SetText(fmt.Sprintf("%s %s", inputText[0], text))
				}

				table.Filter.Input.SetText(text)
			}
			return source == tview.AutocompletedEnter || source == tview.AutocompletedClick
		})

		table.SetInputCapture(nil)
	case 'c':
		table.StartEditingCell(selectedRowIndex, selectedColumnIndex, func(newValue string, row, col int) {
			cellReference := table.GetCell(row, 0).GetReference()

			if cellReference != nil {
				table.MutateInsertedRowCell(cellReference.(uuid.UUID), col, newValue)
			}
		})
	case 'w':
		if selectedColumnIndex+1 < colCount {
			table.Select(selectedRowIndex, selectedColumnIndex+1)
		}
	case 'b':
		if selectedColumnIndex > 0 {
			table.Select(selectedRowIndex, selectedColumnIndex-1)
		}
	case '$':
		table.Select(selectedRowIndex, colCount-1)
	case '0':
		table.Select(selectedRowIndex, 0)
	case 'g':
		go table.Select(1, selectedColumnIndex)
	case 'G':
		go table.Select(rowCount-1, selectedColumnIndex)
	case 4: // Ctrl + D
		if selectedRowIndex+7 > rowCount-1 {
			go table.Select(rowCount-1, selectedColumnIndex)
		} else {
			go table.Select(selectedRowIndex+7, selectedColumnIndex)
		}
	case 21: // Ctrl + U
		if selectedRowIndex-7 < 1 {
			go table.Select(1, selectedColumnIndex)
		} else {
			go table.Select(selectedRowIndex-7, selectedColumnIndex)
		}
	case 'd':
		if table.Menu.GetSelectedOption() == 1 {
			isAnInsertedRow := false
			indexOfInsertedRow := -1

			for i, insertedRow := range *table.state.listOfDbInserts {
				cellReference := table.GetCell(selectedRowIndex, 0).GetReference()

				if cellReference != nil && insertedRow.RowID.String() == cellReference.(uuid.UUID).String() {
					isAnInsertedRow = true
					indexOfInsertedRow = i
				}
			}

			if isAnInsertedRow {
				*table.state.listOfDbInserts = append(
					(*table.state.listOfDbInserts)[:indexOfInsertedRow],
					(*table.state.listOfDbInserts)[indexOfInsertedRow+1:]...,
				)
				table.RemoveRow(selectedRowIndex)

				if selectedRowIndex-1 != 0 {
					table.Select(selectedRowIndex-1, 0)
				} else if selectedRowIndex+1 < rowCount {
					table.Select(selectedRowIndex+1, 0)
				}

				switch {
				case len(*table.state.listOfDbChanges) == 0 && len(*table.state.listOfDbInserts) == 0:
					table.Tree.ForceRemoveHighlight()
				case len(*table.state.listOfDbChanges) == 0 && len(*table.state.listOfDbInserts) > 0:
					table.Tree.GetCurrentNode().SetColor(InsertColor)
				case len(*table.state.listOfDbChanges) > 0 && len(*table.state.listOfDbInserts) == 0:
					table.Tree.GetCurrentNode().SetColor(ChangeColor)
				}
			} else {
				table.AppendNewChange("DELETE", table.GetDBReference(), selectedRowIndex, -1, "")
			}
		}
	case 'o':
		if table.Menu.GetSelectedOption() == 1 {
			newRow := make([]string, table.GetColumnCount())
			newRowIndex := table.GetRowCount()
			newRowUUID := uuid.New()

			for i := 0; i < table.GetColumnCount(); i++ {
				newRow[i] = "Default"
			}

			table.InsertRow(newRow, newRowIndex, newRowUUID)

			for i := 0; i < table.GetColumnCount(); i++ {
				table.GetCell(newRowIndex, i).SetBackgroundColor(tcell.ColorDarkGreen)
			}

			newInsert := models.DbInsert{
				Table:   table.GetDBReference(),
				Columns: table.GetRecords()[0],
				Values:  newRow,
				RowID:   newRowUUID,
				Option:  1,
			}

			*table.state.listOfDbInserts = append(*table.state.listOfDbInserts, newInsert)

			if table.Tree.GetCurrentNode().GetColor() == app.InactiveTextColor || table.Tree.GetCurrentNode().GetColor() == app.FocusTextColor {
				table.Tree.GetCurrentNode().SetColor(InsertColor)
			} else if table.Tree.GetCurrentNode().GetColor() == DeleteColor {
				table.Tree.GetCurrentNode().SetColor(ChangeColor)
			}

			table.Select(newRowIndex, 1)

			App.ForceDraw()
			table.StartEditingCell(newRowIndex, 1, func(newValue string, row, col int) {
				cellReference := table.GetCell(row, 0).GetReference()

				if cellReference != nil {
					table.MutateInsertedRowCell(cellReference.(uuid.UUID), col, newValue)
				}
			})
		}
	}

	if len(table.GetRecords()) > 0 {
		switch eventRune {
		case 'J':
			currentColumnName := table.GetColumnNameByIndex(selectedColumnIndex)
			table.Pagination.SetOffset(0)
			table.SetSortedBy(currentColumnName, "DESC")

		case 'K':
			currentColumnName := table.GetColumnNameByIndex(selectedColumnIndex)
			table.Pagination.SetOffset(0)
			table.SetSortedBy(currentColumnName, "ASC")
		case 'y':
			selectedCell := table.GetCell(selectedRowIndex, selectedColumnIndex)
			if selectedCell == nil || clipboard.Init() != nil {
				break
			}

			if text := []byte(selectedCell.Text); text != nil {
				clipboard.Write(clipboard.FmtText, text)
			}
		}
	}

	return event
}

func (table *ResultsTable) UpdateRows(rows [][]string) {
	table.Clear()
	table.AddRows(rows)
	table.AddInsertedRows()
	App.ForceDraw()
	table.Select(1, 0)
}

func (table *ResultsTable) UpdateRowsColor(headerColor, rowColor tcell.Color) {
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
	table.SetBorderColor(app.InactiveTextColor)
	table.SetBordersColor(app.InactiveTextColor)
	table.SetTitleColor(app.InactiveTextColor)
	table.UpdateRowsColor(app.InactiveTextColor, app.InactiveTextColor)
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
	table.SetBorderColor(app.FocusTextColor)
	table.SetBordersColor(app.FocusTextColor)
	table.SetTitleColor(app.FocusTextColor)
	table.UpdateRowsColor(app.ActiveTextColor, app.FocusTextColor)
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
		if stateChange.Key == "Filter" {
			if stateChange.Value != "" {
				rows := table.FetchRecords(table.GetDBReference())

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
				table.FetchRecords(table.GetDBReference())
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

					rows, err := table.DBDriver.QueryPaginatedRecords(query)

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

					result, err := table.DBDriver.ExecuteDMLQuery(query)
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

func (table *ResultsTable) SetDBReference(dbReference string) {
	table.state.dbReference = dbReference
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

func (table *ResultsTable) SetSortedBy(column, direction string) {
	sort := fmt.Sprintf("%s %s", column, direction)

	if table.GetCurrentSort() != sort {
		where := ""
		if table.Filter != nil {
			where = table.Filter.GetCurrentFilter()
		}

		table.SetLoading(true)
		records, err := table.DBDriver.GetRecords(
			table.GetDBReference(),
			where,
			sort,
			table.Pagination.GetOffset(),
			table.Pagination.GetLimit(),
			true,
		)

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
			if i == 0 {
				continue
			}

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

func (table *ResultsTable) FetchRecords(tableName string) [][]string {
	table.SetLoading(true)

	where := ""
	if table.Filter != nil {
		where = table.Filter.GetCurrentFilter()
	}

	sort := table.GetCurrentSort()

	records, totalRecords, err := table.DBDriver.GetPaginatedRecords(
		tableName,
		where,
		sort,
		table.Pagination.GetOffset(),
		table.Pagination.GetLimit(),
		true,
	)

	if err != nil {
		table.SetError(err.Error(), nil)
		table.SetLoading(false)
	} else {
		if table.GetIsFiltering() {
			table.SetIsFiltering(false)
		}

		columns := table.DBDriver.DescribeTable(tableName)
		constraints := table.DBDriver.GetTableConstraints(tableName)
		foreignKeys := table.DBDriver.GetTableForeignKeys(tableName)
		indexes := table.DBDriver.GetTableIndexes(tableName)

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

	return [][]string{}
}

func (table *ResultsTable) StartEditingCell(row, col int, callback func(newValue string, row, col int)) {
	table.SetIsEditing(true)
	table.SetInputCapture(nil)

	cell := table.GetCell(row, col)
	inputField := tview.NewInputField()
	inputField.SetText(cell.Text)
	inputField.SetFieldBackgroundColor(app.ActiveTextColor)
	inputField.SetFieldTextColor(tcell.ColorBlack)

	inputField.SetDoneFunc(func(key tcell.Key) {
		table.SetIsEditing(false)
		currentValue := cell.Text
		newValue := inputField.GetText()

		switch key {
		case tcell.KeyEnter:
			if currentValue != newValue {
				cell.SetText(inputField.GetText())
				table.AppendNewChange("UPDATE", table.GetDBReference(), row, col, newValue)
			}
		case tcell.KeyTab:
			nextEditableColumnIndex := col + 1

			if nextEditableColumnIndex <= table.GetColumnCount()-1 {
				cell.SetText(inputField.GetText())
				table.Select(row, nextEditableColumnIndex)
				table.StartEditingCell(row, nextEditableColumnIndex, callback)
			}
		case tcell.KeyBacktab:
			nextEditableColumnIndex := col - 1

			if nextEditableColumnIndex >= 0 {
				cell.SetText(inputField.GetText())
				table.Select(row, nextEditableColumnIndex)
				table.StartEditingCell(row, nextEditableColumnIndex, callback)
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

func (table *ResultsTable) CheckIfRowIsInserted(rowID uuid.UUID) bool {
	for _, insertedRow := range *table.state.listOfDbInserts {
		if insertedRow.RowID == rowID {
			return true
		}
	}

	return false
}

func (table *ResultsTable) MutateInsertedRowCell(rowID uuid.UUID, colIndex int, newValue string) {
	for i, insertedRow := range *table.state.listOfDbInserts {
		if insertedRow.RowID == rowID {
			(*table.state.listOfDbInserts)[i].Values[colIndex] = newValue
		}
	}
}

// TODO: encapsulate logic for different changeType
func (table *ResultsTable) AppendNewChange(changeType, tableName string, rowIndex, colIndex int, value string) {
	// check if there is already a change row in the listOfDbChanges variable
	// if there is, update the value
	// if there isn't, append a new change row
	// if the value is the same as the original value, remove the change row

	cellReference := table.GetCell(rowIndex, 0).GetReference()

	isInsertedRow := false

	if cellReference != nil {
		isInsertedRow = table.CheckIfRowIsInserted(cellReference.(uuid.UUID))
	}

	if !isInsertedRow {
		selectedRowID := table.GetRecords()[rowIndex][0]

		alreadyExists := false
		indexOfChange := -1

		for i, change := range *table.state.listOfDbChanges {
			if change.RowID == selectedRowID && change.Column == table.GetColumnNameByIndex(colIndex) {
				alreadyExists = true
				indexOfChange = i
			}
		}

		switch changeType {
		case "UPDATE":
			cell := table.GetCell(rowIndex, colIndex)
			columnName := table.GetColumnNameByIndex(colIndex)
			originalCellValue := table.GetRecords()[rowIndex][colIndex]

			if alreadyExists {
				if value == originalCellValue {
					*table.state.listOfDbChanges = append((*table.state.listOfDbChanges)[:indexOfChange], (*table.state.listOfDbChanges)[indexOfChange+1:]...)

					cell.SetBackgroundColor(tcell.ColorDefault)
					cell.SetTextColor(app.FocusTextColor)

					if len(*table.state.listOfDbChanges) == 0 && len(*table.state.listOfDbInserts) == 0 {
						table.Tree.GetCurrentNode().SetColor(app.InactiveTextColor)
					} else if len(*table.state.listOfDbChanges) == 0 && len(*table.state.listOfDbInserts) > 0 {
						table.Tree.GetCurrentNode().SetColor(InsertColor)
					} else if len(*table.state.listOfDbChanges) > 0 && len(*table.state.listOfDbInserts) == 0 {
						table.Tree.GetCurrentNode().SetColor(ChangeColor)
					}
				} else {
					cell.SetBackgroundColor(tcell.ColorOrange.TrueColor())
					cell.SetTextColor(tcell.ColorBlack.TrueColor())
					table.Tree.GetCurrentNode().SetColor(ChangeColor)

					(*table.state.listOfDbChanges)[indexOfChange].Value = value
				}
			} else {
				newChange := models.DbDmlChange{
					Type:   changeType,
					Table:  tableName,
					Column: columnName,
					Value:  value,
					RowID:  selectedRowID,
					Option: 1,
				}

				*table.state.listOfDbChanges = append(*table.state.listOfDbChanges, newChange)

				cell.SetBackgroundColor(tcell.ColorOrange.TrueColor())
				cell.SetTextColor(tcell.ColorBlack.TrueColor())
				table.Tree.GetCurrentNode().SetColor(ChangeColor)
			}
		case "DELETE":
			if alreadyExists {
				*table.state.listOfDbChanges = append((*table.state.listOfDbChanges)[:indexOfChange], (*table.state.listOfDbChanges)[indexOfChange+1:]...)

				if len(*table.state.listOfDbChanges) == 0 && len(*table.state.listOfDbInserts) == 0 {
					table.Tree.GetCurrentNode().SetColor(app.InactiveTextColor)
				} else if len(*table.state.listOfDbChanges) == 0 && len(*table.state.listOfDbInserts) > 0 {
					table.Tree.GetCurrentNode().SetColor(InsertColor)
				} else if len(*table.state.listOfDbChanges) > 0 && len(*table.state.listOfDbInserts) == 0 {
					table.Tree.GetCurrentNode().SetColor(ChangeColor)
				}

				for i := 0; i < table.GetColumnCount(); i++ {
					table.GetCell(rowIndex, i).SetBackgroundColor(tcell.ColorDefault)
				}

			} else {
				if table.Tree.GetCurrentNode().GetColor() == app.InactiveTextColor || table.Tree.GetCurrentNode().GetColor() == app.FocusTextColor {
					table.Tree.GetCurrentNode().SetColor(DeleteColor)
				} else if table.Tree.GetCurrentNode().GetColor() == InsertColor {
					table.Tree.GetCurrentNode().SetColor(ChangeColor)
				}

				newChange := models.DbDmlChange{
					Type:   changeType,
					Table:  tableName,
					Column: "",
					Value:  "",
					RowID:  selectedRowID,
					Option: 1,
				}

				*table.state.listOfDbChanges = append(*table.state.listOfDbChanges, newChange)

				for i := 0; i < table.GetColumnCount(); i++ {
					table.GetCell(rowIndex, i).SetBackgroundColor(DeleteColor)
				}
			}
		}
	}
}
