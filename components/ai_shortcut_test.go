package components

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
)

func TestAIGlobalShortcutOpensEditorFromWorkspace(t *testing.T) {
	for _, scenario := range []string{"tree", "highlighted db", "single db", "records", "filter", "reuse draft", "existing editor", "empty editor context"} {
		t.Run(scenario, func(t *testing.T) {
			setAIConfigForTest(t, true)
			previousApplication, previousPages := App.Application, mainPages
			App.Application = tview.NewApplication()
			mainPages = tview.NewPages()
			mainPages.SetInputCapture(captureAIShortcut)
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			App.SetScreen(screen)
			driver := &aiSchemaDriver{}
			tree := &Tree{TreeView: tview.NewTreeView().SetRoot(tview.NewTreeNode("database")), Filter: tview.NewInputField(), DBDriver: driver, Schemas: []string{"public"}, state: &TreeState{selectedDatabase: "tree-db", selectedTable: "public.orders"}}
			if scenario == "highlighted db" || scenario == "single db" {
				tree.state.selectedDatabase = ""
				node := tview.NewTreeNode("database").SetReference("highlighted-db")
				tree.GetRoot().AddChild(node)
				if scenario == "highlighted db" {
					tree.SetCurrentNode(node)
				}
			}
			home := &Home{Flex: tview.NewFlex(), Tree: tree, TabbedPane: NewTabbedPane(), DBDriver: driver}
			home.Flex.AddItem(tree, 20, 0, true).AddItem(tree.Filter, 20, 0, false).AddItem(home.TabbedPane.Pages, 0, 1, false)
			var draft, records *ResultsTable
			if scenario == "reuse draft" || scenario == "existing editor" || scenario == "empty editor context" {
				draft = NewResultsTable(&home.ListOfDBChanges, tree, driver, home, "", "", false)
				draft.SetDatabaseName("records-db")
				if scenario == "empty editor context" {
					draft.SetDatabaseName("")
				}
				draft.WithEditor()
				draft.Editor.SetText("SELECT 'existing draft';", true)
				draft.Editor.selecting = true
				draft.Editor.selCX = 0
				draft.Editor.selCY = 0
				draft.Editor.cx = len(draft.Editor.lines[0])
				draft.Editor.cy = 0
				home.TabbedPane.AppendTab(tabNameEditor, draft, tabNameEditor)
			}
			if scenario != "tree" && scenario != "highlighted db" && scenario != "single db" && scenario != "existing editor" && scenario != "empty editor context" {
				records = NewResultsTable(&home.ListOfDBChanges, tree, driver, home, "", "", false).WithFilter()
				records.SetDatabaseName("records-db")
				home.TabbedPane.AppendTab("orders", records, "records")
			}
			mainPages.AddPage("home", home, true, true)
			done := make(chan error, 1)
			go func() { done <- App.Run(mainPages, filepath.Join(t.TempDir(), "runtime.toml")) }()
			App.QueueUpdate(func() {})
			t.Cleanup(func() {
				App.QueueUpdate(func() {
					for tab := home.TabbedPane.state.FirstTab; tab != nil; tab = tab.NextTab {
						if table, ok := tab.Content.(*ResultsTable); ok && table.aiSession != nil {
							table.aiSession.close()
						}
					}
				})
				App.Application.Stop()
				if err := <-done; err != nil {
					t.Error(err)
				}
				App.Application, mainPages = previousApplication, previousPages
			})
			var source *ResultsTable
			App.QueueUpdateDraw(func() {
				switch scenario {
				case "tree", "highlighted db", "single db":
					App.SetFocus(tree)
				case "filter":
					App.SetFocus(records.Filter.Input)
				case "existing editor", "empty editor context":
					App.SetFocus(draft.Editor)
				default:
					App.SetFocus(records)
				}
				mainPages.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModNone), func(p tview.Primitive) { App.SetFocus(p) })
				tab := home.TabbedPane.GetCurrentTab()
				if tab == nil {
					t.Error("global shortcut did not create an editor")
					return
				}
				source = tab.Content.(*ResultsTable)
				if source.Editor == nil || source.aiSession == nil {
					t.Error("global shortcut did not open AI prompt")
					return
				}
				expected := "records-db"
				if scenario == "tree" || scenario == "empty editor context" {
					expected = "tree-db"
				}
				if scenario == "highlighted db" || scenario == "single db" {
					expected = "highlighted-db"
				}
				if source.GetDatabaseName() != expected {
					t.Errorf("wrong database: %q", source.GetDatabaseName())
				}
				if App.GetFocus() != source.aiSession.input {
					t.Error("prompt input is not focused")
				}
				if draft != nil && source != draft {
					t.Error("existing SQL editor was not reused")
				}
				if scenario == "reuse draft" && source.aiSession.selection.text != "" {
					t.Error("background selection was treated as replacement request")
				}
			})
			if source == nil {
				return
			}
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				ready := false
				App.QueueUpdate(func() {
					ready = source.aiSession != nil && !source.aiSession.busy && (scenario == "reuse draft" || scenario == "existing editor" || scenario == "empty editor context" || len(source.Editor.completer.tables) > 0)
				})
				if ready {
					break
				}
				time.Sleep(time.Millisecond)
			}
			App.QueueUpdateDraw(func() {
				session := source.aiSession
				if session == nil || session.busy {
					t.Error("AI prompt did not load")
					return
				}
				session.result.SQL = "SELECT id FROM public.orders;"
				before := home.TabbedPane.GetLength()
				if err := session.accept(); err != nil {
					t.Error(err)
					return
				}
				if draft == nil {
					if home.TabbedPane.GetLength() != before || source.Editor.GetText() != "SELECT id FROM public.orders;" {
						t.Error("empty editor was not filled in place")
					}
					if App.GetFocus() != source.Editor {
						t.Error("accepted SQL did not focus editor")
					}
				} else if scenario == "reuse draft" {
					if source.Editor.GetText() != "SELECT 'existing draft';" || home.TabbedPane.GetLength() != before+1 {
						t.Error("generating from records overwrote existing draft")
					}
				}
				if driver.executions.Load() != 0 {
					t.Error("global shortcut executed SQL")
				}
			})
		})
	}
}

func TestAIGlobalShortcutRespectsDisabledStateModalsAndRemapping(t *testing.T) {
	previousPages := mainPages
	previousKeymaps := app.Keymaps
	t.Cleanup(func() { mainPages = previousPages; app.Keymaps = previousKeymaps })
	setAIConfigForTest(t, false)
	event := tcell.NewEventKey(tcell.KeyCtrlG, 0, tcell.ModNone)
	mainPages = tview.NewPages().AddPage("home", &Home{Flex: tview.NewFlex()}, true, true)
	if captureAIShortcut(event) != event {
		t.Fatal("disabled AI intercepted shortcut")
	}
	setAIConfigForTest(t, true)
	mainPages = tview.NewPages().AddPage("dialog", tview.NewModal(), true, true)
	if captureAIShortcut(event) != event {
		t.Fatal("AI shortcut replaced an unrelated modal")
	}
	app.Keymaps.Global = append(app.Map(nil), app.Keymaps.Global...)
	for index, bind := range app.Keymaps.Global {
		if bind.Cmd == commands.GenerateSQL {
			app.Keymaps.Global[index].Key.Code = tcell.KeyCtrlY
		}
	}
	if app.Keymaps.Resolve(event) == commands.GenerateSQL || app.Keymaps.Resolve(tcell.NewEventKey(tcell.KeyCtrlY, 0, tcell.ModNone)) != commands.GenerateSQL {
		t.Fatal("AI shortcut is not globally remappable")
	}
	if app.Keymaps.Group(app.EditorGroup).Resolve(event) == commands.GenerateSQL {
		t.Fatal("AI shortcut still belongs to editor group")
	}
}

func TestAIDatabaseDoesNotGuessAcrossDatabases(t *testing.T) {
	tree := &Tree{TreeView: tview.NewTreeView().SetRoot(tview.NewTreeNode("root")), state: &TreeState{}}
	tree.SetCurrentNode(tree.GetRoot())
	tree.GetRoot().AddChild(tview.NewTreeNode("one").SetReference("one"))
	tree.GetRoot().AddChild(tview.NewTreeNode("two").SetReference("two"))
	home := &Home{Tree: tree}
	if database := home.aiDatabase(); database != "" {
		t.Fatalf("guessed database: %q", database)
	}
	home.ConnectionURL = "mysql://localhost/configured"
	if database := home.aiDatabase(); database != "configured" {
		t.Fatalf("connection database ignored: %q", database)
	}
	tree.SetCurrentNode(tree.GetRoot().GetChildren()[1])
	if database := home.aiDatabase(); database != "two" {
		t.Fatalf("highlighted database ignored: %q", database)
	}
	tree.state.selectedDatabase = "selected"
	if database := home.aiDatabase(); database != "selected" {
		t.Fatalf("active database ignored: %q", database)
	}
}
