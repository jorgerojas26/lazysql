package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
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

	newButton := tview.NewButton("[darkred]N[black]ew")
	newButton.SetStyle(tcell.StyleDefault.Background(tview.Styles.PrimaryTextColor))
	buttonsWrapper.AddItem(newButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	connectButton := tview.NewButton("[darkred]C[black]onnect")
	connectButton.SetStyle(tcell.StyleDefault.Background(tview.Styles.PrimaryTextColor))
	buttonsWrapper.AddItem(connectButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	editButton := tview.NewButton("[darkred]E[black]dit")
	editButton.SetStyle(tcell.StyleDefault.Background(tview.Styles.PrimaryTextColor))
	buttonsWrapper.AddItem(editButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	deleteButton := tview.NewButton("[darkred]D[black]elete")
	deleteButton.SetStyle(tcell.StyleDefault.Background(tview.Styles.PrimaryTextColor))
	buttonsWrapper.AddItem(deleteButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	quitButton := tview.NewButton("[darkred]Q[black]uit")
	quitButton.SetStyle(tcell.StyleDefault.Background(tview.Styles.PrimaryTextColor))
	buttonsWrapper.AddItem(quitButton, 0, 1, false)

	statusText := tview.NewTextView()
	statusText.SetBorderPadding(0, 1, 0, 0)

	wrapper.AddItem(ConnectionListTable, 0, 1, true)
	wrapper.AddItem(statusText, 3, 0, false)
	wrapper.AddItem(buttonsWrapper, 1, 0, false)

	cs := &ConnectionSelection{
		Flex:       wrapper,
		StatusText: statusText,
	}

	wrapper.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		connections := ConnectionListTable.GetConnections()

		command := app.Keymaps.Group(app.ConnectionGroup).Resolve(event)

		if len(connections) != 0 {
			row, _ := ConnectionListTable.GetSelection()
			selectedConnection := connections[row]

			switch command {
			case commands.Connect:
				go cs.Connect(selectedConnection)
			case commands.EditConnection:
				connectionPages.SwitchToPage("ConnectionForm")
				connectionForm.GetFormItemByLabel("Name").(*tview.InputField).SetText(selectedConnection.Name)
				connectionForm.GetFormItemByLabel("URL").(*tview.InputField).SetText(selectedConnection.URL)
				connectionForm.StatusText.SetText("")

				connectionForm.SetAction("edit")
				return nil
			case commands.DeleteConnection:
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

		switch command {
		case commands.NewConnection:
			connectionForm.SetAction("create")
			connectionForm.GetFormItemByLabel("Name").(*tview.InputField).SetText("")
			connectionForm.GetFormItemByLabel("URL").(*tview.InputField).SetText("")
			connectionForm.StatusText.SetText("")
			connectionPages.SwitchToPage("ConnectionForm")
		case commands.Quit:
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
