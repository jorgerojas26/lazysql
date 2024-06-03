package keymap

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jorgerojas26/lazysql/commands"
)

// Map is a collection of keybinds
type Map []Bind

// Resolve translates a tcell.EventKey to a
// command based on the bindings in the map.
//
// If no binding could be found. commands.Noop is returned.
func (m Map) Resolve(event *tcell.EventKey) commands.Command {
	for _, bind := range m {
		if event.Key() == tcell.KeyRune {
			if bind.Key.Char == event.Rune() {
				return bind.Cmd
			}
		} else if event.Key() == bind.Key.Code {
			return bind.Cmd
		}
	}

	return commands.Noop
}
