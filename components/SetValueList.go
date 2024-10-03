package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/rivo/tview"
)

type SetValueList struct {
	*tview.List
}

type value struct {
	value string
	key   rune
}

var VALUES = []value{
	{value: "NULL", key: 'n'},
	{value: "EMPTY", key: 'e'},
	{value: "DEFAULT", key: 'd'},
}

func NewSetValueList() *SetValueList {
	list := tview.NewList()
	list.SetBorder(true)

	for _, value := range VALUES {
		list.AddItem(value.value, "", value.key, nil)
	}

	return &SetValueList{List: list}
}

func (list *SetValueList) OnFinish(callback func()) {
	list.SetDoneFunc(func() {
		list.Hide()
		callback()
	})

	list.SetSelectedFunc(func(int, string, string, rune) {
		list.Hide()
		callback()
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := app.Keymaps.Group(app.TableGroup).Resolve(event)

		if command == commands.SetValue {
			list.Hide()
			callback()
			return nil
		}

		return event
	})
}

func (list *SetValueList) Show(x, y, width int) {
	list.SetRect(x, y, width, len(VALUES)*2+1)
	MainPages.AddPage("setValue", list, false, true)
	App.SetFocus(list)
	App.ForceDraw()
}

func (list *SetValueList) Hide() {
	MainPages.RemovePage("setValue")
	App.SetFocus(list)
	App.ForceDraw()
}
