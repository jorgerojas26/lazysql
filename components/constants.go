package components

import "github.com/jorgerojas26/lazysql/app"

var App = app.App

// Pages
const (
	// General
	pageNameHelp         string = "Help"
	pageNameConfirmation string = "Confirmation"
	pageNameConnections  string = "Connections"

	// Results table
	pageNameTable                  string = "Table"
	pageNameTableError             string = "TableError"
	pageNameTableLoading           string = "TableLoading"
	pageNameTableEditorTable       string = "TableEditorTable"
	pageNameTableEditorResultsInfo string = "TableEditorResultsInfo"
	pageNameTableEditCell          string = "TableEditCell"

	// Sidebar
	pageNameSidebar string = "Sidebar"

	// Connections
	pageNameConnectionSelection string = "ConnectionSelection"
	pageNameConnectionForm      string = "ConnectionForm"
)

// Tabs
const (
	tabNameEditor string = "Editor"
)

// Events
const (
	eventSidebarEditing    string = "EditingSidebar"
	eventSidebarUnfocusing string = "UnfocusingSidebar"
	eventSidebarToggling   string = "TogglingSidebar"

	eventSqlEditorQuery  string = "Query"
	eventSqlEditorEscape string = "Escape"

	eventResultsTableFiltering string = "FilteringResultsTable"

	eventTreeSelectedDatabase string = "SelectedDatabase"
	eventTreeSelectedTable    string = "SelectedTable"
	eventTreeIsFiltering      string = "IsFiltering"
)

// Results table menu items
const (
	menuRecords     string = "Records"
	menuColumns     string = "Columns"
	menuConstraints string = "Constraints"
	menuForeignKeys string = "Foreign Keys"
	menuIndexes     string = "Indexes"
)

// Actions
const (
	actionNewConnection  string = "NewConnection"
	actionEditConnection string = "EditConnection"
)

// Misc (until i find a better name)
const (
	focusedWrapperLeft  string = "left"
	focusedWrapperRight string = "right"
)
