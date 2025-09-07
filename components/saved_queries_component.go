package components

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/internal/saved"
	"github.com/jorgerojas26/lazysql/lib"
	"github.com/jorgerojas26/lazysql/models"
)

// SavedQueriesState holds the state for the SavedQueriesComponent.
type SavedQueriesState struct {
	isFiltering bool
}

// SavedQueriesComponent is a component that displays saved queries.
type SavedQueriesComponent struct {
	Primitive            tview.Primitive
	state                *SavedQueriesState
	table                *tview.Table
	filterInput          *tview.InputField
	originalQueries      []models.SavedQuery
	displayedQueries     []models.SavedQuery
	onQuerySelected      func(query string)
	connectionIdentifier string
}

// NewSavedQueriesComponent creates a new SavedQueriesComponent.
func NewSavedQueriesComponent(connectionIdentifier string, onSelect func(query string)) *SavedQueriesComponent {
	state := &SavedQueriesState{
		isFiltering: false,
	}

	sqc := &SavedQueriesComponent{
		state:                state,
		onQuerySelected:      onSelect,
		connectionIdentifier: connectionIdentifier,
	}

	sqc.filterInput = tview.NewInputField().
		SetLabel("Search: ").
		SetFieldWidth(30).SetFieldStyle(
		tcell.StyleDefault.
			Background(app.Styles.SecondaryTextColor).
			Foreground(app.Styles.ContrastSecondaryTextColor),
	)
	sqc.filterInput.SetBorderPadding(1, 0, 1, 0)

	sqc.table = tview.NewTable().
		SetBorders(true).
		SetSelectable(true, false).
		SetFixed(1, 0)

	sqc.table.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))
	sqc.table.SetBorderColor(app.Styles.PrimaryTextColor)
	sqc.table.SetTitle(" Saved Queries ")

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(sqc.filterInput, 2, 0, false).
		AddItem(sqc.table, 0, 1, true)

	sqc.Primitive = layout

	sqc.filterInput.SetChangedFunc(func(text string) {
		sqc.filterTable(text)
	})

	sqc.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := app.Keymaps.Group(app.QueryHistoryGroup).Resolve(event)

		switch command {
		case commands.Delete:
			sqc.handleDelete()
		case commands.Search:
			sqc.SetIsFiltering(true)
			app.App.SetFocus(sqc.filterInput)
		case commands.Copy:
			row, _ := sqc.table.GetSelection()
			queryStr := sqc.table.GetCell(row, 1).GetReference().(string)

			clipboard := lib.NewClipboard()

			err := clipboard.Write(queryStr)
			if err != nil {
				logger.Info("Error copying query", map[string]any{"error": err.Error()})
				return event
			}

		}
		return event
	})

	sqc.filterInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter || key == tcell.KeyEscape {
			sqc.SetIsFiltering(false)
			app.App.SetFocus(sqc.table)
		}
	})

	sqc.table.SetSelectedFunc(func(row int, _ int) {
		if row > 0 && row-1 < len(sqc.displayedQueries) {
			selectedQuery := sqc.displayedQueries[row-1].Query
			if sqc.onQuerySelected != nil {
				sqc.onQuerySelected(selectedQuery)
			}
		}
	})

	sqc.loadQueries()

	return sqc
}

func (sqc *SavedQueriesComponent) handleDelete() {
	row, _ := sqc.table.GetSelection()
	if row > 0 && row-1 < len(sqc.displayedQueries) {
		selectedQuery := sqc.displayedQueries[row-1]

		confirmation := NewConfirmationModal("")
		confirmation.SetText("Are you sure you want to delete this query?")
		confirmation.SetDoneFunc(func(_ int, buttonLabel string) {
			if buttonLabel == "Yes" {
				err := saved.DeleteSavedQuery(sqc.connectionIdentifier, selectedQuery.Name)
				if err != nil {
					// TODO: Show error
					return
				}
				sqc.loadQueries()
			}
			mainPages.RemovePage(pageNameSavedQueryDelete)
		})

		mainPages.AddPage(pageNameSavedQueryDelete, confirmation, true, true)
	}
}

func (sqc *SavedQueriesComponent) loadQueries() {
	queries, err := saved.ReadSavedQueries(sqc.connectionIdentifier)
	if err != nil {
		// TODO: Show error
		return
	}
	sqc.originalQueries = queries
	sqc.populateTable(queries)
}

func (sqc *SavedQueriesComponent) populateTable(queries []models.SavedQuery) {
	sqc.table.Clear()
	sqc.displayedQueries = queries

	headers := []string{"Name", "Query"}
	for c, header := range headers {
		sqc.table.SetCell(0, c, tview.NewTableCell(header).
			SetSelectable(false).
			SetTextColor(app.Styles.TertiaryTextColor).
			SetAlign(tview.AlignCenter).SetExpansion(c))
	}

	for r, item := range sqc.displayedQueries {
		sqc.table.SetCell(r+1, 0, tview.NewTableCell(item.Name).SetMaxWidth(30))
		queryCell := tview.NewTableCell(item.Query).SetExpansion(1)
		queryCell.SetReference(item.Query)
		sqc.table.SetCell(r+1, 1, queryCell)
	}

	if len(sqc.displayedQueries) > 0 {
		sqc.table.Select(1, 0)
	}
}

func (sqc *SavedQueriesComponent) filterTable(filterText string) {
	filterText = strings.ToLower(strings.TrimSpace(filterText))
	if filterText == "" {
		sqc.populateTable(sqc.originalQueries)
		return
	}

	var filteredQueries []models.SavedQuery
	for _, item := range sqc.originalQueries {
		if strings.Contains(strings.ToLower(item.Name), filterText) {
			filteredQueries = append(filteredQueries, item)
		}
	}
	sqc.populateTable(filteredQueries)
}

// GetPrimitive returns the primitive for this component.
func (sqc *SavedQueriesComponent) GetPrimitive() tview.Primitive {
	return sqc.Primitive
}

// Refresh reloads the saved queries from the file.
func (sqc *SavedQueriesComponent) Refresh() {
	sqc.loadQueries()
}

// SetIsFiltering sets the filtering state of the component.
func (sqc *SavedQueriesComponent) SetIsFiltering(filtering bool) {
	sqc.state.isFiltering = filtering
	if filtering {
		sqc.table.SetTitle(" Saved Queries (Filtered) ")
	} else {
		sqc.table.SetTitle(" Saved Queries ")
	}
}

// GetIsFiltering returns the filtering state of the component.
func (sqc *SavedQueriesComponent) GetIsFiltering() bool {
	return sqc.state.isFiltering
}
