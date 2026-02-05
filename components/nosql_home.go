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
	"github.com/jorgerojas26/lazysql/internal/history"
	"github.com/jorgerojas26/lazysql/models"
)

type NoSQLHome struct {
	*tview.Flex
	Tree                 *NoSQLTree
	TabbedPane           *TabbedPane
	LeftWrapper          *tview.Flex
	RightWrapper         *tview.Flex
	HelpModal            *HelpModal
	DBDriver             drivers.NoSQLDriver
	FocusedWrapper       string
	ConnectionIdentifier string
	ConnectionURL        string
}

func NewNoSQLHomePage(connection models.Connection, dbdriver drivers.NoSQLDriver) *NoSQLHome {
	tree := NewNoSQLTree(connection.DBName, dbdriver)
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

	home := &NoSQLHome{
		Flex:         tview.NewFlex().SetDirection(tview.FlexRow),
		Tree:         tree,
		LeftWrapper:  leftWrapper,
		RightWrapper: rightWrapper,
		HelpModal:    NewHelpModal(),

		DBDriver:             dbdriver,
		ConnectionIdentifier: connectionIdentifier,
		ConnectionURL:        connection.URL,
	}

	tabbedPane := NewTabbedPane()

	home.TabbedPane = tabbedPane

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

func (home *NoSQLHome) subscribeToTreeChanges() {
	ch := home.Tree.Subscribe()

	for stateChange := range ch {
		switch stateChange.Key {
		case eventNoSQLTreeSelectedCollection:
			databaseName := home.Tree.GetSelectedDatabase()
			collectionName := stateChange.Value.(string)

			tabReference := fmt.Sprintf("%s.%s", databaseName, collectionName)

			tab := home.TabbedPane.GetTabByReference(tabReference)

			var docTable *DocumentTable

			if tab != nil {
				docTable = tab.Content.(*DocumentTable)
				home.TabbedPane.SwitchToTabByReference(tab.Reference)
			} else {
				docTable = NewDocumentTable(home.Tree, home.DBDriver, home.ConnectionIdentifier, home.ConnectionURL).WithFilter()
				docTable.SetDatabaseName(databaseName)
				docTable.SetCollectionName(collectionName)

				home.TabbedPane.AppendTab(collectionName, docTable, tabReference)
			}

			docTable.FetchDocuments(func() {
				home.focusLeftWrapper()
			})

			if docTable.state.error == "" {
				home.focusRightWrapper()
			}

			app.App.ForceDraw()
		case eventNoSQLTreeIsFiltering:
			isFiltering := stateChange.Value.(bool)
			if isFiltering {
				home.SetInputCapture(nil)
			} else {
				home.SetInputCapture(home.homeInputCapture)
			}
		}
	}
}

func (home *NoSQLHome) focusRightWrapper() {
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

func (home *NoSQLHome) focusTab(tab *Tab) {
	if tab != nil {
		docTable := tab.Content.(*DocumentTable)
		docTable.HighlightAll()

		docTable.SetInputCapture(docTable.tableInputCapture)
		app.App.SetFocus(docTable)
	}
}

func (home *NoSQLHome) focusLeftWrapper() {
	home.Tree.Highlight()

	home.RightWrapper.SetBorderColor(app.Styles.InverseTextColor)
	home.LeftWrapper.SetBorderColor(app.Styles.PrimaryTextColor)

	tab := home.TabbedPane.GetCurrentTab()

	if tab != nil {
		docTable := tab.Content.(*DocumentTable)
		docTable.RemoveHighlightAll()
	}

	home.TabbedPane.SetBlur()

	app.App.SetFocus(home.Tree)

	home.FocusedWrapper = focusedWrapperLeft
}

func (home *NoSQLHome) rightWrapperInputCapture(event *tcell.EventKey) *tcell.EventKey {
	var tab *Tab

	command := app.Keymaps.Group(app.TableGroup).Resolve(event)

	switch command {
	case commands.TabPrev:

		tab := home.TabbedPane.GetCurrentTab()

		if tab != nil {
			docTable := tab.Content.(*DocumentTable)
			if !docTable.GetIsFiltering() {
				home.TabbedPane.SwitchToPreviousTab()
				return nil
			}

		}

		return event
	case commands.TabNext:
		tab := home.TabbedPane.GetCurrentTab()

		if tab != nil {
			docTable := tab.Content.(*DocumentTable)
			if !docTable.GetIsFiltering() {
				home.TabbedPane.SwitchToNextTab()
				return nil
			}
		}

		return event
	case commands.TabFirst:
		home.TabbedPane.SwitchToFirstTab()
		return nil
	case commands.TabLast:
		home.TabbedPane.SwitchToLastTab()
		return nil
	case commands.TabClose:
		tab = home.TabbedPane.GetCurrentTab()

		if tab != nil {
			docTable := tab.Content.(*DocumentTable)

			if !docTable.GetIsFiltering() && !docTable.GetIsLoading() {
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
			docTable := tab.Content.(*DocumentTable)

			if ((docTable.Menu != nil && docTable.Menu.GetSelectedOption() == 1) ||
				docTable.Menu == nil) && !docTable.Pagination.GetIsFirstPage() && !docTable.GetIsLoading() {
				docTable.Pagination.SetOffset(docTable.Pagination.GetOffset() - docTable.Pagination.GetLimit())
				docTable.FetchDocuments(nil)
			}
		}

	case commands.PageNext:
		tab = home.TabbedPane.GetCurrentTab()

		if tab != nil {
			docTable := tab.Content.(*DocumentTable)

			if ((docTable.Menu != nil && docTable.Menu.GetSelectedOption() == 1) ||
				docTable.Menu == nil) && !docTable.Pagination.GetIsLastPage() && !docTable.GetIsLoading() {
				docTable.Pagination.SetOffset(docTable.Pagination.GetOffset() + docTable.Pagination.GetLimit())
				docTable.FetchDocuments(nil)
			}
		}
	}

	return event
}

func (home *NoSQLHome) homeInputCapture(event *tcell.EventKey) *tcell.EventKey {
	tab := home.TabbedPane.GetCurrentTab()

	var docTable *DocumentTable

	if tab != nil {
		docTable = tab.Content.(*DocumentTable)
	}

	command := app.Keymaps.Group(app.HomeGroup).Resolve(event)

	switch command {
	case commands.MoveLeft:
		if docTable != nil && !docTable.GetIsFiltering() && home.FocusedWrapper == focusedWrapperRight {
			home.focusLeftWrapper()
		}
	case commands.MoveRight:
		if docTable != nil && !docTable.GetIsFiltering() && home.FocusedWrapper == focusedWrapperLeft {
			home.focusRightWrapper()
		}
	case commands.SwitchToConnectionsView:
		if (docTable != nil && !docTable.GetIsFiltering() && !docTable.GetIsLoading()) || docTable == nil {
			mainPages.SwitchToPage(pageNameConnections)
		}
	case commands.Quit:
		if tab == nil || !docTable.GetIsFiltering() {
			app.App.Stop()
		}
	case commands.HelpPopup:
		mainPages.AddPage(pageNameHelp, home.HelpModal, true, true)
	case commands.SearchGlobal:
		if docTable != nil && !docTable.GetIsFiltering() && !docTable.GetIsLoading() && home.FocusedWrapper == focusedWrapperRight {
			home.focusLeftWrapper()
		}

		home.Tree.ForceRemoveHighlight()
		home.Tree.ClearSearch()
		app.App.SetFocus(home.Tree.Filter)
		home.Tree.SetIsFiltering(true)
	}

	return event
}
