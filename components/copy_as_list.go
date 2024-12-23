package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/rivo/tview"
)

type CopyAsList struct {
	*tview.List
	table *ResultsTable
}

func NewCopyAsList(table *ResultsTable) *CopyAsList {
	list := &CopyAsList{
		List:  tview.NewList(),
		table: table,
	}

	list.SetBorder(true)
	list.SetTitle("Copy as...")
	list.SetTitleColor(app.Styles.PrimaryTextColor)
	list.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)

	// Add options
	list.AddItem("Copy as INSERT", "", 'i', nil)
	list.AddItem("Copy as UPDATE", "", 'u', nil)
	list.AddItem("Copy as SELECT", "", 's', nil)

	list.SetSelectedFunc(func(index int, _ string, _ string, _ rune) {
		list.Hide()
		switch index {
		case 0:
			if err := table.copyRowAsSQL("INSERT"); err != nil {
				table.SetError(err.Error(), nil)
			}
		case 1:
			if err := table.copyRowAsSQL("UPDATE"); err != nil {
				table.SetError(err.Error(), nil)
			}
		case 2:
			if err := table.copyRowAsSQL("SELECT"); err != nil {
				table.SetError(err.Error(), nil)
			}
		}
	})

	// Add shortcut key support
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := app.Keymaps.Group(app.CopyAsMenuGroup).Resolve(event)
		if command == commands.Quit {
			list.Hide()
			return nil
		}
		return event
	})

	return list
}

func (list *CopyAsList) Show(x, y, width int) {
	list.SetRect(x, y, width, 8) // Adjust height based on number of options
	MainPages.AddPage(pageNameCopyAs, list, false, true)
	App.SetFocus(list)
}

func (list *CopyAsList) Hide() {
	MainPages.RemovePage(pageNameCopyAs)
	App.SetFocus(list.table)
}
