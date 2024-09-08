package components

import "github.com/jorgerojas26/lazysql/app"

var App = app.App

// Pages
const (
	// General
	HelpPageName         string = "Help"
	ConfirmationPageName string = "Confirmation"
	ConnectionsPageName  string = "Connections"

	// Results table
	TablePageName                  string = "Table"
	TableErrorPageName             string = "TableError"
	TableLoadingPageName           string = "TableLoading"
	TableEditorTablePageName       string = "TableEditorTable"
	TableEditorResultsInfoPageName string = "TableEditorResultsInfo"
	TableEditCellPageName          string = "TableEditCell"

	// Sidebar
	SidebarPageName string = "Sidebar"

	// Connections
	ConnectionsSelectionPageName string = "ConnectionsSelection"
	ConnectionsFormPageName      string = "ConnectionsForm"
)

// Tabs
const (
	EditorTabName string = "Editor"
)

// Events
const (
	EditingSidebar    string = "EditingSidebar"
	UnfocusingSidebar string = "UnfocusingSidebar"
	TogglingSidebar   string = "TogglingSidebar"

	QuerySQLEditor  string = "Query"
	EscapeSQLEditor string = "Escape"

	FilteringResultsTable string = "FilteringResultsTable"

	SelectedDatabaseTree string = "SelectedDatabase"
	SelectedTableTree    string = "SelectedTable"
	IsFilteringTree      string = "IsFiltering"
)

// Results table menu items
const (
	RecordsMenu     string = "Records"
	ColumnsMenu     string = "Columns"
	ConstraintsMenu string = "Constraints"
	ForeignKeysMenu string = "Foreign Keys"
	IndexesMenu     string = "Indexes"
)

// Actions
const (
	NewConnection  string = "NewConnection"
	EditConnection string = "EditConnection"
)

// Misc (until i find a better name)
const (
	FocusedWrapperLeft  string = "left"
	FocusedWrapperRight string = "right"
)
