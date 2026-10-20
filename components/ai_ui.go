package components

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

type aiContextTab struct{ tview.Primitive }

func (tab aiContextTab) GetPrimitive() tview.Primitive {
	return tab.Primitive
}

type aiPrompt struct{ *tview.TextArea }

func (prompt *aiPrompt) SetText(text string) *aiPrompt {
	prompt.TextArea.SetText(text, true)
	return prompt
}

type aiModal struct {
	*tview.Flex
	panel   *tview.Flex
	session *aiSession
	height  int
}

func (modal *aiModal) Draw(screen tcell.Screen) {
	x, y, width, height := modal.GetRect()
	w, h := min(92, width), min(modal.height, height)
	modal.session.layoutControls(max(1, w-4))
	inputHeight := min(4, max(2, h/4))
	if name, _ := modal.session.content.GetFrontPage(); name == "context" {
		inputHeight = 0
	}
	modal.session.panel.ResizeItem(modal.session.input, inputHeight, 0)
	modal.panel.SetRect(x+(width-w)/2, y+(height-h)/2, w, h)
	if chatWidth := max(1, w-6); chatWidth != modal.session.chatWidth {
		modal.session.chatWidth = chatWidth
		modal.session.renderChat()
	}
	if theme := chatTheme(); theme != modal.session.cachedTheme {
		modal.session.renderChat()
	}
	modal.panel.Draw(screen)
}

func (modal *aiModal) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return modal.WrapMouseHandler(func(action tview.MouseAction, event *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		if !modal.InRect(event.Position()) {
			return false, nil
		}
		if modal.panel.InRect(event.Position()) {
			if handler := modal.panel.MouseHandler(); handler != nil {
				_, capture := handler(action, event, setFocus)
				return true, capture
			}
		}
		return true, nil
	})
}

func (session *aiSession) showPanel(panel *tview.Flex, height int) {
	session.visible = true
	modal := &aiModal{Flex: tview.NewFlex().AddItem(panel, 0, 1, true), panel: panel, session: session, height: height}
	session.surface.AddAndSwitchToPage("ai", modal, true)
	mainPages.AddPage("ai-modal", session.surface, true, true)
}

func (session *aiSession) buildUI() {
	session.surface.Box = tview.NewFlex().Box
	session.input = &aiPrompt{TextArea: tview.NewTextArea().SetPlaceholder("Ask for a query…").SetWrap(true).SetWordWrap(true)}
	session.input.SetTextStyle(tcell.StyleDefault.Foreground(app.Styles.PrimaryTextColor).Background(app.Styles.PrimitiveBackgroundColor))
	session.input.SetBorder(true).SetTitle(" Message ").SetBorderPadding(0, 0, 1, 1)
	session.transcript = tview.NewTextView().SetWrap(true).SetWordWrap(true).SetScrollable(true).SetDynamicColors(true).SetRegions(true)
	session.transcript.SetBorderPadding(0, 0, 1, 1)
	session.transcript.SetHighlightedFunc(func(added, removed, remaining []string) {
		if len(added) == 0 {
			return
		}
		region := added[0]
		session.transcript.Highlight()
		index, err := strconv.Atoi(strings.TrimPrefix(region, "sql-"))
		if err != nil || !strings.HasPrefix(region, "sql-") || index < 0 || index >= len(session.history) || session.busy {
			return
		}
		message := session.history[index]
		if message.Role != "assistant" || message.Error || message.SQL == "" {
			return
		}
		session.useResult(ai.Result{SQL: message.SQL, Database: message.Database, Message: message.Text, Warning: message.Warning})
	})
	session.startStreamUI()
	session.status = tview.NewTextView().SetWrap(false).SetTextColor(app.Styles.SecondaryTextColor)
	session.useButton = tview.NewButton("Ctrl+U Use query").SetSelectedFunc(session.useQuery)
	session.stopButton = tview.NewButton("Ctrl+X Stop").SetSelectedFunc(session.stop)
	session.newButton = tview.NewButton("Ctrl+N New chat").SetSelectedFunc(session.newChat)
	session.contextButton = tview.NewButton("Ctrl+K Context").SetSelectedFunc(session.toggleContext)
	session.contextMeter = tview.NewTextView().SetWrap(false).SetText("Context: measuring…")
	session.contextDetails = tview.NewTextView().SetWrap(true).SetWordWrap(true).SetScrollable(true)
	session.contextDetails.SetBorderPadding(0, 0, 1, 1)
	session.requestDetails = tview.NewTextView().SetWrap(true).SetWordWrap(true).SetScrollable(true)
	session.requestDetails.SetBorderPadding(0, 0, 1, 1)
	session.contextTabs = NewTabbedPane()
	session.contextTabs.AppendTab("Usage", aiContextTab{session.contextDetails}, "usage")
	session.contextTabs.AppendTab("Conversation", aiContextTab{session.requestDetails}, "conversation")
	previousFocus := app.App.GetFocus()
	session.contextTabs.SwitchToTabByReference("usage")
	app.App.SetFocus(previousFocus)
	for tab := session.contextTabs.state.FirstTab; tab != nil; tab = tab.NextTab {
		tab.Header.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
			if action == tview.MouseLeftClick && tab.Header.InRect(event.Position()) {
				session.switchContextTab(tab.Reference)
				return tview.MouseConsumed, nil
			}
			return action, event
		})
		tab.Header.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEnter || (event.Key() == tcell.KeyRune && event.Rune() == ' ') {
				session.switchContextTab(tab.Reference)
				return nil
			}
			return event
		})
	}
	contextPanel := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(session.contextTabs.HeaderContainer, 1, 0, false).AddItem(session.contextTabs.Pages, 0, 1, false)
	session.content = tview.NewPages().AddPage("chat", session.transcript, true, true).AddPage("context", contextPanel, true, false)
	session.refreshRequest()
	config := app.App.AIConfig()
	session.model, session.contextWindow = config.Model, config.ContextWindow
	session.buttons = tview.NewFlex().SetDirection(tview.FlexRow)
	session.panel = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(session.content, 0, 1, false).AddItem(session.status, 1, 0, false).AddItem(session.input, 4, 0, true).AddItem(session.contextMeter, 1, 0, false).AddItem(session.buttons, 1, 0, false)
	title := " AI SQL "
	if session.owner != nil && session.owner.ConnectionIdentifier != "" {
		title = " AI SQL · " + tview.Escape(session.owner.ConnectionIdentifier) + " "
	}
	session.panel.Box = tview.NewBox().SetBackgroundColor(app.Styles.PrimitiveBackgroundColor)
	session.panel.SetBorder(true).SetTitle(title).SetBorderPadding(0, 0, 1, 1)
	session.panel.SetInputCapture(session.capture)
	session.input.SetChangedFunc(session.scheduleContext)
	session.refreshControls()
	session.scheduleContext()
}

func (session *aiSession) actions() []*tview.Button {
	buttons := []*tview.Button{}
	if session.result.SQL != "" {
		buttons = append(buttons, session.useButton)
	}
	if session.busy {
		buttons = append(buttons, session.stopButton)
	}
	return append(buttons, session.newButton, session.contextButton)
}

func (session *aiSession) layoutControls(width int) {
	if session.controlsWidth == width {
		return
	}
	session.controlsWidth = width
	session.buttons.Clear()
	row := tview.NewFlex()
	rows, used := 1, 0
	for _, button := range session.actions() {
		size := min(width, len(button.GetLabel())+2)
		if used > 0 && used+size+1 > width {
			session.buttons.AddItem(row, 1, 0, false)
			row = tview.NewFlex()
			rows++
			used = 0
		}
		if used > 0 {
			row.AddItem(nil, 1, 0, false)
			used++
		}
		row.AddItem(button, size, 0, false)
		used += size
	}
	session.buttons.AddItem(row, 1, 0, false)
	session.panel.ResizeItem(session.buttons, rows, 0)
}

func (session *aiSession) refreshControls() {
	session.useButton.SetDisabled(session.busy || session.result.SQL == "")
	name, content := session.content.GetFrontPage()
	if name == "context" {
		_, content = session.contextTabs.GetFrontPage()
	}
	session.focusable = []tview.Primitive{content}
	if name == "chat" {
		session.focusable = append([]tview.Primitive{session.input}, session.focusable...)
	}
	if name == "context" {
		for tab := session.contextTabs.state.FirstTab; tab != nil; tab = tab.NextTab {
			session.focusable = append(session.focusable, tab.Header)
		}
	}
	for _, button := range session.actions() {
		if !button.IsDisabled() {
			session.focusable = append(session.focusable, button)
		}
	}
	width := session.controlsWidth
	if width < 1 {
		width = 88
	}
	session.controlsWidth = 0
	session.layoutControls(width)
}

func (session *aiSession) appendMessage(message ai.ChatMessage) {
	session.history = append(session.history, message)
	session.historyVersion++
	if len(session.history) > 64 {
		session.historyOmitted = true
		session.history = slices.Clone(session.history[len(session.history)-64:])
	}
	session.renderChat()
	session.scheduleContext()
}

func chatTheme() [13]tcell.Color {
	styles := app.Styles
	return [13]tcell.Color{styles.PrimaryTextColor, styles.SecondaryTextColor, styles.ErrorColor, styles.SQLKeywordColor, styles.SQLStringColor, styles.SQLNumberColor, styles.SQLCommentColor, styles.SQLFunctionColor, styles.SQLOperatorColor, styles.SQLTypeColor, styles.SQLBooleanColor, styles.SQLParameterColor, styles.PrimitiveBackgroundColor}
}

func (session *aiSession) messageMarkup(message ai.ChatMessage, index, width int) string {
	var text strings.Builder
	label := "AI"
	color := app.Styles.SecondaryTextColor
	if message.Role == "user" {
		label = "You"
		color = app.Styles.PrimaryTextColor
	}
	if message.Error {
		color = app.Styles.ErrorColor
	}
	if message.Database != "" && message.SQL != "" {
		label += " · " + message.Database
	}
	fmt.Fprintf(&text, "[%s::b]%s[-:-:-]\n", color, tview.Escape(label))
	if message.Text != "" {
		text.WriteString(tview.Escape(message.Text))
		text.WriteByte('\n')
	}
	if message.SQL != "" {
		clickable := index >= 0 && message.Role == "assistant" && !message.Error
		if clickable {
			fmt.Fprintf(&text, `["sql-%d"]`, index)
		}
		block := ""
		if index >= 0 {
			block = session.sqlBlocks[message.SQL]
		}
		if block == "" {
			block = chatSQLBlock(message.SQL, width)
			if index >= 0 {
				if session.sqlBlocks == nil {
					session.sqlBlocks = map[string]string{}
				}
				session.sqlBlocks[message.SQL] = block
			}
		}
		text.WriteString(block)
		if clickable {
			text.WriteString(`[""]`)
		}
		text.WriteByte('\n')
	}
	if message.Warning != "" {
		text.WriteString(tview.Escape("Note: " + message.Warning))
		text.WriteByte('\n')
	}
	text.WriteByte('\n')
	return text.String()
}

func (session *aiSession) renderChat() {
	width := session.chatWidth
	if width < 1 {
		width = 84
	}
	row, column := session.transcript.GetScrollOffset()
	_, _, _, height := session.transcript.GetInnerRect()
	following := row+height >= session.transcript.GetWrappedLineCount()-1
	theme := chatTheme()
	changed := session.cachedWidth != width || session.cachedTheme != theme
	if changed {
		session.sqlBlocks = nil
		session.cachedWidth = width
		session.cachedTheme = theme
	}
	if changed || session.renderedVersion != session.historyVersion || (session.cachedHistory == "" && len(session.history) > 0) {
		var text strings.Builder
		for index, message := range session.history {
			text.WriteString(session.messageMarkup(message, index, width))
		}
		session.cachedHistory = text.String()
		session.renderedVersion = session.historyVersion
	}
	partial := session.streaming
	content := partial.Message
	if content == "" {
		content = partial.Clarification
	}
	if content == "" {
		content = partial.Missing
	}
	text := session.cachedHistory
	if content != "" || partial.SQL != "" {
		text += session.messageMarkup(ai.ChatMessage{Role: "assistant", Text: content, SQL: partial.SQL, Database: partial.Database, Warning: partial.Warning}, -1, width)
	}
	if text == session.transcript.GetText(false) {
		return
	}
	session.transcript.SetText(text)
	if following {
		session.transcript.ScrollToEnd()
	} else {
		session.transcript.ScrollTo(row, column)
	}
}

func (session *aiSession) showResponse(title, text string, failure bool) {
	if session.busy && title == "Discovering" {
		session.status.SetText(text)
		return
	}
	session.status.SetText("")
	if text != "" {
		session.appendMessage(ai.ChatMessage{Role: "assistant", Text: text, Error: failure})
	}
}

func (session *aiSession) showPreview() {
	session.appendMessage(ai.ChatMessage{Role: "assistant", Text: session.result.Message, SQL: session.result.SQL, Database: session.result.Database, Warning: session.result.Warning})
	session.refreshControls()
}

func (session *aiSession) useQuery() {
	session.useResult(session.result)
}

func (session *aiSession) useResult(result ai.Result) {
	if session.busy || result.SQL == "" {
		return
	}
	if err := session.acceptResult(result); err != nil {
		session.showResponse("Error", err.Error(), true)
		session.refreshControls()
		session.focus(session.input)
	}
}

func (session *aiSession) capture(event *tcell.EventKey) *tcell.EventKey {
	if app.Keymaps.Resolve(event) == commands.GenerateSQL {
		session.hide()
		return nil
	}
	if name, _ := session.content.GetFrontPage(); name == "context" && app.App.GetFocus() != session.input {
		if event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModNone {
			switch event.Rune() {
			case '1':
				session.switchContextTab("usage")
				return nil
			case '2':
				session.switchContextTab("conversation")
				return nil
			}
		}
		switch app.Keymaps.Group(app.TableGroup).Resolve(event) {
		case commands.TabPrev:
			session.contextTabs.SwitchToPreviousTab()
			session.refreshControls()
			return nil
		case commands.TabNext:
			session.contextTabs.SwitchToNextTab()
			session.refreshControls()
			return nil
		}
	}
	switch event.Key() {
	case tcell.KeyCtrlU:
		session.useQuery()
		return nil
	case tcell.KeyCtrlK:
		session.toggleContext()
		return nil
	case tcell.KeyCtrlN:
		session.newChat()
		return nil
	case tcell.KeyCtrlX:
		session.stop()
		return nil
	case tcell.KeyEscape:
		session.hide()
		return nil
	case tcell.KeyTab, tcell.KeyBacktab:
		index := slices.Index(session.focusable, app.App.GetFocus())
		if event.Key() == tcell.KeyTab {
			index++
		} else {
			index += len(session.focusable) - 1
		}
		session.focus(session.focusable[index%len(session.focusable)])
		return nil
	case tcell.KeyEnter:
		if app.App.GetFocus() == session.input {
			if event.Modifiers()&tcell.ModCtrl != 0 {
				return tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)
			}
			session.generate()
			return nil
		}
	}
	return event
}
