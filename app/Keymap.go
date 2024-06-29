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
		Bind{Key: Key{Char: 'L'}, Cmd: MoveRight},
		Bind{Key: Key{Char: 'H'}, Cmd: MoveLeft},
		Bind{Key: Key{Code: tcell.KeyCtrlE}, Cmd: SwitchToEditorView},
		Bind{Key: Key{Code: tcell.KeyCtrlS}, Cmd: Save},
		Bind{Key: Key{Char: 'q'}, Cmd: Quit},
		Bind{Key: Key{Code: tcell.KeyBackspace2}, Cmd: SwitchToConnectionsView},
	},
	Groups: map[string]Map{
		"tree": {
			Bind{Key: Key{Char: 'g'}, Cmd: GotoTop},
			Bind{Key: Key{Char: 'G'}, Cmd: GotoBottom},
			Bind{Key: Key{Code: tcell.KeyEnter}, Cmd: Execute},
			Bind{Key: Key{Char: 'j'}, Cmd: MoveDown},
			Bind{Key: Key{Code: tcell.KeyDown}, Cmd: MoveDown},
			Bind{Key: Key{Char: 'k'}, Cmd: MoveUp},
			Bind{Key: Key{Code: tcell.KeyUp}, Cmd: MoveUp},
		},
		"table": {
			Bind{Key: Key{Char: '/'}, Cmd: Search},
			Bind{Key: Key{Char: 'c'}, Cmd: Edit},
			Bind{Key: Key{Char: 'd'}, Cmd: Delete},
			Bind{Key: Key{Char: 'w'}, Cmd: GotoNext},
			Bind{Key: Key{Char: 'b'}, Cmd: GotoPrev},
			Bind{Key: Key{Char: '$'}, Cmd: GotoEnd},
			Bind{Key: Key{Char: '0'}, Cmd: GotoStart},
			Bind{Key: Key{Char: 'y'}, Cmd: Copy},
			Bind{Key: Key{Char: 'o'}, Cmd: AppendNewRow},
			// Tabs
			Bind{Key: Key{Char: '['}, Cmd: TabPrev},
			Bind{Key: Key{Char: ']'}, Cmd: TabNext},
			Bind{Key: Key{Char: '{'}, Cmd: TabFirst},
			Bind{Key: Key{Char: '}'}, Cmd: TabLast},
			Bind{Key: Key{Char: 'X'}, Cmd: TabClose},
			// Pages
			Bind{Key: Key{Char: '>'}, Cmd: PageNext},
			Bind{Key: Key{Char: '<'}, Cmd: PagePrev},
		},
		"editor": {
			Bind{Key: Key{Code: tcell.KeyCtrlR}, Cmd: Execute},
			Bind{Key: Key{Code: tcell.KeyEscape}, Cmd: Quit},
			Bind{Key: Key{Code: tcell.KeyCtrlSpace}, Cmd: OpenInExternalEditor},
		},
	},
}
