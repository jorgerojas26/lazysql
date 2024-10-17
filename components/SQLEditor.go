package components

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
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

		if command == commands.Execute {
			sqlEditor.Publish(eventSQLEditorQuery, sqlEditor.GetText())
			return nil
		} else if command == commands.UnfocusEditor {
			sqlEditor.Publish(eventSQLEditorEscape, "")
		} else if command == commands.OpenInExternalEditor && runtime.GOOS == "linux" {
			// ----- THIS IS A LINUX-ONLY FEATURE, for now

			text := openExternalEditor(sqlEditor)

			// Set the text from file
			sqlEditor.SetText(text, true)
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

/*
	THIS FUNCTION OPENS EXTERNAL EDITOR.

	Q: WHY OPEN ANOTHER TERMINAL?
	A: OPENING EDITORS LIKE VIM/NEOVIM REALLY MESSED UP INITIAL TERMINAL'S OUTPUT.
*/

func openExternalEditor(s *SQLEditor) string {
	// Current folder as path of temporary file
	path := "./lazysql.sql"

	editor := getEditor()
	terminal := getTerminal()

	// Create a temporary file with the current SQL query content
	content := []byte(s.GetText())

	/*
		0644 Permission
		* User: read & write
		* Group: read
		* Other: read
	*/

	err := os.WriteFile(path, content, 0644)
	if err != nil {
		return s.GetText()
	}

	// Remove the temporary file with the end of function
	defer os.Remove(path)

	// Setup command
	cmd := exec.Command(terminal, "-e", editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	// ----
	// cmd.Stderr = os.Stderr
	// ----

	err = cmd.Run()
	if err != nil {
		return s.GetText()
	}

	// Read the updated content from the temporary file
	updatedContent, err := os.ReadFile(path)
	if err != nil {
		return s.GetText()
	}

	// Convert to string before returning
	return string(updatedContent)
}

func openExternalEditor4Field(s *SQLEditor) string {
	// Current folder as path of temporary file
	path := "./lazysql.data"

	editor := getEditor()
	terminal := getTerminal()

	// Create a temporary file with the current SQL query content
	content := []byte(s.GetText())

	/*
		0644 Permission
		* User: read & write
		* Group: read
		* Other: read
	*/

	err := os.WriteFile(path, content, 0644)
	if err != nil {
		return s.GetText()
	}

	// Remove the temporary file with the end of function
	defer os.Remove(path)

	// Setup command
	cmd := exec.Command(terminal, "-e", editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	// ----
	// cmd.Stderr = os.Stderr
	// ----

	err = cmd.Run()
	if err != nil {
		return s.GetText()
	}

	// Read the updated content from the temporary file
	updatedContent, err := os.ReadFile(path)
	if err != nil {
		return s.GetText()
	}

	// Convert to string before returning
	return string(updatedContent)
}

// Function to select editor
func getEditor() string {
	editor := os.Getenv("SQL_EDITOR")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}

	if editor == "" {
		editor = os.Getenv("VISUAL")
	}

	if editor == "" {
		editor = "vi" // use "vi" if $EDITOR not set
	}

	return editor
}

// Function to select terminal
func getTerminal() string {
	terminal := os.Getenv("SQL_TERMINAL")

	if terminal == "" {
		terminal = os.Getenv("TERMINAL")
	}

	if terminal == "" {
		terminal = "xterm"
	}

	// Check if x-terminal-emulator exists
	terminalEmulator, err := exec.LookPath("x-terminal-emulator")

	// If exists then set terminal as x-terminal-emulator
	if err == nil {
		terminal = terminalEmulator // overload `terminal` if terminalEmulator exists
	}

	return terminal
}
