package components

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestTelemetryPromptDefaultsToNo(t *testing.T) {
	var choices []bool
	modal := newTelemetryModal("https://example.com/v1/events", func(yes bool) { choices = append(choices, yes) })
	if modal.form.GetButton(0).GetLabel() != "No" || modal.form.GetButton(1).GetLabel() != "Yes" {
		t.Fatal("default must be No")
	}
	var focus func(tview.Primitive)
	var current tview.Primitive
	focus = func(p tview.Primitive) {
		if current != nil {
			current.Blur()
		}
		current = p
		p.Focus(focus)
	}
	modal.Focus(focus)
	if !modal.form.GetButton(0).HasFocus() {
		t.Fatal("No button is not focused")
	}
	modal.form.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), focus)
	modal.form.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), focus)
	if len(choices) != 2 || choices[0] || choices[1] {
		t.Fatalf("expected default No and Esc No, got %v", choices)
	}
}

func TestTelemetryPromptRequiresDeliberateYes(t *testing.T) {
	var agreed bool
	modal := newTelemetryModal("https://example.com/v1/events", func(yes bool) { agreed = yes })
	var focus func(tview.Primitive)
	var current tview.Primitive
	focus = func(p tview.Primitive) {
		if current != nil {
			current.Blur()
		}
		current = p
		p.Focus(focus)
	}
	modal.Focus(focus)
	modal.form.InputHandler()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), focus)
	if !modal.form.GetButton(1).HasFocus() {
		t.Fatal("Yes button was not selected")
	}
	modal.form.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), focus)
	if !agreed {
		t.Fatal("Yes was not accepted")
	}
}
