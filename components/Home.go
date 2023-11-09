package components

import (
	"lazysql/app"
	"lazysql/drivers"
	"lazysql/models"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type Home struct {
	*tview.Flex
	Tree            *Tree
	TabbedPane      *TabbedPane
	LeftWrapper     *tview.Flex
	RightWrapper    *tview.Flex
	FocusedWrapper  string
	ListOfDbChanges []models.DbDmlChange
	ListOfDbInserts []models.DbInsert
}

func NewHomePage(name string) *Home {
	tree := NewTree()
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
		if home.FocusedWrapper == "left" || home.FocusedWrapper == "" {
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
		switch stateChange.Key {
		case "SelectedTable":
			tableName := stateChange.Value.(string)

			tab := home.TabbedPane.GetTabByName(tableName)
			var table *ResultsTable = nil

			if tab != nil {
				table = tab.Content
				home.TabbedPane.SwitchToTabByName(tab.Name)
			} else {
				table = NewResultsTable(&home.ListOfDbChanges, &home.ListOfDbInserts, home.Tree).WithFilter()

				home.TabbedPane.AppendTab(tableName, table)
			}

			home.focusRightWrapper()

			table.FetchRecords(tableName)

			app.App.ForceDraw()
		}
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

	home.FocusedWrapper = "right"
}

func focusTab(tab *Tab) {
	if tab != nil {
		table := tab.Content
		table.HighlightAll()

		if table.GetIsFiltering() {
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
		} else {
			App.SetFocus(table)
		}

	}
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

	home.FocusedWrapper = "left"
}

func (home *Home) rightWrapperInputCapture(event *tcell.EventKey) *tcell.EventKey {
	var tab *Tab

	if event.Rune() == '[' {
		focusTab(home.TabbedPane.SwitchToPreviousTab())
		return nil
	} else if event.Rune() == ']' {
		focusTab(home.TabbedPane.SwitchToNextTab())
		return nil
	} else if event.Rune() == '{' {
		focusTab(home.TabbedPane.SwitchToFirstTab())
		return nil
	} else if event.Rune() == '}' {
		focusTab(home.TabbedPane.SwitchToLastTab())
		return nil
	} else if event.Rune() == 'X' {
		tab = home.TabbedPane.GetCurrentTab()

		if tab != nil {
			table := tab.Content

			if !table.GetIsFiltering() && !table.GetIsEditing() && !table.GetIsLoading() {
				home.TabbedPane.RemoveCurrentTab()

				if home.TabbedPane.GetLenght() == 0 {
					home.focusLeftWrapper()
					return nil
				}
			}
		}
	} else if event.Rune() == '<' {
		tab = home.TabbedPane.GetCurrentTab()

		if tab != nil {
			table := tab.Content

			if ((table.Menu != nil && table.Menu.GetSelectedOption() == 1) || table.Menu == nil) && !table.Pagination.GetIsFirstPage() && !table.GetIsLoading() {
				table.Pagination.SetOffset(table.Pagination.GetOffset() - table.Pagination.GetLimit())
				table.FetchRecords(table.GetDBReference())

			}

		}

	} else if event.Rune() == '>' {
		tab = home.TabbedPane.GetCurrentTab()

		if tab != nil {
			table := tab.Content

			if ((table.Menu != nil && table.Menu.GetSelectedOption() == 1) || table.Menu == nil) && !table.Pagination.GetIsLastPage() && !table.GetIsLoading() {
				table.Pagination.SetOffset(table.Pagination.GetOffset() + table.Pagination.GetLimit())
				table.FetchRecords(table.GetDBReference())
			}
		}
	}

	return event
}

func (home *Home) homeInputCapture(event *tcell.EventKey) *tcell.EventKey {
	tab := home.TabbedPane.GetCurrentTab()

	var table *ResultsTable = nil

	if tab != nil {
		table = tab.Content
	}

	if event.Rune() == 'H' {
		if table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && home.FocusedWrapper == "right" {
			home.focusLeftWrapper()
		}
	} else if event.Rune() == 'L' {
		if table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && home.FocusedWrapper == "left" {
			home.focusRightWrapper()
		}
	} else if event.Rune() == 5 {
		tab := home.TabbedPane.GetTabByName("Editor")

		if tab != nil {
			home.TabbedPane.SwitchToTabByName("Editor")
		} else {
			tableWithEditor := NewResultsTable(&home.ListOfDbChanges, &home.ListOfDbInserts, home.Tree).WithEditor()
			home.TabbedPane.AppendTab("Editor", tableWithEditor)
			tableWithEditor.SetIsFiltering(true)
		}
		home.focusRightWrapper()
		App.ForceDraw()
	} else if event.Rune() == 127 {
		if (table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && !table.GetIsLoading()) || table == nil {
			MainPages.SwitchToPage("Connections")
		}
	} else if event.Rune() == 'q' {
		if tab != nil {
			table := tab.Content

			if !table.GetIsFiltering() && !table.GetIsEditing() {
				App.Stop()
			}
		} else {
			App.Stop()
		}
	} else if event.Rune() == 19 {
		if (home.ListOfDbChanges != nil && len(home.ListOfDbChanges) > 0) || (home.ListOfDbInserts != nil && len(home.ListOfDbInserts) > 0) && !table.GetIsEditing() {
			confirmationModal := NewConfirmationModal("")

			confirmationModal.SetDoneFunc(func(_ int, buttonLabel string) {
				MainPages.RemovePage("Confirmation")
				confirmationModal = nil

				if buttonLabel == "Yes" {
					err := drivers.MySQL.ExecutePendingChanges(&home.ListOfDbChanges, &home.ListOfDbInserts)

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
