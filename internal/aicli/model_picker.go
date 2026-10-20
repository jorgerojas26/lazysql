package aicli

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func modelDialog(provider string, models []ai.Model, current string, selected func(string), focus func(tview.Primitive)) (*dialog, *tview.InputField, *tview.List) {
	search := tview.NewInputField().SetLabel(" / Models: ").SetPlaceholder("Search models…")
	search.SetFieldBackgroundColor(app.Styles.ContrastBackgroundColor).SetFieldTextColor(app.Styles.PrimaryTextColor)
	list := tview.NewList().ShowSecondaryText(false).SetHighlightFullLine(true)
	list.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(app.Styles.ContrastSecondaryTextColor))
	list.SetDrawFunc(func(_ tcell.Screen, x, y, width, height int) (int, int, int, int) {
		list.SetCurrentItem(list.GetCurrentItem())
		return x, y, width, height
	})
	container := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(search, 1, 0, true).AddItem(list, 0, 1, false)
	view := newDialog(provider+" models", "", container, 0, min(22, len(models)+7))
	view.footer.SetText("Type to filter  Tab Search/List  ↵ Select")
	var filtered []ai.Model
	fill := func() {
		highlighted := current
		if index := list.GetCurrentItem(); index >= 0 && index < len(filtered) {
			highlighted = filtered[index].ID
		}
		query := strings.ToLower(strings.TrimSpace(search.GetText()))
		filtered = nil
		list.Clear()
		index := 0
		for _, model := range models {
			if !strings.Contains(strings.ToLower(model.Name+" "+model.ID+" "+model.Format), query) {
				continue
			}
			if model.ID == highlighted {
				index = len(filtered)
			}
			filtered = append(filtered, model)
			list.AddItem(tview.Escape(model.Name), "", 0, nil)
		}
		if len(filtered) == 0 {
			list.AddItem("No matching models", "", 0, nil)
		} else {
			list.SetCurrentItem(index)
		}
	}
	submit := func() {
		index := list.GetCurrentItem()
		if index >= 0 && index < len(filtered) {
			selected(filtered[index].ID)
		}
	}
	list.SetSelectedFunc(func(int, string, string, rune) { submit() })
	search.SetChangedFunc(func(string) { fill() })
	search.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			submit()
		}
	})
	search.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyDown, tcell.KeyCtrlN:
			if len(filtered) > 0 {
				list.SetCurrentItem(min(len(filtered)-1, list.GetCurrentItem()+1))
			}
			return nil
		case tcell.KeyUp, tcell.KeyCtrlP:
			if len(filtered) > 0 {
				list.SetCurrentItem(max(0, list.GetCurrentItem()-1))
			}
			return nil
		}
		return event
	})
	view.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab {
			if search.HasFocus() {
				focus(list)
			} else {
				focus(search)
			}
			return nil
		}
		return event
	})
	fill()
	return view, search, list
}

func (ui *terminalUI) Model(provider string, models []ai.Model, current string) (string, error) {
	var result string
	err := ui.run(func(done func(), application *tview.Application) tview.Primitive {
		view, _, _ := modelDialog(provider, models, current, func(id string) { result = id; done() }, func(primitive tview.Primitive) { application.SetFocus(primitive) })
		return view
	})
	return result, err
}
