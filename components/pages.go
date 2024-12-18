package components

import (
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

var mainPages *tview.Pages

func MainPages() *tview.Pages {
	mainPages = tview.NewPages()
	mainPages.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	mainPages.AddPage(pageNameConnections, NewConnectionPages().Flex, true, true)
	return mainPages
}
