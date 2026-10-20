package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

func (prompt *aiPrompt) Draw(screen tcell.Screen) {
	prompt.TextArea.Draw(screen)
	prompt.modelWidth, prompt.thinkingWidth = 0, 0
	x, y, width, height := prompt.GetRect()
	if height < 2 || width < 8 {
		return
	}
	model := prompt.session.selectedModel.Name
	if model == "" {
		model = "Select model"
	}
	thinking := prompt.session.selectedThinkingLabel()
	if !prompt.session.busy {
		model += " ▾"
		if thinking != "Unavailable" {
			thinking += " ▾"
		}
	}
	available := width - 4
	thinkingText := "Thinking: " + thinking
	if available < 48 {
		thinkingText = thinking
	}
	thinkingWidth := tview.TaggedStringWidth(tview.Escape(thinkingText))
	modelWidth := available - thinkingWidth - 3
	if modelWidth < 6 {
		modelWidth, thinkingWidth = available, 0
	}
	modelWidth = min(modelWidth, tview.TaggedStringWidth(tview.Escape(model)))
	prompt.footerY, prompt.modelX, prompt.modelWidth = y+height-1, x+2, modelWidth
	prompt.thinkingX, prompt.thinkingWidth = prompt.modelX+modelWidth+3, thinkingWidth
	color := app.Styles.PrimaryTextColor
	if prompt.session.busy {
		color = app.Styles.SecondaryTextColor
	}
	tview.Print(screen, tview.Escape(model), prompt.modelX, prompt.footerY, modelWidth, tview.AlignLeft, color)
	if thinkingWidth > 0 {
		tview.Print(screen, " · ", prompt.modelX+modelWidth, prompt.footerY, 3, tview.AlignLeft, app.Styles.SecondaryTextColor)
		tview.Print(screen, tview.Escape(thinkingText), prompt.thinkingX, prompt.footerY, thinkingWidth, tview.AlignLeft, color)
	}
}

func (prompt *aiPrompt) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return func(action tview.MouseAction, event *tcell.EventMouse, focus func(tview.Primitive)) (bool, tview.Primitive) {
		x, y := event.Position()
		if action == tview.MouseLeftClick && prompt.InRect(x, y) && y == prompt.footerY {
			if x >= prompt.modelX && x < prompt.modelX+prompt.modelWidth {
				prompt.session.openModelPicker()
				return true, nil
			}
			if x >= prompt.thinkingX && x < prompt.thinkingX+prompt.thinkingWidth {
				prompt.session.openThinkingPicker()
				return true, nil
			}
		}
		return prompt.TextArea.MouseHandler()(action, event, focus)
	}
}
