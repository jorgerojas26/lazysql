package components

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
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
	tree.SetGraphicsColor(tview.Styles.PrimaryTextColor)
	// tree.SetBorder(true)
	tree.SetTitle("Databases")
	tree.SetTitleAlign(tview.AlignLeft)
	// tree.SetBorderPadding(0, 0, 1, 1)

	rootNode := tview.NewTreeNode("-")
	tree.SetRoot(rootNode)
	tree.SetCurrentNode(rootNode)

	tree.SetFocusFunc(func() {
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

		if tree.GetSelectedDatabase() == "" {
			for _, database := range databases {
				childNode := tview.NewTreeNode(database)
				childNode.SetExpanded(false)
				childNode.SetReference(database)
				childNode.SetColor(tview.Styles.PrimaryTextColor)
				rootNode.AddChild(childNode)

				go func(database string, node *tview.TreeNode) {
					tables, err := tree.DBDriver.GetTables(database)
					if err != nil {
						logger.Error(err.Error(), nil)
						return
					}

					tree.databasesToNodes(tables, node, true)
					App.Draw()
				}(database, childNode)
			}
		}
		tree.SetFocusFunc(nil)
	})

	selectedNodeTextColor := fmt.Sprintf("[black:%s]", tview.Styles.SecondaryTextColor.Name())
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
		splittedNodeText := strings.Split(previousNodeText, selectedNodeTextColor)
		if len(splittedNodeText) > 1 {
			previouslyFocusedNode.SetText(splittedNodeText[1])
		}
		previouslyFocusedNode = node
	})

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		if node.GetLevel() == 1 {
			if node.IsExpanded() {
				node.SetExpanded(false)
			} else {
				tree.SetSelectedDatabase(node.GetReference().(string))

				// if node.GetChildren() == nil {
				// 	tables, err := tree.DBDriver.GetTables(tree.GetSelectedDatabase())
				// 	if err != nil {
				// 		// TODO: Handle error
				// 		return
				// 	}
				//
				// 	tree.databasesToNodes(tables, node, true)
				// }
				node.SetExpanded(true)

			}
		} else if node.GetLevel() == 2 {
			if node.GetChildren() == nil {
				nodeReference := node.GetReference().(string)
				split := strings.Split(nodeReference, ".")
				databaseName := ""
				tableName := ""

				if len(split) == 1 {
					tableName = split[0]
				} else if len(split) > 1 {
					databaseName = split[0]
					tableName = split[1]
				}

				tree.SetSelectedDatabase(databaseName)
				tree.SetSelectedTable(tableName)
			} else {
				node.SetExpanded(!node.IsExpanded())
			}
		} else if node.GetLevel() == 3 {
			nodeReference := node.GetReference().(string)
			split := strings.Split(nodeReference, ".")
			databaseName := split[0]
			schemaName := split[1]
			tableName := split[2]

			tree.SetSelectedDatabase(databaseName)
			tree.SetSelectedTable(fmt.Sprintf("%s.%s", schemaName, tableName))
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
		}
		return nil
	})

	tree.Filter.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:

			filterText := tree.Filter.GetText()

			if filterText == "" {
				tree.search("")
				tree.FoundNodeCountInput.SetText("")
				tree.SetBorderPadding(0, 0, 0, 0)
			} else {
				tree.FoundNodeCountInput.SetText(fmt.Sprintf("[%d/%d]", len(tree.state.searchFoundNodes), len(tree.state.searchFoundNodes)))
				tree.SetBorderPadding(1, 0, 0, 0)
			}

		case tcell.KeyEscape:
			tree.search("")
			tree.FoundNodeCountInput.SetText("")
			tree.SetBorderPadding(0, 0, 0, 0)
			tree.Filter.SetText("")
		}

		tree.SetIsFiltering(false)
		tree.Highlight()
		App.SetFocus(tree)
	})

	tree.Filter.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() != tcell.KeyEscape && event.Key() != tcell.KeyEnter {
			isBackSpace := event.Key() == tcell.KeyBackspace2

			filterText := tree.Filter.GetText()

			if isBackSpace {
				if len(filterText) > 0 {
					tree.search(filterText[:len(filterText)-1])
				} else {
					tree.search("")
				}
			} else {
				tree.search(filterText + string(event.Rune()))
			}
		}

		return event
	})

	tree.Filter.SetFieldStyle(tcell.StyleDefault.Background(tview.Styles.PrimitiveBackgroundColor).Foreground(tview.Styles.PrimaryTextColor))
	tree.Filter.SetPlaceholderStyle(tcell.StyleDefault.Background(tview.Styles.PrimitiveBackgroundColor).Foreground(tview.Styles.InverseTextColor))
	tree.Filter.SetBorderPadding(0, 0, 0, 0)
	tree.Filter.SetBorderColor(tview.Styles.PrimaryTextColor)
	tree.Filter.SetLabel("Search: ")
	tree.Filter.SetLabelColor(tview.Styles.InverseTextColor)

	tree.Filter.SetFocusFunc(func() {
		tree.Filter.SetLabelColor(tview.Styles.TertiaryTextColor)
		tree.Filter.SetFieldTextColor(tview.Styles.PrimaryTextColor)
	})

	tree.Filter.SetBlurFunc(func() {
		if tree.Filter.GetText() == "" {
			tree.Filter.SetLabelColor(tview.Styles.InverseTextColor)
		} else {
			tree.Filter.SetLabelColor(tview.Styles.TertiaryTextColor)
		}
		tree.Filter.SetFieldTextColor(tview.Styles.InverseTextColor)
	})

	tree.FoundNodeCountInput.SetFieldStyle(tcell.StyleDefault.Background(tview.Styles.PrimitiveBackgroundColor).Foreground(tview.Styles.PrimaryTextColor))

	tree.Wrapper.SetDirection(tview.FlexRow)
	tree.Wrapper.SetBorder(true)
	tree.Wrapper.SetBorderPadding(0, 0, 1, 1)
	tree.Wrapper.SetTitleColor(tview.Styles.PrimaryTextColor)

	tree.Wrapper.AddItem(tree.Filter, 1, 0, false)
	tree.Wrapper.AddItem(tree.FoundNodeCountInput, 1, 0, false)
	tree.Wrapper.AddItem(tree, 0, 1, true)

	return tree
}

func (tree *Tree) databasesToNodes(children map[string][]string, node *tview.TreeNode, defaultExpanded bool) {
	node.ClearChildren()

	for key, values := range children {
		var rootNode *tview.TreeNode

		nodeReference := node.GetReference().(string)

		if key != nodeReference {
			rootNode = tview.NewTreeNode(key)
			rootNode.SetExpanded(false)
			rootNode.SetReference(key)
			rootNode.SetColor(tview.Styles.PrimaryTextColor)
			node.AddChild(rootNode)
		}

		for _, child := range values {
			childNode := tview.NewTreeNode(child)
			childNode.SetExpanded(defaultExpanded)
			childNode.SetColor(tview.Styles.PrimaryTextColor)
			if tree.DBDriver.GetProvider() == "sqlite3" {
				childNode.SetReference(child)
			} else if tree.DBDriver.GetProvider() == "postgres" {
				childNode.SetReference(fmt.Sprintf("%s.%s.%s", nodeReference, key, child))
			} else {
				childNode.SetReference(fmt.Sprintf("%s.%s", key, child))
			}
			if rootNode != nil {
				rootNode.AddChild(childNode)
			} else {
				node.AddChild(childNode)
			}
		}
	}
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

	// filteredNodes := make([]*TreeStateNode, 0, len(treeNodes))

	rootNode.Walk(func(node, parent *tview.TreeNode) bool {
		nodeText := strings.ToLower(node.GetText())

		if strings.Contains(nodeText, lowerSearchText) {
			if parent != nil {
				parent.SetExpanded(true)
			}
			tree.state.searchFoundNodes = append(tree.state.searchFoundNodes, node)
			tree.SetCurrentNode(node)
			tree.state.currentFocusFoundNode = node
		}
		return true
	})

	App.ForceDraw()
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
		Key:   "SelectedDatabase",
		Value: database,
	})
}

func (tree *Tree) SetSelectedTable(table string) {
	tree.state.selectedTable = table
	tree.Publish(models.StateChange{
		Key:   "SelectedTable",
		Value: table,
	})
}

func (tree *Tree) SetIsFiltering(isFiltering bool) {
	tree.state.isFiltering = isFiltering
	tree.Publish(models.StateChange{
		Key:   "IsFiltering",
		Value: isFiltering,
	})
}

// Blur func
func (tree *Tree) RemoveHighlight() {
	tree.SetBorderColor(tview.Styles.InverseTextColor)
	tree.SetGraphicsColor(tview.Styles.InverseTextColor)
	tree.SetTitleColor(tview.Styles.InverseTextColor)
	// tree.GetRoot().SetColor(tview.Styles.InverseTextColor)

	childrens := tree.GetRoot().GetChildren()

	for _, children := range childrens {
		currentColor := children.GetColor()

		childrenIsCurrentNode := children.GetReference() == tree.GetCurrentNode().GetReference()

		if !childrenIsCurrentNode && currentColor == tview.Styles.PrimaryTextColor {
			children.SetColor(tview.Styles.InverseTextColor)
		}

		childrenOfChildren := children.GetChildren()

		for _, children := range childrenOfChildren {
			currentColor := children.GetColor()

			childrenIsCurrentNode := children.GetReference() == tree.GetCurrentNode().GetReference()

			if !childrenIsCurrentNode && currentColor == tview.Styles.PrimaryTextColor {
				children.SetColor(tview.Styles.InverseTextColor)
			}

		}

	}
}

func (tree *Tree) ForceRemoveHighlight() {
	tree.SetBorderColor(tview.Styles.InverseTextColor)
	tree.SetGraphicsColor(tview.Styles.InverseTextColor)
	tree.SetTitleColor(tview.Styles.InverseTextColor)
	tree.GetRoot().SetColor(tview.Styles.InverseTextColor)

	childrens := tree.GetRoot().GetChildren()

	for _, children := range childrens {

		children.SetColor(tview.Styles.InverseTextColor)

		childrenOfChildren := children.GetChildren()

		for _, children := range childrenOfChildren {
			children.SetColor(tview.Styles.InverseTextColor)
		}

	}
}

// Focus func
func (tree *Tree) Highlight() {
	tree.SetBorderColor(tview.Styles.PrimaryTextColor)
	tree.SetGraphicsColor(tview.Styles.PrimaryTextColor)
	tree.SetTitleColor(tview.Styles.PrimaryTextColor)
	tree.GetRoot().SetColor(tview.Styles.PrimaryTextColor)

	childrens := tree.GetRoot().GetChildren()

	for _, children := range childrens {
		currentColor := children.GetColor()

		if currentColor == tview.Styles.InverseTextColor {
			children.SetColor(tview.Styles.PrimaryTextColor)

			childrenOfChildren := children.GetChildren()

			for _, children := range childrenOfChildren {
				currentColor := children.GetColor()

				if currentColor == tview.Styles.InverseTextColor {
					children.SetColor(tview.Styles.PrimaryTextColor)
				}
			}

		}

	}
}

func (tree *Tree) goToNextFoundNode() {
	foundNodesText := make([]string, len(tree.state.searchFoundNodes))

	for i, node := range tree.state.searchFoundNodes {
		foundNodesText[i] = node.GetText()
	}

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
