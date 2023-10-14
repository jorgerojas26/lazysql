package pages

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"lazysql/app"
	"lazysql/components"
)

var (
	HomePage   = tview.NewFlex()
	Tree       = components.NewTree()
	App        = app.App
	TabbedPane = components.NewTabbedPane()
)

var (
	LeftWrapper  = tview.NewFlex()
	RightWrapper = tview.NewFlex()
)

var FocusedWrapper = "left"

func init() {
	go subscribeToTreeChanges()

	LeftWrapper.SetBorderColor(app.InactiveTextColor)

	RightWrapper.SetBorderColor(app.InactiveTextColor)
	RightWrapper.SetBorder(true)
	RightWrapper.SetDirection(tview.FlexColumnCSS)

	RightWrapper.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		var tab *components.Tab

		if event.Rune() == '[' {
			focusTab(TabbedPane.SwitchToPreviousTab())
		} else if event.Rune() == ']' {
			focusTab(TabbedPane.SwitchToNextTab())
		} else if event.Rune() == '{' {
			focusTab(TabbedPane.SwitchToFirstTab())
		} else if event.Rune() == '}' {
			focusTab(TabbedPane.SwitchToLastTab())
		} else if event.Rune() == 'X' {
			TabbedPane.RemoveCurrentTab()

			if TabbedPane.GetLenght() == 0 {
				focusLeftWrapper()
				return nil
			}

		} else if event.Rune() == '<' {
			tab = TabbedPane.GetCurrentTab()

			if tab != nil {
				table := tab.Content

				if table.Menu.GetSelectedOption() == 1 && !table.Pagination.GetIsFirstPage() && !table.GetIsLoading() {
					table.Pagination.SetOffset(table.Pagination.GetOffset() - table.Pagination.GetLimit())
					table.FetchRecords(table.GetDBReference())

				}

			}

		} else if event.Rune() == '>' {
			tab = TabbedPane.GetCurrentTab()

			if tab != nil {
				table := tab.Content

				if table.Menu.GetSelectedOption() == 1 && !table.Pagination.GetIsLastPage() && !table.GetIsLoading() {
					table.Pagination.SetOffset(table.Pagination.GetOffset() + table.Pagination.GetLimit())
					table.FetchRecords(table.GetDBReference())
				}
			}
		}

		return event
	})
	LeftWrapper.AddItem(Tree, 0, 1, true)

	RightWrapper.AddItem(TabbedPane.HeaderContainer, 1, 0, false)
	RightWrapper.AddItem(TabbedPane.Pages, 0, 1, false)

	HomePage.AddItem(LeftWrapper, 30, 1, true)
	HomePage.AddItem(RightWrapper, 0, 5, false)

	HomePage.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {

		tab := TabbedPane.GetCurrentTab()

		var table *components.ResultsTable = nil

		if tab != nil {
			table = tab.Content
		}

		if event.Rune() == 'H' {
			if table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && FocusedWrapper == "right" {
				focusLeftWrapper()
			}
		} else if event.Rune() == 'L' {
			if table != nil && !table.GetIsEditing() && !table.GetIsFiltering() && FocusedWrapper == "left" {
				focusRightWrapper()
			}
		} else if event.Rune() == 5 {
			tab := TabbedPane.GetTabByName("Editor")

			if tab != nil {
				TabbedPane.SwitchToTabByName("Editor")
			} else {
				tableWithEditor := components.NewResultsTable().WithEditor()
				TabbedPane.AppendTab("Editor", tableWithEditor)
			}
			focusRightWrapper()
			App.ForceDraw()
		} else if event.Rune() == 'q' {
			if tab != nil {
				table := tab.Content

				if !table.GetIsFiltering() && !table.GetIsEditing() {
					App.Stop()
				}
			} else {
				App.Stop()
			}
		}

		return event
	})

	AllPages.AddPage("home", HomePage, true, false)
}

func subscribeToTreeChanges() {
	ch := Tree.Subscribe()

	for stateChange := range ch {
		switch stateChange.Key {
		case "SelectedTable":
			tableName := stateChange.Value.(string)

			tab := TabbedPane.GetTabByName(tableName)
			var table *components.ResultsTable = nil

			if tab != nil {
				table = tab.Content
				TabbedPane.SwitchToTabByName(tab.Name)
			} else {
				table = components.NewResultsTable().WithFilter()

				TabbedPane.AppendTab(tableName, table)
			}

			focusRightWrapper()

			table.FetchRecords(tableName)

			app.App.ForceDraw()

		}
	}
}

func focusRightWrapper() {
	Tree.RemoveHighlight()

	RightWrapper.SetBorderColor(app.FocusTextColor)
	LeftWrapper.SetBorderColor(app.InactiveTextColor)
	TabbedPane.Highlight()
	tab := TabbedPane.GetCurrentTab()

	if tab != nil {
		focusTab(tab)
	}

	FocusedWrapper = "right"
}

func focusTab(tab *components.Tab) {
	if tab != nil {
		table := tab.Content
		table.HighlightAll()

		if table.GetIsFiltering() {
			go func() {
				App.SetFocus(table.Filter.Input)
				table.Filter.HighlightLocal()
				table.RemoveHighlightTable()
				App.Draw()
			}()
		} else {
			App.SetFocus(table)
		}

	}
}

func focusLeftWrapper() {
	Tree.Highlight()

	RightWrapper.SetBorderColor(app.InactiveTextColor)
	LeftWrapper.SetBorderColor(app.FocusTextColor)

	tab := TabbedPane.GetCurrentTab()

	if tab != nil {
		table := tab.Content

		table.RemoveHighlightAll()

	}

	TabbedPane.SetBlur()

	App.SetFocus(Tree)

	FocusedWrapper = "left"
}
