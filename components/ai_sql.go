package components

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/internal/ai"
	"github.com/jorgerojas26/lazysql/models"
)

func captureAIShortcut(event *tcell.EventKey) *tcell.EventKey {
	if !app.App.AIConfig().Enabled || app.Keymaps.Resolve(event) != commands.GenerateSQL {
		return event
	}
	_, page := mainPages.GetFrontPage()
	home, ok := page.(*Home)
	if !ok {
		return event
	}
	home.openAIPrompt()
	return nil
}

func (home *Home) aiDatabase() string {
	if database := home.Tree.GetSelectedDatabase(); database != "" {
		return database
	}
	if node := home.Tree.GetCurrentNode(); node != nil {
		if reference, ok := node.GetReference().(string); ok && reference != "" {
			return strings.SplitN(reference, ".", 2)[0]
		}
	}
	if database := extractDatabaseName(home.ConnectionURL); database != "" {
		return database
	}
	if root := home.Tree.GetRoot(); root != nil {
		children := root.GetChildren()
		if len(children) == 1 {
			if database, ok := children[0].GetReference().(string); ok {
				return database
			}
		}
	}
	return ""
}

func (home *Home) openAIPrompt() {
	if !app.App.AIConfig().Enabled || home.TabbedPane == nil || home.Tree == nil {
		return
	}
	if home.aiChat != nil && home.aiChat.active() {
		home.aiChat.show()
		return
	}
	current := home.TabbedPane.GetCurrentTab()
	database := home.aiDatabase()
	if current != nil {
		if table, ok := current.Content.(*ResultsTable); ok {
			if table.Editor != nil {
				if table.GetDatabaseName() == "" {
					table.SetDatabaseName(database)
				}
				table.openAIPrompt()
				return
			}
			if table.GetDatabaseName() != "" {
				database = table.GetDatabaseName()
			}
		}
	}
	var table *ResultsTable
	for tab := home.TabbedPane.state.FirstTab; tab != nil; tab = tab.NextTab {
		candidate, ok := tab.Content.(*ResultsTable)
		if ok && candidate.Editor != nil && candidate.GetDatabaseName() == database {
			table = candidate
			break
		}
	}
	detached := table == nil
	if table == nil {
		table = NewResultsTable(&home.ListOfDBChanges, home.Tree, home.DBDriver, home, home.ConnectionIdentifier, home.ConnectionURL, home.ReadOnly)
		table.SetDatabaseName(database)
		table.WithEditor()
	}
	table.openAIPrompt()
	if home.aiChat != nil {
		home.aiChat.selection = aiSelection{original: table.Editor.GetText()}
		home.aiChat.sourceClosed = detached
	}
}

type aiSelection struct {
	original string
	text     string
	start    int
	end      int
}

func captureAISelection(editor *SQLEditor) aiSelection {
	selection := aiSelection{original: editor.GetText()}
	if !editor.selecting {
		return selection
	}
	sl, sc, el, ec := editor.getSelectionRange()
	if editor.vimMode == VimModeVisualLine {
		sc = 0
		ec = len(editor.lines[el])
	}
	for i := 0; i < sl; i++ {
		selection.start += len(editor.lines[i]) + 1
	}
	selection.start += sc
	for i := 0; i < el; i++ {
		selection.end += len(editor.lines[i]) + 1
	}
	selection.end += min(ec, len(editor.lines[el]))
	selection.text = selection.original[selection.start:selection.end]
	return selection
}

func (selection aiSelection) apply(editor *SQLEditor, sql string) error {
	if selection.text == "" {
		return errors.New("no SQL selection to replace")
	}
	if editor.GetText() != selection.original {
		return errors.New("editor changed; discard and generate again to avoid replacing newer work")
	}
	editor.SetText(selection.original[:selection.start]+sql+selection.original[selection.end:], true)
	editor.vimMode = VimModeInsert
	return nil
}

type aiSession struct {
	config          models.AIConfig
	selectedModel   ai.Model
	modelCatalog    []ai.Model
	catalogLoading  bool
	catalogLoaded   bool
	catalogError    string
	modelLoader     func(context.Context, models.AIConfig) ([]ai.Model, error)
	picker          *aiPicker
	previousTab     *Tab
	startedRound    int
	streamQueue     chan aiStreamUpdate
	historyVersion  uint64
	renderedVersion uint64
	cachedHistory   string
	cachedWidth     int
	cachedTheme     [13]tcell.Color
	sqlBlocks       map[string]string
	contextMeter    *tview.TextView
	contextTabs     *TabbedPane
	requestDetails  *tview.TextView
	requestBody     string
	requestRound    int
	contextDetails  *tview.TextView
	content         *tview.Pages
	contextButton   *tview.Button
	nextContext     ai.ContextSize
	currentMetrics  ai.RequestMetrics
	lastMetrics     ai.RequestMetrics
	metricTotals    aiMetricTotals
	measuredRound   int
	historyOmitted  bool
	streaming       ai.Result
	chatWidth       int
	lastSent        string
	table           *ResultsTable
	owner           *Home
	surface         *tview.Pages
	visible         bool
	sourceClosed    bool
	previousFocus   tview.Primitive
	client          ai.ToolCompleter
	ctx             context.Context
	cancel          context.CancelFunc
	runCancel       context.CancelFunc
	run             uint64
	busy            bool
	database        string
	current         string
	schemas         []string
	selection       aiSelection
	input           *aiPrompt
	transcript      *tview.TextView
	status          *tview.TextView
	panel           *tview.Flex
	buttons         *tview.Flex
	useButton       *tview.Button
	stopButton      *tview.Button
	newButton       *tview.Button
	controlsWidth   int
	focusable       []tview.Primitive
	history         []ai.ChatMessage
	result          ai.Result
}

func (table *ResultsTable) openAIPrompt() {
	config := app.App.AIConfig()
	if !config.Enabled || table.Home == nil || table.Editor == nil || table.aiSurface == nil {
		return
	}
	if table.Home.aiChat != nil && table.Home.aiChat.active() {
		table.Home.aiChat.show()
		return
	}
	ctx, cancel := context.WithCancel(app.App.Context())
	session := &aiSession{table: table, owner: table.Home, surface: table.aiSurface, ctx: ctx, cancel: cancel, database: table.GetDatabaseName(), selection: captureAISelection(table.Editor)}
	if table.Home.Tree != nil {
		session.current = table.Home.Tree.GetSelectedTable()
		session.schemas = slices.Clone(table.Home.Tree.Schemas)
	}
	session.owner.aiChat = session
	table.Editor.acVisible = false
	session.buildUI()
	session.show()
	client, err := ai.New(config)
	if err != nil {
		session.showResponse("AI setup", err.Error(), true)
		return
	}
	session.client = client
}

func (session *aiSession) active() bool {
	return session.ctx.Err() == nil && session.owner != nil && session.owner.aiChat == session
}

func (session *aiSession) focus(primitive tview.Primitive) {
	if !session.visible {
		return
	}
	if _, page := mainPages.GetFrontPage(); page != session.surface {
		return
	}
	if primitive == session.input {
		if name, _ := session.content.GetFrontPage(); name == "context" {
			_, primitive = session.contextTabs.GetFrontPage()
		}
	}
	app.App.SetFocus(primitive)
}

func (session *aiSession) show() {
	if !session.visible {
		session.previousFocus = app.App.GetFocus()
		session.previousTab = nil
		if session.owner != nil {
			session.previousTab = session.owner.TabbedPane.GetCurrentTab()
		}
	}
	session.showPanel(session.panel, 26)
	session.focus(session.input)
}

func (session *aiSession) hide() {
	session.closePicker()
	session.visible = false
	if _, page := mainPages.GetFrontPage(); page != session.surface {
		return
	}
	mainPages.RemovePage("ai-modal")
	restoreFocus := session.previousFocus != nil
	if restoreFocus && session.owner != nil && session.previousTab != nil && session.owner.TabbedPane.GetTabByReference(session.previousTab.Reference) != session.previousTab {
		restoreFocus = session.previousFocus == session.owner.Tree || session.previousFocus == session.owner.Tree.Filter
	}
	if restoreFocus {
		app.App.SetFocus(session.previousFocus)
	} else if session.owner != nil {
		if tab := session.owner.TabbedPane.GetCurrentTab(); tab != nil {
			app.App.SetFocus(tab.Content.GetPrimitive())
		} else {
			app.App.SetFocus(session.owner.Tree)
		}
	}
}

func (session *aiSession) close() {
	session.cancel()
	if session.runCancel != nil {
		session.runCancel()
	}
	session.hide()
	if session.owner != nil && session.owner.aiChat == session {
		session.owner.aiChat = nil
	}
	session.surface.RemovePage("ai")
}

func (session *aiSession) stop() {
	if !session.busy {
		return
	}
	if session.runCancel != nil {
		session.runCancel()
	}
	session.run++
	session.runCancel = nil
	session.busy = false
	session.streaming = ai.Result{}
	session.input.SetDisabled(false)
	if session.input.GetText() == "" {
		session.input.SetText(session.lastSent)
	}
	session.showResponse("Stopped", "Generation stopped.", false)
	session.refreshControls()
	session.focus(session.input)
}

func (session *aiSession) newChat() {
	session.closePicker()
	if session.runCancel != nil {
		session.runCancel()
	}
	session.run++
	session.runCancel = nil
	session.busy = false
	session.history = nil
	session.historyVersion++
	session.cachedHistory = ""
	session.sqlBlocks = nil
	session.historyOmitted = false
	session.metricTotals = aiMetricTotals{}
	session.currentMetrics = ai.RequestMetrics{}
	session.lastMetrics = ai.RequestMetrics{}
	session.nextContext = ai.ContextSize{}
	session.content.SwitchToPage("chat")
	session.contextButton.SetLabel("Ctrl+K Context")
	session.switchContextTab("usage")
	session.requestBody = ""
	session.requestRound = 0
	session.refreshRequest()
	session.measuredRound = 0
	session.startedRound = 0
	session.result = ai.Result{}
	session.streaming = ai.Result{}
	session.lastSent = ""
	session.input.SetDisabled(false)
	session.input.SetText("").SetPlaceholder("Ask for a query…")
	session.status.SetText("")
	if session.owner != nil {
		session.database = session.owner.aiDatabase()
		session.current = session.owner.Tree.GetSelectedTable()
		if tab := session.owner.TabbedPane.GetCurrentTab(); tab != nil {
			if table, ok := tab.Content.(*ResultsTable); ok && table.GetDatabaseName() != "" {
				session.database = table.GetDatabaseName()
			}
			if table, ok := tab.Content.(*ResultsTable); ok && table.Editor != nil {
				session.table = table
				session.sourceClosed = false
				session.database = table.GetDatabaseName()
			} else {
				session.sourceClosed = true
			}
		} else {
			session.sourceClosed = true
		}
	}
	session.selection = aiSelection{original: session.table.Editor.GetText()}
	session.renderChat()
	session.refreshControls()
	session.measureContext()
	session.focus(session.input)
}

func (session *aiSession) generate() {
	if session.client == nil {
		return
	}
	if session.busy {
		return
	}
	text := strings.TrimSpace(session.input.GetText())
	if text == "" {
		return
	}
	request := session.makeRequest(text)
	if session.selectedModel.ContextWindow > 0 {
		size := ai.MeasureContext(session.selectedModel.ID, request)
		if size.Tokens+max(8192, session.config.ThinkingBudget+8192) > session.selectedModel.ContextWindow {
			session.confirmContext(text, request, size.Tokens)
			return
		}
	}
	session.sendRequest(text, request)
}

func (session *aiSession) sendRequest(text string, request ai.Request) {
	session.streaming = ai.Result{}
	session.lastSent = text
	session.appendMessage(ai.ChatMessage{Role: "user", Text: text})
	ctx, cancel := context.WithTimeout(session.ctx, 120*time.Second)
	session.runCancel = cancel
	session.run++
	run := session.run
	session.busy = true
	session.measuredRound = 0
	session.startedRound = 0
	session.currentMetrics = ai.RequestMetrics{}
	session.input.SetText("")
	session.showResponse("Discovering", "Finding relevant schema…", false)
	session.refreshControls()
	schemas := slices.Clone(request.Schemas)
	loader := session.table.schemaMetadataLoader()
	generation := request.SchemaGeneration
	client, model := session.client, session.selectedModel.ID
	go func() {
		defer cancel()
		metadata := &aiSchemaTools{loader: loader, database: request.Database, schemasFilter: schemas}
		request.Schemas = schemas
		result, err := ai.DiscoverSQLStream(ctx, client, request, metadata.execute, func(update ai.Result) { session.stream(run, update) }, ai.DiscoveryOptions{Model: model, Observe: func(metrics ai.RequestMetrics) { session.observeMetrics(run, metrics) }, ObserveRequest: func(round int, body []byte) { session.observeRequest(run, round, body) }})
		if err == nil && loader.cache.generationValue() != generation {
			err = errors.New("schema changed during generation; retry")
		}
		if err == nil && result.SQL != "" {
			result.Database, err = metadata.resolveDatabase(ctx, result.Database)
		}
		session.finish(run, result, err)
	}()
}

func (session *aiSession) finish(run uint64, result ai.Result, err error) {
	if session.ctx.Err() != nil {
		return
	}
	app.App.QueueUpdateDraw(func() {
		if !session.active() || session.run != run {
			return
		}
		session.busy = false
		session.streaming = ai.Result{}
		session.input.SetDisabled(false)
		session.runCancel = nil
		session.status.SetText("")
		if err != nil {
			text := err.Error()
			if errors.Is(err, context.DeadlineExceeded) {
				text = "Generation timed out. Try again."
			}
			session.showResponse("Error", text, true)
			if session.input.GetText() == "" {
				session.input.SetText(session.lastSent)
			}
		} else if result.Missing != "" {
			session.showResponse("Schema unavailable", result.Missing, true)
		} else if result.Clarification != "" {
			session.showResponse("Question", result.Clarification, false)
			session.input.SetPlaceholder("Your answer…")
		} else {
			if result.SQL != "" {
				session.result = result
				session.showPreview()
			} else {
				session.showResponse("AI", result.Message, false)
			}
			session.input.SetPlaceholder("Ask a follow-up…")
		}
		session.refreshControls()
		session.measureContext()
		session.focus(session.input)
	})
}

func (session *aiSession) usedQuery(table *ResultsTable, sql string, start int) {
	session.table = table
	session.sourceClosed = false
	session.database = table.GetDatabaseName()
	session.selection = aiSelection{original: table.Editor.GetText(), text: sql, start: start, end: start + len(sql)}
	table.SetIsFiltering(true)
	if session.owner != nil {
		session.owner.FocusedWrapper = focusedWrapperRight
		for tab := session.owner.TabbedPane.state.FirstTab; tab != nil; tab = tab.NextTab {
			if tab.Content == table {
				session.owner.TabbedPane.SwitchToTabByReference(tab.Reference)
				break
			}
		}
	}
	session.hide()
	app.App.SetFocus(table.Editor)
}

func (session *aiSession) accept() error {
	return session.acceptResult(session.result)
}

func (session *aiSession) acceptResult(result ai.Result) error {
	if session.busy || result.SQL == "" {
		return errors.New("no generated query to use")
	}
	database := result.Database
	if database == "" {
		database = session.database
	}
	sql := result.SQL
	if !session.sourceClosed && session.selection.text != "" && database == session.table.GetDatabaseName() && session.table.Editor.GetText() == session.selection.original {
		if err := session.selection.apply(session.table.Editor, sql); err != nil {
			return err
		}
		session.usedQuery(session.table, sql, session.selection.start)
		return nil
	}
	if !session.sourceClosed && session.selection.original == "" && session.table.Editor.GetText() == "" {
		changedDatabase := database != session.database
		session.table.SetDatabaseName(database)
		session.table.Editor.SetText(sql, true)
		session.table.SetIsFiltering(true)
		session.usedQuery(session.table, sql, 0)
		if changedDatabase {
			go session.table.loadEditorSchema()
		}
		return nil
	}
	home := session.owner
	if home == nil {
		home = session.table.Home
	}
	if home == nil {
		return errors.New("cannot create a SQL tab in this view")
	}
	table := NewResultsTable(&home.ListOfDBChanges, home.Tree, home.DBDriver, home, home.ConnectionIdentifier, home.ConnectionURL, home.ReadOnly)
	table.SetDatabaseName(database)
	table.WithEditor()
	table.Editor.SetText(sql, true)
	reference := home.TabbedPane.NextAvailableReference("ai-sql")
	home.TabbedPane.AppendTab(fmt.Sprintf("SQL · %s", reference), table, reference)
	table.SetIsFiltering(true)
	session.usedQuery(table, sql, 0)
	go table.loadEditorSchema()
	return nil
}
