package components

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

func TestAIQueryBoxWrapsWithoutChangingLiteralText(t *testing.T) {
	sql := `SELECT 'café🙂', 'a\nb', '[red]', 42 FROM very_long_table_name WHERE id > 10;`
	for _, width := range []int{18, 42, 80} {
		block := chatSQLBlock(sql, width)
		for _, line := range strings.Split(block, "\n") {
			if got := tview.TaggedStringWidth(line); got != width {
				t.Fatalf("query box width = %d, want %d: %q", got, width, line)
			}
		}
		if !strings.Contains(block, "╭─ SQL") || !strings.Contains(block, "╰") {
			t.Fatal("query is not inside a box")
		}
	}
	block := chatSQLBlock(sql, 100)
	view := tview.NewTextView().SetDynamicColors(true).SetText(block)
	plain := view.GetText(true)
	if !strings.Contains(plain, `'a\nb'`) || !strings.Contains(plain, "[red]") || !strings.Contains(plain, "café🙂") {
		t.Fatalf("SQL literal was altered: %q", plain)
	}
}

func TestAIQueryBoxUsesEditorSyntaxColors(t *testing.T) {
	view := tview.NewTextView().SetDynamicColors(true).SetText(chatSQLBlock("SELECT 'hello', 42; -- note", 60))
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(60, 6)
	view.SetRect(0, 0, 60, 6)
	view.Draw(screen)
	expected := map[rune]tcell.Color{'S': app.Styles.SQLKeywordColor, 'h': app.Styles.SQLStringColor, '4': app.Styles.SQLNumberColor, 'n': app.Styles.SQLCommentColor}
	for character, color := range expected {
		found := false
		for x := 0; x < 60; x++ {
			main, _, style, _ := screen.GetContent(x, 1)
			foreground, _, _ := style.Decompose()
			if main == character && foreground == color {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%q did not use editor syntax color %s", character, color)
		}
	}
}
