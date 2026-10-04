package components

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

type ColumnVisibilityModal struct {
	*tview.Flex
	panel     *tview.Flex
	list      *tview.List
	search    *tview.InputField
	status    *tview.TextView
	tableName string
	columns   []string
	hidden    map[string]bool
	filtered  []string
	focusable []tview.Primitive
	onApply   func([]string) error
}

func NewColumnVisibilityModal(columns, hidden []string, onApply func([]string) error, onCancel func()) *ColumnVisibilityModal {
	picker := &ColumnVisibilityModal{
		Flex:    tview.NewFlex(),
		panel:   tview.NewFlex().SetDirection(tview.FlexRow),
		list:    tview.NewList().ShowSecondaryText(false),
		search:  tview.NewInputField().SetLabel(" / Find: "),
		status:  tview.NewTextView().SetTextAlign(tview.AlignCenter),
		columns: slices.Clone(columns),
		hidden:  make(map[string]bool),
		onApply: onApply,
	}
	for _, name := range hidden {
		picker.hidden[name] = true
	}
	if len(columns) > 0 && picker.visibleCount() == 0 {
		delete(picker.hidden, columns[0])
	}

	picker.list.SetHighlightFullLine(true)
	picker.list.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(app.Styles.ContrastSecondaryTextColor))
	picker.search.SetFieldBackgroundColor(app.Styles.ContrastBackgroundColor)
	picker.search.SetFieldTextColor(app.Styles.PrimaryTextColor)
	picker.search.SetChangedFunc(func(_ string) { picker.fillList() })
	picker.search.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			app.App.SetFocus(picker.list)
			return nil
		case tcell.KeyDown, tcell.KeyCtrlN:
			picker.moveSelection(1)
			return nil
		case tcell.KeyUp, tcell.KeyCtrlP:
			picker.moveSelection(-1)
			return nil
		}
		return event
	})
	picker.list.SetSelectedFunc(func(index int, _, _ string, _ rune) { picker.toggle(index) })
	picker.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			picker.apply()
			return nil
		case tcell.KeyCtrlN:
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case tcell.KeyCtrlP:
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case tcell.KeyRune:
			switch event.Rune() {
			case ' ':
				picker.toggle(picker.list.GetCurrentItem())
				return nil
			case '/':
				app.App.SetFocus(picker.search)
				return nil
			case 'a', 'A':
				picker.showAll()
				return nil
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			}
		}
		return event
	})

	all := tview.NewButton("All (A)").SetSelectedFunc(picker.showAll)
	apply := tview.NewButton("Apply ↵").SetSelectedFunc(picker.apply)
	cancel := tview.NewButton("Cancel Esc").SetSelectedFunc(onCancel)
	buttons := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(all, 9, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(apply, 9, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(cancel, 12, 0, false).
		AddItem(nil, 0, 1, false)
	picker.panel.Box = tview.NewBox()
	picker.panel.SetBorder(true).SetBorderColor(app.Styles.PrimaryTextColor).SetBorderPadding(0, 0, 1, 1)
	picker.panel.
		AddItem(picker.search, 1, 0, false).
		AddItem(picker.list, 0, 1, true).
		AddItem(picker.status, 1, 0, false).
		AddItem(buttons, 1, 0, false)
	picker.AddItem(picker.panel, 0, 1, true)
	picker.focusable = []tview.Primitive{picker.list, picker.search, all, apply, cancel}
	picker.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			onCancel()
			return nil
		case tcell.KeyTab, tcell.KeyBacktab:
			index := slices.Index(picker.focusable, app.App.GetFocus())
			if event.Key() == tcell.KeyTab {
				index++
			} else {
				index += len(picker.focusable) - 1
			}
			app.App.SetFocus(picker.focusable[index%len(picker.focusable)])
			return nil
		}
		return event
	})
	picker.fillList()
	return picker
}

func (picker *ColumnVisibilityModal) Draw(screen tcell.Screen) {
	x, y, width, height := picker.GetRect()
	panelWidth := max(1, min(64, width-2))
	panelHeight := max(1, min(len(picker.columns)+5, 22, height-2))
	picker.panel.SetRect(x+(width-panelWidth)/2, y+(height-panelHeight)/2, panelWidth, panelHeight)
	picker.panel.Draw(screen)
}

func (picker *ColumnVisibilityModal) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		x, y := event.Position()
		if !picker.InRect(x, y) {
			return false, nil
		}
		_, capture := picker.Flex.MouseHandler()(action, event, setFocus)
		return true, capture
	}
}

func (picker *ColumnVisibilityModal) fillList() {
	current := picker.list.GetCurrentItem()
	picker.list.Clear()
	picker.filtered = nil
	query := strings.ToLower(strings.TrimSpace(picker.search.GetText()))
	for _, name := range picker.columns {
		if !strings.Contains(strings.ToLower(name), query) {
			continue
		}
		picker.filtered = append(picker.filtered, name)
		picker.list.AddItem(picker.label(name), "", 0, nil)
	}
	if len(picker.filtered) == 0 {
		picker.list.AddItem("No matching columns", "", 0, nil)
	} else {
		picker.list.SetCurrentItem(min(current, len(picker.filtered)-1))
	}
	picker.updateStatus()
}

func (picker *ColumnVisibilityModal) label(name string) string {
	check := "✓"
	if picker.hidden[name] {
		check = " "
	}
	return tview.Escape(fmt.Sprintf(" [%s]  %s", check, name))
}

func (picker *ColumnVisibilityModal) visibleCount() int {
	count := 0
	for _, name := range picker.columns {
		if !picker.hidden[name] {
			count++
		}
	}
	return count
}

func (picker *ColumnVisibilityModal) updateStatus() {
	title := fmt.Sprintf(" Columns · %d/%d visible", picker.visibleCount(), len(picker.columns))
	if picker.tableName != "" {
		title += " · " + picker.tableName
	}
	picker.panel.SetTitle(tview.Escape(title + " "))
	picker.status.SetTextColor(app.Styles.TertiaryTextColor).SetText("Space toggle · / find")
}

func (picker *ColumnVisibilityModal) toggle(index int) {
	if index < 0 || index >= len(picker.filtered) {
		return
	}
	name := picker.filtered[index]
	if !picker.hidden[name] && picker.visibleCount() <= 1 {
		picker.status.SetTextColor(app.Styles.ErrorColor).SetText("Keep at least one column visible")
		return
	}
	if picker.hidden[name] {
		delete(picker.hidden, name)
	} else {
		picker.hidden[name] = true
	}
	picker.list.SetItemText(index, picker.label(name), "")
	picker.updateStatus()
}

func (picker *ColumnVisibilityModal) showAll() {
	clear(picker.hidden)
	picker.fillList()
}

func (picker *ColumnVisibilityModal) moveSelection(direction int) {
	if len(picker.filtered) > 0 {
		picker.list.SetCurrentItem(max(0, min(picker.list.GetCurrentItem()+direction, len(picker.filtered)-1)))
	}
}

func (picker *ColumnVisibilityModal) apply() {
	hidden := make([]string, 0, len(picker.hidden))
	for _, name := range picker.columns {
		if picker.hidden[name] {
			hidden = append(hidden, name)
		}
	}
	if err := picker.onApply(hidden); err != nil {
		picker.status.SetTextColor(app.Styles.ErrorColor).SetText("Could not save: " + err.Error())
	}
}
