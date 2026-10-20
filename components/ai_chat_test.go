package components

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/internal/ai"
)

func attachAIChatHome(session *aiSession) *Home {
	tree := &Tree{TreeView: tview.NewTreeView().SetRoot(tview.NewTreeNode("database")), DBDriver: session.table.DBDriver, state: &TreeState{selectedDatabase: session.database}}
	home := &Home{Flex: tview.NewFlex(), Tree: tree, TabbedPane: NewTabbedPane(), DBDriver: session.table.DBDriver, aiChat: session}
	home.Flex.AddItem(home.TabbedPane.Pages, 0, 1, true)
	session.owner = home
	session.table.Home = home
	session.table.Wrapper = tview.NewFlex().AddItem(session.table.Editor, 0, 1, true)
	session.table.Page = tview.NewPages().AddPage("editor", session.table.Wrapper, true, true)
	home.TabbedPane.AppendTab(tabNameEditor, session.table, tabNameEditor)
	mainPages.AddPage("home", home, true, true)
	session.show()
	return home
}

func TestAIChatSurvivesTabChangesAndSourceTabClosure(t *testing.T) {
	setAIConfigForTest(t, true)
	session := newAILayoutSession(t)
	home := attachAIChatHome(session)
	session.appendMessage(ai.ChatMessage{Role: "user", Text: "Last ten users"})
	session.appendMessage(ai.ChatMessage{Role: "assistant", SQL: "SELECT id FROM Users LIMIT 10;"})
	session.input.SetText("Include email addresses")
	session.hide()
	other := newRecordsFetchTestTable(home.DBDriver)
	other.Page = tview.NewPages().AddPage("other", tview.NewBox(), true, true)
	home.TabbedPane.AppendTab("other", other, "other")
	home.openAIPrompt()
	if home.aiChat != session || !session.visible || len(session.history) != 2 || session.input.GetText() != "Include email addresses" {
		t.Fatal("tab change discarded chat or composer")
	}
	session.hide()
	home.TabbedPane.SwitchToTabByReference(tabNameEditor)
	home.TabbedPane.RemoveCurrentTab()
	if !session.active() || !session.sourceClosed || home.aiChat != session {
		t.Fatal("closing source SQL tab destroyed connection chat")
	}
	home.openAIPrompt()
	if !session.visible || len(session.history) != 2 {
		t.Fatal("chat did not reopen after source tab closed")
	}
	session.close()
	if home.aiChat != nil || session.active() {
		t.Fatal("closing connection chat did not release history owner")
	}
}

func TestAIChatRefinementMnemonicsAndClearRejectLateReplies(t *testing.T) {
	previousApplication := App.Application
	App.Application = tview.NewApplication()
	session := newAILayoutSession(t)
	attachAIChatHome(session)
	session.client = aiDiscoveryFunc(func(_ context.Context, _ string, messages []ai.Message, _ []ai.Tool) (ai.Turn, error) {
		var request ai.Request
		if err := json.Unmarshal([]byte(messages[0].Text), &request); err != nil {
			return ai.Turn{}, err
		}
		sql := "SELECT id FROM Users ORDER BY created_at DESC LIMIT 10;"
		if request.Prompt == "Fresh query" && len(request.History) != 0 {
			t.Error("New chat retained model conversation context")
		}
		if strings.Contains(request.Prompt, "email") {
			if len(request.History) != 2 || request.History[1].SQL != sql {
				t.Error("refinement lost previous SQL")
			}
			sql = "SELECT id, email FROM Users ORDER BY created_at DESC LIMIT 10;"
		}
		output, _ := json.Marshal(ai.Result{SQL: sql, Database: request.Database})
		return ai.Turn{Text: string(output)}, nil
	})
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	App.SetScreen(screen)
	done := make(chan error, 1)
	go func() { done <- App.Run(mainPages, filepath.Join(t.TempDir(), "runtime.toml")) }()
	App.QueueUpdate(func() {})
	t.Cleanup(func() {
		App.QueueUpdate(func() { session.close() })
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
		t.Fatal("chat did not reach expected state")
	}
	send := func(text string) {
		App.QueueUpdateDraw(func() {
			session.input.SetText(text)
			session.capture(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone))
		})
	}
	send("Last ten users")
	wait(func() bool { return !session.busy && len(session.history) == 2 })
	send("Include email addresses")
	wait(func() bool { return !session.busy && len(session.history) == 4 })
	sentinel := tview.NewBox()
	var backgroundRun uint64
	App.QueueUpdate(func() {
		if !strings.Contains(session.result.SQL, "email") || session.input.GetText() != "" {
			t.Error("refined query or composer is incorrect")
		}
		session.busy = true
		session.input.SetDisabled(true)
		backgroundRun = session.run
		session.capture(tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModNone))
		if session.visible || !session.active() {
			t.Error("Hide destroyed the session")
		}
		App.SetFocus(sentinel)
	})
	backgroundDone := make(chan struct{})
	go func() {
		session.finish(backgroundRun, ai.Result{Message: "Background answer"}, nil)
		close(backgroundDone)
	}()
	<-backgroundDone
	App.QueueUpdate(func() {
		if session.visible || App.GetFocus() != sentinel || len(session.history) != 5 {
			t.Error("background answer reopened chat or stole focus")
		}
		session.show()
		session.busy = true
		session.input.SetDisabled(true)
	})
	var oldRun uint64
	canceled := false
	App.QueueUpdate(func() {
		oldRun = session.run
		session.runCancel = func() { canceled = true }
		session.capture(tcell.NewEventKey(tcell.KeyCtrlN, 0, tcell.ModNone))
		if !canceled || session.busy || len(session.history) != 0 || session.result.SQL != "" || session.input.GetText() != "" {
			t.Error("New chat failed to clear or cancel session")
		}
	})
	lateDone := make(chan struct{})
	go func() { session.finish(oldRun, ai.Result{SQL: "SELECT 'late';"}, nil); close(lateDone) }()
	<-lateDone
	App.QueueUpdate(func() {
		if len(session.history) != 0 || session.result.SQL != "" {
			t.Error("late response repopulated cleared chat")
		}
		if session.table.DBDriver.(*aiSchemaDriver).executions.Load() != 0 {
			t.Error("chat executed SQL")
		}
	})
	send("Fresh query")
	wait(func() bool { return !session.busy && len(session.history) == 2 })
	source := session.table
	var generated *ResultsTable
	var tabCount int
	App.QueueUpdateDraw(func() {
		session.capture(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModNone))
		generated = session.table
		tabCount = session.owner.TabbedPane.GetLength()
		if generated == source || source.Editor.GetText() != "existing work" || session.visible || !session.active() {
			t.Error("Use query lost chat or overwrote source draft")
		}
		session.show()
	})
	wait(func() bool { return len(generated.Editor.completer.tables) > 0 })
	send("Include email addresses")
	wait(func() bool { return !session.busy && len(session.history) == 4 })
	App.QueueUpdateDraw(func() {
		session.capture(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModNone))
		if session.table != generated || session.owner.TabbedPane.GetLength() != tabCount || !strings.Contains(generated.Editor.GetText(), "email") {
			t.Error("refined SQL was not applied to its existing editor")
		}
		if source.Editor.GetText() != "existing work" || !session.active() {
			t.Error("refinement damaged draft or chat state")
		}
	})
}

func TestAIChatSessionsAreIsolatedPerConnection(t *testing.T) {
	first := newAILayoutSession(t)
	firstHome := attachAIChatHome(first)
	first.appendMessage(ai.ChatMessage{Role: "user", Text: "First connection private prompt"})
	first.hide()
	secondTable := newRecordsFetchTestTable(&aiSchemaDriver{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	second := &aiSession{table: secondTable, surface: tview.NewPages(), ctx: ctx, cancel: cancel, database: "other"}
	secondTable.Editor = NewSQLEditor("")
	secondTable.aiSession = second
	second.buildUI()
	secondHome := attachAIChatHome(second)
	second.appendMessage(ai.ChatMessage{Role: "user", Text: "Second connection prompt"})
	if firstHome.aiChat == secondHome.aiChat || strings.Contains(second.transcript.GetText(true), "First connection") {
		t.Fatal("conversation leaked across connections")
	}
	second.newChat()
	if len(first.history) != 1 || len(second.history) != 0 {
		t.Fatal("New chat cleared another connection")
	}
	second.close()
	if !first.active() {
		t.Fatal("closing another chat destroyed first connection history")
	}
	first.close()
}
