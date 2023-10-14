package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type SQLEditorState struct {
	isFocused bool
}

type SQLEditor struct {
	*tview.TextArea
	state       *SQLEditorState
	subscribers []chan StateChange
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
		if event.Rune() == 18 { // Ctrl + R
			sqlEditor.Publish(sqlEditor.GetText())
			return nil
		}

		return event

	})

	return sqlEditor
}

func (s *SQLEditor) Subscribe() chan StateChange {
	subscriber := make(chan StateChange)
	s.subscribers = append(s.subscribers, subscriber)
	return subscriber
}

func (s *SQLEditor) Publish(message string) {
	for _, sub := range s.subscribers {
		sub <- StateChange{
			Key:   "Query",
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
