package components

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func TestAIComposerNewlineShortcuts(t *testing.T) {
	for _, shortcut := range []struct {
		name      string
		key       tcell.Key
		modifiers tcell.ModMask
	}{
		{"Alt+Enter", tcell.KeyEnter, tcell.ModAlt},
		{"Ctrl+J", tcell.KeyCtrlJ, tcell.ModNone},
		{"Shift+Enter", tcell.KeyEnter, tcell.ModShift},
		{"Ctrl+Enter", tcell.KeyEnter, tcell.ModCtrl},
	} {
		t.Run(shortcut.name, func(t *testing.T) {
			session := newAILayoutSession(t)
			session.input.SetText("First line")
			session.panel.InputHandler()(tcell.NewEventKey(shortcut.key, 0, shortcut.modifiers), func(p tview.Primitive) { app.App.SetFocus(p) })
			session.panel.PasteHandler()("Second line", func(p tview.Primitive) { app.App.SetFocus(p) })
			if text := session.input.GetText(); text != "First line\nSecond line" {
				t.Fatalf("newline shortcut produced %q", text)
			}
			if session.busy || len(session.history) != 0 || app.App.GetFocus() != session.input {
				t.Fatal("newline shortcut sent the draft or moved focus")
			}
		})
	}
}

func TestAIComposerGrowsForMultilineAndPastedDrafts(t *testing.T) {
	session := newAILayoutSession(t)
	session.input.SetText("")
	draft := "Line one\nLine two\nLine three\nLine four\nLine five"
	session.panel.PasteHandler()(draft, func(p tview.Primitive) { app.App.SetFocus(p) })
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 40)
	mainPages.SetRect(0, 0, 100, 40)
	draw := func() { screen.Clear(); mainPages.Draw(screen) }
	draw()
	_, _, _, height := session.input.GetRect()
	_, _, _, modalHeight := session.panel.GetRect()
	if height != 7 || modalHeight != 7 || !session.minimalChat() {
		t.Fatal("fresh multiline draft did not grow as an input-only modal")
	}
	for _, line := range strings.Split(draft, "\n") {
		if !strings.Contains(simulationText(screen), line) {
			t.Fatalf("multiline composer hides %q:\n%s", line, simulationText(screen))
		}
	}
	if session.makeRequest(session.input.GetText()).Prompt != draft {
		t.Fatal("multiline request lost line breaks")
	}
	session.input.SetText(draft + "\n")
	draw()
	_, _, _, height = session.input.GetRect()
	if height != 8 {
		t.Fatal("trailing newline did not allocate an empty input row")
	}
	session.appendMessage(ai.ChatMessage{Role: "user", Text: "Earlier request"})
	session.refreshControls()
	draw()
	_, _, _, height = session.input.GetRect()
	_, _, _, modalHeight = session.panel.GetRect()
	if height != 8 || modalHeight != 26 {
		t.Fatal("expanded chat did not preserve multiline composer height")
	}
	session.input.SetText("")
	draw()
	_, _, _, height = session.input.GetRect()
	if height != 4 {
		t.Fatal("clearing the draft did not shrink the composer")
	}
}

func TestAIComposerHeightIsBoundedAndLongDraftRemainsEditable(t *testing.T) {
	session := newAILayoutSession(t)
	draft := strings.Repeat("A multiline draft\n", 30) + "LAST INPUT LINE"
	session.input.SetText(draft)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, size := range [][2]int{{100, 40}, {48, 14}, {30, 6}} {
		screen.SetSize(size[0], size[1])
		screen.Clear()
		mainPages.SetRect(0, 0, size[0], size[1])
		mainPages.Draw(screen)
		x, y, width, height := session.panel.GetRect()
		if height > min(10, size[1]) || x < 0 || y < 0 || x+width > size[0] || y+height > size[1] {
			t.Fatal("long draft overflows the terminal")
		}
		if !strings.Contains(simulationText(screen), "LAST INPUT LINE") {
			t.Fatal("long composer does not scroll to the editing cursor")
		}
		if session.input.GetText() != draft {
			t.Fatal("bounded composer truncated the draft")
		}
	}
	session.input.SetText(strings.Repeat("wrapped words ", 30))
	screen.SetSize(48, 30)
	mainPages.SetRect(0, 0, 48, 30)
	mainPages.Draw(screen)
	_, _, _, height := session.input.GetRect()
	if height <= 4 {
		t.Fatal("wrapped text did not grow the input")
	}
}

func TestAIFreshComposerGrowsDuringRootLevelTypingAndAfterNewChat(t *testing.T) {
	session := newAILayoutSession(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 40)
	mainPages.SetRect(0, 0, 100, 40)
	draw := func() { screen.Clear(); mainPages.Draw(screen) }
	for _, reset := range []bool{false, true} {
		if reset {
			session.appendMessage(ai.ChatMessage{Role: "user", Text: "An earlier conversation"})
			draw()
			session.newChat()
		}
		session.input.SetText("")
		draw()
		lines := []string{"First line", "Second line", "Third line", "Fourth line", "Fifth line"}
		focus := func(p tview.Primitive) { app.App.SetFocus(p) }
		for index, line := range lines {
			if index > 0 {
				mainPages.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlJ, 0, tcell.ModNone), focus)
			}
			for _, character := range line {
				mainPages.InputHandler()(tcell.NewEventKey(tcell.KeyRune, character, tcell.ModNone), focus)
				draw()
			}
			text := simulationText(screen)
			for _, previous := range lines[:index+1] {
				if !strings.Contains(text, previous) {
					t.Fatalf("new-chat composer hides %q during typing (reset=%t):\n%s", previous, reset, text)
				}
			}
			_, _, _, height := session.panel.GetRect()
			if height != max(4, index+3) || !session.minimalChat() || len(session.history) != 0 {
				t.Fatal("new chat did not remain an auto-growing input-only view")
			}
		}
		if session.input.GetText() != strings.Join(lines, "\n") {
			t.Fatal("root-level typing lost multiline content")
		}
	}
}
