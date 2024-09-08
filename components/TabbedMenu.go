package components

import (
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

type Header struct {
	*tview.TextView
}

type Tab struct {
	Content     *ResultsTable
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

type TabbedPane struct {
	*tview.Pages
	HeaderContainer *tview.Flex
	state           *TabbedPaneState
}

func NewTabbedPane() *TabbedPane {
	container := tview.NewFlex()
	container.SetBorderPadding(0, 0, 1, 1)

	return &TabbedPane{
		Pages:           tview.NewPages(),
		HeaderContainer: container,
		state:           &TabbedPaneState{},
	}
}

func (t *TabbedPane) AppendTab(name string, content *ResultsTable, reference string) {
	textView := tview.NewTextView()
	textView.SetText(name)
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

	t.HeaderContainer.AddItem(newTab.Header, len(newTab.Name)+2, 0, false)

	t.HighlightTabHeader(newTab)

	t.AddAndSwitchToPage(reference, content.Page, true)
}

func (t *TabbedPane) RemoveCurrentTab() {
	currentTab := t.state.CurrentTab

	if currentTab != nil {
		t.HeaderContainer.RemoveItem(currentTab.Header)
		t.RemovePage(currentTab.Reference)

		t.state.Length--

		if t.state.Length == 0 {
			t.state.FirstTab = nil
			t.state.LastTab = nil
			t.state.CurrentTab = nil
			return
		}

		if currentTab == t.state.FirstTab {
			t.state.FirstTab = currentTab.NextTab
		}

		if currentTab == t.state.LastTab {
			t.state.LastTab = currentTab.PreviousTab
		}

		if currentTab.PreviousTab != nil {
			currentTab.PreviousTab.NextTab = currentTab.NextTab
			t.SetCurrentTab(currentTab.PreviousTab)
		}

		if currentTab.NextTab != nil {
			currentTab.NextTab.PreviousTab = currentTab.PreviousTab
			t.SetCurrentTab(currentTab.NextTab)
		}

	}
}

func (t *TabbedPane) SetCurrentTab(tab *Tab) *Tab {
	t.state.CurrentTab = tab
	t.HighlightTabHeader(tab)

	t.SwitchToPage(tab.Reference)

	app.App.SetFocus(tab.Content.Page)

	return tab
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
	tabToHighlight := t.state.FirstTab

	for i := 0; tabToHighlight != nil && i < t.state.Length; i++ {
		if tabToHighlight.Header == tab.Header {
			tabToHighlight.Header.SetTextColor(app.Styles.SecondaryTextColor)
		} else {
			tabToHighlight.Header.SetTextColor(app.Styles.PrimaryTextColor)
		}
		tabToHighlight = tabToHighlight.NextTab
	}
}

func (t *TabbedPane) Highlight() {
	tab := t.state.FirstTab

	for i := 0; tab != nil && i < t.state.Length; i++ {
		if tab == t.state.CurrentTab {
			tab.Header.SetTextColor(app.Styles.SecondaryTextColor)
		} else {
			tab.Header.SetTextColor(app.Styles.PrimaryTextColor)
		}
		tab = tab.NextTab
	}
}

func (t *TabbedPane) SetBlur() {
	tab := t.state.FirstTab

	for i := 0; tab != nil && i < t.state.Length; i++ {
		tab.Header.SetTextColor(app.Styles.InverseTextColor)
		tab = tab.NextTab
	}
}
