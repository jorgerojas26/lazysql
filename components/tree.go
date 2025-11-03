package components

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

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
	nodeType := NodeTypeSection
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
	case len(split) == 3 && !useSchemas && supportsProgramming:
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

		schema = split[len(split)-2]
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

func NewTree(dbName string, dbdriver drivers.Driver) *Tree {
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

func (tree *Tree) databasesToNodes(databases map[string][]string, node *tview.TreeNode, defaultExpanded bool) {
	node.ClearChildren()

	// Sort the keys and use them to loop over the
	// children so they are always in the same order.
	sortedKeys := slices.Sorted(maps.Keys(databases))

	for _, key := range sortedKeys {
		values := databases[key]

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
			if tree.DBDriver.GetProvider() == "sqlite3" {
				childNode.SetReference(child)
			} else if tree.DBDriver.UseSchemas() {
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

func (tree *Tree) addProgrammingNodes(functions map[string][]string, procedures map[string][]string, views map[string][]string, node *tview.TreeNode) {
	var database = node.GetText()
	var dbFunctions = functions[database]
	sort.Strings(dbFunctions)

	var functionsNode *tview.TreeNode
	var functionsNodeReference = fmt.Sprintf("%s.functions", node.GetReference().(string))
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

	var dbProcedures = procedures[database]
	sort.Strings(dbProcedures)

	var proceduresNode *tview.TreeNode
	var proceduresNodeReference = fmt.Sprintf("%s.procedures", node.GetReference().(string))
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

	var dbViews = views[database]
	sort.Strings(dbViews)

	var viewsNode *tview.TreeNode
	var viewsNodeReference = fmt.Sprintf("%s.views", node.GetReference().(string))
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

	parts := strings.SplitN(lowerSearchText, " ", 2)
	databaseNameFilter := ""
	tableNameFilter := ""

	if len(parts) == 1 {
		tableNameFilter = parts[0]
	} else {
		databaseNameFilter = parts[0]
		tableNameFilter = parts[1]
	}

	// Collect nodes with their match ranks
	type rankedNode struct {
		node *tview.TreeNode
		rank int
	}
	var rankedNodes []rankedNode

	rootNode.Walk(func(node, parent *tview.TreeNode) bool {
		nodeText := strings.ToLower(node.GetText())

		if databaseNameFilter == "" {
			rank := fuzzy.RankMatch(tableNameFilter, nodeText)
			if rank >= 0 {
				if parent != nil {
					parent.SetExpanded(true)
				}
				adjustedRank := prioritizeResult(tableNameFilter, nodeText, rank)
				rankedNodes = append(rankedNodes, rankedNode{node: node, rank: adjustedRank})
			}
		} else {
			rank := fuzzy.RankMatch(tableNameFilter, nodeText)
			if rank >= 0 && parent != nil {
				parentText := strings.ToLower(parent.GetText())
				parentRank := fuzzy.RankMatch(databaseNameFilter, parentText)
				if parentRank >= 0 {
					parent.SetExpanded(true)
					adjustedTableRank := prioritizeResult(tableNameFilter, nodeText, rank)
					adjustedParentRank := prioritizeResult(databaseNameFilter, parentText, parentRank)
					// Combine ranks: prioritize table match but factor in database match
					combinedRank := adjustedTableRank + (adjustedParentRank / 2)
					rankedNodes = append(rankedNodes, rankedNode{node: node, rank: combinedRank})
				}
			}
		}

		return true
	})

	sort.Slice(rankedNodes, func(i, j int) bool {
		return rankedNodes[i].rank < rankedNodes[j].rank
	})

	for _, rn := range rankedNodes {
		tree.state.searchFoundNodes = append(tree.state.searchFoundNodes, rn.node)
	}

	// Set current node to best match
	if len(tree.state.searchFoundNodes) > 0 {
		tree.SetCurrentNode(tree.state.searchFoundNodes[0])
		tree.state.currentFocusFoundNode = tree.state.searchFoundNodes[0]
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
	rootNode := tree.GetRoot()
	if rootNode == nil {
		panic("Internal Error: No tree root")
	}

	var databases []string

	if dbName == "" {
		dbs, err := tree.DBDriver.GetDatabases()
		if err != nil {
			panic(err.Error())
		}
		databases = dbs
	} else {
		databases = []string{dbName}
	}

	for _, database := range databases {
		childNode := tview.NewTreeNode(database)
		childNode.SetExpanded(false)
		childNode.SetReference(database)
		childNode.SetColor(app.Styles.PrimaryTextColor)
		rootNode.AddChild(childNode)

		go func(database string, node *tview.TreeNode) {
			tables, err := tree.DBDriver.GetTables(database)
			if err != nil {
				logger.Error(err.Error(), nil)
				return
			}

			tree.databasesToNodes(tables, node, true)

			if tree.DBDriver.SupportsProgramming() {
				functions, err := tree.DBDriver.GetFunctions(database)
				if err != nil {
					logger.Error(err.Error(), nil)
					return
				}

				procedures, err := tree.DBDriver.GetProcedures(database)
				if err != nil {
					logger.Error(err.Error(), nil)
					return
				}

				views, err := tree.DBDriver.GetViews(database)
				if err != nil {
					logger.Error(err.Error(), nil)
					return
				}

				tree.addProgrammingNodes(functions, procedures, views, node)
			}

			App.Draw()
		}(database, childNode)
	}
}

func (tree *Tree) Refresh(dbName string) {
	rootNode := tree.GetRoot()
	rootNode.ClearChildren()
	// re-add nodes
	tree.InitializeNodes(dbName)
}

func (tree *Tree) ClearSearch() {
	tree.search("")
	tree.FoundNodeCountInput.SetText("")
	tree.SetBorderPadding(0, 0, 0, 0)
	tree.Filter.SetText("")
}
