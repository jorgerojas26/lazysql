package components

import (
	"lazysql/drivers"
	"lazysql/helpers"
	"lazysql/models"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ConnectionForm struct {
	*tview.Flex
	*tview.Form
	StatusText *tview.TextView
	Action     string
}

func NewConnectionForm(connectionPages *models.ConnectionPages) *ConnectionForm {
	wrapper := tview.NewFlex()

	wrapper.SetDirection(tview.FlexColumnCSS)

	addForm := tview.NewForm().SetFieldBackgroundColor(tcell.ColorWhite).SetButtonBackgroundColor(tcell.ColorWhite).SetLabelColor(tcell.ColorWhite.TrueColor()).SetFieldTextColor(tcell.ColorBlack)
	addForm.AddInputField("Name", "", 0, nil, nil)
	addForm.AddInputField("URL", "", 0, nil, nil)

	buttonsWrapper := tview.NewFlex().SetDirection(tview.FlexColumn)

	saveButton := tview.NewButton("[darkred]F1 [black]Save")
	saveButton.SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(saveButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	testButton := tview.NewButton("[darkred]F2 [black]Test")
	testButton.SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(testButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	connectButton := tview.NewButton("[darkred]F3 [black]Connect")
	connectButton.SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(connectButton, 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)

	cancelButton := tview.NewButton("[darkred]Esc [black]Cancel")
	cancelButton.SetStyle(tcell.StyleDefault.Background(tcell.ColorGhostWhite))
	buttonsWrapper.AddItem(cancelButton, 0, 1, false)

	statusText := tview.NewTextView()

	wrapper.AddItem(addForm, 0, 1, true)
	wrapper.AddItem(buttonsWrapper, 1, 0, false)
	wrapper.AddItem(statusText, 1, 0, false)

	form := &ConnectionForm{
		Flex:       wrapper,
		Form:       addForm,
		StatusText: statusText,
	}

	wrapper.SetInputCapture(form.inputCapture(connectionPages))

	return form
}

func (form *ConnectionForm) inputCapture(connectionPages *models.ConnectionPages) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			connectionPages.SwitchToPage("Connections")
		} else if event.Key() == tcell.KeyF1 || event.Key() == tcell.KeyEnter {
			connectionName := form.GetFormItem(0).(*tview.InputField).GetText()

			if connectionName == "" {
				form.StatusText.SetText("Connection name is required").SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
				return event
			}

			connectionString := form.GetFormItem(1).(*tview.InputField).GetText()

			parsed, err := drivers.MySQL.ParseConnectionString(connectionString)

			if err != nil {
				form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
				return event
			} else {

				password, _ := parsed.User.Password()
				databases, _ := helpers.LoadConnections()
				newDatabases := make([]models.Connection, len(databases))

				if form.Action == "create" {
					database := models.Connection{
						Name:     connectionName,
						Provider: parsed.Driver,
						User:     parsed.User.Username(),
						Password: password,
						Host:     parsed.Hostname(),
						Port:     parsed.Port(),
					}

					newDatabases = append(databases, database)
					err := helpers.SaveConnectionConfig(newDatabases)
					if err != nil {
						form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
						return event
					}
				} else if form.Action == "edit" {
					newDatabases = make([]models.Connection, len(databases))
					row, _ := ConnectionListTable.GetSelection()
					for i, database := range databases {
						if i == row {
							newDatabases[i].Name = connectionName
							newDatabases[i].Provider = database.Provider
							newDatabases[i].User = parsed.User.Username()
							newDatabases[i].Password, _ = parsed.User.Password()
							newDatabases[i].Host = parsed.Hostname()
							newDatabases[i].Port = parsed.Port()

						} else {
							newDatabases[i] = database
						}
					}

					err := helpers.SaveConnectionConfig(newDatabases)
					if err != nil {
						form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
						return event
					}

				}
				ConnectionListTable.SetConnections(newDatabases)
				connectionPages.SwitchToPage("Connections")
			}
		} else if event.Key() == tcell.KeyF2 {
			connectionString := form.GetFormItem(1).(*tview.InputField).GetText()
			go form.testConnection(connectionString)
		}
		return event
	}
}

func (form *ConnectionForm) testConnection(connectionString string) {
	form.StatusText.SetText("Connecting...").SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorKhaki.TrueColor()).Background(tcell.ColorBlack))

	db := drivers.MySql{}
	db.SetConnectionString(connectionString)

	err := db.TestConnection()

	if err != nil {
		form.StatusText.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
	} else {
		form.StatusText.SetText("Connection success").SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorKhaki.TrueColor()).Background(tcell.ColorBlack))
	}
	App.ForceDraw()
}

func (form *ConnectionForm) SetAction(action string) {
	form.Action = action
}
