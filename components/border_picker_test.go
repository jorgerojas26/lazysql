package components

import (
	"testing"

	"github.com/jorgerojas26/lazysql/app"
)

func TestBorderPickerPreviewsBothTargetsAndReset(t *testing.T) {
	previous := app.App.BorderConfig()
	t.Cleanup(func() { _ = app.App.PreviewBorders(previous) })
	if err := app.App.PreviewBorders(app.BorderConfig{Style: "auto"}); err != nil {
		t.Fatal(err)
	}
	picker := NewBorderPicker(func() {})
	picker.setSelected("#6B7280")
	picker.focusColor = true
	picker.setSelected("#38BDF8")
	if got := app.App.BorderConfig(); got.NormalColor != "#6B7280" || got.FocusColor != "#38BDF8" {
		t.Fatalf("preview = %+v", got)
	}
	picker.reset()
	if got := app.App.BorderConfig(); got.NormalColor != "" || got.FocusColor != "" || got.Style != "auto" {
		t.Fatalf("reset preview = %+v", got)
	}
}
