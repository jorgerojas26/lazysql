package components

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/models"
)

type SidebarState struct {
	dbProvider        string
	currentFieldIndex int
}

type SidebarFieldParameters struct {
	OriginalValue string
	Height        int
}

type Sidebar struct {
	*tview.Frame
	Flex            *tview.Flex
	state           *SidebarState
	FieldParameters []*SidebarFieldParameters
	subscribers     []chan models.StateChange
}

func NewSidebar(dbProvider string) *Sidebar {
	flex := tview.NewFlex().SetDirection(tview.FlexColumnCSS)
	frame := tview.NewFrame(flex)
	frame.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	frame.SetBorder(true)
	frame.SetBorders(0, 0, 0, 0, 0, 0)

	sidebarState := &SidebarState{
		currentFieldIndex: 0,
		dbProvider:        dbProvider,
	}

	newSidebar := &Sidebar{
		Frame:       frame,
		Flex:        flex,
		state:       sidebarState,
		subscribers: []chan models.StateChange{},
	}

	newSidebar.SetInputCapture(newSidebar.inputCapture)

	newSidebar.SetBlurFunc(func() {
		newSidebar.SetCurrentFieldIndex(0)
	})

	return newSidebar
}

func (sidebar *Sidebar) AddField(title, text string, fieldWidth int, pendingEdit bool) {
	field := tview.NewTextArea()
	field.SetWrap(true)
	field.SetDisabled(true)

	field.SetBorder(true)
	field.SetTitle(title)
	field.SetTitleAlign(tview.AlignLeft)
	field.SetTitleColor(app.Styles.PrimaryTextColor)
	field.SetText(text, true)
	field.SetTextStyle(tcell.StyleDefault.Background(app.Styles.PrimitiveBackgroundColor).Foreground(tview.Styles.SecondaryTextColor))

	if pendingEdit {
		sidebar.SetEditedStyles(field)
	}

	textLength := len(field.GetText())

	itemFixedSize := 3

	if textLength >= fieldWidth*3 {
		itemFixedSize = 5
	} else if textLength >= fieldWidth {
		itemFixedSize = 4
	} else {
		field.SetWrap(false)
	}

	field.SetFocusFunc(func() {
		_, y, _, h := field.GetRect()
		_, _, _, mph := sidebar.GetRect()

		if y >= mph {
			hidingFieldIndex := 0
			fieldCount := sidebar.Flex.GetItemCount()

			for i := 0; i < fieldCount; i++ {
				f := sidebar.Flex.GetItem(i)
				_, _, _, h := f.GetRect()
				if h != 0 {
					hidingFieldIndex = i
					break
				}
			}

			sidebar.Flex.ResizeItem(sidebar.Flex.GetItem(hidingFieldIndex), 0, 0)
		} else if h == 0 {
			sidebar.Flex.ResizeItem(field, itemFixedSize, 0)
		}
	})

	fieldParameters := &SidebarFieldParameters{
		Height:        itemFixedSize,
		OriginalValue: text,
	}

	sidebar.FieldParameters = append(sidebar.FieldParameters, fieldParameters)
	sidebar.Flex.AddItem(field, itemFixedSize, 0, true)
}

func (sidebar *Sidebar) FocusNextField() {
	newIndex := sidebar.GetCurrentFieldIndex() + 1

	if newIndex >= sidebar.Flex.GetItemCount() {
		return
	}

	item := sidebar.Flex.GetItem(newIndex)

	if item == nil {
		return
	}

	sidebar.SetCurrentFieldIndex(newIndex)
	App.SetFocus(item)
	App.ForceDraw()
}

func (sidebar *Sidebar) FocusPreviousField() {
	newIndex := sidebar.GetCurrentFieldIndex() - 1

	if newIndex < 0 {
		return
	}

	item := sidebar.Flex.GetItem(newIndex)

	if item == nil {
		return
	}

	sidebar.SetCurrentFieldIndex(newIndex)
	App.SetFocus(item)
	App.ForceDraw()
}

func (sidebar *Sidebar) FocusFirstField() {
	sidebar.SetCurrentFieldIndex(0)
	App.SetFocus(sidebar.Flex.GetItem(0))

	fieldCount := sidebar.Flex.GetItemCount()

	for i := 0; i < fieldCount; i++ {
		field := sidebar.Flex.GetItem(i)
		height := sidebar.FieldParameters[i].Height
		sidebar.Flex.ResizeItem(field, height, 0)
	}
}

func (sidebar *Sidebar) FocusLastField() {
	newIndex := sidebar.Flex.GetItemCount() - 1
	sidebar.SetCurrentFieldIndex(newIndex)
	App.SetFocus(sidebar.Flex.GetItem(newIndex))

	_, _, _, ph := sidebar.GetRect()

	hSum := 0

	for i := sidebar.Flex.GetItemCount() - 1; i >= 0; i-- {
		field := sidebar.Flex.GetItem(i).(*tview.TextArea)
		_, _, _, h := field.GetRect()

		hSum += h

		if hSum >= ph {
			sidebar.Flex.ResizeItem(field, 0, 0)
		}
	}
}

func (sidebar *Sidebar) FocusField(index int) {
	sidebar.SetCurrentFieldIndex(index)
	App.SetFocus(sidebar.Flex.GetItem(index))
}

func (sidebar *Sidebar) Clear() {
	sidebar.FieldParameters = make([]*SidebarFieldParameters, 0)
	sidebar.Flex.Clear()
}

func (sidebar *Sidebar) EditTextCurrentField() {
	index := sidebar.GetCurrentFieldIndex()
	item := sidebar.Flex.GetItem(index).(*tview.TextArea)

	sidebar.SetEditingStyles(item)
}

func (sidebar *Sidebar) inputCapture(event *tcell.EventKey) *tcell.EventKey {
	command := app.Keymaps.Group(app.SidebarGroup).Resolve(event)

	switch command {
	case commands.UnfocusSidebar:
		sidebar.Publish(models.StateChange{Key: eventSidebarUnfocusing, Value: nil})
	case commands.ToggleSidebar:
		sidebar.Publish(models.StateChange{Key: eventSidebarToggling, Value: nil})
	case commands.MoveDown:
		sidebar.FocusNextField()
	case commands.MoveUp:
		sidebar.FocusPreviousField()
	case commands.GotoStart:
		sidebar.FocusFirstField()
	case commands.GotoEnd:
		sidebar.FocusLastField()
	case commands.Edit:
		sidebar.Publish(models.StateChange{Key: eventSidebarEditing, Value: true})

		currentItemIndex := sidebar.GetCurrentFieldIndex()
		item := sidebar.Flex.GetItem(currentItemIndex).(*tview.TextArea)
		text := item.GetText()

		columnName := item.GetTitle()
		columnNameSplit := strings.Split(columnName, "[")
		columnName = columnNameSplit[0]

		sidebar.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			command := app.Keymaps.Group(app.SidebarGroup).Resolve(event)

			switch command {
			case commands.CommitEdit:
				sidebar.SetInputCapture(sidebar.inputCapture)
				originalValue := sidebar.FieldParameters[currentItemIndex].OriginalValue
				newText := item.GetText()

				if originalValue == newText {
					sidebar.SetDisabledStyles(item)
				} else {
					sidebar.SetEditedStyles(item)
					sidebar.Publish(models.StateChange{Key: eventSidebarCommitEditing, Value: models.SidebarEditingCommitParams{ColumnName: columnName, Type: models.String, NewValue: newText}})
				}

				return nil
			case commands.DiscardEdit:
				sidebar.SetInputCapture(sidebar.inputCapture)
				sidebar.SetDisabledStyles(item)
				item.SetText(text, true)
				sidebar.Publish(models.StateChange{Key: eventSidebarEditing, Value: false})
				return nil
			}

			return event
		})

		sidebar.EditTextCurrentField()

		return nil
	case commands.SetValue:
		currentItemIndex := sidebar.GetCurrentFieldIndex()
		item := sidebar.Flex.GetItem(currentItemIndex).(*tview.TextArea)
		x, y, _, _ := item.GetRect()

		columnName := item.GetTitle()
		columnNameSplit := strings.Split(columnName, "[")
		columnName = columnNameSplit[0]

		list := NewSetValueList(sidebar.state.dbProvider)

		sidebar.Publish(models.StateChange{Key: eventSidebarEditing, Value: true})

		list.OnFinish(func(selection models.CellValueType, value string) {
			sidebar.Publish(models.StateChange{Key: eventSidebarEditing, Value: false})
			App.SetFocus(item)

			if selection >= 0 {
				sidebar.SetEditedStyles(item)
				item.SetText(value, true)
				sidebar.Publish(models.StateChange{Key: eventSidebarCommitEditing, Value: models.SidebarEditingCommitParams{ColumnName: columnName, Type: selection, NewValue: value}})
			}
		})

		list.Show(x, y, 30)

		return nil
	}
	return event
}

func (sidebar *Sidebar) SetEditingStyles(item *tview.TextArea) {
	item.SetBackgroundColor(app.Styles.SecondaryTextColor)
	item.SetTextStyle(tcell.StyleDefault.Background(app.Styles.SecondaryTextColor).Foreground(tview.Styles.ContrastSecondaryTextColor))
	item.SetTitleColor(app.Styles.ContrastSecondaryTextColor)
	item.SetBorderColor(app.Styles.SecondaryTextColor)

	item.SetWrap(true)
	item.SetDisabled(false)
}

func (sidebar *Sidebar) SetDisabledStyles(item *tview.TextArea) {
	item.SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	item.SetTextStyle(tcell.StyleDefault.Background(app.Styles.PrimitiveBackgroundColor).Foreground(tview.Styles.SecondaryTextColor))
	item.SetTitleColor(app.Styles.PrimaryTextColor)
	item.SetBorderColor(app.Styles.BorderColor)

	item.SetWrap(true)
	item.SetDisabled(true)
}

func (sidebar *Sidebar) SetEditedStyles(item *tview.TextArea) {
	item.SetBackgroundColor(colorTableChange)
	item.SetTextStyle(tcell.StyleDefault.Background(colorTableChange).Foreground(tview.Styles.ContrastSecondaryTextColor))
	item.SetTitleColor(app.Styles.ContrastSecondaryTextColor)
	item.SetBorderColor(app.Styles.ContrastSecondaryTextColor)

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
