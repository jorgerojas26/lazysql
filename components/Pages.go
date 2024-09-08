package components

import (
	"github.com/rivo/tview"
)

var MainPages = tview.NewPages()

func init() {
	MainPages.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	MainPages.AddPage(pageNameConnections, NewConnectionPages().Flex, true, true)
}
