package pages

import (
	"lazysql/drivers"
	"lazysql/utils"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	AddConnectionForm        *tview.Form
	AddConnectionFormWrapper *tview.Flex
)

func init() {
	AddConnectionFormWrapper, AddConnectionForm = renderConnectionForm()
	ConnectionPages.AddPage("NewConnection", AddConnectionFormWrapper, true, false)
}

func renderConnectionForm() (wrapper *tview.Flex, addForm *tview.Form) {
	wrapper = tview.NewFlex().SetDirection(tview.FlexRow)

	addForm = tview.NewForm().SetFieldBackgroundColor(tcell.ColorWhite).SetButtonBackgroundColor(tcell.ColorWhite).SetLabelColor(tcell.ColorWhite.TrueColor()).SetFieldTextColor(tcell.ColorBlack)
	addForm.AddInputField("URL", "", 0, nil, nil)

	wrapper.AddItem(addForm, 0, 1, true)

	ConnectionStatus.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorKhaki.TrueColor()).Background(tcell.ColorBlack))
	wrapper.AddItem(ConnectionStatus, 1, 0, false)

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
	wrapper.SetInputCapture(SaveConnectionInputHandler())
	wrapper.AddItem(buttonsWrapper, 1, 0, false)

	return wrapper, addForm
}

func TestConnection(connectionString string) {
	ConnectionStatus.SetText("Connecting...").SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorKhaki.TrueColor()).Background(tcell.ColorBlack))

	db := drivers.MySql{}
	db.SetConnectionString(connectionString)

	err := db.TestConnection()

	if err != nil {
		ConnectionStatus.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
	} else {
		ConnectionStatus.SetText("Connection success").SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorKhaki.TrueColor()).Background(tcell.ColorBlack))
	}
}

func SaveConnectionInputHandler() func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			urlInput := AddConnectionForm.GetFormItemByLabel("URL").(*tview.InputField)
			if AddConnectionForm.HasFocus() {
				if urlInput.GetText() == "" {
					ConnectionStatus.SetText("")
					ConnectionPages.SwitchToPage("ConnectionList")
				} else {
					urlInput.SetText("")
				}
			} else if AddConnectionFormWrapper.HasFocus() {
				ConnectionStatus.SetText("")
				ConnectionPages.SwitchToPage("ConnectionList")
			}
		} else if event.Key() == tcell.KeyF1 || event.Key() == tcell.KeyEnter {
			connectionString := AddConnectionForm.GetFormItem(0).(*tview.InputField).GetText()
			parsed, err := drivers.Database.ParseConnectionString(connectionString)
			if err != nil {
				ConnectionStatus.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
				return event
			} else {
				password, _ := parsed.User.Password()

				database := utils.Connection{
					Name:     parsed.Short(),
					Provider: parsed.Driver,
					User:     parsed.User.Username(),
					Password: password,
					Host:     parsed.Hostname(),
					Port:     parsed.Port(),
				}

				databases, _ := utils.LoadConnections()
				newDatabases := append(databases, database)
				err := utils.SaveConnectionConfig(newDatabases)
				if err != nil {
					ConnectionStatus.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
					return event
				}

				ConnectionPages.SwitchToPage("ConnectionList")
			}
		} else if event.Key() == tcell.KeyF2 {
			connectionString := AddConnectionForm.GetFormItem(0).(*tview.InputField).GetText()
			go TestConnection(connectionString)
		}
		return event
	}
}

func EditConnectionInputHandler(databases []utils.Connection, row int) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			ConnectionStatus.SetText("")
			ConnectionPages.SwitchToPage("ConnectionList")
		} else if event.Key() == tcell.KeyF1 || event.Key() == tcell.KeyEnter {
			connectionString := AddConnectionForm.GetFormItem(0).(*tview.InputField).GetText()
			parsed, err := drivers.Database.ParseConnectionString(connectionString)

			if err != nil {
				ConnectionStatus.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
				return event
			} else {
				newDatabases := make([]utils.Connection, len(databases))
				for i, database := range databases {
					if i == row {
						newDatabases[i].Name = database.Name
						newDatabases[i].Provider = database.Provider
						newDatabases[i].User = parsed.User.Username()
						newDatabases[i].Password, _ = parsed.User.Password()
						newDatabases[i].Host = parsed.Hostname()
						newDatabases[i].Port = parsed.Port()

					} else {
						newDatabases[i] = database
					}
				}

				err := utils.SaveConnectionConfig(newDatabases)
				if err != nil {
					ConnectionStatus.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
					return event
				}

			}

			ConnectionPages.SwitchToPage("ConnectionList")

		} else if event.Key() == tcell.KeyF2 {
			connectionString := AddConnectionForm.GetFormItem(0).(*tview.InputField).GetText()
			go TestConnection(connectionString)
		}
		return event
	}
}
