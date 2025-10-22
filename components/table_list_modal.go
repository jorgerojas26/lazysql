package components

import (
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
)

// TableListModal is a modal for selecting a table from the current database.
type TableListModal struct {
	*tview.Flex
	Filter        *tview.InputField
	Table         *tview.Table
	Wrapper       *tview.Flex
	Tables        []string
	SelectedTable string
	OnSelect      func(database, table string)
	DatabaseName  string
}

// NewTableListModal creates a new TableListModal.
// Pass the database name, list of tables, and a callback for when a table is selected.
func NewTableListModal(databaseName string, tables []string, onSelect func(database, table string)) *TableListModal {
	modal := &TableListModal{
		Flex:         tview.NewFlex().SetDirection(tview.FlexRow),
		Filter:       tview.NewInputField(),
		Table:        tview.NewTable(),
		Wrapper:      tview.NewFlex(),
		Tables:       tables,
		DatabaseName: databaseName,
		OnSelect:     onSelect,
	}

	modal.Table.SetBorder(true)
	modal.Table.SetBorderColor(app.Styles.PrimaryTextColor)
	modal.Table.SetTitle(" Tables ")
	modal.Table.SetSelectable(true, false)
	modal.Table.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))

	modal.Filter.SetLabel("Search table: ")
	modal.Filter.SetFieldWidth(30)
	modal.Filter.SetFieldBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	modal.Filter.SetChangedFunc(func(text string) {
		modal.fillTable(text)
	})

	modal.Wrapper.SetDirection(tview.FlexRow)

	modal.AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexColumn).
				AddItem(nil, 0, 1, false).
				AddItem(modal.Filter, 1, 0, true).
				AddItem(modal.Wrapper, 0, 1, true).
				AddItem(nil, 0, 1, false),
			0, 3, true).
		AddItem(nil, 0, 1, false)

	modal.Filter.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			modal.showSearchBar(false)
		}
		if key == tcell.KeyEnter {
			app.App.SetFocus(modal.Table)
		}
	})

	modal.Table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyRune && event.Rune() == '/' {
			modal.showSearchBar(true)
			return nil
		}

		command := app.Keymaps.Group(app.HomeGroup).Resolve(event)

		if command == commands.Quit || command == commands.ShowTableListModal || event.Key() == tcell.KeyEsc {
			modal.showSearchBar(false)
			mainPages.RemovePage(pageNameTableList)
			return nil
		}
		return event
	})

	modal.Table.SetSelectedFunc(func(row, column int) {
		if row >= 0 && row < modal.Table.GetRowCount() {
			cell := modal.Table.GetCell(row, 0)
			tableName := cell.Text
			modal.SelectedTable = tableName
			if modal.OnSelect != nil {
				modal.OnSelect(modal.DatabaseName, tableName)
			}
			mainPages.RemovePage(pageNameTableList)
		}
	})

	modal.showSearchBar(false)
	modal.fillTable("")

	return modal
}

func (modal *TableListModal) showSearchBar(show bool) {
	modal.Wrapper.Clear()
	if show {
		modal.Wrapper.
			AddItem(modal.Filter, 1, 0, true).
			AddItem(modal.Table, 0, 1, false)
		app.App.SetFocus(modal.Filter)
	} else {
		modal.Filter.SetText("")
		modal.fillTable("")
		modal.Wrapper.
			AddItem(modal.Table, 0, 1, true)
		app.App.SetFocus(modal.Table)
	}
}

// fillTable populates the table list, filtered by the search string.
func (modal *TableListModal) fillTable(filter string) {
	modal.Table.Clear()

	filtered := make([]string, 0, len(modal.Tables))
	filterLower := strings.ToLower(strings.TrimSpace(filter))

	for _, table := range modal.Tables {
		if filterLower == "" || fuzzy.Match(filterLower, strings.ToLower(table)) {
			filtered = append(filtered, table)
		}
	}

	slices.SortFunc(filtered, func(a, b string) int {
		lenA := len(a)
		lenB := len(b)

		if lenA < lenB {
			return -1
		} else if lenA > lenB {
			return 1
		}
		return 0
	})

	for i, table := range filtered {
		modal.Table.SetCell(i, 0, tview.NewTableCell(table).SetAlign(tview.AlignLeft).SetExpansion(1))
	}
	modal.Table.ScrollToBeginning()
}
