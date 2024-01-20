package components

import (
	"github.com/jorgerojas26/lazysql/app"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ConfirmationModal struct {
	*tview.Modal
}

func NewConfirmationModal(confirmationText string) *ConfirmationModal {
	modal := tview.NewModal()
	if confirmationText != "" {
		modal.SetText(confirmationText)
	} else {
		modal.SetText("Are you sure?")
	}
	modal.AddButtons([]string{"[green]Enter [white]Yes", "[green]Esc [white]No"})
	modal.SetButtonBackgroundColor(tcell.ColorDefault)
	modal.SetBackgroundColor(tcell.ColorDefault)
	modal.SetButtonActivatedStyle(tcell.StyleDefault.Foreground(app.ActiveTextColor).Background(tcell.ColorDefault))
	modal.SetTextColor(app.ActiveTextColor)

	return &ConfirmationModal{
		Modal: modal,
	}
}
