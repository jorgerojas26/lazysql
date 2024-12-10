package components

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
)

type HelpModal struct {
	tview.Primitive
}

func NewHelpModal() *HelpModal {
	// Returns a new primitive which puts the provided primitive in the center and
	// sets its size to the given width and height.
	modal := func(p tview.Primitive, width, height int) tview.Primitive {
		return tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(p, height, 1, true).
				AddItem(nil, 0, 1, false), width, 1, true).
			AddItem(nil, 0, 1, false)
	}

	table := tview.NewTable()

	// table.SetBorders(true)
	table.SetBorder(true)
	table.SetBorderColor(app.Styles.PrimaryTextColor)
	table.SetTitle(" Keybindings ")
	table.SetSelectable(true, false)
	table.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))

	keymapGroups := app.Keymaps.Groups

	mostLengthyKey := ""

	for groupName := range keymapGroups {
		for _, key := range keymapGroups[groupName] {
			if len(key.Key.String()) > len(mostLengthyKey) {
				mostLengthyKey = key.Key.String()
			}
		}
	}

	for groupName, keys := range keymapGroups {
		rowCount := table.GetRowCount()
		groupNameCell := tview.NewTableCell(strings.ToUpper(groupName))
		groupNameCell.SetTextColor(app.Styles.TertiaryTextColor)
		groupNameCell.SetSelectable(rowCount == 0)

		table.SetCell(rowCount, 0, tview.NewTableCell("").SetSelectable(false))
		table.SetCell(rowCount+1, 0, groupNameCell)
		table.SetCell(rowCount+2, 0, tview.NewTableCell("").SetSelectable(false))

		for i, key := range keys {
			keyText := key.Key.String()

			if len(keyText) < len(mostLengthyKey) {
				keyText = strings.Repeat(" ", len(mostLengthyKey)-len(keyText)) + keyText
			}
			table.SetCell(rowCount+3+i, 0, tview.NewTableCell(keyText).SetAlign(tview.AlignRight).SetTextColor(app.Styles.SecondaryTextColor))
			table.SetCell(rowCount+3+i, 1, tview.NewTableCell(key.Description).SetAlign(tview.AlignLeft).SetExpansion(1))
		}

	}

	table.Select(3, 0)

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := app.Keymaps.Group(app.HomeGroup).Resolve(event)
		if command == commands.Quit || command == commands.HelpPopup || event.Key() == tcell.KeyEsc {
			MainPages.RemovePage(pageNameHelp)
		}
		return event
	})

	r := &HelpModal{modal(table, 0, 30)}

	return r
}
