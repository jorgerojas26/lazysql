package components

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
)

// ---------------------------------------------------------------------------
// Vim mode
// ---------------------------------------------------------------------------

// VimMode represents the current editor mode.
type VimMode int

const (
	VimModeInsert VimMode = iota
	VimModeNormal
	VimModeVisual
	VimModeVisualLine
)

func (m VimMode) String() string {
	switch m {
	case VimModeInsert:
		return "-- INSERT --"
	case VimModeNormal:
		return "-- NORMAL --"
	case VimModeVisual:
		return "-- VISUAL --"
	case VimModeVisualLine:
		return "-- VISUAL LINE --"
	default:
		return ""
	}
}

// ---------------------------------------------------------------------------
// Undo / Redo
// ---------------------------------------------------------------------------

type undoEntry struct {
	lines  []string
	cx, cy int
}

// ---------------------------------------------------------------------------
// SQLEditor
// ---------------------------------------------------------------------------

// SQLEditorState holds focus state.
type SQLEditorState struct {
	isFocused bool
}

// SQLEditor is a custom multi-line SQL editor with syntax highlighting,
// vim keybindings, and autocomplete.
type SQLEditor struct {
	*tview.Box

	// --- text buffer ---
	lines    []string
	cx, cy   int // cursor: byte column, line index
	ox, oy   int // scroll: byte offset, line offset
	tabWidth int

	// --- selection (visual mode) ---
	selecting    bool
	selCX, selCY int

	// --- vim mode ---
	vimMode VimMode

	// --- double-key leader tracking ---
	leaderKey   rune
	leaderAt    time.Time
	leaderDelay time.Duration

	// --- yank / paste ---
	yankText string

	// --- undo ---
	undoStack []undoEntry
	redoStack []undoEntry
	maxUndo   int

	// --- autocomplete ---
	completer   *Autocompleter
	acItems     []CompletionItem
	acSelected  int
	acOffset    int // scroll offset — index of first visible item
	acPrefix    string
	acTableHint string
	acVisible   bool

	// --- existing API fields ---
	state         *SQLEditorState
	subscribers   []chan models.StateChange
	ConnectionURL string
}

// NewSQLEditor creates a new SQL editor.
func NewSQLEditor(connectionURL string) *SQLEditor {
	e := &SQLEditor{
		Box:         tview.NewBox(),
		lines:       []string{""},
		cx:          0,
		cy:          0,
		ox:          0,
		oy:          0,
		tabWidth:    4,
		vimMode:     VimModeInsert,
		leaderDelay: 500 * time.Millisecond,
		maxUndo:     100,
		completer:   NewAutocompleter(),
		state: &SQLEditorState{
			isFocused: false,
		},
		subscribers:   nil,
		ConnectionURL: connectionURL,
	}
	return e
}

// ---------------------------------------------------------------------------
// Public API (compatible with old tview.TextArea wrapper)
// ---------------------------------------------------------------------------

// GetText returns the full editor content.
func (e *SQLEditor) GetText() string {
	return strings.Join(e.lines, "\n")
}

// SetText replaces the editor content and optionally resets the cursor.
func (e *SQLEditor) SetText(text string, setCursor bool) {
	e.pushUndo()
	// Strip \r from Windows-style line endings
	text = strings.ReplaceAll(text, "\r", "")
	e.lines = splitLines(text)
	if len(e.lines) == 0 {
		e.lines = []string{""}
	}
	if setCursor {
		e.cy = len(e.lines) - 1
		e.cx = len(e.lines[e.cy])
		e.oy = max(0, e.cy-5)
		e.ox = 0
	} else {
		e.cx = 0
		e.cy = 0
		e.ox = 0
		e.oy = 0
	}
	e.selecting = false
	e.leaderKey = 0
	e.acVisible = false
}

// Subscribe returns a channel for state change events.
func (e *SQLEditor) Subscribe() chan models.StateChange {
	subscriber := make(chan models.StateChange, 5)
	e.subscribers = append(e.subscribers, subscriber)
	return subscriber
}

// Publish sends a state change event to all subscribers.
func (e *SQLEditor) Publish(key string, message string) {
	for _, sub := range e.subscribers {
		select {
		case sub <- models.StateChange{Key: key, Value: message}:
		default:
			// Drop if subscriber is slow
		}
	}
}

// GetIsFocused returns whether the editor is focused.
func (e *SQLEditor) GetIsFocused() bool {
	return e.state.isFocused
}

// SetIsFocused sets the focus state.
func (e *SQLEditor) SetIsFocused(isFocused bool) {
	e.state.isFocused = isFocused
}

// Focus overrides tview.Box.Focus to track focus state.
// This ensures the cursor is drawn when the editor has focus.
func (e *SQLEditor) Focus(delegate func(p tview.Primitive)) {
	e.Box.Focus(delegate)
	e.state.isFocused = true
}

// Blur overrides tview.Box.Blur to track focus state.
func (e *SQLEditor) Blur() {
	e.Box.Blur()
	e.state.isFocused = false
}

// Highlight sets the border/foreground color to active.
func (e *SQLEditor) Highlight() {
	e.SetBorderColor(app.Styles.PrimaryTextColor)
}

// SetBlur sets the border/foreground color to inactive.
func (e *SQLEditor) SetBlur() {
	e.SetBorderColor(app.Styles.InverseTextColor)
}

// SetTables passes table names to the autocompleter.
func (e *SQLEditor) SetTables(tables []string) {
	e.completer.SetTables(tables)
}

// SetColumns passes column names for a table to the autocompleter.
func (e *SQLEditor) SetColumns(table string, columns []string) {
	e.completer.SetColumns(table, columns)
}

// ---------------------------------------------------------------------------
// Input handling
// ---------------------------------------------------------------------------

// InputHandler returns the event handler for this widget.
func (e *SQLEditor) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return func(event *tcell.EventKey, _ func(p tview.Primitive)) {
		// --- 1. Always handle open-in-external-editor (Ctrl+Space) ---
		cmd := app.Keymaps.Group(app.EditorGroup).Resolve(event)
		if cmd == commands.OpenInExternalEditor {
			if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
				var newText string
				app.App.Suspend(func() {
					newText = openExternalEditor(e.GetText(), e.ConnectionURL)
				})
				if newText != e.GetText() {
					// SetText already calls pushUndo internally
					e.SetText(newText, true)
				}
			}
			return
		}

		// --- 2. Autocomplete handling ---
		if e.acVisible {
			switch {
			case event.Key() == tcell.KeyDown || event.Key() == tcell.KeyTab || event.Key() == tcell.KeyCtrlN:
				e.acSelected = min(e.acSelected+1, len(e.acItems)-1)
				e.scrollAutocomplete()
				return
			case event.Key() == tcell.KeyUp || event.Key() == tcell.KeyCtrlP:
				e.acSelected = max(e.acSelected-1, 0)
				e.scrollAutocomplete()
				return
			case event.Key() == tcell.KeyEnter || event.Key() == tcell.KeyTab:
				if e.acSelected >= 0 && e.acSelected < len(e.acItems) {
					e.acceptCompletion()
				}
				e.acVisible = false
				return
			case event.Key() == tcell.KeyEscape:
				e.acVisible = false
				return
			}
		}

		// --- 3. Vim mode dispatch ---
		// In normal mode, vim commands take priority over keymap.
		// In insert/visual mode, keymap (Ctrl+R = Execute) takes priority.
		if e.vimMode == VimModeNormal {
			// Check for keymap Execute (Ctrl+R) — still handled in normal mode
			if cmd == commands.Execute {
				e.Publish(eventSQLEditorQuery, e.GetText())
				return
			}
			e.handleNormalMode(event)
			return
		}

		// Insert & Visual mode: keymap commands first
		if cmd == commands.Execute {
			e.Publish(eventSQLEditorQuery, e.GetText())
			return
		}

		switch e.vimMode {
		case VimModeInsert:
			e.handleInsertMode(event)
		case VimModeVisual, VimModeVisualLine:
			e.handleVisualMode(event)
		}
	}
}

// ---------------------------------------------------------------------------
// Insert mode handling
// ---------------------------------------------------------------------------

func (e *SQLEditor) handleInsertMode(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEscape:
		e.vimMode = VimModeNormal
		e.acVisible = false
		e.leaderKey = 0
		if e.cx > 0 {
			e.cx--
		}
		return

	case tcell.KeyEnter:
		e.pushUndo()
		// Uppercase the keyword before Enter (if any)
		if e.cx > 0 {
			e.autoUppercaseKeyword()
		}
		e.splitLine()
		return

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if e.cx > 0 || e.cy > 0 {
			e.pushUndo()
			e.backspace()
		}
		return

	case tcell.KeyDelete:
		e.pushUndo()
		e.deleteChar()
		return

	case tcell.KeyLeft:
		e.moveLeft()
		return

	case tcell.KeyRight:
		e.moveRight()
		return

	case tcell.KeyUp:
		e.moveUp()
		return

	case tcell.KeyDown:
		e.moveDown()
		return

	case tcell.KeyHome:
		e.cx = 0
		return

	case tcell.KeyEnd:
		e.cx = len(e.lines[e.cy])
		return

	case tcell.KeyCtrlU:
		e.pushUndo()
		e.deleteLineTextBeforeCursor()

	case tcell.KeyCtrlK:
		e.pushUndo()
		e.deleteFromCursorToEnd()

	case tcell.KeyCtrlW:
		e.pushUndo()
		e.deleteLastWord()
	}

	// Printable character
	if event.Key() == tcell.KeyRune {
		ch := event.Rune()
		e.insertRune(ch)
		e.triggerAutocomplete()
		// Auto-uppercase SQL keywords when followed by a word boundary
		if isWordBoundaryRune(ch) && e.cx > 1 {
			e.autoUppercaseKeyword()
		}
	}
}

// ---------------------------------------------------------------------------
// Normal mode handling
// ---------------------------------------------------------------------------

func (e *SQLEditor) handleNormalMode(event *tcell.EventKey) {
	// Leader key timeout check
	if e.leaderKey != 0 && time.Since(e.leaderAt) > e.leaderDelay {
		e.executeLeaderCommand(e.leaderKey)
		e.leaderKey = 0
	}

	if event.Key() == tcell.KeyRune {
		ch := event.Rune()

		// Double-key leader handling
		if e.leaderKey != 0 {
			if ch == e.leaderKey {
				e.leaderKey = 0
				e.executeDoubleKeyCommand(ch)
				return
			}
			// Non-matching: execute the leader, then process this key
			e.executeLeaderCommand(e.leaderKey)
			e.leaderKey = 0
			// Fall through to process ch as a single command
		}

		switch ch {
		case 'j':
			e.moveDown()
		case 'k':
			e.moveUp()
		case 'h':
			e.moveLeft()
		case 'l':
			e.moveRight()
		case 'w':
			e.wordForward()
		case 'b':
			e.wordBackward()
		case '0':
			e.cx = 0
		case '^':
			e.cx = firstNonWhitespace(e.lines[e.cy])
		case '$':
			e.cx = len(e.lines[e.cy])
		case 'i':
			e.vimMode = VimModeInsert
		case 'a':
			e.vimMode = VimModeInsert
			if e.cx < len(e.lines[e.cy]) {
				e.cx++
			}
		case 'I':
			e.cx = 0
			e.vimMode = VimModeInsert
		case 'A':
			e.cx = len(e.lines[e.cy])
			e.vimMode = VimModeInsert
		case 'o':
			e.pushUndo()
			e.cy++
			e.lines = append(e.lines[:e.cy], append([]string{""}, e.lines[e.cy:]...)...)
			e.cx = 0
			e.vimMode = VimModeInsert
		case 'O':
			e.pushUndo()
			e.lines = append(e.lines[:e.cy], append([]string{""}, e.lines[e.cy:]...)...)
			e.cx = 0
			e.vimMode = VimModeInsert
		case 'x':
			e.pushUndo()
			e.deleteChar()
		case 'X':
			e.pushUndo()
			e.backspace()
		case 'D':
			e.pushUndo()
			e.lines[e.cy] = e.lines[e.cy][:e.cx]
		case 'p':
			e.pasteAfter()
		case 'P':
			e.pasteBefore()
		case 'u':
			e.undo()
		case 'd':
			e.leaderKey = 'd'
			e.leaderAt = time.Now()
		case 'y':
			e.leaderKey = 'y'
			e.leaderAt = time.Now()
		case 'g':
			e.leaderKey = 'g'
			e.leaderAt = time.Now()
		case 'G':
			e.cy = len(e.lines) - 1
			e.cx = 0
		case 'v':
			e.vimMode = VimModeVisual
			e.selecting = true
			e.selCX, e.selCY = e.cx, e.cy
		case 'V':
			e.vimMode = VimModeVisualLine
			e.selecting = true
			e.selCX, e.selCY = 0, e.cy
		case '.':
			// Repeat last action (stub - future improvement)
		}

		return
	}

	// Non-rune keys
	switch event.Key() {
	case tcell.KeyEscape:
		if e.vimMode != VimModeNormal {
			e.vimMode = VimModeNormal
		} else {
			// Unfocus the editor (existing behavior)
			e.Publish(eventSQLEditorEscape, "")
		}
	case tcell.KeyEnter:
		e.pushUndo()
		e.splitLine()
	case tcell.KeyUp:
		e.moveUp()
	case tcell.KeyDown:
		e.moveDown()
	case tcell.KeyLeft:
		e.moveLeft()
	case tcell.KeyRight:
		e.moveRight()
	case tcell.KeyHome:
		e.cx = 0
	case tcell.KeyEnd:
		e.cx = len(e.lines[e.cy])
	case tcell.KeyPgUp:
		e.pageUp()
	case tcell.KeyPgDn:
		e.pageDown()
	case tcell.KeyCtrlR:
		e.redo()
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		e.pushUndo()
		e.backspace()
	case tcell.KeyDelete:
		e.pushUndo()
		e.deleteChar()
	}
}

// ---------------------------------------------------------------------------
// Visual mode handling
// ---------------------------------------------------------------------------

func (e *SQLEditor) handleVisualMode(event *tcell.EventKey) {
	if event.Key() == tcell.KeyRune {
		ch := event.Rune()

		switch ch {
		case 'j':
			e.moveDown()
		case 'k':
			e.moveUp()
		case 'h':
			e.moveLeft()
		case 'l':
			e.moveRight()
		case 'w':
			e.wordForward()
		case 'b':
			e.wordBackward()
		case '0':
			e.cx = 0
		case '$':
			e.cx = len(e.lines[e.cy])
		case 'y':
			e.yankSelection()
			e.vimMode = VimModeNormal
			e.selecting = false
		case 'd', 'x':
			e.pushUndo()
			e.deleteSelection()
			e.vimMode = VimModeNormal
			e.selecting = false
		case 'D':
			e.pushUndo()
			e.deleteSelection()
			e.vimMode = VimModeNormal
			e.selecting = false
		case 'V':
			e.vimMode = VimModeVisualLine
			e.selCY = e.cy
		}

		return
	}

	switch event.Key() {
	case tcell.KeyEscape:
		e.vimMode = VimModeNormal
		e.selecting = false
	case tcell.KeyUp:
		e.moveUp()
	case tcell.KeyDown:
		e.moveDown()
	case tcell.KeyLeft:
		e.moveLeft()
	case tcell.KeyRight:
		e.moveRight()
	}
}

// ---------------------------------------------------------------------------
// Buffer operations
// ---------------------------------------------------------------------------

func (e *SQLEditor) insertRune(ch rune) {
	// cx is a byte index; use string concatenation
	cx := min(e.cx, len(e.lines[e.cy]))
	e.lines[e.cy] = e.lines[e.cy][:cx] + string(ch) + e.lines[e.cy][cx:]
	e.cx = cx + len(string(ch))
	e.clampCursor()
}

func (e *SQLEditor) backspace() {
	if e.cx > 0 {
		// cx is a byte index; string slicing is safe for ASCII SQL text
		e.lines[e.cy] = e.lines[e.cy][:e.cx-1] + e.lines[e.cy][e.cx:]
		e.cx--
	} else if e.cy > 0 {
		// Join with previous line
		prevLen := len(e.lines[e.cy-1])
		e.lines[e.cy-1] += e.lines[e.cy]
		e.lines = append(e.lines[:e.cy], e.lines[e.cy+1:]...)
		e.cy--
		e.cx = prevLen
	}
}

func (e *SQLEditor) deleteChar() {
	if e.cx < len(e.lines[e.cy]) {
		e.lines[e.cy] = e.lines[e.cy][:e.cx] + e.lines[e.cy][e.cx+1:]
	} else if e.cy < len(e.lines)-1 {
		// Join with next line
		e.lines[e.cy] += e.lines[e.cy+1]
		e.lines = append(e.lines[:e.cy+1], e.lines[e.cy+2:]...)
	}
}

func (e *SQLEditor) splitLine() {
	line := e.lines[e.cy]
	before := ""
	after := ""
	if e.cx <= len(line) {
		before = line[:e.cx]
		after = line[e.cx:]
	}
	e.lines[e.cy] = before
	e.cy++
	e.lines = append(e.lines[:e.cy], append([]string{after}, e.lines[e.cy:]...)...)
	e.cx = 0
}

func (e *SQLEditor) deleteLineTextBeforeCursor() {
	if e.cx > 0 {
		e.lines[e.cy] = e.lines[e.cy][e.cx:]
		e.cx = 0
	}
}

func (e *SQLEditor) deleteFromCursorToEnd() {
	if e.cx < len(e.lines[e.cy]) {
		e.lines[e.cy] = e.lines[e.cy][:e.cx]
	}
}

func (e *SQLEditor) deleteLastWord() {
	if e.cx == 0 {
		if e.cy > 0 {
			// Join with previous line (same as backspace at line start)
			prevLen := len(e.lines[e.cy-1])
			e.lines[e.cy-1] += e.lines[e.cy]
			e.lines = append(e.lines[:e.cy], e.lines[e.cy+1:]...)
			e.cy--
			e.cx = prevLen
		}
		return
	}

	line := e.lines[e.cy]
	// 1. Skip whitespace right before cursor
	end := e.cx - 1
	for end >= 0 && (line[end] == ' ' || line[end] == '\t') {
		end--
	}
	// 2. Skip the word (non-whitespace)
	for end >= 0 && line[end] != ' ' && line[end] != '\t' {
		end--
	}
	// 3. Delete from end+1 to e.cx (the word + any whitespace before cursor)
	start := end + 1
	e.lines[e.cy] = line[:start] + line[e.cx:]
	e.cx = start
}

// autoUppercaseKeyword converts the word before the cursor to uppercase if
// it is a known SQL keyword. Called after inserting a word-boundary character.
func (e *SQLEditor) autoUppercaseKeyword() {
	line := e.lines[e.cy]
	if len(line) < 2 {
		return
	}
	// Find the start of the word before cursor.
	// e.cx points past the boundary character, so the word is at [start, e.cx-1).
	end := e.cx - 1
	if end <= 0 {
		return
	}
	start := end - 1
	for start >= 0 {
		b := line[start]
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' || isPunctByte(b) {
			start++
			break
		}
		start--
	}
	if start < 0 {
		start = 0
	}
	if start >= end {
		return
	}

	word := line[start:end]
	upper := strings.ToUpper(word)
	if upper == word {
		return // already uppercase
	}
	if !isKeyword(upper) {
		return // not a SQL keyword
	}

	e.lines[e.cy] = line[:start] + upper + line[end:]
	e.cx = start + len(upper) + 1 // +1 for the boundary character
}

// isWordBoundaryRune returns true for characters that typically end a SQL word.
func isWordBoundaryRune(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' ||
		ch == ';' || ch == ',' || ch == '(' || ch == ')' ||
		ch == '\'' || ch == '"'
}

func isPunctByte(b byte) bool {
	switch b {
	case ';', ',', '(', ')', '\'', '"', '=', '<', '>', '!', '+', '-', '*', '/':
		return true
	}
	return false
}

func (e *SQLEditor) pasteAfter() {
	if e.yankText == "" {
		return
	}
	e.pushUndo()
	parts := strings.Split(e.yankText, "\n")
	if len(parts) == 1 {
		// Paste inline
		pos := min(e.cx, len(e.lines[e.cy]))
		e.lines[e.cy] = e.lines[e.cy][:pos] + e.yankText + e.lines[e.cy][pos:]
		e.cx = pos + len(e.yankText)
	} else {
		// Paste as lines below current
		var newLines []string
		newLines = append(newLines, e.lines[:e.cy+1]...)
		newLines = append(newLines, parts...)
		newLines = append(newLines, e.lines[e.cy+1:]...)
		e.lines = newLines
		e.cy += len(parts) - 1
		e.cx = 0
	}
}

func (e *SQLEditor) pasteBefore() {
	if e.yankText == "" {
		return
	}
	e.pushUndo()
	parts := strings.Split(e.yankText, "\n")
	if len(parts) == 1 {
		// Paste inline before cursor
		pos := min(e.cx, len(e.lines[e.cy]))
		e.lines[e.cy] = e.lines[e.cy][:pos] + e.yankText + e.lines[e.cy][pos:]
		e.cx = pos
	} else {
		// Paste as lines above current
		var newLines []string
		newLines = append(newLines, e.lines[:e.cy]...)
		newLines = append(newLines, parts...)
		newLines = append(newLines, e.lines[e.cy:]...)
		e.lines = newLines
		e.cy--
		if e.cy < 0 {
			e.cy = 0
		}
		e.cx = 0
	}
}

// ---------------------------------------------------------------------------
// Selection operations
// ---------------------------------------------------------------------------

func (e *SQLEditor) getSelectionRange() (startLine, startCol, endLine, endCol int) {
	// Determine which point is earlier
	if e.selCY < e.cy || (e.selCY == e.cy && e.selCX <= e.cx) {
		return e.selCY, e.selCX, e.cy, e.cx
	}
	return e.cy, e.cx, e.selCY, e.selCX
}

func (e *SQLEditor) getSelectedText() string {
	sl, sc, el, ec := e.getSelectionRange()
	if sl == el {
		if sc < len(e.lines[sl]) {
			end := min(ec, len(e.lines[sl]))
			return e.lines[sl][sc:end]
		}
		return ""
	}
	var parts []string
	parts = append(parts, e.lines[sl][sc:])
	for i := sl + 1; i < el; i++ {
		parts = append(parts, e.lines[i])
	}
	if el < len(e.lines) && ec <= len(e.lines[el]) {
		parts = append(parts, e.lines[el][:ec])
	}
	return strings.Join(parts, "\n")
}

func (e *SQLEditor) yankSelection() {
	e.yankText = e.getSelectedText()
}

func (e *SQLEditor) deleteSelection() {
	sl, sc, el, ec := e.getSelectionRange()
	e.yankText = e.getSelectedText()

	if sl == el {
		end := min(ec, len(e.lines[sl]))
		e.lines[sl] = e.lines[sl][:sc] + e.lines[sl][end:]
		e.cy = sl
		e.cx = sc
	} else {
		// Build new lines without selection
		var newLines []string
		newLines = append(newLines, e.lines[:sl]...)
		// First line: keep before selection
		firstPart := ""
		if sc < len(e.lines[sl]) {
			firstPart = e.lines[sl][:sc]
		}
		// Last line: keep after selection
		lastPart := ""
		if el < len(e.lines) && ec <= len(e.lines[el]) {
			lastPart = e.lines[el][ec:]
		}
		combinedLine := firstPart + lastPart
		newLines = append(newLines, combinedLine)
		for i := el + 1; i < len(e.lines); i++ {
			newLines = append(newLines, e.lines[i])
		}
		e.lines = newLines
		e.cy = sl
		e.cx = sc
	}
}

// ---------------------------------------------------------------------------
// Cursor movement
// ---------------------------------------------------------------------------

func (e *SQLEditor) moveLeft() {
	if e.cx > 0 {
		e.cx--
	} else if e.cy > 0 {
		e.cy--
		e.cx = len(e.lines[e.cy])
	}
}

func (e *SQLEditor) moveRight() {
	if e.cx < len(e.lines[e.cy]) {
		e.cx++
	} else if e.cy < len(e.lines)-1 {
		e.cy++
		e.cx = 0
	}
}

func (e *SQLEditor) moveUp() {
	if e.cy > 0 {
		e.cy--
		e.cx = min(e.cx, len(e.lines[e.cy]))
	}
}

func (e *SQLEditor) moveDown() {
	if e.cy < len(e.lines)-1 {
		e.cy++
		e.cx = min(e.cx, len(e.lines[e.cy]))
	}
}

func (e *SQLEditor) wordForward() {
	line := e.lines[e.cy]
	if e.cx >= len(line) {
		if e.cy < len(e.lines)-1 {
			e.cy++
			e.cx = 0
		}
		return
	}
	// Skip whitespace
	for e.cx < len(line) && (line[e.cx] == ' ' || line[e.cx] == '\t') {
		e.cx++
	}
	if e.cx >= len(line) {
		if e.cy < len(e.lines)-1 {
			e.cy++
			e.cx = 0
		}
		return
	}
	// Move through word
	for e.cx < len(line) && line[e.cx] != ' ' && line[e.cx] != '\t' {
		e.cx++
	}
}

func (e *SQLEditor) wordBackward() {
	if e.cx <= 0 {
		if e.cy > 0 {
			e.cy--
			e.cx = len(e.lines[e.cy])
		}
		return
	}
	e.cx--
	// Skip whitespace backward
	for e.cx > 0 && (e.lines[e.cy][e.cx] == ' ' || e.lines[e.cy][e.cx] == '\t') {
		e.cx--
	}
	// Move through word backward
	for e.cx > 0 && e.lines[e.cy][e.cx-1] != ' ' && e.lines[e.cy][e.cx-1] != '\t' {
		e.cx--
	}
}

func (e *SQLEditor) pageUp() {
	// GetInnerRect returns content area (excluding border).
	// height-2: 1 line for status bar, 1 line margin.
	_, _, _, height := e.GetInnerRect()
	n := max(height-2, 1)
	e.cy = max(e.cy-n, 0)
	e.cx = min(e.cx, len(e.lines[e.cy]))
}

func (e *SQLEditor) pageDown() {
	_, _, _, height := e.GetInnerRect()
	n := max(height-2, 1)
	// cy is 0-indexed, len(lines)-1 is the last valid line index.
	e.cy = min(e.cy+n, len(e.lines)-1)
	e.cx = min(e.cx, len(e.lines[e.cy]))
}

// ---------------------------------------------------------------------------
// Double-key commands
// ---------------------------------------------------------------------------

func (e *SQLEditor) executeLeaderCommand(ch rune) {
	switch ch {
	case 'g':
		e.cy = 0
		e.cx = 0
	case 'd':
		e.pushUndo()
		e.lines[e.cy] = ""
		e.cx = 0
	case 'y':
		e.yankText = e.lines[e.cy]
	}
}

func (e *SQLEditor) executeDoubleKeyCommand(ch rune) {
	switch ch {
	case 'g':
		e.cy = 0
		e.cx = 0
	case 'd':
		e.pushUndo()
		if len(e.lines) == 1 {
			e.lines[0] = ""
			e.cx = 0
		} else {
			e.lines = append(e.lines[:e.cy], e.lines[e.cy+1:]...)
			if e.cy >= len(e.lines) {
				e.cy = len(e.lines) - 1
			}
			e.cx = 0
		}
	case 'y':
		e.yankText = e.lines[e.cy]
	}
}

// ---------------------------------------------------------------------------
// Undo / Redo
// ---------------------------------------------------------------------------

func (e *SQLEditor) pushUndo() {
	state := undoEntry{
		lines: copyLines(e.lines),
		cx:    e.cx,
		cy:    e.cy,
	}
	e.undoStack = append(e.undoStack, state)
	if len(e.undoStack) > e.maxUndo {
		e.undoStack = e.undoStack[1:]
	}
	e.redoStack = nil
}

func (e *SQLEditor) undo() {
	if len(e.undoStack) == 0 {
		return
	}
	// Save current state to redo
	e.redoStack = append(e.redoStack, undoEntry{
		lines: copyLines(e.lines),
		cx:    e.cx,
		cy:    e.cy,
	})
	// Restore
	prev := e.undoStack[len(e.undoStack)-1]
	e.undoStack = e.undoStack[:len(e.undoStack)-1]
	e.lines = prev.lines
	e.cx = prev.cx
	e.cy = prev.cy
	e.clampCursor()
	e.scrollToCursor()
}

func (e *SQLEditor) redo() {
	if len(e.redoStack) == 0 {
		return
	}
	// Save current state to undo
	e.undoStack = append(e.undoStack, undoEntry{
		lines: copyLines(e.lines),
		cx:    e.cx,
		cy:    e.cy,
	})
	// Restore
	next := e.redoStack[len(e.redoStack)-1]
	e.redoStack = e.redoStack[:len(e.redoStack)-1]
	e.lines = next.lines
	e.cx = next.cx
	e.cy = next.cy
	e.clampCursor()
	e.scrollToCursor()
}

// ---------------------------------------------------------------------------
// Autocomplete
// ---------------------------------------------------------------------------

func (e *SQLEditor) triggerAutocomplete() {
	text := e.GetText()
	cursorPos := e.cursorByteOffset()

	// 1. Extract context: handles "table.col", "alias.col", and plain words
	prefix, tableName := extractCompletionContext(text, cursorPos)

	// 2. Resolve table name: check aliases, then fall back to FROM/JOIN hints
	if tableName != "" {
		aliases := resolveAliases(text, cursorPos)
		if resolved, ok := aliases[strings.ToLower(tableName)]; ok {
			tableName = resolved
		}
		// Keep the original tableName if alias resolution didn't find it
		// (it might be a real table name, not an alias)
	} else {
		tableName = extractTableHint(text, cursorPos)
	}

	e.acPrefix = prefix
	e.acTableHint = tableName

	// Show completions when:
	// - A table is specified (even with empty prefix — the "table." case)
	// - The user has typed at least 1 character of a word
	if tableName != "" || len(prefix) >= 1 {
		items := e.completer.GetCompletions(prefix, tableName)
		if len(items) > 0 {
			e.acItems = items
			e.acSelected = 0
			e.acOffset = 0
			e.acVisible = true
			return
		}
	}

	e.acVisible = false
}

// scrollAutocomplete adjusts acOffset so the selected item stays visible.
func (e *SQLEditor) scrollAutocomplete() {
	// Use a generous estimate (10) for the scroll window. The exact
	// visible count is computed in drawAutocomplete, but we need a
	// reasonable window here so the user can see scrolling happening.
	// Any minor mismatch at boundaries is corrected in drawAutocomplete.
	w := 10
	if w > len(e.acItems) {
		w = len(e.acItems)
	}
	if e.acSelected < e.acOffset {
		e.acOffset = e.acSelected
	}
	if e.acSelected >= e.acOffset+w {
		e.acOffset = e.acSelected - w + 1
	}
}

// cursorByteOffset returns the byte offset of the cursor in the full text.
func (e *SQLEditor) cursorByteOffset() int {
	offset := 0
	for i := 0; i < e.cy && i < len(e.lines); i++ {
		offset += len(e.lines[i]) + 1 // +1 for newline
	}
	offset += e.cx
	return offset
}

func (e *SQLEditor) acceptCompletion() {
	if e.acSelected < 0 || e.acSelected >= len(e.acItems) {
		e.acVisible = false
		return
	}
	e.pushUndo()
	completion := e.acItems[e.acSelected].Text

	// Find start of current word
	text := e.GetText()
	cursorPos := e.cursorByteOffset()
	prefix := extractPrefix(text, cursorPos)
	if prefix == "" {
		e.acVisible = false
		return
	}

	// Replace the prefix with the completion
	prefixLen := len(prefix)
	suffix := ""
	if e.cx >= prefixLen {
		suffix = e.lines[e.cy][e.cx:]
		e.lines[e.cy] = e.lines[e.cy][:e.cx-prefixLen]
	} else {
		e.lines[e.cy] = e.lines[e.cy][e.cx:]
	}

	// Insert the completion text
	e.lines[e.cy] += completion + suffix
	e.cx = len(e.lines[e.cy]) - len(suffix)
	e.acVisible = false
}

// ---------------------------------------------------------------------------
// Drawing
// ---------------------------------------------------------------------------

// Draw renders the editor on the screen.
func (e *SQLEditor) Draw(screen tcell.Screen) {
	e.Box.DrawForSubclass(screen, e)
	x, y, width, height := e.Box.GetInnerRect()

	if width <= 0 || height <= 0 {
		return
	}

	// Reserve bottom line for status bar
	statusY := y + height - 1
	textHeight := height - 1
	if textHeight < 1 {
		textHeight = 1
	}

	// Ensure cursor is visible
	e.scrollToCursor()

	// Default colors
	defaultFg := app.Styles.PrimaryTextColor
	defaultBg := app.Styles.PrimitiveBackgroundColor

	// Draw text lines
	for lineIdx := 0; lineIdx < textHeight; lineIdx++ {
		bufIdx := e.oy + lineIdx
		if bufIdx >= len(e.lines) {
			// Draw empty line
			e.drawLineText(screen, x, y+lineIdx, width, "", 0, defaultFg, defaultBg)
			continue
		}

		// Get syntax highlighting for this line
		lineText := e.lines[bufIdx]

		// Check if this line is within visual selection
		inSelection := false
		selStart, selEnd := 0, 0
		if e.selecting && e.cy != e.selCY {
			sl, sc, el, ec := e.getSelectionRange()
			if bufIdx > sl && bufIdx < el {
				inSelection = true
				selStart = 0
				selEnd = len(lineText)
			} else if bufIdx == sl {
				inSelection = true
				selStart = sc
				selEnd = len(lineText)
			} else if bufIdx == el {
				inSelection = true
				selStart = 0
				selEnd = ec
			}
		} else if e.selecting && e.cy == e.selCY {
			sl, sc, el, ec := e.getSelectionRange()
			if bufIdx == sl && bufIdx == el {
				inSelection = true
				selStart = sc
				selEnd = ec
			}
		}

		// Visual line mode: entire line is selected
		if e.vimMode == VimModeVisualLine && e.selecting {
			sl, _, el, _ := e.getSelectionRange()
			if bufIdx >= sl && bufIdx <= el {
				inSelection = true
				selStart = 0
				selEnd = len(lineText)
			}
		}

		e.drawLineText(screen, x, y+lineIdx, width, lineText, e.ox, defaultFg, defaultBg)

		// Draw selection highlight
		if inSelection {
			e.drawSelection(screen, x, y+lineIdx, width, lineText, selStart, selEnd, e.ox)
		}
	}

	// Draw cursor
	if e.state.isFocused {
		cursorScreenX := x - e.ox + visibleLen(e.lines[e.cy][:min(e.cx, len(e.lines[e.cy]))], e.tabWidth)
		cursorScreenY := y + e.cy - e.oy
		if cursorScreenY >= y && cursorScreenY < statusY && cursorScreenX >= x && cursorScreenX < x+width {
			screen.ShowCursor(cursorScreenX, cursorScreenY)
		}
	}

	// Draw status bar
	e.drawStatusBar(screen, x, statusY, width, defaultFg, defaultBg)

	// Draw autocomplete popup
	if e.acVisible && len(e.acItems) > 0 {
		e.drawAutocomplete(screen, x, y, width, textHeight, defaultFg, defaultBg)
	}
}

func (e *SQLEditor) drawLineText(screen tcell.Screen, x, y, width int, lineText string, ox int, defaultFg, defaultBg tcell.Color) {
	if lineText == "" {
		// Draw spaces for empty line
		for i := 0; i < width; i++ {
			style := tcell.StyleDefault.Foreground(defaultFg).Background(defaultBg)
			screen.SetContent(x+i, y, ' ', nil, style)
		}
		return
	}

	line := []rune(lineText)
	// Pre-compute syntax highlighting styles
	lineStyles := tokenizeSQL(lineText, defaultFg, defaultBg)

	// Clip to scroll offset
	startRune := 0
	visOffset := 0
	for visOffset < ox && startRune < len(line) {
		w := runeWidth(line[startRune])
		if visOffset+w > ox {
			break
		}
		visOffset += w
		startRune++
	}

	col := 0
	for i := startRune; i < len(line) && col < width; i++ {
		ch := line[i]

		if ch == '\t' {
			tabStop := e.tabWidth - ((col) % e.tabWidth)
			for t := 0; t < tabStop && col < width; t++ {
				style := tcell.StyleDefault.Foreground(defaultFg).Background(defaultBg)
				if i < len(lineStyles) {
					style = lineStyles[i]
				}
				screen.SetContent(x+col, y, ' ', nil, style)
				col++
			}
		} else {
			style := tcell.StyleDefault.Foreground(defaultFg).Background(defaultBg)
			if i < len(lineStyles) {
				style = lineStyles[i]
			}
			screen.SetContent(x+col, y, ch, nil, style)
			col++
		}
	}

	// Fill remaining line with spaces
	for col < width {
		style := tcell.StyleDefault.Foreground(defaultFg).Background(defaultBg)
		screen.SetContent(x+col, y, ' ', nil, style)
		col++
	}
}

func (e *SQLEditor) drawSelection(screen tcell.Screen, x, y, width int, lineText string, selStart, selEnd int, ox int) {
	// Handle empty lines: select from start to width
	if lineText == "" {
		for col := 0; col < width; col++ {
			mainc, combc, style, _ := screen.GetContent(x+col, y)
			screen.SetContent(x+col, y, mainc, combc, style.Background(tcell.ColorDarkCyan))
		}
		return
	}

	// Calculate visible column positions
	line := []rune(lineText)

	selVisStart := 0
	selVisEnd := 0
	visCol := 0

	for i := 0; i < len(line) && (i < selEnd || selEnd < 0); i++ {
		if i == selStart {
			selVisStart = visCol
		}
		if i == selEnd {
			selVisEnd = visCol
			break
		}
		visCol += runeWidth(line[i])
	}
	if selEnd >= len(line) {
		selVisEnd = visCol
	}

	startCol := selVisStart - ox
	endCol := selVisEnd - ox

	if startCol < 0 {
		startCol = 0
	}
	if endCol > width {
		endCol = width
	}

	for col := startCol; col < endCol; col++ {
		mainc, combc, style, _ := screen.GetContent(x+col, y)
		screen.SetContent(x+col, y, mainc, combc, style.Background(tcell.ColorDarkCyan))
	}
}

func (e *SQLEditor) drawStatusBar(screen tcell.Screen, x, y, width int, _, defaultBg tcell.Color) {
	modeText := e.vimMode.String()
	posText := "Ln " + itoa(e.cy+1) + ", Col " + itoa(cursorDisplayCol(e.lines, e.cy, e.cx, e.tabWidth)+1)

	statusBg := tcell.ColorDarkSlateGray
	statusFg := tcell.ColorWhite

	// Clear status line
	for i := 0; i < width; i++ {
		style := tcell.StyleDefault.Foreground(statusFg).Background(statusBg)
		screen.SetContent(x+i, y, ' ', nil, style)
	}

	// Draw mode text
	for i, ch := range modeText {
		if i >= width {
			break
		}
		style := tcell.StyleDefault.Foreground(statusFg).Background(statusBg)
		screen.SetContent(x+i, y, ch, nil, style)
	}

	// Draw position text (right-aligned)
	posStart := width - len(posText)
	if posStart < 0 {
		posStart = 0
	}
	for i, ch := range posText {
		col := posStart + i
		if col >= width {
			break
		}
		style := tcell.StyleDefault.Foreground(statusFg).Background(statusBg)
		screen.SetContent(x+col, y, ch, nil, style)
	}
}

func (e *SQLEditor) drawAutocomplete(screen tcell.Screen, x, y, width, height int, _, defaultBg tcell.Color) {
	if len(e.acItems) == 0 {
		return
	}

	// Calculate popup position: near the cursor, but avoid overflow
	cursorScreenX := x - e.ox + visibleLen(e.lines[e.cy][:min(e.cx, len(e.lines[e.cy]))], e.tabWidth)
	popupX := cursorScreenX
	if popupX+30 > x+width {
		popupX = x + width - 30
		if popupX < x {
			popupX = x
		}
	}

	// Determine how many popup items fit within the editor's vertical space
	cursorLine := y + e.cy - e.oy
	spaceBelow := y + height - cursorLine - 1 // rows from cursor+1 to editor bottom
	spaceAbove := cursorLine - y              // rows from editor top to cursor-1

	wanted := 10
	if wanted > len(e.acItems) {
		wanted = len(e.acItems)
	}
	need := wanted + 2 // items + top/bottom borders

	var maxItems int
	var popupY int

	if need <= spaceBelow {
		// All requested items fit below the cursor
		maxItems = wanted
		popupY = cursorLine + 1
	} else if need <= spaceAbove {
		// All requested items fit above the cursor
		maxItems = wanted
		popupY = cursorLine - need
	} else {
		// Neither side fits all items — use the side with more room
		if spaceBelow >= spaceAbove {
			maxItems = spaceBelow - 2
			if maxItems < 1 {
				maxItems = 1
			}
			popupY = cursorLine + 1
		} else {
			maxItems = spaceAbove - 2
			if maxItems < 1 {
				maxItems = 1
			}
			popupY = cursorLine - (maxItems + 2)
			if popupY < y {
				popupY = y
			}
		}
		if maxItems > len(e.acItems) {
			maxItems = len(e.acItems)
		}
	}

	// Keep the scroll window in sync with the actual visible item count
	if e.acOffset+maxItems > len(e.acItems) {
		e.acOffset = len(e.acItems) - maxItems
	}
	if e.acOffset < 0 {
		e.acOffset = 0
	}
	if e.acSelected < e.acOffset {
		e.acOffset = e.acSelected
	}
	if e.acSelected >= e.acOffset+maxItems {
		e.acOffset = e.acSelected - maxItems + 1
	}

	maxWidth := 0
	for i := e.acOffset; i < e.acOffset+maxItems; i++ {
		item := e.acItems[i]
		w := visibleLen(item.Text, e.tabWidth)
		if item.Description != "" {
			w += 3 + visibleLen(item.Description, e.tabWidth) // " - desc"
		}
		if w > maxWidth {
			maxWidth = w
		}
	}
	maxWidth = min(maxWidth+2, width-2)
	if maxWidth < 10 {
		maxWidth = 10
	}
	popupWidth := maxWidth + 2  // border
	popupHeight := maxItems + 2 // border

	// Ensure popup fits horizontally
	if popupX+popupWidth > x+width {
		popupX = x + width - popupWidth
	}

	// Draw popup border and background
	borderStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorDarkSlateGray)
	contentStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(tcell.ColorDarkSlateGray)
	selectedBg := tcell.ColorDodgerBlue

	for py := 0; py < popupHeight; py++ {
		for px := 0; px < popupWidth; px++ {
			sx := popupX + px
			sy := popupY + py
			if sx < x || sx >= x+width || sy < y || sy >= y+height {
				continue
			}
			// Border
			if py == 0 || py == popupHeight-1 || px == 0 || px == popupWidth-1 {
				ch := ' '
				if py == 0 && px == 0 {
					ch = tcell.RuneULCorner
				} else if py == 0 && px == popupWidth-1 {
					ch = tcell.RuneURCorner
				} else if py == popupHeight-1 && px == 0 {
					ch = tcell.RuneLLCorner
				} else if py == popupHeight-1 && px == popupWidth-1 {
					ch = tcell.RuneLRCorner
				} else if py == 0 || py == popupHeight-1 {
					ch = tcell.RuneHLine
				} else if px == 0 || px == popupWidth-1 {
					ch = tcell.RuneVLine
				}
				screen.SetContent(sx, sy, ch, nil, borderStyle)
				continue
			}

			itemIdx := py - 1
			if itemIdx >= maxItems {
				screen.SetContent(sx, sy, ' ', nil, contentStyle)
				continue
			}

			actualIdx := e.acOffset + itemIdx
			item := e.acItems[actualIdx]
			itemStyle := contentStyle
			if actualIdx == e.acSelected {
				itemStyle = contentStyle.Background(selectedBg)
			}

			// Draw item text
			textX := px - 1
			if textX < len(item.Text) {
				ch := rune(item.Text[textX])
				screen.SetContent(sx, sy, ch, nil, itemStyle)
			} else if item.Description != "" {
				descStart := len(item.Text) + 1
				if textX == descStart {
					screen.SetContent(sx, sy, ' ', nil, itemStyle)
				} else if textX == descStart+1 {
					screen.SetContent(sx, sy, '-', nil, itemStyle.Foreground(tcell.ColorGray))
				} else if textX > descStart+2 {
					descIdx := textX - descStart - 3
					if descIdx < len(item.Description) {
						ch := rune(item.Description[descIdx])
						screen.SetContent(sx, sy, ch, nil, itemStyle.Foreground(tcell.ColorLightGray))
					} else {
						screen.SetContent(sx, sy, ' ', nil, itemStyle)
					}
				} else {
					screen.SetContent(sx, sy, ' ', nil, itemStyle)
				}
			} else {
				screen.SetContent(sx, sy, ' ', nil, itemStyle)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Scroll management
// ---------------------------------------------------------------------------

func (e *SQLEditor) scrollToCursor() {
	_, _, width, height := e.Box.GetInnerRect()

	// Vertical scroll
	if e.cy < e.oy {
		e.oy = e.cy
	}
	if e.cy >= e.oy+height-2 { // -2 for status bar and border
		e.oy = e.cy - height + 3
	}
	if e.oy < 0 {
		e.oy = 0
	}

	// Horizontal scroll
	cursorDisplayCol := visibleLen(e.lines[e.cy][:min(e.cx, len(e.lines[e.cy]))], e.tabWidth)
	if cursorDisplayCol < e.ox {
		e.ox = max(0, cursorDisplayCol-2)
	}
	if cursorDisplayCol >= e.ox+width-1 {
		e.ox = cursorDisplayCol - width + 2
	}
	if e.ox < 0 {
		e.ox = 0
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (e *SQLEditor) clampCursor() {
	if e.cy < 0 {
		e.cy = 0
	}
	if e.cy >= len(e.lines) {
		e.cy = len(e.lines) - 1
	}
	if e.cx < 0 {
		e.cx = 0
	}
	if e.cx > len(e.lines[e.cy]) {
		e.cx = len(e.lines[e.cy])
	}
}

func splitLines(text string) []string {
	if text == "" {
		return []string{""}
	}
	return strings.Split(text, "\n")
}

func copyLines(src []string) []string {
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func firstNonWhitespace(s string) int {
	for i, ch := range s {
		if ch != ' ' && ch != '\t' {
			return i
		}
	}
	return len(s)
}

func cursorDisplayCol(lines []string, cy, cx, tabWidth int) int {
	if cy < 0 || cy >= len(lines) {
		return 0
	}
	if cx > len(lines[cy]) {
		cx = len(lines[cy])
	}
	col := 0
	for _, ch := range lines[cy][:cx] {
		if ch == '\t' {
			col += tabWidth - (col % tabWidth)
		} else {
			col++
		}
	}
	return col
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	// Simple conversion (no import needed beyond standard)
	var buf [20]byte
	i := len(buf)
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// ---------------------------------------------------------------------------
// External editor (unchanged from original)
// ---------------------------------------------------------------------------

// openExternalEditor opens the user's preferred editor to edit the query.
// It should be called within app.Suspend() to ensure the TUI is properly restored.
func openExternalEditor(currentText string, connectionURL string) string {
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
		err := tmpFile.Close()
		if err != nil {
			logger.Error("Failed to close temporary file", map[string]any{"error": err.Error()})
		}
		return currentText
	}

	if err := tmpFile.Close(); err != nil {
		logger.Error("Failed to close temporary file", map[string]any{"error": err.Error()})
		return currentText
	}

	if connectionURL != "" {
		err := os.Setenv("LAZYSQL_CONNECTION_URL", connectionURL)
		if err != nil {
			logger.Error("Failed to set environment variable", map[string]any{"error": err.Error()})
			return currentText
		}
		defer os.Unsetenv("LAZYSQL_CONNECTION_URL")
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

// Ensure SQLEditor implements tview.Primitive (compile-time check).
var _ tview.Primitive = (*SQLEditor)(nil)
