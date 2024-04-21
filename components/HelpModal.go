package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type KeyBindings struct {
	Global    map[string]string
	Table     map[string]string
	Tree      map[string]string
	SQLEditor map[string]string
}

func (kb KeyBindings) String() string {
	var result string

	// Global
	result += "---Global---\n"
	for key, value := range kb.Global {
		result += fmt.Sprintf("%s: %s\n", key, value)
	}

	// Table
	result += "\n---Table---\n"
	for key, value := range kb.Table {
		result += fmt.Sprintf("%s: %s\n", key, value)
	}

	// Tree
	result += "\n---Tree---\n"
	for key, value := range kb.Tree {
		result += fmt.Sprintf("%s: %s\n", key, value)
	}

	// SQLEditor
	result += "\n---SQLEditor---\n"
	for key, value := range kb.SQLEditor {
		result += fmt.Sprintf("%s: %s\n", key, value)
	}
	return result
}

type HelpModal struct {
	*tview.Modal
}

func NewHelpModal() *HelpModal {
	keybindings := KeyBindings{
		Global: map[string]string{
			"?":         "Show help informations",
			"CTRL+e":    "Open SQL editor",
			"BACKSPACE": "Return to connection selection",
		},
		Table: map[string]string{
			"c":      "Edit table cell",
			"d":      "Delete row",
			"o":      "Add row",
			"/":      "Focus the filter input or SQL editor",
			"CTRL+s": "Commit changes",
			">":      "Next page",
			"<":      "Previous page",
			"K":      "Sort ASC",
			"J":      "Sort DESC",
			"H":      "Focus tree panel",
			"[":      "Focus previous tab",
			"]":      "Focus next tab",
			"X":      "Close current tab",
		},
		Tree: map[string]string{
			"L": "Focus table pane",
		},
		SQLEditor: map[string]string{
			"CTRL+R": "Run the SQL statement",
		},
	}
	modal := tview.NewModal().
		SetText("Help").
		AddButtons([]string{"ESC"}).
		SetBackgroundColor(tcell.ColorBlack).
		SetTextColor(tview.Styles.PrimaryTextColor).SetText(keybindings.String())

	return &HelpModal{
		Modal: modal,
	}
}
