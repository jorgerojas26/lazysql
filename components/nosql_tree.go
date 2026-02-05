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

type NoSQLTreeState struct {
	currentFocusFoundNode *tview.TreeNode
	selectedDatabase      string
	selectedCollection    string
	searchFoundNodes      []*tview.TreeNode
	isFiltering           bool
}

type NoSQLTree struct {
	DBDriver drivers.NoSQLDriver
	*tview.TreeView
	state               *NoSQLTreeState
	Filter              *tview.InputField
	Wrapper             *tview.Flex
	FoundNodeCountInput *tview.InputField
	subscribers         []chan models.StateChange
}

func NewNoSQLTree(dbName string, dbdriver drivers.NoSQLDriver) *NoSQLTree {
	state := &NoSQLTreeState{
		selectedDatabase:   "",
		selectedCollection: "",
	}

	tree := &NoSQLTree{
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
	tree.SetTitle("Databases")
	tree.SetTitleAlign(tview.AlignLeft)

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
		// NoSQL has simpler 2-level hierarchy: databases → collections
		if node.GetLevel() == 1 {
			// Database level - expand/collapse
			if node.IsExpanded() {
				node.SetExpanded(false)
			} else {
				tree.SetSelectedDatabase(node.GetReference().(string))
				node.SetExpanded(true)
			}
		} else if node.GetLevel() == 2 {
			// Collection level - select collection
			nodeReference := node.GetReference().(string)
			split := strings.Split(nodeReference, ".")
			databaseName := ""
			collectionName := ""

			if len(split) == 1 {
				collectionName = split[0]
			} else if len(split) >= 2 {
				databaseName = split[0]
				collectionName = split[1]
			}

			tree.SetSelectedDatabase(databaseName)
			tree.SetSelectedCollection(collectionName)
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

func (tree *NoSQLTree) collectionsToNodes(collections map[string][]string, node *tview.TreeNode) {
	node.ClearChildren()

	// Sort the keys and use them to loop over the
	// collections so they are always in the same order.
	sortedKeys := slices.Sorted(maps.Keys(collections))

	for _, key := range sortedKeys {
		values := collections[key]

		// Sort the values.
		sort.Strings(values)

		nodeReference := node.GetReference().(string)

		// For MongoDB, collections are flat (key is empty string "")
		// For Redis, collections are grouped by namespace
		for _, collection := range values {
			collectionNode := tview.NewTreeNode(collection)
			collectionNode.SetExpanded(false)
			collectionNode.SetColor(app.Styles.PrimaryTextColor)
			collectionNode.SetReference(fmt.Sprintf("%s.%s", nodeReference, collection))
			node.AddChild(collectionNode)
		}
	}
}

func prioritizeNoSQLResult(pattern, target string, fuzzyRank int) int {
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

func (tree *NoSQLTree) search(searchText string) {
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
	collectionNameFilter := ""

	if len(parts) == 1 {
		collectionNameFilter = parts[0]
	} else {
		databaseNameFilter = parts[0]
		collectionNameFilter = parts[1]
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
			rank := fuzzy.RankMatch(collectionNameFilter, nodeText)
			if rank >= 0 {
				if parent != nil {
					parent.SetExpanded(true)
				}
				adjustedRank := prioritizeNoSQLResult(collectionNameFilter, nodeText, rank)
				rankedNodes = append(rankedNodes, rankedNode{node: node, rank: adjustedRank})
			}
		} else {
			rank := fuzzy.RankMatch(collectionNameFilter, nodeText)
			if rank >= 0 && parent != nil {
				parentText := strings.ToLower(parent.GetText())
				parentRank := fuzzy.RankMatch(databaseNameFilter, parentText)
				if parentRank >= 0 {
					parent.SetExpanded(true)
					adjustedCollectionRank := prioritizeNoSQLResult(collectionNameFilter, nodeText, rank)
					adjustedParentRank := prioritizeNoSQLResult(databaseNameFilter, parentText, parentRank)
					// Combine ranks: prioritize collection match but factor in database match
					combinedRank := adjustedCollectionRank + (adjustedParentRank / 2)
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
func (tree *NoSQLTree) Subscribe() chan models.StateChange {
	subscriber := make(chan models.StateChange)
	tree.subscribers = append(tree.subscribers, subscriber)
	return subscriber
}

// Publish subscribers of changes in the tree state
func (tree *NoSQLTree) Publish(change models.StateChange) {
	for _, subscriber := range tree.subscribers {
		subscriber <- change
	}
}

// Getters and Setters
func (tree *NoSQLTree) GetSelectedDatabase() string {
	return tree.state.selectedDatabase
}

func (tree *NoSQLTree) GetSelectedCollection() string {
	return tree.state.selectedCollection
}

func (tree *NoSQLTree) GetIsFiltering() bool {
	return tree.state.isFiltering
}

func (tree *NoSQLTree) SetSelectedDatabase(database string) {
	tree.state.selectedDatabase = database
	tree.Publish(models.StateChange{
		Key:   eventNoSQLTreeSelectedDatabase,
		Value: database,
	})
}

func (tree *NoSQLTree) SetSelectedCollection(collection string) {
	tree.state.selectedCollection = collection
	tree.Publish(models.StateChange{
		Key:   eventNoSQLTreeSelectedCollection,
		Value: collection,
	})
}

func (tree *NoSQLTree) SetIsFiltering(isFiltering bool) {
	tree.state.isFiltering = isFiltering
	tree.Publish(models.StateChange{
		Key:   eventNoSQLTreeIsFiltering,
		Value: isFiltering,
	})
}

// Blur func
func (tree *NoSQLTree) RemoveHighlight() {
	tree.SetBorderColor(app.Styles.InverseTextColor)
	tree.SetGraphicsColor(app.Styles.InverseTextColor)
	tree.SetTitleColor(app.Styles.InverseTextColor)

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

func (tree *NoSQLTree) ForceRemoveHighlight() {
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
func (tree *NoSQLTree) Highlight() {
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

func (tree *NoSQLTree) goToNextFoundNode() {
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

func (tree *NoSQLTree) goToPreviousFoundNode() {
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

func (tree *NoSQLTree) CollapseAll() {
	tree.GetRoot().Walk(func(node, _ *tview.TreeNode) bool {
		if node.IsExpanded() && node != tree.GetRoot() {
			node.Collapse()
		}
		return true
	})
}

func (tree *NoSQLTree) ExpandAll() {
	tree.GetRoot().Walk(func(node, _ *tview.TreeNode) bool {
		if !node.IsExpanded() && node != tree.GetRoot() {
			node.Expand()
		}
		return true
	})
}

func (tree *NoSQLTree) InitializeNodes(dbName string) {
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
			collections, err := tree.DBDriver.GetCollections(database)
			if err != nil {
				logger.Error(err.Error(), nil)
				return
			}

			tree.collectionsToNodes(collections, node)
			App.Draw()
		}(database, childNode)
	}
}

func (tree *NoSQLTree) Refresh(dbName string) {
	rootNode := tree.GetRoot()
	rootNode.ClearChildren()
	// re-add nodes
	tree.InitializeNodes(dbName)
}

func (tree *NoSQLTree) ClearSearch() {
	tree.search("")
	tree.FoundNodeCountInput.SetText("")
	tree.SetBorderPadding(0, 0, 0, 0)
	tree.Filter.SetText("")
}
