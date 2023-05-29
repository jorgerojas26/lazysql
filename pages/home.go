package pages

import (
	"lazysql/drivers"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Table struct {
	database string
	tables   []string
}

var HomePage = tview.NewFlex()
var InputFilter = tview.NewInputField()
var DatabaseTable = tview.NewTable().SetSelectable(true, true)
var DatabaseTablePages = tview.NewPages()
var leftWrapper = tview.NewFlex()
var rightWrapper = tview.NewFlex()
var databaseTableRowLimit = 100
var currentTable = ""
var Tree *tview.TreeView
var Databases []string
var Tables []Table
var FilteredTables []Table

func init() {
	DatabaseTableConfig()

	rightWrapper.SetBorder(true)

	leftWrapper.AddItem(databaseList(), 0, 1, true)
	rightWrapper.AddItem(DatabaseTablePages, 0, 1, false)

	HomePage.AddItem(leftWrapper, 0, 1, true)
	HomePage.AddItem(rightWrapper, 0, 5, false)

	AllPages.AddPage("home", HomePage, true, false)
}

func databaseList() *tview.Flex {

	wrapper := tview.NewFlex().SetDirection(tview.FlexRow)
	wrapper.SetTitle("Databases")
	wrapper.SetBorder(true)

	wrapper.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 47 {
			go App.SetFocus(InputFilter)
			InputFilter.SetBorderColor(tcell.ColorKhaki)
		} else if event.Key() == tcell.KeyEnter {
			App.SetFocus(Tree)
		} else if event.Rune() == 76 { // L Key
			App.SetFocus(DatabaseTable)
		}

		return event
	})

	wrapper.SetFocusFunc(func() {
		databases, err := drivers.Database.GetDatabases()
		Databases = databases

		Tree = tview.NewTreeView()
		rootNode := tview.NewTreeNode("---------------------------------------------------------------------")

		Tree.SetTopLevel(1)
		Tree.SetGraphicsColor(tcell.ColorKhaki)
		Tree.SetSelectedFunc(func(node *tview.TreeNode) {
			if node.GetLevel() == 1 {
				if node.IsExpanded() {
					node.SetExpanded(false)
				} else {
					node.SetExpanded(true)
					selectedDatabase := node.GetReference()
					tables := getDatabaseTables(selectedDatabase.(string))

					node.ClearChildren()
					updateTreeNodes(node, tables, selectedDatabase.(string))

				}
			} else if node.GetLevel() == 2 {
				currentTable = node.GetReference().(string)
				data := drivers.Database.GetTableData(currentTable, 0, 0, true)
				updateTableRows(data)
				App.SetFocus(DatabaseTable)
				go DatabaseTable.Select(1, 0)
			}

		})

		Tree.SetRoot(rootNode)

		if err != nil {
			panic(err)
		}

		drawDatabaseTree(databases, nil, false)

		wrapper.AddItem(Tree, 0, 1, true)
		App.SetFocus(Tree)
	})

	filter := tview.NewFlex()
	filter.AddItem(inputFieldFilter(), 0, 1, false)

	wrapper.AddItem(filter, 2, 1, false)

	return wrapper
}

func DatabaseTableConfig() {
	DatabaseTable.SetBorders(true)
	DatabaseTable.SetBordersColor(tcell.ColorKhaki)
	DatabaseTable.SetFixed(1, 0)
	DatabaseTable.SetWrapSelection(true, false)

	DatabaseTable.SetSelectionChangedFunc(func(row, col int) {
		selectedRow, _ := DatabaseTable.GetSelection()
		rowCount := DatabaseTable.GetRowCount()

		if selectedRow == rowCount-1 {
			data := drivers.Database.GetTableData(currentTable, rowCount, databaseTableRowLimit, false)
			addTableRows(data)

		}
	})

	DatabaseTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		selectedRow, selectedColumn := DatabaseTable.GetSelection()
		rowCount := DatabaseTable.GetRowCount()

		if event.Rune() == 99 { // c Key
			cell := DatabaseTable.GetCell(selectedRow, selectedColumn)
			inputField := tview.NewInputField()
			inputField.SetText(cell.Text)
			inputField.SetDoneFunc(func(key tcell.Key) {
				cell.SetText(inputField.GetText())
				DatabaseTablePages.RemovePage("edit")
				App.SetFocus(DatabaseTable)
			})
			x, y, width := cell.GetLastPosition()
			inputField.SetRect(x, y, width+1, 1)
			DatabaseTablePages.AddPage("edit", inputField, false, true)
			App.SetFocus(inputField)

		} else if event.Rune() == 36 { // $ Key
			colCount := DatabaseTable.GetColumnCount()
			DatabaseTable.Select(selectedRow, colCount-1)
		} else if event.Rune() == 48 { // 0 Key
			DatabaseTable.Select(selectedRow, 0)
		} else if event.Rune() == 103 { // g Key
			go DatabaseTable.Select(1, selectedColumn)
		} else if event.Rune() == 71 { // G Key
			go DatabaseTable.Select(rowCount-1, selectedColumn)
		} else if event.Rune() == 100 { // d Key
			if selectedRow+7 > rowCount-1 {
				go DatabaseTable.Select(rowCount-1, selectedColumn)
			} else {
				go DatabaseTable.Select(selectedRow+7, selectedColumn)
			}
		} else if event.Rune() == 117 { // u Key
			if selectedRow-7 < 1 {
				go DatabaseTable.Select(1, selectedColumn)
			} else {
				go DatabaseTable.Select(selectedRow-7, selectedColumn)
			}
		} else if event.Rune() == 72 { // H Key
			App.SetFocus(leftWrapper)
		}

		return event
	})

	DatabaseTablePages.AddPage("table", DatabaseTable, true, true)
}

func updateTableRows(data [][]string) {
	DatabaseTable.Clear()

	for x, row := range data {
		for y, column := range row {
			cell := tview.NewTableCell(column)
			cell.SetReference(column)
			cell.SetSelectable(x > 0)
			cell.SetExpansion(1)

			if x == 0 {
				cell.SetTextColor(tcell.ColorKhaki)
			}

			DatabaseTable.SetCell(x, y, cell)
		}
	}
}

func addTableRows(data [][]string) {
	rowCount := DatabaseTable.GetRowCount()

	for x, row := range data {
		for y, column := range row {
			cell := tview.NewTableCell(column)
			cell.SetReference(column)
			cell.SetSelectable(x > 0)
			cell.SetExpansion(1)

			DatabaseTable.SetCell(x+rowCount, y, cell)
		}
	}
}

func inputFieldFilter() *tview.InputField {
	InputFilter.SetPlaceholder("Filter tables")
	InputFilter.SetChangedFunc(func(text string) {
		rootNode := Tree.GetRoot()

		rootNode.ClearChildren()
		if text != "" {
			FilteredTables = make([]Table, 0)
			for _, table := range Tables {
				tables := table.tables
				db := table.database

				for _, tableName := range tables {
					if strings.Contains(strings.ToLower(tableName), strings.ToLower(text)) {
						found := false
						for _, filteredTable := range FilteredTables {
							if filteredTable.database == db {
								found = true
								filteredTable.tables = append(filteredTable.tables, tableName)
							}
						}

						if !found {

							FilteredTables = append(FilteredTables, Table{database: db, tables: []string{tableName}})
						}

					}
				}
			}

			drawDatabaseTree(Databases, FilteredTables, true)
		} else {
			drawDatabaseTree(Databases, Tables, false)
		}

	})

	InputFilter.SetDoneFunc(func(key tcell.Key) {
		FilteredTables = make([]Table, 0)
		App.SetFocus(Tree)
		if key == tcell.KeyEscape {
			InputFilter.SetText("")
			rootNode := Tree.GetRoot()
			rootNode.ClearChildren()
			drawDatabaseTree(Databases, Tables, false)
		}

	})

	return InputFilter
}

func getDatabaseTables(database string) []string {
	tables, err := drivers.Database.GetTables(database)

	if err != nil {
		panic(err)
	}

	return tables

}

func updateTreeNodes(node *tview.TreeNode, children []string, database string) {
	for _, child := range children {
		childNode := tview.NewTreeNode(child)
		if database != "" {
			childNode.SetReference(database + "." + child)
		} else {
			childNode.SetReference(database)

		}
		childNode.SetColor(tcell.ColorKhaki)
		childNode.SetExpanded(true)

		node.AddChild(childNode)
	}
}

func drawDatabaseTree(databases []string, tables []Table, expanded bool) {
	for i, database := range Databases {
		databaseNode := tview.NewTreeNode(database)
		databaseNode.SetReference(database)
		databaseNode.SetExpanded(expanded)

		if tables != nil {
			for _, table := range tables {
				if table.database == database {
					updateTreeNodes(databaseNode, table.tables, database)
				}
			}
		} else {

			tbls := getDatabaseTables(database)
			Tables = append(Tables, Table{database: database, tables: tbls})
			updateTreeNodes(databaseNode, tbls, database)

		}

		Tree.GetRoot().AddChild(databaseNode)

		if i == 0 {
			Tree.SetCurrentNode(databaseNode)
		}

	}
}
