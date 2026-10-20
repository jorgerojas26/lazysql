package aicli

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func TestInvalidKeyStaysMaskedInlineAndCanBeCorrected(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	completed := false
	attempts := 0
	view, input := textDialog("Connect OpenCode Go", "Paste your subscription key.", "", true, func(value string) error {
		attempts++
		if err := ai.SaveOpenCodeKey(value); err != nil {
			return err
		}
		completed = true
		return nil
	})
	application := tview.NewApplication()
	application.SetFocus(view)
	enter := func() {
		view.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { application.SetFocus(p) })
	}
	input.SetText("invalid secret")
	enter()
	if completed || input.GetText() != "invalid secret" || attempts != 1 || application.GetFocus() != input {
		t.Fatal("invalid key closed the dialog, lost input or moved focus")
	}
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
		if !strings.Contains(text, "whitespace") || !strings.Contains(text, "retry") || strings.Contains(text, "invalid secret") {
			t.Fatalf("inline validation is missing or leaked input:\n%s", text)
		}
	}
	path, _ := ai.OpenCodeCredentialPath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("invalid key was saved")
	}
	input.SetText("corrected-key")
	enter()
	if !completed || attempts != 2 || input.GetText() != "" {
		t.Fatal("correcting the key did not complete submission")
	}
	if key, err := ai.LoadOpenCodeKey(); err != nil || key != "corrected-key" {
		t.Fatal("corrected key was not saved")
	}
}

func TestCredentialSaveFailureCanRetryWithoutReenteringKey(t *testing.T) {
	attempts := 0
	completed := false
	view, input := textDialog("Connect", "Paste your key.", "", true, func(string) error {
		attempts++
		if attempts == 1 {
			return errors.New("cannot write subscription credential")
		}
		completed = true
		return nil
	})
	application := tview.NewApplication()
	application.SetFocus(view)
	input.SetText("keep-this-key")
	enter := func() {
		view.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { application.SetFocus(p) })
	}
	enter()
	if completed || input.GetText() != "keep-this-key" {
		t.Fatal("save error discarded key input")
	}
	enter()
	if !completed || input.GetText() != "" || attempts != 2 {
		t.Fatal("retry did not resubmit the preserved key")
	}
}

func TestLoginValidationRetriesWithinKeyEntry(t *testing.T) {
	ui := &fakeUI{provider: "opencode-go", texts: []string{"invalid key", "valid-key"}, confirms: []bool{false}}
	flow, _ := newTestCommand(t, ui)
	if err := flow.execute(context.Background(), "login"); err != nil {
		t.Fatal(err)
	}
	if len(ui.keyErrors) != 1 || len(ui.recovered) != 0 {
		t.Fatal("key validation escaped key-entry instead of retrying inline")
	}
	if key, err := ai.LoadOpenCodeKey(); err != nil || key != "valid-key" {
		t.Fatal("corrected login was not saved")
	}
}

func TestInteractiveFailuresAreShownInUIAndCanCloseCleanly(t *testing.T) {
	previous, previousTView := app.Styles, tview.Styles
	t.Cleanup(func() { app.Styles = previous; tview.Styles = previousTView })
	ui := &fakeUI{provider: "chatgpt"}
	flow, output := newTestCommand(t, ui)
	flow.actions.login = func(context.Context) error { return errors.New("provider login failed") }
	if err := flow.run(context.Background(), "login"); err != nil {
		t.Fatal("displayed runtime error still exits with failure:", err)
	}
	if len(ui.recovered) != 1 || ui.recovered[0] != "provider login failed" {
		t.Fatal("runtime failure did not reach in-app recovery")
	}
	if strings.Contains(output.String(), "provider login failed") {
		t.Fatal("runtime failure was printed as a terminal error")
	}
}

func TestInteractiveFailureCanReturnToProviderSelectionAndRetry(t *testing.T) {
	previous, previousTView := app.Styles, tview.Styles
	t.Cleanup(func() { app.Styles = previous; tview.Styles = previousTView })
	ui := &fakeUI{provider: "chatgpt", recoverChoices: []bool{true}, confirms: []bool{false}}
	flow, _ := newTestCommand(t, ui)
	attempts := 0
	flow.actions.login = func(context.Context) error {
		attempts++
		if attempts == 1 {
			return errors.New("temporary authentication failure")
		}
		return nil
	}
	if err := flow.run(context.Background(), "login"); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || len(ui.recovered) != 1 {
		t.Fatal("authentication recovery did not retry")
	}
}
