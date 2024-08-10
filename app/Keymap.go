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
		Bind{Key: Key{Char: 'L'}, Cmd: cmd.MoveRight, Description: "Right"},
		Bind{Key: Key{Char: 'H'}, Cmd: cmd.MoveLeft, Description: "Left"},
		Bind{Key: Key{Code: tcell.KeyCtrlE}, Cmd: cmd.SwitchToEditorView, Description: "EditorView"},
		Bind{Key: Key{Code: tcell.KeyCtrlS}, Cmd: cmd.Save, Description: "Save"},
		Bind{Key: Key{Char: 'q'}, Cmd: cmd.Quit, Description: "Quit"},
		Bind{Key: Key{Code: tcell.KeyBackspace2}, Cmd: cmd.SwitchToConnectionsView, Description: "ConnectionsView"},
		Bind{Key: Key{Char: '?'}, Cmd: cmd.HelpPopup, Description: "Help"},
	},
	Groups: map[string]Map{
		"tree": {
			Bind{Key: Key{Char: 'g'}, Cmd: cmd.GotoTop, Description: "Goto Top"},
			Bind{Key: Key{Char: 'G'}, Cmd: cmd.GotoBottom, Description: "Goto Bottom"},
			Bind{Key: Key{Code: tcell.KeyEnter}, Cmd: cmd.Execute, Description: "Execute"},
			Bind{Key: Key{Char: 'j'}, Cmd: cmd.MoveDown, Description: "Down"},
			Bind{Key: Key{Code: tcell.KeyDown}, Cmd: cmd.MoveDown, Description: "Down"},
			Bind{Key: Key{Char: 'k'}, Cmd: cmd.MoveUp, Description: "Up"},
			Bind{Key: Key{Code: tcell.KeyUp}, Cmd: cmd.MoveUp, Description: "Up"},
		},
		"table": {
			Bind{Key: Key{Char: '/'}, Cmd: cmd.Search, Description: "Search"},
			Bind{Key: Key{Char: 'c'}, Cmd: cmd.Edit, Description: "Goto Edit"},
			Bind{Key: Key{Char: 'd'}, Cmd: cmd.Delete, Description: "Goto Delete"},
			Bind{Key: Key{Char: 'w'}, Cmd: cmd.GotoNext, Description: "Goto Next"},
			Bind{Key: Key{Char: 'b'}, Cmd: cmd.GotoPrev, Description: "Goto Prev"},
			Bind{Key: Key{Char: '$'}, Cmd: cmd.GotoEnd, Description: "Goto End"},
			Bind{Key: Key{Char: '0'}, Cmd: cmd.GotoStart, Description: "Goto Start"},
			Bind{Key: Key{Char: 'y'}, Cmd: cmd.Copy, Description: "Copy"},
			Bind{Key: Key{Char: 'o'}, Cmd: cmd.AppendNewRow, Description: "New Row"},
			// Tabs
			Bind{Key: Key{Char: '['}, Cmd: cmd.TabPrev, Description: "Tab Prev"},
			Bind{Key: Key{Char: ']'}, Cmd: cmd.TabNext, Description: "Tab Next"},
			Bind{Key: Key{Char: '{'}, Cmd: cmd.TabFirst, Description: "Tab First"},
			Bind{Key: Key{Char: '}'}, Cmd: cmd.TabLast, Description: "Tab Last"},
			Bind{Key: Key{Char: 'X'}, Cmd: cmd.TabClose, Description: "Close"},
			// Pages
			Bind{Key: Key{Char: '>'}, Cmd: cmd.PageNext, Description: "Page Next"},
			Bind{Key: Key{Char: '<'}, Cmd: cmd.PagePrev, Description: "Page Prev"},
		},
		"editor": {
			Bind{Key: Key{Code: tcell.KeyCtrlR}, Cmd: cmd.Execute, Description: "Execute"},
			Bind{Key: Key{Code: tcell.KeyEscape}, Cmd: cmd.Quit, Description: "Quit"},
			Bind{Key: Key{Code: tcell.KeyCtrlSpace}, Cmd: cmd.OpenInExternalEditor, Description: "ExternalEditor"},
		},
	},
}
