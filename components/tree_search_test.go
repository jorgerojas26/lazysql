package components

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func newTreeSearchTestTree() *Tree {
	tree := NewTree("", nil, nil)
	tree.SetFocusFunc(nil)
	tree.GetRoot().SetChildren(buildTwoDatabaseTreeWithSameTableName().GetRoot().GetChildren())
	tree.CollapseAll()
	return tree
}

func TestTreeSearchFilterUsesLatestQuery(t *testing.T) {
	tree := newTreeSearchTestTree()
	for range 20 {
		for _, query := range []string{"u", "us", "use", "user", "users", "dbA.users", "dbB.users"} {
			tree.Filter.SetText(query)
			wantCount := 2
			if query == "dbA.users" || query == "dbB.users" {
				wantCount = 1
			}
			found := tree.state.searchFoundNodes
			if len(found) != wantCount {
				t.Fatalf("query %q: got %d results, want %d", query, len(found), wantCount)
			}
			unique := make(map[*tview.TreeNode]bool)
			for _, node := range found {
				if unique[node] {
					t.Fatalf("query %q: duplicate result %s", query, node.GetReference())
				}
				unique[node] = true
			}
		}
	}

	found := tree.state.searchFoundNodes
	if found[0].GetReference() != "dbB.tables.users" {
		t.Fatalf("expected only dbB.tables.users for the latest query, got %s", found[0].GetReference())
	}
	tree.Filter.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(tview.Primitive) {})
	if tree.GetCurrentNode() != found[0] || tree.state.currentFocusFoundNode != found[0] {
		t.Fatal("search focus does not point to the latest query's result")
	}
	if got := tree.FoundNodeCountInput.GetText(); got != "[1/1]" {
		t.Fatalf("counter = %q, want [1/1]", got)
	}
}

func TestTreeSearchNavigationExpandsAncestors(t *testing.T) {
	for _, direction := range []string{"next", "previous"} {
		t.Run(direction, func(t *testing.T) {
			tree := newTreeSearchTestTree()
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(screen.Fini)
			tree.SetRect(0, 0, 80, 20)
			tree.Filter.SetText("users")
			tree.Filter.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(tview.Primitive) {})
			found := tree.state.searchFoundNodes
			if len(found) != 2 {
				t.Fatalf("expected two users tables, got %d results", len(found))
			}
			tree.Draw(screen)

			for _, index := range []int{1, 0, 1} {
				tree.CollapseAll()
				key := 'n'
				if direction == "previous" {
					key = 'N'
				}
				tree.InputHandler()(tcell.NewEventKey(tcell.KeyRune, key, 0), func(tview.Primitive) {})
				target := found[index]
				tree.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
					if node == target && parent != nil && !parent.IsExpanded() {
						t.Error("result's parent remains collapsed after navigation")
					}
					return true
				})
				tree.Draw(screen)
				if tree.GetCurrentNode() != target {
					t.Fatalf("drawing lost focus on result %d (%s)", index+1, target.GetReference())
				}
				if got, want := tree.FoundNodeCountInput.GetText(), fmt.Sprintf("[%d/2]", index+1); got != want {
					t.Fatalf("counter = %q, want %q", got, want)
				}
			}
		})
	}
}

func TestTreeSearchNoMatchesAndClearResetState(t *testing.T) {
	for _, query := range []string{"no_match", ""} {
		t.Run(fmt.Sprintf("query=%q", query), func(t *testing.T) {
			tree := newTreeSearchTestTree()
			tree.Filter.SetText("users")
			tree.Filter.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(tview.Primitive) {})
			tree.Filter.SetText(query)
			tree.Filter.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(tview.Primitive) {})

			if len(tree.state.searchFoundNodes) != 0 || tree.state.currentFocusFoundNode != nil {
				t.Fatal("search without matches retained previous results or search focus")
			}
			wantCounter := "[0/0]"
			if query == "" {
				wantCounter = ""
			}
			if got := tree.FoundNodeCountInput.GetText(); got != wantCounter {
				t.Fatalf("counter = %q, want %q", got, wantCounter)
			}
			current := tree.GetCurrentNode()
			tree.goToNextFoundNode()
			tree.goToPreviousFoundNode()
			if tree.GetCurrentNode() != current {
				t.Fatal("navigation without matches changed the selected node")
			}
		})
	}
}

func TestTreeSearchRapidTypingOnLargeTree(t *testing.T) {
	tree := NewTree("", nil, nil)
	tree.SetFocusFunc(nil)
	root := tree.GetRoot()
	database := tview.NewTreeNode("dbA").SetReference("dbA")
	tables := tview.NewTreeNode("tables").SetReference("dbA.tables")
	root.AddChild(database)
	database.AddChild(tables)
	users := tview.NewTreeNode("users").SetReference("dbA.tables.users")
	tables.AddChild(users)
	for i := range 1000 {
		tables.AddChild(tview.NewTreeNode(fmt.Sprintf("user_profiles_%d", i)))
	}

	for range 20 {
		for _, query := range []string{"u", "us", "use", "user", "users", "dbA.users"} {
			tree.Filter.SetText(query)
		}
	}

	unique := make(map[*tview.TreeNode]bool)
	for _, node := range tree.state.searchFoundNodes {
		unique[node] = true
	}
	t.Logf("query=%q results=%d unique=%d expected=1", tree.Filter.GetText(), len(tree.state.searchFoundNodes), len(unique))
	if len(tree.state.searchFoundNodes) != 1 || tree.state.searchFoundNodes[0] != users {
		t.Fatalf("expected only the users table, got %d results (%d unique)", len(tree.state.searchFoundNodes), len(unique))
	}
}

func TestTreeSearchEscapeClearsActiveResults(t *testing.T) {
	tree := newTreeSearchTestTree()
	tree.Filter.SetText("users")
	tree.Filter.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(tview.Primitive) {})
	tree.Filter.SetText("dbA.users")
	tree.Filter.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, 0), func(tview.Primitive) {})

	if tree.Filter.GetText() != "" || tree.FoundNodeCountInput.GetText() != "" {
		t.Fatal("escape did not clear the search text and result counter")
	}
	if len(tree.state.searchFoundNodes) != 0 || tree.state.currentFocusFoundNode != nil {
		t.Fatal("escape left active search results or search focus")
	}
}

func TestTreeSearchRankingPreservesTreeOrderForTies(t *testing.T) {
	tree := NewTree("", nil, nil)
	tree.SetFocusFunc(nil)
	var exact, prefixes []*tview.TreeNode
	for i := range 30 {
		database := tview.NewTreeNode(fmt.Sprintf("database_%d", i))
		tree.GetRoot().AddChild(database)
		archive := tview.NewTreeNode("users_archive")
		database.AddChild(archive)
		users := tview.NewTreeNode("users")
		database.AddChild(users)
		exact = append(exact, users)
		prefixes = append(prefixes, archive)
	}

	tree.Filter.SetText("users")
	expected := append(exact, prefixes...)
	if len(tree.state.searchFoundNodes) != len(expected) {
		t.Fatalf("expected %d substring matches, got %d", len(expected), len(tree.state.searchFoundNodes))
	}
	for i, node := range expected {
		if tree.state.searchFoundNodes[i] != node {
			t.Fatalf("result %d does not preserve relevance and tree order for ties", i)
		}
	}
	if tree.GetCurrentNode() != exact[0] {
		t.Fatal("initial focus is not on the best match")
	}
}

func TestTreeSearchUserDoesNotMatchBusinessProgram(t *testing.T) {
	tree := NewTree("", nil, nil)
	tree.SetFocusFunc(nil)
	tree.GetRoot().AddChild(tview.NewTreeNode("BusinessProgram"))
	tree.Filter.SetText("user")
	if len(tree.state.searchFoundNodes) != 0 {
		t.Fatalf("user matched BusinessProgram: got %d results, want 0", len(tree.state.searchFoundNodes))
	}
}

func TestTreeSearchRanksResultsButNavigatesSpatially(t *testing.T) {
	tree := NewTree("", nil, nil)
	tree.SetFocusFunc(nil)
	nodes := []*tview.TreeNode{
		tview.NewTreeNode("a_user"),
		tview.NewTreeNode("user_profiles"),
		tview.NewTreeNode("user"),
	}
	tree.GetRoot().SetChildren(nodes)
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(screen.Fini)
	tree.SetRect(0, 0, 80, 20)
	tree.Filter.SetText("user")
	tree.Filter.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(tview.Primitive) {})
	tree.Draw(screen)
	ranked := []*tview.TreeNode{nodes[2], nodes[1], nodes[0]}
	if len(tree.state.searchFoundNodes) != len(ranked) {
		t.Fatalf("expected %d ranked results, got %d", len(ranked), len(tree.state.searchFoundNodes))
	}
	for i, node := range ranked {
		if tree.state.searchFoundNodes[i] != node {
			t.Fatalf("result %d is not ordered by relevance", i)
		}
	}
	if tree.GetCurrentNode() != ranked[0] || tree.FoundNodeCountInput.GetText() != "[1/3]" {
		t.Fatal("initial selection is not the most relevant result")
	}

	for _, key := range []rune{'n', 'n', 'n', 'p', 'p', 'p'} {
		currentIndex := -1
		for i, node := range nodes {
			if node == tree.GetCurrentNode() {
				currentIndex = i
			}
		}
		if currentIndex < 0 {
			t.Fatal("focus is not on a result")
		}
		delta := 1
		if key == 'p' {
			delta = -1
		}
		expectedIndex := (currentIndex + delta + len(nodes)) % len(nodes)
		expected := nodes[expectedIndex]
		tree.InputHandler()(tcell.NewEventKey(tcell.KeyRune, key, 0), func(tview.Primitive) {})
		tree.Draw(screen)
		if tree.GetCurrentNode() != expected {
			t.Fatalf("%c from %q: selected %q, want nearest result %q", key, stripColorTags(nodes[currentIndex].GetText()), stripColorTags(tree.GetCurrentNode().GetText()), stripColorTags(expected.GetText()))
		}
		for i, node := range ranked {
			if tree.state.searchFoundNodes[i] != node {
				t.Fatal("navigation changed the relevance order")
			}
			if node == expected {
				if got, want := tree.FoundNodeCountInput.GetText(), fmt.Sprintf("[%d/3]", i+1); got != want {
					t.Fatalf("counter = %q, want %q", got, want)
				}
			}
		}
	}
}

func TestTreeSearchNavigationStartsAtActualCursor(t *testing.T) {
	for _, testCase := range []struct {
		movement string
		key      rune
		want     int
	}{
		{movement: "j", key: 'n', want: 2},
		{movement: "j", key: 'p', want: 0},
		{movement: "jj", key: 'n', want: 0},
		{movement: "jj", key: 'p', want: 0},
		{movement: "jjk", key: 'n', want: 2},
		{movement: "jjk", key: 'p', want: 0},
	} {
		t.Run(fmt.Sprintf("%s/%c", testCase.movement, testCase.key), func(t *testing.T) {
			tree := NewTree("", nil, nil)
			tree.SetFocusFunc(nil)
			nodes := []*tview.TreeNode{
				tview.NewTreeNode("users"),
				tview.NewTreeNode("orders"),
				tview.NewTreeNode("user_profiles"),
			}
			tree.GetRoot().SetChildren(nodes)
			screen := tcell.NewSimulationScreen("UTF-8")
			if err := screen.Init(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(screen.Fini)
			tree.SetRect(0, 0, 80, 20)
			tree.Filter.SetText("user")
			tree.Filter.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, 0), func(tview.Primitive) {})
			tree.Draw(screen)

			for _, key := range testCase.movement {
				tree.InputHandler()(tcell.NewEventKey(tcell.KeyRune, key, 0), func(tview.Primitive) {})
				tree.Draw(screen)
			}
			tree.InputHandler()(tcell.NewEventKey(tcell.KeyRune, testCase.key, 0), func(tview.Primitive) {})
			tree.Draw(screen)
			if tree.GetCurrentNode() != nodes[testCase.want] {
				t.Fatalf("%c selected %q, want %q", testCase.key, stripColorTags(tree.GetCurrentNode().GetText()), stripColorTags(nodes[testCase.want].GetText()))
			}
		})
	}
}

func TestTreeSearchMatchesOnlyContiguousText(t *testing.T) {
	for _, query := range []string{"user", "USER", "UsEr", " user "} {
		t.Run(query, func(t *testing.T) {
			tree := NewTree("", nil, nil)
			tree.SetFocusFunc(nil)
			for _, name := range []string{"[green]UserProfiles", "BusinessProgram", "app_user_roles", "superuser", "users", "us_ers"} {
				tree.GetRoot().AddChild(tview.NewTreeNode(name))
			}
			nodes := tree.GetRoot().GetChildren()
			want := []*tview.TreeNode{nodes[4], nodes[0], nodes[3], nodes[2]}
			tree.Filter.SetText(query)
			if len(tree.state.searchFoundNodes) != len(want) {
				t.Fatalf("got %d results, want %d contiguous matches", len(tree.state.searchFoundNodes), len(want))
			}
			for i, node := range want {
				if tree.state.searchFoundNodes[i] != node {
					t.Fatalf("result %d is %q, want %q by relevance", i, tree.state.searchFoundNodes[i].GetText(), node.GetText())
				}
			}
		})
	}
}

func TestTreeSearchQualifiersMatchOnlyContiguousText(t *testing.T) {
	for _, schemaScoped := range []bool{false, true} {
		t.Run(fmt.Sprintf("schema=%t", schemaScoped), func(t *testing.T) {
			tree := NewTree("", nil, nil)
			tree.SetFocusFunc(nil)
			parent := tree.GetRoot()
			query := "user.users"
			if schemaScoped {
				parent = tview.NewTreeNode("catalog")
				tree.GetRoot().AddChild(parent)
				query = "catalog.user.users"
			}
			var expected *tview.TreeNode
			for _, name := range []string{"BusinessProgram", "UserData"} {
				ancestor := tview.NewTreeNode(name)
				parent.AddChild(ancestor)
				tables := tview.NewTreeNode("tables")
				ancestor.AddChild(tables)
				users := tview.NewTreeNode("users")
				tables.AddChild(users)
				if name == "UserData" {
					expected = users
				}
			}
			tree.Filter.SetText(query)
			if len(tree.state.searchFoundNodes) != 1 || tree.state.searchFoundNodes[0] != expected {
				t.Fatalf("query %q: expected only UserData's users table, got %d results", query, len(tree.state.searchFoundNodes))
			}
		})
	}
}

func TestTreeSearchEmptyPatternHasNoResults(t *testing.T) {
	for _, query := range []string{"...", "   ", "\t"} {
		t.Run(fmt.Sprintf("query=%q", query), func(t *testing.T) {
			tree := newTreeSearchTestTree()
			tree.Filter.SetText(query)
			if len(tree.state.searchFoundNodes) != 0 || tree.state.currentFocusFoundNode != nil {
				t.Fatal("empty pattern matched tree nodes")
			}
		})
	}
}

func TestTreeSearchQualifiedRankingIncludesAncestors(t *testing.T) {
	tree := NewTree("", nil, nil)
	tree.SetFocusFunc(nil)
	var nodes []*tview.TreeNode
	for _, name := range []string{"app_user", "users"} {
		database := tview.NewTreeNode(name)
		tree.GetRoot().AddChild(database)
		tables := tview.NewTreeNode("tables")
		database.AddChild(tables)
		users := tview.NewTreeNode("users")
		tables.AddChild(users)
		nodes = append(nodes, users)
	}

	tree.Filter.SetText("user.user")
	if len(tree.state.searchFoundNodes) != 2 || tree.state.searchFoundNodes[0] != nodes[1] || tree.state.searchFoundNodes[1] != nodes[0] {
		t.Fatal("qualified search does not rank the better ancestor match first")
	}
	if tree.GetCurrentNode() != nodes[1] {
		t.Fatal("initial focus ignores ancestor relevance")
	}
}
