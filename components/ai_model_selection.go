package components

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"maps"
	"strconv"
	"strings"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func providerLabel(provider string) string {
	if provider == "chatgpt" {
		return "ChatGPT"
	}
	if provider == "opencode-go" {
		return "OpenCode Go"
	}
	return provider
}

func thinkingLabel(level string) string {
	switch level {
	case "", "auto":
		return "Auto"
	case "none":
		return "Off"
	case "on":
		return "On"
	case "xhigh":
		return "Extra high"
	case "max":
		return "Maximum"
	default:
		return strings.ToUpper(level[:1]) + level[1:]
	}
}

func modelWindow(window int) string {
	if window <= 0 {
		return "Window unknown"
	}
	if window >= 1000000 {
		return strconv.FormatFloat(float64(window)/1000000, 'f', -1, 64) + "M"
	}
	if window >= 1000 {
		return strconv.FormatFloat(float64(window)/1000, 'f', -1, 64) + "k"
	}
	return strconv.Itoa(window)
}

func (session *aiSession) showPicker(picker *aiPicker) {
	if session.busy {
		return
	}
	session.picker = picker
	session.surface.AddPage("ai-picker", picker, true, true)
	if picker.searchable {
		session.focus(picker.search)
	} else {
		session.focus(picker.table)
	}
}

func (session *aiSession) closePicker() {
	if session.picker == nil {
		return
	}
	session.picker = nil
	session.surface.RemovePage("ai-picker")
	session.focus(session.input)
}

func (session *aiSession) openModelPicker() {
	if session.busy {
		return
	}
	if session.picker != nil && session.picker.kind == "models" {
		session.closePicker()
		return
	}
	picker := newAIPicker(session, "models", "Select model · Ctrl+O", func(key string) {
		for _, model := range session.modelCatalog {
			if model.Key() == key {
				level := app.App.AIConfig().ThinkingPreferences[key]
				config := session.config
				if model.ConfigureThinking(&config, level) != nil {
					level = ""
				}
				session.selectModel(model, level)
				return
			}
		}
		if key == session.selectedModel.Key() {
			session.closePicker()
		}
	})
	picker.retry = func() { session.catalogLoaded = false; session.loadModelCatalog() }
	session.showPicker(picker)
	session.refreshModelPicker()
	session.loadModelCatalog()
}

func (session *aiSession) openThinkingPicker() {
	if session.busy {
		return
	}
	if session.picker != nil && session.picker.kind == "thinking" {
		session.closePicker()
		return
	}
	picker := newAIPicker(session, "thinking", "Thinking · Ctrl+T", func(level string) { session.selectModel(session.selectedModel, level) })
	picker.retry = func() { session.catalogLoaded = false; session.loadModelCatalog() }
	session.showPicker(picker)
	session.refreshModelPicker()
	session.loadModelCatalog()
}

func (session *aiSession) refreshModelPicker() {
	if session.picker == nil {
		return
	}
	picker := session.picker
	status := session.catalogError
	if session.catalogLoading {
		status = "Loading available models…"
	}
	if picker.kind == "models" {
		var options []aiPickerOption
		for _, model := range session.modelCatalog {
			options = append(options, aiPickerOption{Key: model.Key(), Label: model.Name, Detail: modelWindow(model.ContextWindow), Group: providerLabel(model.Provider), Search: model.ID + " " + model.Format + " " + model.Provider})
		}
		if len(options) == 0 && session.selectedModel.ID != "" {
			model := session.selectedModel
			options = append(options, aiPickerOption{Key: model.Key(), Label: model.Name, Detail: "Current", Group: providerLabel(model.Provider)})
		}
		picker.setOptions(options, session.selectedModel.Key(), status)
		return
	}
	if picker.kind != "thinking" {
		return
	}
	options := []aiPickerOption{{Key: "auto", Label: "Auto", Detail: "Model default"}}
	for _, level := range session.selectedModel.ThinkingLevels {
		detail := "Reasoning effort"
		switch level.ID {
		case "none":
			detail = "Disable thinking"
		case "low", "minimal":
			detail = "Lower latency"
		case "medium":
			detail = "Balanced reasoning"
		case "high", "xhigh", "max":
			detail = "More reasoning"
		case "on":
			detail = "Enable thinking"
		}
		options = append(options, aiPickerOption{Key: level.ID, Label: thinkingLabel(level.ID), Detail: detail})
	}
	if session.catalogLoaded && len(options) == 1 && status == "" {
		status = "Adjustable thinking is unavailable for this model."
	}
	current := session.config.Thinking
	if current == "" {
		current = "auto"
	}
	picker.setOptions(options, current, status)
}

func (session *aiSession) loadModelCatalog() {
	if session.catalogLoading || session.catalogLoaded {
		return
	}
	session.catalogLoading = true
	session.catalogError = ""
	session.refreshModelPicker()
	config, application := session.config, app.App.Application
	loader := session.modelLoader
	if loader == nil {
		loader = ai.AvailableModels
	}
	go func() {
		catalog, err := loader(session.ctx, config)
		if session.ctx.Err() != nil {
			return
		}
		application.QueueUpdateDraw(func() {
			if !session.active() {
				return
			}
			session.catalogLoading, session.catalogLoaded = false, true
			session.modelCatalog = catalog
			if err != nil {
				session.catalogError = err.Error()
			}
			for _, model := range catalog {
				if model.Key() == session.selectedModel.Key() {
					session.selectedModel = model
					if model.ContextWindow > 0 {
						session.contextWindow = model.ContextWindow
					}
					break
				}
			}
			session.refreshModelPicker()
			session.scheduleContext()
		})
	}()
}

func (session *aiSession) selectModel(model ai.Model, level string) {
	if session.busy {
		return
	}
	config := session.config
	config.Provider, config.Model, config.APIFormat, config.ContextWindow = model.Provider, model.ID, model.Format, model.ContextWindow
	if err := model.ConfigureThinking(&config, level); err != nil {
		if session.picker != nil {
			session.picker.status.SetText(err.Error())
		}
		return
	}
	config.ThinkingPreferences = maps.Clone(app.App.AIConfig().ThinkingPreferences)
	if config.ThinkingPreferences == nil {
		config.ThinkingPreferences = map[string]string{}
	}
	if session.selectedModel.ID != "" {
		config.ThinkingPreferences[session.selectedModel.Key()] = session.config.Thinking
	}
	config.ThinkingPreferences[model.Key()] = config.Thinking
	client, err := ai.New(config)
	if err == nil {
		err = app.App.SaveAISelection(config)
	}
	if err != nil {
		if session.picker != nil {
			session.picker.status.SetText("Selection not changed: " + err.Error())
		}
		return
	}
	session.config, session.selectedModel, session.client = config, model, client
	session.model, session.contextWindow = model.ID, model.ContextWindow
	session.nextContext = ai.MeasureContext(model.ID, session.makeRequest(session.input.GetText()))
	session.refreshContext()
	session.scheduleContext()
	session.closePicker()
}

func (session *aiSession) confirmContext(text string, request ai.Request, estimate int) {
	picker := newAIPicker(session, "context-warning", "Context may not fit", func(choice string) {
		session.closePicker()
		if choice == "model" {
			session.openModelPicker()
			return
		}
		if !session.busy && strings.TrimSpace(session.input.GetText()) == text {
			session.sendRequest(text, request)
		}
	})
	picker.setOptions([]aiPickerOption{{Key: "model", Label: "Choose another model"}, {Key: "send", Label: "Send anyway", Detail: "Nothing will be trimmed"}}, "model", fmt.Sprintf("~%s input + output room / %s window", compactCount(estimate), modelWindow(session.contextWindow)))
	session.showPicker(picker)
}

func (session *aiSession) selectedThinkingLabel() string {
	if session.catalogLoaded && len(session.selectedModel.ThinkingLevels) == 0 {
		return "Unavailable"
	}
	return thinkingLabel(session.config.Thinking)
}

func captureAIModalShortcut(event *tcell.EventKey) bool {
	if event.Key() != tcell.KeyCtrlT || mainPages == nil {
		return false
	}
	name, page := mainPages.GetFrontPage()
	if name != "ai-modal" {
		return false
	}
	surface, ok := page.(*tview.Pages)
	if !ok {
		return false
	}
	_, modal := surface.GetFrontPage()
	switch view := modal.(type) {
	case *aiModal:
		view.session.openThinkingPicker()
		return true
	case *aiPicker:
		view.session.openThinkingPicker()
		return true
	}
	return false
}
