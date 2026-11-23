package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/rivo/tview"
)

func restoreBorders(t *testing.T) {
	t.Helper()
	previous := activeBorders
	t.Cleanup(func() { _ = App.PreviewBorders(previous) })
}

func TestBorderStylesRenderNormalAndFocusedWithoutChangingText(t *testing.T) {
	restoreBorders(t)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(18, 6)
	normal := tview.NewBox().SetBorder(true)
	normal.SetRect(0, 0, 8, 6)
	focused := tview.NewBox().SetBorder(true)
	focused.SetRect(9, 0, 8, 6)
	focused.Focus(nil)
	for _, tc := range []struct {
		style         string
		normal, focus rune
	}{
		{"auto", '┌', '╔'}, {"single", '┌', '┌'}, {"double", '╔', '╔'},
	} {
		t.Run(tc.style, func(t *testing.T) {
			if err := App.PreviewBorders(BorderConfig{Style: tc.style}); err != nil {
				t.Fatal(err)
			}
			normal.Draw(screen)
			focused.Draw(screen)
			screen.SetContent(17, 0, '─', nil, tcell.StyleDefault)
			drawBorders(screen)
			ch, _, _, _ := screen.GetContent(0, 0)
			if ch != tc.normal {
				t.Errorf("normal = %q, want %q", ch, tc.normal)
			}
			ch, _, _, _ = screen.GetContent(9, 0)
			if ch != tc.focus {
				t.Errorf("focused = %q, want %q", ch, tc.focus)
			}
			ch, _, _, _ = screen.GetContent(17, 0)
			if ch != '─' {
				t.Errorf("content changed to %q", ch)
			}
		})
	}
}

func TestBorderColorsRenderIndependently(t *testing.T) {
	restoreBorders(t)
	if err := App.PreviewBorders(BorderConfig{Style: "single", NormalColor: "#6B7280", FocusColor: "#38BDF8"}); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(18, 6)
	left, right := tview.NewBox().SetBorder(true), tview.NewBox().SetBorder(true)
	left.SetRect(0, 0, 8, 6)
	right.SetRect(9, 0, 8, 6)
	right.Focus(nil)
	left.Draw(screen)
	right.Draw(screen)
	drawBorders(screen)
	_, _, normalStyle, _ := screen.GetContent(0, 0)
	_, _, focusStyle, _ := screen.GetContent(9, 0)
	normal, _, _ := normalStyle.Decompose()
	focus, _, _ := focusStyle.Decompose()
	if normal != tcell.GetColor("#6B7280") {
		t.Errorf("normal color = %v", normal)
	}
	if focus != tcell.GetColor("#38BDF8") {
		t.Errorf("focus color = %v", focus)
	}
}

func TestFocusedTableGridUsesFocusColorWithMatchingThemeColors(t *testing.T) {
	restoreBorders(t)
	previousTheme := Styles
	t.Cleanup(func() { Styles = previousTheme; tview.Styles = previousTheme.Theme })
	if err := ApplyTheme(ThemeConfig{Preset: "dracula"}); err != nil {
		t.Fatal(err)
	}
	if Styles.PrimaryTextColor != Styles.BorderColor {
		t.Fatal("test requires matching theme colors")
	}
	if err := App.PreviewBorders(BorderConfig{Style: "double", NormalColor: "#6B7280", FocusColor: "#38BDF8"}); err != nil {
		t.Fatal(err)
	}
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(14, 5)
	table := tview.NewTable().SetBorders(true)
	table.SetCell(0, 0, tview.NewTableCell("a"))
	table.SetCell(0, 1, tview.NewTableCell("b"))
	table.SetBordersColor(TableBorderMarker(true))
	table.SetRect(0, 0, 14, 5)
	table.Draw(screen)
	drawBorders(screen)
	found := false
	for y := 0; y < 5; y++ {
		for x := 0; x < 14; x++ {
			ch, _, style, _ := screen.GetContent(x, y)
			if ch != '║' && ch != '═' && ch != '╬' {
				continue
			}
			fg, _, _ := style.Decompose()
			if fg == tcell.GetColor("#38BDF8") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("focused table grid did not use focus border color")
	}
}

func TestBorderConfigLocalPrecedenceAndSavePreservesConnections(t *testing.T) {
	restoreBorders(t)
	global := "[[database]]\nName = 'prod'\nURL = 'postgres://${env:DB_PASS}@host/db'\n\n[borders]\nstyle = 'double'\nnormal_color = '#6B7280'\n\n[other]\nkeep = true\n"
	local := "[borders]\nstyle = 'single'\nextra = 'still-here'\n\n[project]\nkeep = true\n"
	globalPath, localPath := loadProjectConfig(t, global, local)
	if got := App.BorderConfig(); got.Style != "single" || got.NormalColor != "#6B7280" {
		t.Fatalf("merged borders = %+v", got)
	}
	if err := App.SaveBorderColors("", "#38BDF8"); err != nil {
		t.Fatal(err)
	}
	if err := App.SaveBorderStyle("auto"); err != nil {
		t.Fatal(err)
	}
	localText := readFile(t, localPath)
	if strings.Contains(localText, "database") || strings.Contains(localText, "DB_PASS") {
		t.Fatalf("global data copied to local file: %s", localText)
	}
	localTable := readTable(t, localPath)
	borders := localTable["borders"].(map[string]any)
	if borders["normal_color"] != "" || borders["focus_color"] != "#38BDF8" || borders["style"] != "auto" {
		t.Fatalf("saved borders = %v", borders)
	}
	if borders["extra"] != "still-here" {
		t.Fatal("unknown border option removed")
	}
	if localTable["project"].(map[string]any)["keep"] != true {
		t.Fatal("unknown local option removed")
	}
	if got := readFile(t, globalPath); got != global {
		t.Fatal("global config changed")
	}
	App.config = &Config{ConfigFile: globalPath}
	if err := LoadConfig(globalPath); err != nil {
		t.Fatal(err)
	}
	if got := App.BorderConfig(); got.Style != "auto" || got.NormalColor != "" || got.FocusColor != "#38BDF8" {
		t.Fatalf("reloaded borders = %+v", got)
	}
}

func TestBorderConfigValidation(t *testing.T) {
	restoreBorders(t)
	for _, cfg := range []BorderConfig{{Style: "none"}, {Style: "single", NormalColor: "#12345"}, {Style: "double", FocusColor: "unknown-color"}} {
		if err := App.PreviewBorders(cfg); err == nil {
			t.Errorf("accepted %+v", cfg)
		}
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[borders]\nstyle = 'none'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	previous := App.config
	t.Cleanup(func() { App.config = previous })
	App.config = &Config{ConfigFile: path}
	if err := LoadConfig(path); err == nil {
		t.Fatal("invalid border style loaded")
	}
}

func TestThemeChangeKeepsBorderCustomization(t *testing.T) {
	restoreBorders(t)
	previous := Styles
	t.Cleanup(func() {
		Styles = previous
		tview.Styles = previous.Theme
		themeRenderer.mu.Lock()
		themeRenderer.target, themeRenderer.sources = nil, nil
		themeRenderer.fg, themeRenderer.bg = nil, nil
		themeRenderer.mu.Unlock()
	})
	if err := ApplyTheme(ThemeConfig{Preset: "light"}); err != nil {
		t.Fatal(err)
	}
	if err := App.PreviewBorders(BorderConfig{Style: "single", FocusColor: "#38BDF8"}); err != nil {
		t.Fatal(err)
	}
	lightNormal := BorderForeground(Styles.BorderColor, false)
	if err := App.PreviewTheme(ThemeConfig{Preset: "tokyo-night"}); err != nil {
		t.Fatal(err)
	}
	if got := App.BorderConfig(); got.Style != "single" || got.FocusColor != "#38BDF8" {
		t.Fatalf("border customization changed: %+v", got)
	}
	if got := BorderForeground(Styles.BorderColor, false); got == lightNormal {
		t.Fatal("unconfigured normal color did not follow theme")
	}
	if got := BorderForeground(Styles.PrimaryTextColor, true); got != tcell.GetColor("#38BDF8") {
		t.Fatalf("focus color = %v", got)
	}
}

func TestBorderShortcutsRespectCustomBindings(t *testing.T) {
	previous := App.config
	previousKeys := append(Map(nil), Keymaps.Global...)
	t.Cleanup(func() {
		App.config = previous
		Keymaps.Global = previousKeys
		App.SetOnBorderStyleChange(nil)
		App.SetOnBorderPickerRequest(nil)
	})
	App.config = defaultConfig()
	styleCalls, pickerCalls := 0, 0
	App.SetOnBorderStyleChange(func(string) { styleCalls++ })
	App.SetOnBorderPickerRequest(func() { pickerCalls++ })
	ctrlB := tcell.NewEventKey(tcell.KeyCtrlB, 0, tcell.ModCtrl)
	ctrlG := tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModCtrl)
	if App.GetInputCapture()(ctrlB) != nil || App.GetInputCapture()(ctrlG) != nil {
		t.Fatal("default border shortcuts were forwarded")
	}
	if styleCalls != 1 || pickerCalls != 1 {
		t.Fatalf("default shortcut calls = %d, %d", styleCalls, pickerCalls)
	}
	App.config.Keymaps = map[string]map[string]string{"Home": {"SwitchToEditorView": "Ctrl-B", "SearchGlobal": "Ctrl-G"}}
	if App.GetInputCapture()(ctrlB) == nil || App.GetInputCapture()(ctrlG) == nil {
		t.Fatal("custom shortcut was consumed")
	}
	if styleCalls != 1 || pickerCalls != 1 {
		t.Fatal("border handlers ran despite custom shortcuts")
	}
	// An explicitly remapped global command must also win over a new default.
	App.config.Keymaps = map[string]map[string]string{"Global": {"CheckForUpdates": "Ctrl-G"}}
	if err := ApplyKeymapConfig(App.config.Keymaps); err != nil {
		t.Fatal(err)
	}
	if got := Keymaps.Resolve(ctrlG); got != commands.CheckForUpdates {
		t.Fatalf("Ctrl+G resolves to %s, want CheckForUpdates", got)
	}
}
