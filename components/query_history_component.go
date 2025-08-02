package components

import (
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
	"github.com/rivo/tview"
)

// QueryHistoryComponent is a component that displays query history.
type QueryHistoryComponent struct {
	tview.Primitive
	table            *tview.Table
	filterInput      *tview.InputField
	originalHistory  []models.QueryHistoryItem
	displayedHistory []models.QueryHistoryItem
	onQuerySelected  func(query string)
}

// NewQueryHistoryComponent creates a new QueryHistoryComponent.
func NewQueryHistoryComponent(history []models.QueryHistoryItem, onSelect func(query string)) *QueryHistoryComponent {
	qhc := &QueryHistoryComponent{
		originalHistory: history,
		onQuerySelected: onSelect,
	}

	qhc.filterInput = tview.NewInputField().
		SetLabel("Filter: ").
		SetFieldWidth(30)
	qhc.filterInput.SetBorder(true).SetTitle(" Filter Query History ")

	qhc.table = tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false).
		SetFixed(1, 0)
	qhc.table.SetBorder(true)

	qhc.table.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))
	qhc.table.SetBorderColor(app.Styles.PrimaryTextColor)
	qhc.table.SetTitle(" Query History (Newest First) ")

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(qhc.filterInput, 3, 0, false).
		AddItem(qhc.table, 0, 1, true)

	qhc.Primitive = layout

	logger.Info("QueryHistoryComponent loaded history.", map[string]any{"items": history})
	qhc.populateTable(history)

	qhc.filterInput.SetChangedFunc(func(text string) {
		qhc.filterTable(text)
	})

	qhc.table.SetSelectedFunc(func(row int, _ int) {
		if row > 0 && row-1 < len(qhc.displayedHistory) {
			selectedQuery := qhc.displayedHistory[row-1].QueryText
			if qhc.onQuerySelected != nil {
				qhc.onQuerySelected(selectedQuery)
			}
		}
	})

	return qhc
}

func (qhc *QueryHistoryComponent) populateTable(items []models.QueryHistoryItem) {
	qhc.table.Clear()
	qhc.displayedHistory = make([]models.QueryHistoryItem, len(items))
	copy(qhc.displayedHistory, items)

	sort.SliceStable(qhc.displayedHistory, func(i, j int) bool {
		return qhc.displayedHistory[i].Timestamp.After(qhc.displayedHistory[j].Timestamp)
	})

	headers := []string{"Timestamp", "Query"}
	for c, header := range headers {
		qhc.table.SetCell(0, c, tview.NewTableCell(header).
			SetSelectable(false).
			SetTextColor(app.Styles.TertiaryTextColor).
			SetAlign(tview.AlignCenter))
	}

	for r, item := range qhc.displayedHistory {
		qhc.table.SetCell(r+1, 0, tview.NewTableCell(item.Timestamp.Format("2006-01-02 15:04:05")).SetMaxWidth(20))
		firstLineQuery := strings.Split(item.QueryText, "\n")[0]
		if len(firstLineQuery) > 100 {
			firstLineQuery = firstLineQuery[:97] + "..."
		}
		qhc.table.SetCell(r+1, 1, tview.NewTableCell(firstLineQuery).SetExpansion(1))
	}

	if len(qhc.displayedHistory) > 0 {
		qhc.table.Select(1, 0)
	}
}

func (qhc *QueryHistoryComponent) filterTable(filterText string) {
	filterText = strings.ToLower(strings.TrimSpace(filterText))
	if filterText == "" {
		qhc.populateTable(qhc.originalHistory)
		return
	}

	var filteredHistory []models.QueryHistoryItem
	for _, item := range qhc.originalHistory {
		if strings.Contains(strings.ToLower(item.QueryText), filterText) {
			filteredHistory = append(filteredHistory, item)
		}
	}
	qhc.populateTable(filteredHistory)
}

// GetPrimitive returns the primitive for this component.
func (qhc *QueryHistoryComponent) GetPrimitive() tview.Primitive {
	return qhc.Primitive
}
