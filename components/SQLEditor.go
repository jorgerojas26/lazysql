package components

import (
	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/models"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type SQLEditorState struct {
	isFocused bool
}

type SQLEditor struct {
	*tview.TextArea
	state       *SQLEditorState
	subscribers []chan models.StateChange
}

func NewSQLEditor() *SQLEditor {
	textarea := tview.NewTextArea()

	textarea.SetBorder(true)
	textarea.SetTitleAlign(tview.AlignLeft)
	textarea.SetPlaceholder("Enter your SQL query here...")

	sqlEditor := &SQLEditor{
		TextArea: textarea,
		state: &SQLEditorState{
			isFocused: false,
		},
	}

	sqlEditor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := app.Keymaps.Group("editor").Resolve(event)

		if command == commands.Execute {
			sqlEditor.Publish("Query", sqlEditor.GetText())
			return nil
		} else if command == commands.Quit {
			sqlEditor.Publish("Escape", "")
		}

		return event
	})

	return sqlEditor
}

func (s *SQLEditor) Subscribe() chan models.StateChange {
	subscriber := make(chan models.StateChange)
	s.subscribers = append(s.subscribers, subscriber)
	return subscriber
}

func (s *SQLEditor) Publish(key string, message string) {
	for _, sub := range s.subscribers {
		sub <- models.StateChange{
			Key:   key,
			Value: message,
		}
	}
}

func (s *SQLEditor) GetIsFocused() bool {
	return s.state.isFocused
}

func (s *SQLEditor) SetIsFocused(isFocused bool) {
	s.state.isFocused = isFocused
}

func (s *SQLEditor) Highlight() {
	s.SetBorderColor(tview.Styles.PrimaryTextColor)
	s.SetTextStyle(tcell.StyleDefault.Foreground(tview.Styles.PrimaryTextColor))
}

func (s *SQLEditor) SetBlur() {
	s.SetBorderColor(tcell.ColorWhite)
	s.SetTextStyle(tcell.StyleDefault.Foreground(tcell.ColorWhite))
}
