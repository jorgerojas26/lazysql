package components

import (
	"runtime"
	"strings"
	"testing"
	"time"

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

func setupUpdateLayout(t *testing.T, startHome bool) (*tview.Pages, *updateUI, *Home, *tview.Box) {
	t.Helper()
	setupUpdateUI(t)
	oldRequest := app.App.OnUpdateRequest
	t.Cleanup(func() { app.App.OnUpdateRequest = oldRequest })
	t.Setenv("LAZYSQL_NO_UPDATE_CHECK", "1")
	mainPages.RemovePage("input")
	picker := tview.NewBox()
	mainPages.AddPage(pageNameConnections, picker, true, true)
	right := tview.NewFlex()
	right.SetBorder(true).SetTitle("Results")
	home := &Home{Flex: tview.NewFlex(), RightWrapper: right}
	home.AddItem(right, 0, 1, true)
	mainPages.AddPage("home", home, true, false)
	if startHome {
		mainPages.SwitchToPage("home")
	}
	root := WithUpdates(mainPages, "1.0.0")
	_, primitive := root.GetFrontPage()
	return root, primitive.(*updateUI), home, picker
}

func updateTestScreen(t *testing.T) tcell.SimulationScreen {
	t.Helper()
	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	screen.SetSize(120, 24)
	t.Cleanup(screen.Fini)
	return screen
}

func updateScreenRow(screen tcell.Screen, y int) string {
	width, _ := screen.Size()
	var row strings.Builder
	for x := 0; x < width; x++ {
		ch, _, _, _ := screen.GetContent(x, y)
		row.WriteRune(ch)
	}
	return row.String()
}

func TestUpdateNoticeUsesHomeBottomBorder(t *testing.T) {
	for _, startHome := range []bool{false, true} {
		t.Run(map[bool]string{false: "picker startup", true: "home startup"}[startHome], func(t *testing.T) {
			root, u, home, picker := setupUpdateLayout(t, startHome)
			screen := updateTestScreen(t)
			root.SetRect(0, 0, 120, 24)
			if !startHome {
				root.Draw(screen)
				_, _, _, height := picker.GetRect()
				if height != 23 || !strings.Contains(updateScreenRow(screen, 23), "LazySQL 1.0.0 | <F10>: updates") {
					t.Fatalf("connection picker height = %d, footer = %q", height, updateScreenRow(screen, 23))
				}
				mainPages.SwitchToPage("home")
			}
			u.status("LazySQL v1.1.0 available")
			root.Draw(screen)
			_, _, _, height := home.GetRect()
			if height != 24 {
				t.Fatalf("home height = %d, want full 24 rows", height)
			}
			_, _, _, footerHeight := u.footer.GetRect()
			if footerHeight != 0 {
				t.Fatal("home still reserves a footer row")
			}
			bottom := updateScreenRow(screen, 23)
			if !strings.HasSuffix(bottom, " LazySQL 1.0.0 "+string(tview.Borders.BottomRight)) {
				t.Fatalf("current version missing from bottom-right edge: %q", bottom)
			}
			if strings.Contains(bottom, "available") || strings.Contains(bottom, "updates") {
				t.Fatal("home border contains more than the running version")
			}
			if !strings.Contains(updateScreenRow(screen, 0), "Results") {
				t.Fatal("home title was overwritten")
			}
			_, _, innerWidth, innerHeight := home.RightWrapper.GetInnerRect()
			if innerWidth != 118 || innerHeight != 22 {
				t.Fatalf("notice changed content dimensions to %dx%d", innerWidth, innerHeight)
			}
			mainPages.AddPage("overlay", tview.NewModal().SetText("Help"), true, true)
			root.Draw(screen)
			_, _, _, height = home.GetRect()
			if height != 24 {
				t.Fatal("opening a dialog restored the home footer")
			}
			mainPages.RemovePage("overlay")
			mainPages.SwitchToPage(pageNameConnections)
			root.Draw(screen)
			if !strings.Contains(updateScreenRow(screen, 23), "v1.1.0 available") {
				t.Fatal("returning to the picker lost the update notice")
			}
		})
	}
}

func TestUpgradeToastDoesNotStealFocusOrReserveSpace(t *testing.T) {
	root, u, home, _ := setupUpdateLayout(t, true)
	screen := updateTestScreen(t)
	root.SetRect(0, 0, 120, 24)
	app.App.SetFocus(home)
	previous := app.App.GetFocus()
	u.status("LazySQL v1.1.0 available")
	u.showUpgradeToast()
	root.Draw(screen)
	if app.App.GetFocus() != previous || mainPages.HasPage(pageNameUpdate) {
		t.Fatal("upgrade toast interrupted database work")
	}
	if !strings.Contains(updateScreenRow(screen, 21), "LazySQL v1.1.0 available | <F10>: updates") {
		t.Fatalf("upgrade toast is missing its version or shortcut: %q", updateScreenRow(screen, 21))
	}
	_, _, _, height := home.GetRect()
	if height != 24 {
		t.Fatal("upgrade toast reserved screen space")
	}
	if !strings.Contains(updateScreenRow(screen, 23), "LazySQL 1.0.0") {
		t.Fatal("toast replaced the running version")
	}
	mainPages.AddPage("overlay", tview.NewModal().SetText("Help"), true, true)
	root.Draw(screen)
	if strings.Contains(updateScreenRow(screen, 21), "v1.1.0 available") {
		t.Fatal("toast was drawn over a dialog")
	}
	mainPages.RemovePage("overlay")
	u.toastUntil = time.Now().Add(-time.Second)
	root.Draw(screen)
	if strings.Contains(updateScreenRow(screen, 21), "v1.1.0 available") {
		t.Fatal("expired toast is still visible")
	}
}
