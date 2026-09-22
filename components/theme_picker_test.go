package components

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestThemePickerVimNavigation(t *testing.T) {
	tests := []struct {
		name  string
		event *tcell.EventKey
		want  tcell.Key
	}{
		{"j moves down", tcell.NewEventKey(tcell.KeyRune, 'j', tcell.ModNone), tcell.KeyDown},
		{"k moves up", tcell.NewEventKey(tcell.KeyRune, 'k', tcell.ModNone), tcell.KeyUp},
		{"ctrl-n moves down", tcell.NewEventKey(tcell.KeyCtrlN, 0, tcell.ModCtrl), tcell.KeyDown},
		{"ctrl-p moves up", tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModCtrl), tcell.KeyUp},
		{"q closes", tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone), tcell.KeyEscape},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := themePickerNavigationEvent(tt.event).Key(); got != tt.want {
				t.Fatalf("translated key = %v, want %v", got, tt.want)
			}
		})
	}
}
