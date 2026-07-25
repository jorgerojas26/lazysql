package components

import (
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

var (
	mainPages *tview.Pages
	// quitConfirmationModal holds the currently visible quit dialog, if any, so
	// its focus can be re-asserted after an async callback steals it.
	quitConfirmationModal *ConfirmationModal
)

func showQuitConfirmation() {
	if !app.App.Config().ConfirmOnQuit {
		app.App.Stop()
		return
	}
	if mainPages == nil {
		app.App.Stop()
		return
	}

	if mainPages.HasPage(pageNameConfirmation) {
		// The dialog is already up. A background task (e.g. a table finishing
		// loading) may have stolen focus from it, leaving it undismissable, so
		// re-assert focus instead of silently doing nothing.
		if quitConfirmationModal != nil {
			app.App.SetFocus(quitConfirmationModal)
		}
		return
	}

	confirmationModal := NewConfirmationModal("Exit LazySQL?")
	quitConfirmationModal = confirmationModal
	confirmationModal.SetDoneFunc(func(_ int, buttonLabel string) {
		mainPages.RemovePage(pageNameConfirmation)
		quitConfirmationModal = nil
		if buttonLabel == confirmationYes {
			app.App.Stop()
		}
	})
	mainPages.AddPage(pageNameConfirmation, confirmationModal, true, true)
	app.App.SetFocus(confirmationModal)
}

func closeQuitConfirmation() {
	if mainPages != nil {
		mainPages.RemovePage(pageNameConfirmation)
	}
	quitConfirmationModal = nil
}

// quitConfirmationVisible reports whether the quit dialog is currently shown.
// Async completion callbacks use it to avoid moving focus away from the dialog.
func quitConfirmationVisible() bool {
	return mainPages != nil && mainPages.HasPage(pageNameConfirmation)
}

// keepQuitConfirmationFocused re-asserts focus on the quit dialog when it is
// visible. Async completion callbacks that call SetFocus (table loads, filter
// changes, etc.) must call this last so they cannot strand the modal on-screen
// but unfocused, which would make it impossible to dismiss.
func keepQuitConfirmationFocused() {
	if quitConfirmationVisible() && quitConfirmationModal != nil {
		app.App.SetFocus(quitConfirmationModal)
	}
}

func MainPages() *tview.Pages {
	mainPages = tview.NewPages()
	mainPages.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	mainPages.AddPage(pageNameConnections, NewConnectionPages().Grid, true, true)

	// Show quit confirmation on Ctrl+C / OS interrupt.
	app.App.SetOnQuitRequest(showQuitConfirmation)

	return mainPages
}
