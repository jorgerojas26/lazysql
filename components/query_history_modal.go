package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/models"
)

// QueryHistoryModal displays the query history for the current connection.
type QueryHistoryModal struct {
	tview.Primitive
	tabbedPane            *TabbedPane
	savedQueriesComponent *SavedQueriesComponent
	queryHistoryComponent *QueryHistoryComponent
	onQuerySelected       func(query string)
	grid                  *tview.Grid // The main modal grid container for centering
}

// NewQueryHistoryModal creates a new modal to display query history.
func NewQueryHistoryModal(history []models.QueryHistoryItem, onSelect func(query string)) *QueryHistoryModal {
	qhm := &QueryHistoryModal{
		onQuerySelected: onSelect,
	}

	qhm.savedQueriesComponent = NewSavedQueriesComponent(onSelect)
	qhm.queryHistoryComponent = NewQueryHistoryComponent(history, onSelect)

	qhm.tabbedPane = NewTabbedPane(func(tab *Tab) {})
	qhm.tabbedPane.AppendTab("Saved Queries", qhm.savedQueriesComponent, "saved_queries")
	qhm.tabbedPane.AppendTab("History", qhm.queryHistoryComponent, "history")

	qhm.tabbedPane.SetCurrentTab(qhm.tabbedPane.GetTabByReference("saved_queries"))

	tabbedPaneContainer := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(qhm.tabbedPane.HeaderContainer, 1, 0, false).
		AddItem(qhm.tabbedPane.Pages, 0, 1, true)
	tabbedPaneContainer.SetBorder(true)

	// Create a grid for both small and large screens
	smallScreenGrid := tview.NewGrid().
		SetRows(1, 0, 1).    // Top, center, and bottom rows (center is flexible)
		SetColumns(1, 0, 1). // Left, center, and right columns (center is flexible)
		SetMinSize(1, 1)     // Minimum cell size

	// Add pages to the small screen grid (with small margins)
	smallScreenGrid.AddItem(tabbedPaneContainer, 0, 0, 3, 3, 0, 0, true)

	// Create a grid specifically for large screens with a more compact center box
	largeScreenGrid := tview.NewGrid().
		SetRows(0, 20, 0).     // Top margin, fixed center height, bottom margin
		SetColumns(0, 150, 0). // Left margin, fixed center width, right margin
		SetMinSize(1, 1)       // Minimum cell size

	// Add pages to the center of large screen grid
	largeScreenGrid.AddItem(tabbedPaneContainer, 1, 1, 1, 1, 0, 0, true)

	// Create a responsive grid that switches between small and large layouts
	mainGrid := tview.NewGrid().
		SetRows(0).
		SetColumns(0)

	// Add the small screen layout as default
	mainGrid.AddItem(smallScreenGrid, 0, 0, 1, 1, 0, 0, true)

	// Add the large screen layout for screens with width > 100
	mainGrid.AddItem(largeScreenGrid, 0, 0, 1, 1, 0, 100, true)

	qhm.grid = mainGrid
	qhm.Primitive = qhm.grid

	qhm.grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			mainPages.RemovePage(pageNameQueryHistory)
			return nil
		}

		if event.Rune() == 's' {
			qhm.showSaveQueryModal()
			return nil
		}

		command := app.Keymaps.Group(app.QueryHistoryGroup).Resolve(event)
		if command == commands.ToggleQueryHistory {
			mainPages.RemovePage(pageNameQueryHistory)
			return nil
		}

		return event
	})

	return qhm
}

func (qhm *QueryHistoryModal) showSaveQueryModal() {
	var selectedQuery string

	tab := qhm.tabbedPane.GetCurrentTab()

	if tab.Reference == "saved_queries" {
		row, _ := qhm.savedQueriesComponent.table.GetSelection()
		if row > 0 && row-1 < len(qhm.savedQueriesComponent.displayedQueries) {
			selectedQuery = qhm.savedQueriesComponent.displayedQueries[row-1].Query
		}
	} else {
		row, _ := qhm.queryHistoryComponent.table.GetSelection()
		if row > 0 && row-1 < len(qhm.queryHistoryComponent.displayedHistory) {
			selectedQuery = qhm.queryHistoryComponent.displayedHistory[row-1].QueryText
		}
	}

	if selectedQuery != "" {
		saveModal := NewSaveQueryModal(selectedQuery, func() {
			qhm.savedQueriesComponent.Refresh()
			mainPages.RemovePage(pageNameSaveQuery)
			App.SetFocus(qhm.grid)
		})
		mainPages.AddPage(pageNameSaveQuery, saveModal, true, true)
		App.SetFocus(saveModal.form.GetFormItem(0))
	}
}

// GetPrimitive returns the top-level tview.Primitive for this modal.
func (qhm *QueryHistoryModal) GetPrimitive() tview.Primitive {
	return qhm.Primitive
}
