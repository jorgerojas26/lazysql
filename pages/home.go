package pages

import (
	"lazysql/components"
	"lazysql/drivers"
	"lazysql/utils"
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

var DBRecordsTable = tview.NewTable()
var DBColumnsTable = tview.NewTable()
var DBConstraintsTable = tview.NewTable()
var DBForeignKeysTable = tview.NewTable()
var DBIndexesTable = tview.NewTable()

var TableTreeWrapper = tview.NewFlex()
var rightWrapper = tview.NewFlex()
var dbTableRowLimit = 100
var currentTable = ""
var Tree *tview.TreeView
var Databases []string
var Tables []Table
var FilteredTables []Table
var SelectedOption = 1
var StagedChanges []string

func init() {
	rightWrapper.SetBorder(true)

	TableTreeWrapper.AddItem(databaseList(), 0, 1, true)
	rightWrapper.AddItem(renderDatabaseResults(), 0, 1, false)

	HomePage.AddItem(TableTreeWrapper, 0, 1, true)
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
			App.SetFocus(DBRecordsTable)
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
				components.RenderColumns(currentTable, DBColumnsTable)
				tblName := strings.Split(currentTable, ".")[1]
				components.RenderConstraints(tblName, DBConstraintsTable)
				components.RenderForeignKeys(tblName, DBForeignKeysTable)
				components.RenderIndexes(currentTable, DBIndexesTable)

				if SelectedOption == 1 {
					App.SetFocus(DBRecordsTable)
					go DBRecordsTable.Select(1, 0)
				} else if SelectedOption == 2 {
					App.SetFocus(DBColumnsTable)
					go DBColumnsTable.Select(1, 0)
				} else if SelectedOption == 3 {
					App.SetFocus(DBConstraintsTable)
					go DBConstraintsTable.Select(1, 0)
				} else if SelectedOption == 4 {
					App.SetFocus(DBForeignKeysTable)
					go DBForeignKeysTable.Select(1, 0)
				} else if SelectedOption == 5 {
					App.SetFocus(DBIndexesTable)
					go DBIndexesTable.Select(1, 0)
				}
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

func renderDatabaseResults() *tview.Flex {
	dbTablePages := tview.NewPages()
	wrapper := tview.NewFlex().SetDirection(tview.FlexColumnCSS)

	dbRecordsTable := DBRecordsTable
	dbColumnsTable := DBColumnsTable
	dbConstraintsTable := DBConstraintsTable
	dbForeignKeysTable := DBForeignKeysTable
	dbIndexesTable := DBIndexesTable

	dbRecordsTable.SetSelectable(true, true)
	dbRecordsTable.SetBorders(true)
	dbRecordsTable.SetBorderColor(tcell.ColorKhaki)
	dbRecordsTable.SetFixed(1, 0)
	dbRecordsTable.SetWrapSelection(true, false)
	components.AddVimKeyBindings(dbRecordsTable, TableTreeWrapper, App, dbTablePages)

	dbColumnsTable.SetSelectable(true, true)
	dbColumnsTable.SetBorders(true)
	dbColumnsTable.SetBorderColor(tcell.ColorKhaki)
	dbColumnsTable.SetFixed(1, 0)
	dbColumnsTable.SetWrapSelection(true, false)
	components.AddVimKeyBindings(dbColumnsTable, TableTreeWrapper, App, dbTablePages)

	dbConstraintsTable.SetSelectable(true, true)
	dbConstraintsTable.SetBorders(true)
	dbConstraintsTable.SetBordersColor(tcell.ColorKhaki)
	dbConstraintsTable.SetFixed(1, 0)
	dbConstraintsTable.SetWrapSelection(true, false)
	components.AddVimKeyBindings(dbConstraintsTable, TableTreeWrapper, App, dbTablePages)

	dbForeignKeysTable.SetSelectable(true, true)
	dbForeignKeysTable.SetBorders(true)
	dbForeignKeysTable.SetBordersColor(tcell.ColorKhaki)
	dbForeignKeysTable.SetFixed(1, 0)
	dbForeignKeysTable.SetWrapSelection(true, false)
	components.AddVimKeyBindings(dbForeignKeysTable, TableTreeWrapper, App, dbTablePages)

	dbIndexesTable.SetSelectable(true, true)
	dbIndexesTable.SetBorders(true)
	dbIndexesTable.SetBorderColor(tcell.ColorKhaki)
	dbIndexesTable.SetFixed(1, 0)
	dbIndexesTable.SetWrapSelection(true, false)
	components.AddVimKeyBindings(dbIndexesTable, TableTreeWrapper, App, dbTablePages)

	dbOptions := tview.NewFlex()
	dbOptions.SetBorder(true)

	records := tview.NewTextView().SetText("Records [1] | ").SetTextColor(tcell.ColorKhaki)
	columns := tview.NewTextView().SetText("Columns [2] | ")
	constraints := tview.NewTextView().SetText("Constraints [3] | ")
	foreignKeys := tview.NewTextView().SetText("Foreign Keys [4] | ")
	indexes := tview.NewTextView().SetText("Indexes [5]")

	wrapper.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 49 {
			SelectedOption = 1
			records.SetTextColor(tcell.ColorKhaki)
			columns.SetTextColor(tcell.ColorWhite)
			constraints.SetTextColor(tcell.ColorWhite)
			foreignKeys.SetTextColor(tcell.ColorWhite)
			indexes.SetTextColor(tcell.ColorWhite)
			App.SetFocus(dbRecordsTable)
			dbTablePages.SwitchToPage("records")
		} else if event.Rune() == 50 {
			SelectedOption = 2
			columns.SetTextColor(tcell.ColorKhaki)
			records.SetTextColor(tcell.ColorWhite)
			constraints.SetTextColor(tcell.ColorWhite)
			foreignKeys.SetTextColor(tcell.ColorWhite)
			indexes.SetTextColor(tcell.ColorWhite)
			App.SetFocus(dbColumnsTable)
			dbTablePages.SwitchToPage("columns")
		} else if event.Rune() == 51 {
			SelectedOption = 3
			constraints.SetTextColor(tcell.ColorKhaki)
			records.SetTextColor(tcell.ColorWhite)
			columns.SetTextColor(tcell.ColorWhite)
			foreignKeys.SetTextColor(tcell.ColorWhite)
			indexes.SetTextColor(tcell.ColorWhite)
			App.SetFocus(dbConstraintsTable)
			dbTablePages.SwitchToPage("constraints")
		} else if event.Rune() == 52 {
			SelectedOption = 4
			foreignKeys.SetTextColor(tcell.ColorKhaki)
			records.SetTextColor(tcell.ColorWhite)
			columns.SetTextColor(tcell.ColorWhite)
			constraints.SetTextColor(tcell.ColorWhite)
			indexes.SetTextColor(tcell.ColorWhite)
			App.SetFocus(dbForeignKeysTable)
			dbTablePages.SwitchToPage("foreignKeys")
		} else if event.Rune() == 53 {
			SelectedOption = 5
			indexes.SetTextColor(tcell.ColorKhaki)
			records.SetTextColor(tcell.ColorWhite)
			columns.SetTextColor(tcell.ColorWhite)
			constraints.SetTextColor(tcell.ColorWhite)
			foreignKeys.SetTextColor(tcell.ColorWhite)
			App.SetFocus(dbIndexesTable)
			dbTablePages.SwitchToPage("indexes")
		}

		return event
	})

	dbOptions.AddItem(records, 15, 0, false)
	dbOptions.AddItem(columns, 15, 0, false)
	dbOptions.AddItem(constraints, 19, 0, false)
	dbOptions.AddItem(foreignKeys, 19, 0, false)
	dbOptions.AddItem(indexes, 16, 0, false)

	dbRecordsTable.SetSelectionChangedFunc(func(row, col int) {
		selectedRow, _ := dbRecordsTable.GetSelection()
		rowCount := dbRecordsTable.GetRowCount()

		if selectedRow == rowCount-1 {
			data := drivers.Database.GetTableData(currentTable, rowCount, dbTableRowLimit, false)
			utils.AddTableRows(DBRecordsTable, data)

		}
	})

	wrapper.AddItem(dbOptions, 3, 0, false)

	dbTablePages.AddPage("records", dbRecordsTable, true, true)
	dbTablePages.AddPage("columns", dbColumnsTable, true, false)
	dbTablePages.AddPage("constraints", dbConstraintsTable, true, false)
	dbTablePages.AddPage("foreignKeys", dbForeignKeysTable, true, false)
	dbTablePages.AddPage("indexes", dbIndexesTable, true, false)

	wrapper.AddItem(dbTablePages, 0, 1, true)

	return wrapper
}

func updateTableRows(data [][]string) {
	DBRecordsTable.Clear()

	for x, row := range data {
		for y, column := range row {
			cell := tview.NewTableCell(column)
			cell.SetReference(column)
			cell.SetSelectable(x > 0)
			cell.SetExpansion(1)

			if x == 0 {
				cell.SetTextColor(tcell.ColorKhaki)
			}

			DBRecordsTable.SetCell(x, y, cell)
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
						for i, filteredTable := range FilteredTables {
							if filteredTable.database == db {
								found = true
								FilteredTables[i].tables = append(FilteredTables[i].tables, tableName)
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
