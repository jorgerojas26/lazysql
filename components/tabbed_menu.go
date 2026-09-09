package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

type Header struct {
	*tview.TextView
}

type TabContent interface {
	GetPrimitive() tview.Primitive
}

type Tab struct {
	Content     TabContent
	NextTab     *Tab
	PreviousTab *Tab
	Header      *Header
	Name        string
	Reference   string
}

type TabbedPaneState struct {
	CurrentTab *Tab
	FirstTab   *Tab
	LastTab    *Tab
	Length     int
}

const (
	// headerArrowWidth is the number of columns reserved on each side of the
	// header strip to show that tabs are hidden outside the visible area.
	headerArrowWidth = 2
)

type TabbedPane struct {
	*tview.Pages
	HeaderContainer      *tview.Grid
	state                *TabbedPaneState
	headerWidths         []int
	headerHasHiddenLeft  bool
	headerHasHiddenRight bool
	headerRightShift     int
}

func NewTabbedPane() *TabbedPane {
	container := tview.NewGrid()
	container.SetBorderPadding(0, 0, 1, 1)
	container.SetRows(1)

	tabbedPane := &TabbedPane{
		Pages:           tview.NewPages(),
		HeaderContainer: container,
		state:           &TabbedPaneState{},
	}

	container.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		innerWidth := width - 2

		leftReserve, rightReserve := 0, 0
		availWidth := innerWidth
		for {
			availWidth = innerWidth - leftReserve - rightReserve
			if availWidth <= 0 {
				// Not enough room left for the arrows: drop them instead of
				// oscillating between reserving and releasing space.
				leftReserve, rightReserve = 0, 0
				availWidth = innerWidth
				tabbedPane.alignHeaderToWidth(availWidth)
				break
			}

			tabbedPane.alignHeaderToWidth(availWidth)

			newLeft, newRight := 0, 0
			if tabbedPane.headerHasHiddenLeft {
				newLeft = headerArrowWidth
			}
			if tabbedPane.headerHasHiddenRight {
				newRight = headerArrowWidth
			}

			if newLeft == leftReserve && newRight == rightReserve {
				break
			}
			leftReserve, rightReserve = newLeft, newRight
		}

		if leftReserve > 0 {
			tview.Print(screen, "◀ ", x+1, y, leftReserve, tview.AlignLeft, app.Styles.GraphicsColor)
		}
		if rightReserve > 0 {
			tview.Print(screen, " ▶", x+1+innerWidth-rightReserve, y, rightReserve, tview.AlignRight, app.Styles.GraphicsColor)
		}

		// When the strip has scrolled to the last tab and does not fill the
		// available width, right-align it so the active tab touches the right
		// edge instead of leaving dead space after it.
		shift := tabbedPane.headerRightShift

		return x + 1 + leftReserve + shift, y, availWidth - shift, height
	})

	return tabbedPane
}

func (t *TabbedPane) AppendTab(name string, content TabContent, reference string) {
	textView := tview.NewTextView()
	textView.SetText(name)
	textView.SetTextAlign(tview.AlignCenter)
	item := &Header{textView}

	newTab := &Tab{
		Content:   content,
		Name:      name,
		Header:    item,
		Reference: reference,
	}

	t.state.Length++

	if t.state.LastTab == nil {
		t.state.FirstTab = newTab
		t.state.LastTab = newTab
		t.state.CurrentTab = newTab
	} else {
		newTab.PreviousTab = t.state.LastTab
		t.state.LastTab.NextTab = newTab
		t.state.LastTab = newTab
		t.state.CurrentTab = newTab
	}

	t.headerWidths = append(t.headerWidths, len(newTab.Name)+2)
	t.rebuildHeaderStrip()

	t.HighlightTabHeader(newTab)
	t.AlignHeaderToCurrentTab()

	t.AddAndSwitchToPage(reference, content.GetPrimitive(), true)
}

func (t *TabbedPane) RemoveCurrentTab() *Tab {
	currentTab := t.state.CurrentTab

	if currentTab != nil {
		index := 0
		for tab := t.state.FirstTab; tab != nil; tab = tab.NextTab {
			if tab == currentTab {
				break
			}
			index++
		}

		t.HeaderContainer.RemoveItem(currentTab.Header)
		t.RemovePage(currentTab.Reference)

		t.state.Length--

		if t.state.Length == 0 {
			t.state.FirstTab = nil
			t.state.LastTab = nil
			t.state.CurrentTab = nil

			t.headerWidths = nil
			t.rebuildHeaderStrip()

			return nil
		}

		if currentTab == t.state.FirstTab {
			t.state.FirstTab = currentTab.NextTab
		}

		if currentTab == t.state.LastTab {
			t.state.LastTab = currentTab.PreviousTab
		}

		if currentTab.PreviousTab != nil {
			currentTab.PreviousTab.NextTab = currentTab.NextTab
		}
		if currentTab.NextTab != nil {
			currentTab.NextTab.PreviousTab = currentTab.PreviousTab
		}

		if index < len(t.headerWidths) {
			t.headerWidths = append(t.headerWidths[:index], t.headerWidths[index+1:]...)
		}
		t.rebuildHeaderStrip()

		if currentTab.PreviousTab != nil {
			t.SetCurrentTab(currentTab.PreviousTab)
			return currentTab.PreviousTab
		}
		t.SetCurrentTab(currentTab.NextTab)
		return currentTab.NextTab
	}

	return nil
}

func (t *TabbedPane) SetCurrentTab(tab *Tab) *Tab {
	t.state.CurrentTab = tab
	t.HighlightTabHeader(tab)

	t.SwitchToPage(tab.Reference)

	t.AlignHeaderToCurrentTab()

	app.App.SetFocus(tab.Content.GetPrimitive())

	return tab
}

func (t *TabbedPane) rebuildHeaderStrip() {
	t.HeaderContainer.Clear()

	if len(t.headerWidths) == 0 {
		t.HeaderContainer.SetColumns()
		return
	}

	t.HeaderContainer.SetColumns(t.headerWidths...)

	tab := t.state.FirstTab
	for i := 0; tab != nil && i < len(t.headerWidths); i++ {
		// minGridWidth is 1 so tabs are still drawn (clipped) in narrow
		// terminals instead of being skipped by the grid.
		t.HeaderContainer.AddItem(tab.Header, 0, i, 1, 1, 1, 1, false)
		tab = tab.NextTab
	}
}

func (t *TabbedPane) AlignHeaderToCurrentTab() {
	if t.state.CurrentTab == nil || len(t.headerWidths) == 0 {
		return
	}
	_, _, width, _ := t.HeaderContainer.GetInnerRect()
	t.alignHeaderToWidth(width)
}

func (t *TabbedPane) alignHeaderToWidth(width int) {
	t.headerHasHiddenLeft = false
	t.headerHasHiddenRight = false
	t.headerRightShift = 0

	if width <= 0 || len(t.headerWidths) == 0 {
		t.HeaderContainer.SetOffset(0, 0)
		return
	}

	index := 0
	for tab := t.state.FirstTab; tab != nil; tab = tab.NextTab {
		if tab == t.state.CurrentTab {
			break
		}
		index++
	}
	if index >= len(t.headerWidths) {
		return
	}

	used := t.headerWidths[index]
	target := index
	for i := index - 1; i >= 0; i-- {
		if used+t.headerWidths[i] > width {
			break
		}
		used += t.headerWidths[i]
		target = i
	}

	visibleColumns := 0
	used = 0
	for i := target; i < len(t.headerWidths); i++ {
		if used+t.headerWidths[i] > width {
			break
		}
		used += t.headerWidths[i]
		visibleColumns++
	}

	t.headerHasHiddenLeft = target > 0
	t.headerHasHiddenRight = visibleColumns < len(t.headerWidths)-target

	// Reaching the last tab parks the remaining tabs flush against the right
	// edge of the available space (only when the strip is scrolled, so tabs
	// that all fit stay left-aligned like a browser bar).
	if t.headerHasHiddenLeft && !t.headerHasHiddenRight && t.state.CurrentTab == t.state.LastTab && used < width {
		t.headerRightShift = width - used
	}

	t.HeaderContainer.SetOffset(0, target)
}

func (t *TabbedPane) GetCurrentTab() *Tab {
	return t.state.CurrentTab
}

func (t *TabbedPane) GetTabByName(name string) *Tab {
	tab := t.state.FirstTab
	for i := 0; tab != nil && i < t.state.Length; i++ {
		if tab.Name == name {
			break
		}
		tab = tab.NextTab
	}

	return tab
}

func (t *TabbedPane) GetTabByReference(reference string) *Tab {
	tab := t.state.FirstTab

	for i := 0; tab != nil && i < t.state.Length; i++ {
		if tab.Reference == reference {
			break
		}
		tab = tab.NextTab
	}

	return tab
}

func (t *TabbedPane) GetLength() int {
	return t.state.Length
}

func (t *TabbedPane) SwitchToNextTab() *Tab {
	if t.state.CurrentTab != nil {
		if t.state.CurrentTab == t.state.LastTab {
			t.SetCurrentTab(t.state.FirstTab)
		} else {
			if t.state.CurrentTab.NextTab != nil {
				t.SetCurrentTab(t.state.CurrentTab.NextTab)
			}
		}
	}

	return t.state.CurrentTab
}

func (t *TabbedPane) SwitchToPreviousTab() *Tab {
	if t.state.CurrentTab != nil {
		if t.state.CurrentTab == t.state.FirstTab {
			t.SetCurrentTab(t.state.LastTab)
		} else {
			if t.state.CurrentTab.PreviousTab != nil {
				t.SetCurrentTab(t.state.CurrentTab.PreviousTab)
			}
		}
	}

	return t.state.CurrentTab
}

func (t *TabbedPane) SwitchToFirstTab() *Tab {
	if t.state.FirstTab != nil {
		t.SetCurrentTab(t.state.FirstTab)
	}

	return t.state.CurrentTab
}

func (t *TabbedPane) SwitchToLastTab() *Tab {
	if t.state.LastTab != nil {
		t.SetCurrentTab(t.state.LastTab)
	}

	return t.state.CurrentTab
}

func (t *TabbedPane) SwitchToTabByName(name string) *Tab {
	tab := t.state.FirstTab

	for i := 0; tab != nil && i < t.state.Length; i++ {
		if tab.Name == name {
			break
		}
		tab = tab.NextTab
	}

	if tab != nil {
		t.SetCurrentTab(tab)
		return tab
	}

	return nil
}

func (t *TabbedPane) SwitchToTabByReference(reference string) *Tab {
	tab := t.state.FirstTab

	for i := 0; tab != nil && i < t.state.Length; i++ {
		if tab.Reference == reference {
			break
		}
		tab = tab.NextTab
	}

	if tab != nil {
		t.SetCurrentTab(tab)
		return tab
	}

	return nil
}

func (t *TabbedPane) HighlightTabHeader(tab *Tab) {
	t.styleHeaders(tab, app.Styles.PrimaryTextColor)
}

func (t *TabbedPane) Highlight() {
	t.styleHeaders(t.state.CurrentTab, app.Styles.PrimaryTextColor)
}

func (t *TabbedPane) SetBlur() {
	t.styleHeaders(t.state.CurrentTab, app.Styles.InverseTextColor)
}

// styleHeaders paints the selected tab with a solid background and every other
// tab with plain text. The selected tab keeps its highlight while the pane is
// blurred so the active table stays identifiable.
func (t *TabbedPane) styleHeaders(selected *Tab, normalColor tcell.Color) {
	selectedStyle := tcell.StyleDefault.
		Background(app.Styles.SecondaryTextColor).
		Foreground(app.Styles.ContrastSecondaryTextColor)

	for tab := t.state.FirstTab; tab != nil; tab = tab.NextTab {
		if tab == selected {
			tab.Header.SetTextStyle(selectedStyle)
			continue
		}
		tab.Header.SetTextStyle(tcell.StyleDefault.Foreground(normalColor))
	}
}
