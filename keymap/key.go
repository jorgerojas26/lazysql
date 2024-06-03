package keymap

import "github.com/gdamore/tcell/v2"

// Key is a structure that represents a key that can be bound
// to an command
type Key struct {
	Code tcell.Key // Special character codes.
	Char rune      // used when the key represents a single ascii char like "a" or "2".
}

func (k Key) String() string {
	if k.Char != 0 {
		return string(k.Char)
	}

	if desc, ok := tcell.KeyNames[k.Code]; ok {
		return "<" + desc + ">"
	}
	return ""
}
