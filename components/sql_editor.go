package components

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
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
		command := app.Keymaps.Group(app.EditorGroup).Resolve(event)

		switch command {
		case commands.Execute:
			sqlEditor.Publish(eventSQLEditorQuery, sqlEditor.GetText())
			return nil

		case commands.UnfocusEditor:
			sqlEditor.Publish(eventSQLEditorEscape, "")

		case commands.OpenInExternalEditor:
			// THIS IS A LINUX-ONLY FEATURE (for now)
			if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
				var newText string
				app.App.Suspend(func() {
					newText = openExternalEditor(sqlEditor.GetText())
				})
				sqlEditor.SetText(newText, true)
			}
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
	s.SetBorderColor(app.Styles.PrimaryTextColor)
	s.SetTextStyle(tcell.StyleDefault.Foreground(app.Styles.PrimaryTextColor))
}

func (s *SQLEditor) SetBlur() {
	s.SetBorderColor(app.Styles.InverseTextColor)
	s.SetTextStyle(tcell.StyleDefault.Foreground(app.Styles.InverseTextColor))
}

// openExternalEditor opens the user's preferred editor to edit the query.
// It should be called within app.Suspend() to ensure the TUI is properly restored.
func openExternalEditor(currentText string) string {
	tmpFile, err := os.CreateTemp("", "lazysql-*.sql")
	if err != nil {
		logger.Error("Failed to create temporary file", map[string]any{"error": err.Error()})
		return currentText
	}
	defer os.Remove(tmpFile.Name())

	path := tmpFile.Name()
	content := []byte(currentText)

	if _, err := tmpFile.Write(content); err != nil {
		logger.Error("Failed to write to temporary file", map[string]any{"error": err.Error()})
		tmpFile.Close()
		return currentText
	}

	if err := tmpFile.Close(); err != nil {
		logger.Error("Failed to close temporary file", map[string]any{"error": err.Error()})
		return currentText
	}

	editor := getEditor()

	cmd := exec.Command(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Error executing command", map[string]any{"error": err.Error(), "command": cmd.String()})
	}

	updatedContent, err := os.ReadFile(path)
	if err != nil {
		logger.Error("Failed to read from temporary file", map[string]any{"error": err.Error()})
		return currentText
	}

	return string(updatedContent)
}

func getEditor() string {
	editor := os.Getenv("SQL_EDITOR")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}

	if editor == "" {
		editor = os.Getenv("VISUAL")
	}

	if editor == "" {
		editor = "vi"
	}

	return editor
}
