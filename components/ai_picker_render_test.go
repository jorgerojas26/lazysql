package components

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func TestAIPickerBackgroundIsOpaqueWithinPopupBounds(t *testing.T) {
	for _, kind := range []string{"models", "thinking"} {
		t.Run(kind, func(t *testing.T) {
			session, _ := newAIModelSelectionSession(t)
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			defer screen.Fini()
			screen.SetSize(100, 40)
			mainPages.SetRect(0, 0, 100, 40)
			if kind == "models" {
				session.openModelPicker()
			} else {
				session.openThinkingPicker()
			}
			mainPages.Draw(screen)
			picker := session.picker
			type cell struct {
				character rune
				style     tcell.Style
			}
			for _, width := range []int{100, 48} {
				screen.SetSize(width, 40)
				picker.SetRect(0, 0, width, 40)
				for y := range 40 {
					for x := range width {
						screen.SetContent(x, y, ' ', nil, tcell.StyleDefault.Background(app.Styles.PrimitiveBackgroundColor))
					}
				}
				picker.Draw(screen)
				px, py, w, h := picker.frame.GetRect()
				expected := make([]cell, w*h)
				for y := range h {
					for x := range w {
						character, _, style, _ := screen.GetContent(px+x, py+y)
						expected[y*w+x] = cell{character, style}
					}
				}
				background := tcell.StyleDefault.Foreground(tcell.ColorRed).Background(tcell.ColorGreen)
				for y := range 40 {
					for x := range width {
						screen.SetContent(x, y, 'X', nil, background)
					}
				}
				picker.Draw(screen)
				for y := range 40 {
					for x := range width {
						character, _, style, _ := screen.GetContent(x, y)
						actual := cell{character, style}
						if x >= px && x < px+w && y >= py && y < py+h {
							if actual != expected[(y-py)*w+x-px] {
								t.Fatalf("%s picker shows underlying content at %d,%d (width %d)", kind, x, y, width)
							}
						} else if actual != (cell{'X', background}) {
							t.Fatalf("%s picker erased content outside its bounds at %d,%d", kind, x, y)
						}
					}
				}
			}
		})
	}
}

func TestAIThinkingPickerHasNoSearchAndFocusesList(t *testing.T) {
	session, _ := newAIModelSelectionSession(t)
	session.openThinkingPicker()
	picker := session.picker
	if picker.searchable || picker.frame.GetItemCount() != 3 || app.App.GetFocus() != picker.table {
		t.Fatal("thinking picker still contains/focuses a search field")
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(100, 40)
	mainPages.SetRect(0, 0, 100, 40)
	mainPages.Draw(screen)
	if strings.Contains(simulationText(screen), "Search:") || strings.Contains(simulationText(screen), "Type to filter") {
		t.Fatal("thinking picker renders search UI")
	}
	for _, key := range []tcell.Key{tcell.KeyTab, tcell.KeyBacktab} {
		picker.frame.InputHandler()(tcell.NewEventKey(key, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
		if app.App.GetFocus() != picker.table {
			t.Fatal("Tab focused a hidden search field")
		}
	}
	picker.table.Select(0, 0)
	picker.frame.InputHandler()(tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	picker.frame.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if session.picker != nil || session.config.Thinking != "low" || app.App.GetFocus() != session.input {
		t.Fatal("thinking cannot be selected directly with arrows and Enter")
	}
	session.openModelPicker()
	if !session.picker.searchable || app.App.GetFocus() != session.picker.search {
		t.Fatal("model picker lost search")
	}
}

func TestAIPickersOpenBelowFreshInputAndAboveExpandedComposer(t *testing.T) {
	for _, kind := range []string{"models", "thinking"} {
		t.Run(kind, func(t *testing.T) {
			session, _ := newAIModelSelectionSession(t)
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			defer screen.Fini()
			screen.SetSize(100, 40)
			mainPages.SetRect(0, 0, 100, 40)
			open := func() {
				if kind == "models" {
					session.openModelPicker()
				} else {
					session.openThinkingPicker()
				}
			}
			mainPages.Draw(screen)
			_, inputY, _, inputHeight := session.input.GetRect()
			open()
			mainPages.Draw(screen)
			_, pickerY, _, pickerHeight := session.picker.frame.GetRect()
			if pickerY != inputY+inputHeight || pickerY+pickerHeight > 40 {
				t.Fatal("fresh-chat picker is not directly below the input")
			}
			session.closePicker()
			session.appendMessage(ai.ChatMessage{Role: "user", Text: "A sent request"})
			session.refreshControls()
			screen.SetSize(100, 28)
			mainPages.SetRect(0, 0, 100, 28)
			mainPages.Draw(screen)
			_, inputY, _, _ = session.input.GetRect()
			open()
			mainPages.Draw(screen)
			_, pickerY, _, pickerHeight = session.picker.frame.GetRect()
			if pickerY+pickerHeight > inputY || pickerY < 0 {
				t.Fatal("picker did not fall back above the bottom-positioned composer")
			}
		})
	}
}
