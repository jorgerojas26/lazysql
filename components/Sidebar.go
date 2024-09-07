package components

import "github.com/rivo/tview"

type Sidebar struct {
	*tview.Flex
}

func NewSidebar() *Sidebar {
	sidebar := tview.NewFlex().SetDirection(tview.FlexColumnCSS)
	sidebar.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	sidebar.SetBorder(true)

	return &Sidebar{sidebar}
}
