package aicli

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

type terminalUI struct {
	ctx        context.Context
	configFile string
}

type choice struct{ name, value string }

type dialog struct {
	*tview.Flex
	panel       *tview.Flex
	height      int
	footer      *tview.TextView
	description *tview.TextView
	options     *tview.List
}

func (view *dialog) Draw(screen tcell.Screen) {
	view.DrawForSubclass(screen, view)
	x, y, width, height := view.GetRect()
	panelWidth := max(1, min(68, width-2))
	panelHeight := max(1, min(view.height, height-2))
	view.panel.SetRect(x+(width-panelWidth)/2, y+(height-panelHeight)/2, panelWidth, panelHeight)
	view.panel.Draw(screen)
}

func newDialog(title, message string, control tview.Primitive, controlHeight, height int) *dialog {
	description := tview.NewTextView().SetText(message).SetWrap(true)
	description.SetTextColor(app.Styles.PrimaryTextColor)
	panel := tview.NewFlex().SetDirection(tview.FlexRow)
	panel.SetBorder(true).SetTitle(" "+tview.Escape(title)+" ").SetBorderColor(app.Styles.SecondaryTextColor).SetBorderPadding(1, 1, 1, 1)
	footer := tview.NewTextView().SetText("↑/↓ Choose  ↵ Continue  Esc Cancel").SetTextColor(app.Styles.TertiaryTextColor)
	controlWeight := 0
	if controlHeight == 0 {
		controlWeight = 3
	}
	if message != "" {
		panel.AddItem(description, 0, 1, false)
	}
	panel.AddItem(control, controlHeight, controlWeight, true).AddItem(nil, 1, 0, false).AddItem(footer, 1, 0, false)
	view := &dialog{Flex: tview.NewFlex(), panel: panel, height: height, footer: footer, description: description}
	view.Box = tview.NewBox()
	view.AddItem(panel, 0, 1, true)
	view.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyPgDn || event.Key() == tcell.KeyPgUp {
			row, column := description.GetScrollOffset()
			if event.Key() == tcell.KeyPgDn {
				row += 3
			} else {
				row = max(0, row-3)
			}
			description.ScrollTo(row, column)
			return nil
		}
		return event
	})
	return view
}

func choiceDialog(title, message string, choices []choice, selected func(string)) *dialog {
	list := tview.NewList().ShowSecondaryText(false).SetHighlightFullLine(true)
	list.SetSelectedStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(app.Styles.ContrastSecondaryTextColor))
	for _, item := range choices {
		list.AddItem(item.name, "", 0, nil)
	}
	list.SetSelectedFunc(func(index int, _ string, _ string, _ rune) {
		if index >= 0 && index < len(choices) {
			selected(choices[index].value)
		}
	})
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlN:
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case tcell.KeyCtrlP:
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case tcell.KeyRune:
			if event.Rune() == 'j' {
				return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
			}
			if event.Rune() == 'k' {
				return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
			}
		}
		return event
	})
	listHeight := len(choices)
	height := 14
	if message == "" {
		height = listHeight + 6
	}
	view := newDialog(title, message, list, listHeight, height)
	view.options = list
	return view
}

func textDialog(title, message, value string, secret bool, submit func(string) error) (*dialog, *tview.InputField) {
	input := tview.NewInputField().SetLabel(" › ").SetText(value)
	input.SetFieldBackgroundColor(app.Styles.ContrastBackgroundColor).SetFieldTextColor(app.Styles.PrimaryTextColor)
	if secret {
		input.SetMaskCharacter('*')
	}
	status := tview.NewTextView().SetWrap(true).SetTextColor(app.Styles.ErrorColor)
	content := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(input, 1, 0, true).AddItem(status, 0, 0, false)
	view := newDialog(title, message, content, 1, 15)
	view.footer.SetText("↵ Continue  Esc Cancel")
	if secret {
		input.SetLabel(" Key › ")
		view.footer.SetText("↵ Save key  Esc Cancel · input is masked")
	}
	showError := func(message string) {
		status.SetText(message + "\nEnter to retry · Ctrl+U to clear.")
		content.ResizeItem(status, 3, 0)
		view.panel.ResizeItem(content, 4, 0)
	}
	input.SetAcceptanceFunc(func(text string, _ rune) bool {
		if len(text) > 8192 {
			showError("The key is too long. Paste only the subscription key.")
			return false
		}
		return true
	})
	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlU {
			input.SetText("")
			return nil
		}
		return event
	})
	input.SetChangedFunc(func(string) {
		status.SetText("")
		content.ResizeItem(status, 0, 0)
		view.panel.ResizeItem(content, 1, 0)
	})
	input.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		if strings.TrimSpace(input.GetText()) == "" {
			showError("Enter your subscription API key.")
			return
		}
		if err := submit(input.GetText()); err != nil {
			showError(err.Error())
			return
		}
		if secret {
			input.SetText("")
		}
	})
	return view, input
}

func (ui *terminalUI) run(build func(func(), *tview.Application) tview.Primitive) error {
	application := tview.NewApplication().EnableMouse(true).EnablePaste(true)
	cancelled := true
	root := build(func() { cancelled = false; application.Stop() }, application)
	application.SetRoot(root, true)
	application.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyCtrlC {
			application.Stop()
			return nil
		}
		return event
	})
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-ui.ctx.Done():
			application.QueueEvent(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
		case <-finished:
		}
	}()
	if err := application.Run(); err != nil {
		return err
	}
	if ui.ctx.Err() != nil {
		return ui.ctx.Err()
	}
	if cancelled {
		return errCancelled
	}
	return nil
}

func providerDialog(action string, status map[string]ai.ProviderStatus, selected func(string)) *dialog {
	choices := []choice{{name: "ChatGPT", value: "chatgpt"}, {name: "OpenCode Go", value: "opencode-go"}}
	for index := range choices {
		value := status[choices[index].value].Label
		if value == "" {
			value = "Not logged in"
		}
		color := app.Styles.GraphicsColor
		switch value {
		case "Logged in":
			color = app.Styles.TertiaryTextColor
		case "Account saved":
			color = app.Styles.ReadOnlyColor
		case "Plan access needed", "Sign in again":
			color = app.Styles.SecondaryTextColor
		case "Check credentials", "Check key", "Unavailable":
			color = app.Styles.ErrorColor
		}
		choices[index].name += fmt.Sprintf(" · [%s]%s[-]", color, tview.Escape(value))
	}
	return choiceDialog("LazySQL · AI "+action, "", choices, selected)
}

func (ui *terminalUI) Provider(action string) (string, error) {
	config, err := app.ReadAISettings(ui.configFile)
	if err != nil {
		return "", err
	}
	status := ai.ProviderLoginStatus(ui.ctx, config)
	var provider string
	err = ui.run(func(done func(), _ *tview.Application) tview.Primitive {
		return providerDialog(action, status, func(value string) { provider = value; done() })
	})
	return provider, err
}

func (ui *terminalUI) Key(title, message string, save func(string) error) error {
	return ui.run(func(done func(), _ *tview.Application) tview.Primitive {
		view, _ := textDialog(title, message, "", true, func(value string) error {
			if err := save(value); err != nil {
				return err
			}
			done()
			return nil
		})
		return view
	})
}

func (ui *terminalUI) Recover(message string) (bool, error) {
	retry := false
	err := ui.run(func(done func(), _ *tview.Application) tview.Primitive {
		return recoveryDialog(message, func(value string) { retry = value == "retry"; done() })
	})
	return retry, err
}

func recoveryDialog(message string, selected func(string)) *dialog {
	view := choiceDialog("Could not finish", message, []choice{{name: "Try again", value: "retry"}, {name: "Close", value: "close"}}, selected)
	view.description.SetTextColor(app.Styles.ErrorColor)
	return view
}

func confirmationDialog(title, message string, defaultYes bool, selected func(string)) *dialog {
	view := choiceDialog(title, message, []choice{{name: "Not now", value: "no"}, {name: "Yes", value: "yes"}}, selected)
	if defaultYes {
		view.options.SetCurrentItem(1)
	}
	return view
}

func (ui *terminalUI) Confirm(title, message string, defaultYes bool) (bool, error) {
	var confirmed bool
	err := ui.run(func(done func(), _ *tview.Application) tview.Primitive {
		return confirmationDialog(title, message, defaultYes, func(value string) { confirmed = value == "yes"; done() })
	})
	return confirmed, err
}
