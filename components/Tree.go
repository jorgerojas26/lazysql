package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"lazysql/app"
	"lazysql/drivers"
)

type TreeState struct {
	SelectedDatabase string
	SelectedTable    string
}

type Tree struct {
	*tview.TreeView
	state       *TreeState
	subscribers []chan StateChange
}

func NewTree() *Tree {
	state := &TreeState{
		SelectedDatabase: "",
		SelectedTable:    "",
	}

	tree := &Tree{
		TreeView:    tview.NewTreeView(),
		state:       state,
		subscribers: []chan StateChange{},
	}

	tree.SetTopLevel(1)
	tree.SetGraphicsColor(app.ActiveTextColor)
	tree.SetBorder(true)
	tree.SetTitle("Databases")
	tree.SetTitleAlign(tview.AlignLeft)
	tree.SetBorderPadding(0, 0, 1, 1)

	rootNode := tview.NewTreeNode("-")
	tree.SetRoot(rootNode)
	tree.SetCurrentNode(rootNode)

	tree.SetFocusFunc(func() {
		databases, err := drivers.Database.GetDatabases()
		if err != nil {
			panic(err)
		}

		if tree.GetSelectedDatabase() == "" {
			tree.updateNodes(databases, rootNode, false)
		}
	})

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		if node.GetLevel() == 1 {
			if node.IsExpanded() {
				node.SetExpanded(false)
			} else {
				tree.SetSelectedDatabase(node.GetText())

				tables, err := drivers.Database.GetTables(state.SelectedDatabase)
				if err != nil {
					// TODO: Handle error
					return
				}

				tree.updateNodes(tables, node, true)
				for _, node := range node.GetChildren() {
					node.SetColor(app.ActiveTextColor)
				}
				node.SetExpanded(true)

			}
		} else if node.GetLevel() == 2 {
			tree.SetSelectedTable(fmt.Sprintf("%s.%s", node.GetReference(), node.GetText()))
		}
	})

	return tree
}

func (tree *Tree) updateNodes(children []string, node *tview.TreeNode, defaultExpanded bool) {
	node.ClearChildren()

	for _, child := range children {
		childNode := tview.NewTreeNode(child)
		childNode.SetExpanded(defaultExpanded)
		childNode.SetReference(node.GetText())
		childNode.SetColor(tcell.ColorWhite.TrueColor())
		node.AddChild(childNode)
	}
}

// Subscribe to changes in the tree state
func (tree *Tree) Subscribe() chan StateChange {
	subscriber := make(chan StateChange)
	tree.subscribers = append(tree.subscribers, subscriber)
	return subscriber
}

// Notify subscribers of changes in the tree state
func (tree *Tree) Notify(change StateChange) {
	for _, subscriber := range tree.subscribers {
		subscriber <- change
	}
}

// Getters and Setters
func (tree *Tree) GetSelectedDatabase() string {
	return tree.state.SelectedDatabase
}

func (tree *Tree) GetSelectedTable() string {
	return tree.state.SelectedTable
}

func (tree *Tree) SetSelectedDatabase(database string) {
	tree.state.SelectedDatabase = database
	tree.Notify(StateChange{
		Key:   "SelectedDatabase",
		Value: database,
	})
}

func (tree *Tree) SetSelectedTable(table string) {
	tree.state.SelectedTable = table
	tree.Notify(StateChange{
		Key:   "SelectedTable",
		Value: table,
	})
}

// Blur func
func (tree *Tree) RemoveHighlight() {
	tree.SetBorderColor(app.BlurTextColor)
	tree.SetGraphicsColor(app.BlurTextColor)
	tree.SetTitleColor(app.BlurTextColor)
	tree.GetRoot().SetColor(app.BlurTextColor)

	childrens := tree.GetRoot().GetChildren()

	for _, children := range childrens {
		children.SetColor(app.BlurTextColor)

		childrenOfChildren := children.GetChildren()

		for _, children := range childrenOfChildren {
			children.SetColor(app.BlurTextColor)
		}

	}
}

// Focus func
func (tree *Tree) Highlight() {
	tree.SetBorderColor(tcell.ColorWhite.TrueColor())
	tree.SetGraphicsColor(app.ActiveTextColor)
	tree.SetTitleColor(tcell.ColorWhite.TrueColor())
	tree.GetRoot().SetColor(tcell.ColorWhite.TrueColor())

	childrens := tree.GetRoot().GetChildren()

	for _, children := range childrens {
		children.SetColor(tcell.ColorWhite.TrueColor())

		childrenOfChildren := children.GetChildren()

		for _, children := range childrenOfChildren {
			children.SetColor(app.ActiveTextColor)
		}

	}
}
