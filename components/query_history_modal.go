package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
)

type QueryHistoryModal struct {
	tview.Primitive
	tabbedPane            *TabbedPane
	savedQueriesComponent *SavedQueriesComponent
	queryHistoryComponent *QueryHistoryComponent
	onQuerySelected       func(query string)
	connectionIdentifier  string
	grid                  *tview.Grid
}

func NewQueryHistoryModal(connectionIdentifier string, onSelect func(query string)) *QueryHistoryModal {
	qhm := &QueryHistoryModal{
		onQuerySelected:      onSelect,
		connectionIdentifier: connectionIdentifier,
	}

	qhm.savedQueriesComponent = NewSavedQueriesComponent(connectionIdentifier, func(query string) {
		mainPages.RemovePage(pageNameQueryHistory)
		onSelect(query)
	})
	qhm.queryHistoryComponent = NewQueryHistoryComponent(connectionIdentifier, func(query string) {
		mainPages.RemovePage(pageNameQueryHistory)
		onSelect(query)
	}, func() {
		qhm.savedQueriesComponent.Refresh()
		App.SetFocus(qhm.grid)
	})

	qhm.tabbedPane = NewTabbedPane()
	qhm.tabbedPane.AppendTab("Saved Queries", qhm.savedQueriesComponent, savedQueryTabReference)
	qhm.tabbedPane.AppendTab("History", qhm.queryHistoryComponent, queryHistoryTabReference)

	qhm.tabbedPane.SetCurrentTab(qhm.tabbedPane.GetTabByReference(savedQueryTabReference))

	tabbedPaneContainer := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(qhm.tabbedPane.HeaderContainer, 1, 0, false).
		AddItem(qhm.tabbedPane.Pages, 0, 1, true)

	frame := tview.NewFrame(tabbedPaneContainer)
	frame.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	frame.SetBorder(true)
	frame.SetBorders(0, 0, 0, 0, 0, 0)

	smallScreenGrid := tview.NewGrid().
		SetRows(1, 0, 1).
		SetColumns(1, 0, 1).
		SetMinSize(1, 1)

	smallScreenGrid.AddItem(frame, 0, 0, 3, 3, 0, 0, true)

	largeScreenGrid := tview.NewGrid().
		SetRows(0, 20, 0).
		SetColumns(0, 150, 0).
		SetMinSize(1, 1)

	largeScreenGrid.AddItem(frame, 1, 1, 1, 1, 0, 0, true)

	mainGrid := tview.NewGrid().
		SetRows(0).
		SetColumns(0)

	mainGrid.AddItem(smallScreenGrid, 0, 0, 1, 1, 0, 0, true)

	mainGrid.AddItem(largeScreenGrid, 0, 0, 1, 1, 0, 100, true)

	qhm.grid = mainGrid
	qhm.Primitive = qhm.grid

	qhm.grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc && !qhm.queryHistoryComponent.GetIsFiltering() && !qhm.savedQueriesComponent.GetIsFiltering() {
			mainPages.RemovePage(pageNameQueryHistory)
			return nil
		}

		command := app.Keymaps.Group(app.QueryHistoryGroup).Resolve(event)

		switch command {
		case commands.ToggleQueryHistory:
			mainPages.RemovePage(pageNameQueryHistory)
			return nil
		case commands.Quit:
			if !qhm.queryHistoryComponent.GetIsFiltering() && !qhm.savedQueriesComponent.GetIsFiltering() {
				mainPages.RemovePage(pageNameQueryHistory)
				return nil
			}
		case commands.TabPrev:
			qhm.tabbedPane.SwitchToPreviousTab()
			return nil
		case commands.TabNext:
			qhm.tabbedPane.SwitchToNextTab()
			return nil
		}

		return event
	})

	return qhm
}

func (qhm *QueryHistoryModal) GetPrimitive() tview.Primitive {
	return qhm.Primitive
}
