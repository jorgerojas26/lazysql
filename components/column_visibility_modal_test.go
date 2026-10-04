package components

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

func modalKey(picker *ColumnVisibilityModal, key tcell.Key, character rune) {
	picker.InputHandler()(tcell.NewEventKey(key, character, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
}

func TestColumnVisibilityModalKeyboard(t *testing.T) {
	previous := app.App.GetFocus()
	t.Cleanup(func() { app.App.SetFocus(previous) })
	var applied []string
	canceled := false
	picker := NewColumnVisibilityModal([]string{"id", "name", "secret"}, []string{"secret"}, func(hidden []string) error {
		applied = hidden
		return nil
	}, func() { canceled = true })
	app.App.SetFocus(picker.list)
	modalKey(picker, tcell.KeyRune, ' ')
	if !picker.hidden["id"] || applied != nil {
		t.Fatal("Space should change draft visibility without applying")
	}
	modalKey(picker, tcell.KeyRune, 'j')
	modalKey(picker, tcell.KeyRune, ' ')
	if picker.hidden["name"] || !strings.Contains(picker.status.GetText(false), "at least one") {
		t.Fatal("last visible column must remain checked")
	}
	modalKey(picker, tcell.KeyRune, '/')
	if app.App.GetFocus() != picker.search {
		t.Fatal("/ should focus search")
	}
	picker.search.SetText("SECRET")
	if !reflect.DeepEqual(picker.filtered, []string{"secret"}) {
		t.Fatalf("case insensitive search = %v", picker.filtered)
	}
	modalKey(picker, tcell.KeyEnter, 0)
	if applied != nil || app.App.GetFocus() != picker.list {
		t.Fatal("Enter in search should return to checkboxes")
	}
	modalKey(picker, tcell.KeyRune, ' ')
	modalKey(picker, tcell.KeyEnter, 0)
	if !reflect.DeepEqual(applied, []string{"id"}) {
		t.Fatalf("Apply = %v, want [id]", applied)
	}
	modalKey(picker, tcell.KeyRune, 'A')
	if picker.visibleCount() != 3 || len(picker.hidden) != 0 {
		t.Fatal("Show all should include columns outside the search")
	}
	modalKey(picker, tcell.KeyEscape, 0)
	if !canceled {
		t.Fatal("Esc should cancel")
	}
}

func TestColumnVisibilityModalSearchFocusAndFailure(t *testing.T) {
	previous := app.App.GetFocus()
	t.Cleanup(func() { app.App.SetFocus(previous) })
	canceled := false
	picker := NewColumnVisibilityModal([]string{"id", "name"}, nil, func(_ []string) error {
		return errors.New("permission denied")
	}, func() { canceled = true })
	app.App.SetFocus(picker.list)
	modalKey(picker, tcell.KeyEnter, 0)
	if !strings.Contains(picker.status.GetText(false), "Could not save: permission denied") || canceled {
		t.Fatal("failed save should keep the modal open with an error")
	}
	modalKey(picker, tcell.KeyTab, 0)
	if app.App.GetFocus() != picker.search {
		t.Fatal("Tab should focus search")
	}
	picker.search.SetText("no match")
	modalKey(picker, tcell.KeyEnter, 0)
	modalKey(picker, tcell.KeyRune, ' ')
	if picker.visibleCount() != 2 {
		t.Fatal("empty search results must not toggle a column")
	}
	modalKey(picker, tcell.KeyBacktab, 0)
	if app.App.GetFocus() != picker.focusable[len(picker.focusable)-1] {
		t.Fatal("Shift+Tab from list should wrap to Cancel")
	}
	modalKey(picker, tcell.KeyEnter, 0)
	if !canceled {
		t.Fatal("Cancel button should work from keyboard")
	}
	app.App.SetFocus(picker.search)
	canceled = false
	modalKey(picker, tcell.KeyEscape, 0)
	if !canceled {
		t.Fatal("Esc from search should dismiss the modal")
	}
}

func simulationText(screen tcell.SimulationScreen) string {
	screen.Show()
	cells, width, height := screen.GetContents()
	var text strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			text.WriteString(string(cells[y*width+x].Runes))
		}
		text.WriteByte('\n')
	}
	return text.String()
}

func TestColumnVisibilityModalMouseAndCompactLayout(t *testing.T) {
	previous := app.App.GetFocus()
	t.Cleanup(func() { app.App.SetFocus(previous) })
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	applied := false
	picker := NewColumnVisibilityModal([]string{"id", "secret[red]name", "email", "created_at"}, nil, func(_ []string) error {
		applied = true
		return nil
	}, func() {})
	app.App.SetFocus(picker.list)
	for _, size := range [][2]int{{80, 24}, {48, 16}, {120, 50}} {
		screen.SetSize(size[0], size[1])
		screen.Clear()
		picker.SetRect(0, 0, size[0], size[1])
		picker.Draw(screen)
		text := simulationText(screen)
		if !strings.Contains(text, "[✓]") || !strings.Contains(text, "secret[red]name") || !strings.Contains(text, "All (A)") || !strings.Contains(text, "Cancel") {
			t.Fatalf("missing modal controls at %v:\n%s", size, text)
		}
		x, y, width, height := picker.panel.GetRect()
		if width > 64 || height > 22 || x < 0 || y < 0 || x+width > size[0] || y+height > size[1] {
			t.Fatalf("modal not compact and contained: (%d,%d,%d,%d) in %v", x, y, width, height, size)
		}
	}
	consumed, _ := picker.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if !consumed || picker.visibleCount() != 4 {
		t.Fatal("clicks outside the panel should not reach the underlying table")
	}
	x, y, _, _ := picker.list.GetInnerRect()
	picker.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x+2, y+1, tcell.Button1, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if !picker.hidden["secret[red]name"] || applied {
		t.Fatal("clicking a checkbox should toggle draft visibility")
	}
	apply := picker.focusable[3]
	x, y, _, _ = apply.GetRect()
	picker.MouseHandler()(tview.MouseLeftClick, tcell.NewEventMouse(x+1, y, tcell.Button1, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if !applied {
		t.Fatal("Apply button should work with the mouse")
	}
}

func TestColumnVisibilityModalRecoversStalePreferences(t *testing.T) {
	picker := NewColumnVisibilityModal([]string{"id", "name"}, []string{"id", "name", "removed"}, func(_ []string) error { return nil }, func() {})
	if picker.visibleCount() != 1 || picker.hidden["id"] {
		t.Fatal("a stale preference hiding every available column should reveal the first")
	}
}

func TestColumnVisibilityModalOpaquePanel(t *testing.T) {
	previous := app.App.GetFocus()
	t.Cleanup(func() { app.App.SetFocus(previous) })
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	underlying := tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(tcell.ColorBlue)
	for y := 0; y < 24; y++ {
		for x := 0; x < 80; x++ {
			screen.SetContent(x, y, '░', nil, underlying)
		}
	}
	picker := NewColumnVisibilityModal([]string{"id", "name", "email"}, nil, func(_ []string) error { return nil }, func() {})
	app.App.SetFocus(picker.list)
	picker.SetRect(0, 0, 80, 24)
	picker.Draw(screen)
	x, y, width, height := picker.panel.GetRect()
	for row := y; row < y+height; row++ {
		for column := x; column < x+width; column++ {
			character, _, _, _ := screen.GetContent(column, row)
			if character == '░' {
				t.Fatalf("underlying content shows through modal panel at (%d, %d)", column, row)
			}
		}
	}
	if character, _, _, _ := screen.GetContent(x-1, y); character != '░' {
		t.Fatal("the surrounding backdrop should remain visible")
	}
}

func TestColumnVisibilityModalCompactFooter(t *testing.T) {
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	picker := NewColumnVisibilityModal([]string{"id", "name", "email"}, nil, func(_ []string) error { return nil }, func() {})
	picker.SetRect(0, 0, 80, 24)
	picker.Draw(screen)
	_, _, _, height := picker.panel.GetRect()
	if height > len(picker.columns)+5 {
		t.Fatalf("modal uses %d rows for %d columns; footer should need only two rows", height, len(picker.columns))
	}
	if picker.status.GetText(false) != "Space toggle · / find" {
		t.Fatalf("expected one short hint line, got %q", picker.status.GetText(false))
	}
	if !strings.Contains(picker.panel.GetTitle(), "3/3 visible") {
		t.Fatal("visible-column count should appear in the title, not the footer")
	}
	picker.toggle(0)
	if !strings.Contains(picker.panel.GetTitle(), "2/3 visible") {
		t.Fatal("title should reflect draft visibility changes")
	}
}
