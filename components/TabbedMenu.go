package components

import (
	"github.com/rivo/tview"

	"lazysql/app"
)

type Item struct {
	*tview.TextView
}

type Tab struct {
	Page  *ResultsTable
	Name  string
	Index int
}

type TabbedPaneState struct {
	CurrentTab *Tab
	Tabs       []*Tab
}

type TabbedPane struct {
	*tview.Pages
	Wrapper *tview.Flex
	state   *TabbedPaneState
}

func NewTabbedPane() *TabbedPane {
	wrapper := tview.NewFlex()
	return &TabbedPane{
		Pages:   tview.NewPages(),
		Wrapper: wrapper,
		state:   &TabbedPaneState{},
	}
}

func (t *TabbedPane) AddTab(tab *Tab) {
	tabWithIndex := &Tab{
		Page:  tab.Page,
		Name:  tab.Name,
		Index: len(t.state.Tabs),
	}
	t.state.Tabs = append(t.state.Tabs, tabWithIndex)
	t.state.CurrentTab = tabWithIndex

	textView := tview.NewTextView()
	textView.SetText(tab.Name)
	textView.SetDynamicColors(true)
	item := &Item{textView}

	t.Wrapper.AddItem(item, len(tabWithIndex.Name)+1, 1, false)
	t.HighlightTabItem(len(t.state.Tabs) - 1)

	t.AddAndSwitchToPage(tab.Name, tab.Page.Page, true)
}

func (t *TabbedPane) RemoveTab(index int) {
	tab := t.state.Tabs[index]
	t.RemovePage(tab.Name)
	t.state.Tabs = append(t.state.Tabs[:index], t.state.Tabs[index+1:]...)
	item := t.Wrapper.GetItem(index)
	t.Wrapper.RemoveItem(item)

	if t.GetTabCount() > 0 {
		t.SwitchToPreviousTab()
	} else {
		t.state.CurrentTab = nil
	}
}

func (t *TabbedPane) SetCurrentTab(index int) *Tab {
	tab := t.state.Tabs[index]

	t.state.CurrentTab = tab

	t.SwitchToPage(t.state.Tabs[index].Name)

	t.HighlightTabItem(index)

	app.App.SetFocus(tab.Page.Page)

	return tab
}

func (t *TabbedPane) GetCurrentTab() *Tab {
	return t.state.CurrentTab
}

func (t *TabbedPane) GetCurrentTabName() string {
	return t.state.CurrentTab.Name
}

func (t *TabbedPane) GetCurrentTabPrimitive() tview.Primitive {
	return t.state.CurrentTab.Page
}

func (t *TabbedPane) GetTabs() []*Tab {
	return t.state.Tabs
}

func (t *TabbedPane) GetTabByName(name string) *Tab {
	for _, tab := range t.state.Tabs {
		if tab.Name == name {
			return tab
		}
	}

	return nil
}

func (t *TabbedPane) GetTabByIndex(index int) *Tab {
	return t.state.Tabs[index]
}

func (t *TabbedPane) GetTabIndexByName(name string) int {
	for i, tab := range t.state.Tabs {
		if tab.Name == name {
			return i
		}
	}

	return -1
}

func (t *TabbedPane) GetTabCount() int {
	return len(t.state.Tabs)
}

func (t *TabbedPane) SwitchToTab(name string) {
	for i, tab := range t.state.Tabs {
		if tab.Name == name {
			t.SetCurrentTab(i)
			break
		}
	}

	index := t.GetTabIndexByName(name)

	item := t.Wrapper.GetItem(index).(*Item)
	item.SetTextColor(app.ActiveTextColor)
}

// switch to last tab
func (t *TabbedPane) SwitchToLastTab() *Tab {
	t.SetCurrentTab(t.GetTabCount() - 1)
	return t.state.CurrentTab
}

// switch to first tab
func (t *TabbedPane) SwitchToFirstTab() *Tab {
	t.SetCurrentTab(0)
	return t.state.CurrentTab
}

// switch to next tab
func (t *TabbedPane) SwitchToNextTab() *Tab {
	if t.state.CurrentTab != nil {
		if t.state.CurrentTab.Index == t.GetTabCount()-1 {
			t.SwitchToFirstTab()
		} else {
			t.SetCurrentTab(t.state.CurrentTab.Index + 1)
		}
	}

	return t.state.CurrentTab
}

// switch to previous tab
func (t *TabbedPane) SwitchToPreviousTab() *Tab {
	if t.state.CurrentTab != nil {
		if t.state.CurrentTab.Index == 0 {
			t.SwitchToLastTab()
		} else {
			t.SetCurrentTab(t.state.CurrentTab.Index - 1)
		}
	}

	return t.state.CurrentTab
}

func (t *TabbedPane) HighlightTabItem(index int) {
	itemCount := t.Wrapper.GetItemCount()

	for i := 0; i < itemCount; i++ {
		if i == index {
			t.Wrapper.GetItem(i).(*Item).SetTextColor(app.ActiveTextColor)
		} else {
			t.Wrapper.GetItem(i).(*Item).SetTextColor(app.FocusTextColor)
		}
	}
}
