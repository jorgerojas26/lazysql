package components

import (
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/models"
)

func NewConnectionPages() *models.ConnectionPages {
	wrapper := tview.NewFlex()
	container := tview.NewFlex().SetDirection(tview.FlexColumnCSS)

	pages := tview.NewPages()

	wrapper.SetDirection(tview.FlexRowCSS)

	pages.SetBorder(true)

	container.AddItem(nil, 0, 1, false)
	container.AddItem(pages, 0, 1, true)
	container.AddItem(nil, 0, 1, false)

	wrapper.AddItem(nil, 0, 1, false)
	wrapper.AddItem(container, 0, 1, true)
	wrapper.AddItem(nil, 0, 1, false)

	cp := &models.ConnectionPages{
		Flex:  wrapper,
		Pages: pages,
	}

	connectionForm := NewConnectionForm(cp)
	connectionSelection := NewConnectionSelection(connectionForm, cp)

	cp.AddPage(ConnectionsSelectionPageName, connectionSelection.Flex, true, true)
	cp.AddPage(ConnectionsFormPageName, connectionForm.Flex, true, false)

	return cp
}
