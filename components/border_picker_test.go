package components

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

func TestBorderPickerPreviewsBothTargetsAndReset(t *testing.T) {
	previous := app.App.BorderConfig()
	t.Cleanup(func() { _ = app.App.PreviewBorders(previous) })
	if err := app.App.PreviewBorders(app.BorderConfig{Style: "auto"}); err != nil {
		t.Fatal(err)
	}
	picker := NewBorderPicker(func() {})
	picker.list.SetCurrentItem(1) // Slate: preview the normal border.
	picker.list.InputHandler()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone), func(tview.Primitive) {})
	picker.list.SetCurrentItem(2) // Sky: preview the focused border.
	if got := app.App.BorderConfig(); got.NormalColor != "#6B7280" || got.FocusColor != "#38BDF8" {
		t.Fatalf("preview = %+v", got)
	}
	picker.reset()
	if picker.list.GetCurrentItem() != 0 {
		t.Fatal("reset did not select theme colors")
	}
	if got := app.App.BorderConfig(); got.NormalColor != "" || got.FocusColor != "" || got.Style != "auto" {
		t.Fatalf("reset preview = %+v", got)
	}
}
