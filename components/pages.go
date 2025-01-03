package components

import (
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

var MainPages = tview.NewPages()

func init() {
	MainPages.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	MainPages.AddPage(pageNameConnections, NewConnectionPages().Flex, true, true)
}
