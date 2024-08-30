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
//
// A map can have several groups. But it always has a "Global" one.
type KeymapSystem struct {
	Global Map
	Groups map[string]Map
}

func (c KeymapSystem) Group(name string) Map {
	// Global is special.
	if name == "global" {
		return c.Global
	}

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

// Define a global KeymapSystem object with default keybinds
var Keymaps = KeymapSystem{
	Global: Map{
		Bind{Key: Key{Char: 'L'}, Cmd: cmd.MoveRight, Description: "Focus table"},
		Bind{Key: Key{Char: 'H'}, Cmd: cmd.MoveLeft, Description: "Focus tree"},
		Bind{Key: Key{Code: tcell.KeyCtrlE}, Cmd: cmd.SwitchToEditorView, Description: "Open SQL editor"},
		Bind{Key: Key{Code: tcell.KeyCtrlS}, Cmd: cmd.Save, Description: "Execute pending changes"},
		Bind{Key: Key{Char: 'q'}, Cmd: cmd.Quit, Description: "Quit"},
		Bind{Key: Key{Code: tcell.KeyBackspace2}, Cmd: cmd.SwitchToConnectionsView, Description: "Switch to connections list"},
		Bind{Key: Key{Char: '?'}, Cmd: cmd.HelpPopup, Description: "Help"},
	},
	Groups: map[string]Map{
		"tree": {
			Bind{Key: Key{Char: 'g'}, Cmd: cmd.GotoTop, Description: "Go to top"},
			Bind{Key: Key{Char: 'G'}, Cmd: cmd.GotoBottom, Description: "Go to bottom"},
			Bind{Key: Key{Code: tcell.KeyEnter}, Cmd: cmd.Execute, Description: "Open"},
			Bind{Key: Key{Char: 'j'}, Cmd: cmd.MoveDown, Description: "Go down"},
			Bind{Key: Key{Code: tcell.KeyDown}, Cmd: cmd.MoveDown, Description: "Go down"},
			Bind{Key: Key{Char: 'k'}, Cmd: cmd.MoveUp, Description: "Go up"},
			Bind{Key: Key{Code: tcell.KeyUp}, Cmd: cmd.MoveUp, Description: "Go up"},
		},
		"table": {
			Bind{Key: Key{Char: '/'}, Cmd: cmd.Search, Description: "Search"},
			Bind{Key: Key{Char: 'c'}, Cmd: cmd.Edit, Description: "Change cell"},
			Bind{Key: Key{Char: 'd'}, Cmd: cmd.Delete, Description: "Delete row"},
			Bind{Key: Key{Char: 'w'}, Cmd: cmd.GotoNext, Description: "Go to next cell"},
			Bind{Key: Key{Char: 'b'}, Cmd: cmd.GotoPrev, Description: "Go to previous cell"},
			Bind{Key: Key{Char: '$'}, Cmd: cmd.GotoEnd, Description: "Go to last cell"},
			Bind{Key: Key{Char: '0'}, Cmd: cmd.GotoStart, Description: "Go to first cell"},
			Bind{Key: Key{Char: 'y'}, Cmd: cmd.Copy, Description: "Copy cell to clipboard"},
			Bind{Key: Key{Char: 'o'}, Cmd: cmd.AppendNewRow, Description: "Append new row"},
			// Tabs
			Bind{Key: Key{Char: '['}, Cmd: cmd.TabPrev, Description: "Switch to previous tab"},
			Bind{Key: Key{Char: ']'}, Cmd: cmd.TabNext, Description: "Switch to next tab"},
			Bind{Key: Key{Char: '{'}, Cmd: cmd.TabFirst, Description: "Switch to first tab"},
			Bind{Key: Key{Char: '}'}, Cmd: cmd.TabLast, Description: "Switch to last tab"},
			Bind{Key: Key{Char: 'X'}, Cmd: cmd.TabClose, Description: "Close tab"},
			// Pages
			Bind{Key: Key{Char: '>'}, Cmd: cmd.PageNext, Description: "Switch to next page"},
			Bind{Key: Key{Char: '<'}, Cmd: cmd.PagePrev, Description: "Switch to previous page"},
		},
		"editor": {
			Bind{Key: Key{Code: tcell.KeyCtrlR}, Cmd: cmd.Execute, Description: "Execute query"},
			Bind{Key: Key{Code: tcell.KeyEscape}, Cmd: cmd.Quit, Description: "Unfocus editor"},
			Bind{Key: Key{Code: tcell.KeyCtrlSpace}, Cmd: cmd.OpenInExternalEditor, Description: "Open in external editor"},
		},
	},
}
