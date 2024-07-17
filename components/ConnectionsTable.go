package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/models"
)

type ConnectionsTable struct {
	*tview.Table
	Wrapper       *tview.Flex
	errorTextView *tview.TextView
	error         string
	connections   []models.Connection
}

func NewConnectionsTable() *ConnectionsTable {
	wrapper := tview.NewFlex()

	errorTextView := tview.NewTextView()
	errorTextView.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))

	table := &ConnectionsTable{
		Table:         tview.NewTable().SetSelectable(true, false),
		Wrapper:       wrapper,
		errorTextView: errorTextView,
	}

	table.SetSelectedStyle(tcell.StyleDefault.Foreground(tview.Styles.PrimaryTextColor).Background(tview.Styles.SecondaryTextColor))

	wrapper.AddItem(table, 0, 1, true)

	connections, err := helpers.LoadConnections()

	if err != nil {
		table.SetError(err.Error())
	} else {
		table.SetConnections(connections)
	}

	return table
}

func (ct *ConnectionsTable) AddConnection(connection models.Connection) {
	rowCount := ct.GetRowCount()

	ct.SetCellSimple(rowCount, 0, connection.Name)

	ct.connections = append(ct.connections, connection)
}

func (ct *ConnectionsTable) GetConnections() []models.Connection {
	return ct.connections
}

func (ct *ConnectionsTable) GetError() string {
	return ct.error
}

func (ct *ConnectionsTable) SetConnections(connections []models.Connection) {
	ct.connections = make([]models.Connection, 0)

	ct.Clear()

	for _, connection := range connections {
		ct.AddConnection(connection)
	}

	ct.Select(0, 0)
	App.ForceDraw()
}

func (ct *ConnectionsTable) SetError(error string) {
	ct.error = error

	ct.errorTextView.SetText(error)
}
