package components

import (
	"github.com/rivo/tview"
)

var MainPages = tview.NewPages()

func init() {
	MainPages.AddPage("Connections", NewConnectionPages().Flex, true, true)
}
