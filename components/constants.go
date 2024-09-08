package components

import "github.com/jorgerojas26/lazysql/app"

var App = app.App

// Pages
const (
	// General
	helpPageName         string = "Help"
	confirmationPageName string = "Confirmation"
	connectionsPageName  string = "Connections"

	// Results table
	tablePageName                  string = "Table"
	tableErrorPageName             string = "TableError"
	tableLoadingPageName           string = "TableLoading"
	tableEditorTablePageName       string = "TableEditorTable"
	tableEditorResultsInfoPageName string = "TableEditorResultsInfo"
	tableEditCellPageName          string = "TableEditCell"

	// Sidebar
	sidebarPageName string = "Sidebar"

	// Connections
	connectionsSelectionPageName string = "ConnectionsSelection"
	connectionsFormPageName      string = "ConnectionsForm"
)

// Tabs
const (
	editorTabName string = "Editor"
)

// Events
const (
	editingSidebar    string = "EditingSidebar"
	unfocusingSidebar string = "UnfocusingSidebar"
	togglingSidebar   string = "TogglingSidebar"

	querySQLEditor  string = "Query"
	escapeSQLEditor string = "Escape"

	filteringResultsTable string = "FilteringResultsTable"

	selectedDatabaseTree string = "SelectedDatabase"
	selectedTableTree    string = "SelectedTable"
	isFilteringTree      string = "IsFiltering"
)

// Results table menu items
const (
	recordsMenu     string = "Records"
	columnsMenu     string = "Columns"
	constraintsMenu string = "Constraints"
	foreignKeysMenu string = "Foreign Keys"
	indexesMenu     string = "Indexes"
)

// Actions
const (
	newConnection  string = "NewConnection"
	editConnection string = "EditConnection"
)

// Misc (until i find a better name)
const (
	focusedWrapperLeft  string = "left"
	focusedWrapperRight string = "right"
)
