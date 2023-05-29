package pages

import (
	"github.com/rivo/tview"
)

var AllPages = tview.NewPages()
var App = tview.NewApplication()

func init() {
	AllPages.AddPage("Connections", Connections, true, true)

}
