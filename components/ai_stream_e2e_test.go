package components

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/pelletier/go-toml/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/internal/ai"
	"github.com/jorgerojas26/lazysql/models"
)

type aiE2ETransport func(*http.Request) (*http.Response, error)

func (transport aiE2ETransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

type aiE2EDriver struct {
	*drivers.SQLite
	executions atomic.Int32
}

func (driver *aiE2EDriver) ExecuteQuery(context.Context, string, string) ([][]string, int, error) {
	driver.executions.Add(1)
	return nil, 0, errors.New("generated SQL must not execute")
}
func (driver *aiE2EDriver) ExecuteDMLStatement(context.Context, string, string) (string, error) {
	driver.executions.Add(1)
	return "", errors.New("generated SQL must not execute")
}
func (driver *aiE2EDriver) StreamQuery(context.Context, string, string, int, func(drivers.QueryBatch) error) (drivers.QueryStreamResult, error) {
	driver.executions.Add(1)
	return drivers.QueryStreamResult{}, errors.New("generated SQL must not execute")
}

func TestAIChatStreamingEndToEnd(t *testing.T) {
	setAIConfigForTest(t, true)
	t.Setenv("E2E_SQL_API_KEY", "fixture-key")
	configPath := filepath.Join(t.TempDir(), "config.toml")
	config, _ := toml.Marshal(struct {
		AI models.AIConfig `toml:"ai"`
	}{models.AIConfig{Enabled: true, Provider: "opencode-go", Model: "fixture-model", APIFormat: "responses", APIKeyEnv: "E2E_SQL_API_KEY"}})
	if err := os.WriteFile(configPath, config, 0600); err != nil {
		t.Fatal(err)
	}
	if err := app.LoadConfig(configPath); err != nil {
		t.Fatal(err)
	}
	driver := &aiE2EDriver{SQLite: &drivers.SQLite{}}
	if err := driver.Connect(context.Background(), filepath.Join(t.TempDir(), "app")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { driver.Connection.Close() })
	if _, err := driver.Connection.ExecContext(context.Background(), "CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL, created_at TEXT NOT NULL, verified INTEGER NOT NULL DEFAULT 0)"); err != nil {
		t.Fatal(err)
	}
	query := "SELECT id, email\nFROM users\nORDER BY created_at DESC\nLIMIT 10;"
	requests := atomic.Int32{}
	started := make(chan int, 8)
	canceled := make(chan int, 8)
	allowFirst := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(allowFirst) }) }
	allowFailure := make(chan struct{})
	var failureOnce sync.Once
	releaseFailure := func() { failureOnce.Do(func() { close(allowFailure) }) }
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		number := int(requests.Add(1))
		var body map[string]any
		if json.NewDecoder(request.Body).Decode(&body) != nil || body["stream"] != true || body["store"] != false || body["tools"] == nil || request.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Error("end-to-end request bypassed native streaming protocol")
		}
		if number == 5 {
			inputs, _ := body["input"].([]any)
			first, _ := inputs[0].(map[string]any)
			content, _ := first["content"].(string)
			var context ai.Request
			if json.Unmarshal([]byte(content), &context) != nil || len(context.Tables) == 0 || len(context.Tables[0].Columns) < 4 {
				t.Error("refinement did not reuse verified schema")
			}
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		event := func(value any) { data, _ := json.Marshal(value); fmt.Fprintf(writer, "data: %s\n\n", data) }
		if number <= 3 {
			names := []string{"list_databases", "list_tables", "describe_tables"}
			args := []string{`{}`, `{"database":"app"}`, `{"database":"app","names":["users"]}`}
			event(map[string]any{"type": "response.output_item.done", "output_index": 0, "item": map[string]any{"type": "function_call", "id": fmt.Sprintf("fc%d", number), "call_id": fmt.Sprintf("call%d", number), "name": names[number-1], "arguments": args[number-1]}})
			event(map[string]any{"type": "response.completed", "response": map[string]any{"status": "completed", "usage": map[string]any{"input_tokens": 100 * number, "output_tokens": 5, "input_tokens_details": map[string]int{"cached_tokens": 20}}}})
			return
		}
		if number == 4 {
			data, _ := json.Marshal(body)
			if !strings.Contains(string(data), "created_at") || !strings.Contains(string(data), "verified") {
				t.Error("real SQLite schema never reached provider")
			}
		}
		sql := query
		if number > 4 {
			sql = "SELECT id, email\nFROM users\nWHERE verified = 1\nORDER BY created_at DESC\nLIMIT 10;"
		}
		output, _ := json.Marshal(ai.Result{Message: "Here are the latest users.", Database: "app", SQL: sql})
		cut := strings.Index(string(output), "ORDER BY")
		if cut < 1 {
			t.Error("invalid streaming fixture")
			return
		}
		event(map[string]any{"type": "response.output_text.delta", "delta": string(output[:cut])})
		writer.(http.Flusher).Flush()
		started <- number
		if number == 7 {
			select {
			case <-allowFailure:
			case <-request.Context().Done():
			}
			return
		}
		if number == 4 {
			select {
			case <-allowFirst:
			case <-request.Context().Done():
				canceled <- number
				return
			}
			event(map[string]any{"type": "response.output_text.delta", "delta": string(output[cut:])})
			event(map[string]any{"type": "response.completed", "response": map[string]any{"status": "completed", "usage": map[string]any{"input_tokens": 400, "output_tokens": 40, "input_tokens_details": map[string]int{"cached_tokens": 100}}}})
		} else {
			<-request.Context().Done()
			canceled <- number
		}
	}))
	t.Cleanup(server.Close)
	t.Cleanup(release)
	t.Cleanup(releaseFailure)
	endpoint, _ := url.Parse(server.URL)
	previousTransport := http.DefaultTransport
	http.DefaultTransport = aiE2ETransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "opencode.ai" || request.URL.Path != "/inference/go/openai/v1/responses" {
			return nil, errors.New("unexpected external HTTP request during end-to-end test")
		}
		clone := request.Clone(request.Context())
		clone.URL.Scheme = endpoint.Scheme
		clone.URL.Host = endpoint.Host
		return previousTransport.RoundTrip(clone)
	})
	t.Cleanup(func() { http.DefaultTransport = previousTransport })
	previousApplication, previousPages := App.Application, mainPages
	App.Application = tview.NewApplication()
	App.EnableMouse(true)
	mainPages = MainPages()
	home := NewHomePage(models.Connection{Name: "stream-e2e", Provider: drivers.DriverSqlite, URL: "sqlite3:///app", DBName: "app", ReadOnly: true}, driver)
	mainPages.AddAndSwitchToPage("stream-e2e", home, true)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	App.SetScreen(screen)
	screen.SetSize(120, 36)
	done := make(chan error, 1)
	go func() { done <- App.Run(mainPages, filepath.Join(t.TempDir(), "runtime.toml")) }()
	App.QueueUpdate(func() {})
	t.Cleanup(func() {
		App.QueueUpdate(func() {
			if home.aiChat != nil {
				home.aiChat.close()
			}
			if home.Tree.loadCancel != nil {
				home.Tree.loadCancel()
			}
		})
		App.Application.Stop()
		if err := <-done; err != nil {
			t.Error(err)
		}
		App.Application, mainPages = previousApplication, previousPages
	})
	wait := func(label string, condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			ready := false
			App.QueueUpdateDraw(func() { ready = condition() })
			if ready {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatalf("end-to-end timeout: %s", label)
	}
	press := func(key tcell.Key) { App.QueueEvent(tcell.NewEventKey(key, 0, tcell.ModNone)) }
	send := func(text string) {
		App.QueueUpdateDraw(func() { mainPages.PasteHandler()(text, func(p tview.Primitive) { App.SetFocus(p) }) })
		press(tcell.KeyEnter)
	}
	typeDraft := func(text string) {
		for _, character := range text {
			App.QueueEvent(tcell.NewEventKey(tcell.KeyRune, character, tcell.ModNone))
		}
		wait("editable composer during response", func() bool { return home.aiChat != nil && home.aiChat.input.GetText() == text })
	}
	press(tcell.KeyCtrlG)
	wait("open chat", func() bool { return home.aiChat != nil && home.aiChat.visible })
	session := home.aiChat
	App.QueueUpdateDraw(func() {
		mainPages.Draw(screen)
		_, _, _, height := session.panel.GetRect()
		if height != 4 || strings.Contains(simulationText(screen), "Ctrl+N New chat") {
			t.Error("fresh chat did not open as a compact input")
		}
	})
	send("Give me the last 10 registered users")
	select {
	case number := <-started:
		if number != 4 {
			t.Fatalf("expected discovery plus answer, got request %d", number)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("provider did not send first answer chunk")
	}
	wait("visible SQL before completion", func() bool {
		mainPages.Draw(screen)
		text := simulationText(screen)
		return strings.Contains(text, "Here are the latest users.") && strings.Contains(text, "╭─ SQL") && strings.Contains(text, "SELECT id, email") && session.streaming.SQL != ""
	})
	App.QueueUpdate(func() {
		_, _, _, height := session.panel.GetRect()
		if height != 26 {
			t.Error("sending did not expand the compact input into full chat")
		}
		if !session.busy || session.result.SQL != "" || len(session.history) != 1 || !session.useButton.IsDisabled() {
			t.Error("partial SQL became a completed usable answer")
		}
		found := false
		for y := 0; y < 36; y++ {
			for x := 0; x < 115; x++ {
				var word strings.Builder
				for offset := 0; offset < 6; offset++ {
					character, _, _, _ := screen.GetContent(x+offset, y)
					word.WriteRune(character)
				}
				if word.String() == "SELECT" {
					_, _, style, _ := screen.GetContent(x, y)
					foreground, _, _ := style.Decompose()
					if foreground == app.Styles.SQLKeywordColor {
						found = true
					}
				}
			}
		}
		if !found {
			t.Error("visible streamed SQL was not syntax highlighted")
		}
	})
	press(tcell.KeyCtrlK)
	wait("context details", func() bool { name, _ := session.content.GetFrontPage(); return name == "context" })
	App.QueueUpdate(func() {
		if !strings.Contains(session.contextDetails.GetText(false), "300 tokens") || session.metricTotals.Calls != 4 || session.metricTotals.Input != 600 {
			t.Error("context details did not report actual planning usage")
		}
		if session.currentMetrics.Round == 0 || session.contextMeter.GetText(false) == "" {
			t.Error("active request context is not visible")
		}
		session.capture(tcell.NewEventKey(tcell.KeyRune, '2', tcell.ModNone))
		if name, _ := session.contextTabs.GetFrontPage(); name != "conversation" || App.GetFocus() != session.requestDetails {
			t.Error("conversation tab did not open")
		}
		if !json.Valid([]byte(session.requestBody)) || session.requestRound != 4 {
			t.Error("active request body was not captured")
		}
		for _, text := range []string{"instructions", "Give me the last 10 registered users", "describe_tables", "function_call_output", "verified"} {
			if !strings.Contains(session.requestDetails.GetText(false), text) {
				t.Errorf("request inspection omitted %q", text)
			}
		}
		if strings.Contains(session.requestDetails.GetText(false), "fixture-key") {
			t.Error("request inspection exposed credentials")
		}
	})
	press(tcell.KeyCtrlK)
	wait("return to chat", func() bool { name, _ := session.content.GetFrontPage(); return name == "chat" })
	typeDraft("Only include verified users")
	press(tcell.KeyCtrlG)
	wait("hide while thinking", func() bool { return !session.visible })
	press(tcell.KeyCtrlG)
	wait("reopen editable composer while thinking", func() bool { return session.visible && App.GetFocus() == session.input && !session.input.GetDisabled() })
	App.QueueUpdateDraw(func() {
		mainPages.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { App.SetFocus(p) })
		if !session.busy || session.input.GetDisabled() || session.input.GetText() != "Only include verified users" || requests.Load() != 4 {
			t.Error("Enter while busy changed or sent the draft")
		}
	})
	App.QueueUpdateDraw(func() {
		mainPages.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModNone), func(p tview.Primitive) { App.SetFocus(p) })
		if session.table.Editor.GetText() != "" {
			t.Error("partial SQL was applied")
		}
	})
	release()
	wait("completed reply", func() bool { return !session.busy && session.result.SQL == query && len(session.history) == 2 })
	App.QueueUpdate(func() {
		if session.metricTotals.Calls != 4 || session.metricTotals.Reports != 4 || session.metricTotals.Input != 1000 || session.metricTotals.Output != 55 || session.metricTotals.Cached != 160 {
			t.Errorf("usage totals counted tool updates incorrectly: %+v", session.metricTotals)
		}
	})
	App.QueueUpdate(func() {
		if session.input.GetText() != "Only include verified users" {
			t.Error("completed response overwrote the next draft")
		}
	})
	var clickX, clickY int
	App.QueueUpdateDraw(func() {
		mainPages.Draw(screen)
		var found bool
		clickX, clickY, found = findAIChatText(screen, "SELECT id, email")
		if !found {
			t.Error("completed SQL block is not visible for clicking")
		}
	})
	App.QueueEvent(tcell.NewEventMouse(clickX, clickY, tcell.Button1, tcell.ModNone))
	App.QueueEvent(tcell.NewEventMouse(clickX, clickY, tcell.ButtonNone, tcell.ModNone))
	wait("click SQL block to use query", func() bool { return !session.visible && session.table.Editor.GetText() == query })
	if driver.executions.Load() != 0 {
		t.Fatal("Use query executed SQL")
	}
	press(tcell.KeyCtrlG)
	wait("reopen chat", func() bool { return session.visible })
	App.QueueUpdate(func() {
		if App.GetFocus() != session.input || session.input.GetText() != "Only include verified users" {
			t.Error("reopening lost draft or input focus")
		}
	})
	press(tcell.KeyEnter)
	select {
	case number := <-started:
		if number != 5 {
			t.Fatalf("wrong refinement request %d", number)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("refinement did not stream")
	}
	wait("refinement preview", func() bool { return session.busy && strings.Contains(session.streaming.SQL, "verified") })
	typeDraft("Keep this next draft")
	press(tcell.KeyCtrlX)
	wait("Stop stream", func() bool { return !session.busy && session.streaming.SQL == "" })
	select {
	case number := <-canceled:
		if number != 5 {
			t.Errorf("wrong canceled stream %d", number)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not cancel provider HTTP stream")
	}
	App.QueueUpdate(func() {
		if session.table.Editor.GetText() != query || session.result.SQL != query {
			t.Error("stopped partial query replaced valid SQL")
		}
		if session.input.GetText() != "Keep this next draft" {
			t.Error("Stop overwrote the typed draft")
		}
		session.input.SetText("")
	})
	send("Add a verified filter again")
	select {
	case number := <-started:
		if number != 6 {
			t.Fatalf("wrong clear-session request %d", number)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("third answer did not stream")
	}
	wait("third partial answer", func() bool { return session.streaming.SQL != "" })
	press(tcell.KeyCtrlN)
	wait("New chat", func() bool {
		return !session.busy && len(session.history) == 0 && session.streaming.SQL == "" && session.result.SQL == ""
	})
	App.QueueUpdateDraw(func() {
		mainPages.Draw(screen)
		_, _, _, height := session.panel.GetRect()
		if height != 4 || strings.Contains(simulationText(screen), "Ctrl+N New chat") {
			t.Error("New chat did not collapse to a compact input")
		}
	})
	select {
	case number := <-canceled:
		if number != 6 {
			t.Errorf("wrong cleared stream %d", number)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("New chat did not cancel provider HTTP stream")
	}
	send("A response that gets interrupted")
	select {
	case number := <-started:
		if number != 7 {
			t.Errorf("wrong interrupted request %d", number)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("interrupted response never started")
	}
	wait("interrupted stream preview", func() bool { return session.streaming.SQL != "" })
	typeDraft("Another draft during failure")
	releaseFailure()
	wait("discard interrupted output", func() bool { return !session.busy && len(session.history) == 2 })
	App.QueueUpdate(func() {
		if session.input.GetText() != "Another draft during failure" {
			t.Error("failed response overwrote the typed draft")
		}
		if session.history[1].SQL != "" || !session.history[1].Error || session.streaming.SQL != "" || session.result.SQL != "" {
			t.Error("interrupted partial SQL entered completed history")
		}
		if session.table.Editor.GetText() != query || driver.executions.Load() != 0 {
			t.Error("stream lifecycle altered or executed the editor query")
		}
		if requests.Load() != 7 {
			t.Errorf("unexpected provider calls: %d", requests.Load())
		}
	})
}
