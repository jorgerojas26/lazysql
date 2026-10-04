package components

import (
	"runtime"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/internal/updater"
)

func setupUpdateUI(t *testing.T) *updateUI {
	t.Helper()
	oldPages, oldFocus := mainPages, app.App.GetFocus()
	oldQuit := quitConfirmationModal
	mainPages = tview.NewPages()
	quitConfirmationModal = nil
	input := tview.NewInputField()
	mainPages.AddPage("input", input, true, true)
	app.App.SetFocus(input)
	t.Cleanup(func() {
		mainPages = oldPages
		quitConfirmationModal = oldQuit
		app.App.SetFocus(oldFocus)
	})
	return &updateUI{current: "1.0.0", footer: tview.NewTextView(), release: &updater.Release{Tag: "v1.1.0"}}
}

func TestUpgradeRequiresConfirmation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows provides manual instructions")
	}
	target, err := updater.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if updater.InstallGuidance(target, runtime.GOOS) != "" {
		t.Skip("test binary is in a managed path")
	}
	u := setupUpdateUI(t)
	previous := app.App.GetFocus()
	u.open()
	if !mainPages.HasPage(pageNameUpdate) {
		t.Fatal("upgrade dialog not opened")
	}
	// Enter on the initial selection must dismiss, never upgrade.
	u.modal.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if u.busy || mainPages.HasPage(pageNameUpdate) {
		t.Fatal("default action was not Later")
	}
	if app.App.GetFocus() != previous {
		t.Fatal("focus was not restored")
	}
}

func TestUpdateProgressCanBeDismissed(t *testing.T) {
	u := setupUpdateUI(t)
	u.busy = true
	u.open()
	u.modal.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if u.modal != nil || mainPages.HasPage(pageNameUpdate) {
		t.Fatal("Escape did not dismiss progress")
	}
	if !u.busy {
		t.Fatal("closing dialog changed operation state")
	}
}

func TestUpdateStatusDoesNotStealFocus(t *testing.T) {
	u := setupUpdateUI(t)
	previous := app.App.GetFocus()
	u.status("LazySQL v1.1.0 available")
	if app.App.GetFocus() != previous || mainPages.HasPage(pageNameUpdate) {
		t.Fatal("notice interrupted database work")
	}
	if !strings.Contains(u.footer.GetText(false), "v1.1.0 available") {
		t.Fatal("missing update notice")
	}
	if app.Keymaps.Resolve(tcell.NewEventKey(tcell.KeyF10, 0, tcell.ModNone)) != commands.CheckForUpdates {
		t.Fatal("missing global shortcut")
	}
}

func TestInstalledUpdateCannotBeInstalledAgain(t *testing.T) {
	u := setupUpdateUI(t)
	u.installed = true
	u.open()
	if u.busy {
		t.Fatal("installed release started another operation")
	}
	u.modal.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(p tview.Primitive) { app.App.SetFocus(p) })
	if u.modal != nil {
		t.Fatal("restart notice was not dismissed")
	}
}
