package components

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func TestAIFreshChatShowsOnlyCenteredInput(t *testing.T) {
	session := newAILayoutSession(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, size := range [][2]int{{120, 40}, {80, 24}, {48, 14}, {30, 6}} {
		width, height := size[0], size[1]
		screen.SetSize(width, height)
		screen.Clear()
		mainPages.SetRect(0, 0, width, height)
		mainPages.Draw(screen)
		x, y, w, h := session.panel.GetRect()
		if w != min(92, width) || h != min(4, height) || x != (width-w)/2 || y != (height-h)/2 {
			t.Fatalf("fresh chat is not compact and centered at %v: %d,%d %dx%d", size, x, y, w, h)
		}
		text := simulationText(screen)
		if !strings.Contains(text, "Message") || !strings.Contains(text, "My draft request") {
			t.Fatalf("compact composer is not visible:\n%s", text)
		}
		for _, hidden := range []string{"AI SQL", "Ctrl+N", "Ctrl+K", "tokens", "measuring"} {
			if strings.Contains(text, hidden) {
				t.Fatalf("fresh chat contains %q:\n%s", hidden, text)
			}
		}
		if len(session.focusable) != 1 || session.focusable[0] != session.input {
			t.Fatal("fresh chat allows focusing hidden controls")
		}
		session.capture(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
		if app.App.GetFocus() != session.input {
			t.Fatal("Tab moved focus outside the compact input")
		}
	}
}

func TestAIFreshChatExpandsAndNewChatCollapses(t *testing.T) {
	session := newAILayoutSession(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 40)
	mainPages.SetRect(0, 0, 100, 40)
	draw := func() {
		screen.Clear()
		mainPages.Draw(screen)
	}
	draw()
	session.hide()
	session.show()
	draw()
	if !session.minimalChat() || session.input.GetText() != "My draft request" {
		t.Fatal("reopening an unsent draft expanded or lost the draft")
	}
	session.toggleContext()
	draw()
	_, _, _, height := session.panel.GetRect()
	if height != 26 || !strings.Contains(simulationText(screen), "Conversation") {
		t.Fatal("context is not available from a fresh chat")
	}
	session.toggleContext()
	draw()
	_, _, _, height = session.panel.GetRect()
	if height != 4 || session.input.GetText() != "My draft request" {
		t.Fatal("leaving context did not restore the compact draft")
	}
	session.appendMessage(ai.ChatMessage{Role: "user", Text: "A sent request"})
	session.busy = true
	session.refreshControls()
	draw()
	_, _, _, height = session.panel.GetRect()
	if height != 26 || !strings.Contains(simulationText(screen), "A sent request") || !strings.Contains(simulationText(screen), "Ctrl+X Stop") {
		t.Fatal("sending did not expand the full conversation layout")
	}
	session.busy = false
	session.newChat()
	draw()
	_, _, _, height = session.panel.GetRect()
	if height != 4 || !strings.Contains(simulationText(screen), "Ask for a query") || strings.Contains(simulationText(screen), "Ctrl+N") {
		t.Fatal("New chat did not return to an input-only modal")
	}
}
