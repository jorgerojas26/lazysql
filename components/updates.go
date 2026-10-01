package components

import (
	"fmt"
	"os"
	"runtime"

	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/internal/updater"
)

const pageNameUpdate = "application-update"

// updateUI state is owned exclusively by the tview event loop.
type updateUI struct {
	client        *updater.Client
	current       string
	footer        *tview.TextView
	release       *updater.Release
	busy          bool
	installed     bool
	modal         *tview.Modal
	previousFocus tview.Primitive
}

// WithUpdates adds a non-disruptive status line in both the picker and editor.
// Automatic checks never open a dialog or take focus from database work.
func WithUpdates(pages *tview.Pages, version string) *tview.Pages {
	u := &updateUI{client: updater.New(), current: version, footer: tview.NewTextView()}
	u.footer.SetTextColor(app.Styles.TertiaryTextColor).SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	u.status("LazySQL " + version)
	app.App.OnUpdateRequest = u.open
	root := tview.NewPages()
	root.AddPage("application", tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true).AddItem(u.footer, 1, 0, false), true, true)
	if !app.App.Config().DisableUpdateCheck && os.Getenv("LAZYSQL_NO_UPDATE_CHECK") == "" && updater.ValidVersion(version) {
		u.check(false)
	}
	return root
}

func (u *updateUI) status(message string) {
	key := "Updates"
	for _, bind := range app.Keymaps.Global {
		if bind.Cmd == commands.CheckForUpdates {
			key = bind.Key.String() + ": updates"
			break
		}
	}
	u.footer.SetText(fmt.Sprintf(" %s | %s", message, key))
}

func (u *updateUI) close() {
	mainPages.RemovePage(pageNameUpdate)
	u.modal = nil
	if u.previousFocus != nil {
		app.App.SetFocus(u.previousFocus)
	}
	keepQuitConfirmationFocused()
}

func (u *updateUI) dialog(text string, buttons []string, done func(string)) {
	if u.modal == nil {
		u.previousFocus = app.App.GetFocus()
	}
	modal := tview.NewModal().SetText(text).AddButtons(buttons)
	modal.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor).SetTextColor(app.Styles.PrimaryTextColor)
	modal.SetDoneFunc(func(_ int, label string) {
		if label == "" || label == "Close" || label == "Later" {
			u.close()
			return
		}
		done(label)
	})
	u.modal = modal
	mainPages.AddPage(pageNameUpdate, modal, true, true)
	app.App.SetFocus(modal)
	keepQuitConfirmationFocused()
}

func (u *updateUI) open() {
	if quitConfirmationVisible() {
		return
	}
	if u.installed {
		u.dialog("Upgrade installed. Restart LazySQL when ready to use the new version. Your current session has not been interrupted.", []string{"Close"}, nil)
	} else if u.busy {
		u.dialog("An update operation is in progress. You can close this dialog and keep working.", []string{"Close"}, nil)
	} else if u.release != nil {
		u.offer()
	} else {
		u.dialog("Checking for updates…", []string{"Close"}, nil)
		u.check(true)
	}
}

func (u *updateUI) check(manual bool) {
	if u.busy {
		return
	}
	u.busy = true
	ctx := app.App.Context()
	go func() {
		release, err := u.client.Check(ctx, u.current, manual)
		if ctx.Err() != nil {
			return
		}
		app.App.QueueUpdateDraw(func() {
			u.busy = false
			u.release = release
			if release != nil {
				u.status("LazySQL " + release.Tag + " available")
			}
			if u.modal == nil {
				return
			}
			if err != nil {
				u.dialog("Could not check for updates: "+tview.Escape(err.Error()), []string{"Close"}, nil)
			} else if release == nil {
				u.dialog("LazySQL "+u.current+" is up to date.", []string{"Close"}, nil)
			} else {
				u.offer()
			}
		})
	}()
}

func (u *updateUI) offer() {
	target, err := updater.Executable()
	if err != nil {
		u.dialog("Cannot locate the running executable: "+tview.Escape(err.Error()), []string{"Close"}, nil)
		return
	}
	text := fmt.Sprintf("LazySQL %s is available (running %s).\n\nRelease notes: https://github.com/jorgerojas26/lazysql/releases/tag/%s", u.release.Tag, u.current, u.release.Tag)
	if guidance := updater.InstallGuidance(target, runtime.GOOS); guidance != "" {
		u.dialog(text+"\n\n"+guidance, []string{"Close"}, nil)
		return
	}
	u.dialog(text+"\n\nUpgrade will download and verify the official release, then replace:\n"+tview.Escape(target)+"\n\nRestart manually when ready. Upgrade now?", []string{"Later", "Upgrade"}, func(label string) {
		if label == "Upgrade" {
			u.install(target)
		}
	})
	// Keep the non-destructive action selected by default.
	u.modal.SetFocus(0)
}

func (u *updateUI) install(target string) {
	u.busy = true
	release := *u.release
	u.status("Downloading and verifying " + release.Tag + "…")
	u.dialog("Installing update… You can close this dialog and continue working.", []string{"Close"}, nil)
	ctx := app.App.Context()
	done := app.App.Register()
	go func() {
		err := u.client.Install(ctx, release, target)
		// Stop waits for file operations, never for UI callbacks.
		done()
		if ctx.Err() != nil {
			return
		}
		app.App.QueueUpdateDraw(func() {
			u.busy = false
			if err != nil {
				u.status("Upgrade failed; open updates to retry")
				if u.modal != nil {
					u.dialog("Upgrade failed: "+tview.Escape(err.Error()), []string{"Close"}, nil)
				}
				return
			}
			u.installed = true
			u.status("Installed " + release.Tag + "; restart LazySQL when ready")
			if u.modal != nil {
				u.open()
			}
		})
	}()
}
