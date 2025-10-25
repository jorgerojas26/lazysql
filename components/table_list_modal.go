package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/rivo/tview"
)

// TableListModal is a modal for selecting a table from the current database.
type TableListModal struct {
	*tview.Flex
	Tree    *Tree
	Wrapper *tview.Flex
}

// NewTableListModal creates a new TableListModal.
// Pass the database name, list of tables, and a callback for when a table is selected.
func NewTableListModal(tree *Tree) *TableListModal {
	modal := &TableListModal{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		Tree:    tree,
		Wrapper: tview.NewFlex(),
	}

	modal.Wrapper.SetDirection(tview.FlexRow)

	modal.AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexColumn).
				AddItem(nil, 0, 1, false).
				AddItem(modal.Tree.Wrapper, 0, 1, true).
				AddItem(nil, 0, 1, false),
			0, 3, true).
		AddItem(nil, 0, 1, false)

	modal.Tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := app.Keymaps.Group(app.HomeGroup).Resolve(event)

		if command == commands.Quit || command == commands.ToggleTableListModal || event.Key() == tcell.KeyEsc || command == commands.CommitTreeFilter {
			mainPages.RemovePage(pageNameTableListModal)
			modal.Tree.ClearSearch()
		}

		if event.Rune() == '/' {
			app.App.SetFocus(modal.Tree.Filter)
		}

		return event
	})

	return modal
}
