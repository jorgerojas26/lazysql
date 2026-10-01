package app

import (
	"testing"

	cmd "github.com/jorgerojas26/lazysql/commands"
)

func TestPerformanceKeybindingsAreDocumented(t *testing.T) {
	var exactCountDescription, cancelQueryDescription string
	for _, bind := range Keymaps.Group(TableGroup) {
		if bind.Cmd == cmd.ExactCount {
			exactCountDescription = bind.Description
		}
	}
	for _, bind := range Keymaps.Group(EditorGroup) {
		if bind.Cmd == cmd.CancelQuery {
			cancelQueryDescription = bind.Description
		}
	}

	if exactCountDescription == "" || exactCountDescription != "Calculate or cancel exact row count" {
		t.Fatalf("ExactCount help description = %q", exactCountDescription)
	}
	if cancelQueryDescription == "" || cancelQueryDescription != "Cancel active query; otherwise unfocus editor" {
		t.Fatalf("CancelQuery help description = %q", cancelQueryDescription)
	}
}
