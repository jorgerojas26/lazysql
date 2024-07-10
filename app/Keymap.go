package app

import (
	"github.com/gdamore/tcell/v2"
	. "github.com/jorgerojas26/lazysql/commands"
	. "github.com/jorgerojas26/lazysql/keymap"
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
func (c KeymapSystem) Resolve(event *tcell.EventKey) Command {
	return c.Global.Resolve(event)
}

// Define a global KeymapSystem object with default keybinds
var Keymaps KeymapSystem = KeymapSystem{
	Global: Map{
		Bind{Key: Key{Char: 'L'}, Cmd: MoveRight, Description: "Right"},
		Bind{Key: Key{Char: 'H'}, Cmd: MoveLeft, Description: "Left"},
		Bind{Key: Key{Code: tcell.KeyCtrlE}, Cmd: SwitchToEditorView, Description: "Open SQL editor"},
		Bind{Key: Key{Code: tcell.KeyCtrlS}, Cmd: Save, Description: "Commit changes"},
		Bind{Key: Key{Char: 'q'}, Cmd: Quit, Description: "Quit"},
		Bind{Key: Key{Code: tcell.KeyBackspace2}, Cmd: SwitchToConnectionsView, Description: "Return to connection selection"},
		Bind{Key: Key{Char: '?'}, Cmd: OpenKeymapMenu, Description: "Open Keymap/Help Menu"},
	},
	Groups: map[string]Map{
		"tree": {
			Bind{Key: Key{Char: 'g'}, Cmd: GotoTop, Description: "Goto Top"},
			Bind{Key: Key{Char: 'G'}, Cmd: GotoBottom, Description: "Goto Bottom"},
			Bind{Key: Key{Code: tcell.KeyEnter}, Cmd: Execute, Description: "Enter"},
			Bind{Key: Key{Char: 'j'}, Cmd: MoveDown, Description: "Down"},
			Bind{Key: Key{Code: tcell.KeyDown}, Cmd: MoveDown, Description: "Down"},
			Bind{Key: Key{Char: 'k'}, Cmd: MoveUp, Description: "Up"},
			Bind{Key: Key{Code: tcell.KeyUp}, Cmd: MoveUp, Description: "Up"},
		},
		"table": {
			Bind{Key: Key{Char: '/'}, Cmd: Search, Description: "Focus the filter input or SQL editor"},
			Bind{Key: Key{Char: 'c'}, Cmd: Edit, Description: "Edit table cell"},
			Bind{Key: Key{Char: 'd'}, Cmd: Delete, Description: "Delete row"},
			Bind{Key: Key{Char: 'w'}, Cmd: GotoNext, Description: "Goto Next"},
			Bind{Key: Key{Char: 'b'}, Cmd: GotoPrev, Description: "Goto Prev"},
			Bind{Key: Key{Char: '$'}, Cmd: GotoEnd, Description: "Goto End"},
			Bind{Key: Key{Char: '0'}, Cmd: GotoStart, Description: "Goto Start"},
			Bind{Key: Key{Char: 'y'}, Cmd: Copy, Description: "Copy"},
			Bind{Key: Key{Char: 'o'}, Cmd: AppendNewRow, Description: "Add row"},
			// Tabs
			Bind{Key: Key{Char: '['}, Cmd: TabPrev, Description: "Focus previous tab"},
			Bind{Key: Key{Char: ']'}, Cmd: TabNext, Description: "Focus next tab"},
			Bind{Key: Key{Char: '{'}, Cmd: TabFirst, Description: "First Tab"},
			Bind{Key: Key{Char: '}'}, Cmd: TabLast, Description: "Last Tab"},
			Bind{Key: Key{Char: 'X'}, Cmd: TabClose, Description: "Close current tab"},
			// Pages
			Bind{Key: Key{Char: '>'}, Cmd: PageNext, Description: "Next Page"},
			Bind{Key: Key{Char: '<'}, Cmd: PagePrev, Description: "Previous page"},
		},
		"editor": {
			Bind{Key: Key{Code: tcell.KeyCtrlR}, Cmd: Execute, Description: "Run Sql Statment"},
			Bind{Key: Key{Code: tcell.KeyEscape}, Cmd: Quit, Description: "Quit"},
			Bind{Key: Key{Code: tcell.KeyCtrlSpace}, Cmd: OpenInExternalEditor, Description: "Open extranl editer(Linux Only)"},
		},
	},
}
