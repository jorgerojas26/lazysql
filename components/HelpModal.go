package components

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type KD struct {
	Key  string
	Desc string
}
type KeyBindings struct {
	Global    []KD
	Table     []KD
	Tree      []KD
	SqlEditor []KD
}

func (kb KeyBindings) String() string {
	var result string

	// Global
	result += "---Global---\n"
	for _, kd := range kb.Global {
		result += fmt.Sprintf("%s: %s\n", kd.Key, kd.Desc)
	}

	// Table
	result += "\n---Table---\n"
	for _, kd := range kb.Table {
		result += fmt.Sprintf("%s: %s\n", kd.Key, kd.Desc)
	}

	// Tree
	result += "\n---Tree---\n"
	for _, kd := range kb.Tree {
		result += fmt.Sprintf("%s: %s\n", kd.Key, kd.Desc)
	}

	// SQLEditor
	result += "\n---SQLEditor---\n"
	for _, kd := range kb.SqlEditor {
		result += fmt.Sprintf("%s: %s\n", kd.Key, kd.Desc)
	}
	return result
}

type HelpModal struct {
	*tview.Modal
}

func NewHelpModal() *HelpModal {
	keybindings := KeyBindings{
		Global: []KD{
			{"?", "Show help informations"},
			{"CTRL+e", "Open SQL editor"},
			{"BACKSPACE", "Return to connection selection"},
		},
		Table: []KD{
			{"c", "Edit table cell"},
			{"d", "Delete row"},
			{"o", "Add row"},
			{"/", "Focus the filter input or SQL editor"},
			{"CTRL+s", "Commit changes"},
			{">", "Next page"},
			{"<", "Previous page"},
			{"K", "Sort ASC"},
			{"J", "Sort DESC"},
			{"H", "Focus tree panel"},
			{"[", "Focus previous tab"},
			{"]", "Focus next tab"},
			{"X", "Close current tab"},
		},
		Tree: []KD{
			{"L", "Focus table pane"},
		},
		SqlEditor: []KD{
			{"CTRL+R", "Run the SQL statement"},
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
