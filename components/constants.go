package components

import (
	"github.com/gdamore/tcell/v2"

	"github.com/jorgerojas26/lazysql/app"
)

var App = app.App

// Pages
const (
	// General
	pageNameHelp          string = "Help"
	pageNameConfirmation  string = "Confirmation"
	pageNameConnections   string = "Connections"
	pageNameDMLPreview    string = "DMLPreview"
	pageNameErrorModal    string = "ErrorModal"
	pageNameReadOnlyError string = "readOnlyError"

	// Results table
	pageNameTable                  string = "Table"
	pageNameTableError             string = "TableError"
	pageNameTableLoading           string = "TableLoading"
	pageNameTableEditorTable       string = "TableEditorTable"
	pageNameTableEditorResultsInfo string = "TableEditorResultsInfo"
	pageNameTableEditCell          string = "TableEditCell"
	pageNameQueryPreviewError      string = "QueryPreviewError"
	pageNameJSONViewer                    = "json_viewer"

	// Sidebar
	pageNameSidebar string = "Sidebar"

	// Connections
	pageNameConnectionSelection string = "ConnectionSelection"
	pageNameConnectionForm      string = "ConnectionForm"

	// SetValueList
	pageNameSetValue string = "SetValue"

	// Query History
	pageNameQueryHistory     string = "QueryHistoryModal"
	pageNameSaveQuery        string = "SaveQueryModal"
	pageNameSavedQueryDelete string = "SavedQueryDeleteModal"

	// CSV Export
	pageNameCSVExport        string = "CSVExportModal"
	pageNameCSVExportSuccess string = "CSVExportSuccessModal"
	pageNameCSVExportError   string = "CSVExportErrorModal"
)

// Tabs
const (
	tabNameEditor string = "Editor"

	savedQueryTabReference   string = "saved_queries"
	queryHistoryTabReference string = "query_history"
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

	eventTreeSelectedDatabase  string = "SelectedDatabase"
	eventTreeSelectedTable     string = "SelectedTable"
	eventTreeSelectedFunction  string = "SelectedFunction"
	eventTreeSelectedProcedure string = "SelectedProcedure"
	eventTreeSelectedView      string = "SelectedView"
	eventTreeIsFiltering       string = "IsFiltering"

	eventNoSQLTreeSelectedDatabase   string = "NoSQLSelectedDatabase"
	eventNoSQLTreeSelectedCollection string = "NoSQLSelectedCollection"
	eventNoSQLTreeIsFiltering        string = "NoSQLIsFiltering"
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
