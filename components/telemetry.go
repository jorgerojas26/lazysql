package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/telemetry"
)

type telemetryPrompt struct {
	*tview.Flex
	text *tview.TextView
	form *tview.Form
}

func newTelemetryModal(endpoint string, done func(bool)) *telemetryPrompt {
	text := tview.NewTextView().SetWordWrap(true).SetText(fmt.Sprintf(
		"Share anonymous usage counts? (optional)\n\n"+
			"Sends a launch count, a heartbeat every minute while open, and the public release version to:\n%s\n\n"+
			"No user, install or session IDs; no SQL, database details or device metadata. "+
			"The host sees your IP during delivery; the collector stores only aggregate counters, with a 30-day retention target.\n\n"+
			"No is the default. Disable later with --no-telemetry or telemetry.toml. "+
			"Privacy details: docs/telemetry.md in the project repository.\n\n"+
			"Use Up/Down to scroll, Tab to choose, Enter to confirm, Esc to decline.", endpoint))
	form := tview.NewForm().AddButton("No", func() { done(false) }).AddButton("Yes", func() { done(true) }).
		SetButtonsAlign(tview.AlignCenter).SetCancelFunc(func() { done(false) })
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp, tcell.KeyDown, tcell.KeyPgUp, tcell.KeyPgDn, tcell.KeyHome, tcell.KeyEnd:
			text.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})
	flex := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(text, 0, 1, false).AddItem(form, 3, 0, true)
	flex.SetBorder(true).SetTitle(" Optional telemetry ").SetBorderPadding(1, 1, 1, 1)
	return &telemetryPrompt{Flex: flex, text: text, form: form}
}

// PrepareTelemetry is called after connection argument handling and before Run.
// Its cleanup must be called when Run returns, including on errors.
func PrepareTelemetry(pages *tview.Pages, endpoint, version, preferencePath string, disabled bool) func() {
	stop := func() {}
	if disabled || telemetry.Disabled() || !telemetry.ValidEndpoint(endpoint) {
		return stop
	}
	enabled, decided, err := telemetry.LoadConsent(preferencePath, endpoint)
	if err != nil || (decided && !enabled) {
		return stop // Fail closed on unreadable or malformed consent.
	}
	start := func() { stop = telemetry.Start(app.App.Context(), endpoint, version) }
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
		var modal *telemetryPrompt
		modal = newTelemetryModal(endpoint, func(consent bool) {
			if err := telemetry.SaveConsent(preferencePath, endpoint, consent); err != nil {
				modal.text.SetText("Could not save your telemetry preference. Telemetry remains off. " +
					"Use --no-telemetry to suppress this prompt next time.")
				modal.form.Clear(true).AddButton("OK", closeModal).SetCancelFunc(closeModal)
				app.App.SetFocus(modal)
				return
			}
			closeModal()
			if consent {
				start()
			}
		})
		pages.AddPage(page, modal, true, true)
		app.App.SetFocus(modal)
		app.App.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyCtrlC {
				app.App.Stop()
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
	}
}
