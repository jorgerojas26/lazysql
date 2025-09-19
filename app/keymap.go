package app

import (
	"github.com/gdamore/tcell/v2"

	cmd "github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/keymap"
)

// local alias added for clarity purpose
type (
	Bind = keymap.Bind
	Key  = keymap.Key
	Map  = keymap.Map
)

// KeymapSystem is the actual key mapping system.
// A map can have several groups. But it always has a "Global" one.
type KeymapSystem struct {
	Groups map[string]Map
	Global Map
}

func (c KeymapSystem) Group(name string) Map {
	// Lookup the group
	if group, ok := c.Groups[name]; ok {
		return group
	}

	// Did not find any maps. Return a empty one
	return Map{}
}

// Resolve translates a tcell.EventKey into a command based on the mappings in
// the global group
func (c KeymapSystem) Resolve(event *tcell.EventKey) cmd.Command {
	return c.Global.Resolve(event)
}

const (
	HomeGroup         = "home"
	TreeGroup         = "tree"
	TreeFilterGroup   = "treefilter"
	TableGroup        = "table"
	EditorGroup       = "editor"
	ConnectionGroup   = "connection"
	SidebarGroup      = "sidebar"
	QueryPreviewGroup = "querypreview"
	QueryHistoryGroup = "queryhistory"
	TabbedMenuGroup   = "tabbedmenu"
)

// Define a global KeymapSystem object with default keybinds
var Keymaps = KeymapSystem{
	Groups: map[string]Map{
		HomeGroup: {
			Bind{Key: Key{Char: 'L'}, Cmd: cmd.MoveRight, Description: "Focus table"},
			Bind{Key: Key{Char: 'H'}, Cmd: cmd.MoveLeft, Description: "Focus tree"},
			Bind{Key: Key{Code: tcell.KeyCtrlE}, Cmd: cmd.SwitchToEditorView, Description: "Open SQL editor"},
			Bind{Key: Key{Code: tcell.KeyCtrlS}, Cmd: cmd.Save, Description: "Execute pending changes"},
			Bind{Key: Key{Char: 'q'}, Cmd: cmd.Quit, Description: "Quit"},
			Bind{Key: Key{Code: tcell.KeyBackspace2}, Cmd: cmd.SwitchToConnectionsView, Description: "Switch to connections list"},
			Bind{Key: Key{Char: '?'}, Cmd: cmd.HelpPopup, Description: "Help"},
			Bind{Key: Key{Code: tcell.KeyCtrlP}, Cmd: cmd.SearchGlobal, Description: "Global search"},
			Bind{Key: Key{Code: tcell.KeyCtrlUnderscore}, Cmd: cmd.ToggleQueryHistory, Description: "Toggle query history modal"},
		},
		ConnectionGroup: {
			Bind{Key: Key{Char: 'n'}, Cmd: cmd.NewConnection, Description: "Create a new database connection"},
			Bind{Key: Key{Char: 'c'}, Cmd: cmd.Connect, Description: "Connect to database"},
			Bind{Key: Key{Code: tcell.KeyEnter}, Cmd: cmd.Connect, Description: "Connect to database"},
			Bind{Key: Key{Char: 'e'}, Cmd: cmd.EditConnection, Description: "Edit a database connection"},
			Bind{Key: Key{Char: 'd'}, Cmd: cmd.DeleteConnection, Description: "Delete a database connection"},
			Bind{Key: Key{Char: 'q'}, Cmd: cmd.Quit, Description: "Quit"},
		},
		TreeGroup: {
			Bind{Key: Key{Char: 'g'}, Cmd: cmd.GotoTop, Description: "Go to top"},
			Bind{Key: Key{Char: 'G'}, Cmd: cmd.GotoBottom, Description: "Go to bottom"},
			Bind{Key: Key{Code: tcell.KeyEnter}, Cmd: cmd.Execute, Description: "Open"},
			Bind{Key: Key{Char: 'j'}, Cmd: cmd.MoveDown, Description: "Go down"},
			Bind{Key: Key{Code: tcell.KeyDown}, Cmd: cmd.MoveDown, Description: "Go down"},
			Bind{Key: Key{Code: tcell.KeyCtrlU}, Cmd: cmd.PagePrev, Description: "Go page up"},
			Bind{Key: Key{Code: tcell.KeyCtrlD}, Cmd: cmd.PageNext, Description: "Go page down"},
			Bind{Key: Key{Char: 'k'}, Cmd: cmd.MoveUp, Description: "Go up"},
			Bind{Key: Key{Code: tcell.KeyUp}, Cmd: cmd.MoveUp, Description: "Go up"},
			Bind{Key: Key{Char: '/'}, Cmd: cmd.Search, Description: "Search"},
			Bind{Key: Key{Char: 'n'}, Cmd: cmd.NextFoundNode, Description: "Go to next found node"},
			Bind{Key: Key{Char: 'N'}, Cmd: cmd.PreviousFoundNode, Description: "Go to previous found node"},
			Bind{Key: Key{Char: 'p'}, Cmd: cmd.PreviousFoundNode, Description: "Go to previous found node"},
			Bind{Key: Key{Char: 'P'}, Cmd: cmd.NextFoundNode, Description: "Go to next found node"},
			Bind{Key: Key{Char: 'c'}, Cmd: cmd.TreeCollapseAll, Description: "Collapse all"},
			Bind{Key: Key{Char: 'e'}, Cmd: cmd.ExpandAll, Description: "Expand all"},
			Bind{Key: Key{Char: 'R'}, Cmd: cmd.Refresh, Description: "Refresh tree"},
		},
		TreeFilterGroup: {
			Bind{Key: Key{Code: tcell.KeyEscape}, Cmd: cmd.UnfocusTreeFilter, Description: "Unfocus tree filter"},
			Bind{Key: Key{Code: tcell.KeyEnter}, Cmd: cmd.CommitTreeFilter, Description: "Commit tree filter search"},
		},
		TableGroup: {
			Bind{Key: Key{Char: '/'}, Cmd: cmd.Search, Description: "Search"},
			Bind{Key: Key{Char: 'c'}, Cmd: cmd.Edit, Description: "Change cell"},
			Bind{Key: Key{Char: 'd'}, Cmd: cmd.Delete, Description: "Delete row"},
			Bind{Key: Key{Char: 'w'}, Cmd: cmd.GotoNext, Description: "Go to next cell"},
			Bind{Key: Key{Char: 'b'}, Cmd: cmd.GotoPrev, Description: "Go to previous cell"},
			Bind{Key: Key{Char: '$'}, Cmd: cmd.GotoEnd, Description: "Go to last cell"},
			Bind{Key: Key{Char: '0'}, Cmd: cmd.GotoStart, Description: "Go to first cell"},
			Bind{Key: Key{Char: 'y'}, Cmd: cmd.Copy, Description: "Copy cell value to clipboard"},
			Bind{Key: Key{Char: 'o'}, Cmd: cmd.AppendNewRow, Description: "Append new row"},
			Bind{Key: Key{Char: 'O'}, Cmd: cmd.DuplicateRow, Description: "Duplicate row"},
			Bind{Key: Key{Char: 'J'}, Cmd: cmd.SortDesc, Description: "Sort descending"},
			Bind{Key: Key{Char: 'R'}, Cmd: cmd.Refresh, Description: "Refresh the current table"},
			Bind{Key: Key{Char: 'K'}, Cmd: cmd.SortAsc, Description: "Sort ascending"},
			Bind{Key: Key{Char: 'C'}, Cmd: cmd.SetValue, Description: "Toggle value menu to put values like NULL, EMPTY or DEFAULT"},
			// Tabs
			Bind{Key: Key{Char: '['}, Cmd: cmd.TabPrev, Description: "Switch to previous tab"},
			Bind{Key: Key{Char: ']'}, Cmd: cmd.TabNext, Description: "Switch to next tab"},
			Bind{Key: Key{Char: '{'}, Cmd: cmd.TabFirst, Description: "Switch to first tab"},
			Bind{Key: Key{Char: '}'}, Cmd: cmd.TabLast, Description: "Switch to last tab"},
			Bind{Key: Key{Char: 'X'}, Cmd: cmd.TabClose, Description: "Close tab"},
			// Pages
			Bind{Key: Key{Char: '>'}, Cmd: cmd.PageNext, Description: "Switch to next page"},
			Bind{Key: Key{Char: '<'}, Cmd: cmd.PagePrev, Description: "Switch to previous page"},
			Bind{Key: Key{Char: '1'}, Cmd: cmd.RecordsMenu, Description: "Switch to records menu"},
			Bind{Key: Key{Char: '2'}, Cmd: cmd.ColumnsMenu, Description: "Switch to columns menu"},
			Bind{Key: Key{Char: '3'}, Cmd: cmd.ConstraintsMenu, Description: "Switch to constraints menu"},
			Bind{Key: Key{Char: '4'}, Cmd: cmd.ForeignKeysMenu, Description: "Switch to foreign keys menu"},
			Bind{Key: Key{Char: '5'}, Cmd: cmd.IndexesMenu, Description: "Switch to indexes menu"},
			// Sidebar
			Bind{Key: Key{Char: 'S'}, Cmd: cmd.ToggleSidebar, Description: "Toggle sidebar"},
			Bind{Key: Key{Char: 's'}, Cmd: cmd.FocusSidebar, Description: "Focus sidebar"},
		},
		EditorGroup: {
			Bind{Key: Key{Code: tcell.KeyCtrlR}, Cmd: cmd.Execute, Description: "Execute query"},
			Bind{Key: Key{Code: tcell.KeyEscape}, Cmd: cmd.UnfocusEditor, Description: "Unfocus editor"},
			Bind{Key: Key{Code: tcell.KeyCtrlSpace}, Cmd: cmd.OpenInExternalEditor, Description: "Open in external editor"},
		},
		SidebarGroup: {
			Bind{Key: Key{Char: 's'}, Cmd: cmd.UnfocusSidebar, Description: "Focus table"},
			Bind{Key: Key{Char: 'S'}, Cmd: cmd.ToggleSidebar, Description: "Toggle sidebar"},
			Bind{Key: Key{Char: 'j'}, Cmd: cmd.MoveDown, Description: "Focus next field"},
			Bind{Key: Key{Char: 'k'}, Cmd: cmd.MoveUp, Description: "Focus previous field"},
			Bind{Key: Key{Char: 'g'}, Cmd: cmd.GotoStart, Description: "Focus first field"},
			Bind{Key: Key{Char: 'G'}, Cmd: cmd.GotoEnd, Description: "Focus last field"},
			Bind{Key: Key{Char: 'c'}, Cmd: cmd.Edit, Description: "Edit field"},
			Bind{Key: Key{Code: tcell.KeyEnter}, Cmd: cmd.CommitEdit, Description: "Add edit to pending changes"},
			Bind{Key: Key{Code: tcell.KeyEscape}, Cmd: cmd.DiscardEdit, Description: "Discard edit"},
			Bind{Key: Key{Char: 'C'}, Cmd: cmd.SetValue, Description: "Toggle value menu to put values like NULL, EMPTY or DEFAULT"},
			Bind{Key: Key{Char: 'y'}, Cmd: cmd.Copy, Description: "Copy value to clipboard"},
		},
		QueryPreviewGroup: {
			Bind{Key: Key{Code: tcell.KeyCtrlS}, Cmd: cmd.Save, Description: "Execute queries"},
			Bind{Key: Key{Char: 'q'}, Cmd: cmd.Quit, Description: "Quit"},
			Bind{Key: Key{Char: 'y'}, Cmd: cmd.Copy, Description: "Copy query to clipboard"},
			Bind{Key: Key{Char: 'd'}, Cmd: cmd.Delete, Description: "Delete query"},
		},
		QueryHistoryGroup: {
			Bind{Key: Key{Char: 's'}, Cmd: cmd.Save, Description: "Save query"},
			Bind{Key: Key{Char: 'd'}, Cmd: cmd.Delete, Description: "Delete query"},
			Bind{Key: Key{Char: 'q'}, Cmd: cmd.Quit, Description: "Quit"},
			Bind{Key: Key{Char: 'y'}, Cmd: cmd.Copy, Description: "Copy query to clipboard"},
			Bind{Key: Key{Char: '/'}, Cmd: cmd.Search, Description: "Search"},
			Bind{Key: Key{Code: tcell.KeyCtrlUnderscore}, Cmd: cmd.ToggleQueryHistory, Description: "Toggle query history modal"},
			Bind{Key: Key{Char: '['}, Cmd: cmd.TabPrev, Description: "Switch to previous tab"},
			Bind{Key: Key{Char: ']'}, Cmd: cmd.TabNext, Description: "Switch to next tab"},
		},
	},
}
