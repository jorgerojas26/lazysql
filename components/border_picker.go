package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

// BorderPicker previews normal and focused border colors on the live screen.
type BorderPicker struct {
	*tview.Flex
	list       *tview.List
	input      *tview.InputField
	status     *tview.TextView
	original   app.BorderConfig
	working    app.BorderConfig
	previous   tview.Primitive
	focusColor bool
}

var borderPalette = []struct{ name, value string }{
	{"Slate", "#6B7280"}, {"Sky", "#38BDF8"},
	{"White", "#FFFFFF"}, {"Black", "#000000"},
	{"Red", "#EF4444"}, {"Orange", "#F97316"},
	{"Yellow", "#EAB308"}, {"Green", "#22C55E"},
	{"Cyan", "#06B6D4"}, {"Blue", "#3B82F6"},
	{"Purple", "#A855F7"}, {"Pink", "#EC4899"},
}

func NewBorderPicker(onClose func()) *BorderPicker {
	p := &BorderPicker{
		Flex:     tview.NewFlex().SetDirection(tview.FlexRow),
		list:     tview.NewList().ShowSecondaryText(false),
		input:    tview.NewInputField().SetLabel(" Color name or #RRGGBB: "),
		status:   tview.NewTextView().SetTextAlign(tview.AlignCenter),
		original: app.App.BorderConfig(),
		previous: app.App.GetFocus(),
	}
	p.working = p.original
	p.list.SetBorder(true).SetTitle(" Preset colors ")
	p.list.SetHighlightFullLine(true)
	p.list.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(app.Styles.ContrastSecondaryTextColor))
	for _, color := range borderPalette {
		p.list.AddItem(color.name+"  "+color.value, "", 0, nil)
	}
	p.list.SetChangedFunc(func(index int, _, _ string, _ rune) {
		if index >= 0 && index < len(borderPalette) {
			p.setSelected(borderPalette[index].value)
		}
	})
	p.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			p.focusColor = !p.focusColor
			p.showStatus("")
			return nil
		case tcell.KeyCtrlR:
			p.reset()
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case 'j':
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			case 'k':
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			case 'q':
				return tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
			case 'h':
				app.App.SetFocus(p.input)
				return nil
			case 'r':
				p.reset()
				return nil
			}
		}
		return event
	})
	p.list.SetSelectedFunc(func(_ int, _, _ string, _ rune) { p.save(onClose) })
	p.list.SetDoneFunc(func() {
		_ = app.App.PreviewBorders(p.original)
		onClose()
		if p.previous != nil {
			app.App.SetFocus(p.previous)
		}
	})
	p.input.SetFieldBackgroundColor(app.Styles.ContrastBackgroundColor)
	p.input.SetFieldTextColor(app.Styles.PrimaryTextColor)
	p.input.SetChangedFunc(func(value string) {
		if value == "" {
			return
		}
		if !validBorderPickerColor(value) {
			p.showStatus("Enter a color name or #RRGGBB")
			return
		}
		p.setSelected(value)
	})
	p.input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			if validBorderPickerColor(p.input.GetText()) {
				p.setSelected(p.input.GetText())
				p.save(onClose)
			} else {
				p.showStatus("Enter a valid color before saving")
			}
		case tcell.KeyEscape:
			app.App.SetFocus(p.list)
		}
	})
	p.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape && app.App.GetFocus() != p.input {
			_ = app.App.PreviewBorders(p.original)
			onClose()
			if p.previous != nil {
				app.App.SetFocus(p.previous)
			}
			return nil
		}
		return event
	})
	content := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.list, 0, 1, true).
		AddItem(p.input, 1, 0, false).
		AddItem(p.status, 3, 0, false)
	content.SetBorder(true).SetTitle(" Border colors ")
	p.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().AddItem(nil, 0, 1, false).AddItem(content, 54, 0, true).AddItem(nil, 0, 1, false), 21, 0, true).
		AddItem(nil, 0, 1, false)
	p.showStatus("")
	return p
}

func (p *BorderPicker) setSelected(value string) {
	if p.focusColor {
		p.working.FocusColor = value
	} else {
		p.working.NormalColor = value
	}
	if err := app.App.PreviewBorders(p.working); err != nil {
		p.showStatus(err.Error())
		return
	}
	p.showStatus("")
}

func (p *BorderPicker) reset() {
	p.working.NormalColor, p.working.FocusColor = "", ""
	_ = app.App.PreviewBorders(p.working)
	p.showStatus("Theme colors restored in preview; Enter saves")
}

func (p *BorderPicker) save(onClose func()) {
	if err := app.App.SaveBorderColors(p.working.NormalColor, p.working.FocusColor); err != nil {
		p.showStatus(fmt.Sprintf("Could not save border colors: %s", err))
		return
	}
	onClose()
	if p.previous != nil {
		app.App.SetFocus(p.previous)
	}
}

func (p *BorderPicker) showStatus(message string) {
	target := "normal"
	if p.focusColor {
		target = "focused"
	}
	normal := p.working.NormalColor
	if normal == "" {
		normal = "theme"
	}
	focus := p.working.FocusColor
	if focus == "" {
		focus = "theme"
	}
	p.status.SetText(fmt.Sprintf(" Editing %s | normal: %s | focused: %s\n Tab: target  h: hex  r/Ctrl+R: reset  Enter: save  Esc: cancel\n %s", target, normal, focus, message))
}

func validBorderPickerColor(value string) bool {
	name := strings.ToLower(strings.TrimSpace(value))
	return name == "default" || (name != "" && tcell.GetColor(name) != tcell.ColorDefault)
}
