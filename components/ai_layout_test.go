package components

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func newAILayoutSession(t *testing.T) *aiSession {
	t.Helper()
	previousPages := mainPages
	mainPages = tview.NewPages()
	t.Cleanup(func() { mainPages = previousPages })
	previous := app.App.GetFocus()
	t.Cleanup(func() { app.App.SetFocus(previous) })
	table := newRecordsFetchTestTable(&aiSchemaDriver{})
	table.Editor = NewSQLEditor("")
	table.Editor.SetText("existing work", false)
	table.aiSurface = tview.NewPages()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	session := &aiSession{table: table, surface: table.aiSurface, ctx: ctx, cancel: cancel, database: "database"}
	table.aiSession = session
	session.buildUI()
	session.input.SetText("My draft request")
	session.show()
	return session
}

func TestAIChatLayoutAndScroll(t *testing.T) {
	session := newAILayoutSession(t)
	session.appendMessage(ai.ChatMessage{Role: "user", Text: "Last ten users"})
	session.result = ai.Result{Database: "app", SQL: "SELECT id, email FROM Users\nORDER BY created_at DESC\nLIMIT 10;"}
	session.showPreview()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, width := range []int{120, 80, 48} {
		screen.SetSize(width, 28)
		screen.Clear()
		mainPages.SetRect(0, 0, width, 28)
		mainPages.Draw(screen)
		text := simulationText(screen)
		for _, expected := range []string{"AI SQL", "You", "Last ten users", "AI · app", "SELECT", "Ctrl+U Use query", "Ctrl+N New chat"} {
			if !strings.Contains(text, expected) {
				t.Fatalf("missing %q at width %d:\n%s", expected, width, text)
			}
		}
	}
	session.showResponse("Error", strings.Repeat("A readable diagnostic.\n", 50)+"LAST DETAIL", true)
	session.focus(session.transcript)
	screen.Clear()
	mainPages.Draw(screen)
	if !strings.Contains(simulationText(screen), "LAST DETAIL") {
		t.Fatal("long assistant response is not visible at end of chat")
	}
	mainPages.InputHandler()(tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	screen.Clear()
	mainPages.Draw(screen)
	if !strings.Contains(simulationText(screen), "Last ten users") {
		t.Fatal("older chat messages are not scrollable")
	}
}

func TestAIChatCenteredAndResponsive(t *testing.T) {
	session := newAILayoutSession(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, size := range [][2]int{{120, 40}, {80, 24}, {48, 14}, {30, 6}} {
		width, height := size[0], size[1]
		screen.SetSize(width, height)
		mainPages.SetRect(0, 0, width, height)
		mainPages.Draw(screen)
		x, y, w, h := session.panel.GetRect()
		if w != min(92, width) || h != min(26, height) || x != (width-w)/2 || y != (height-h)/2 {
			t.Fatalf("chat not centered at %v: %d,%d %dx%d", size, x, y, w, h)
		}
	}
}

func TestAIChatMnemonicsPreserveAndClearSession(t *testing.T) {
	session := newAILayoutSession(t)
	session.appendMessage(ai.ChatMessage{Role: "user", Text: "[red]literal markup"})
	session.result = ai.Result{Database: "app", SQL: "SELECT 1;"}
	session.showPreview()
	key := func(code tcell.Key) {
		session.panel.InputHandler()(tcell.NewEventKey(code, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	}
	key(tcell.KeyCtrlG)
	if session.visible || !session.active() || len(session.history) != 2 || session.input.GetText() != "My draft request" {
		t.Fatal("hiding discarded chat or composer")
	}
	session.show()
	if !session.visible || app.App.GetFocus() != session.input {
		t.Fatal("chat did not reopen at composer")
	}
	key(tcell.KeyCtrlN)
	if len(session.history) != 0 || session.result.SQL != "" || session.input.GetText() != "" || session.question != "" {
		t.Fatal("New chat did not clear conversation")
	}
	session.input.SetText("Line one")
	session.panel.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModCtrl), func(p tview.Primitive) { app.App.SetFocus(p) })
	session.panel.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if session.input.GetText() != "Line one\nx" {
		t.Fatalf("multiline composer failed: %q", session.input.GetText())
	}
}
func TestAIReadOnlyEditorDoesNotPublishExecution(t *testing.T) {
	editor := NewSQLEditor("")
	editor.readOnly = true
	editor.SetText("SELECT 1;", false)
	changes := editor.Subscribe()
	for _, key := range []*tcell.EventKey{tcell.NewEventKey(tcell.KeyCtrlR, 0, tcell.ModNone), tcell.NewEventKey(tcell.KeyCtrlSpace, 0, tcell.ModNone), tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)} {
		editor.InputHandler()(key, func(tview.Primitive) {})
	}
	select {
	case <-changes:
		t.Fatal("read-only preview published a query")
	default:
	}
	if editor.GetText() != "SELECT 1;" {
		t.Fatal("read-only preview modified SQL")
	}
}

func TestAIChatOverlayPreservesWorkspaceBackdrop(t *testing.T) {
	session := newAILayoutSession(t)
	session.hide()
	backdrop := tview.NewTextView().SetText(strings.Repeat(strings.Repeat("W", 120)+"\n", 38))
	backdrop.SetTextColor(tcell.ColorGreen).SetBackgroundColor(tcell.ColorNavy)
	backdropClicks := 0
	backdrop.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		backdropClicks++
		return action, event
	})
	mainPages.AddPage("workspace", backdrop, true, true)
	app.App.SetFocus(backdrop)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(120, 38)
	mainPages.SetRect(0, 0, 120, 38)
	mainPages.Draw(screen)
	type cell struct {
		character rune
		style     tcell.Style
	}
	original := make([]cell, 120*38)
	for y := 0; y < 38; y++ {
		for x := 0; x < 120; x++ {
			character, _, style, _ := screen.GetContent(x, y)
			original[y*120+x] = cell{character, style}
		}
	}
	session.show()
	mainPages.Draw(screen)
	px, py, width, height := session.panel.GetRect()
	for y := 0; y < 38; y++ {
		for x := 0; x < 120; x++ {
			if x >= px && x < px+width && y >= py && y < py+height {
				continue
			}
			character, _, style, _ := screen.GetContent(x, y)
			if original[y*120+x] != (cell{character, style}) {
				t.Fatalf("chat erased workspace outside modal at %d,%d", x, y)
			}
		}
	}
	character, _, style, _ := screen.GetContent(px+1, py+1)
	_, background, _ := style.Decompose()
	if character != ' ' || background != app.Styles.PrimitiveBackgroundColor {
		t.Fatal("modal interior did not cover underlying workspace")
	}
	mainPages.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(0, 0, tcell.ButtonNone, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if backdropClicks != 0 || app.App.GetFocus() != session.input || !session.visible {
		t.Fatal("outside click passed through modal or stole focus")
	}
	text := simulationText(screen)
	if strings.Contains(text, "Enter Send") || strings.Contains(text, "Ctrl+H") {
		t.Fatal("removed controls are still visible")
	}
	session.hide()
	mainPages.Draw(screen)
	for y := 0; y < 38; y++ {
		for x := 0; x < 120; x++ {
			character, _, style, _ := screen.GetContent(x, y)
			if original[y*120+x] != (cell{character, style}) {
				t.Fatalf("workspace was not restored after hiding at %d,%d", x, y)
			}
		}
	}
}

func TestAIChatCtrlHDoesNotHideAndToggleStillWorks(t *testing.T) {
	session := newAILayoutSession(t)
	session.input.SetText("draftx")
	session.panel.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlH, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if !session.visible || session.input.GetText() != "draft" {
		t.Fatal("Ctrl+H still hides chat instead of normal backspace behavior")
	}
	session.capture(tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModNone))
	if session.visible || !session.active() {
		t.Fatal("global shortcut no longer toggles chat without clearing it")
	}
}
