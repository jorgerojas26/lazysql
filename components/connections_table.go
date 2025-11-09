package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/models"
)

type ConnectionsTable struct {
	*tview.Table
	Wrapper       *tview.Flex
	errorTextView *tview.TextView
	error         string
	connections   []models.Connection
}

var connectionsTable *ConnectionsTable

func NewConnectionsTable() *ConnectionsTable {
	wrapper := tview.NewFlex()

	errorTextView := tview.NewTextView()
	errorTextView.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed))

	table := &ConnectionsTable{
		Table:         tview.NewTable().SetSelectable(true, false),
		Wrapper:       wrapper,
		errorTextView: errorTextView,
	}

	table.SetOffset(5, 0)
	table.SetSelectedStyle(tcell.StyleDefault.Foreground(app.Styles.SecondaryTextColor).Background(tview.Styles.PrimitiveBackgroundColor))

	wrapper.AddItem(table, 0, 1, true)
	table.SetConnections(app.App.Connections())

	connectionsTable = table

	return connectionsTable
}

func (ct *ConnectionsTable) AddConnection(connection models.Connection) {
	rowCount := ct.GetRowCount()
	displayName := connection.Name

	cell := tview.NewTableCell(displayName)

	if connection.ReadOnly {
		displayName = "[READ] " + connection.Name
		cell.SetText(displayName)
		cell.SetTextColor(tcell.ColorYellow)
	}

	ct.SetCell(rowCount, 0, cell)
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

func (ct *ConnectionsTable) SetError(err error) {
	ct.error = err.Error()
	ct.errorTextView.SetText(ct.error)
}
