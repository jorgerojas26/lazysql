package components

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/internal/ai"
)

func TestAIStreamUpdatesDoNotBlockNetworkReader(t *testing.T) {
	session := newAILayoutSession(t)
	started := time.Now()
	for i := 0; i < 1000; i++ {
		session.stream(1, ai.Result{SQL: "SELECT pending;"})
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("stream reader blocked on UI: %s", elapsed)
	}
	session.newChat()
	if len(session.history) != 0 || session.streaming.SQL != "" {
		t.Fatal("queued stream update entered cleared conversation")
	}
}

func TestAIRequestOmitsDuplicatedEditorSQL(t *testing.T) {
	session := newAILayoutSession(t)
	query := "SELECT id FROM users;"
	session.table.Editor.SetText(query, false)
	session.selection = aiSelection{original: query, text: query, start: 0, end: len(query)}
	session.history = []ai.ChatMessage{{Role: "user", Text: "Users"}, {Role: "assistant", SQL: query, Database: session.database}}
	request := session.makeRequest("Include email")
	if request.EditorSQL != "" || request.SelectedSQL != "" || request.History[1].SQL != query {
		t.Fatal("request duplicated the same query across history, selection and editor")
	}
	session.selection.text = "id"
	request = session.makeRequest("Change this selection")
	if request.SelectedSQL != "id" {
		t.Fatal("deduplication lost an actual partial selection")
	}
}

func TestAICompletedHighlightingIsCachedAcrossStreamUpdates(t *testing.T) {
	session := newAILayoutSession(t)
	session.appendMessage(ai.ChatMessage{Role: "assistant", SQL: "SELECT 1;"})
	cached := session.cachedHistory
	version := session.renderedVersion
	session.streaming = ai.Result{SQL: "SELECT partial;"}
	session.renderChat()
	if session.cachedHistory != cached || session.renderedVersion != version || session.sqlBlocks["SELECT 1;"] == "" {
		t.Fatal("completed query cache was not reused")
	}
	if !strings.Contains(session.transcript.GetText(true), "SELECT partial;") || !strings.Contains(session.transcript.GetText(true), "SELECT 1;") {
		t.Fatal("streaming reply did not flow with completed conversation")
	}
}

func BenchmarkAIChatStreamingFrame(b *testing.B) {
	for _, count := range []int{2, 32} {
		name, lines := "small", 3
		if count > 2 {
			name, lines = "long", 30
		}
		b.Run(name, func(b *testing.B) {
			transcript := tview.NewTextView().SetWrap(true).SetWordWrap(true).SetScrollable(true).SetDynamicColors(true).SetRegions(true)
			session := &aiSession{ctx: context.Background(), chatWidth: 86, transcript: transcript, streaming: ai.Result{SQL: "SELECT id, email FROM users ORDER BY created_at DESC LIMIT 10;"}}
			for i := 0; i < count; i++ {
				session.history = append(session.history, ai.ChatMessage{Role: "assistant", Database: "app", SQL: strings.Repeat("SELECT id, email FROM users WHERE verified = 1;\n", lines)})
			}
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				b.Fatal(err)
			}
			defer screen.Fini()
			screen.SetSize(86, 18)
			transcript.SetRect(0, 0, 86, 18)
			session.renderChat()
			transcript.Draw(screen)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				session.renderChat()
				transcript.Draw(screen)
			}
		})
	}
}
