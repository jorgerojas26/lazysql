package components

import (
	"github.com/rivo/tview"
)

func NewErrorModal(message string) *tview.Modal {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"})
	return modal
}
