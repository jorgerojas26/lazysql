package components

import (
	"lazysql/app"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ConfirmationModal struct {
	*tview.Modal
}

func NewConfirmationModal() *ConfirmationModal {
	modal := tview.NewModal()
	modal.SetText("Are you sure?")
	modal.AddButtons([]string{"Yes", "No"})
	modal.SetBackgroundColor(tcell.ColorBlack)
	modal.SetTextColor(app.ActiveTextColor)

	return &ConfirmationModal{
		Modal: modal,
	}
}
