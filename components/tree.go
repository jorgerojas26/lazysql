package components

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
)

type TreeState struct {
	currentFocusFoundNode *tview.TreeNode
	selectedDatabase      string
	selectedTable         string
	searchFoundNodes      []*tview.TreeNode
	isFiltering           bool
}

type Tree struct {
	DBDriver drivers.Driver
	*tview.TreeView
	state               *TreeState
	Filter              *tview.InputField
	Wrapper             *tview.Flex
	FoundNodeCountInput *tview.InputField
	subscribers         []chan models.StateChange
	Schemas             []string
	schemaLoader        *schemaLoader
	loadMu              sync.Mutex
	loadGeneration      uint64
	loadCancel          context.CancelFunc
	queueUpdateDraw     func(func())
}

type TreeNodeType int

const (
	NodeTypeSection TreeNodeType = iota
	NodeTypeDatabase
	NodeTypeTable
	NodeTypeFunction
	NodeTypeProcedure
	NodeTypeView
)

type TreeNodeData struct {
	Type     TreeNodeType
	Database string
	Schema   string
	Name     string
}

func (tree *Tree) GetTreeNodeData(node *tview.TreeNode) *TreeNodeData {
	key := node.GetReference().(string)
	supportsProgramming := tree.DBDriver.SupportsProgramming()
	useSchemas := tree.DBDriver.UseSchemas()
	var nodeType TreeNodeType
	schema := ""

	split := strings.Split(key, ".")
	database := split[0]
	name := split[len(split)-1]

	switch {
	case len(split) == 1:
		nodeType = NodeTypeDatabase
	case len(split) == 2 && !useSchemas && !supportsProgramming:
		nodeType = NodeTypeTable
	case len(split) == 3 && useSchemas && !supportsProgramming:
		nodeType = NodeTypeTable
		schema = split[len(split)-2]
	case len(split) == 3 && useSchemas && supportsProgramming:
		// Section header: [database, schema, section]
		schema = split[1]
		nodeType = NodeTypeSection
	case len(split) == 3 && !useSchemas && supportsProgramming:
		// Flat (non-schema) items: [database, section, name]
		switch parentType := split[len(split)-2]; parentType {
		case "tables":
			nodeType = NodeTypeTable
		case "procedures":
			nodeType = NodeTypeProcedure
		case "functions":
			nodeType = NodeTypeFunction
		case "views":
			nodeType = NodeTypeView
		default:
			nodeType = NodeTypeSection
		}
	case len(split) == 4 && useSchemas && supportsProgramming:
		// Items under a schema: [database, schema, section, name]
		schema = split[1]
		switch split[2] {
		case "tables":
			nodeType = NodeTypeTable
		case "procedures":
			nodeType = NodeTypeProcedure
		case "functions":
			nodeType = NodeTypeFunction
		case "views":
			nodeType = NodeTypeView
		default:
			nodeType = NodeTypeSection
		}
	default:
		nodeType = NodeTypeSection
	}

	return &TreeNodeData{
		Type:     nodeType,
		Database: database,
		Schema:   schema,
		Name:     name,
	}
}

func NewTree(dbName string, dbdriver drivers.Driver, schemas []string) *Tree {
	state := &TreeState{
		selectedDatabase: "",
		selectedTable:    "",
	}

	tree := &Tree{
		Wrapper:             tview.NewFlex(),
		TreeView:            tview.NewTreeView(),
		state:               state,
		subscribers:         []chan models.StateChange{},
		DBDriver:            dbdriver,
		Filter:              tview.NewInputField(),
		FoundNodeCountInput: tview.NewInputField(),
		Schemas:             schemas,
		queueUpdateDraw:     func(update func()) { App.QueueUpdateDraw(update) },
	}

	tree.SetTopLevel(1)
	tree.SetGraphicsColor(app.Styles.PrimaryTextColor)
	// tree.SetBorder(true)
	tree.SetTitle("Databases")
	tree.SetTitleAlign(tview.AlignLeft)
	// tree.SetBorderPadding(0, 0, 1, 1)

	rootNode := tview.NewTreeNode("-")
	tree.SetRoot(rootNode)
	tree.SetCurrentNode(rootNode)

	tree.SetFocusFunc(func() {
		tree.InitializeNodes(dbName)
		tree.SetFocusFunc(nil)
	})

	selectedNodeTextColor := fmt.Sprintf("[black:%s]", app.Styles.SecondaryTextColor.Name())
	previouslyFocusedNode := tree.GetCurrentNode()
	previouslyFocusedNode.SetText(selectedNodeTextColor + previouslyFocusedNode.GetText())

	tree.SetChangedFunc(func(node *tview.TreeNode) {
		// Set colors on focused node
		nodeText := node.GetText()
		if !strings.Contains(nodeText, selectedNodeTextColor) {
			node.SetText(selectedNodeTextColor + nodeText)
		}

		// Remove colors on previously focused node
		previousNodeText := previouslyFocusedNode.GetText()
		splitNodeText := strings.Split(previousNodeText, selectedNodeTextColor)
		if len(splitNodeText) > 1 {
			previouslyFocusedNode.SetText(splitNodeText[1])
		}
		previouslyFocusedNode = node
	})

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		nodeData := tree.GetTreeNodeData(node)

		switch nodeData.Type {
		case NodeTypeSection:
			node.SetExpanded(!node.IsExpanded())
		case NodeTypeDatabase:
			if node.IsExpanded() {
				node.SetExpanded(false)
			} else {
				tree.SetSelectedDatabase(nodeData.Database)
				node.SetExpanded(true)
			}
		case NodeTypeTable:
			tree.SetSelectedDatabase(nodeData.Database)
			if nodeData.Schema == "" {
				tree.SetSelectedTable(nodeData.Name)
			} else {
				tree.SetSelectedTable(fmt.Sprintf("%s.%s", nodeData.Schema, nodeData.Name))
			}
		case NodeTypeProcedure:
			tree.SetSelectedDatabase(nodeData.Database)
			if nodeData.Schema == "" {
				tree.SetSelectedProcedure(nodeData.Name)
			} else {
				tree.SetSelectedProcedure(fmt.Sprintf("%s.%s", nodeData.Schema, nodeData.Name))
			}
		case NodeTypeFunction:
			tree.SetSelectedDatabase(nodeData.Database)
			if nodeData.Schema == "" {
				tree.SetSelectedUserDefinedFunction(nodeData.Name)
			} else {
				tree.SetSelectedUserDefinedFunction(fmt.Sprintf("%s.%s", nodeData.Schema, nodeData.Name))
			}
		case NodeTypeView:
			tree.SetSelectedDatabase(nodeData.Database)
			if nodeData.Schema == "" {
				tree.SetSelectedView(nodeData.Name)
			} else {
				tree.SetSelectedView(fmt.Sprintf("%s.%s", nodeData.Schema, nodeData.Name))
			}
		default:
			break
		}
	})

	tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := app.Keymaps.Group(app.TreeGroup).Resolve(event)

		switch command {
		case commands.GotoBottom:
			childrens := tree.GetRoot().GetChildren()
			lastNode := childrens[len(childrens)-1]

			if lastNode.IsExpanded() {
				childNodes := lastNode.GetChildren()
				lastChildren := childNodes[len(childNodes)-1]
				tree.SetCurrentNode(lastChildren)
			} else {
				tree.SetCurrentNode(lastNode)
			}
		case commands.GotoTop:
			tree.SetCurrentNode(rootNode)
		case commands.PageNext:
			tree.Move(5)
		case commands.PagePrev:
			tree.Move(-5)
		case commands.MoveDown:
			tree.Move(1)
		case commands.MoveUp:
			tree.Move(-1)
		case commands.Execute:
			// Can't "select" the current node via TreeView api.
			// So fake it by sending it a Enter key event
			return tcell.NewEventKey(tcell.KeyEnter, 0, 0)
		case commands.Search:
			tree.RemoveHighlight()
			App.SetFocus(tree.Filter)
			tree.SetIsFiltering(true)
		case commands.NextFoundNode:
			tree.goToNextFoundNode()
		case commands.PreviousFoundNode:
			tree.goToPreviousFoundNode()
		case commands.TreeCollapseAll:
			tree.CollapseAll()
		case commands.ExpandAll:
			tree.ExpandAll()
		case commands.Refresh:
			tree.Refresh(dbName)
		}
		return nil
	})

	tree.Filter.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:

			filterText := tree.Filter.GetText()

			if filterText == "" {
				tree.ClearSearch()
			} else {
				if len(tree.state.searchFoundNodes) > 0 {
					tree.FoundNodeCountInput.SetText(fmt.Sprintf("[1/%d]", len(tree.state.searchFoundNodes)))
				}
				tree.SetBorderPadding(1, 0, 0, 0)
			}

		case tcell.KeyEscape:
			tree.ClearSearch()
		}

		tree.SetIsFiltering(false)
		tree.Highlight()
		App.SetFocus(tree)
	})

	tree.Filter.SetChangedFunc(func(text string) {
		go tree.search(text)
	})

	tree.Filter.SetFieldStyle(tcell.StyleDefault.Background(app.Styles.PrimitiveBackgroundColor).Foreground(tview.Styles.PrimaryTextColor))
	tree.Filter.SetPlaceholderStyle(tcell.StyleDefault.Background(app.Styles.PrimitiveBackgroundColor).Foreground(tview.Styles.InverseTextColor))
	tree.Filter.SetBorderPadding(0, 0, 0, 0)
	tree.Filter.SetBorderColor(app.Styles.PrimaryTextColor)
	tree.Filter.SetLabel("Search: ")
	tree.Filter.SetLabelColor(app.Styles.InverseTextColor)

	tree.Filter.SetFocusFunc(func() {
		tree.Filter.SetLabelColor(app.Styles.TertiaryTextColor)
		tree.Filter.SetFieldTextColor(app.Styles.PrimaryTextColor)
	})

	tree.Filter.SetBlurFunc(func() {
		if tree.Filter.GetText() == "" {
			tree.Filter.SetLabelColor(app.Styles.InverseTextColor)
		} else {
			tree.Filter.SetLabelColor(app.Styles.TertiaryTextColor)
		}
		tree.Filter.SetFieldTextColor(app.Styles.InverseTextColor)
	})

	tree.FoundNodeCountInput.SetFieldStyle(tcell.StyleDefault.Background(app.Styles.PrimitiveBackgroundColor).Foreground(tview.Styles.PrimaryTextColor))

	tree.Wrapper.SetDirection(tview.FlexRow)
	tree.Wrapper.SetBorder(true)
	tree.Wrapper.SetBorderPadding(0, 0, 1, 1)
	tree.Wrapper.SetTitleColor(app.Styles.PrimaryTextColor)

	tree.Wrapper.AddItem(tree.Filter, 1, 0, false)
	tree.Wrapper.AddItem(tree.FoundNodeCountInput, 1, 0, false)
	tree.Wrapper.AddItem(tree, 0, 1, true)

	return tree
}

func (tree *Tree) databasesToNodes(children map[string][]string, node *tview.TreeNode, defaultExpanded bool) {
	node.ClearChildren()

	// Sort the keys and use them to loop over the
	// children so they are always in the same order.
	sortedKeys := slices.Sorted(maps.Keys(children))

	for _, key := range sortedKeys {
		// Filter schemas if Schemas is configured (PostgreSQL/MSSQL)
		if len(tree.Schemas) > 0 && tree.DBDriver.UseSchemas() {
			found := false
			for _, schema := range tree.Schemas {
				if schema == key {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		values := children[key]

		// Sort the values.
		sort.Strings(values)

		var tablesContainer *tview.TreeNode
		var rootNode *tview.TreeNode

		nodeReference := node.GetReference().(string)

		if key != nodeReference {
			rootNode = tview.NewTreeNode(key)
			rootNode.SetExpanded(false)
			rootNode.SetReference(key)
			rootNode.SetColor(app.Styles.PrimaryTextColor)
			node.AddChild(rootNode)
			tablesContainer = rootNode
		} else {
			tablesContainer = node
		}

		supportsProgramming := tree.DBDriver.SupportsProgramming()
		if supportsProgramming {
			tablesNode := tview.NewTreeNode("tables")
			tablesNode.SetExpanded(false)
			tablesNode.SetColor(app.Styles.PrimaryTextColor)

			if rootNode != nil {
				tablesNode.SetReference(fmt.Sprintf("%s.tables", key))
				rootNode.AddChild(tablesNode)
			} else {
				tablesNode.SetReference(fmt.Sprintf("%s.tables", nodeReference))
				node.AddChild(tablesNode)
			}

			tablesContainer = tablesNode
		}

		for _, child := range values {
			childNode := tview.NewTreeNode(child)
			childNode.SetExpanded(defaultExpanded)
			childNode.SetColor(app.Styles.PrimaryTextColor)

			if tree.DBDriver.UseSchemas() {
				if supportsProgramming {
					childNode.SetReference(fmt.Sprintf("%s.%s.tables.%s", nodeReference, key, child))
				} else {
					childNode.SetReference(fmt.Sprintf("%s.%s.%s", nodeReference, key, child))
				}
			} else {
				if supportsProgramming {
					childNode.SetReference(fmt.Sprintf("%s.tables.%s", key, child))
				} else {
					childNode.SetReference(fmt.Sprintf("%s.%s", key, child))
				}
			}

			tablesContainer.AddChild(childNode)
		}
	}
}

// buildSchemaTree builds the complete per-schema subtree for a database node.
// Each schema gets a node with "tables", and optionally "functions", "procedures", "views" sections.
// The functions/procedures/views maps are keyed by database name and contain schema-qualified names.
func (tree *Tree) buildSchemaTree(database string, node *tview.TreeNode, tables, functions, procedures, views map[string][]string) {
	supportsProgramming := tree.DBDriver.SupportsProgramming()

	// Collect unique schema names from tables and, when supported, from
	// functions/procedures/views (whose values are "schema.name" strings).
	schemaSet := make(map[string]struct{})
	for k := range tables {
		schemaSet[k] = struct{}{}
	}
	if supportsProgramming {
		for _, items := range functions[database] {
			if idx := strings.IndexByte(items, '.'); idx > 0 {
				schemaSet[items[:idx]] = struct{}{}
			}
		}
		for _, items := range procedures[database] {
			if idx := strings.IndexByte(items, '.'); idx > 0 {
				schemaSet[items[:idx]] = struct{}{}
			}
		}
		for _, items := range views[database] {
			if idx := strings.IndexByte(items, '.'); idx > 0 {
				schemaSet[items[:idx]] = struct{}{}
			}
		}
	}
	sortedKeys := slices.Sorted(maps.Keys(schemaSet))

	for _, schema := range sortedKeys {
		// Filter schemas if Schemas is configured
		if len(tree.Schemas) > 0 {
			found := false
			for _, s := range tree.Schemas {
				if s == schema {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		schemaNode := tview.NewTreeNode(schema)
		schemaNode.SetExpanded(false)
		schemaNode.SetReference(schema)
		schemaNode.SetColor(app.Styles.PrimaryTextColor)
		node.AddChild(schemaNode)

		// Sort the tables.
		schemaTables := tables[schema]
		sort.Strings(schemaTables)

		if supportsProgramming {
			tablesNode := tview.NewTreeNode("tables")
			tablesNode.SetExpanded(false)
			tablesNode.SetReference(fmt.Sprintf("%s.tables", schema))
			tablesNode.SetColor(app.Styles.PrimaryTextColor)
			schemaNode.AddChild(tablesNode)

			for _, child := range schemaTables {
				childNode := tview.NewTreeNode(child)
				childNode.SetExpanded(true)
				childNode.SetColor(app.Styles.PrimaryTextColor)
				childNode.SetReference(fmt.Sprintf("%s.%s.tables.%s", database, schema, child))
				tablesNode.AddChild(childNode)
			}
		} else {
			for _, child := range schemaTables {
				childNode := tview.NewTreeNode(child)
				childNode.SetExpanded(true)
				childNode.SetColor(app.Styles.PrimaryTextColor)
				childNode.SetReference(fmt.Sprintf("%s.%s.%s", database, schema, child))
				schemaNode.AddChild(childNode)
			}
		}

		if supportsProgramming {
			tree.addSchemaProgrammingSection(schemaNode, database, schema, "functions", functions)
			tree.addSchemaProgrammingSection(schemaNode, database, schema, "procedures", procedures)
			tree.addSchemaProgrammingSection(schemaNode, database, schema, "views", views)
		}
	}
}

// addSchemaProgrammingSection adds a single programming section (e.g. "functions") under a schema node.
// It filters items from the programmingMap that belong to the given schema (schema-qualified names).
func (tree *Tree) addSchemaProgrammingSection(schemaNode *tview.TreeNode, database, schema, section string, programmingMap map[string][]string) {
	prefix := schema + "."
	var items []string
	for _, qualified := range programmingMap[database] {
		if strings.HasPrefix(qualified, prefix) {
			items = append(items, strings.TrimPrefix(qualified, prefix))
		}
	}
	tree.addSchemaProgrammingItems(schemaNode, database, schema, section, items)
}

func (tree *Tree) addSchemaProgrammingItems(schemaNode *tview.TreeNode, database, schema, section string, items []string) {
	if len(items) == 0 {
		return
	}

	sort.Strings(items)

	sectionNode := tview.NewTreeNode(section)
	sectionNode.SetExpanded(false)
	sectionNode.SetReference(fmt.Sprintf("%s.%s.%s", database, schema, section))
	sectionNode.SetColor(app.Styles.PrimaryTextColor)
	schemaNode.AddChild(sectionNode)

	for _, item := range items {
		itemNode := tview.NewTreeNode(item)
		itemNode.SetExpanded(false)
		itemNode.SetColor(app.Styles.PrimaryTextColor)
		itemNode.SetReference(fmt.Sprintf("%s.%s.%s.%s", database, schema, section, item))
		sectionNode.AddChild(itemNode)
	}
}

// addSchemaProgrammingNodes enriches the table-first schema tree. It creates
// schema nodes that contain only programming objects as needed, while keeping
// the programming sections in deterministic order.
func (tree *Tree) addSchemaProgrammingNodes(node *tview.TreeNode, database string, functions, procedures, views map[string][]string) {
	type programmingGroup struct {
		section string
		items   map[string][]string
	}
	groups := []programmingGroup{
		{section: "functions", items: functions},
		{section: "procedures", items: procedures},
		{section: "views", items: views},
	}

	bySchema := make(map[string]map[string][]string)
	for _, group := range groups {
		for _, qualified := range group.items[database] {
			separator := strings.IndexByte(qualified, '.')
			if separator <= 0 || separator == len(qualified)-1 {
				continue
			}
			schema := qualified[:separator]
			if len(tree.Schemas) > 0 && !slices.Contains(tree.Schemas, schema) {
				continue
			}
			if bySchema[schema] == nil {
				bySchema[schema] = make(map[string][]string)
			}
			bySchema[schema][group.section] = append(bySchema[schema][group.section], qualified[separator+1:])
		}
	}

	for _, schema := range slices.Sorted(maps.Keys(bySchema)) {
		schemaNode := tree.findTreeChild(node, schema)
		if schemaNode == nil {
			schemaNode = tview.NewTreeNode(schema)
			schemaNode.SetExpanded(false)
			schemaNode.SetReference(schema)
			schemaNode.SetColor(app.Styles.PrimaryTextColor)
			node.AddChild(schemaNode)
		}
		for _, group := range groups {
			tree.addSchemaProgrammingItems(schemaNode, database, schema, group.section, bySchema[schema][group.section])
		}
	}
}

func (tree *Tree) findTreeChild(node *tview.TreeNode, text string) *tview.TreeNode {
	for _, child := range node.GetChildren() {
		if child.GetText() == text {
			return child
		}
	}
	return nil
}

func (tree *Tree) addProgrammingNodes(functions map[string][]string, procedures map[string][]string, views map[string][]string, node *tview.TreeNode) {
	database := node.GetText()
	dbFunctions := functions[database]
	sort.Strings(dbFunctions)

	var functionsNode *tview.TreeNode
	functionsNodeReference := fmt.Sprintf("%s.functions", node.GetReference().(string))
	functionsNode = tview.NewTreeNode("functions")
	functionsNode.SetExpanded(false)
	functionsNode.SetReference(functionsNodeReference)
	functionsNode.SetColor(app.Styles.PrimaryTextColor)
	node.AddChild(functionsNode)

	for _, function := range dbFunctions {
		functionNode := tview.NewTreeNode(function)
		functionNode.SetExpanded(false)
		functionNode.SetColor(app.Styles.PrimaryTextColor)
		functionNode.SetReference(fmt.Sprintf("%s.%s", functionsNodeReference, function))
		functionsNode.AddChild(functionNode)
	}

	dbProcedures := procedures[database]
	sort.Strings(dbProcedures)

	var proceduresNode *tview.TreeNode
	proceduresNodeReference := fmt.Sprintf("%s.procedures", node.GetReference().(string))
	proceduresNode = tview.NewTreeNode("procedures")
	proceduresNode.SetExpanded(false)
	proceduresNode.SetReference(proceduresNodeReference)
	proceduresNode.SetColor(app.Styles.PrimaryTextColor)
	node.AddChild(proceduresNode)

	for _, procedure := range dbProcedures {
		procedureNode := tview.NewTreeNode(procedure)
		procedureNode.SetExpanded(false)
		procedureNode.SetColor(app.Styles.PrimaryTextColor)
		procedureNode.SetReference(fmt.Sprintf("%s.%s", proceduresNodeReference, procedure))
		proceduresNode.AddChild(procedureNode)
	}

	dbViews := views[database]
	sort.Strings(dbViews)

	var viewsNode *tview.TreeNode
	viewsNodeReference := fmt.Sprintf("%s.views", node.GetReference().(string))
	viewsNode = tview.NewTreeNode("views")
	viewsNode.SetExpanded(false)
	viewsNode.SetReference(viewsNodeReference)
	viewsNode.SetColor(app.Styles.PrimaryTextColor)
	node.AddChild(viewsNode)

	for _, view := range dbViews {
		viewNode := tview.NewTreeNode(view)
		viewNode.SetExpanded(false)
		viewNode.SetColor(app.Styles.PrimaryTextColor)
		viewNode.SetReference(fmt.Sprintf("%s.%s", viewsNodeReference, view))
		viewsNode.AddChild(viewNode)
	}
}

// stripColorTags removes tview color formatting like [black:primary] from node text
func stripColorTags(text string) string {
	for {
		start := strings.Index(text, "[")
		if start == -1 {
			break
		}
		end := strings.Index(text[start:], "]")
		if end == -1 {
			break
		}
		end += start // make absolute

		inner := text[start+1 : end]
		// tview color tags never contain spaces: [black:primary], [red], [green:black:b]
		if !strings.Contains(inner, " ") {
			text = text[:start] + text[end+1:]
		} else {
			// Not a color tag: replace '[' with sentinel so we don't loop forever
			text = text[:start] + "\x00" + text[start+1:]
		}
	}
	return strings.ReplaceAll(text, "\x00", "[")
}

func prioritizeResult(pattern, target string, fuzzyRank int) int {
	// play match golf - lowest score wins

	// Exact match
	if pattern == target {
		return 0
	}

	// Prefix is scored on length difference, 1-99
	if strings.HasPrefix(target, pattern) {
		lengthDiff := len(target) - len(pattern)
		if lengthDiff > 98 {
			lengthDiff = 98
		}
		return 1 + lengthDiff
	}

	// Substr penalized by distance from start and length diff
	if strings.Contains(target, pattern) {
		index := strings.Index(target, pattern)
		lengthPenalty := len(target) - len(pattern)
		score := 100 + index + lengthPenalty
		if score > 9999 {
			score = 9999
		}
		return score
	}

	// If no other matches, fall back to fuzzy match with a low score
	return 10000 + fuzzyRank
}

// expandAncestors expands all ancestor nodes of the given node up to (but not including) root.
// tview TreeNode doesn't expose a parent pointer, so we walk from root to find the path.
func expandAncestors(target *tview.TreeNode, root *tview.TreeNode) {
	// Collect ancestors by walking the tree with a parent stack
	type stackEntry struct {
		node   *tview.TreeNode
		parent *tview.TreeNode
	}
	stack := []stackEntry{{node: root, parent: nil}}
	var ancestors []*tview.TreeNode

	for len(stack) > 0 {
		entry := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if entry.node == target {
			// Build ancestor chain walking back up
			for e := entry.parent; e != nil && e != root; {
				ancestors = append(ancestors, e)
				// Find e's parent by walking the tree (brute force but tree is small)
				found := false
				root.Walk(func(n, p *tview.TreeNode) bool {
					if n == e {
						e = p
						found = true
						return false
					}
					return true
				})
				if !found {
					break
				}
			}
			// Expand from top to bottom
			for i := len(ancestors) - 1; i >= 0; i-- {
				ancestors[i].SetExpanded(true)
			}
			return
		}

		for _, child := range entry.node.GetChildren() {
			stack = append(stack, stackEntry{node: child, parent: entry.node})
		}
	}
}

// parseSearchQuery splits a lowercased search query into ancestor filters
// (outermost first) and a final table/node name filter.
//
// Supports:
//   - single part: "users"                         -> ([], "users")
//   - dotted path: "db.users"                       -> (["db"], "users")
//   - dotted path: "db.schema.users"                -> (["db", "schema"], "users")
//   - space form:  "schema users"                   -> (["schema"], "users") (legacy, still supported)
//
// A "." anywhere in the query takes priority over spaces, since qualified
// database/schema/table paths (as typed in SQL, e.g. db.schema.table) are
// the primary use case for multi-part search. The legacy single-space,
// two-part syntax ( "schema table") keeps working unchanged when no dot is
// present.
func parseSearchQuery(lowerSearchText string) (ancestorFilters []string, tableNameFilter string) {
	if strings.Contains(lowerSearchText, ".") {
		parts := strings.Split(lowerSearchText, ".")
		// Drop empty segments (e.g. leading/trailing/duplicate dots) so a
		// stray "." doesn't produce a spurious empty filter.
		nonEmpty := make([]string, 0, len(parts))
		for _, p := range parts {
			if p != "" {
				nonEmpty = append(nonEmpty, p)
			}
		}
		if len(nonEmpty) == 0 {
			return nil, ""
		}
		return nonEmpty[:len(nonEmpty)-1], nonEmpty[len(nonEmpty)-1]
	}

	parts := strings.SplitN(lowerSearchText, " ", 2)
	if len(parts) == 1 {
		return nil, parts[0]
	}
	return []string{parts[0]}, parts[1]
}

func (tree *Tree) search(searchText string) {
	rootNode := tree.GetRoot()
	lowerSearchText := strings.ToLower(searchText)
	tree.state.searchFoundNodes = []*tview.TreeNode{}

	if lowerSearchText == "" {
		rootNode.Walk(func(_, parent *tview.TreeNode) bool {
			if parent != nil && parent != rootNode && parent.IsExpanded() {
				parent.SetExpanded(false)
			}
			return true
		})
		return
	}

	// ancestorFilters are, in order from outermost to innermost, the
	// qualifiers that must each match a distinct ancestor of the node
	// (further qualifiers must match closer ancestors than earlier ones).
	// tableNameFilter always matches the node itself.
	ancestorFilters, tableNameFilter := parseSearchQuery(lowerSearchText)

	// Collect nodes with their match ranks
	type rankedNode struct {
		node *tview.TreeNode
		rank int
	}
	var rankedNodes []rankedNode

	// Build parent map while walking so we can walk up the ancestor chain
	// when a qualified search (e.g. "schema tablename" or "db.schema.table")
	// needs to match against non-immediate ancestors in deep trees with
	// section headers.
	parentMap := make(map[*tview.TreeNode]*tview.TreeNode)

	rootNode.Walk(func(node, parent *tview.TreeNode) bool {
		parentMap[node] = parent
		nodeText := strings.ToLower(stripColorTags(node.GetText()))

		if len(ancestorFilters) == 0 {
			rank := fuzzy.RankMatch(tableNameFilter, nodeText)
			if rank >= 0 {
				adjustedRank := prioritizeResult(tableNameFilter, nodeText, rank)
				rankedNodes = append(rankedNodes, rankedNode{node: node, rank: adjustedRank})
			}
			return true
		}

		rank := fuzzy.RankMatch(tableNameFilter, nodeText)
		if rank < 0 {
			return true
		}

		// Walk up the ancestor chain once, matching each ancestor filter
		// (outermost first) against the closest possible ancestor that is
		// still further from the node than the previous filter's match.
		// This lets e.g. "db.schema.table" require the "db" match to be a
		// stricter (or equal) ancestor than the "schema" match, walking
		// through section headers (tables/views/functions) in between.
		ancestorRanks := make([]int, len(ancestorFilters))
		for i := range ancestorRanks {
			ancestorRanks[i] = -1
		}
		filterIdx := len(ancestorFilters) - 1
		for e := parentMap[node]; e != nil && e != rootNode && filterIdx >= 0; e = parentMap[e] {
			eText := strings.ToLower(stripColorTags(e.GetText()))
			eRank := fuzzy.RankMatch(ancestorFilters[filterIdx], eText)
			if eRank >= 0 {
				ancestorRanks[filterIdx] = prioritizeResult(ancestorFilters[filterIdx], eText, eRank)
				filterIdx--
			}
		}

		// Every ancestor filter must have matched some ancestor for this
		// node to be considered a result at all (this is what actually
		// scopes "dbA.users" to dbA instead of leaking dbB's users table).
		allMatched := true
		ancestorScore := 0
		for _, ar := range ancestorRanks {
			if ar < 0 {
				allMatched = false
				break
			}
			ancestorScore += ar
		}

		if allMatched {
			adjustedTableRank := prioritizeResult(tableNameFilter, nodeText, rank)
			// Combine ranks: prioritize table match but factor in ancestor matches.
			combinedRank := adjustedTableRank + (ancestorScore / (2 * len(ancestorFilters)))
			rankedNodes = append(rankedNodes, rankedNode{node: node, rank: combinedRank})
		}

		return true
	})

	// Once the query is qualified (at least one ancestor filter present,
	// i.e. a dotted or space-separated "db[.schema].table" query), an exact
	// (case-insensitive) match on the final segment collapses the result
	// set to just the exact hit(s), dropping near-name fuzzy siblings that
	// only partially match. This only applies once an exact match actually
	// exists among the ancestor-scoped candidates — mid-typing queries with
	// no exact hit yet keep fuzzy-matching normally, so the "narrow as you
	// type" UX is preserved. Unqualified (single-part) queries are
	// untouched, keeping their existing fuzzy behavior.
	if len(ancestorFilters) > 0 {
		hasExactMatch := false
		for _, rn := range rankedNodes {
			nodeText := strings.ToLower(stripColorTags(rn.node.GetText()))
			if nodeText == tableNameFilter {
				hasExactMatch = true
				break
			}
		}
		if hasExactMatch {
			exactNodes := rankedNodes[:0]
			for _, rn := range rankedNodes {
				nodeText := strings.ToLower(stripColorTags(rn.node.GetText()))
				if nodeText == tableNameFilter {
					exactNodes = append(exactNodes, rn)
				}
			}
			rankedNodes = exactNodes
		}
	}

	sort.Slice(rankedNodes, func(i, j int) bool {
		return rankedNodes[i].rank < rankedNodes[j].rank
	})

	for _, rn := range rankedNodes {
		tree.state.searchFoundNodes = append(tree.state.searchFoundNodes, rn.node)
	}

	// Set current node to best match
	if len(tree.state.searchFoundNodes) > 0 {
		bestNode := tree.state.searchFoundNodes[0]
		expandAncestors(bestNode, rootNode)
		tree.SetCurrentNode(bestNode)
		tree.state.currentFocusFoundNode = bestNode
	}
}

// Subscribe to changes in the tree state
func (tree *Tree) Subscribe() chan models.StateChange {
	subscriber := make(chan models.StateChange)
	tree.subscribers = append(tree.subscribers, subscriber)
	return subscriber
}

// Publish subscribers of changes in the tree state
func (tree *Tree) Publish(change models.StateChange) {
	for _, subscriber := range tree.subscribers {
		subscriber <- change
	}
}

// Getters and Setters
func (tree *Tree) GetSelectedDatabase() string {
	return tree.state.selectedDatabase
}

func (tree *Tree) GetSelectedTable() string {
	return tree.state.selectedTable
}

func (tree *Tree) GetIsFiltering() bool {
	return tree.state.isFiltering
}

func (tree *Tree) SetSelectedDatabase(database string) {
	tree.state.selectedDatabase = database
	tree.Publish(models.StateChange{
		Key:   eventTreeSelectedDatabase,
		Value: database,
	})
}

func (tree *Tree) SetSelectedTable(table string) {
	tree.state.selectedTable = table
	tree.Publish(models.StateChange{
		Key:   eventTreeSelectedTable,
		Value: table,
	})
}

func (tree *Tree) SetSelectedUserDefinedFunction(name string) {
	tree.Publish(models.StateChange{
		Key:   eventTreeSelectedFunction,
		Value: name,
	})
}

func (tree *Tree) SetSelectedProcedure(name string) {
	tree.Publish(models.StateChange{
		Key:   eventTreeSelectedProcedure,
		Value: name,
	})
}

func (tree *Tree) SetSelectedView(name string) {
	tree.Publish(models.StateChange{
		Key:   eventTreeSelectedView,
		Value: name,
	})
}

func (tree *Tree) SetIsFiltering(isFiltering bool) {
	tree.state.isFiltering = isFiltering
	tree.Publish(models.StateChange{
		Key:   eventTreeIsFiltering,
		Value: isFiltering,
	})
}

// Blur func
func (tree *Tree) RemoveHighlight() {
	tree.SetBorderColor(app.Styles.InverseTextColor)
	tree.SetGraphicsColor(app.Styles.InverseTextColor)
	tree.SetTitleColor(app.Styles.InverseTextColor)
	// tree.GetRoot().SetColor(app.Styles.InverseTextColor)

	childrens := tree.GetRoot().GetChildren()

	for _, children := range childrens {
		currentColor := children.GetColor()

		childrenIsCurrentNode := children.GetReference() == tree.GetCurrentNode().GetReference()

		if !childrenIsCurrentNode && currentColor == app.Styles.PrimaryTextColor {
			children.SetColor(app.Styles.InverseTextColor)
		}

		childrenOfChildren := children.GetChildren()

		for _, children := range childrenOfChildren {
			currentColor := children.GetColor()

			childrenIsCurrentNode := children.GetReference() == tree.GetCurrentNode().GetReference()

			if !childrenIsCurrentNode && currentColor == app.Styles.PrimaryTextColor {
				children.SetColor(app.Styles.InverseTextColor)
			}

		}

	}
}

func (tree *Tree) ForceRemoveHighlight() {
	tree.SetBorderColor(app.Styles.InverseTextColor)
	tree.SetGraphicsColor(app.Styles.InverseTextColor)
	tree.SetTitleColor(app.Styles.InverseTextColor)
	tree.GetRoot().SetColor(app.Styles.InverseTextColor)

	childrens := tree.GetRoot().GetChildren()

	for _, children := range childrens {

		children.SetColor(app.Styles.InverseTextColor)

		childrenOfChildren := children.GetChildren()

		for _, children := range childrenOfChildren {
			children.SetColor(app.Styles.InverseTextColor)
		}

	}
}

// Focus func
func (tree *Tree) Highlight() {
	tree.SetBorderColor(app.Styles.PrimaryTextColor)
	tree.SetGraphicsColor(app.Styles.PrimaryTextColor)
	tree.SetTitleColor(app.Styles.PrimaryTextColor)
	tree.GetRoot().SetColor(app.Styles.PrimaryTextColor)

	childrens := tree.GetRoot().GetChildren()

	for _, children := range childrens {
		currentColor := children.GetColor()

		if currentColor == app.Styles.InverseTextColor {
			children.SetColor(app.Styles.PrimaryTextColor)

			childrenOfChildren := children.GetChildren()

			for _, children := range childrenOfChildren {
				currentColor := children.GetColor()

				if currentColor == app.Styles.InverseTextColor {
					children.SetColor(app.Styles.PrimaryTextColor)
				}
			}

		}

	}
}

func (tree *Tree) goToNextFoundNode() {
	for i, node := range tree.state.searchFoundNodes {
		if node == tree.state.currentFocusFoundNode {
			var newFocusNodeIndex int

			if i+1 < len(tree.state.searchFoundNodes) {
				newFocusNodeIndex = i + 1
			} else {
				newFocusNodeIndex = 0
			}

			newFocusNode := tree.state.searchFoundNodes[newFocusNodeIndex]
			tree.SetCurrentNode(newFocusNode)
			tree.state.currentFocusFoundNode = newFocusNode
			tree.FoundNodeCountInput.SetText(fmt.Sprintf("[%d/%d]", newFocusNodeIndex+1, len(tree.state.searchFoundNodes)))
			break
		}
	}
}

func (tree *Tree) goToPreviousFoundNode() {
	for i, node := range tree.state.searchFoundNodes {
		if node == tree.state.currentFocusFoundNode {
			var newFocusNodeIndex int

			if i-1 >= 0 {
				newFocusNodeIndex = i - 1
			} else {
				newFocusNodeIndex = len(tree.state.searchFoundNodes) - 1
			}

			newFocusNode := tree.state.searchFoundNodes[newFocusNodeIndex]
			tree.SetCurrentNode(newFocusNode)
			tree.state.currentFocusFoundNode = newFocusNode
			tree.FoundNodeCountInput.SetText(fmt.Sprintf("[%d/%d]", newFocusNodeIndex+1, len(tree.state.searchFoundNodes)))
			break
		}
	}
}

func (tree *Tree) CollapseAll() {
	tree.GetRoot().Walk(func(node, _ *tview.TreeNode) bool {
		if node.IsExpanded() && node != tree.GetRoot() {
			node.Collapse()
		}
		return true
	})
}

func (tree *Tree) ExpandAll() {
	tree.GetRoot().Walk(func(node, _ *tview.TreeNode) bool {
		if !node.IsExpanded() && node != tree.GetRoot() {
			node.Expand()
		}
		return true
	})
}

func (tree *Tree) InitializeNodes(dbName string) {
	generation, ctx := tree.beginLoad()
	tree.initializeNodes(ctx, dbName, generation)
}

func (tree *Tree) beginLoad() (uint64, context.Context) {
	tree.loadMu.Lock()
	defer tree.loadMu.Unlock()
	if tree.loadCancel != nil {
		tree.loadCancel()
	}

	parent := context.Background()
	if app.App != nil {
		parent = app.App.Context()
	}
	ctx, cancel := context.WithCancel(parent)
	tree.loadCancel = cancel
	tree.loadGeneration++
	return tree.loadGeneration, ctx
}

func (tree *Tree) isCurrentLoad(generation uint64) bool {
	tree.loadMu.Lock()
	defer tree.loadMu.Unlock()
	return tree.loadGeneration == generation
}

// initializeNodes renders database nodes immediately, then loads each
// database independently. Tables are applied as soon as their shared schema
// request completes; programming objects are fetched only after that first
// paint and enrich the existing table subtree afterward.
func (tree *Tree) initializeNodes(ctx context.Context, dbName string, generation uint64) {
	rootNode := tree.GetRoot()
	if rootNode == nil {
		panic("Internal Error: No tree root")
	}

	var databases []string

	if dbName == "" {
		started := time.Now()
		dbs, err := tree.DBDriver.GetDatabases(ctx)
		logDatabaseOperation("get_databases", started, ctx, nil, err)
		if err != nil {
			panic(err.Error())
		}
		sanitizedDbs := make([]string, 0, len(dbs))
		for _, db := range dbs {
			sanitizedDbs = append(sanitizedDbs, sanitizeDBName(db))
		}
		databases = sanitizedDbs
	} else {
		sanitizedDBName := sanitizeDBName(dbName)
		databases = []string{sanitizedDBName}
	}

	for _, database := range databases {
		childNode := tview.NewTreeNode(database)
		childNode.SetExpanded(dbName != "")
		childNode.SetReference(database)
		childNode.SetColor(app.Styles.PrimaryTextColor)
		rootNode.AddChild(childNode)

		go tree.loadDatabaseNodes(ctx, generation, database, childNode)
	}
}

func (tree *Tree) loadDatabaseNodes(ctx context.Context, generation uint64, database string, node *tview.TreeNode) {
	var tables map[string][]string
	var err error
	if tree.schemaLoader != nil {
		tables, err = tree.schemaLoader.loadTables(ctx, database)
	} else {
		started := time.Now()
		tables, err = tree.DBDriver.GetTables(ctx, database)
		logDatabaseOperation("get_tables", started, ctx, map[string]any{
			"database":  database,
			"cache_hit": false,
		}, err)
	}
	if err != nil {
		logger.Error(err.Error(), nil)
		return
	}
	if !tree.isCurrentLoad(generation) {
		return
	}

	// Render the primary table catalog before asking for any programming
	// objects. This is the progressive tree's first useful paint.
	tree.renderLoadUpdate(generation, func() {
		tree.addTableNodes(database, node, tables)
	})

	if !tree.DBDriver.SupportsProgramming() || !tree.isCurrentLoad(generation) {
		return
	}

	functionsStarted := time.Now()
	functions, err := tree.DBDriver.GetFunctions(ctx, database)
	logDatabaseOperation("get_functions", functionsStarted, ctx, map[string]any{
		"database": database,
	}, err)
	if err != nil {
		logger.Error(err.Error(), nil)
		return
	}
	proceduresStarted := time.Now()
	procedures, err := tree.DBDriver.GetProcedures(ctx, database)
	logDatabaseOperation("get_procedures", proceduresStarted, ctx, map[string]any{
		"database": database,
	}, err)
	if err != nil {
		logger.Error(err.Error(), nil)
		return
	}
	viewsStarted := time.Now()
	views, err := tree.DBDriver.GetViews(ctx, database)
	logDatabaseOperation("get_views", viewsStarted, ctx, map[string]any{
		"database": database,
	}, err)
	if err != nil {
		logger.Error(err.Error(), nil)
		return
	}
	if !tree.isCurrentLoad(generation) {
		return
	}

	tree.renderLoadUpdate(generation, func() {
		tree.enrichProgrammingNodes(database, node, functions, procedures, views)
	})
}

func (tree *Tree) renderLoadUpdate(generation uint64, update func()) {
	if !tree.isCurrentLoad(generation) {
		return
	}

	if tree.queueUpdateDraw != nil {
		tree.queueUpdateDraw(func() {
			if tree.isCurrentLoad(generation) {
				update()
			}
		})
		return
	}

	// Minimal trees in unit tests do not have an application event loop. Keep
	// that seam synchronous while production trees serialize node mutations on
	// tview's UI loop through queueUpdateDraw above.
	if tree.isCurrentLoad(generation) {
		update()
	}
	if App != nil {
		App.Draw()
	}
}

func (tree *Tree) addTableNodes(database string, node *tview.TreeNode, tables map[string][]string) {
	if tree.DBDriver.UseSchemas() {
		tree.buildSchemaTree(database, node, tables, nil, nil, nil)
		return
	}
	tree.databasesToNodes(tables, node, true)
}

func (tree *Tree) enrichProgrammingNodes(database string, node *tview.TreeNode, functions, procedures, views map[string][]string) {
	if tree.DBDriver.UseSchemas() {
		tree.addSchemaProgrammingNodes(node, database, functions, procedures, views)
		return
	}
	tree.addProgrammingNodes(functions, procedures, views, node)
}

func (tree *Tree) Refresh(dbName string) {
	if tree.schemaLoader != nil {
		tree.schemaLoader.invalidateAll()
	}
	tree.refreshNodes(dbName)
}

// RefreshAsync is used by background SQL completion paths. The visible tree
// mutation is queued on tview's UI loop, while catalog work remains in the
// per-database goroutines started by refreshNodes.
func (tree *Tree) RefreshAsync(dbName string) {
	if tree.queueUpdateDraw == nil {
		go tree.Refresh(dbName)
		return
	}
	go tree.queueUpdateDraw(func() {
		tree.Refresh(dbName)
	})
}

func (tree *Tree) refreshNodes(dbName string) {
	generation, ctx := tree.beginLoad()
	rootNode := tree.GetRoot()
	if dbName != "" {
		// A connection without a fixed database shows several database nodes.
		// Refresh only the selected database so DDL does not make unrelated
		// visible databases disappear.
		sanitizedName := sanitizeDBName(dbName)
		for _, child := range rootNode.GetChildren() {
			if reference, ok := child.GetReference().(string); ok && reference == sanitizedName {
				rootNode.RemoveChild(child)
				break
			}
		}
	} else {
		rootNode.ClearChildren()
	}
	// Re-add the requested scope. Per-database work remains asynchronous.
	tree.initializeNodes(ctx, dbName, generation)
}

func (tree *Tree) ClearSearch() {
	tree.search("")
	tree.FoundNodeCountInput.SetText("")
	tree.SetBorderPadding(0, 0, 0, 0)
	tree.Filter.SetText("")
}

func sanitizeDBName(dbName string) string {
	// Remove dots from db name
	return strings.ReplaceAll(dbName, ".", "_")
}
