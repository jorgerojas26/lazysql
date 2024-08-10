package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/keymap"
)

type HelpStatus struct {
	*tview.TextView
}

func NewHelpStatus() HelpStatus {

	status := HelpStatus{tview.NewTextView().SetTextColor(tcell.ColorBlue)}

	status.SetStatusOnTree()

	return status
}

func (status *HelpStatus) UpdateText(binds []keymap.Bind) {

	newtext := ""

	for i, key := range binds {

		newText += key.Cmd.String()

		newText += ": "

		newText += key.Key.String()

		islast := i == len(binds)-1

		if !islast {
			newText += " | "
		}

	}

	status.SetText(newText)
}

func (status *HelpStatus) SetStatusOnTree() {
	status.UpdateText(app.Keymaps.Global)
}
func (status *HelpStatus) SetStatusOnEditorView() {
	status.UpdateText(app.Keymaps.Group("editor"))
}
func (status *HelpStatus) SetStatusOnTableView() {
	status.UpdateText(app.Keymaps.Group("table"))
}
