package components

import (
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

var mainPages *tview.Pages

func showQuitConfirmation() {
	if mainPages == nil {
		app.App.Stop()
		return
	}

	if mainPages.HasPage(pageNameConfirmation) {
		return
	}

	confirmationModal := NewConfirmationModal("Exit LazySQL?")
	confirmationModal.SetDoneFunc(func(_ int, buttonLabel string) {
		mainPages.RemovePage(pageNameConfirmation)
		if buttonLabel == confirmationYes {
			app.App.Stop()
		}
	})
	mainPages.AddPage(pageNameConfirmation, confirmationModal, true, true)
	app.App.SetFocus(confirmationModal)
}

func MainPages() *tview.Pages {
	mainPages = tview.NewPages()
	mainPages.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	mainPages.AddPage(pageNameConnections, NewConnectionPages().Grid, true, true)

	// Show quit confirmation on Ctrl+C / OS interrupt.
	app.App.SetOnQuitRequest(showQuitConfirmation)

	return mainPages
}
