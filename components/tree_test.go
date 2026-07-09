package components

import (
	"fmt"
	"testing"

	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
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

// ── schemaProgrammingMock ───────────────────────────────────────────────────────
// Implements drivers.Driver with SupportsProgramming()=true and UseSchemas()=true.

var _ drivers.Driver = (*schemaProgrammingMock)(nil)

type schemaProgrammingMock struct{}

func (m *schemaProgrammingMock) Connect(string) error                               { return nil }
func (m *schemaProgrammingMock) TestConnection(string) error                        { return nil }
func (m *schemaProgrammingMock) GetDatabases() ([]string, error)                    { return nil, nil }
func (m *schemaProgrammingMock) GetTables(string) (map[string][]string, error)      { return nil, nil }
func (m *schemaProgrammingMock) GetTableColumns(string, string) ([][]string, error) { return nil, nil }
func (m *schemaProgrammingMock) GetConstraints(string, string) ([][]string, error)  { return nil, nil }
func (m *schemaProgrammingMock) GetForeignKeys(string, string) ([][]string, error)  { return nil, nil }
func (m *schemaProgrammingMock) GetIndexes(string, string) ([][]string, error)      { return nil, nil }
func (m *schemaProgrammingMock) GetRecords(string, string, string, string, int, int) ([][]string, int, string, error) {
	return nil, 0, "", nil
}
func (m *schemaProgrammingMock) UpdateRecord(string, string, string, string, string, string) error {
	return nil
}
func (m *schemaProgrammingMock) DeleteRecord(string, string, string, string) error { return nil }
func (m *schemaProgrammingMock) ExecuteDMLStatement(string) (string, error)        { return "", nil }
func (m *schemaProgrammingMock) ExecuteQuery(string) ([][]string, int, error)      { return nil, 0, nil }
func (m *schemaProgrammingMock) ExecutePendingChanges([]models.DBDMLChange) error  { return nil }
func (m *schemaProgrammingMock) GetProvider() string                               { return "mock" }
func (m *schemaProgrammingMock) GetPrimaryKeyColumnNames(string, string) ([]string, error) {
	return nil, nil
}
func (m *schemaProgrammingMock) SupportsProgramming() bool                            { return true }
func (m *schemaProgrammingMock) UseSchemas() bool                                     { return true }
func (m *schemaProgrammingMock) GetFunctions(string) (map[string][]string, error)     { return nil, nil }
func (m *schemaProgrammingMock) GetProcedures(string) (map[string][]string, error)    { return nil, nil }
func (m *schemaProgrammingMock) GetViews(string) (map[string][]string, error)         { return nil, nil }
func (m *schemaProgrammingMock) GetFunctionDefinition(string, string) (string, error) { return "", nil }
func (m *schemaProgrammingMock) GetProcedureDefinition(string, string) (string, error) {
	return "", nil
}
func (m *schemaProgrammingMock) GetViewDefinition(string, string) (string, error) { return "", nil }

func (m *schemaProgrammingMock) FormatArg(arg any, _ models.CellValueType) any {
	return arg
}
func (m *schemaProgrammingMock) FormatArgForQueryString(arg any) string {
	return fmt.Sprintf("%v", arg)
}
func (m *schemaProgrammingMock) FormatReference(reference string) string {
	return fmt.Sprintf("\"%s\"", reference)
}
func (m *schemaProgrammingMock) FormatPlaceholder(index int) string {
	return fmt.Sprintf("$%d", index)
}
func (m *schemaProgrammingMock) DMLChangeToQueryString(models.DBDMLChange) (string, error) {
	return "", nil
}
func (m *schemaProgrammingMock) SetProvider(string) {}

// ── qualified category tree tests ──────────────────────────────────────────────

func TestBuildQualifiedCategoryTree_PreservesCategoryNodesAndQualifiedLabels(t *testing.T) {
	tree := &Tree{DBDriver: &schemaProgrammingMock{}}

	dbNode := tview.NewTreeNode("mydb")
	dbNode.SetReference("mydb")

	tables := map[string][]string{
		"dbo":   {"users"},
		"sales": {"orders", "users"},
	}
	functions := map[string][]string{"mydb": {"dbo.add_user", "sales.add_user"}}
	procedures := map[string][]string{"mydb": {"dbo.cleanup"}}
	views := map[string][]string{"mydb": {"sales.active_users"}}

	tree.buildQualifiedCategoryTree("mydb", dbNode, tables, functions, procedures, views)

	children := dbNode.GetChildren()
	if len(children) != 4 {
		t.Fatalf("expected 4 category nodes, got %d", len(children))
	}

	sectionNames := []string{"tables", "functions", "procedures", "views"}
	for i, name := range sectionNames {
		if children[i].GetText() != name {
			t.Fatalf("expected category %d to be %q, got %q", i, name, children[i].GetText())
		}
	}

	tableChildren := children[0].GetChildren()
	if len(tableChildren) != 3 {
		t.Fatalf("expected 3 tables, got %d", len(tableChildren))
	}

	if tableChildren[0].GetText() != "dbo.users" || tableChildren[0].GetReference().(string) != "mydb.dbo.tables.users" {
		t.Fatalf("expected first table to be dbo.users with qualified reference, got %q / %q", tableChildren[0].GetText(), tableChildren[0].GetReference().(string))
	}
	if tableChildren[1].GetText() != "sales.orders" || tableChildren[1].GetReference().(string) != "mydb.sales.tables.orders" {
		t.Fatalf("expected second table to be sales.orders with qualified reference, got %q / %q", tableChildren[1].GetText(), tableChildren[1].GetReference().(string))
	}
	if tableChildren[2].GetText() != "sales.users" || tableChildren[2].GetReference().(string) != "mydb.sales.tables.users" {
		t.Fatalf("expected third table to be sales.users with qualified reference, got %q / %q", tableChildren[2].GetText(), tableChildren[2].GetReference().(string))
	}

	functionChildren := children[1].GetChildren()
	if len(functionChildren) != 2 {
		t.Fatalf("expected 2 functions, got %d", len(functionChildren))
	}
	if functionChildren[0].GetText() != "dbo.add_user" || functionChildren[1].GetText() != "sales.add_user" {
		t.Fatalf("expected qualified function labels, got %q and %q", functionChildren[0].GetText(), functionChildren[1].GetText())
	}
}

func TestBuildQualifiedCategoryTree_RespectsSchemaFilter(t *testing.T) {
	tree := &Tree{DBDriver: &schemaProgrammingMock{}, Schemas: []string{"sales"}}

	dbNode := tview.NewTreeNode("mydb")
	dbNode.SetReference("mydb")

	tree.buildQualifiedCategoryTree(
		"mydb",
		dbNode,
		map[string][]string{"dbo": {"users"}, "sales": {"orders"}},
		map[string][]string{"mydb": {"dbo.add_user", "sales.add_user"}},
		map[string][]string{},
		map[string][]string{},
	)

	tableChildren := dbNode.GetChildren()[0].GetChildren()
	if len(tableChildren) != 1 || tableChildren[0].GetText() != "sales.orders" {
		t.Fatalf("expected only sales.orders after schema filtering, got %#v", tableChildren)
	}

	functionChildren := dbNode.GetChildren()[1].GetChildren()
	if len(functionChildren) != 1 || functionChildren[0].GetText() != "sales.add_user" {
		t.Fatalf("expected only sales.add_user after schema filtering, got %#v", functionChildren)
	}
}

// ── GetTreeNodeData schema programming tests ────────────────────────────────────

func TestGetTreeNodeDataQualifiedCategorySectionHeader(t *testing.T) {
	tree := &Tree{DBDriver: &schemaProgrammingMock{}}

	node := tview.NewTreeNode("functions")
	node.SetReference("mydb.functions")

	data := tree.GetTreeNodeData(node)

	if data.Type != NodeTypeSection {
		t.Errorf("expected NodeTypeSection, got %v", data.Type)
	}
	if data.Database != "mydb" {
		t.Errorf("expected Database 'mydb', got '%s'", data.Database)
	}
	if data.Schema != "" {
		t.Errorf("expected empty Schema, got '%s'", data.Schema)
	}
	if data.Name != "functions" {
		t.Errorf("expected Name 'functions', got '%s'", data.Name)
	}
}

func TestGetTreeNodeDataSchemaProgramming_ItemNode(t *testing.T) {
	tree := &Tree{DBDriver: &schemaProgrammingMock{}}

	node := tview.NewTreeNode("public.add_user")
	node.SetReference("mydb.public.functions.add_user")

	data := tree.GetTreeNodeData(node)

	if data.Type != NodeTypeFunction {
		t.Errorf("expected NodeTypeFunction, got %v", data.Type)
	}
	if data.Database != "mydb" {
		t.Errorf("expected Database 'mydb', got '%s'", data.Database)
	}
	if data.Schema != "public" {
		t.Errorf("expected Schema 'public', got '%s'", data.Schema)
	}
	if data.Name != "add_user" {
		t.Errorf("expected Name 'add_user', got '%s'", data.Name)
	}
}

func TestGetTreeNodeDataSchemaProgramming_TableItem(t *testing.T) {
	tree := &Tree{DBDriver: &schemaProgrammingMock{}}

	node := tview.NewTreeNode("public.users")
	node.SetReference("mydb.public.tables.users")

	data := tree.GetTreeNodeData(node)

	if data.Type != NodeTypeTable {
		t.Errorf("expected NodeTypeTable, got %v", data.Type)
	}
	if data.Database != "mydb" {
		t.Errorf("expected Database 'mydb', got '%s'", data.Database)
	}
	if data.Schema != "public" {
		t.Errorf("expected Schema 'public', got '%s'", data.Schema)
	}
	if data.Name != "users" {
		t.Errorf("expected Name 'users', got '%s'", data.Name)
	}
}

func TestHandleSelectedNodePublishesQualifiedTableName(t *testing.T) {
	subscriber := make(chan models.StateChange, 2)
	tree := &Tree{
		DBDriver:    &schemaProgrammingMock{},
		state:       &TreeState{},
		subscribers: []chan models.StateChange{subscriber},
	}

	node := tview.NewTreeNode("sales.users")
	node.SetReference("mydb.sales.tables.users")

	tree.handleSelectedNode(node)

	if got := tree.GetSelectedDatabase(); got != "mydb" {
		t.Fatalf("expected selected database mydb, got %q", got)
	}
	if got := tree.GetSelectedTable(); got != "sales.users" {
		t.Fatalf("expected selected table sales.users, got %q", got)
	}

	first := <-subscriber
	second := <-subscriber
	if first.Key != eventTreeSelectedDatabase || first.Value != "mydb" {
		t.Fatalf("expected first event to select database, got %+v", first)
	}
	if second.Key != eventTreeSelectedTable || second.Value != "sales.users" {
		t.Fatalf("expected second event to select qualified table, got %+v", second)
	}
}

func TestHandleSelectedNodePublishesQualifiedFunctionName(t *testing.T) {
	subscriber := make(chan models.StateChange, 2)
	tree := &Tree{
		DBDriver:    &schemaProgrammingMock{},
		state:       &TreeState{},
		subscribers: []chan models.StateChange{subscriber},
	}

	node := tview.NewTreeNode("sales.add_user")
	node.SetReference("mydb.sales.functions.add_user")

	tree.handleSelectedNode(node)

	<-subscriber // database event
	event := <-subscriber
	if event.Key != eventTreeSelectedFunction || event.Value != "sales.add_user" {
		t.Fatalf("expected qualified function event, got %+v", event)
	}
}
