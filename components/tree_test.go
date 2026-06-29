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
// Used to test buildSchemaTree, addSchemaProgrammingSection, and the new
// GetTreeNodeData paths.

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

// ── buildSchemaTree tests ───────────────────────────────────────────────────────

func TestBuildSchemaTree_BasicStructure(t *testing.T) {
	tree := &Tree{DBDriver: &schemaProgrammingMock{}}

	dbNode := tview.NewTreeNode("mydb")
	dbNode.SetReference("mydb")

	tables := map[string][]string{"public": {"users", "posts"}}
	functions := map[string][]string{"mydb": {"public.add_user"}}
	procedures := map[string][]string{"mydb": {"public.cleanup"}}
	views := map[string][]string{"mydb": {"public.user_view"}}

	tree.buildSchemaTree("mydb", dbNode, tables, functions, procedures, views)

	// ── db node has 1 child (schema "public") ──
	children := dbNode.GetChildren()
	if len(children) != 1 {
		t.Fatalf("expected 1 child (schema) under db node, got %d", len(children))
	}

	schemaNode := children[0]
	if schemaNode.GetText() != "public" {
		t.Errorf("expected schema node text 'public', got '%s'", schemaNode.GetText())
	}

	// ── schema node has 4 children: tables, functions, procedures, views ──
	schemaChildren := schemaNode.GetChildren()
	if len(schemaChildren) != 4 {
		t.Fatalf("expected 4 children under schema node, got %d", len(schemaChildren))
	}

	sectionNames := []string{"tables", "functions", "procedures", "views"}
	for i, name := range sectionNames {
		if schemaChildren[i].GetText() != name {
			t.Errorf("expected section %d text '%s', got '%s'", i, name, schemaChildren[i].GetText())
		}
	}

	// ── "tables" section has 2 children: "users", "posts" ──
	tablesSection := schemaChildren[0]
	tableChildren := tablesSection.GetChildren()
	if len(tableChildren) != 2 {
		t.Fatalf("expected 2 table children, got %d", len(tableChildren))
	}
	if tableChildren[0].GetText() != "users" {
		t.Errorf("expected first table 'users', got '%s'", tableChildren[0].GetText())
	}
	if tableChildren[1].GetText() != "posts" {
		t.Errorf("expected second table 'posts', got '%s'", tableChildren[1].GetText())
	}
	if tableChildren[0].GetReference().(string) != "mydb.public.tables.users" {
		t.Errorf("expected table reference 'mydb.public.tables.users', got '%s'", tableChildren[0].GetReference().(string))
	}
	if tableChildren[1].GetReference().(string) != "mydb.public.tables.posts" {
		t.Errorf("expected table reference 'mydb.public.tables.posts', got '%s'", tableChildren[1].GetReference().(string))
	}

	// ── "functions" section has 1 child: "add_user" (NOT "public.add_user") ──
	functionsSection := schemaChildren[1]
	funcChildren := functionsSection.GetChildren()
	if len(funcChildren) != 1 {
		t.Fatalf("expected 1 function child, got %d", len(funcChildren))
	}
	if funcChildren[0].GetText() != "add_user" {
		t.Errorf("expected function 'add_user', got '%s'", funcChildren[0].GetText())
	}
	if funcChildren[0].GetReference().(string) != "mydb.public.functions.add_user" {
		t.Errorf("expected reference 'mydb.public.functions.add_user', got '%s'", funcChildren[0].GetReference().(string))
	}

	// ── "procedures" section has 1 child: "cleanup" ──
	proceduresSection := schemaChildren[2]
	procChildren := proceduresSection.GetChildren()
	if len(procChildren) != 1 {
		t.Fatalf("expected 1 procedure child, got %d", len(procChildren))
	}
	if procChildren[0].GetText() != "cleanup" {
		t.Errorf("expected procedure 'cleanup', got '%s'", procChildren[0].GetText())
	}
	if procChildren[0].GetReference().(string) != "mydb.public.procedures.cleanup" {
		t.Errorf("expected reference 'mydb.public.procedures.cleanup', got '%s'", procChildren[0].GetReference().(string))
	}

	// ── "views" section has 1 child: "user_view" ──
	viewsSection := schemaChildren[3]
	viewChildren := viewsSection.GetChildren()
	if len(viewChildren) != 1 {
		t.Fatalf("expected 1 view child, got %d", len(viewChildren))
	}
	if viewChildren[0].GetText() != "user_view" {
		t.Errorf("expected view 'user_view', got '%s'", viewChildren[0].GetText())
	}
	if viewChildren[0].GetReference().(string) != "mydb.public.views.user_view" {
		t.Errorf("expected reference 'mydb.public.views.user_view', got '%s'", viewChildren[0].GetReference().(string))
	}
}

// TestBuildSchemaTree_SchemaWithOnlyFunctions validates that schemas containing
// only programming objects (functions/procedures/views) but no tables still
// appear in the tree. On this branch, buildSchemaTree collects schema names
// from all maps (tables + functions/procedures/views), so "api" appears even
// though it has no tables.
func TestBuildSchemaTree_SchemaWithOnlyFunctions(t *testing.T) {
	tree := &Tree{DBDriver: &schemaProgrammingMock{}}

	dbNode := tview.NewTreeNode("mydb")
	dbNode.SetReference("mydb")

	tables := map[string][]string{"public": {"users"}}
	functions := map[string][]string{"mydb": {"api.get_data"}}
	procedures := map[string][]string{}
	views := map[string][]string{}

	tree.buildSchemaTree("mydb", dbNode, tables, functions, procedures, views)

	children := dbNode.GetChildren()

	// Expect both "api" (from functions) and "public" (from tables)
	if len(children) != 2 {
		t.Fatalf("expected 2 schema children (api, public), got %d", len(children))
	}

	// Sorted keys: ["api", "public"] — "api" comes first alphabetically
	apiNode := children[0]
	if apiNode.GetText() != "api" {
		t.Errorf("expected first schema 'api', got '%s'", apiNode.GetText())
	}

	// "api" has a "tables" section (created unconditionally when supportsProgramming)
	// plus a "functions" section with "get_data"
	apiChildren := apiNode.GetChildren()
	if len(apiChildren) != 2 {
		t.Fatalf("expected 2 children under 'api' (tables + functions), got %d", len(apiChildren))
	}
	if apiChildren[0].GetText() != "tables" {
		t.Errorf("expected 'tables' section under 'api', got '%s'", apiChildren[0].GetText())
	}
	if apiChildren[1].GetText() != "functions" {
		t.Errorf("expected 'functions' section under 'api', got '%s'", apiChildren[1].GetText())
	}

	// "api" tables section should have no children (no tables for "api")
	if len(apiChildren[0].GetChildren()) != 0 {
		t.Errorf("expected no tables under 'api', got %d", len(apiChildren[0].GetChildren()))
	}

	// Verify the function item
	funcSection := apiChildren[1]
	funcItems := funcSection.GetChildren()
	if len(funcItems) != 1 {
		t.Fatalf("expected 1 function under api, got %d", len(funcItems))
	}
	if funcItems[0].GetText() != "get_data" {
		t.Errorf("expected function 'get_data', got '%s'", funcItems[0].GetText())
	}

	// Second child: "public" has tables but no matching functions/procedures/views
	publicNode := children[1]
	if publicNode.GetText() != "public" {
		t.Errorf("expected second schema 'public', got '%s'", publicNode.GetText())
	}
	publicChildren := publicNode.GetChildren()
	if len(publicChildren) != 1 {
		t.Fatalf("expected 1 child under 'public' (just tables), got %d", len(publicChildren))
	}
	if publicChildren[0].GetText() != "tables" {
		t.Errorf("expected 'tables' section under 'public', got '%s'", publicChildren[0].GetText())
	}
	if len(publicChildren[0].GetChildren()) != 1 {
		t.Fatalf("expected 1 table under 'public', got %d", len(publicChildren[0].GetChildren()))
	}
	if publicChildren[0].GetChildren()[0].GetText() != "users" {
		t.Errorf("expected table 'users', got '%s'", publicChildren[0].GetChildren()[0].GetText())
	}
}

// ── addSchemaProgrammingSection tests ───────────────────────────────────────────

func TestAddSchemaProgrammingSection_EmptySection(t *testing.T) {
	tree := &Tree{DBDriver: &schemaProgrammingMock{}}

	schemaNode := tview.NewTreeNode("public")
	schemaNode.SetReference("public")

	// programmingMap with no items for the "public" schema
	programmingMap := map[string][]string{"mydb": {"other_schema.some_func"}}

	tree.addSchemaProgrammingSection(schemaNode, "mydb", "public", "functions", programmingMap)

	// No child should have been added because no items matched the prefix "public."
	if len(schemaNode.GetChildren()) != 0 {
		t.Errorf("expected no children added for empty section, got %d", len(schemaNode.GetChildren()))
	}
}

// ── GetTreeNodeData schema programming tests ────────────────────────────────────

func TestGetTreeNodeDataSchemaProgramming_SectionHeader(t *testing.T) {
	tree := &Tree{DBDriver: &schemaProgrammingMock{}}

	node := tview.NewTreeNode("functions")
	node.SetReference("mydb.public.functions")

	data := tree.GetTreeNodeData(node)

	if data.Type != NodeTypeSection {
		t.Errorf("expected NodeTypeSection, got %v", data.Type)
	}
	if data.Database != "mydb" {
		t.Errorf("expected Database 'mydb', got '%s'", data.Database)
	}
	if data.Schema != "public" {
		t.Errorf("expected Schema 'public', got '%s'", data.Schema)
	}
	if data.Name != "functions" {
		t.Errorf("expected Name 'functions', got '%s'", data.Name)
	}
}

func TestGetTreeNodeDataSchemaProgramming_ItemNode(t *testing.T) {
	tree := &Tree{DBDriver: &schemaProgrammingMock{}}

	node := tview.NewTreeNode("add_user")
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

	node := tview.NewTreeNode("users")
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
