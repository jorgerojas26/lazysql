package components

import (
	"fmt"
	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/models"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ConnectionSelection struct {
	*tview.Flex
	StatusText *tview.TextView
}

var ConnectionListTable = NewConnectionsTable()

func NewConnectionSelection(connectionForm *ConnectionForm, connectionPages *models.ConnectionPages) *ConnectionSelection {

	buttonsWrapper := tview.NewFlex().SetDirection(tview.FlexRowCSS)

	newButton := tview.NewButton("[darkred]N[black]ew").
		SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(newButton, 0, 1, false).AddItem(nil, 1, 0, false)

	connectButton := tview.NewButton("[darkred]C[black]onnect").
		SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(connectButton, 0, 1, false).AddItem(nil, 1, 0, false)

	editButton := tview.NewButton("[darkred]E[black]dit").
		SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(editButton, 0, 1, false).AddItem(nil, 1, 0, false)

	deleteButton := tview.NewButton("[darkred]D[black]elete").
		SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(deleteButton, 0, 1, false).AddItem(nil, 1, 0, false)

	quitButton := tview.NewButton("[darkred]Q[black]uit").
		SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(quitButton, 0, 1, false).AddItem(nil, 1, 0, false)

	statusText := tview.NewTextView()
	statusText.SetBorderPadding(0, 1, 0, 0)

	wrapper := tview.NewFlex().SetDirection(tview.FlexColumnCSS).
		AddItem(ConnectionListTable, 0, 1, true).
		AddItem(statusText, 3, 0, false).
		AddItem(buttonsWrapper, 1, 0, false)

	cs := &ConnectionSelection{
		Flex:       wrapper,
		StatusText: statusText,
	}

	wrapper.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		connections := ConnectionListTable.GetConnections()
		if len(connections) == 0 {
			return event
		}

		row, _ := ConnectionListTable.GetSelection()
		selectedConnection := connections[row]

		eventKey := event.Rune()
		if eventKey == rune(0) {
			eventKey = rune(event.Key())
		}

		switch eventKey {
		case 'c', rune(tcell.KeyEnter):
			go cs.Connect(selectedConnection)

		case 'e':
			connectionPages.SwitchToPage("ConnectionForm")
			connectionForm.GetFormItemByLabel("Name").(*tview.InputField).SetText(selectedConnection.Name)
			connectionForm.GetFormItemByLabel("URL").(*tview.InputField).SetText(selectedConnection.URL)
			connectionForm.StatusText.SetText("")
			connectionForm.SetAction("edit")
			return nil

		case 'd':
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

		case 'n':
			connectionForm.SetAction("create")
			connectionForm.GetFormItemByLabel("Name").(*tview.InputField).SetText("")
			connectionForm.GetFormItemByLabel("URL").(*tview.InputField).SetText("")
			connectionForm.StatusText.SetText("")
			connectionPages.SwitchToPage("ConnectionForm")

		case '?':
			HelpModal := NewHelpModal()
			HelpModal.SetDoneFunc(func(_ int, buttonLabel string) {
				MainPages.RemovePage("Help")
				HelpModal = nil
			})
			MainPages.AddPage("Help", HelpModal, true, true)

		case 'q':
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
		cs.StatusText.SetText("Connecting...").SetTextColor(tcell.ColorGreen)
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
