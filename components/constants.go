package components

import (
	"github.com/gdamore/tcell/v2"

	"github.com/jorgerojas26/lazysql/app"
)

var App = app.App

// Pages
const (
	// General
	pageNameHelp         string = "Help"
	pageNameConfirmation string = "Confirmation"
	pageNameConnections  string = "Connections"
	pageNameDMLPreview   string = "DMLPreview"

	// Results table
	pageNameTable                  string = "Table"
	pageNameTableError             string = "TableError"
	pageNameTableLoading           string = "TableLoading"
	pageNameTableEditorTable       string = "TableEditorTable"
	pageNameTableEditorResultsInfo string = "TableEditorResultsInfo"
	pageNameTableEditCell          string = "TableEditCell"
	pageNameQueryPreviewError      string = "QueryPreviewError"

	// Sidebar
	pageNameSidebar string = "Sidebar"

	// Connections
	pageNameConnectionSelection string = "ConnectionSelection"
	pageNameConnectionForm      string = "ConnectionForm"

	// SetValueList
	pageNameSetValue string = "SetValue"
)

// Tabs
const (
	tabNameEditor string = "Editor"
)

// Events
const (
	eventSidebarEditing       string = "EditingSidebar"
	eventSidebarUnfocusing    string = "UnfocusingSidebar"
	eventSidebarToggling      string = "TogglingSidebar"
	eventSidebarCommitEditing string = "CommitEditingSidebar"
	eventSidebarError         string = "ErrorSidebar"

	eventSQLEditorQuery  string = "Query"
	eventSQLEditorEscape string = "Escape"

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

	colorTableChange = tcell.ColorOrange
	colorTableInsert = tcell.ColorDarkGreen
	colorTableDelete = tcell.ColorRed
)
