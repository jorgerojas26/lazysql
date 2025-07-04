package components

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/internal/history"
	"github.com/jorgerojas26/lazysql/lib"
	"github.com/jorgerojas26/lazysql/models"
)

type Home struct {
	*tview.Flex

	Tree                 *Tree
	TabbedPane           *TabbedPane
	LeftWrapper          *tview.Flex
	RightWrapper         *tview.Flex
	HelpStatus           HelpStatus
	HelpModal            *HelpModal
	DBDriver             drivers.Driver
	FocusedWrapper       string
	ListOfDBChanges      []models.DBDMLChange
	ConnectionIdentifier string
}

func NewHomePage(connection models.Connection, dbdriver drivers.Driver) *Home {
	tree := NewTree(connection.DBName, dbdriver)
	tabbedPane := NewTabbedPane()
	leftWrapper := tview.NewFlex()
	rightWrapper := tview.NewFlex()

	maincontent := tview.NewFlex()

	connectionIdentifier := connection.Name
	if connectionIdentifier == "" {
		parsedURL, err := url.Parse(connection.URL)
		if err == nil {
			connectionIdentifier = history.SanitizeFilename(parsedURL.Host + strings.ReplaceAll(parsedURL.Path, "/", "_"))
		} else {
			connectionIdentifier = "unnamed_or_invalid_url_connection"
		}
	}

	home := &Home{
		Flex:                 tview.NewFlex().SetDirection(tview.FlexRow),
		Tree:                 tree,
		TabbedPane:           tabbedPane,
		LeftWrapper:          leftWrapper,
		RightWrapper:         rightWrapper,
		HelpStatus:           NewHelpStatus(),
		HelpModal:            NewHelpModal(),
		DBDriver:             dbdriver,
		ListOfDBChanges:      []models.DBDMLChange{},
		ConnectionIdentifier: connectionIdentifier,
	}

	go home.subscribeToTreeChanges()

	leftWrapper.SetBorderColor(app.Styles.InverseTextColor)
	leftWrapper.AddItem(tree.Wrapper, 0, 1, true)

	rightWrapper.SetBorderColor(app.Styles.InverseTextColor)
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
		if home.FocusedWrapper == focusedWrapperLeft || home.FocusedWrapper == "" {
			home.focusLeftWrapper()
		} else {
			home.focusRightWrapper()
		}
	})

	mainPages.AddPage(connection.URL, home, true, false)
	return home
}

func (home *Home) subscribeToTreeChanges() {
	ch := home.Tree.Subscribe()

	for stateChange := range ch {
		switch stateChange.Key {
		case eventTreeSelectedTable:
			databaseName := home.Tree.GetSelectedDatabase()
			tableName := stateChange.Value.(string)

			tabReference := fmt.Sprintf("%s.%s", databaseName, tableName)

			tab := home.TabbedPane.GetTabByReference(tabReference)

			var table *ResultsTable

			if tab != nil {
				table = tab.Content
				home.TabbedPane.SwitchToTabByReference(tab.Reference)
			} else {
				table = NewResultsTable(&home.ListOfDBChanges, home.Tree, home.DBDriver, home.ConnectionIdentifier).WithFilter()
				table.SetDatabaseName(databaseName)
				table.SetTableName(tableName)

				home.TabbedPane.AppendTab(tableName, table, tabReference)
			}

			results := table.FetchRecords(func() {
				home.focusLeftWrapper()
			})

			// Show sidebar if there is more then 1 row (row 0 are
			// the column names) and the sidebar is not disabled.
			if !app.App.Config().DisableSidebar && len(results) > 1 && !table.GetShowSidebar() {
				table.ShowSidebar(true)
			}

			if table.state.error == "" {
				home.focusRightWrapper()
			}

			app.App.ForceDraw()
		case eventTreeIsFiltering:
			isFiltering := stateChange.Value.(bool)
			if isFiltering {
				home.SetInputCapture(nil)
			} else {
				home.SetInputCapture(home.homeInputCapture)
			}
		}
	}
}

func (home *Home) focusRightWrapper() {
	home.Tree.RemoveHighlight()

	home.RightWrapper.SetBorderColor(app.Styles.PrimaryTextColor)
	home.LeftWrapper.SetBorderColor(app.Styles.InverseTextColor)
	home.TabbedPane.Highlight()
	tab := home.TabbedPane.GetCurrentTab()

	if tab != nil {
		home.focusTab(tab)
	}

	home.FocusedWrapper = focusedWrapperRight
}

func (home *Home) focusTab(tab *Tab) {
	if tab != nil {
		table := tab.Content
		table.HighlightAll()

		if table.GetIsFiltering() {
			go func() {
				if table.Filter != nil {
					app.App.SetFocus(table.Filter.Input)
					table.Filter.HighlightLocal()
				} else if table.Editor != nil {
					app.App.SetFocus(table.Editor)
					table.Editor.Highlight()
				}

				table.RemoveHighlightTable()
				app.App.Draw()
			}()
		} else {
			table.SetInputCapture(table.tableInputCapture)
			app.App.SetFocus(table)
		}

		if tab.Name == tabNameEditor {
			home.HelpStatus.SetStatusOnEditorView()
		} else {
			home.HelpStatus.SetStatusOnTableView()
		}
	}
}

func (home *Home) focusLeftWrapper() {
	home.Tree.Highlight()

	home.RightWrapper.SetBorderColor(app.Styles.InverseTextColor)
	home.LeftWrapper.SetBorderColor(app.Styles.PrimaryTextColor)

	tab := home.TabbedPane.GetCurrentTab()

	if tab != nil {
		table := tab.Content

		table.RemoveHighlightAll()

	}

	home.TabbedPane.SetBlur()

	app.App.SetFocus(home.Tree)

	home.FocusedWrapper = focusedWrapperLeft
}

func (home *Home) rightWrapperInputCapture(event *tcell.EventKey) *tcell.EventKey {
	var tab *Tab

	command := app.Keymaps.Group(app.TableGroup).Resolve(event)

	switch command {
	case commands.TabPrev:

		tab := home.TabbedPane.GetCurrentTab()

		if tab != nil {
			table := tab.Content
			if !table.GetIsEditing() && !table.GetIsFiltering() {
				home.focusTab(home.TabbedPane.SwitchToPreviousTab())
				return nil
			}

		}

		return event
	case commands.TabNext:
		tab := home.TabbedPane.GetCurrentTab()

		if tab != nil {
			table := tab.Content
			if !table.GetIsEditing() && !table.GetIsFiltering() {
				home.focusTab(home.TabbedPane.SwitchToNextTab())
				return nil
			}
		}

		return event
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

				if home.TabbedPane.GetLength() == 0 {
					home.focusLeftWrapper()
					return nil
				}
			}
		}
	case commands.PagePrev:
		tab = home.TabbedPane.GetCurrentTab()

		if tab != nil {
			table := tab.Content

			if ((table.Menu != nil && table.Menu.GetSelectedOption() == 1) ||
				table.Menu == nil) && !table.Pagination.GetIsFirstPage() && !table.GetIsLoading() {
				table.Pagination.SetOffset(table.Pagination.GetOffset() - table.Pagination.GetLimit())
				table.FetchRecords(nil)
			}
		}

	case commands.PageNext:
		tab = home.TabbedPane.GetCurrentTab()

		if tab != nil {
			table := tab.Content

			if ((table.Menu != nil && table.Menu.GetSelectedOption() == 1) ||
				table.Menu == nil) && !table.Pagination.GetIsLastPage() && !table.GetIsLoading() {
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
		if table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && home.FocusedWrapper == focusedWrapperRight {
			home.focusLeftWrapper()
		}
	case commands.MoveRight:
		if table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && home.FocusedWrapper == focusedWrapperLeft {
			home.focusRightWrapper()
		}
	case commands.SwitchToEditorView:
		tab := home.TabbedPane.GetTabByName(tabNameEditor)

		if tab != nil {
			home.TabbedPane.SwitchToTabByName(tabNameEditor)
			tab.Content.SetIsFiltering(true)
		} else {
			tableWithEditor := NewResultsTable(&home.ListOfDBChanges, home.Tree, home.DBDriver, home.ConnectionIdentifier).WithEditor()
			home.TabbedPane.AppendTab(tabNameEditor, tableWithEditor, tabNameEditor)
			tableWithEditor.SetIsFiltering(true)
		}
		home.HelpStatus.SetStatusOnEditorView()
		home.focusRightWrapper()
		app.App.ForceDraw()
	case commands.SwitchToConnectionsView:
		if (table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && !table.GetIsLoading()) || table == nil {
			mainPages.SwitchToPage(pageNameConnections)
		}
	case commands.Quit:
		if tab == nil || (!table.GetIsEditing() && !table.GetIsFiltering()) {
			app.App.Stop()
		}
	case commands.Save:
		if (len(home.ListOfDBChanges) > 0) && !table.GetIsEditing() {
			queryPreviewModal := NewQueryPreviewModal(&home.ListOfDBChanges, home.DBDriver, func() {
				for _, change := range home.ListOfDBChanges {
					queryString, err := home.DBDriver.DMLChangeToQueryString(change)
					if err != nil {
						logger.Error("Failed to convert DML change to query string", map[string]any{"error": err})
						continue
					}
					err = history.AddQueryToHistory(home.ConnectionIdentifier, queryString)
					if err != nil {
						logger.Error("Failed to add query to history", map[string]any{"error": err})
					}
				}
				home.ListOfDBChanges = []models.DBDMLChange{}
				table.FetchRecords(nil)
				home.Tree.ForceRemoveHighlight()
			})

			mainPages.AddPage(pageNameDMLPreview, queryPreviewModal, true, true)
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
			mainPages.AddPage(pageNameHelp, home.HelpModal, true, true)
		}
	case commands.SearchGlobal:
		if table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && home.FocusedWrapper == focusedWrapperRight {
			home.focusLeftWrapper()
		}

		home.Tree.ForceRemoveHighlight()
		home.Tree.ClearSearch()
		app.App.SetFocus(home.Tree.Filter)
		home.Tree.SetIsFiltering(true)
	case commands.ToggleQueryHistory:
		if (table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && !table.GetIsLoading()) || table == nil {

			historyFilePath, err := history.GetHistoryFilePath(home.ConnectionIdentifier)
			if err != nil {
				logger.Error("Failed to get history file path", map[string]any{"error": err, "connection": home.ConnectionIdentifier})
				// Optionally: show an error modal to the user
				return nil // Command handled (by logging error)
			}

			// The limit from config is primarily for writing, but pass it to ReadHistory for potential future use.
			// QueryHistoryModal will display all items passed to it after sorting.
			historyLimit := app.App.Config().MaxQueryHistoryPerConnection
			if historyLimit <= 0 { // Ensure a positive limit if not set or invalid in config
				historyLimit = 100 // Default limit
			}

			historyItems, err := history.ReadHistory(historyFilePath, historyLimit)
			if err != nil {
				logger.Error("Failed to read query history", map[string]any{"error": err, "path": historyFilePath})
				// Show empty history on error, or an error message to the user.
				// For now, proceed with empty items.
				historyItems = []models.QueryHistoryItem{}
			}

			queryHistoryModal := NewQueryHistoryModal(historyItems, func(selectedQuery string) {
				// Action on query selection:
				// 1. Copy to clipboard
				cb := lib.NewClipboard()
				err := cb.Write(selectedQuery)
				if err != nil {
					logger.Error("Failed to copy query history item to clipboard", map[string]any{"error": err})
					// Optionally notify user of copy failure (e.g., via a short-lived status message)
				} else {
					// Optionally notify user of copy success
					logger.Info("Query copied to clipboard from history.", map[string]any{"query": selectedQuery})
				}

				// 2. Close the modal
				mainPages.RemovePage(pageNameQueryHistory)
				app.App.SetFocus(home) // Or focus the previously focused element if tracked

				// 3. (Future Phase) Paste into SQL Editor and focus editor
				// if home.TabbedPane != nil {
				//    editorTab := home.TabbedPane.GetTabByName(tabNameEditor)
				//    if editorTab != nil && editorTab.Content != nil && editorTab.Content.Editor != nil {
				//        editorTab.Content.Editor.SetText(selectedQuery, true) // Assuming SetText exists and takes (text, moveCursorToEnd)
				//        home.focusRightWrapper() // Ensure right pane is focused
				//        app.App.SetFocus(editorTab.Content.Editor) // Focus the editor
				//    }
				// }
			})

			mainPages.AddPage(pageNameQueryHistory, queryHistoryModal.GetPrimitive(), true, true)
			// The QueryHistoryModal should set its own focus, typically on the filter input.
			// If it doesn't, uncomment: app.App.SetFocus(queryHistoryModal.GetPrimitive())
		}
		return nil // Command handled
	}

	return event
}
