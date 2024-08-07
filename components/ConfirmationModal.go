package components

import (
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
	modal.AddButtons([]string{"Yes", "No"})
	modal.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	modal.SetTextColor(tview.Styles.PrimaryTextColor)

	return &ConfirmationModal{
		Modal: modal,
	}
}
