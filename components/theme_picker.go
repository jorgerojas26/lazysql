package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

// ThemePicker previews built-in themes while preserving the current UI state.
type ThemePicker struct {
	*tview.Flex

	list           *tview.List
	preview        *tview.TextView
	status         *tview.TextView
	originalConfig app.ThemeConfig
	previousFocus  tview.Primitive
	selectedPreset string
}

func NewThemePicker(onClose func()) *ThemePicker {
	picker := &ThemePicker{
		Flex:           tview.NewFlex().SetDirection(tview.FlexRow),
		list:           tview.NewList().ShowSecondaryText(false),
		preview:        tview.NewTextView().SetDynamicColors(true),
		status:         tview.NewTextView(),
		originalConfig: app.App.ThemeConfig(),
		previousFocus:  app.App.GetFocus(),
	}

	picker.list.SetBorder(true).SetTitle(" Themes ")
	picker.list.SetHighlightFullLine(true)
	picker.list.SetSelectedStyle(tcell.StyleDefault.
		Background(app.Styles.SecondaryTextColor).
		Foreground(app.Styles.ContrastSecondaryTextColor))
	picker.list.SetInputCapture(themePickerNavigationEvent)
	picker.preview.SetBorder(true).SetTitle(" Live preview ")
	picker.preview.SetWrap(true)
	picker.status.SetTextAlign(tview.AlignCenter)
	picker.status.SetDynamicColors(true)

	names := app.ThemePresetNames()
	currentPreset := strings.ToLower(picker.originalConfig.Preset)
	if currentPreset == "" {
		currentPreset = app.DefaultThemePreset
	}
	currentIndex := 0
	for index, name := range names {
		picker.list.AddItem(name, "", 0, nil)
		if name == currentPreset {
			currentIndex = index
		}
	}
	picker.list.SetCurrentItem(currentIndex)
	picker.selectedPreset = names[currentIndex]
	picker.updatePreview(app.Styles, picker.selectedPreset)

	picker.list.SetChangedFunc(func(_ int, name, _ string, _ rune) {
		picker.selectedPreset = name
		if err := app.App.PreviewTheme(app.ThemeConfig{Preset: name}); err != nil {
			picker.status.SetText(fmt.Sprintf("[%s]%s[-]", app.Styles.ErrorColor, err))
			return
		}
		picker.status.SetText("j/k or Ctrl+N/P to preview • Enter to save • q/Esc to cancel")
		picker.updatePreview(app.Styles, name)
	})

	closePicker := func(restore bool) {
		if restore {
			if err := app.App.PreviewTheme(picker.originalConfig); err != nil {
				picker.status.SetText(fmt.Sprintf("[%s]%s[-]", app.Styles.ErrorColor, err))
				return
			}
		}
		onClose()
		if picker.previousFocus != nil {
			app.App.SetFocus(picker.previousFocus)
		}
	}

	picker.list.SetSelectedFunc(func(_ int, name, _ string, _ rune) {
		if err := app.App.PreviewTheme(app.ThemeConfig{Preset: name}); err != nil {
			picker.status.SetText(fmt.Sprintf("[%s]%s[-]", app.Styles.ErrorColor, err))
			return
		}
		if err := app.App.SaveThemePreset(name); err != nil {
			picker.status.SetText(fmt.Sprintf("[%s]Could not save theme: %s[-]", app.Styles.ErrorColor, err))
			return
		}
		closePicker(false)
	})
	picker.list.SetDoneFunc(func() {
		closePicker(true)
	})

	content := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(picker.list, 28, 0, true).
		AddItem(picker.preview, 0, 1, false)
	panel := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(content, 0, 1, true).
		AddItem(picker.status, 1, 0, false)
	panel.SetBorder(true).SetTitle(" Theme picker ")

	picker.AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(nil, 0, 1, false).
			AddItem(panel, 86, 0, true).
			AddItem(nil, 0, 1, false), 24, 0, true).
		AddItem(nil, 0, 1, false)

	picker.status.SetText("j/k or Ctrl+N/P to preview • Enter to save • q/Esc to cancel")
	app.App.BeginThemePreview()
	return picker
}

func themePickerNavigationEvent(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlN:
		return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
	case tcell.KeyCtrlP:
		return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
	case tcell.KeyRune:
		switch event.Rune() {
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case 'q':
			return tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
		}
	}
	return event
}

func (picker *ThemePicker) updatePreview(theme *app.Theme, name string) {
	picker.preview.SetText(fmt.Sprintf(
		"\n  [%s::b]%s[-]\n\n"+
			"  [%s]SELECT[-] [%s]users[-] [%s]FROM[-] [%s]'production'[-]\n"+
			"  [%s]42[-]  [%s]-- syntax preview[-]\n\n"+
			"  [%s:%s] selected row [-:-]  [%s:%s] inserted [-:-]\n"+
			"  [%s:%s] changed [-:-]       [%s:%s] deleted [-:-]\n\n"+
			"  [%s]JSON key[-]: [%s]\"value\"[-]  [%s]true[-]  [%s]null[-]\n\n"+
			"  [%s]Primary text[-]  [%s]Secondary[-]  [%s]Success[-]  [%s]Error[-]",
		theme.TitleColor, name,
		theme.SQLKeywordColor, theme.PrimaryTextColor, theme.SQLKeywordColor, theme.SQLStringColor,
		theme.SQLNumberColor, theme.SQLCommentColor,
		theme.ContrastSecondaryTextColor, theme.TableMarkedColor,
		theme.ContrastSecondaryTextColor, theme.TableInsertColor,
		theme.ContrastSecondaryTextColor, theme.TableChangeColor,
		theme.ContrastSecondaryTextColor, theme.TableDeleteColor,
		theme.JSONKeyColor, theme.JSONStringColor, theme.JSONBooleanColor, theme.JSONNullColor,
		theme.PrimaryTextColor, theme.SecondaryTextColor, theme.TertiaryTextColor, theme.ErrorColor,
	))
}
