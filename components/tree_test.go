package components

import (
	"testing"

	"github.com/rivo/tview"
)

// ── stripColorTags ──────────────────────────────────────────────────────────

func TestStripColorTags_NoTags(t *testing.T) {
	result := stripColorTags("choir")
	if result != "choir" {
		t.Errorf("expected 'choir', got '%s'", result)
	}
}

func TestStripColorTags_SingleTag(t *testing.T) {
	result := stripColorTags("[black:primary]choir")
	if result != "choir" {
		t.Errorf("expected 'choir', got '%s'", result)
	}
}

func TestStripColorTags_NoSpacesInsideBrackets(t *testing.T) {
	result := stripColorTags("[red]choir")
	if result != "choir" {
		t.Errorf("expected 'choir', got '%s'", result)
	}
}

func TestStripColorTags_PlainBracketTextPreserved(t *testing.T) {
	// If text contains brackets with spaces, it's not a color tag — keep it
	result := stripColorTags("table [with spaces] name")
	if result != "table [with spaces] name" {
		t.Errorf("expected 'table [with spaces] name', got '%s'", result)
	}
}

func TestStripColorTags_MultipleColorTags(t *testing.T) {
	result := stripColorTags("[black:primary][red]choir")
	if result != "choir" {
		t.Errorf("expected 'choir', got '%s'", result)
	}
}

func TestStripColorTags_NoChangeIfClean(t *testing.T) {
	inputs := []string{"choir", "choir_members", "users", "my_table"}
	for _, input := range inputs {
		result := stripColorTags(input)
		if result != input {
			t.Errorf("input '%s': expected no change, got '%s'", input, result)
		}
	}
}

// ── prioritizeResult ────────────────────────────────────────────────────────

func TestPrioritizeResult_ExactMatchWinsOverPrefix(t *testing.T) {
	exactRank := prioritizeResult("choir", "choir", 0)
	prefixRank := prioritizeResult("choir", "choir_members", 0)

	if exactRank >= prefixRank {
		t.Errorf("exact match rank (%d) should be less than prefix rank (%d)", exactRank, prefixRank)
	}
}

func TestPrioritizeResult_ShorterPrefixWins(t *testing.T) {
	rankShort := prioritizeResult("choir", "choir_a", 0)
	rankLong := prioritizeResult("choir", "choir_abcde", 0)

	if rankShort >= rankLong {
		t.Errorf("shorter prefix rank (%d) should be less than longer prefix rank (%d)", rankShort, rankLong)
	}
}

func TestPrioritizeResult_ExactMatchBeatsEverything(t *testing.T) {
	pattern := "choir"
	targets := []string{"choir_a", "choir_longer", "xchoir", "something_choir_suffix", "choir"}

	bestRank := 99999
	var bestTarget string
	for _, target := range targets {
		rank := prioritizeResult(pattern, target, 0)
		if rank < bestRank {
			bestRank = rank
			bestTarget = target
		}
	}

	if bestTarget != "choir" {
		t.Errorf("expected 'choir' to win, but '%s' won with rank %d", bestTarget, bestRank)
	}
}

func TestPrioritizeResult_SubstringPenalized(t *testing.T) {
	prefixRank := prioritizeResult("abc", "abcdef", 0)
	substrRank := prioritizeResult("abc", "xabcdef", 0)

	if prefixRank >= substrRank {
		t.Errorf("prefix rank (%d) should be less than substring rank (%d)", prefixRank, substrRank)
	}
}

// ── Real-world scenario ─────────────────────────────────────────────────────

func TestSearchRanking_ChoirTableWinsOverChoirPrefixes(t *testing.T) {
	// Simulates: tables "choir", "choir_members", "choir_events" in the tree.
	// When user searches "choir", the exact match "choir" must rank #1.

	// Build a minimal tree
	root := tview.NewTreeNode("-")
	root.SetReference("-")

	db := tview.NewTreeNode("mydb")
	db.SetReference("mydb")
	db.SetExpanded(false)
	root.AddChild(db)

	tables := []string{"choir_members", "choir_events", "choir", "other_table"}
	for _, name := range tables {
		child := tview.NewTreeNode(name)
		child.SetReference("mydb." + name)
		child.SetExpanded(false)
		db.AddChild(child)
	}

	// Run the ranking logic from the search function, adapted for test
	pattern := "choir"

	type ranked struct {
		name string
		rank int
	}
	var results []ranked

	root.Walk(func(node, _ *tview.TreeNode) bool {
		nodeText := stripColorTags(node.GetText())
		rank := prioritizeResult(pattern, nodeText, 0)
		// Only include nodes where the pattern actually matches (contains/substring check)
		// The real search uses fuzzy.RankMatch first, we skip that here
		if rank == 0 || nodeText == pattern || len(nodeText) >= len(pattern) {
			// Include all table nodes for comparison
			for _, tableName := range tables {
				if nodeText == tableName {
					results = append(results, ranked{name: nodeText, rank: rank})
				}
			}
		}
		return true
	})

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// Find the minimum rank — should be the exact match "choir"
	bestIdx := 0
	for i, r := range results {
		if r.rank < results[bestIdx].rank {
			bestIdx = i
		}
	}
	bestName := results[bestIdx].name

	// The real search sorts by rank; the first should be the exact match
	if bestName != "choir" {
		t.Errorf("expected 'choir' to be best match, but '%s' got the best rank", bestName)
	}
}

func TestSearchRanking_PrioritizeResultIntegration(t *testing.T) {
	// Simulates the full ranking pipeline with color-tagged node texts
	// as they would appear during actual use.

	entries := []struct {
		nodeText string // raw node text, possibly with color tags
	}{
		{nodeText: "[black:primary]choir_members"},
		{nodeText: "[black:primary]choir_events"},
		{nodeText: "[black:primary]choir"},
		{nodeText: "[black:primary]other_table"},
	}

	pattern := "choir"
	type ranked struct {
		cleaned string
		rank    int
	}
	var results []ranked

	for _, e := range entries {
		cleaned := stripColorTags(e.nodeText)
		rank := prioritizeResult(pattern, cleaned, 0)
		results = append(results, ranked{cleaned: cleaned, rank: rank})
	}

	// Find the minimum rank
	bestIdx := 0
	for i, r := range results {
		if r.rank < results[bestIdx].rank {
			bestIdx = i
		}
	}

	if results[bestIdx].cleaned != "choir" {
		t.Errorf("expected 'choir' to win (rank %d), but '%s' won (rank %d)",
			prioritizeResult(pattern, "choir", 0),
			results[bestIdx].cleaned,
			results[bestIdx].rank,
		)
	}
}

// ── expandAncestors ─────────────────────────────────────────────────────────

func TestExpandAncestors_DeepTree(t *testing.T) {
	root := tview.NewTreeNode("-")
	root.SetReference("-")

	db := tview.NewTreeNode("mydb")
	db.SetReference("mydb")
	db.Collapse()
	root.AddChild(db)

	tables := tview.NewTreeNode("tables")
	tables.SetReference("mydb.tables")
	tables.Collapse()
	db.AddChild(tables)

	target := tview.NewTreeNode("users")
	target.SetReference("mydb.tables.users")
	target.Collapse()
	tables.AddChild(target)

	// Initially nothing expanded
	if db.IsExpanded() {
		t.Error("db should not be expanded initially")
	}
	if tables.IsExpanded() {
		t.Error("tables should not be expanded initially")
	}

	expandAncestors(target, root)

	if !db.IsExpanded() {
		t.Error("db should be expanded after expandAncestors")
	}
	if !tables.IsExpanded() {
		t.Error("tables should be expanded after expandAncestors")
	}
	if target.IsExpanded() {
		t.Error("target node itself should not be expanded")
	}
}

func TestExpandAncestors_DirectChild(t *testing.T) {
	root := tview.NewTreeNode("-")
	root.SetReference("-")

	child := tview.NewTreeNode("direct")
	child.SetReference("direct")
	child.Collapse()
	root.AddChild(child)

	expandAncestors(child, root)
	// Direct child of root: root is never expanded (it doesn't have SetExpanded)
	// child itself shouldn't be expanded; only ancestors
	if child.IsExpanded() {
		t.Error("target node itself should not be expanded")
	}
}

func TestExpandAncestors_AlreadyExpanded(t *testing.T) {
	root := tview.NewTreeNode("-")
	root.SetReference("-")

	db := tview.NewTreeNode("mydb")
	db.SetReference("mydb")
	db.SetExpanded(true)
	root.AddChild(db)

	child := tview.NewTreeNode("table1")
	child.SetReference("mydb.table1")
	db.AddChild(child)

	expandAncestors(child, root)

	if !db.IsExpanded() {
		t.Error("db should remain expanded")
	}
}
