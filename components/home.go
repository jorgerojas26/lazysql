package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

const (
	wrapperLeftSide  = "left"
	wrapperRightSide = "right"

	previousTabSwitch  = '['
	nextTabSwitch      = ']'
	firstTabSwitch     = '{'
	lastTabSwitch      = '}'
	removeTabSwitch    = 'X'
	previousPageSwitch = '<'
	nextPageSwitch     = '>'

	quitSwitch         = 'q'
	leftWrapperSwitch  = 'H'
	rightWrapperSwitch = 'L'
	editorSwitch       = 5
	connectionsSwitch  = 127
	confirmationSwitch = 19
)

type Home struct {
	*tview.Flex
	Tree            *Tree
	TabbedPane      *TabbedPane
	LeftWrapper     *tview.Flex
	RightWrapper    *tview.Flex
	DBDriver        drivers.MySQL
	FocusedWrapper  string
	ListOfDbChanges []models.DbDmlChange
	ListOfDbInserts []models.DbInsert
}

func NewHomePage(name string, dbdriver drivers.MySQL) *Home {
	tree := NewTree(&dbdriver)
	tabbedPane := NewTabbedPane()
	leftWrapper := tview.NewFlex()
	rightWrapper := tview.NewFlex()

	home := &Home{
		Flex:            tview.NewFlex(),
		Tree:            tree,
		TabbedPane:      tabbedPane,
		LeftWrapper:     leftWrapper,
		RightWrapper:    rightWrapper,
		ListOfDbChanges: []models.DbDmlChange{},
		ListOfDbInserts: []models.DbInsert{},
		DBDriver:        dbdriver,
	}

	go home.subscribeToTreeChanges()

	leftWrapper.SetBorderColor(app.InactiveTextColor)
	leftWrapper.AddItem(tree, 0, 1, true)

	rightWrapper.SetBorderColor(app.InactiveTextColor)
	rightWrapper.SetBorder(true)
	rightWrapper.SetDirection(tview.FlexColumnCSS)
	rightWrapper.SetInputCapture(home.rightWrapperInputCapture)
	rightWrapper.AddItem(tabbedPane.HeaderContainer, 1, 0, false)
	rightWrapper.AddItem(tabbedPane.Pages, 0, 1, false)

	home.AddItem(leftWrapper, 30, 1, false)
	home.AddItem(rightWrapper, 0, 5, false)

	home.SetInputCapture(home.homeInputCapture)

	home.SetFocusFunc(func() {
		if home.FocusedWrapper == wrapperLeftSide || home.FocusedWrapper == "" {
			home.focusLeftWrapper()
		} else {
			home.focusRightWrapper()
		}
	})

	MainPages.AddPage(name, home, true, false)

	return home
}

func (home *Home) subscribeToTreeChanges() {
	ch := home.Tree.Subscribe()

	for stateChange := range ch {
		if stateChange.Key != "SelectedTable" {
			continue
		}

		tableName := stateChange.Value.(string)
		tab := home.TabbedPane.GetTabByName(tableName)

		var table *ResultsTable

		if tab != nil {
			table = tab.Content
			home.TabbedPane.SwitchToTabByName(tab.Name)
		} else {
			table = NewResultsTable(&home.ListOfDbChanges, &home.ListOfDbInserts, home.Tree, &home.DBDriver).WithFilter()
			home.TabbedPane.AppendTab(tableName, table)
		}

		home.focusRightWrapper()
		table.FetchRecords(tableName)
		app.App.ForceDraw()
	}
}

func (home *Home) focusRightWrapper() {
	home.Tree.RemoveHighlight()
	home.RightWrapper.SetBorderColor(app.FocusTextColor)
	home.LeftWrapper.SetBorderColor(app.InactiveTextColor)
	home.TabbedPane.Highlight()

	tab := home.TabbedPane.GetCurrentTab()
	if tab != nil {
		focusTab(tab)
	}

	home.FocusedWrapper = wrapperRightSide
}

func focusTab(tab *Tab) {
	if tab == nil {
		return
	}

	table := tab.Content
	table.HighlightAll()

	if !table.GetIsFiltering() {
		App.SetFocus(table)
		return
	}

	go func() {
		if table.Filter != nil {
			App.SetFocus(table.Filter.Input)
			table.Filter.HighlightLocal()
		} else if table.Editor != nil {
			App.SetFocus(table.Editor)
			table.Editor.Highlight()
		}

		table.RemoveHighlightTable()
		App.Draw()
	}()
}

func (home *Home) focusLeftWrapper() {
	home.Tree.Highlight()

	home.RightWrapper.SetBorderColor(app.InactiveTextColor)
	home.LeftWrapper.SetBorderColor(app.FocusTextColor)

	tab := home.TabbedPane.GetCurrentTab()
	if tab != nil {
		table := tab.Content
		table.RemoveHighlightAll()
	}

	home.TabbedPane.SetBlur()

	App.SetFocus(home.Tree)

	home.FocusedWrapper = wrapperLeftSide
}

func (home *Home) rightWrapperInputCapture(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case previousTabSwitch:
		focusTab(home.TabbedPane.SwitchToPreviousTab())
		return nil
	case nextTabSwitch:
		focusTab(home.TabbedPane.SwitchToNextTab())
		return nil
	case firstTabSwitch:
		focusTab(home.TabbedPane.SwitchToFirstTab())
		return nil
	case lastTabSwitch:
		focusTab(home.TabbedPane.SwitchToLastTab())
		return nil
	case removeTabSwitch:
		if tab := home.TabbedPane.GetCurrentTab(); tab != nil {
			table := tab.Content

			if !table.GetIsFiltering() && !table.GetIsEditing() && !table.GetIsLoading() {
				home.TabbedPane.RemoveCurrentTab()

				if home.TabbedPane.GetLenght() == 0 {
					home.focusLeftWrapper()
					return nil
				}
			}
		}
	case previousPageSwitch:
		if tab := home.TabbedPane.GetCurrentTab(); tab != nil {
			table := tab.Content

			if ((table.Menu != nil && table.Menu.GetSelectedOption() == 1) || table.Menu == nil) &&
				!table.Pagination.GetIsFirstPage() && !table.GetIsLoading() {
				table.Pagination.SetOffset(table.Pagination.GetOffset() - table.Pagination.GetLimit())
				table.FetchRecords(table.GetDBReference())
			}
		}
	case nextPageSwitch:
		if tab := home.TabbedPane.GetCurrentTab(); tab != nil {
			table := tab.Content

			if ((table.Menu != nil && table.Menu.GetSelectedOption() == 1) || table.Menu == nil) &&
				!table.Pagination.GetIsLastPage() && !table.GetIsLoading() {
				table.Pagination.SetOffset(table.Pagination.GetOffset() + table.Pagination.GetLimit())
				table.FetchRecords(table.GetDBReference())
			}
		}
	}

	return event
}

func (home *Home) homeInputCapture(event *tcell.EventKey) *tcell.EventKey {
	var table *ResultsTable

	tab := home.TabbedPane.GetCurrentTab()
	if tab != nil {
		table = tab.Content
	} else if event.Rune() == quitSwitch {
		App.Stop()
		return event
	}

	switch event.Rune() {
	case leftWrapperSwitch:
		if table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && home.FocusedWrapper == wrapperRightSide {
			home.focusLeftWrapper()
		}
	case rightWrapperSwitch:
		if table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && home.FocusedWrapper == wrapperLeftSide {
			home.focusRightWrapper()
		}
	case editorSwitch:
		editorTab := home.TabbedPane.GetTabByName("Editor")
		if editorTab != nil {
			home.TabbedPane.SwitchToTabByName("Editor")
		} else {
			tableWithEditor := NewResultsTable(&home.ListOfDbChanges, &home.ListOfDbInserts, home.Tree, &home.DBDriver).WithEditor()
			home.TabbedPane.AppendTab("Editor", tableWithEditor)
			tableWithEditor.SetIsFiltering(true)
		}

		home.focusRightWrapper()
		App.ForceDraw()
	case connectionsSwitch:
		if (table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && !table.GetIsLoading()) || table == nil {
			MainPages.SwitchToPage("Connections")
		}
	case quitSwitch:
		if table != nil && !table.GetIsFiltering() && !table.GetIsEditing() {
			App.Stop()
		}
	case confirmationSwitch:
		if (home.ListOfDbChanges != nil && len(home.ListOfDbChanges) > 0) ||
			(home.ListOfDbInserts != nil && len(home.ListOfDbInserts) > 0) && !table.GetIsEditing() {
			confirmationModal := NewConfirmationModal("")

			confirmationModal.SetDoneFunc(func(_ int, buttonLabel string) {
				MainPages.RemovePage("Confirmation")
				confirmationModal = nil

				if buttonLabel == "Yes" {
					// fmt.Println("list of changes: ", home.ListOfDbChanges)
					// fmt.Println("list of inserts: ", home.ListOfDbInserts)
					err := home.DBDriver.ExecutePendingChanges(home.ListOfDbChanges, home.ListOfDbInserts)

					if err != nil {
						table.SetError(err.Error(), nil)
					} else {
						home.ListOfDbChanges = []models.DbDmlChange{}
						home.ListOfDbInserts = []models.DbInsert{}

						table.FetchRecords(table.GetDBReference())
						home.Tree.ForceRemoveHighlight()

					}

				}
			})

			MainPages.AddPage("Confirmation", confirmationModal, true, true)
		}
	}

	return event
}
