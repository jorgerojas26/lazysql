package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
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
	modal.AddButtons([]string{"Yes", "No"})
	modal.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	modal.SetButtonActivatedStyle(tcell.StyleDefault.
		Background(app.Styles.InverseTextColor).
		Foreground(app.Styles.ContrastSecondaryTextColor),
	)
	modal.SetTextColor(app.Styles.PrimaryTextColor)

	return &ConfirmationModal{
		Modal: modal,
	}
}
