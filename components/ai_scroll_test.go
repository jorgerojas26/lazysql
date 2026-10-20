package components

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func TestAIChatHalfPageScrollPreservesDraftAndFocus(t *testing.T) {
	session := newAILayoutSession(t)
	session.appendMessage(ai.ChatMessage{Role: "assistant", Text: strings.Repeat("A line of conversation\n", 80)})
	session.result = ai.Result{SQL: "SELECT 1;", Database: session.database}
	session.refreshControls()
	session.focus(session.input)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 40)
	mainPages.SetRect(0, 0, 100, 40)
	draw := func() { screen.Clear(); mainPages.Draw(screen) }
	press := func(key tcell.Key) {
		session.panel.InputHandler()(tcell.NewEventKey(key, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
		draw()
	}
	draw()
	bottom, _ := session.transcript.GetScrollOffset()
	_, _, _, height := session.transcript.GetInnerRect()
	step := max(1, height/2)
	press(tcell.KeyCtrlU)
	row, _ := session.transcript.GetScrollOffset()
	if row != bottom-step {
		t.Fatalf("Ctrl+U scrolled to %d, want %d", row, bottom-step)
	}
	if session.input.GetText() != "My draft request" || app.App.GetFocus() != session.input || !session.visible || len(session.history) != 1 || session.table.Editor.GetText() != "existing work" {
		t.Fatal("scrolling changed the draft/focus or attempted to use the query")
	}
	press(tcell.KeyCtrlD)
	row, _ = session.transcript.GetScrollOffset()
	if row != bottom {
		t.Fatalf("Ctrl+D scrolled to %d, want %d", row, bottom)
	}
	session.focus(session.transcript)
	press(tcell.KeyCtrlU)
	row, _ = session.transcript.GetScrollOffset()
	if row != bottom-step || app.App.GetFocus() != session.transcript {
		t.Fatal("scroll shortcuts do not work when the transcript is focused")
	}
	session.busy = true
	session.streaming = ai.Result{Message: "A streaming response"}
	session.renderChat()
	draw()
	if updated, _ := session.transcript.GetScrollOffset(); updated != row {
		t.Fatal("stream updates interrupted Ctrl+U scrollback")
	}
	for range 30 {
		press(tcell.KeyCtrlU)
	}
	if row, _ := session.transcript.GetScrollOffset(); row != 0 {
		t.Fatal("Ctrl+U did not stop at the beginning")
	}
	for range 30 {
		press(tcell.KeyCtrlD)
	}
	if !strings.Contains(simulationText(screen), "A streaming response") {
		t.Fatal("Ctrl+D did not reach the live response")
	}
	session.streaming.Message += " with new text"
	session.renderChat()
	draw()
	if !strings.Contains(simulationText(screen), "with new text") {
		t.Fatal("scrolling to the bottom did not resume following the response")
	}
}

func TestAIHalfPageScrollTargetsContextAndIgnoresFreshComposer(t *testing.T) {
	session := newAILayoutSession(t)
	for _, key := range []tcell.Key{tcell.KeyCtrlU, tcell.KeyCtrlD} {
		session.capture(tcell.NewEventKey(key, 0, tcell.ModNone))
	}
	if session.input.GetText() != "My draft request" || !session.minimalChat() || app.App.GetFocus() != session.input {
		t.Fatal("scrolling a fresh chat altered the input-only composer")
	}
	session.requestBody = strings.Repeat("A line of request context\n", 80)
	session.refreshRequest()
	session.toggleContext()
	session.switchContextTab("conversation")
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 40)
	mainPages.SetRect(0, 0, 100, 40)
	mainPages.Draw(screen)
	_, _, _, height := session.requestDetails.GetInnerRect()
	session.capture(tcell.NewEventKey(tcell.KeyCtrlD, 0, tcell.ModNone))
	mainPages.Draw(screen)
	if row, _ := session.requestDetails.GetScrollOffset(); row != max(1, height/2) {
		t.Fatal("Ctrl+D did not scroll the visible context tab")
	}
	if app.App.GetFocus() != session.requestDetails {
		t.Fatal("scrolling context changed focus")
	}
	session.capture(tcell.NewEventKey(tcell.KeyCtrlU, 0, tcell.ModNone))
	mainPages.Draw(screen)
	if row, _ := session.requestDetails.GetScrollOffset(); row != 0 {
		t.Fatal("Ctrl+U did not scroll context back to the beginning")
	}
}
