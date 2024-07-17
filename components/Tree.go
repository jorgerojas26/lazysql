package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

type TreeState struct {
	selectedDatabase string
	selectedTable    string
}

type Tree struct {
	*tview.TreeView
	state       *TreeState
	DBDriver    drivers.Driver
	subscribers []chan models.StateChange
}

func NewTree(dbName string, dbdriver drivers.Driver) *Tree {
	state := &TreeState{
		selectedDatabase: "",
		selectedTable:    "",
	}

	tree := &Tree{
		TreeView:    tview.NewTreeView(),
		state:       state,
		subscribers: []chan models.StateChange{},
		DBDriver:    dbdriver,
	}

	tree.SetTopLevel(1)
	tree.SetGraphicsColor(tview.Styles.SecondaryTextColor)
	tree.SetBorder(true)
	tree.SetTitle("Databases")
	tree.SetTitleAlign(tview.AlignLeft)
	tree.SetBorderPadding(0, 0, 1, 1)

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
			for _, child := range databases {
				childNode := tview.NewTreeNode(child)
				childNode.SetExpanded(false)
				childNode.SetReference(child)
				childNode.SetColor(tview.Styles.PrimaryTextColor)
				rootNode.AddChild(childNode)
			}
		}
		tree.SetFocusFunc(nil)
	})

	tree.SetSelectedFunc(func(node *tview.TreeNode) {
		if node.GetLevel() == 1 {
			if node.IsExpanded() {
				node.SetExpanded(false)
			} else {
				tree.SetSelectedDatabase(node.GetText())

				if node.GetChildren() == nil {
					tables, err := tree.DBDriver.GetTables(tree.GetSelectedDatabase())
					if err != nil {
						// TODO: Handle error
						return
					}

					tree.updateNodes(tables, node, true)
				}
				node.SetExpanded(true)

			}
		} else if node.GetLevel() == 2 {
			if node.GetChildren() == nil {
				tableName := fmt.Sprintf("%s.%s", node.GetReference(), node.GetText())

				if tree.DBDriver.GetProvider() == "sqlite3" {
					tableName = node.GetText()
				}

				tree.SetSelectedTable(tableName)
			} else {
				node.SetExpanded(!node.IsExpanded())
			}
		} else if node.GetLevel() == 3 {
			tableName := fmt.Sprintf("%s.%s", node.GetReference(), node.GetText())

			if tree.DBDriver.GetProvider() == "sqlite3" {
				tableName = node.GetText()
			}

			tree.SetSelectedTable(tableName)
		}
	})

	tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := app.Keymaps.Group("tree").Resolve(event)

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
		}
		return nil
	})

	return tree
}

func (tree *Tree) updateNodes(children map[string][]string, node *tview.TreeNode, defaultExpanded bool) {
	node.ClearChildren()

	for key, values := range children {
		var rootNode *tview.TreeNode

		if key != node.GetReference().(string) {
			rootNode = tview.NewTreeNode(key)
			rootNode.SetExpanded(false)
			rootNode.SetReference(key)
			rootNode.SetColor(tview.Styles.PrimaryTextColor)
			node.AddChild(rootNode)
		}

		for _, child := range values {
			childNode := tview.NewTreeNode(child)
			childNode.SetExpanded(defaultExpanded)
			childNode.SetReference(key)
			childNode.SetColor(tview.Styles.SecondaryTextColor)
			if rootNode != nil {
				rootNode.AddChild(childNode)
			} else {
				node.AddChild(childNode)
			}
		}
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

// Blur func
func (tree *Tree) RemoveHighlight() {
	tree.SetBorderColor(tview.Styles.InverseTextColor)
	tree.SetGraphicsColor(tview.Styles.InverseTextColor)
	tree.SetTitleColor(tview.Styles.InverseTextColor)
	tree.GetRoot().SetColor(tview.Styles.InverseTextColor)

	childrens := tree.GetRoot().GetChildren()

	for _, children := range childrens {
		currentColor := children.GetColor()

		if currentColor == tview.Styles.PrimaryTextColor {
			children.SetColor(tview.Styles.InverseTextColor)
		}

		childrenOfChildren := children.GetChildren()

		for _, children := range childrenOfChildren {
			currentColor := children.GetColor()

			if currentColor == tview.Styles.PrimaryTextColor {
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
	tree.SetGraphicsColor(tview.Styles.SecondaryTextColor)
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
					children.SetColor(tview.Styles.SecondaryTextColor)
				}
			}

		}

	}
}
