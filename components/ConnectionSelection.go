package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/models"
)

type ConnectionSelection struct {
	*tview.Flex
	StatusText *tview.TextView
}

var ConnectionListTable = NewConnectionsTable()

func NewConnectionSelection(connectionForm *ConnectionForm, connectionPages *models.ConnectionPages) *ConnectionSelection {
	wrapper := tview.NewFlex()

	wrapper.SetDirection(tview.FlexColumnCSS)

	buttonsWrapper := tview.NewFlex().SetDirection(tview.FlexRowCSS)

	newButton := tview.NewButton("[yellow]N[dark]ew")
	newButton.SetStyle(tcell.StyleDefault.Background(tview.Styles.PrimitiveBackgroundColor))
	newButton.SetBorder(true)

	buttonsWrapper.AddItem(newButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	connectButton := tview.NewButton("[yellow]C[dark]onnect")
	connectButton.SetStyle(tcell.StyleDefault.Background(tview.Styles.PrimitiveBackgroundColor))
	connectButton.SetBorder(true)

	buttonsWrapper.AddItem(connectButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	editButton := tview.NewButton("[yellow]E[dark]dit")
	editButton.SetStyle(tcell.StyleDefault.Background(tview.Styles.PrimitiveBackgroundColor))
	editButton.SetBorder(true)

	buttonsWrapper.AddItem(editButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	deleteButton := tview.NewButton("[yellow]D[dark]elete")
	deleteButton.SetStyle(tcell.StyleDefault.Background(tview.Styles.PrimitiveBackgroundColor))
	deleteButton.SetBorder(true)

	buttonsWrapper.AddItem(deleteButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	quitButton := tview.NewButton("[yellow]Q[dark]uit")
	quitButton.SetStyle(tcell.StyleDefault.Background(tview.Styles.PrimitiveBackgroundColor))
	quitButton.SetBorder(true)

	statusText := tview.NewTextView()
	statusText.SetBorderPadding(1, 1, 0, 0)

	wrapper.AddItem(ConnectionListTable, 0, 1, true)
	wrapper.AddItem(statusText, 4, 0, false)
	wrapper.AddItem(buttonsWrapper, 3, 0, false)

	cs := &ConnectionSelection{
		Flex:       wrapper,
		StatusText: statusText,
	}

	wrapper.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		connections := ConnectionListTable.GetConnections()

		if len(connections) != 0 {
			row, _ := ConnectionListTable.GetSelection()
			selectedConnection := connections[row]

			if event.Rune() == 'c' || event.Key() == tcell.KeyEnter {
				go cs.Connect(selectedConnection)
			} else if event.Rune() == 'e' {
				connectionPages.SwitchToPage("ConnectionForm")
				connectionForm.GetFormItemByLabel("Name").(*tview.InputField).SetText(selectedConnection.Name)
				connectionForm.GetFormItemByLabel("URL").(*tview.InputField).SetText(selectedConnection.URL)
				connectionForm.StatusText.SetText("")

				connectionForm.SetAction("edit")
				return nil

			} else if event.Rune() == 'd' {
				confirmationModal := NewConfirmationModal("")

				confirmationModal.SetDoneFunc(func(_ int, buttonLabel string) {
					MainPages.RemovePage("Confirmation")
					confirmationModal = nil

					if buttonLabel == "Yes" {
						newConnections := append(connections[:row], connections[row+1:]...)

						err := helpers.SaveConnectionConfig(newConnections)
						if err != nil {
							ConnectionListTable.SetError(err.Error())
						} else {
							ConnectionListTable.SetConnections(newConnections)
						}

					}
				})

				MainPages.AddPage("Confirmation", confirmationModal, true, true)

				return nil
			}
		}

		if event.Rune() == 'n' {
			connectionForm.SetAction("create")
			connectionForm.GetFormItemByLabel("Name").(*tview.InputField).SetText("")
			connectionForm.GetFormItemByLabel("URL").(*tview.InputField).SetText("")
			connectionForm.StatusText.SetText("")
			connectionPages.SwitchToPage("ConnectionForm")
		} else if event.Rune() == 'q' {
			if wrapper.HasFocus() {
				app.App.Stop()
			}
		}

		return event
	})

	return cs
}

func (cs *ConnectionSelection) Connect(connection models.Connection) {
	if MainPages.HasPage(connection.URL) {
		MainPages.SwitchToPage(connection.URL)
		App.Draw()
	} else {
		cs.StatusText.SetText("Connecting...").SetTextColor(tview.Styles.TertiaryTextColor)
		App.Draw()

		var newDbDriver drivers.Driver

		switch connection.Provider {
		case "mysql":
			newDbDriver = &drivers.MySQL{}
		case "postgres":
			newDbDriver = &drivers.Postgres{}
		case "sqlite3":
			newDbDriver = &drivers.SQLite{}
		}

		err := newDbDriver.Connect(connection.URL)

		if err != nil {
			cs.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
			App.Draw()
		} else {
			newHome := NewHomePage(connection, newDbDriver)

			MainPages.AddAndSwitchToPage(connection.URL, newHome, true)

			cs.StatusText.SetText("")
			App.Draw()

			selectedRow, selectedCol := ConnectionListTable.GetSelection()
			cell := ConnectionListTable.GetCell(selectedRow, selectedCol)
			cell.SetText(fmt.Sprintf("[green]* %s", cell.Text))

			ConnectionListTable.SetCell(selectedRow, selectedCol, cell)

			MainPages.SwitchToPage(connection.URL)
			newHome.Tree.SetCurrentNode(newHome.Tree.GetRoot())
			newHome.Tree.SetTitle(fmt.Sprintf("%s (%s)", connection.Name, strings.ToUpper(connection.Provider)))
			App.SetFocus(newHome.Tree)
			App.Draw()
		}

	}
}
