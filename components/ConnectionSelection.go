package components

import (
	"fmt"
	"lazysql/app"
	"lazysql/drivers"
	"lazysql/helpers"
	"lazysql/models"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
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
	newButton.SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(newButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	connectButton := tview.NewButton("[darkred]C[black]onnect")
	connectButton.SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(connectButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	editButton := tview.NewButton("[darkred]E[black]dit")
	editButton.SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(editButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	deleteButton := tview.NewButton("[darkred]D[black]elete")
	deleteButton.SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(deleteButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	quitButton := tview.NewButton("[darkred]Q[black]uit")
	quitButton.SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(quitButton, 0, 1, false)

	statusText := tview.NewTextView()
	statusText.SetBorderPadding(0, 2, 0, 0)

	wrapper.AddItem(ConnectionListTable, 0, 1, true)
	wrapper.AddItem(statusText, 1, 0, false)
	wrapper.AddItem(buttonsWrapper, 1, 0, false)

	cs := &ConnectionSelection{
		Flex:       wrapper,
		StatusText: statusText,
	}

	wrapper.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		connections := ConnectionListTable.GetConnections()

		if len(connections) != 0 {
			row, _ := ConnectionListTable.GetSelection()
			selectedConnection := connections[row]
			connectionUrl := fmt.Sprintf("%s://%s:%s@%s:%s", selectedConnection.Provider, selectedConnection.User, selectedConnection.Password, selectedConnection.Host, selectedConnection.Port)

			if event.Rune() == 'c' || event.Key() == tcell.KeyEnter {
				go cs.connect(connectionUrl)
			} else if event.Rune() == 'e' {
				connectionPages.SwitchToPage("ConnectionForm")
				connectionForm.GetFormItemByLabel("Name").(*tview.InputField).SetText(selectedConnection.Name)
				connectionForm.GetFormItemByLabel("URL").(*tview.InputField).SetText(connectionUrl)
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

func (cs *ConnectionSelection) connect(connectionUrl string) {
	if MainPages.HasPage(connectionUrl) {
		MainPages.SwitchToPage(connectionUrl)
		App.Draw()
	} else {
		newHome := NewHomePage(connectionUrl)
		cs.StatusText.SetText("Connecting...").SetTextStyle(tcell.StyleDefault.Foreground(app.ActiveTextColor))
		App.Draw()

		drivers.MySQL.SetConnectionString(connectionUrl)
		err := drivers.MySQL.Connect()

		if err != nil {
			cs.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))
		} else {
			selectedRow, selectedCol := ConnectionListTable.GetSelection()
			cell := ConnectionListTable.GetCell(selectedRow, selectedCol)
			cell.SetText(fmt.Sprintf("[green]* %s", cell.Text))

			ConnectionListTable.SetCell(selectedRow, selectedCol, cell)

			MainPages.SwitchToPage(connectionUrl)
			newHome.Tree.SetCurrentNode(newHome.Tree.GetRoot())
		}

		MainPages.AddAndSwitchToPage(connectionUrl, newHome, true)

		cs.StatusText.SetText("")
		App.Draw()
	}
}
