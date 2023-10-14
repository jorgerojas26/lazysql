package pages

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"lazysql/app"
	"lazysql/drivers"
	"lazysql/utils"
)

var (
	Connections      = tview.NewFlex().SetDirection(tview.FlexRow)
	ConnectionPages  = tview.NewPages()
	ConnectionStatus = tview.NewTextView().SetChangedFunc(func() { App.Draw() })
	ConnectionsTable = tview.NewTable().SetSelectable(true, false)
)

func init() {
	connectionsList := renderConnectionList()

	ConnectionPages.AddPage("ConnectionList", connectionsList, true, true)

	connectionsBox := tview.NewFlex()
	connectionsBox.AddItem(ConnectionPages, 0, 1, true)

	connectionsBoxWrapper := tview.NewFlex()
	connectionsBoxWrapper.AddItem(connectionsBox, 0, 1, true).SetBorder(true).SetTitle("Connections").SetBorderColor(app.FocusTextColor)

	wrapperBox := tview.NewFlex().AddItem(nil, 0, 1, false)
	wrapperBox.AddItem(connectionsBoxWrapper, 0, 2, true)
	wrapperBox.AddItem(nil, 0, 1, false)

	Connections.AddItem(nil, 0, 1, false)
	Connections.AddItem(wrapperBox, 0, 1, true)
	Connections.AddItem(nil, 0, 1, false)
}

func renderConnectionList() *tview.Flex {
	ConnectionsTable.SetFocusFunc(func() {
		connections, _ := utils.LoadConnections()
		refreshConnectionList(connections)
	})

	connections, _ := utils.LoadConnections()

	refreshConnectionList(connections)

	buttonsWrapper := tview.NewFlex().SetDirection(tview.FlexColumn)

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

	connectionsListWrapper := tview.NewFlex().SetDirection(tview.FlexRow)
	connectionsListWrapper.AddItem(ConnectionsTable, 0, 1, true)
	connectionsListWrapper.AddItem(ConnectionStatus, 3, 0, false)
	connectionsListWrapper.AddItem(buttonsWrapper, 1, 0, false)
	connectionsListWrapper.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		connections, _ := utils.LoadConnections()
		if len(connections) != 0 {
			row, _ := ConnectionsTable.GetSelection()
			selectedConnection := connections[row]
			connectionUrl := fmt.Sprintf("%s://%s:%s@%s:%s", selectedConnection.Provider, selectedConnection.User, selectedConnection.Password, selectedConnection.Host, selectedConnection.Port)

			if event.Rune() == 99 || event.Key() == tcell.KeyEnter {
				go connect(connectionUrl)
			} else if event.Rune() == 101 { // E Key
				ConnectionPages.SwitchToPage("NewConnection")
				AddConnectionForm.GetFormItemByLabel("Name").(*tview.InputField).SetText(selectedConnection.Name)
				AddConnectionForm.GetFormItemByLabel("URL").(*tview.InputField).SetText(connectionUrl)

				AddConnectionFormWrapper.SetInputCapture(EditConnectionInputHandler(connections, row))

				// D Key
			} else if event.Rune() == 100 {
				newConnections := append(connections[:row], connections[row+1:]...)

				err := utils.SaveConnectionConfig(newConnections)
				if err != nil {
					ConnectionStatus.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
					return event
				}

				refreshConnectionList(newConnections)

			}
		}

		if event.Rune() == 110 { // N Key
			AddConnectionForm.GetFormItemByLabel("URL").(*tview.InputField).SetText("")
			ConnectionPages.SwitchToPage("NewConnection")
		} else if event.Rune() == 113 { // Q Key
			if connectionsListWrapper.HasFocus() {
				App.Stop()
			}
		}

		return event
	})

	return connectionsListWrapper
}

func refreshConnectionList(connections []utils.Connection) {
	ConnectionsTable.Clear()

	for i, connection := range connections {
		connectionUrl := fmt.Sprintf("%s://%s:%s@%s:%s", connection.Provider, connection.User, connection.Password, connection.Host, connection.Port)
		cell := tview.NewTableCell(connection.Name).SetExpansion(1)
		if AllPages.HasPage(connectionUrl) {
			cell.SetTextColor(tcell.ColorDarkSeaGreen)
		}
		ConnectionsTable.SetCell(i, 0, cell)
	}

	selectedRow, _ := ConnectionsTable.GetSelection()
	rowCount := ConnectionsTable.GetRowCount()

	if selectedRow > rowCount {
		ConnectionsTable.Select(rowCount-1, 0)
	} else {
		ConnectionsTable.Select(selectedRow, 0)
	}
}

func connect(connectionUrl string) {
	if AllPages.HasPage(connectionUrl) {
		AllPages.SwitchToPage(connectionUrl)
		App.Draw()
	} else {
		newHome := NewHomePage(connectionUrl)
		ConnectionStatus.SetText("Connecting...").SetTextStyle(tcell.StyleDefault.Foreground(app.ActiveTextColor).Background(tcell.ColorBlack))

		drivers.MySQL.SetConnectionString(connectionUrl)
		err := drivers.MySQL.Connect()

		if err != nil {
			ConnectionStatus.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
		} else {
			AllPages.SwitchToPage(connectionUrl)
			newHome.Tree.SetCurrentNode(newHome.Tree.GetRoot())
			App.Draw()
		}

		AllPages.AddAndSwitchToPage(connectionUrl, newHome, true)

		ConnectionStatus.SetText("")
	}

}
