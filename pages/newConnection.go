package pages

import (
	"lazysql/drivers"
	"lazysql/utils"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var AddConnectionForm *tview.Form
var AddConnectionFormWrapper *tview.Flex

func init() {
	AddConnectionFormWrapper, AddConnectionForm = renderConnectionForm()
	ConnectionPages.AddPage("NewConnection", AddConnectionFormWrapper, true, false)
}

func renderConnectionForm() (wrapper *tview.Flex, addForm *tview.Form) {

	wrapper = tview.NewFlex().SetDirection(tview.FlexRow)

	addForm = tview.NewForm().SetFieldBackgroundColor(tcell.Color100).SetButtonBackgroundColor(tcell.Color101).SetLabelColor(tcell.ColorAntiqueWhite)
	addForm.AddInputField("URL", "", 0, nil, nil)

	wrapper.AddItem(addForm, 0, 1, true)

	ConnectionStatus.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorKhaki).Background(tcell.ColorBlack))
	wrapper.AddItem(ConnectionStatus, 1, 0, false)

	buttonsWrapper := tview.NewFlex().SetDirection(tview.FlexColumn)
	buttonsWrapper.AddItem(tview.NewButton("[black]F1 [white]Save"), 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)
	buttonsWrapper.AddItem(tview.NewButton("[black]F2 [white]Test"), 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)
	buttonsWrapper.AddItem(tview.NewButton("[black]F3 [white]Connect"), 0, 1, false)
	buttonsWrapper.AddItem(nil, 1, 0, false)
	buttonsWrapper.AddItem(tview.NewButton("[black]Esc [white]Cancel"), 0, 1, false)

	wrapper.SetInputCapture(SaveConnectionInputHandler())
	wrapper.AddItem(buttonsWrapper, 1, 0, false)

	return wrapper, addForm
}

func TestConnection(connectionString string) {
	ConnectionStatus.SetText("Connecting...").SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorKhaki).Background(tcell.ColorBlack))

	db := drivers.MySql{}
	db.SetConnectionString(connectionString)

	err := db.TestConnection()

	if err != nil {
		ConnectionStatus.SetText(err.Error()).SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorBlack))
	} else {
		ConnectionStatus.SetText("Connection success").SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorKhaki).Background(tcell.ColorBlack))
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
