package components

import (
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/models"
)

// QueryHistoryModal displays the query history for the current connection.
type QueryHistoryModal struct {
	tview.Primitive
	table            *tview.Table
	filterInput      *tview.InputField
	originalHistory  []models.QueryHistoryItem
	displayedHistory []models.QueryHistoryItem
	onQuerySelected  func(query string)
	parentFlex       *tview.Flex // The main modal flex container for centering
}

// NewQueryHistoryModal creates a new modal to display query history.
func NewQueryHistoryModal(history []models.QueryHistoryItem, onSelect func(query string)) *QueryHistoryModal {
	qhm := &QueryHistoryModal{
		originalHistory: history,
		onQuerySelected: onSelect,
	}

	qhm.filterInput = tview.NewInputField().
		SetLabel("Filter: ").
		SetFieldWidth(30)
	qhm.filterInput.SetBorder(true).SetTitle(" Filter Query History ")

	qhm.table = tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false). // Rows selectable, not columns
		SetFixed(1, 0)              // Fix header row
	qhm.table.SetBorder(true)

	qhm.table.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))
	qhm.table.SetBorderColor(app.Styles.PrimaryTextColor)
	qhm.table.SetTitle(" Query History (Newest First) ")

	// Layout: Input field on top, table below
	contentFlex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(qhm.filterInput, 3, 0, false). // Input field takes 3 rows height, has focus
		AddItem(qhm.table, 0, 1, true)         // Table takes remaining space

	// Centering modal logic (similar to HelpModal)
	qhm.parentFlex = tview.NewFlex().
		AddItem(nil, 0, 1, false). // Left space
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).        // Top space
			AddItem(contentFlex, 0, 8, true). // Modal content, 80% of inner height
			AddItem(nil, 0, 1, false),        // Bottom space
						0, 8, true). // Modal container, 80% of outer width
		AddItem(nil, 0, 1, false) // Right space

	qhm.Primitive = qhm.parentFlex // The primitive to return is the outer flex

	qhm.populateTable(history)

	qhm.filterInput.SetChangedFunc(func(text string) {
		qhm.filterTable(text)
	})

	qhm.table.SetSelectedFunc(func(row int, _ int) {
		if row > 0 && row-1 < len(qhm.displayedHistory) { // row 0 is header
			selectedQuery := qhm.displayedHistory[row-1].QueryText
			if qhm.onQuerySelected != nil {
				qhm.onQuerySelected(selectedQuery)
			}
		}
	})

	contentFlex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := app.Keymaps.Group(app.QueryHistoryGroup).Resolve(event)

		if command == commands.ToggleQueryHistory || (event.Key() == tcell.KeyEsc && !qhm.filterInput.HasFocus()) {
			mainPages.RemovePage(pageNameQueryHistory)
			return nil
		} else if event.Rune() == '/' {
			App.SetFocus(qhm.filterInput)
			return nil
		}

		// If input field has focus and user presses Down arrow, switch to table
		if qhm.filterInput.HasFocus() && event.Key() == tcell.KeyDown {
			if len(qhm.displayedHistory) > 0 {
				App.SetFocus(qhm.table)
				qhm.table.Select(1, 0) // Select first data row
				return nil             // Absorb event
			}
		}
		// // If table has focus and user types letters/numbers, switch to filterInput
		// if qhm.table.HasFocus() && (event.Key() == tcell.KeyRune) {
		// 	// Check if it's a character that should go to the input field
		// 	// This is a basic check; more sophisticated handling might be needed
		// 	// if certain runes should still be handled by the table.
		// 	App.SetFocus(qhm.filterInput)
		// 	// The filterInput will now receive subsequent key events.
		// 	// We don't need to explicitly pass this event to it.
		// 	// We return the original event so it can be processed by the newly focused primitive if tview's loop allows,
		// 	// or simply consumed if not. Often, the first event that triggers focus change is just for focus change.
		// 	return event // Or return nil if we want to ensure this event is fully consumed. Let's try returning event.
		// }
		return event
	})

	qhm.filterInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter, tcell.KeyDown:
			if len(qhm.displayedHistory) > 0 {
				App.SetFocus(qhm.table)
				qhm.table.Select(1, 0) // Select first data row
			}
		case tcell.KeyEsc:
			App.SetFocus(qhm.table)
		}
	})

	return qhm
}

// populateTable fills the history table with items.
func (qhm *QueryHistoryModal) populateTable(items []models.QueryHistoryItem) {
	qhm.table.Clear()
	qhm.displayedHistory = make([]models.QueryHistoryItem, len(items)) // Create a fresh slice
	copy(qhm.displayedHistory, items)                                  // Copy items to avoid modifying original slice during sort

	// Sort by timestamp descending (newest first)
	sort.SliceStable(qhm.displayedHistory, func(i, j int) bool {
		return qhm.displayedHistory[i].Timestamp.After(qhm.displayedHistory[j].Timestamp)
	})

	// Set headers
	headers := []string{"Timestamp", "Query"}
	for c, header := range headers {
		qhm.table.SetCell(0, c, tview.NewTableCell(header).
			SetSelectable(false).
			SetTextColor(app.Styles.TertiaryTextColor).
			SetAlign(tview.AlignCenter))
	}

	for r, item := range qhm.displayedHistory {
		qhm.table.SetCell(r+1, 0, tview.NewTableCell(item.Timestamp.Format("2006-01-02 15:04:05")).SetMaxWidth(20))
		// Show only the first line of the query for brevity in the table
		firstLineQuery := strings.Split(item.QueryText, "\n")[0]
		if len(firstLineQuery) > 100 { // Truncate very long first lines
			firstLineQuery = firstLineQuery[:97] + "..."
		}
		qhm.table.SetCell(r+1, 1, tview.NewTableCell(firstLineQuery).SetExpansion(1))
	}

	if len(qhm.displayedHistory) > 0 {
		qhm.table.Select(1, 0) // Select first data row if history exists
	}
}

// filterTable filters the displayed history based on the input text.
func (qhm *QueryHistoryModal) filterTable(filterText string) {
	filterText = strings.ToLower(strings.TrimSpace(filterText))
	if filterText == "" {
		qhm.populateTable(qhm.originalHistory)
		return
	}

	var filteredHistory []models.QueryHistoryItem
	for _, item := range qhm.originalHistory {
		if strings.Contains(strings.ToLower(item.QueryText), filterText) {
			filteredHistory = append(filteredHistory, item)
		}
	}
	qhm.populateTable(filteredHistory) // This will re-sort the filtered list
}

// GetPrimitive returns the top-level tview.Primitive for this modal.
func (qhm *QueryHistoryModal) GetPrimitive() tview.Primitive {
	return qhm.Primitive
}
