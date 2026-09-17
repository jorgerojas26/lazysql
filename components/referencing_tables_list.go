package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
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
type ReferencingTablesList struct {
	tview.Primitive
	list *tview.List
}

// NewReferencingTablesList builds the picker. onSelect receives the index of
// the chosen entry, onCancel is called when the user dismisses the picker.
func NewReferencingTablesList(entries []referencingTableEntry, useSchemas bool, onSelect func(index int), onCancel func()) *ReferencingTablesList {
	rtl := &ReferencingTablesList{list: tview.NewList()}

	rtl.list.ShowSecondaryText(true)
	rtl.list.SetBorder(false)
	rtl.list.SetMainTextColor(app.Styles.PrimaryTextColor)
	rtl.list.SetSecondaryTextColor(app.Styles.TertiaryTextColor)
	rtl.list.SetSelectedStyle(tcell.StyleDefault.
		Background(app.Styles.SecondaryTextColor).
		Foreground(tview.Styles.ContrastSecondaryTextColor))

	for _, entry := range entries {
		rtl.list.AddItem(
			entry.QualifiedTable(useSchemas),
			fmt.Sprintf("%s → %s", entry.Column, entry.ReferencedColumn),
			0,
			nil,
		)
	}

	rtl.list.SetSelectedFunc(func(index int, _ string, _ string, _ rune) {
		onSelect(index)
	})

	rtl.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEsc, event.Rune() == 'q':
			onCancel()
			return nil
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}

		return event
	})

	hint := tview.NewTextView().
		SetText("Enter to open filtered · Esc to cancel").
		SetTextAlign(tview.AlignCenter).
		SetTextColor(app.Styles.TertiaryTextColor)

	listWithHint := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(rtl.list, 0, 1, true).
		AddItem(hint, 1, 0, false)
	listWithHint.SetBorder(true).
		SetTitle(" Referencing tables ").
		SetTitleAlign(tview.AlignLeft)

	height := len(entries)*2 + 4
	if height > 24 {
		height = 24
	}

	grid := tview.NewGrid().
		SetRows(0, height, 0).
		SetColumns(0, 60, 0).
		AddItem(listWithHint, 1, 1, 1, 1, 0, 0, true)

	rtl.Primitive = grid

	return rtl
}

// GetList exposes the inner list so callers can focus it.
func (rtl *ReferencingTablesList) GetList() *tview.List {
	return rtl.list
}
