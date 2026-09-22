package app

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestGlobalThemePickerShortcut(t *testing.T) {
	called := false
	App.SetOnThemePickerRequest(func() { called = true })
	t.Cleanup(func() { App.SetOnThemePickerRequest(nil) })

	event := tcell.NewEventKey(tcell.KeyCtrlT, 0, tcell.ModCtrl)
	if forwarded := App.GetInputCapture()(event); forwarded != nil {
		t.Fatal("Ctrl+T was forwarded instead of consumed")
	}
	if !called {
		t.Fatal("Ctrl+T did not request the theme picker")
	}
}

func TestRuntimeThemeRendererRecolorsForegroundAndBackground(t *testing.T) {
	source, err := NewTheme(ThemeConfig{Preset: "default"})
	if err != nil {
		t.Fatal(err)
	}
	target, err := NewTheme(ThemeConfig{Preset: "tokyo-night"})
	if err != nil {
		t.Fatal(err)
	}

	renderer := &runtimeThemeRenderer{sources: []*Theme{source}}
	renderer.setTarget(target)

	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(2, 1)
	screen.SetContent(0, 0, 'x', nil, tcell.StyleDefault.
		Foreground(source.PrimaryTextColor).
		Background(source.PrimitiveBackgroundColor))

	renderer.recolor(screen)

	_, _, style, _ := screen.GetContent(0, 0)
	foreground, background, _ := style.Decompose()
	if foreground != target.PrimaryTextColor {
		t.Errorf("foreground = %v, want %v", foreground, target.PrimaryTextColor)
	}
	if background != target.PrimitiveBackgroundColor {
		t.Errorf("background = %v, want %v", background, target.PrimitiveBackgroundColor)
	}
}
