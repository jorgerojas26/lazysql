package components

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
)

type aiPickerOption struct {
	Key    string
	Label  string
	Detail string
	Group  string
	Search string
}

type aiPicker struct {
	*tview.Flex
	session    *aiSession
	searchable bool
	frame      *tview.Flex
	search     *tview.InputField
	table      *tview.Table
	status     *tview.TextView
	options    []aiPickerOption
	rows       map[int]aiPickerOption
	current    string
	kind       string
	selected   func(string)
	retry      func()
}

func newAIPicker(session *aiSession, kind, title string, selected func(string)) *aiPicker {
	picker := &aiPicker{Flex: tview.NewFlex(), session: session, kind: kind, selected: selected, searchable: kind != "thinking"}
	picker.search = tview.NewInputField().SetLabel("Search: ").SetPlaceholder("Type to filter…")
	picker.search.SetFieldBackgroundColor(app.Styles.ContrastBackgroundColor).SetFieldTextColor(app.Styles.PrimaryTextColor)
	picker.table = tview.NewTable().SetSelectable(true, false)
	picker.table.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(app.Styles.ContrastSecondaryTextColor))
	picker.status = tview.NewTextView().SetWrap(false).SetTextColor(app.Styles.SecondaryTextColor)
	footer := tview.NewTextView().SetWrap(false).SetText("↑↓ / Ctrl+N/P · Enter Select · Esc Back").SetTextColor(app.Styles.SecondaryTextColor)
	picker.frame = tview.NewFlex().SetDirection(tview.FlexRow)
	picker.frame.Box = tview.NewBox().SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	if picker.searchable {
		picker.frame.AddItem(picker.search, 1, 0, true)
	}
	picker.frame.AddItem(picker.status, 1, 0, false).AddItem(picker.table, 0, 1, !picker.searchable).AddItem(footer, 1, 0, false)
	picker.frame.SetBorder(true).SetTitle(" "+title+" ").SetBorderPadding(1, 0, 1, 1).SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	picker.AddItem(picker.frame, 0, 1, true)
	picker.search.SetChangedFunc(func(string) { picker.fill() })
	submit := func() {
		row, _ := picker.table.GetSelection()
		if option, ok := picker.rows[row]; ok {
			picker.selected(option.Key)
		}
	}
	picker.table.SetSelectedFunc(func(int, int) { submit() })
	picker.search.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			submit()
		}
	})
	picker.search.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyDown:
			picker.table.InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), func(tview.Primitive) {})
			return nil
		case tcell.KeyUp:
			picker.table.InputHandler()(tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone), func(tview.Primitive) {})
			return nil
		}
		return event
	})
	picker.frame.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if app.Keymaps.Resolve(event) == commands.GenerateSQL {
			session.closePicker()
			session.hide()
			return nil
		}
		switch event.Key() {
		case tcell.KeyCtrlO:
			session.openModelPicker()
			return nil
		case tcell.KeyCtrlT:
			session.openThinkingPicker()
			return nil
		case tcell.KeyCtrlN, tcell.KeyCtrlP:
			key := tcell.KeyDown
			if event.Key() == tcell.KeyCtrlP {
				key = tcell.KeyUp
			}
			picker.table.InputHandler()(tcell.NewEventKey(key, 0, tcell.ModNone), func(tview.Primitive) {})
			return nil
		case tcell.KeyEscape:
			session.closePicker()
			return nil
		case tcell.KeyCtrlR:
			if picker.retry != nil {
				picker.retry()
			}
			return nil
		case tcell.KeyTab, tcell.KeyBacktab:
			if !picker.searchable {
				return nil
			}
			if picker.search.HasFocus() {
				session.focus(picker.table)
			} else {
				session.focus(picker.search)
			}
			return nil
		}
		return event
	})
	return picker
}

func (picker *aiPicker) setOptions(options []aiPickerOption, current, status string) {
	picker.options, picker.current = options, current
	picker.status.SetText(status)
	picker.fill()
}

func (picker *aiPicker) fill() {
	highlighted := picker.current
	if row, _ := picker.table.GetSelection(); picker.rows != nil {
		if option, ok := picker.rows[row]; ok {
			highlighted = option.Key
		}
	}
	picker.table.Clear()
	picker.rows = map[int]aiPickerOption{}
	query := strings.ToLower(strings.TrimSpace(picker.search.GetText()))
	group, selected, first := "", -1, -1
	for _, option := range picker.options {
		if !strings.Contains(strings.ToLower(option.Label+" "+option.Search+" "+option.Group), query) {
			continue
		}
		row := picker.table.GetRowCount()
		if option.Group != "" && option.Group != group {
			group = option.Group
			picker.table.SetCell(row, 0, tview.NewTableCell(tview.Escape(group)).SetSelectable(false).SetTextColor(app.Styles.SecondaryTextColor))
			row++
		}
		prefix := "  "
		if option.Key == picker.current {
			prefix = "✓ "
		}
		picker.table.SetCell(row, 0, tview.NewTableCell(tview.Escape(prefix+option.Label)).SetExpansion(1))
		picker.table.SetCell(row, 1, tview.NewTableCell(tview.Escape(option.Detail)).SetTextColor(app.Styles.SecondaryTextColor))
		picker.rows[row] = option
		if first < 0 {
			first = row
		}
		if option.Key == highlighted {
			selected = row
		}
	}
	if first < 0 {
		picker.table.SetCell(0, 0, tview.NewTableCell("No matches").SetSelectable(false))
		return
	}
	if selected < 0 {
		selected = first
	}
	picker.table.Select(selected, 0)
}

func (picker *aiPicker) Draw(screen tcell.Screen) {
	x, y, width, height := picker.GetRect()
	rows := picker.table.GetRowCount() + 4
	statusHeight := 0
	if picker.status.GetText(false) != "" {
		statusHeight = 1
	}
	picker.frame.ResizeItem(picker.status, statusHeight, 0)
	rows += statusHeight
	if picker.searchable {
		rows++
	}
	w, h := min(64, width), min(16, rows, height)
	_, inputY, _, inputHeight := picker.session.input.GetRect()
	py := inputY + inputHeight
	if py+h > y+height {
		py = inputY - h
	}
	py = max(y, min(y+height-h, py))
	picker.frame.SetRect(x+(width-w)/2, py, w, h)
	for row := 0; row < picker.table.GetRowCount(); row++ {
		picker.table.GetCell(row, 0).SetMaxWidth(max(8, w-26))
		picker.table.GetCell(row, 1).SetMaxWidth(min(22, max(0, w/3)))
	}
	picker.frame.Draw(screen)
}

func (picker *aiPicker) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return picker.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, focus func(tview.Primitive)) (bool, tview.Primitive) {
		if !picker.frame.InRect(event.Position()) {
			if action == tview.MouseLeftClick {
				picker.session.closePicker()
			}
			return true, nil
		}
		_, capture := picker.frame.MouseHandler()(action, event, focus)
		return true, capture
	})
}
