package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type HelpModal struct {
	*tview.Modal
}

func NewHelpModal() *HelpModal {
	modal := tview.NewModal()
	modal.SetText("Help")

	modal.SetBackgroundColor(tcell.ColorBlack)
	return &HelpModal{Modal: modal}
}
