package pages

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"lazysql/app"
	"lazysql/components"
	"lazysql/drivers"
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

		}

		return event
	})
	LeftWrapper.AddItem(Tree, 0, 1, true)

	RightWrapper.AddItem(TabbedPane.Wrapper, 2, 0, false)
	RightWrapper.AddItem(TabbedPane.Pages, 0, 1, false)

	HomePage.AddItem(LeftWrapper, 30, 1, true)
	HomePage.AddItem(RightWrapper, 0, 5, false)

	HomePage.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		tab := TabbedPane.GetCurrentTab()

		if event.Rune() == 'H' {
			focusLeftWrapper()
		} else if event.Rune() == 'L' {
			focusRightWrapper()
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

			columns := drivers.Database.DescribeTable(tableName)
			constraints := drivers.Database.GetTableConstraints(tableName)
			foreignKeys := drivers.Database.GetTableForeignKeys(tableName)
			indexes := drivers.Database.GetTableIndexes(tableName)

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

			table.SetLoading(true)
			records, err := drivers.Database.GetRecords(tableName, "", "", 0, 100, true)
			if err != nil {
				table.SetError(err.Error(), func() {
					focusLeftWrapper()
				})
				return
			}

			table.SetRecords(records)
			table.SetColumns(columns)
			table.SetConstraints(constraints)
			table.SetForeignKeys(foreignKeys)
			table.SetIndexes(indexes)
			table.SetDBReference(tableName)
			table.Select(1, 0)

			focusRightWrapper()
			table.SetLoading(false)
		}
	}
}

func focusRightWrapper() {
	Tree.RemoveHighlight()

	RightWrapper.SetBorderColor(app.FocusTextColor)
	LeftWrapper.SetBorderColor(app.BlurTextColor)
	tab := TabbedPane.GetCurrentTab()

	if tab != nil {
		focusTab(tab)
	}
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

	App.SetFocus(Tree)
}
