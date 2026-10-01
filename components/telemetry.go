package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/telemetry"
)

var usage telemetry.Reporter

// Argument connections complete before the TUI/disclosure. Retain only these
// bounded facts and report them once the disclosure gate opens.
var initialTelemetryEngine string
var initialTelemetryReadOnly bool

type telemetryPrompt struct {
	*tview.Flex
	text  *tview.TextView
	form  *tview.Form
	shown bool
}

func newTelemetryModal(endpoint string, done func(bool)) *telemetryPrompt {
	text := tview.NewTextView().SetWordWrap(true).SetText(fmt.Sprintf(
		"LazySQL collects minimal anonymous aggregate usage counts.\n\n"+
			"We do NOT collect SQL, database/schema/table/column names, connection details, identifiers or personal information. "+
			"No cookies or device metadata.\n\n"+
			"Counts cover release, startup mode, distribution, database engine, read-only usage, features and coarse connection failures. "+
			"Feature counts are sent every five minutes to:\n%s\n\n"+
			"Keep enabled is the default; Esc also keeps it enabled. Disable later with --no-telemetry or global telemetry.toml.\n\n"+
			"The host sees your IP during delivery; only aggregates are stored (30-day retention target). "+
			"Details: docs/telemetry.md in the repository.\n\n"+
			"Up/Down: scroll. Tab: choose. Enter: confirm.", endpoint))
	form := tview.NewForm().AddButton("Keep enabled", func() { done(true) }).AddButton("Disable", func() { done(false) }).
		SetButtonsAlign(tview.AlignCenter).SetCancelFunc(func() { done(true) })
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
			text.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})
	flex := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(text, 0, 1, false).AddItem(form, 3, 0, true)
	flex.SetBorder(true).SetTitle(" Anonymous usage counts ").SetBorderPadding(1, 1, 1, 1)
	return &telemetryPrompt{Flex: flex, text: text, form: form}
}

// Keep the disclosure centered and scrollable even on small terminals.
func (p *telemetryPrompt) Draw(screen tcell.Screen) {
	x, y, width, height := p.GetRect()
	w, h := min(width, 80), min(height, 22)
	p.SetRect(x+(width-w)/2, y+(height-h)/2, w, h)
	p.Flex.Draw(screen)
	p.shown = true
}

// PrepareTelemetry is called after connection argument handling and before Run.
// Its cleanup must be called when Run returns, including on errors.
func PrepareTelemetry(pages *tview.Pages, endpoint, version, preferencePath string, disabled bool, mode telemetry.StartupMode, distribution telemetry.Distribution) func() {
	stop := func() {}
	restore := func() {}
	if disabled || telemetry.Disabled() || !telemetry.ValidEndpoint(endpoint) {
		return stop
	}
	enabled, decided, err := telemetry.LoadConsent(preferencePath, endpoint)
	if err != nil || (decided && !enabled) {
		return stop // Fail closed on unreadable or malformed consent.
	}
	start := func() {
		stop = usage.Start(app.App.Context(), endpoint, version, mode, distribution)
		if initialTelemetryEngine != "" {
			usage.Connected(initialTelemetryEngine, initialTelemetryReadOnly)
		}
	}
	if enabled {
		// No requests if terminal initialization fails. Draw callbacks hold the
		// tview application lock: do NOT call SetFocus/GetFocus from here.
		app.App.SetBeforeDrawFunc(func(_ tcell.Screen) bool {
			app.App.SetBeforeDrawFunc(nil)
			start()
			return false
		})
	} else {
		const page = "telemetry-consent"
		previousCapture := app.App.GetInputCapture()
		closeModal := func() {
			pages.RemovePage(page)
			app.App.SetInputCapture(previousCapture)
			app.App.SetFocus(pages)
		}
		restore = closeModal
		var modal *telemetryPrompt
		finish := func(consent, exiting bool) {
			if !modal.shown {
				if exiting {
					app.App.Stop()
				}
				return // No preference or send before the first disclosure draw.
			}
			if err := telemetry.SaveConsent(preferencePath, endpoint, consent); err != nil {
				modal.text.SetText("Could not save your telemetry preference. Telemetry remains off. " +
					"Use --no-telemetry to suppress this prompt next time.")
				modal.form.Clear(true).AddButton("OK", closeModal).SetCancelFunc(closeModal)
				app.App.SetFocus(modal)
				if exiting {
					app.App.Stop()
				}
				return
			}
			closeModal()
			if exiting {
				app.App.Stop()
			} else if consent {
				start()
			}
		}
		modal = newTelemetryModal(endpoint, func(consent bool) { finish(consent, false) })
		pages.AddPage(page, modal, true, true)
		app.App.SetFocus(modal)
		app.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyCtrlC {
				finish(true, true)
				return nil
			}
			// Async database loads must not redirect consent keystrokes to SQL.
			if !modal.HasFocus() {
				app.App.SetFocus(modal)
			}
			return event
		})
	}
	return func() {
		app.App.SetBeforeDrawFunc(nil)
		stop()
		restore()
	}
}
