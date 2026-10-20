package components

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/pelletier/go-toml/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
	"github.com/jorgerojas26/lazysql/models"
)

type aiCompletionFunc func(context.Context, string) (string, error)

func (complete aiCompletionFunc) Step(ctx context.Context, instructions string, messages []ai.Message, tools []ai.Tool) (ai.Turn, error) {
	if len(messages) == 1 {
		return ai.Turn{Calls: []ai.ToolCall{{ID: "list", Name: "list_tables", Arguments: json.RawMessage(`{}`)}}}, nil
	}
	last := messages[len(messages)-1]
	var failure struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(last.Text), &failure) == nil && failure.Error != "" {
		return ai.Turn{}, errors.New(failure.Error)
	}
	if last.Name == "list_tables" {
		var catalog struct {
			Tables []string `json:"tables"`
		}
		if err := json.Unmarshal([]byte(last.Text), &catalog); err != nil {
			return ai.Turn{}, err
		}
		arguments, _ := json.Marshal(map[string]any{"names": catalog.Tables})
		return ai.Turn{Calls: []ai.ToolCall{{ID: "describe", Name: "describe_tables", Arguments: arguments}}}, nil
	}
	var prompt strings.Builder
	prompt.WriteString(instructions)
	for _, message := range messages {
		prompt.WriteString(message.Text)
	}
	text, err := complete(ctx, prompt.String())
	return ai.Turn{Text: text}, err
}

func setAIConfigForTest(t *testing.T, enabled bool) {
	t.Helper()
	previous := app.App.AIConfig()
	directory := t.TempDir()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(directory); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "config.toml")
	write := func(config models.AIConfig) {
		t.Helper()
		data, err := toml.Marshal(struct {
			AI models.AIConfig `toml:"ai"`
		}{config})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if err := app.LoadConfig(path); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { write(previous); _ = os.Chdir(original) })
	write(models.AIConfig{Enabled: enabled, Provider: "chatgpt"})
}

func TestAIDisabledDoesNotInitializeUIOrInterceptShortcut(t *testing.T) {
	setAIConfigForTest(t, false)
	table := newRecordsFetchTestTable(&schemaProgrammingMock{})
	table.Wrapper = tview.NewFlex().SetDirection(tview.FlexRow)
	table.WithEditor()
	if table.aiSurface != nil {
		t.Fatal("disabled AI installed a UI surface or callback")
	}
	table.Editor.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModNone), func(tview.Primitive) {})
	table.openAIPrompt()
	if table.Home != nil && table.Home.aiChat != nil {
		t.Fatal("disabled AI intercepted a shortcut or opened a prompt")
	}
}

func TestAISelectionReplacementIsAtomicAndRejectsStaleWork(t *testing.T) {
	editor := NewSQLEditor("")
	original := "SELECT café;\nSELECT 2;"
	editor.SetText(original, false)
	editor.selecting = true
	editor.selCX = 0
	editor.selCY = 0
	editor.cx = len("SELECT café;")
	editor.cy = 0
	selection := captureAISelection(editor)
	before := len(editor.undoStack)
	if err := selection.apply(editor, "SELECT 42;"); err != nil {
		t.Fatal(err)
	}
	if editor.GetText() != "SELECT 42;\nSELECT 2;" || len(editor.undoStack) != before+1 {
		t.Fatal("replacement must be a single undoable edit")
	}
	editor.undo()
	if editor.GetText() != original {
		t.Fatal("undo did not restore original work")
	}
	editor.SetText("newer work", false)
	if err := selection.apply(editor, "bad overwrite"); err == nil || editor.GetText() != "newer work" {
		t.Fatal("stale selection overwrote newer work")
	}
}

func TestAISQLPromptAutomaticContextAndCancellation(t *testing.T) {
	setAIConfigForTest(t, true)
	previousApplication := App.Application
	previousPages := mainPages
	App.Application = tview.NewApplication()
	mainPages = tview.NewPages()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(90, 25)
	App.SetScreen(screen)
	driver := &aiSchemaDriver{}
	table := newRecordsFetchTestTable(driver)
	table.Wrapper = tview.NewFlex().SetDirection(tview.FlexRow)
	table.WithEditor()
	tree := &Tree{TreeView: tview.NewTreeView(), DBDriver: driver, state: &TreeState{selectedTable: "public.orders"}, Schemas: []string{"public"}}
	table.Home = &Home{Tree: tree, DBDriver: driver, TabbedPane: NewTabbedPane()}
	original := "SELECT 'private-literal' FROM public.orders;"
	table.Editor.SetText(original, true)
	table.Editor.selecting = true
	table.Editor.selCX = 0
	table.Editor.selCY = 0
	table.Home.Flex = tview.NewFlex().AddItem(table.Home.TabbedPane.Pages, 0, 1, true)
	table.Page = tview.NewPages().AddPage("table", table.Wrapper, true, true)
	table.Home.TabbedPane.AppendTab(tabNameEditor, table, tabNameEditor)
	mainPages.SetInputCapture(captureAIShortcut)
	mainPages.AddPage("editor", table.Home, true, true)
	done := make(chan error, 1)
	go func() { done <- App.Run(mainPages, filepath.Join(t.TempDir(), "runtime.toml")) }()
	App.QueueUpdate(func() {})
	t.Cleanup(func() {
		App.QueueUpdate(func() {
			if table.Home != nil && table.Home.aiChat != nil {
				table.Home.aiChat.close()
			}
		})
		App.Application.Stop()
		if err := <-done; err != nil {
			t.Error(err)
		}
		App.Application = previousApplication
		mainPages = previousPages
	})
	wait := func(condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			ready := false
			App.QueueUpdate(func() { ready = condition() })
			if ready {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("AI UI did not reach expected state")
	}
	var session *aiSession
	App.QueueUpdateDraw(func() {
		mainPages.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModNone), func(p tview.Primitive) { App.SetFocus(p) })
		session = table.Home.aiChat
	})
	if session == nil {
		t.Fatal("shortcut did not open prompt")
	}
	wait(func() bool { return !session.busy })
	App.QueueUpdateDraw(func() {
		mainPages.Draw(screen)
		x, y, width, height := session.panel.GetRect()
		rootX, rootY, rootWidth, rootHeight := mainPages.GetRect()
		if x != rootX+(rootWidth-width)/2 || y != rootY+(rootHeight-height)/2 || width != min(92, rootWidth) || height != min(4, rootHeight) {
			t.Errorf("prompt is not centered on screen: %d,%d %dx%d", x, y, width, height)
		}
		text := simulationText(screen)
		for _, removed := range []string{"Using ChatGPT", "Schema only", "automatic table selection", "related", "never executed automatically", "Include selected SQL", "Tables"} {
			if strings.Contains(text, removed) {
				t.Errorf("prompt still displays %q", removed)
			}
		}
	})
	prompts := make(chan string, 4)
	App.QueueUpdateDraw(func() {
		session.client = aiCompletionFunc(func(_ context.Context, prompt string) (string, error) {
			prompts <- prompt
			return `{"sql":"SELECT id FROM public.orders;","clarification":"","missing":"","warning":""}`, nil
		})
		session.input.SetText("List order identifiers")
		session.generate()
	})
	wait(func() bool { return !session.busy && session.result.SQL != "" })
	first := <-prompts
	if !strings.Contains(first, "private-literal") {
		t.Fatal("selected SQL was not included automatically")
	}
	if !strings.Contains(first, "public.customers") || !strings.Contains(first, "customer_fk") {
		t.Fatal("generation lacks related schema context")
	}
	App.QueueUpdateDraw(func() {
		if table.Editor.GetText() != original {
			t.Error("preview changed editor contents")
		}
		_, preview := table.aiSurface.GetFrontPage()
		preview.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModNone), func(p tview.Primitive) { App.SetFocus(p) })
		if table.Editor.GetText() != original {
			t.Error("preview accepted an execution shortcut")
		}
		preview.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModNone), func(p tview.Primitive) { App.SetFocus(p) })
		if session.visible || !session.active() {
			t.Error("chat did not hide persistently")
		}
		table.openAIPrompt()
		session.input.SetText("Include email addresses")
		session.result.SQL = ""
		session.generate()
	})
	wait(func() bool { return !session.busy && session.result.SQL != "" })
	if second := <-prompts; !strings.Contains(second, "private-literal") {
		t.Fatal("selected SQL was not included automatically")
	}
	App.QueueUpdateDraw(func() {
		_, preview := table.aiSurface.GetFrontPage()
		preview.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModNone), func(p tview.Primitive) { App.SetFocus(p) })
		if table.Home.aiChat != session || session.visible || table.Editor.GetText() != "SELECT id FROM public.orders;" {
			t.Error("accept did not replace selection and close preview")
		}
		table.Editor.undo()
		if table.Editor.GetText() != original {
			t.Error("accept was not undoable")
		}
		table.openAIPrompt()
		session = table.Home.aiChat
		session.newChat()
	})
	wait(func() bool { return !session.busy })
	started := make(chan struct{})
	canceled := make(chan struct{})
	App.QueueUpdateDraw(func() {
		session.client = aiCompletionFunc(func(ctx context.Context, _ string) (string, error) {
			close(started)
			<-ctx.Done()
			close(canceled)
			return "", ctx.Err()
		})
		session.input.SetText("Keep this prompt")
		session.generate()
	})
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("generation never started")
	}
	App.QueueUpdateDraw(func() { session.capture(tcell.NewEventKey(tcell.KeyCtrlX, 0, tcell.ModNone)) })
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("generation did not cancel")
	}
	App.QueueUpdateDraw(func() {
		if session.busy || session.input.GetText() != "Keep this prompt" || table.Editor.GetText() != original {
			t.Error("cancellation lost prompt or existing SQL")
		}
		session.capture(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone))
		if session.visible || table.Home.aiChat != session {
			t.Error("Escape did not hide chat persistently")
		}
	})
	var generated *ResultsTable
	App.QueueUpdateDraw(func() {
		tree.state.selectedTable = ""
		table.Editor.selecting = false
		table.openAIPrompt()
		session = table.Home.aiChat
		session.newChat()
	})
	wait(func() bool { return !session.busy })
	App.QueueUpdateDraw(func() {
		session.client = aiCompletionFunc(func(_ context.Context, prompt string) (string, error) {
			prompts <- prompt
			return `{"sql":"SELECT id FROM public.orders;","clarification":"","missing":"","warning":"Review this query"}`, nil
		})
		session.input.SetText("List order identifiers")
		session.generate()
	})
	wait(func() bool { return !session.busy && session.result.SQL != "" })
	if prompt := <-prompts; strings.Contains(prompt, "private.secrets") || !strings.Contains(prompt, "public.items") || !strings.Contains(prompt, "editor_sql") {
		t.Fatal("automatic context leaked a filtered schema or omitted visible tables")
	}
	App.QueueUpdateDraw(func() {
		if err := session.accept(); err != nil {
			t.Error(err)
			return
		}
		generated = table.Home.TabbedPane.GetCurrentTab().Content.(*ResultsTable)
		if generated == table || generated.Editor.GetText() != "SELECT id FROM public.orders;" || table.Editor.GetText() != original {
			t.Error("accept without selection must open a new tab and preserve existing SQL")
		}
	})
	if generated == nil {
		t.Fatal("new SQL tab not created")
	}
	wait(func() bool {
		return len(generated.Editor.completer.tables) > 0 && generated.Editor.completer.HasColumns("public.orders") && generated.Editor.completer.HasColumns("public.customers") && generated.Editor.completer.HasColumns("public.items")
	})
	if driver.executions.Load() != 0 {
		t.Fatal("AI generation or acceptance executed SQL")
	}

}

type aiRetryCatalogDriver struct {
	aiSchemaDriver
	catalogCalls atomic.Int32
}

func (driver *aiRetryCatalogDriver) GetTables(ctx context.Context, database string) (map[string][]string, error) {
	if driver.catalogCalls.Add(1) == 1 {
		return nil, errors.New("private database connection failure")
	}
	return driver.aiSchemaDriver.GetTables(ctx, database)
}

func TestAIGenerationRetriesCatalogFailureWithoutReopeningPrompt(t *testing.T) {
	previousApplication := App.Application
	App.Application = tview.NewApplication()
	session := newAILayoutSession(t)
	driver := &aiRetryCatalogDriver{}
	session.table.DBDriver = driver
	session.table.schemaLoader = nil
	session.schemas = []string{"public"}
	session.client = aiCompletionFunc(func(context.Context, string) (string, error) {
		return `{"sql":"SELECT id FROM public.orders;","clarification":"","missing":"","warning":""}`, nil
	})
	session.showPanel(session.panel, 24)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	App.SetScreen(screen)
	done := make(chan error, 1)
	go func() { done <- App.Run(mainPages, filepath.Join(t.TempDir(), "runtime.toml")) }()
	App.QueueUpdate(func() {})
	t.Cleanup(func() {
		App.QueueUpdate(func() {
			if session.active() {
				session.close()
			}
		})
		App.Application.Stop()
		if err := <-done; err != nil {
			t.Error(err)
		}
		App.Application = previousApplication
	})
	wait := func(condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			ready := false
			App.QueueUpdate(func() { ready = condition() })
			if ready {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("AI generation did not finish")
	}
	App.QueueUpdateDraw(func() { session.input.SetText("Last 10 registered users"); session.generate() })
	wait(func() bool { return !session.busy })
	App.QueueUpdate(func() {
		status := session.transcript.GetText(true)
		if !strings.Contains(status, "could not load schema metadata") || strings.Contains(status, "private") {
			t.Errorf("original sanitized error was lost: %q", status)
		}
		if session.input.GetText() != "Last 10 registered users" {
			t.Error("catalog failure lost or disabled the prompt")
		}
		if session.result.SQL != "" {
			t.Error("catalog failure generated ungrounded SQL")
		}
		session.generate()
	})
	wait(func() bool { return !session.busy && session.result.SQL != "" })
	App.QueueUpdate(func() {
		if driver.catalogCalls.Load() != 2 {
			t.Errorf("catalog was not retried: %d calls", driver.catalogCalls.Load())
		}
		if session.table.Home.aiChat != session || len(session.history) != 4 || session.history[0].Text != "Last 10 registered users" || session.input.GetText() != "" {
			t.Error("retry replaced the session or prompt")
		}
		if driver.executions.Load() != 0 {
			t.Error("retry executed SQL")
		}
	})
}

func TestAIAcceptUsesDiscoveredDatabaseWithoutChangingSourceDraft(t *testing.T) {
	previousApplication := App.Application
	App.Application = tview.NewApplication()
	session := newAILayoutSession(t)
	driver := &aiMultiDatabaseDriver{}
	tree := &Tree{TreeView: tview.NewTreeView(), DBDriver: driver, state: &TreeState{selectedDatabase: "acl"}}
	home := &Home{Tree: tree, TabbedPane: NewTabbedPane(), DBDriver: driver}
	session.table.Home = home
	session.owner = home
	home.aiChat = session
	source := session.table
	session.table.DBDriver = driver
	session.table.schemaLoader = nil
	session.table.SetDatabaseName("acl")
	session.database = "acl"
	session.selection = captureAISelection(session.table.Editor)
	session.client = aiDiscoveryFunc(func(_ context.Context, _ string, messages []ai.Message, _ []ai.Tool) (ai.Turn, error) {
		if len(messages) == 1 {
			return ai.Turn{Calls: []ai.ToolCall{{ID: "users", Name: "describe_tables", Arguments: json.RawMessage(`{"database":"app","names":["Users"]}`)}}}, nil
		}
		return ai.Turn{Text: `{"database":"app","sql":"SELECT id, email FROM Users ORDER BY created_at DESC LIMIT 10;","clarification":"","missing":"","warning":""}`}, nil
	})
	session.showPanel(session.panel, 24)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	App.SetScreen(screen)
	done := make(chan error, 1)
	go func() { done <- App.Run(mainPages, filepath.Join(t.TempDir(), "runtime.toml")) }()
	App.QueueUpdate(func() {})
	t.Cleanup(func() {
		App.QueueUpdate(func() {
			if session.active() {
				session.close()
			}
		})
		App.Application.Stop()
		if err := <-done; err != nil {
			t.Error(err)
		}
		App.Application = previousApplication
	})
	wait := func(condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			ready := false
			App.QueueUpdate(func() { ready = condition() })
			if ready {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("AI did not finish cross-database generation")
	}
	App.QueueUpdateDraw(func() { session.generate() })
	wait(func() bool { return !session.busy && session.result.SQL != "" })
	var generated *ResultsTable
	App.QueueUpdateDraw(func() {
		if err := session.accept(); err != nil {
			t.Error(err)
			return
		}
		generated = home.TabbedPane.GetCurrentTab().Content.(*ResultsTable)
		if generated.GetDatabaseName() != "app" || generated.Editor.GetText() != session.result.SQL {
			t.Error("generated SQL was not routed to discovered database")
		}
		if source.GetDatabaseName() != "acl" || source.Editor.GetText() != "existing work" {
			t.Error("source database or draft was changed")
		}
	})
	if generated != nil {
		wait(func() bool { return generated.Editor.completer.HasColumns("Users") })
	}
	if driver.executions.Load() != 0 {
		t.Fatal("generated SQL executed automatically")
	}
}
