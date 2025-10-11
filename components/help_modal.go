package components

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/keymap"
)

type KeybindGroup struct {
	Group string
	Binds []keymap.Bind
}

type HelpModal struct {
	*tview.Flex
	Filter        *tview.InputField
	Table         *tview.Table
	Wrapper       *tview.Flex
	KeybindGroups []KeybindGroup
	LongestKey    string
}

func NewHelpModal() *HelpModal {
	modal := &HelpModal{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		Filter:  tview.NewInputField(),
		Table:   tview.NewTable(),
		Wrapper: tview.NewFlex(),
	}

	for group, binds := range app.Keymaps.Groups {
		modal.KeybindGroups = append(modal.KeybindGroups, KeybindGroup{
			Group: group,
			Binds: binds,
		})
	}

	for _, keybindGroup := range modal.KeybindGroups {
		for _, bind := range keybindGroup.Binds {
			if len(bind.Key.String()) > len(modal.LongestKey) {
				modal.LongestKey = bind.Key.String()
			}
		}
	}

	modal.Table.SetBorder(true)
	modal.Table.SetBorderColor(app.Styles.PrimaryTextColor)
	modal.Table.SetTitle(" Keybindings ")
	modal.Table.SetSelectable(true, false)
	modal.Table.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))

	modal.Filter.SetLabel("Search keybinding: ")
	modal.Filter.SetFieldWidth(30)
	modal.Filter.SetFieldBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	modal.Filter.SetChangedFunc(func(text string) {
		modal.fillTable(text)
	})

	modal.Wrapper.SetDirection(tview.FlexRow)

	modal.AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexColumn).
				AddItem(nil, 0, 1, false).
				AddItem(modal.Wrapper, 0, 1, true).
				AddItem(nil, 0, 1, false),
			0, 3, true).
		AddItem(nil, 0, 1, false)

	modal.Filter.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			modal.showSearchBar(false)
		}
		if key == tcell.KeyEnter {
			app.App.SetFocus(modal.Table)
		}
	})

	modal.Table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == '/' {
			modal.showSearchBar(true)
			return nil
		}

		command := app.Keymaps.Group(app.HomeGroup).Resolve(event)

		if command == commands.Quit || command == commands.HelpPopup || event.Key() == tcell.KeyEsc {
			modal.showSearchBar(false)
			mainPages.RemovePage(pageNameHelp)
		}
		return event
	})

	modal.showSearchBar(false)

	return modal

}

func (modal *HelpModal) showSearchBar(show bool) {
	modal.Wrapper.Clear()
	if show {
		modal.Wrapper.
			AddItem(modal.Table, 0, 1, false).
			AddItem(modal.Filter, 1, 0, true)
		app.App.SetFocus(modal.Filter)
	} else {
		modal.Filter.SetText("")
		modal.fillTable("")
		modal.Wrapper.
			AddItem(modal.Table, 0, 1, true)
		app.App.SetFocus(modal.Table)
	}
}

func (modal *HelpModal) fillTable(filter string) {
	modal.Table.Clear()

	var filtered []KeybindGroup

	for _, group := range modal.KeybindGroups {
		var groupBinds []keymap.Bind
		for _, bind := range group.Binds {
			if filter != "" &&
				!strings.Contains(strings.ToLower(bind.Description), strings.ToLower(filter)) &&
				!strings.Contains(strings.ToLower(bind.Key.String()), strings.ToLower(filter)) {
				continue
			}
			groupBinds = append(groupBinds, bind)
		}
		if len(groupBinds) > 0 {
			filtered = append(filtered, KeybindGroup{
				Group: group.Group,
				Binds: groupBinds,
			})
		}
	}

	for _, group := range filtered {
		rowCount := modal.Table.GetRowCount()
		groupNameCell := tview.NewTableCell(strings.ToUpper(group.Group))
		groupNameCell.SetTextColor(app.Styles.TertiaryTextColor)
		groupNameCell.SetSelectable(rowCount == 0)

		modal.Table.SetCell(rowCount, 0, tview.NewTableCell("").SetSelectable(false))
		modal.Table.SetCell(rowCount+1, 0, groupNameCell)
		modal.Table.SetCell(rowCount+2, 0, tview.NewTableCell("").SetSelectable(false))

		for i, key := range group.Binds {
			keyText := key.Key.String()

			if len(keyText) < len(modal.LongestKey) {
				keyText = strings.Repeat(" ", len(modal.LongestKey)-len(keyText)) + keyText
			}
			modal.Table.SetCell(rowCount+3+i, 0, tview.NewTableCell(keyText).SetAlign(tview.AlignRight).SetTextColor(app.Styles.SecondaryTextColor))
			modal.Table.SetCell(rowCount+3+i, 1, tview.NewTableCell(key.Description).SetAlign(tview.AlignLeft).SetExpansion(1))
		}

	}
	modal.Table.ScrollToBeginning()
}
