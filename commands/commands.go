package commands

type Command uint8

const (
	Noop Command = iota

	// Views
	SwitchToEditorView
	SwitchToConnectionsView
	HelpPopup
	ToggleQueryHistory

	// Movement: Basic
	MoveUp
	MoveDown
	MoveLeft
	MoveRight
	// Movement: Jumps
	GotoNext
	GotoPrev
	GotoStart
	GotoEnd
	GotoTop
	GotoBottom

	// Movement: Page
	PageNext
	PagePrev

	// Menu:
	RecordsMenu
	ColumnsMenu
	ConstraintsMenu
	ForeignKeysMenu
	IndexesMenu

	// Tabs
	TabNext
	TabPrev
	TabFirst
	TabLast
	TabClose

	// Operations
	Refresh
	UnfocusEditor
	Copy
	Edit
	CommitEdit
	DiscardEdit
	Save
	Delete
	Search
	SearchGlobal
	Quit
	Execute
	OpenInExternalEditor
	AppendNewRow
	DuplicateRow
	SortAsc
	SortDesc
	UnfocusTreeFilter
	CommitTreeFilter
	NextFoundNode
	PreviousFoundNode
	TreeCollapseAll
	ExpandAll
	SetValue
	FocusSidebar
	UnfocusSidebar
	ToggleSidebar
	ShowRowJSONViewer
	ShowCellJSONViewer

	// Connection
	NewConnection
	Connect
	TestConnection
	EditConnection
	DeleteConnection
)

func (c Command) String() string {
	switch c {
	case Noop:
		return "Noop"
	// Views
	case SwitchToEditorView:
		return "SwitchToEditorView"
	case SwitchToConnectionsView:
		return "SwitchToConnectionsView"
	case HelpPopup:
		return "HelpPopup"
	case ToggleQueryHistory:
		return "ToggleQueryHistory"

	// Movement: Basic
	case MoveUp:
		return "MoveUp"
	case MoveDown:
		return "MoveDown"
	case MoveLeft:
		return "MoveRight"
	case MoveRight:
		return "MoveRight"
	// Movement: Jumps
	case GotoNext:
		return "GotoNext"
	case GotoPrev:
		return "GotoPrev"
	case GotoStart:
		return "GotoStart"
	case GotoEnd:
		return "GotoEnd"
	case GotoTop:
		return "GotoTop"
	case GotoBottom:
		return "GotoBottom"

	// Movement: Page
	case PageNext:
		return "PageNext"
	case PagePrev:
		return "PagePrev"

	// Tabs
	case TabNext:
		return "TabNext"
	case TabPrev:
		return "TabPrev"
	case TabFirst:
		return "TabFirst"
	case TabLast:
		return "TabLast"
	case TabClose:
		return "TabClose"

	// Operations
	case Copy:
		return "Copy"
	case Edit:
		return "Edit"
	case Save:
		return "Save"
	case Delete:
		return "Delete"
	case Search:
		return "Search"
	case SearchGlobal:
		return "SearchGlobal"
	case Quit:
		return "Quit"
	case Execute:
		return "Execute"
	case OpenInExternalEditor:
		return "OpenInExternalEditor"
	case AppendNewRow:
		return "AppendNewRow"
	case DuplicateRow:
		return "DuplicateRow"
	case SortAsc:
		return "SortAsc"
	case SortDesc:
		return "SortDesc"
	case NewConnection:
		return "NewConnection"
	case Connect:
		return "Connect"
	case TestConnection:
		return "TestConnection"
	case EditConnection:
		return "EditConnection"
	case DeleteConnection:
		return "DeleteConnection"
	case Refresh:
		return "Refresh"
	case UnfocusEditor:
		return "UnfocusEditor"
	case RecordsMenu:
		return "RecordsMenu"
	case ColumnsMenu:
		return "ColumnsMenu"
	case ConstraintsMenu:
		return "ConstraintsMenu"
	case ForeignKeysMenu:
		return "ForeignKeysMenu"
	case IndexesMenu:
		return "IndexesMenu"
	case UnfocusTreeFilter:
		return "UnfocusTreeFilter"
	case CommitTreeFilter:
		return "CommitTreeFilter"
	case NextFoundNode:
		return "NextFoundNode"
	case PreviousFoundNode:
		return "PreviousFoundNode"
	case TreeCollapseAll:
		return "TreeCollapseAll"
	case ExpandAll:
		return "ExpandAll"
	case SetValue:
		return "SetValue"
	case FocusSidebar:
		return "FocusSidebar"
	case ToggleSidebar:
		return "ToggleSidebar"
	case UnfocusSidebar:
		return "UnfocusSidebar"
	case CommitEdit:
		return "CommitEdit"
	case DiscardEdit:
		return "DiscardEdit"
	case ShowRowJSONViewer:
		return "ShowRowJSONViewer"
	case ShowCellJSONViewer:
		return "ShowCellJSONViewer"
	}

	return "Unknown"
}
