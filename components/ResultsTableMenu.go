package components

import (
	"fmt"

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

	for i, item := range menuItems {
		separator := " | "
		if i == len(menuItems)-1 {
			separator = ""
		}

		text := fmt.Sprintf("%s [%d] %s", item, i+1, separator)
		textview := tview.NewTextView().SetText(text)

		if i == 0 {
			textview.SetTextColor(tview.Styles.PrimaryTextColor)
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
			menu.GetItem(i).(*tview.TextView).SetTextColor(tview.Styles.PrimaryTextColor)
		}

		menu.GetItem(option - 1).(*tview.TextView).SetTextColor(tview.Styles.SecondaryTextColor)
	}
}

func (menu *ResultsTableMenu) SetBlur() {
	menu.SetBorderColor(tview.Styles.InverseTextColor)

	for _, item := range menu.MenuItems {
		item.SetTextColor(tview.Styles.InverseTextColor)
	}
}

func (menu *ResultsTableMenu) SetFocus() {
	menu.SetBorderColor(tview.Styles.PrimaryTextColor)

	for i, item := range menu.MenuItems {
		if i+1 == menu.GetSelectedOption() {
			item.SetTextColor(tview.Styles.SecondaryTextColor)
		} else {
			item.SetTextColor(tview.Styles.PrimaryTextColor)
		}
	}
}
