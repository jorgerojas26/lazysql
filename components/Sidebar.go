package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/models"
)

type SidebarState struct {
	currentFieldIndex int
}

type Sidebar struct {
	*tview.Frame
	Flex        *tview.Flex
	state       *SidebarState
	Fields      []*tview.TextArea
	subscribers []chan models.StateChange
}

func NewSidebar() *Sidebar {
	flex := tview.NewFlex().SetDirection(tview.FlexColumnCSS)
	frame := tview.NewFrame(flex)
	frame.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	frame.SetBorder(true)

	sidebarState := &SidebarState{
		currentFieldIndex: 0,
	}

	newSidebar := &Sidebar{
		Frame:       frame,
		Flex:        flex,
		state:       sidebarState,
		Fields:      []*tview.TextArea{},
		subscribers: []chan models.StateChange{},
	}

	newSidebar.SetInputCapture(newSidebar.inputCapture)

	newSidebar.SetBlurFunc(func() {
		newSidebar.SetCurrentFieldIndex(0)
	})

	return newSidebar
}

func (sidebar *Sidebar) AddField(title, text string, fieldWidth int) {
	field := tview.NewTextArea()
	field.SetWrap(true)
	field.SetDisabled(true)

	field.SetBorder(true)
	field.SetTitle(title)
	field.SetTitleAlign(tview.AlignLeft)
	field.SetTitleColor(tview.Styles.PrimaryTextColor)
	field.SetText(text, true)
	field.SetTextStyle(tcell.StyleDefault.Background(tview.Styles.PrimitiveBackgroundColor).Foreground(tview.Styles.SecondaryTextColor))

	textLength := len(field.GetText())

	itemFixedSize := 3

	if textLength >= fieldWidth*3 {
		itemFixedSize = 5
	} else if textLength >= fieldWidth {
		itemFixedSize = 4
	} else {
		field.SetWrap(false)
	}

	sidebar.Fields = append(sidebar.Fields, field)
	sidebar.Flex.AddItem(field, itemFixedSize, 0, true)
}

func (sidebar *Sidebar) FocusNextField() {
	newIndex := sidebar.GetCurrentFieldIndex() + 1

	if newIndex < sidebar.Flex.GetItemCount() {
		item := sidebar.Fields[newIndex]

		if item != nil {
			sidebar.SetCurrentFieldIndex(newIndex)
			App.SetFocus(item)
			App.ForceDraw()
			return
		}

	}
}

func (sidebar *Sidebar) FocusPreviousField() {
	newIndex := sidebar.GetCurrentFieldIndex() - 1

	if newIndex >= 0 {
		item := sidebar.Fields[newIndex]

		if item != nil {
			sidebar.SetCurrentFieldIndex(newIndex)
			App.SetFocus(item)
			App.ForceDraw()
			return
		}
	}
}

func (sidebar *Sidebar) FocusFirstField() {
	sidebar.SetCurrentFieldIndex(0)
	App.SetFocus(sidebar.Fields[0])
}

func (sidebar *Sidebar) FocusLastField() {
	newIndex := sidebar.Flex.GetItemCount() - 1
	sidebar.SetCurrentFieldIndex(newIndex)
	App.SetFocus(sidebar.Fields[newIndex])
}

func (sidebar *Sidebar) FocusField(index int) {
	sidebar.SetCurrentFieldIndex(index)
	App.SetFocus(sidebar.Fields[index])
}

func (sidebar *Sidebar) Clear() {
	sidebar.Fields = make([]*tview.TextArea, 0)
	sidebar.Flex.Clear()
}

func (sidebar *Sidebar) EditTextCurrentField() {
	index := sidebar.GetCurrentFieldIndex()
	item := sidebar.Fields[index]

	sidebar.SetEditingStyles(item)
}

func (sidebar *Sidebar) inputCapture(event *tcell.EventKey) *tcell.EventKey {
	command := app.Keymaps.Group(app.SidebarGroup).Resolve(event)

	switch command {
	case commands.UnfocusSidebar:
		sidebar.Publish(models.StateChange{Key: UnfocusingSidebar, Value: nil})
	case commands.ToggleSidebar:
		sidebar.Publish(models.StateChange{Key: TogglingSidebar, Value: nil})
	case commands.MoveDown:
		sidebar.FocusNextField()
	case commands.MoveUp:
		sidebar.FocusPreviousField()
	case commands.GotoStart:
		sidebar.FocusFirstField()
	case commands.GotoEnd:
		sidebar.FocusLastField()
	case commands.Edit:
		sidebar.Publish(models.StateChange{Key: EditingSidebar, Value: true})

		currentItemIndex := sidebar.GetCurrentFieldIndex()
		item := sidebar.Fields[currentItemIndex]
		text := item.GetText()

		sidebar.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			command := app.Keymaps.Group(app.SidebarGroup).Resolve(event)

			switch command {
			case commands.CommitEdit:
				sidebar.SetInputCapture(sidebar.inputCapture)
				sidebar.SetDisabledStyles(item)
				sidebar.Publish(models.StateChange{Key: EditingSidebar, Value: false})
				return nil
			case commands.DiscardEdit:
				sidebar.SetInputCapture(sidebar.inputCapture)
				sidebar.SetDisabledStyles(item)
				item.SetText(text, true)
				sidebar.Publish(models.StateChange{Key: EditingSidebar, Value: false})
				return nil
			}

			return event
		})

		sidebar.EditTextCurrentField()

		return nil
	}
	return event
}

func (sidebar *Sidebar) SetEditingStyles(item *tview.TextArea) {
	item.SetBackgroundColor(tview.Styles.SecondaryTextColor)
	item.SetTextStyle(tcell.StyleDefault.Background(tview.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))
	item.SetTitleColor(tview.Styles.ContrastSecondaryTextColor)
	item.SetBorderColor(tview.Styles.ContrastSecondaryTextColor)

	item.SetWrap(true)
	item.SetDisabled(false)
}

func (sidebar *Sidebar) SetDisabledStyles(item *tview.TextArea) {
	item.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	item.SetTextStyle(tcell.StyleDefault.Background(tview.Styles.PrimitiveBackgroundColor).Foreground(tview.Styles.SecondaryTextColor))
	item.SetTitleColor(tview.Styles.PrimaryTextColor)
	item.SetBorderColor(tview.Styles.BorderColor)

	item.SetWrap(true)
	item.SetDisabled(true)
}

// Getters
func (sidebar *Sidebar) GetCurrentFieldIndex() int {
	return sidebar.state.currentFieldIndex
}

// Setters
func (sidebar *Sidebar) SetCurrentFieldIndex(index int) {
	sidebar.state.currentFieldIndex = index
}

// Subscribe to changes in the sidebar state
func (sidebar *Sidebar) Subscribe() chan models.StateChange {
	subscriber := make(chan models.StateChange)
	sidebar.subscribers = append(sidebar.subscribers, subscriber)
	return subscriber
}

// Publish subscribers of changes in the sidebar state
func (sidebar *Sidebar) Publish(change models.StateChange) {
	for _, subscriber := range sidebar.subscribers {
		subscriber <- change
	}
}
