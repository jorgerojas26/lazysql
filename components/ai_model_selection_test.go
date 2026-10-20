package components

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
	"github.com/jorgerojas26/lazysql/models"
)

func newAIModelSelectionSession(t *testing.T) (*aiSession, string) {
	t.Helper()
	setAIConfigForTest(t, true)
	t.Setenv("SELECTOR_TEST_KEY", "selector-secret")
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := app.UpdateAISettings(path, map[string]any{"enabled": true, "provider": "opencode-go", "model": "first", "api_format": "chat-completions", "api_key_env": "SELECTOR_TEST_KEY", "thinking": "low", "thinking_mode": "effort"}); err != nil {
		t.Fatal(err)
	}
	if err := app.LoadConfig(path); err != nil {
		t.Fatal(err)
	}
	session := newAILayoutSession(t)
	session.modelCatalog = []ai.Model{
		{Provider: "opencode-go", ID: "first", Name: "First Model", Format: "chat-completions", ContextWindow: 200000, ThinkingMode: "effort", ThinkingLevels: []ai.ThinkingLevel{{ID: "low"}, {ID: "high"}}},
		{Provider: "opencode-go", ID: "second", Name: "Second Model", Format: "chat-completions", ContextWindow: 400000, ThinkingMode: "effort", ThinkingLevels: []ai.ThinkingLevel{{ID: "low"}, {ID: "medium"}, {ID: "high"}}},
		{Provider: "opencode-go", ID: "fixed", Name: "Fixed Model", Format: "chat-completions", ContextWindow: 128000},
	}
	session.selectedModel = session.modelCatalog[0]
	session.catalogLoaded = true
	return session, path
}

func TestAIModelPickerSearchSelectionAndThinkingPersistence(t *testing.T) {
	session, path := newAIModelSelectionSession(t)
	session.appendMessage(ai.ChatMessage{Role: "assistant", Text: strings.Repeat("Previous conversation\n", 40)})
	session.refreshControls()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 40)
	mainPages.SetRect(0, 0, 100, 40)
	mainPages.Draw(screen)
	session.transcript.ScrollTo(5, 0)
	mainPages.Draw(screen)
	session.capture(tcell.NewEventKey(tcell.KeyCtrlO, 0, tcell.ModNone))
	if session.picker == nil || app.App.GetFocus() != session.picker.search {
		t.Fatal("Ctrl+O did not focus model search")
	}
	mainPages.Draw(screen)
	if !strings.Contains(simulationText(screen), "Select model") || !strings.Contains(simulationText(screen), "First Model") {
		t.Fatal("model picker is not visible")
	}
	session.picker.search.SetText("second")
	if len(session.picker.rows) != 1 {
		t.Fatal("model search did not filter")
	}
	session.picker.search.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if session.picker != nil || session.model != "second" || session.contextWindow != 400000 || app.App.GetFocus() != session.input {
		t.Fatalf("selecting model failed: model=%s window=%d picker=%v status=%s config=%+v", session.model, session.contextWindow, session.picker, session.picker.status.GetText(false), session.config)
	}
	if session.input.GetText() != "My draft request" || len(session.history) != 1 {
		t.Fatal("model selection discarded draft or conversation")
	}
	if row, _ := session.transcript.GetScrollOffset(); row != 5 {
		t.Fatal("model selection reset scroll position")
	}
	session.capture(tcell.NewEventKey(tcell.KeyCtrlT, 0, tcell.ModNone))
	if session.picker == nil || session.picker.kind != "thinking" {
		t.Fatal("Ctrl+T did not open thinking picker")
	}
	for row, option := range session.picker.rows {
		if option.Key == "high" {
			session.picker.table.Select(row, 0)
		}
	}
	session.picker.table.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if session.config.Thinking != "high" || session.config.ThinkingMode != "effort" {
		t.Fatal("thinking selection was not applied")
	}
	stored, err := app.ReadAISettings(path)
	if err != nil || stored.Model != "second" || stored.Thinking != "high" || stored.ThinkingPreferences["opencode-go/second"] != "high" {
		t.Fatalf("model preferences were not saved: %+v %v", stored, err)
	}
	content, _ := os.ReadFile(path)
	if strings.Contains(string(content), "selector-secret") {
		t.Fatal("selector persisted credentials")
	}
	session.openModelPicker()
	session.picker.selected(session.modelCatalog[0].Key())
	if session.config.Thinking != "low" {
		t.Fatal("the original model's thinking preference was not restored")
	}
	session.openModelPicker()
	session.picker.selected(session.modelCatalog[1].Key())
	if session.config.Thinking != "high" {
		t.Fatal("per-model thinking preference was not restored")
	}
	previous := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = previous })
	calls := 0
	http.DefaultTransport = aiE2ETransport(func(request *http.Request) (*http.Response, error) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "second" || body["reasoning_effort"] != "high" {
			t.Errorf("next request ignored selection: %+v", body)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"sql\":\"SELECT 1;\"}"}}]}`))}, nil
	})
	if _, err := session.client.Step(context.Background(), "instructions", []ai.Message{{Role: "user", Text: "Prompt"}}, nil); err != nil || calls != 1 {
		t.Fatalf("selected model request: %v, calls=%d", err, calls)
	}
	session.newChat()
	if session.config.Model != "second" || session.config.Thinking != "high" {
		t.Fatal("New chat discarded selected model/thinking")
	}
}

func TestAIModelPickerCancelLockingAndUnavailableThinking(t *testing.T) {
	session, _ := newAIModelSelectionSession(t)
	session.openModelPicker()
	picker := session.picker
	picker.search.SetText("no matching model")
	if len(picker.rows) != 0 {
		t.Fatal("empty search still has a selectable model")
	}
	picker.search.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	if session.picker != picker {
		t.Fatal("Enter on empty results changed selection")
	}
	picker.frame.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(tview.Primitive) {})
	if session.picker != nil || app.App.GetFocus() != session.input || session.model != "first" || session.input.GetText() != "My draft request" {
		t.Fatal("cancel did not preserve selection/draft/focus")
	}
	session.busy = true
	session.openModelPicker()
	session.openThinkingPicker()
	session.selectModel(session.modelCatalog[1], "high")
	if session.picker != nil || session.model != "first" {
		t.Fatal("selectors changed an active request")
	}
	session.busy = false
	session.selectModel(session.modelCatalog[2], "auto")
	session.openThinkingPicker()
	if len(session.picker.options) != 1 || session.picker.options[0].Key != "auto" || !strings.Contains(session.picker.status.GetText(false), "unavailable") {
		t.Fatalf("unsupported thinking controls: %+v status=%s config=%+v", session.picker.options, session.picker.status.GetText(false), session.config)
	}
}

func TestAIPromptFooterMouseAndResponsivePicker(t *testing.T) {
	session, _ := newAIModelSelectionSession(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, size := range [][2]int{{100, 40}, {48, 14}, {30, 10}} {
		screen.SetSize(size[0], size[1])
		mainPages.SetRect(0, 0, size[0], size[1])
		mainPages.Draw(screen)
		if session.input.modelWidth <= 0 {
			t.Fatal("model footer disappeared on narrow screen")
		}
		mainPages.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(session.input.modelX, session.input.footerY, tcell.ButtonNone, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
		if session.picker == nil || session.picker.kind != "models" {
			t.Fatal("clicking model footer did not open picker")
		}
		mainPages.Draw(screen)
		x, y, w, h := session.picker.frame.GetRect()
		if x < 0 || y < 0 || x+w > size[0] || y+h > size[1] {
			t.Fatal("picker overflows small terminal")
		}
		session.closePicker()
		mainPages.Draw(screen)
		if session.input.thinkingWidth > 0 {
			mainPages.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(session.input.thinkingX, session.input.footerY, tcell.ButtonNone, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
			if session.picker == nil || session.picker.kind != "thinking" {
				t.Fatal("clicking thinking footer did not open picker")
			}
			session.closePicker()
		}
	}
}

func TestAIContextOverflowRequiresExplicitConfirmationWithoutTrimming(t *testing.T) {
	session, _ := newAIModelSelectionSession(t)
	calls := 0
	session.client = aiCompletionFunc(func(context.Context, string) (string, error) { calls++; return `{"sql":"SELECT 1;"}`, nil })
	session.contextWindow = 100
	session.generate()
	if calls != 0 || session.busy || len(session.history) != 0 || session.picker == nil || session.picker.kind != "context-warning" {
		t.Fatal("oversized context was sent without warning")
	}
	if session.input.GetText() != "My draft request" {
		t.Fatal("context warning changed the draft")
	}
	session.closePicker()
	if len(session.history) != 0 {
		t.Fatal("context warning silently trimmed/changed history")
	}
}

func TestAIModelCatalogLoadsWithoutBlockingAndIsCached(t *testing.T) {
	session, _ := newAIModelSelectionSession(t)
	catalog := session.modelCatalog
	session.catalogLoaded, session.modelCatalog = false, nil
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	var calls atomic.Int32
	session.modelLoader = func(ctx context.Context, config models.AIConfig) ([]ai.Model, error) {
		calls.Add(1)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-release:
			return catalog, nil
		}
	}
	previous := app.App.Application
	application := tview.NewApplication()
	app.App.Application = application
	screen := tcell.NewSimulationScreen("UTF-8")
	application.SetScreen(screen).SetRoot(mainPages, true)
	done := make(chan error, 1)
	go func() { done <- application.Run() }()
	t.Cleanup(func() {
		unblock()
		session.cancel()
		application.Stop()
		if err := <-done; err != nil {
			t.Error(err)
		}
		app.App.Application = previous
	})
	application.QueueUpdateDraw(func() {
		session.openModelPicker()
		if session.picker == nil || !session.catalogLoading || !strings.Contains(session.picker.status.GetText(false), "Loading") {
			t.Error("picker did not open immediately while loading")
		}
		session.closePicker()
		if session.input.GetText() != "My draft request" {
			t.Error("loading/cancel changed draft")
		}
	})
	unblock()
	deadline := time.Now().Add(3 * time.Second)
	loaded := false
	for time.Now().Before(deadline) {
		application.QueueUpdateDraw(func() { loaded = session.catalogLoaded })
		if loaded {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !loaded {
		t.Fatal("model catalog never finished loading")
	}
	application.QueueUpdateDraw(func() {
		if session.picker != nil {
			t.Error("late catalog completion reopened a canceled picker")
		}
		session.openModelPicker()
		if len(session.picker.options) != 3 || calls.Load() != 1 {
			t.Error("cached picker repeated catalog fetch or lost models")
		}
		session.closePicker()
		session.openThinkingPicker()
		if len(session.picker.options) != 3 || calls.Load() != 1 {
			t.Error("thinking picker did not reuse model capabilities")
		}
		session.closePicker()
	})
}

func TestAISelectionSaveFailurePreservesCurrentModel(t *testing.T) {
	session, path := newAIModelSelectionSession(t)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	session.openModelPicker()
	session.picker.selected(session.modelCatalog[1].Key())
	if session.model != "first" || session.picker == nil || !strings.Contains(session.picker.status.GetText(false), "Selection not changed") {
		t.Fatal("failed persistence silently changed the active model")
	}
	if app.App.AIConfig().Model != "first" {
		t.Fatal("failed persistence changed application defaults")
	}
}

func TestAIThinkingShortcutOverridesThemeOnlyInsideAIModal(t *testing.T) {
	session, _ := newAIModelSelectionSession(t)
	previous := app.App.OnModalInput
	app.App.OnModalInput = captureAIModalShortcut
	t.Cleanup(func() { app.App.OnModalInput = previous })
	event := tcell.NewEventKey(tcell.KeyCtrlT, 0, tcell.ModNone)
	if capture := app.App.GetInputCapture(); capture != nil {
		if capture(event) != nil || session.picker == nil || session.picker.kind != "thinking" {
			t.Fatal("global input capture intercepted thinking shortcut")
		}
	} else {
		t.Fatal("test application has no global input capture")
	}
	if !captureAIModalShortcut(event) || session.picker != nil || app.App.GetFocus() != session.input {
		t.Fatal("application-level thinking shortcut did not toggle picker closed")
	}
	session.busy = true
	if !captureAIModalShortcut(event) || session.picker != nil {
		t.Fatal("busy modal did not consume/lock thinking shortcut")
	}
	session.busy = false
	session.hide()
	if captureAIModalShortcut(event) {
		t.Fatal("AI shortcut intercepted theme picker outside AI modal")
	}
}

func TestAIModelPickerKeepsCurrentModelVisibleInLongCatalog(t *testing.T) {
	session, _ := newAIModelSelectionSession(t)
	session.modelCatalog = nil
	for index := range 30 {
		session.modelCatalog = append(session.modelCatalog, ai.Model{ID: fmt.Sprintf("model-%02d", index), Name: fmt.Sprintf("Model %02d", index), Provider: "opencode-go", Format: "chat-completions"})
	}
	session.selectedModel = session.modelCatalog[29]
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, size := range [][2]int{{100, 40}, {48, 14}} {
		screen.SetSize(size[0], size[1])
		mainPages.SetRect(0, 0, size[0], size[1])
		session.openModelPicker()
		mainPages.Draw(screen)
		if !strings.Contains(simulationText(screen), "Model 29") {
			t.Fatal("current model is scrolled out of the picker")
		}
		session.closePicker()
	}
}

type aiSelectionStep func(context.Context, string, []ai.Message, []ai.Tool) (ai.Turn, error)

func (step aiSelectionStep) Step(ctx context.Context, instructions string, messages []ai.Message, tools []ai.Tool) (ai.Turn, error) {
	return step(ctx, instructions, messages, tools)
}

func TestAIContextWarningCanSendFullHistoryWithExplicitConfirmation(t *testing.T) {
	previous := app.App.Application
	application := tview.NewApplication()
	app.App.Application = application
	t.Cleanup(func() { app.App.Application = previous })
	session, _ := newAIModelSelectionSession(t)
	session.history = []ai.ChatMessage{{Role: "user", Text: strings.Repeat("Earlier request ", 4000)}, {Role: "assistant", Text: "Earlier answer"}}
	session.historyVersion++
	session.contextWindow = 100
	session.input.SetText("First line\nSecond line\nThird line")
	requests := make(chan ai.Request, 1)
	session.client = aiSelectionStep(func(_ context.Context, _ string, messages []ai.Message, _ []ai.Tool) (ai.Turn, error) {
		var request ai.Request
		if err := json.Unmarshal([]byte(messages[0].Text), &request); err != nil {
			return ai.Turn{}, err
		}
		requests <- request
		return ai.Turn{Text: `{"message":"Done"}`}, nil
	})
	screen := tcell.NewSimulationScreen("UTF-8")
	application.SetScreen(screen).SetRoot(mainPages, true)
	done := make(chan error, 1)
	go func() { done <- application.Run() }()
	t.Cleanup(func() {
		session.cancel()
		application.Stop()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	application.QueueUpdateDraw(func() {
		session.generate()
		if session.picker == nil || session.picker.kind != "context-warning" {
			t.Error("oversized context skipped confirmation")
			return
		}
		session.picker.selected("send")
	})
	select {
	case request := <-requests:
		if request.Prompt != "First line\nSecond line\nThird line" {
			t.Error("sending a multiline draft lost its line breaks")
		}
		if len(request.History) != 2 || len(request.History[0].Text) != len(strings.Repeat("Earlier request ", 4000)) || request.HistoryTruncated {
			t.Error("context confirmation silently pruned history")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("explicit context confirmation did not send the request")
	}
	deadline := time.Now().Add(3 * time.Second)
	finished := false
	for time.Now().Before(deadline) {
		application.QueueUpdateDraw(func() { finished = !session.busy })
		if finished {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !finished {
		t.Fatal("confirmed generation did not finish")
	}
}

func TestAISelectorShortcutsToggleAndSwitchPickers(t *testing.T) {
	for _, target := range []struct {
		kind string
		key  tcell.Key
	}{{"models", tcell.KeyCtrlO}, {"thinking", tcell.KeyCtrlT}} {
		t.Run(target.kind, func(t *testing.T) {
			session, _ := newAIModelSelectionSession(t)
			session.capture(tcell.NewEventKey(target.key, 0, tcell.ModNone))
			if session.picker == nil || session.picker.kind != target.kind {
				t.Fatal("shortcut did not open picker")
			}
			if target.kind == "models" {
				session.picker.search.SetText("second")
			}
			picker := session.picker
			picker.frame.InputHandler()(tcell.NewEventKey(target.key, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
			if session.picker != nil || session.surface.HasPage("ai-picker") || app.App.GetFocus() != session.input {
				t.Fatal("repeated shortcut did not close picker and restore composer")
			}
			if session.model != "first" || session.config.Thinking != "low" || session.input.GetText() != "My draft request" {
				t.Fatal("toggle changed selection or draft")
			}
			session.capture(tcell.NewEventKey(target.key, 0, tcell.ModNone))
			if session.picker == nil || session.picker.kind != target.kind {
				t.Fatal("shortcut did not reopen picker")
			}
			otherKey, otherKind := tcell.KeyCtrlT, "thinking"
			if target.kind == "thinking" {
				otherKey, otherKind = tcell.KeyCtrlO, "models"
			}
			session.picker.frame.InputHandler()(tcell.NewEventKey(otherKey, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
			if session.picker == nil || session.picker.kind != otherKind {
				t.Fatal("other selector shortcut did not switch pickers")
			}
		})
	}
}

func TestAIPickerCtrlNPNavigatesWithoutTogglingOrClearingChat(t *testing.T) {
	for _, kind := range []string{"models", "thinking"} {
		t.Run(kind, func(t *testing.T) {
			session, _ := newAIModelSelectionSession(t)
			session.appendMessage(ai.ChatMessage{Role: "user", Text: "Keep this conversation"})
			if kind == "models" {
				session.openModelPicker()
			} else {
				session.openThinkingPicker()
			}
			picker := session.picker
			focuses := []tview.Primitive{picker.table}
			if picker.searchable {
				focuses = append(focuses, picker.search)
			}
			for _, focus := range focuses {
				session.focus(focus)
				first := 0
				if kind == "models" {
					first = 1
				}
				picker.table.Select(first, 0)
				picker.frame.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlN, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
				if row, _ := picker.table.GetSelection(); row != first+1 {
					t.Fatal("Ctrl+N did not move to the next item")
				}
				picker.frame.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
				if row, _ := picker.table.GetSelection(); row != first {
					t.Fatal("Ctrl+P did not move to the previous item")
				}
				if session.picker != picker || app.App.GetFocus() != focus || len(session.history) != 1 || session.input.GetText() != "My draft request" || session.model != "first" || session.config.Thinking != "low" {
					t.Fatal("list navigation changed focus, closed picker, or changed chat/selection")
				}
			}
		})
	}
}
