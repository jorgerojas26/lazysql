package aicli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
	"github.com/rivo/tview"
)

func screenText(screen tcell.SimulationScreen) string {
	screen.Show()
	cells, width, height := screen.GetContents()
	var output strings.Builder
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			output.WriteString(string(cells[y*width+x].Runes))
		}
		output.WriteByte('\n')
	}
	return output.String()
}

func TestProviderSelectorKeyboardAndCompactLayout(t *testing.T) {
	selected := ""
	view := providerDialog("login", map[string]string{"chatgpt": "Logged in", "opencode-go": "Logged in"}, func(value string) { selected = value })
	application := tview.NewApplication()
	application.SetFocus(view)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, size := range [][2]int{{80, 24}, {48, 16}, {120, 35}} {
		screen.SetSize(size[0], size[1])
		screen.Clear()
		view.SetRect(0, 0, size[0], size[1])
		view.Draw(screen)
		text := screenText(screen)
		if strings.Contains(text, "Browser") || strings.Contains(text, "Subscription") || strings.Contains(text, "Choose a provider") || strings.Contains(text, "Authentication") {
			t.Fatal("provider picker contains explanatory text")
		}
		if view.panel.GetItem(0).(*tview.List).GetItemCount() != 2 {
			t.Fatal("provider picker must contain only the provider names")
		}

		for _, label := range []string{"ChatGPT", "OpenCode Go", "Logged in", "Continue", "Cancel"} {
			if !strings.Contains(text, label) {
				t.Fatalf("selector missing %s at %v:\n%s", label, size, text)
			}
		}
		x, y, width, height := view.panel.GetRect()
		if width > 68 || height > 8 || x < 0 || y < 0 || x+width > size[0] || y+height > size[1] {
			t.Fatal("selector escaped terminal bounds")
		}
	}
	key := func(code tcell.Key, character rune) {
		view.InputHandler()(tcell.NewEventKey(code, character, tcell.ModNone), func(p tview.Primitive) { application.SetFocus(p) })
	}
	key(tcell.KeyRune, 'j')
	key(tcell.KeyEnter, 0)
	if selected != "opencode-go" {
		t.Fatalf("keyboard selection = %q", selected)
	}
	key(tcell.KeyCtrlP, 0)
	key(tcell.KeyEnter, 0)
	if selected != "chatgpt" {
		t.Fatalf("keyboard selection = %q", selected)
	}
}

func TestKeyInputIsMaskedAndClearedAfterSubmission(t *testing.T) {
	submitted := ""
	view, input := textDialog("Connect OpenCode Go", "Paste your subscription API key.", "", true, func(value string) error { submitted = value; return nil })
	application := tview.NewApplication()
	application.SetFocus(view)
	input.SetText("never-display-this-secret")
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, width := range []int{48, 80} {
		screen.SetSize(width, 18)
		screen.Clear()
		view.SetRect(0, 0, width, 18)
		view.Draw(screen)
		text := screenText(screen)
		if strings.Contains(text, "never-display") || !strings.Contains(text, "*") || !strings.Contains(text, "masked") {
			t.Fatalf("unsafe or unclear key entry:\n%s", text)
		}
	}
	view.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { application.SetFocus(p) })
	if submitted != "never-display-this-secret" || input.GetText() != "" {
		t.Fatalf("key was not submitted and cleared: submitted length=%d input length=%d focus=%T", len(submitted), len(input.GetText()), application.GetFocus())
	}
}

func TestConfirmationDefaultsToNo(t *testing.T) {
	selected := ""
	view := choiceDialog("Enable AI?", "Only enable after explicit consent.", []choice{{name: "Not now", value: "no"}, {name: "Yes", value: "yes"}}, func(value string) { selected = value })
	application := tview.NewApplication()
	application.SetFocus(view)
	view.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { application.SetFocus(p) })
	list := view.panel.GetItem(1).(*tview.List)
	for index := 0; index < list.GetItemCount(); index++ {
		_, description := list.GetItemText(index)
		if description != "" {
			t.Fatal("confirmation options must not have descriptions")
		}
	}
	if selected != "no" {
		t.Fatal("confirmation opted in by default")
	}
}

func TestEmptyKeyCannotBeSubmitted(t *testing.T) {
	submitted := false
	view, _ := textDialog("Connect", "Enter a key", "", true, func(string) error { submitted = true; return nil })
	application := tview.NewApplication()
	application.SetFocus(view)
	view.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { application.SetFocus(p) })
	if submitted {
		t.Fatal("empty subscription key accepted")
	}
}

func TestAuthenticationDialogsRespectThemeAndPaintBackground(t *testing.T) {
	previous, previousTView := app.Styles, tview.Styles
	t.Cleanup(func() { app.Styles = previous; tview.Styles = previousTView })
	if err := app.ApplyTheme(app.ThemeConfig{Preset: "nord", Colors: map[string]string{
		"PrimitiveBackground": "#101112", "ContrastBackground": "#202122", "PrimaryText": "#303132", "SecondaryText": "#404142", "ContrastSecondaryText": "#505152", "TertiaryText": "#606162",
	}}); err != nil {
		t.Fatal(err)
	}
	view := choiceDialog("Themed login", "Description", []choice{{name: "ChatGPT", value: "chatgpt"}, {name: "OpenCode Go", value: "opencode-go"}}, func(string) {})
	application := tview.NewApplication()
	application.SetFocus(view)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 24)
	view.SetRect(0, 0, 80, 24)
	view.Draw(screen)
	_, _, style, _ := screen.GetContent(0, 0)
	_, background, _ := style.Decompose()
	if background != app.Styles.PrimitiveBackgroundColor {
		t.Fatal("auth screen background ignores the theme")
	}
	x, y, _, _ := view.panel.GetRect()
	_, _, style, _ = screen.GetContent(x, y)
	foreground, background, _ := style.Decompose()
	if foreground != app.Styles.SecondaryTextColor || background != app.Styles.PrimitiveBackgroundColor {
		t.Fatal("dialog border ignores theme")
	}
	list := view.panel.GetItem(1).(*tview.List)
	x, y, _, _ = list.GetInnerRect()
	_, _, style, _ = screen.GetContent(x, y)
	foreground, background, _ = style.Decompose()
	if foreground != app.Styles.ContrastSecondaryTextColor || background != app.Styles.SecondaryTextColor {
		t.Fatal("provider selection ignores theme")
	}
	view, input := textDialog("Themed key", "Description", "key", true, func(string) error { return nil })
	application.SetFocus(view)
	screen.Clear()
	view.SetRect(0, 0, 80, 24)
	view.Draw(screen)
	x, y, width, _ := input.GetInnerRect()
	found := false
	for offset := 0; offset < width; offset++ {
		character, _, style, _ := screen.GetContent(x+offset, y)
		if character != '*' {
			continue
		}
		foreground, background, _ := style.Decompose()
		if foreground != app.Styles.PrimaryTextColor || background != app.Styles.ContrastBackgroundColor {
			t.Fatal("key entry ignores theme")
		}
		found = true
		break
	}
	if !found {
		t.Fatal("masked key input was not rendered")
	}
}

func TestModelPickerSearchSelectionAndCurrentModel(t *testing.T) {
	models := []ai.Model{
		{ID: "claude-haiku", Name: "Claude Haiku", Format: "anthropic"},
		{ID: "glm-flash", Name: "GLM Flash", Format: "chat-completions"},
		{ID: "gpt-model", Name: "GPT model", Format: "responses"},
		{ID: "gemini-model", Name: "Gemini model", Format: "gemini"},
	}
	selected := ""
	application := tview.NewApplication()
	view, search, list := modelDialog("OpenCode Go", models, "glm-flash", func(id string) { selected = id }, func(p tview.Primitive) { application.SetFocus(p) })
	application.SetFocus(view)
	for index := 0; index < list.GetItemCount(); index++ {
		_, description := list.GetItemText(index)
		if description != "" {
			t.Fatal("model options must not have descriptions")
		}
	}
	if list.GetCurrentItem() != 1 || list.GetItemCount() != len(models) {
		t.Fatal("picker must include every model and preselect current model")
	}
	key := func(code tcell.Key) {
		view.InputHandler()(tcell.NewEventKey(code, 0, tcell.ModNone), func(p tview.Primitive) { application.SetFocus(p) })
	}
	search.SetText("GEMINI")
	key(tcell.KeyEnter)
	if selected != "gemini-model" {
		t.Fatalf("case-insensitive model search selected %q", selected)
	}
	selected = ""
	search.SetText("no matches")
	key(tcell.KeyEnter)
	if selected != "" {
		t.Fatal("unmatched model search accepted arbitrary input")
	}
	search.SetText("")
	key(tcell.KeyTab)
	if application.GetFocus() != list {
		t.Fatal("Tab must focus model list")
	}
	key(tcell.KeyDown)
	key(tcell.KeyEnter)
	if selected == "" {
		t.Fatal("model list keyboard selection failed")
	}
	key(tcell.KeyBacktab)
	if application.GetFocus() != search {
		t.Fatal("Shift+Tab must return to model search")
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	for _, size := range [][2]int{{48, 16}, {80, 24}} {
		screen.SetSize(size[0], size[1])
		screen.Clear()
		view.SetRect(0, 0, size[0], size[1])
		view.Draw(screen)
		text := screenText(screen)
		if !strings.Contains(text, "OpenCode Go models") || !strings.Contains(text, "Select") {
			t.Fatalf("model picker unusable at %v:\n%s", size, text)
		}
	}
}

func TestModelPickerKeepsPreselectedModelVisibleOnSmallTerminals(t *testing.T) {
	var models []ai.Model
	for index := 0; index < 50; index++ {
		id := fmt.Sprintf("model-%02d", index)
		models = append(models, ai.Model{ID: id, Name: id, Format: "chat-completions"})
	}
	application := tview.NewApplication()
	view, _, _ := modelDialog("OpenCode Go", models, "model-45", func(string) {}, func(p tview.Primitive) { application.SetFocus(p) })
	application.SetFocus(view)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(48, 16)
	view.SetRect(0, 0, 48, 16)
	view.Draw(screen)
	if !strings.Contains(screenText(screen), "model-45") {
		t.Fatal("preselected model is outside the visible list")
	}
}

func TestDefaultProviderConfirmationPreselectsYes(t *testing.T) {
	selected := ""
	view := confirmationDialog("Default AI provider", "Use OpenCode Go by default?", true, func(value string) { selected = value })
	application := tview.NewApplication()
	application.SetFocus(view)
	view.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { application.SetFocus(p) })
	if selected != "yes" {
		t.Fatal("default provider confirmation should preselect Yes")
	}
}

func TestChatGPTModelPickerIsProviderSpecificAndMinimal(t *testing.T) {
	application := tview.NewApplication()
	view, _, list := modelDialog("ChatGPT", []ai.Model{{ID: "account-model", Name: "Account Model"}}, "account-model", func(string) {}, func(p tview.Primitive) { application.SetFocus(p) })
	application.SetFocus(view)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(48, 16)
	view.SetRect(0, 0, 48, 16)
	view.Draw(screen)
	text := screenText(screen)
	if !strings.Contains(text, "ChatGPT models") || !strings.Contains(text, "Account Model") || strings.Contains(text, "OpenCode") {
		t.Fatalf("incorrect ChatGPT selector:\n%s", text)
	}
	if _, description := list.GetItemText(0); description != "" {
		t.Fatal("ChatGPT selector added option descriptions")
	}
}

func TestProviderStatusColorsRespectThemeAndSelection(t *testing.T) {
	previous, previousTView := app.Styles, tview.Styles
	t.Cleanup(func() { app.Styles = previous; tview.Styles = previousTView })
	if err := app.ApplyTheme(app.ThemeConfig{Preset: "nord", Colors: map[string]string{
		"PrimitiveBackground": "#101112", "TertiaryText": "#20aa30", "Graphics": "#808182", "SecondaryText": "#e0a020", "Error": "#dd3030", "ReadOnly": "#2080dd",
	}}); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		state string
		color tcell.Color
	}{
		{"Logged in", app.Styles.TertiaryTextColor},
		{"Not logged in", app.Styles.GraphicsColor}, {"Plan access needed", app.Styles.SecondaryTextColor}, {"Sign in again", app.Styles.SecondaryTextColor},
		{"Check credentials", app.Styles.ErrorColor}, {"Check key", app.Styles.ErrorColor}, {"Unavailable", app.Styles.ErrorColor}, {"Account saved", app.Styles.ReadOnlyColor},
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(80, 16)
	application := tview.NewApplication()
	for _, test := range cases {
		view := providerDialog("login", map[string]string{"chatgpt": test.state}, func(string) {})
		application.SetFocus(view)
		for _, selected := range []int{0, 1} {
			view.options.SetCurrentItem(selected)
			screen.Clear()
			view.SetRect(0, 0, 80, 16)
			view.Draw(screen)
			x, y, _, _ := view.options.GetInnerRect()
			position := tview.TaggedStringWidth("ChatGPT · ")
			_, _, style, _ := screen.GetContent(x+position, y)
			foreground, background, _ := style.Decompose()
			expectedBackground := app.Styles.PrimitiveBackgroundColor
			if selected == 0 {
				expectedBackground = app.Styles.SecondaryTextColor
			}
			if foreground != test.color || background != expectedBackground {
				t.Fatalf("%s selected=%v colors=%v/%v", test.state, selected == 0, foreground, background)
			}
			if !strings.Contains(screenText(screen), test.state) {
				t.Fatal("colored state text was not rendered")
			}
		}
	}
}
