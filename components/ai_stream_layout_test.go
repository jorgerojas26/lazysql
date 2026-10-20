package components

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/jorgerojas26/lazysql/internal/ai"
)

func TestAIStreamFlowsDirectlyAfterPrompt(t *testing.T) {
	session := newAILayoutSession(t)
	session.appendMessage(ai.ChatMessage{Role: "user", Text: "Show the latest resources"})
	session.streaming = ai.Result{SQL: "SELECT id FROM resources;"}
	session.renderChat()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, width := range []int{120, 80, 48} {
		screen.SetSize(width, 30)
		screen.Clear()
		mainPages.SetRect(0, 0, width, 30)
		mainPages.Draw(screen)
		_, promptY, promptFound := findAIChatText(screen, "Show the latest resources")
		_, blockY, blockFound := findAIChatText(screen, "╭─ SQL")
		if !promptFound || !blockFound || blockY != promptY+3 {
			t.Fatalf("stream separated from prompt at width %d:\n%s", width, simulationText(screen))
		}
		if !strings.Contains(simulationText(screen), "SELECT id FROM resources;") {
			t.Fatal("short streamed SQL was clipped")
		}
	}
}

func TestAIStreamUsesFullChatViewportAndPreservesScrolling(t *testing.T) {
	session := newAILayoutSession(t)
	session.appendMessage(ai.ChatMessage{Role: "user", Text: "Show the latest resources"})
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)
	mainPages.SetRect(0, 0, 100, 30)
	draw := func() {
		screen.Clear()
		mainPages.Draw(screen)
	}
	draw()
	for line := 1; line <= 30; line++ {
		session.streaming.SQL += fmt.Sprintf("SELECT %02d;\n", line)
		session.renderChat()
		draw()
		if !strings.Contains(simulationText(screen), fmt.Sprintf("SELECT %02d;", line)) {
			t.Fatalf("stream stopped following new SQL at line %d", line)
		}
	}
	x, y, _, height := session.transcript.GetInnerRect()
	character, _, _, _ := screen.GetContent(x, y)
	if character != '│' || height < 10 {
		t.Fatalf("stream is not using the full chat viewport:\n%s", simulationText(screen))
	}
	if !strings.Contains(simulationText(screen), "SELECT 20;") {
		t.Fatalf("stream clipped SQL despite available chat space:\n%s", simulationText(screen))
	}
	session.transcript.ScrollToBeginning()
	draw()
	session.streaming.SQL += "SELECT 31;"
	session.renderChat()
	draw()
	if !strings.Contains(simulationText(screen), "Show the latest resources") || strings.Contains(simulationText(screen), "SELECT 31;") {
		t.Fatal("stream update interrupted reading earlier messages")
	}
	session.transcript.ScrollToEnd()
	draw()
	if !strings.Contains(simulationText(screen), "SELECT 31;") {
		t.Fatal("cannot scroll to live SQL in the conversation")
	}
	before := simulationText(screen)
	result := session.streaming
	session.streaming = ai.Result{}
	session.appendMessage(ai.ChatMessage{Role: "assistant", SQL: result.SQL})
	draw()
	if after := simulationText(screen); after != before {
		t.Fatalf("finishing the stream changed the chat layout:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}
