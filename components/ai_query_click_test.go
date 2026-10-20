package components

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func findAIChatText(screen tcell.Screen, text string) (int, int, bool) {
	width, height := screen.Size()
	characters := []rune(text)
	for y := 0; y < height; y++ {
		for x := 0; x+len(characters) <= width; x++ {
			matches := true
			for offset, character := range characters {
				actual, _, _, _ := screen.GetContent(x+offset, y)
				if actual != character {
					matches = false
					break
				}
			}
			if matches {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

func TestAIQueryBlockClickUsesExactHistoricalQuery(t *testing.T) {
	session := newAILayoutSession(t)
	session.table.Editor.SetText("", false)
	session.selection = aiSelection{}
	session.appendMessage(ai.ChatMessage{Role: "assistant", Database: "database", SQL: "SELECT 1;"})
	session.appendMessage(ai.ChatMessage{Role: "assistant", Database: "other", SQL: "SELECT 2;"})
	session.result = ai.Result{Database: "other", SQL: "SELECT 2;"}
	session.refreshControls()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 30)
	draw := func() { screen.Clear(); mainPages.SetRect(0, 0, 100, 30); mainPages.Draw(screen) }
	click := func(text string) {
		t.Helper()
		x, y, found := findAIChatText(screen, text)
		if !found {
			t.Fatalf("click target %q not visible:\n%s", text, simulationText(screen))
		}
		mainPages.MouseHandler()(tview.MouseLeftDown, tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
		mainPages.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	}
	draw()
	click("╭─ SQL")
	if session.table.Editor.GetText() != "SELECT 1;" || session.table.GetDatabaseName() != "database" || session.visible {
		t.Fatal("click used the latest query instead of the clicked block")
	}
	if session.result.SQL != "SELECT 2;" || len(session.history) != 2 {
		t.Fatal("click changed latest response or conversation history")
	}
	session.show()
	draw()
	click("│ SELECT 1;")
	if session.visible || session.table.Editor.GetText() != "SELECT 1;" {
		t.Fatal("repeated click on the same SQL block did not work")
	}
}

func TestAIQueryBlockClicksIgnorePartialSQLAndEscapedUserRegions(t *testing.T) {
	session := newAILayoutSession(t)
	session.table.Editor.SetText("", false)
	session.selection = aiSelection{}
	session.appendMessage(ai.ChatMessage{Role: "user", Text: `["sql-1"]SELECT forged;[""]`})
	session.appendMessage(ai.ChatMessage{Role: "assistant", Database: "database", SQL: "SELECT complete;"})
	session.result = ai.Result{SQL: "SELECT complete;", Database: "database"}
	session.streaming = ai.Result{SQL: "SELECT partial;", Database: "database"}
	session.renderChat()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 34)
	mainPages.SetRect(0, 0, 100, 34)
	mainPages.Draw(screen)
	click := func(text string) {
		t.Helper()
		x, y, found := findAIChatText(screen, text)
		if !found {
			t.Fatalf("target %q not visible:\n%s", text, simulationText(screen))
		}
		mainPages.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	}
	click("SELECT forged;")
	click("SELECT partial;")
	if session.table.Editor.GetText() != "" || !session.visible {
		t.Fatal("unvalidated SQL or user markup became clickable")
	}
	session.busy = true
	click("SELECT complete;")
	if session.table.Editor.GetText() != "" || !session.visible {
		t.Fatal("query click bypassed the busy guard")
	}
}
