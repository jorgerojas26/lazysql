package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/drivers"
)

// referencingTableEntry describes a single foreign key pointing at the table
// that is currently open, i.e. the reverse direction of a foreign key jump.
type referencingTableEntry struct {
	Schema           string
	Table            string
	Column           string
	ReferencedColumn string
}

// QualifiedTable returns the table name in the shape the driver expects when
// opening a new tab for it.
func (entry referencingTableEntry) QualifiedTable(useSchemas bool) string {
	if useSchemas && entry.Schema != "" {
		return fmt.Sprintf("%s.%s", entry.Schema, entry.Table)
	}

	return entry.Table
}

func useQualifiedReferencingTables(driverUsesSchemas bool, provider string) bool {
	return driverUsesSchemas || provider == drivers.DriverMSSQL
}

// buildReferencingEntries converts driver rows (a header row followed by rows
// shaped as constraint_name, table_schema, table_name, column_name,
// referenced_column_name) into entries.
//
// Composite foreign keys are skipped: a single cell value cannot filter a multi
// column relationship, which is the same rule handleForeignKeyEnter applies in
// the forward direction.
func buildReferencingEntries(rows [][]string) []referencingTableEntry {
	if len(rows) <= 1 {
		return nil
	}

	constraintKey := func(row []string) string {
		return strings.Join([]string{row[1], row[2], row[0]}, "\x00")
	}

	constraintColumnCounts := map[string]int{}

	for _, row := range rows[1:] {
		if len(row) < 5 {
			continue
		}
		constraintColumnCounts[constraintKey(row)]++
	}

	entries := make([]referencingTableEntry, 0, len(rows)-1)

	for _, row := range rows[1:] {
		if len(row) < 5 {
			continue
		}

		if constraintColumnCounts[constraintKey(row)] != 1 {
			continue
		}

		if row[2] == "" || row[3] == "" || row[4] == "" {
			continue
		}

		entries = append(entries, referencingTableEntry{
			Schema:           row[1],
			Table:            row[2],
			Column:           row[3],
			ReferencedColumn: row[4],
		})
	}

	if len(entries) == 0 {
		return nil
	}

	return entries
}

// ReferencingTablesList lets the user pick which referencing table to open.
// Its layout intentionally mirrors HelpModal so the table is drawn on an
// opaque, bordered surface instead of over the records behind it.
type ReferencingTablesList struct {
	*tview.Flex
	Filter          *tview.InputField
	Table           *tview.Table
	Wrapper         *tview.Flex
	hint            *tview.TextView
	entries         []referencingTableEntry
	filteredIndices []int
	useSchemas      bool
	onSelect        func(index int)
	onCancel        func()
}

// NewReferencingTablesList builds the picker. onSelect receives the index of
// the chosen entry, onCancel is called when the user dismisses the picker.
func NewReferencingTablesList(entries []referencingTableEntry, useSchemas bool, onSelect func(index int), onCancel func()) *ReferencingTablesList {
	rtl := &ReferencingTablesList{
		Flex:       tview.NewFlex().SetDirection(tview.FlexRow),
		Filter:     tview.NewInputField(),
		Table:      tview.NewTable(),
		Wrapper:    tview.NewFlex().SetDirection(tview.FlexRow),
		hint:       tview.NewTextView(),
		entries:    entries,
		useSchemas: useSchemas,
		onSelect:   onSelect,
		onCancel:   onCancel,
	}

	rtl.Table.SetBorder(true)
	rtl.Table.SetBorderColor(app.Styles.PrimaryTextColor)
	rtl.Table.SetTitleColor(app.Styles.SecondaryTextColor)
	rtl.Table.SetTitleAlign(tview.AlignLeft)
	rtl.Table.SetSelectable(true, false)
	rtl.Table.SetFixed(1, 0)
	rtl.Table.SetSelectedStyle(tcell.StyleDefault.
		Background(app.Styles.SecondaryTextColor).
		Foreground(tview.Styles.ContrastSecondaryTextColor))

	rtl.Filter.SetLabel("Search references: ")
	rtl.Filter.SetFieldBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	rtl.Filter.SetFieldTextColor(app.Styles.PrimaryTextColor)
	rtl.Filter.SetLabelColor(app.Styles.SecondaryTextColor)
	rtl.Filter.SetChangedFunc(rtl.fillTable)

	rtl.hint.
		SetText("/ search    ↑/↓ · j/k · Ctrl+n/p move    Enter open    Esc/q close").
		SetTextAlign(tview.AlignCenter).
		SetTextColor(app.Styles.TertiaryTextColor)

	rtl.AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexColumn).
				AddItem(nil, 0, 1, false).
				AddItem(rtl.Wrapper, 0, 3, true).
				AddItem(nil, 0, 1, false),
			0, 3, true).
		AddItem(nil, 0, 1, false)

	rtl.Table.SetSelectedFunc(func(row, _ int) {
		if row <= 0 || row > len(rtl.filteredIndices) {
			return
		}
		rtl.onSelect(rtl.filteredIndices[row-1])
	})

	rtl.Table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEsc, event.Rune() == 'q':
			rtl.onCancel()
			return nil
		case event.Rune() == '/':
			rtl.showSearchBar(true)
			return nil
		case event.Key() == tcell.KeyCtrlN, event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Key() == tcell.KeyCtrlP, event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}

		return event
	})

	rtl.Filter.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEsc:
			rtl.showSearchBar(false)
		case tcell.KeyEnter:
			row, _ := rtl.Table.GetSelection()
			if row > 0 && row <= len(rtl.filteredIndices) {
				rtl.onSelect(rtl.filteredIndices[row-1])
			}
		}
	})
	rtl.Filter.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlN:
			rtl.moveSelection(1)
			return nil
		case tcell.KeyCtrlP:
			rtl.moveSelection(-1)
			return nil
		}
		return event
	})

	rtl.fillTable("")
	rtl.showSearchBar(false)

	return rtl
}

func (rtl *ReferencingTablesList) fillTable(filter string) {
	rtl.Table.Clear()
	rtl.filteredIndices = rtl.filteredIndices[:0]

	rtl.Table.SetCell(0, 0, tview.NewTableCell("TABLE").
		SetTextColor(app.Styles.SecondaryTextColor).
		SetSelectable(false).
		SetExpansion(1))
	rtl.Table.SetCell(0, 1, tview.NewTableCell("RELATIONSHIP").
		SetTextColor(app.Styles.SecondaryTextColor).
		SetSelectable(false).
		SetExpansion(1))

	query := strings.ToLower(strings.TrimSpace(filter))
	for index, entry := range rtl.entries {
		searchable := strings.ToLower(strings.Join([]string{
			entry.QualifiedTable(rtl.useSchemas),
			entry.Column,
			entry.ReferencedColumn,
		}, " "))
		if query != "" && !strings.Contains(searchable, query) {
			continue
		}

		rtl.filteredIndices = append(rtl.filteredIndices, index)
		row := len(rtl.filteredIndices)
		rtl.Table.SetCell(row, 0, tview.NewTableCell(entry.QualifiedTable(rtl.useSchemas)).
			SetTextColor(app.Styles.PrimaryTextColor).
			SetExpansion(1))
		rtl.Table.SetCell(row, 1, tview.NewTableCell(fmt.Sprintf("%s  →  %s", entry.Column, entry.ReferencedColumn)).
			SetTextColor(app.Styles.TertiaryTextColor).
			SetExpansion(1))
	}

	if len(rtl.filteredIndices) == 0 {
		rtl.Table.SetCell(1, 0, tview.NewTableCell("No matching referencing tables").
			SetTextColor(app.Styles.TertiaryTextColor).
			SetSelectable(false).
			SetExpansion(1))
		rtl.Table.SetCell(1, 1, tview.NewTableCell("").SetSelectable(false))
	}

	title := fmt.Sprintf(" Referencing tables (%d) ", len(rtl.filteredIndices))
	if query != "" {
		title = fmt.Sprintf(" Referencing tables (%d/%d) ", len(rtl.filteredIndices), len(rtl.entries))
	}
	rtl.Table.SetTitle(title)
	rtl.Table.Select(1, 0)
	rtl.Table.ScrollToBeginning()
}

func (rtl *ReferencingTablesList) showSearchBar(show bool) {
	rtl.Wrapper.Clear()
	rtl.Wrapper.AddItem(rtl.Table, 0, 1, true)

	if show {
		rtl.Wrapper.AddItem(rtl.Filter, 1, 0, true)
		rtl.Wrapper.AddItem(rtl.hint, 1, 0, false)
		App.SetFocus(rtl.Filter)
		return
	}

	rtl.Filter.SetText("")
	rtl.Wrapper.AddItem(rtl.hint, 1, 0, false)
	App.SetFocus(rtl.Table)
}

func (rtl *ReferencingTablesList) moveSelection(delta int) {
	if len(rtl.filteredIndices) == 0 {
		return
	}

	row, _ := rtl.Table.GetSelection()
	row += delta
	if row < 1 {
		row = 1
	}
	if row > len(rtl.filteredIndices) {
		row = len(rtl.filteredIndices)
	}
	rtl.Table.Select(row, 0)
}

// GetTable exposes the inner table so callers can focus it.
func (rtl *ReferencingTablesList) GetTable() *tview.Table {
	return rtl.Table
}
