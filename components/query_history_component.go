package components

import (
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/internal/history"
	"github.com/jorgerojas26/lazysql/lib"
	"github.com/jorgerojas26/lazysql/models"
	"github.com/rivo/tview"
)

// QueryHistoryState holds the state for the QueryHistoryComponent.
type QueryHistoryState struct {
	isFiltering bool
}

// QueryHistoryComponent is a component that displays query history.
type QueryHistoryComponent struct {
	*tview.Flex
	state                *QueryHistoryState
	table                *tview.Table
	filterInput          *tview.InputField
	originalHistory      []models.QueryHistoryItem
	displayedHistory     []models.QueryHistoryItem
	onQuerySelected      func(query string)
	onSave               func()
	connectionIdentifier string
}

// NewQueryHistoryComponent creates a new QueryHistoryComponent.
func NewQueryHistoryComponent(connectionIdentifier string, onSelect func(query string), onSave func()) *QueryHistoryComponent {
	state := &QueryHistoryState{
		isFiltering: false,
	}

	qhc := &QueryHistoryComponent{
		state:                state,
		onQuerySelected:      onSelect,
		onSave:               onSave,
		connectionIdentifier: connectionIdentifier,
	}

	qhc.filterInput = tview.NewInputField().
		SetLabel("Search: ").
		SetFieldWidth(30).SetFieldStyle(
		tcell.StyleDefault.
			Background(app.Styles.SecondaryTextColor).
			Foreground(app.Styles.ContrastSecondaryTextColor),
	)

	qhc.filterInput.SetBorderPadding(1, 0, 1, 0)

	qhc.table = tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false).
		SetFixed(1, 0)

	qhc.table.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))
	qhc.table.SetBorderColor(app.Styles.PrimaryTextColor)
	qhc.table.SetTitle(" Query History (Newest First) ")

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(qhc.filterInput, 2, 0, false).
		AddItem(qhc.table, 0, 1, true)

	qhc.Flex = layout

	layout.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := app.Keymaps.Group(app.QueryHistoryGroup).Resolve(event)

		switch command {
		case commands.Save:
			qhc.showSaveQueryModal()
			return nil
		case commands.Search:
			qhc.SetIsFiltering(true)
			app.App.SetFocus(qhc.filterInput)
			return nil
		case commands.Copy:
			row, _ := qhc.table.GetSelection()
			queryStr := qhc.table.GetCell(row, 1).Text

			clipboard := lib.NewClipboard()

			err := clipboard.Write(queryStr)
			if err != nil {
				logger.Info("Error copying query", map[string]any{"error": err.Error()})
				return event
			}
			return nil
		}

		return event
	})

	qhc.filterInput.SetChangedFunc(func(text string) {
		qhc.filterTable(text)
	})

	qhc.filterInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter || key == tcell.KeyEscape {
			qhc.SetIsFiltering(false)
			App.SetFocus(qhc.table)
		}
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

func (qhc *QueryHistoryComponent) showSaveQueryModal() {
	row, _ := qhc.table.GetSelection()
	if row > 0 && row-1 < len(qhc.displayedHistory) {
		selectedQuery := qhc.displayedHistory[row-1].QueryText

		if selectedQuery != "" {
			saveModal := NewSaveQueryModal(qhc.connectionIdentifier, selectedQuery, func() {
				mainPages.RemovePage(pageNameSaveQuery)
				if qhc.onSave != nil {
					qhc.onSave()
				}
			})
			mainPages.AddPage(pageNameSaveQuery, saveModal, true, true)
			App.SetFocus(saveModal.form.GetFormItem(0))
		}
	}
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
			SetAlign(tview.AlignCenter).SetExpansion(c))
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
	return qhc.Flex
}

// SetIsFiltering sets the filtering state of the component.
func (qhc *QueryHistoryComponent) SetIsFiltering(filtering bool) {
	qhc.state.isFiltering = filtering
	if filtering {
		qhc.table.SetTitle(" Query History (Filtered) ")
	} else {
		qhc.table.SetTitle(" Query History (Newest First) ")
	}
}

func (qhc *QueryHistoryComponent) LoadHistory(connectionIdentifier string) {
	historyFilePath, err := history.GetHistoryFilePath(connectionIdentifier)
	if err != nil {
		logger.Error("Failed to get history file path", map[string]any{"error": err, "connection": connectionIdentifier})
		return
	}

	historyLimit := app.App.Config().MaxQueryHistoryPerConnection
	if historyLimit <= 0 {
		historyLimit = 100
	}

	historyItems, err := history.ReadHistory(historyFilePath, historyLimit)
	if err != nil {
		logger.Error("Failed to read query history", map[string]any{"error": err, "path": historyFilePath})
		// Show empty history on error, or an error message to the user.
		// For now, proceed with empty items.
		historyItems = []models.QueryHistoryItem{}
	}
	qhc.originalHistory = historyItems
	qhc.populateTable(historyItems)
	App.ForceDraw()
}

// GetIsFiltering returns the filtering state of the component.
func (qhc *QueryHistoryComponent) GetIsFiltering() bool {
	return qhc.state.isFiltering
}
