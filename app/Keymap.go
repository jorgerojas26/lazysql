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
		Bind{Key: Key{Char: 'L'}, Cmd: cmd.MoveRight},
		Bind{Key: Key{Char: 'H'}, Cmd: cmd.MoveLeft},
		Bind{Key: Key{Code: tcell.KeyCtrlE}, Cmd: cmd.SwitchToEditorView},
		Bind{Key: Key{Code: tcell.KeyCtrlS}, Cmd: cmd.Save},
		Bind{Key: Key{Char: 'q'}, Cmd: cmd.Quit},
		Bind{Key: Key{Code: tcell.KeyBackspace2}, Cmd: cmd.SwitchToConnectionsView},
	},
	Groups: map[string]Map{
		"tree": {
			Bind{Key: Key{Char: 'g'}, Cmd: cmd.GotoTop},
			Bind{Key: Key{Char: 'G'}, Cmd: cmd.GotoBottom},
			Bind{Key: Key{Code: tcell.KeyEnter}, Cmd: cmd.Execute},
			Bind{Key: Key{Char: 'j'}, Cmd: cmd.MoveDown},
			Bind{Key: Key{Code: tcell.KeyDown}, Cmd: cmd.MoveDown},
			Bind{Key: Key{Char: 'k'}, Cmd: cmd.MoveUp},
			Bind{Key: Key{Code: tcell.KeyUp}, Cmd: cmd.MoveUp},
		},
		"table": {
			Bind{Key: Key{Char: '/'}, Cmd: cmd.Search},
			Bind{Key: Key{Char: 'c'}, Cmd: cmd.Edit},
			Bind{Key: Key{Char: 'd'}, Cmd: cmd.Delete},
			Bind{Key: Key{Char: 'w'}, Cmd: cmd.GotoNext},
			Bind{Key: Key{Char: 'b'}, Cmd: cmd.GotoPrev},
			Bind{Key: Key{Char: '$'}, Cmd: cmd.GotoEnd},
			Bind{Key: Key{Char: '0'}, Cmd: cmd.GotoStart},
			Bind{Key: Key{Char: 'y'}, Cmd: cmd.Copy},
			Bind{Key: Key{Char: 'o'}, Cmd: cmd.AppendNewRow},
			// Tabs
			Bind{Key: Key{Char: '['}, Cmd: cmd.TabPrev},
			Bind{Key: Key{Char: ']'}, Cmd: cmd.TabNext},
			Bind{Key: Key{Char: '{'}, Cmd: cmd.TabFirst},
			Bind{Key: Key{Char: '}'}, Cmd: cmd.TabLast},
			Bind{Key: Key{Char: 'X'}, Cmd: cmd.TabClose},
			// Pages
			Bind{Key: Key{Char: '>'}, Cmd: cmd.PageNext},
			Bind{Key: Key{Char: '<'}, Cmd: cmd.PagePrev},
		},
		"editor": {
			Bind{Key: Key{Code: tcell.KeyCtrlR}, Cmd: cmd.Execute},
			Bind{Key: Key{Code: tcell.KeyEscape}, Cmd: cmd.Quit},
			Bind{Key: Key{Code: tcell.KeyCtrlSpace}, Cmd: cmd.OpenInExternalEditor},
		},
	},
}
