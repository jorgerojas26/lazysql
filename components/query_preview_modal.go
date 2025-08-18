package components

import (
	"fmt"
	"slices"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/lib"
	"github.com/jorgerojas26/lazysql/models"
)

type QueryPreviewModal struct {
	tview.Primitive
	Queries  *[]models.DBDMLChange
	Table    *tview.Table
	DBDriver drivers.Driver
	Error    *tview.Modal
}

func NewQueryPreviewModal(queries *[]models.DBDMLChange, dbdriver drivers.Driver, onFinish func()) *QueryPreviewModal {
	modal := func(p tview.Primitive) tview.Primitive {
		return tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(p, 0, 3, true).
				AddItem(nil, 0, 1, false), 0, 3, true).
			AddItem(nil, 0, 1, false)
	}

	container := tview.NewFlex().SetDirection(tview.FlexColumnCSS)

	table := tview.NewTable()

	table.SetBorders(true)
	table.SetBorder(true)
	table.SetTitle(" Queries ")
	table.SetSelectable(true, false)
	table.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))

	errorModal := tview.NewModal()
	errorModal.AddButtons([]string{"Ok"})
	errorModal.SetText("An error occurred")
	errorModal.SetBackgroundColor(tcell.ColorRed)
	errorModal.SetTextColor(app.Styles.PrimaryTextColor)
	errorModal.SetButtonStyle(tcell.StyleDefault.Foreground(app.Styles.PrimaryTextColor))
	errorModal.SetFocus(0)

	keybindings := tview.NewTextView()
	keybindings.SetDynamicColors(true)
	keybindings.SetRegions(true)
	keybindings.SetWrap(false)
	keybindings.SetBorder(true)
	keybindings.SetTitle(" Keybindings ")

	for _, command := range app.Keymaps.Group(app.QueryPreviewGroup) {
		keybindings.SetText(fmt.Sprintf("%s [yellow](%s) [default]%s", keybindings.GetText(false), command.Key.String(), command.Description))
	}

	container.AddItem(table, 0, 1, true)
	container.AddItem(keybindings, 3, 1, false)

	r := &QueryPreviewModal{
		Primitive: modal(container),
		Queries:   queries,
		Table:     table,
		DBDriver:  dbdriver,
		Error:     errorModal,
	}

	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := app.Keymaps.Group(app.QueryPreviewGroup).Resolve(event)

		if command == commands.Quit || event.Key() == tcell.KeyEsc {
			mainPages.RemovePage(pageNameDMLPreview)
		} else if command == commands.Save {
			confirmationModal := NewConfirmationModal("Are you sure you want to save the queries?")

			confirmationModal.SetDoneFunc(func(_ int, buttonLabel string) {
				if buttonLabel == "Yes" {
					err := dbdriver.ExecutePendingChanges(*queries)
					if err != nil {
						r.SetError(err.Error())
						return
					}

					onFinish()
				}

				mainPages.RemovePage(pageNameConfirmation)
				mainPages.RemovePage(pageNameDMLPreview)
			})

			mainPages.AddPage(pageNameConfirmation, confirmationModal, true, true)

		} else if command == commands.Copy {
			row, col := table.GetSelection()
			queryStr := table.GetCell(row, col).Text

			clipboard := lib.NewClipboard()

			err := clipboard.Write(queryStr)
			if err != nil {
				logger.Info("Error copying query", map[string]any{"error": err.Error()})
				return event
			}
		} else if command == commands.Delete {
			row, _ := table.GetSelection()

			confirmationModal := NewConfirmationModal("Are you sure you want to delete the query?")

			confirmationModal.SetDoneFunc(func(_ int, buttonLabel string) {
				if buttonLabel == "Yes" {
					*queries = slices.Delete((*queries), row, row+1)
					table.Clear()
					r.populateTable()
				}

				mainPages.RemovePage(pageNameConfirmation)
			})

			mainPages.AddPage(pageNameConfirmation, confirmationModal, true, true)
		}

		return event
	})

	r.populateTable()

	return r
}

func (modal *QueryPreviewModal) SetError(err string) {
	modal.Error.SetText(err)

	modal.Error.SetDoneFunc(func(_ int, _ string) {
		mainPages.RemovePage(pageNameQueryPreviewError)
	})

	mainPages.AddPage(pageNameQueryPreviewError, modal.Error, true, true)
	mainPages.ShowPage(pageNameQueryPreviewError)
	App.SetFocus(modal.Error)
}

func (modal *QueryPreviewModal) populateTable() {
	modal.Table.Clear()

	for i, query := range *modal.Queries {

		queryStr, err := modal.DBDriver.DMLChangeToQueryString(query)
		if err != nil {
			return
		}

		cell := tview.NewTableCell(tview.Escape(queryStr))
		cell.SetExpansion(1)

		modal.Table.SetCell(i, 0, cell)
	}
}
