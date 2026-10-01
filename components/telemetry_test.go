package components

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/telemetry"
)

func TestTelemetryPromptDefaultsToEnabled(t *testing.T) {
	var choices []bool
	modal := newTelemetryModal("https://example.com/v1/events", func(yes bool) { choices = append(choices, yes) })
	if modal.form.GetButton(0).GetLabel() != "Keep enabled" || modal.form.GetButton(1).GetLabel() != "Disable" {
		t.Fatal("default must be Keep enabled")
	}
	if !strings.Contains(modal.text.GetText(true), "every five minutes") {
		t.Fatal("consent prompt must disclose the five-minute heartbeat interval")
	}
	var focus func(tview.Primitive)
	var current tview.Primitive
	focus = func(p tview.Primitive) {
		if current != nil {
			current.Blur()
		}
		current = p
		p.Focus(focus)
	}
	modal.Focus(focus)
	if !modal.form.GetButton(0).HasFocus() {
		t.Fatal("Keep enabled button is not focused")
	}
	modal.form.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), focus)
	modal.form.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), focus)
	if len(choices) != 2 || !choices[0] || !choices[1] {
		t.Fatalf("expected default enabled and Esc enabled, got %v", choices)
	}
}

func TestTelemetryPromptExplicitDisable(t *testing.T) {
	agreed := true
	modal := newTelemetryModal("https://example.com/v1/events", func(yes bool) { agreed = yes })
	var focus func(tview.Primitive)
	var current tview.Primitive
	focus = func(p tview.Primitive) {
		if current != nil {
			current.Blur()
		}
		current = p
		p.Focus(focus)
	}
	modal.Focus(focus)
	modal.form.InputHandler()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), focus)
	if !modal.form.GetButton(1).HasFocus() {
		t.Fatal("Disable button was not selected")
	}
	modal.form.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), focus)
	if agreed {
		t.Fatal("Disable was not accepted")
	}
}

func prepareTelemetryTest(t *testing.T, endpoint, preference string, disabled bool) (*tview.Pages, func()) {
	t.Helper()
	for _, key := range []string{"CI", "DO_NOT_TRACK", "LAZYSQL_NO_TELEMETRY"} {
		t.Setenv(key, "")
	}
	previous := app.App.Application
	app.App.Application = tview.NewApplication()
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(100, 30)
	app.App.SetScreen(screen)
	t.Cleanup(func() {
		screen.Fini()
		app.App.Application = previous
	})
	pages := tview.NewPages().AddPage("base", tview.NewBox(), true, true)
	app.App.SetRoot(pages, true)
	stop := PrepareTelemetry(pages, endpoint, "1.2.3", preference, disabled, telemetry.Picker, telemetry.ReleaseBuild)
	t.Cleanup(stop)
	return pages, stop
}

func TestTelemetryDisclosureGateAndPreferences(t *testing.T) {
	endpoint := "https://example.com/v1/events"
	for _, choice := range []string{"default", "escape", "disable"} {
		t.Run(choice, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "telemetry.toml")
			pages, _ := prepareTelemetryTest(t, endpoint, path, false)
			_, primitive := pages.GetFrontPage()
			modal, ok := primitive.(*telemetryPrompt)
			if !ok {
				t.Fatal("first run needs a disclosure")
			}
			// A queued Enter before the first draw must not enable telemetry.
			modal.form.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { app.App.SetFocus(p) })
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("stored preference before showing disclosure")
			}
			app.App.ForceDraw()
			if !modal.shown {
				t.Fatal("disclosure not drawn")
			}
			// Disable the network independently so this UI test never contacts
			// an external collector. Client sends/gates are tested separately.
			t.Setenv("CI", "1")
			switch choice {
			case "default":
				modal.form.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { app.App.SetFocus(p) })
			case "escape":
				modal.form.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(p tview.Primitive) { app.App.SetFocus(p) })
			case "disable":
				modal.form.GetButton(1).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { app.App.SetFocus(p) })
			}
			enabled, decided, err := telemetry.LoadConsent(path, endpoint)
			if err != nil || !decided || enabled != (choice != "disable") {
				t.Fatalf("preference: %v %v %v", enabled, decided, err)
			}
			if pages.HasPage("telemetry-consent") {
				t.Fatal("disclosure was not dismissed")
			}
		})
	}
}

func TestTelemetryDisclosureSuppression(t *testing.T) {
	endpoint := "https://example.com/v1/events"
	for _, tc := range []struct {
		name, endpoint, content, env string
		disabled                     bool
		prompt                       bool
	}{
		{name: "new", endpoint: endpoint, prompt: true},
		{name: "source"},
		{name: "non-interactive-or-flag", endpoint: endpoint, disabled: true},
		{name: "disabled-globally", endpoint: endpoint, content: "enabled = false"},
		{name: "enabled", endpoint: endpoint, content: "enabled = true\nendpoint = '" + endpoint + "'"},
		{name: "changed-endpoint", endpoint: endpoint, content: "enabled = true\nendpoint = 'https://old.example.com/v1/events'", prompt: true},
		{name: "disabled-old-endpoint", endpoint: endpoint, content: "enabled = false\nendpoint = 'https://old.example.com/v1/events'"},
		{name: "malformed", endpoint: endpoint, content: "enabled = 'true'"},
		{name: "missing-endpoint", endpoint: endpoint, content: "enabled = true"},
		{name: "unreadable", endpoint: endpoint},
		{name: "DNT", endpoint: endpoint, env: "DO_NOT_TRACK"},
		{name: "no-telemetry-env", endpoint: endpoint, env: "LAZYSQL_NO_TELEMETRY"},
		{name: "CI", endpoint: endpoint, env: "CI"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "telemetry.toml")
			if tc.content != "" {
				if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.name == "unreadable" {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			pages, stop := prepareTelemetryTest(t, "", path, true)
			stop()
			if tc.env != "" {
				t.Setenv(tc.env, "1")
			}
			cleanup := PrepareTelemetry(pages, tc.endpoint, "1.2.3", path, tc.disabled, telemetry.Picker, telemetry.ReleaseBuild)
			defer cleanup()
			if pages.HasPage("telemetry-consent") != tc.prompt {
				t.Fatalf("prompt = %v, want %v", pages.HasPage("telemetry-consent"), tc.prompt)
			}
			// No drawing here: even an existing positive preference must wait
			// for terminal initialization before starting the sender.
		})
	}
}

func TestTelemetryPreferenceWriteFailureStaysOff(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "parent")
	path := filepath.Join(parent, "telemetry.toml")
	pages, _ := prepareTelemetryTest(t, "https://example.com/v1/events", path, false)
	_, primitive := pages.GetFrontPage()
	modal := primitive.(*telemetryPrompt)
	app.App.ForceDraw()
	// Make saving fail only after the valid first-run read.
	if err := os.WriteFile(parent, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	modal.form.GetButton(0).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(p tview.Primitive) { app.App.SetFocus(p) })
	if !strings.Contains(modal.text.GetText(true), "Telemetry remains off") || modal.form.GetButton(0).GetLabel() != "OK" {
		t.Fatal("write failure did not fail closed")
	}
}
