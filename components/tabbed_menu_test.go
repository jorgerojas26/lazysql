package components

import (
	"testing"

	"github.com/rivo/tview"
)

type stubTabContent struct{}

func (stubTabContent) GetPrimitive() tview.Primitive { return tview.NewBox() }

func newTestTabbedPane(t *testing.T, names ...string) *TabbedPane {
	t.Helper()

	pane := NewTabbedPane()
	for _, name := range names {
		pane.AppendTab(name, stubTabContent{}, name)
	}
	return pane
}

func TestAlignHeaderToWidth(t *testing.T) {
	tests := []struct {
		name        string
		names       []string
		current     string
		width       int
		wantOffset  int
		wantHiddenL bool
		wantHiddenR bool
	}{
		{
			name:        "all tabs fit keeps the strip unscrolled",
			names:       []string{"users", "orders", "products"},
			current:     "products",
			width:       80,
			wantOffset:  0,
			wantHiddenL: false,
			wantHiddenR: false,
		},
		{
			name:        "current tab at the end scrolls left",
			names:       []string{"users", "orders", "products", "invoices", "sessions"},
			current:     "sessions",
			width:       36,
			wantOffset:  2,
			wantHiddenL: true,
			wantHiddenR: false,
		},
		{
			name:        "current tab in the middle scrolls right",
			names:       []string{"users", "orders", "products", "invoices", "sessions"},
			current:     "products",
			width:       36,
			wantOffset:  0,
			wantHiddenL: false,
			wantHiddenR: true,
		},
		{
			name:        "tab wider than the strip is still scrolled to it",
			names:       []string{"a-very-long-tab-name", "b"},
			current:     "a-very-long-tab-name",
			width:       10,
			wantOffset:  0,
			wantHiddenL: false,
			wantHiddenR: true,
		},
		{
			name:        "no width keeps the strip at the first column",
			names:       []string{"users", "orders"},
			current:     "orders",
			width:       0,
			wantOffset:  0,
			wantHiddenL: false,
			wantHiddenR: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pane := newTestTabbedPane(t, tt.names...)
			pane.SetCurrentTab(pane.GetTabByName(tt.current))

			pane.alignHeaderToWidth(tt.width)

			_, offset := pane.HeaderContainer.GetOffset()
			if offset != tt.wantOffset {
				t.Errorf("offset = %d, want %d", offset, tt.wantOffset)
			}
			if pane.headerHasHiddenLeft != tt.wantHiddenL {
				t.Errorf("hiddenLeft = %v, want %v", pane.headerHasHiddenLeft, tt.wantHiddenL)
			}
			if pane.headerHasHiddenRight != tt.wantHiddenR {
				t.Errorf("hiddenRight = %v, want %v", pane.headerHasHiddenRight, tt.wantHiddenR)
			}
		})
	}
}

func TestHeaderRightShift(t *testing.T) {
	tests := []struct {
		name      string
		names     []string
		current   string
		width     int
		wantShift int
	}{
		{
			name:      "scrolled to the last tab parks the strip at the right edge",
			names:     []string{"users", "orders", "products", "invoices", "sessions"},
			current:   "sessions",
			width:     36,
			wantShift: 6, // 36 - (10 + 10 + 10)
		},
		{
			name:      "exact fit leaves no shift",
			names:     []string{"users", "orders", "products", "invoices", "sessions"},
			current:   "sessions",
			width:     30,
			wantShift: 0,
		},
		{
			name:      "all tabs fit keeps the strip left-aligned",
			names:     []string{"users", "orders", "products", "invoices", "sessions"},
			current:   "sessions",
			width:     80,
			wantShift: 0,
		},
		{
			name:      "middle tab never shifts",
			names:     []string{"users", "orders", "products", "invoices", "sessions"},
			current:   "products",
			width:     36,
			wantShift: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pane := newTestTabbedPane(t, tt.names...)
			pane.SetCurrentTab(pane.GetTabByName(tt.current))

			pane.alignHeaderToWidth(tt.width)

			if pane.headerRightShift != tt.wantShift {
				t.Errorf("rightShift = %d, want %d", pane.headerRightShift, tt.wantShift)
			}
		})
	}
}
