package components

import (
	"os"
	"os/exec"
	"runtime"

	"github.com/gdamore/tcell/v2"
	"github.com/jorgerojas26/lazysql/models"
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
		if event.Rune() == 18 { // Ctrl + R
			sqlEditor.Publish("Query", sqlEditor.TextArea.GetText())
			return nil
		} else if event.Key() == tcell.KeyEscape {
			sqlEditor.Publish("Escape", "")
		} else if event.Key() == tcell.KeyCtrlSpace && runtime.GOOS == "linux" { 
			// ----- THIS IS A LINUX-ONLY FEATURE, for now

			text := openExternalEditor(sqlEditor)

			// Set the text from file
			sqlEditor.TextArea.SetText(text, true)
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


/*
	THIS FUNCTION OPENS EXTERNAL EDITOR.

	Q: WHY OPEN ANOTHER TERMINAL?
	A: OPENING EDITORS LIKE VIM/NEOVIM REALLY MESSED UP INITIAL TERMINAL'S OUTPUT.
*/

func openExternalEditor(s *SQLEditor) string {
	// Path of temporary file
	path := "/tmp/lazysql.sql"

	editor := getEditor()
	terminal := getTerminal()

	// Create a temporary file with the current SQL query content
	content := []byte(s.TextArea.GetText())

	/*
	0644 Permission
	* User: read & write
	* Group: read
	* Other: read
	*/

	err := os.WriteFile(path, content, 0644)
	if err != nil {
		return s.TextArea.GetText()
	}

	// Setup command
	cmd := exec.Command(terminal, "-e", editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	// cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return s.TextArea.GetText()
	}

	// Read the updated content from the temporary file
	updatedContent, err := os.ReadFile(path)
	if err != nil {
		return s.TextArea.GetText()
	}

	// Remove the temporary file
	err = os.Remove(path)

	if err != nil {
		// TODO: Handle error
	}

	// Convert to string before returning
	return string(updatedContent)
}

// Function to select editor
func getEditor() string {
	var editor string = os.Getenv("SQL_EDITOR")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}

	if editor == "" {
		editor = os.Getenv("VISUAL")
	}

	if editor == "" {
		editor = "vi" 		// use "vi" if $EDITOR not set
	}

	return editor
}

// Function to select terminal
func getTerminal() string {
	var terminal string = os.Getenv("SQL_TERMINAL")

	if terminal == "" {
		terminal = os.Getenv("TERMINAL")
	}

	if terminal == "" {
		terminal = "xterm"		// use "xterm" if $TERMINAL not set
	}

	return terminal
}
