package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

type Home struct {
	*tview.Flex
	Tree            *Tree
	TabbedPane      *TabbedPane
	LeftWrapper     *tview.Flex
	RightWrapper    *tview.Flex
	HelpStatus      HelpStatus
	HelpModal       *HelpModal
	DBDriver        drivers.Driver
	FocusedWrapper  string
	ListOfDbChanges []models.DbDmlChange
	ListOfDbInserts []models.DbInsert
}

func NewHomePage(connection models.Connection, dbdriver drivers.Driver) *Home {
	tree := NewTree(connection.DBName, dbdriver)
	tabbedPane := NewTabbedPane()
	leftWrapper := tview.NewFlex()
	rightWrapper := tview.NewFlex()

	maincontent := tview.NewFlex()

	home := &Home{
		Flex:            tview.NewFlex().SetDirection(tview.FlexRow),
		Tree:            tree,
		TabbedPane:      tabbedPane,
		LeftWrapper:     leftWrapper,
		RightWrapper:    rightWrapper,
		HelpStatus:      NewHelpStatus(),
		HelpModal:       NewHelpModal(),
		ListOfDbChanges: []models.DbDmlChange{},
		ListOfDbInserts: []models.DbInsert{},
		DBDriver:        dbdriver,
	}

	go home.subscribeToTreeChanges()

	leftWrapper.SetBorderColor(tview.Styles.InverseTextColor)
	leftWrapper.AddItem(tree, 0, 1, true)

	rightWrapper.SetBorderColor(tview.Styles.InverseTextColor)
	rightWrapper.SetBorder(true)
	rightWrapper.SetDirection(tview.FlexColumnCSS)
	rightWrapper.SetInputCapture(home.rightWrapperInputCapture)
	rightWrapper.AddItem(tabbedPane.HeaderContainer, 1, 0, false)
	rightWrapper.AddItem(tabbedPane.Pages, 0, 1, false)

	maincontent.AddItem(leftWrapper, 30, 1, false)
	maincontent.AddItem(rightWrapper, 0, 5, false)

	home.AddItem(maincontent, 0, 1, false)
	// home.AddItem(home.HelpStatus, 1, 1, false)

	home.SetInputCapture(home.homeInputCapture)

	home.SetFocusFunc(func() {
		if home.FocusedWrapper == "left" || home.FocusedWrapper == "" {
			home.focusLeftWrapper()
		} else {
			home.focusRightWrapper()
		}
	})

	MainPages.AddPage(connection.URL, home, true, false)
	return home
}

func (home *Home) subscribeToTreeChanges() {
	ch := home.Tree.Subscribe()

	for stateChange := range ch {
		switch stateChange.Key {
		case "SelectedTable":
			tableName := stateChange.Value.(string)

			tab := home.TabbedPane.GetTabByName(tableName)
			var table *ResultsTable

			if tab != nil {
				table = tab.Content
				home.TabbedPane.SwitchToTabByName(tab.Name)
			} else {
				table = NewResultsTable(&home.ListOfDbChanges, &home.ListOfDbInserts, home.Tree, home.DBDriver).WithFilter()
				table.SetDBReference(tableName)

				home.TabbedPane.AppendTab(tableName, table)
			}

			table.FetchRecords(func() {
				home.focusLeftWrapper()
			})

			if table.state.error == "" {
				home.focusRightWrapper()
			}

			app.App.ForceDraw()
		}
	}
}

func (home *Home) focusRightWrapper() {
	home.Tree.RemoveHighlight()

	home.RightWrapper.SetBorderColor(tview.Styles.PrimaryTextColor)
	home.LeftWrapper.SetBorderColor(tview.Styles.InverseTextColor)
	home.TabbedPane.Highlight()
	tab := home.TabbedPane.GetCurrentTab()

	if tab != nil {
		home.focusTab(tab)
	}

	home.FocusedWrapper = "right"
}

func (home *Home) focusTab(tab *Tab) {
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
			table.SetInputCapture(table.tableInputCapture)
			App.SetFocus(table)
		}

		if tab.Name == EditorTabName {
			home.HelpStatus.SetStatusOnEditorView()
		} else {
			home.HelpStatus.SetStatusOnTableView()
		}
	}
}

func (home *Home) focusLeftWrapper() {
	home.Tree.Highlight()

	home.RightWrapper.SetBorderColor(tview.Styles.InverseTextColor)
	home.LeftWrapper.SetBorderColor(tview.Styles.PrimaryTextColor)

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

	command := app.Keymaps.Group(app.TableGroup).Resolve(event)

	switch command {
	case commands.TabPrev:
		home.focusTab(home.TabbedPane.SwitchToPreviousTab())
		return nil
	case commands.TabNext:
		home.focusTab(home.TabbedPane.SwitchToNextTab())
		return nil
	case commands.TabFirst:
		home.focusTab(home.TabbedPane.SwitchToFirstTab())
		return nil
	case commands.TabLast:
		home.focusTab(home.TabbedPane.SwitchToLastTab())
		return nil
	case commands.TabClose:
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
	case commands.PagePrev:
		tab = home.TabbedPane.GetCurrentTab()

		if tab != nil {
			table := tab.Content

			if ((table.Menu != nil && table.Menu.GetSelectedOption() == 1) || table.Menu == nil) && !table.Pagination.GetIsFirstPage() && !table.GetIsLoading() {
				table.Pagination.SetOffset(table.Pagination.GetOffset() - table.Pagination.GetLimit())
				table.FetchRecords(nil)

			}

		}

	case commands.PageNext:
		tab = home.TabbedPane.GetCurrentTab()

		if tab != nil {
			table := tab.Content

			if ((table.Menu != nil && table.Menu.GetSelectedOption() == 1) || table.Menu == nil) && !table.Pagination.GetIsLastPage() && !table.GetIsLoading() {
				table.Pagination.SetOffset(table.Pagination.GetOffset() + table.Pagination.GetLimit())
				table.FetchRecords(nil)
			}
		}
	}

	return event
}

func (home *Home) homeInputCapture(event *tcell.EventKey) *tcell.EventKey {
	tab := home.TabbedPane.GetCurrentTab()

	var table *ResultsTable

	if tab != nil {
		table = tab.Content
	}

	command := app.Keymaps.Group(app.HomeGroup).Resolve(event)

	switch command {
	case commands.MoveLeft:
		if table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && home.FocusedWrapper == "right" {
			home.focusLeftWrapper()
		}
	case commands.MoveRight:
		if table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && home.FocusedWrapper == "left" {
			home.focusRightWrapper()
		}
	case commands.SwitchToEditorView:
		tab := home.TabbedPane.GetTabByName(EditorTabName)

		if tab != nil {
			home.TabbedPane.SwitchToTabByName(EditorTabName)
		} else {
			tableWithEditor := NewResultsTable(&home.ListOfDbChanges, &home.ListOfDbInserts, home.Tree, home.DBDriver).WithEditor()
			home.TabbedPane.AppendTab(EditorTabName, tableWithEditor)
			tableWithEditor.SetIsFiltering(true)
		}
		home.HelpStatus.SetStatusOnEditorView()
		home.focusRightWrapper()
		App.ForceDraw()
	case commands.SwitchToConnectionsView:
		if (table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && !table.GetIsLoading()) || table == nil {
			MainPages.SwitchToPage("Connections")
		}
	case commands.Quit:
		if tab != nil {
			table := tab.Content

			if !table.GetIsFiltering() && !table.GetIsEditing() {
				App.Stop()
			}
		} else {
			App.Stop()
		}
	case commands.Save:
		if ((len(home.ListOfDbChanges) > 0) || len(home.ListOfDbInserts) > 0) && !table.GetIsEditing() {
			confirmationModal := NewConfirmationModal("")

			confirmationModal.SetDoneFunc(func(_ int, buttonLabel string) {
				MainPages.RemovePage("Confirmation")
				confirmationModal = nil

				if buttonLabel == "Yes" {

					err := home.DBDriver.ExecutePendingChanges(home.ListOfDbChanges, home.ListOfDbInserts)

					if err != nil {
						table.SetError(err.Error(), nil)
					} else {
						home.ListOfDbChanges = []models.DbDmlChange{}
						home.ListOfDbInserts = []models.DbInsert{}

						table.FetchRecords(nil)
						home.Tree.ForceRemoveHighlight()

					}

				}
			})

			MainPages.AddPage("Confirmation", confirmationModal, true, true)
		}
	case commands.HelpPopup:
		if table == nil || !table.GetIsEditing() {
			// home.HelpModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			// 	command := app.Keymaps.Resolve(event)
			// 	if command == commands.Quit {
			// 		App.Stop()
			// 	} else if event.Key() == tcell.KeyEsc {
			// 		MainPages.RemovePage(HelpPageName)
			// 	}
			// 	return event
			// })
			MainPages.AddPage(HelpPageName, home.HelpModal, true, true)
		}
	}

	return event
}
