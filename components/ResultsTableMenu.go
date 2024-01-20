package components

import (
	"fmt"

	"github.com/jorgerojas26/lazysql/app"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type ResultsTableMenuState struct {
	SelectedOption int
}

type ResultsTableMenu struct {
	*tview.Flex
	state     *ResultsTableMenuState
	MenuItems []*tview.TextView
}

var menuItems = []string{
	"Records",
	"Columns",
	"Constraints",
	"Foreign Keys",
	"Indexes",
}

func NewResultsTableMenu() *ResultsTableMenu {
	state := &ResultsTableMenuState{
		SelectedOption: 1,
	}

	menu := &ResultsTableMenu{
		Flex:  tview.NewFlex(),
		state: state,
	}

	menu.SetBorder(true)
	menu.SetBackgroundColor(tcell.ColorDefault)

	for i, item := range menuItems {
		separator := " | "
		if i == len(menuItems)-1 {
			separator = ""
		}

		text := fmt.Sprintf("%s [%d] %s", item, i+1, separator)
		textview := tview.NewTextView().SetText(text)
		textview.SetBackgroundColor(tcell.ColorDefault)

		if i == 0 {
			textview.SetTextColor(app.ActiveTextColor)
		}

		size := 15

		switch item {
		case "Constraints":
			size = 19
		case "Foreign Keys":
			size = 20
		case "Indexes":
			size = 16
		}

		menu.MenuItems = append(menu.MenuItems, textview)
		menu.AddItem(textview, size, 0, false)
	}

	return menu
}

// Getters and Setters
func (menu *ResultsTableMenu) GetSelectedOption() int {
	return menu.state.SelectedOption
}

func (menu *ResultsTableMenu) SetSelectedOption(option int) {
	if menu.state.SelectedOption != option {

		menu.state.SelectedOption = option

		itemCount := menu.GetItemCount()

		for i := 0; i < itemCount; i++ {
			menu.GetItem(i).(*tview.TextView).SetTextColor(app.FocusTextColor)
		}

		menu.GetItem(option - 1).(*tview.TextView).SetTextColor(app.ActiveTextColor)
	}
}

func (menu *ResultsTableMenu) SetBlur() {
	menu.SetBorderColor(tcell.ColorDarkGray)

	for _, item := range menu.MenuItems {
		item.SetTextColor(app.InactiveTextColor)
	}
}

func (menu *ResultsTableMenu) SetFocus() {
	menu.SetBorderColor(tcell.ColorWhite)

	for i, item := range menu.MenuItems {
		if i+1 == menu.GetSelectedOption() {
			item.SetTextColor(app.ActiveTextColor)
		} else {
			item.SetTextColor(tcell.ColorWhite)
		}
	}
}
