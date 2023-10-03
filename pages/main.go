package pages

import (
	"github.com/rivo/tview"
)

var AllPages = tview.NewPages()

func init() {
	AllPages.AddPage("Connections", Connections, true, true)
}
