package components

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/models"
)

func TestAIChatHideRestoresWorkspaceNavigation(t *testing.T) {
	for _, scenario := range []string{"tree", "records", "records with background editor", "editor results", "filter"} {
		t.Run(scenario, func(t *testing.T) {
			setAIConfigForTest(t, true)
			App.Config().ConfirmOnQuit = true
			previousApplication, previousPages := App.Application, mainPages
			App.Application = tview.NewApplication()
			mainPages = tview.NewPages()
			mainPages.SetInputCapture(captureAIShortcut)
			mainPages.AddPage(pageNameConnections, tview.NewTextView().SetText("connections"), true, false)
			home := NewHomePage(models.Connection{Name: "ai-navigation", URL: "mysql://localhost/database", DBName: "database"}, &aiSchemaDriver{})
			mainPages.AddAndSwitchToPage("workspace", home, true)
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			App.SetScreen(screen)
			done := make(chan error, 1)
			go func() { done <- App.Run(mainPages, filepath.Join(t.TempDir(), "runtime.toml")) }()
			App.QueueUpdate(func() {})
			t.Cleanup(func() {
				App.QueueUpdate(func() {
					closeQuitConfirmation()
					if home.aiChat != nil {
						home.aiChat.close()
					}
					if home.Tree.loadCancel != nil {
						home.Tree.loadCancel()
					}
				})
				App.Application.Stop()
				if err := <-done; err != nil {
					t.Error(err)
				}
				App.Application, mainPages = previousApplication, previousPages
			})
			wait := func(label string, condition func() bool) {
				t.Helper()
				deadline := time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) {
					ready := false
					App.QueueUpdateDraw(func() { ready = condition() })
					if ready {
						return
					}
					time.Sleep(time.Millisecond)
				}
				t.Fatalf("%s: workspace keybinding did not work", label)
			}
			press := func(code tcell.Key, character rune) {
				App.QueueEvent(tcell.NewEventKey(code, character, tcell.ModNone))
			}
			var editor, records *ResultsTable
			var originFocus tview.Primitive
			var originTab *Tab
			var originWrapper string
			App.QueueUpdateDraw(func() {
				if scenario == "records with background editor" || scenario == "editor results" {
					editor = NewResultsTable(&home.ListOfDBChanges, home.Tree, home.DBDriver, home, "", "", false)
					editor.SetDatabaseName("database")
					editor.WithEditor()
					editor.Editor.SetText("SELECT id FROM public.orders;", false)
					editor.EditorPages.SwitchToPage(pageNameTableEditorTable)
					editor.SetIsFiltering(false)
					home.TabbedPane.AppendTab(tabNameEditor, editor, tabNameEditor)
				}
				if scenario == "records" || scenario == "records with background editor" || scenario == "filter" {
					records = NewResultsTable(&home.ListOfDBChanges, home.Tree, home.DBDriver, home, "", "", false).WithFilter()
					records.SetDatabaseName("database")
					home.TabbedPane.AppendTab("orders", records, "orders")
					if scenario == "filter" {
						records.SetIsFiltering(true)
						records.Filter.Input.SetText("id > 0")
					}
				}
				if scenario == "tree" {
					home.focusLeftWrapper()
				} else {
					home.focusRightWrapper()
				}
				originFocus = App.GetFocus()
				originTab = home.TabbedPane.GetCurrentTab()
				originWrapper = home.FocusedWrapper
			})
			for _, closeKey := range []tcell.Key{tcell.KeyEscape, tcell.KeyCtrlG} {
				press(tcell.KeyCtrlG, 0)
				wait("open chat", func() bool { return home.aiChat != nil && home.aiChat.visible })
				press(closeKey, 0)
				wait("hide chat", func() bool { return !home.aiChat.visible })
				App.QueueUpdate(func() {
					if App.GetFocus() != originFocus {
						t.Errorf("AI restored %T instead of %T", App.GetFocus(), originFocus)
					}
					if home.TabbedPane.GetCurrentTab() != originTab {
						t.Error("opening AI changed active workspace tab")
					}
					if home.FocusedWrapper != originWrapper {
						t.Error("opening AI changed focused workspace wrapper")
					}
					if editor != nil && (editor.GetIsEditing() || editor.GetIsFiltering()) {
						t.Error("AI left editor editing/filtering mode enabled")
					}
					if records != nil && (records.GetIsEditing() || records.GetIsFiltering() != (scenario == "filter")) {
						t.Error("AI changed records input mode")
					}
				})
				if scenario == "filter" {
					press(tcell.KeyRune, 'q')
					press(tcell.KeyBackspace2, 0)
					wait("filter typing", func() bool { return records.Filter.Input.GetText() == "id > 0" })
					continue
				}
				press(tcell.KeyRune, '?')
				wait("help shortcut", func() bool { return mainPages.HasPage(pageNameHelp) })
				App.QueueUpdateDraw(func() { mainPages.RemovePage(pageNameHelp); App.SetFocus(originFocus) })
				press(tcell.KeyRune, 'q')
				wait("q shortcut", func() bool { return quitConfirmationVisible() })
				App.QueueUpdateDraw(func() { closeQuitConfirmation(); App.SetFocus(originFocus) })
				press(tcell.KeyBackspace2, 0)
				wait("Backspace shortcut", func() bool { name, _ := mainPages.GetFrontPage(); return name == pageNameConnections })
				App.QueueUpdateDraw(func() { mainPages.SwitchToPage("workspace"); App.SetFocus(originFocus) })
			}
		})
	}
}
