package pages

import (
	"fmt"
	"lazysql/drivers"
	"lazysql/utils"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var Connections = tview.NewFlex().SetDirection(tview.FlexRow)
var ConnectionPages = tview.NewPages()
var ConnectionStatus = tview.NewTextView().SetChangedFunc(func() { App.Draw() })
var ConnectionsTable = tview.NewTable().SetSelectable(true, false)

func init() {

	connectionsList := renderConnectionList()

	ConnectionPages.AddPage("ConnectionList", connectionsList, true, true)

	connectionsBox := tview.NewFlex()
	connectionsBox.AddItem(ConnectionPages, 0, 1, true)

	connectionsBoxWrapper := tview.NewFlex()
	connectionsBoxWrapper.AddItem(connectionsBox, 0, 1, true).SetBorder(true).SetTitle("Connections").SetBorderColor(tcell.ColorKhaki)

	wrapperBox := tview.NewFlex().AddItem(nil, 0, 1, false)
	wrapperBox.AddItem(connectionsBoxWrapper, 0, 2, true)
	wrapperBox.AddItem(nil, 0, 1, false)

	Connections.AddItem(nil, 0, 1, false)
	Connections.AddItem(wrapperBox, 0, 1, true)
	Connections.AddItem(nil, 0, 1, false)
}

func renderConnectionList() *tview.Flex {
	ConnectionsTable.SetFocusFunc(func() {
		databases, _ := utils.LoadConnections()
		RefreshDatabaseList(databases)
	})

	databases, _ := utils.LoadConnections()

	RefreshDatabaseList(databases)

	buttonsWrapper := tview.NewFlex().SetDirection(tview.FlexColumn)
	buttonsWrapper.AddItem(tview.NewButton("[black]N[white]ew"), 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)
	buttonsWrapper.AddItem(tview.NewButton("[black]C[white]onnect"), 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)
	buttonsWrapper.AddItem(tview.NewButton("[black]E[white]dit"), 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)
	buttonsWrapper.AddItem(tview.NewButton("[black]D[white]elete"), 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)
	buttonsWrapper.AddItem(tview.NewButton("[black]Q[white]uit"), 0, 1, false)

	connectionsListWrapper := tview.NewFlex().SetDirection(tview.FlexRow)
	connectionsListWrapper.AddItem(ConnectionsTable, 0, 1, true)
	connectionsListWrapper.AddItem(ConnectionStatus, 3, 0, false)
	connectionsListWrapper.AddItem(buttonsWrapper, 1, 0, false)
	connectionsListWrapper.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		databases, _ := utils.LoadConnections()
		row, _ := ConnectionsTable.GetSelection()
		selectedConnection := databases[row]
		connectionUrl := fmt.Sprintf("%s://%s:%s@%s:%s", selectedConnection.Provider, selectedConnection.User, selectedConnection.Password, selectedConnection.Host, selectedConnection.Port)

		// N Key
		if event.Rune() == 110 {
			AddConnectionForm.GetFormItemByLabel("URL").(*tview.InputField).SetText("")
			ConnectionPages.SwitchToPage("NewConnection")
			// C Key
		} else if event.Rune() == 99 {
			go connect(connectionUrl)
			// E Key
		} else if event.Rune() == 101 {
			ConnectionPages.SwitchToPage("NewConnection")
			AddConnectionForm.GetFormItemByLabel("URL").(*tview.InputField).SetText(connectionUrl)

			AddConnectionFormWrapper.SetInputCapture(EditConnectionInputHandler(databases, row))

			// D Key
		} else if event.Rune() == 100 {
			newDatabases := append(databases[:row], databases[row+1:]...)

			err := utils.SaveConnectionConfig(newDatabases)
			if err != nil {
				ConnectionStatus.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
				return event
			}

			if err != nil {
				ConnectionStatus.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
				return event
			}

			RefreshDatabaseList(newDatabases)

			// Q Key
		} else if event.Rune() == 113 {
			if connectionsListWrapper.HasFocus() {
				App.Stop()
			}
		}

		return event
	})

	return connectionsListWrapper

}

func RefreshDatabaseList(databases []utils.Connection) {
	ConnectionsTable.Clear()

	for i, database := range databases {
		ConnectionsTable.SetCell(i, 0, tview.NewTableCell(database.Name).SetExpansion(1))
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

	ConnectionStatus.SetText("Connecting...").SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorKhaki).Background(tcell.ColorBlack))

	drivers.Database.SetConnectionString(connectionUrl)
	err := drivers.Database.TestConnection()

	if err != nil {
		ConnectionStatus.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
	} else {
		AllPages.SwitchToPage("home")
		App.Draw()
	}

}
