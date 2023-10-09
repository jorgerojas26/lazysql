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

	LeftWrapper.SetBorderColor(app.BlurTextColor)

	RightWrapper.SetBorderColor(app.BlurTextColor)
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
			tab = TabbedPane.GetCurrentTab()

			if tab != nil {
				TabbedPane.RemoveTab(tab.Index)

				if TabbedPane.GetTabCount() == 0 {
					focusLeftWrapper()
					return nil
				}
			}

		} else if event.Rune() == '<' {
			tab = TabbedPane.GetCurrentTab()

			if tab != nil {
				table := tab.Page

				if table.Menu.GetSelectedOption() == 1 && !table.Pagination.GetIsFirstPage() && !table.GetIsLoading() {
					table.Pagination.SetOffset(table.Pagination.GetOffset() - table.Pagination.GetLimit())
					table.FetchRecords(table.GetDBReference())

				}

			}

		} else if event.Rune() == '>' {
			tab = TabbedPane.GetCurrentTab()

			if tab != nil {
				table := tab.Page

				if table.Menu.GetSelectedOption() == 1 && !table.Pagination.GetIsLastPage() && !table.GetIsLoading() {
					table.Pagination.SetOffset(table.Pagination.GetOffset() + table.Pagination.GetLimit())
					table.FetchRecords(table.GetDBReference())
				}
			}
		}

		return event
	})
	LeftWrapper.AddItem(Tree, 0, 1, true)

	RightWrapper.AddItem(TabbedPane.Wrapper, 1, 0, false)
	RightWrapper.AddItem(TabbedPane.Pages, 0, 1, false)

	HomePage.AddItem(LeftWrapper, 30, 1, true)
	HomePage.AddItem(RightWrapper, 0, 5, false)

	HomePage.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		tab := TabbedPane.GetCurrentTab()

		if event.Rune() == 'H' {
			if FocusedWrapper == "right" {
				focusLeftWrapper()
			}
		} else if event.Rune() == 'L' {
			if FocusedWrapper == "left" {
				focusRightWrapper()
			}
		} else if event.Rune() == 'q' {
			if tab != nil {
				table := tab.Page

				if !table.Filter.GetIsFiltering() && !table.GetIsEditing() {
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
				table = tab.Page
				TabbedPane.SwitchToTab(tab.Name)
			} else {
				table = components.NewResultsTable()

				TabbedPane.AddTab(&components.Tab{
					Name: tableName,
					Page: table,
				})
			}

			table.FetchRecords(tableName)

			focusRightWrapper()

			app.App.ForceDraw()

		}
	}
}

func focusRightWrapper() {
	Tree.RemoveHighlight()

	RightWrapper.SetBorderColor(app.FocusTextColor)
	LeftWrapper.SetBorderColor(app.BlurTextColor)
	TabbedPane.Highlight()
	tab := TabbedPane.GetCurrentTab()

	if tab != nil {
		focusTab(tab)
	}

	FocusedWrapper = "right"
}

func focusTab(tab *components.Tab) {
	if tab != nil {
		table := tab.Page
		table.HighlightAll()

		if table.Filter.GetIsFiltering() {
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

	RightWrapper.SetBorderColor(app.BlurTextColor)
	LeftWrapper.SetBorderColor(app.FocusTextColor)

	tab := TabbedPane.GetCurrentTab()

	if tab != nil {
		table := tab.Page

		table.RemoveHighlightAll()

	}

	TabbedPane.RemoveHighlight()

	App.SetFocus(Tree)

	FocusedWrapper = "left"
}
