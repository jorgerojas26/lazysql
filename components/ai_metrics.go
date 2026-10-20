package components

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

type aiMetricTotals struct {
	Calls, Reports, Input, Output, Cached int
	InputKnown, OutputKnown, CachedKnown  bool
}

func (session *aiSession) makeRequest(text string) ai.Request {
	prompt, answer := text, ""
	if session.question != "" {
		prompt = session.request
		answer = text
	}
	request := ai.Request{History: append([]ai.ChatMessage(nil), session.history...), HistoryTruncated: session.historyOmitted, Dialect: session.table.DBDriver.GetProvider(), Database: session.database, CurrentTable: session.current, Prompt: prompt, Clarification: session.question, Answer: answer, SelectedSQL: session.selection.text, EditorSQL: session.selection.original, Schemas: append([]string(nil), session.schemas...)}
	if request.SelectedSQL == request.EditorSQL {
		request.EditorSQL = ""
	}
	for index := len(request.History) - 1; index >= 0; index-- {
		previous := request.History[index]
		if previous.SQL == "" {
			continue
		}
		if previous.Database == "" || previous.Database == request.Database {
			if request.EditorSQL == previous.SQL {
				request.EditorSQL = ""
			}
			if session.selection.original == previous.SQL && request.SelectedSQL == previous.SQL {
				request.SelectedSQL = ""
			}
		}
		break
	}
	if session.owner != nil && session.owner.Tree != nil {
		request.Schemas = append([]string(nil), session.owner.Tree.Schemas...)
	}
	preferred := request.Database
	if session.result.Database != "" {
		preferred = session.result.Database
	}
	loader := session.table.schemaMetadataLoader()
	request.SchemaGeneration = loader.cache.generationValue()
	request.Databases, request.DatabaseCatalogComplete, request.Catalog, request.Tables = cachedAIContext(loader, preferred, request.Schemas)
	if generation := loader.cache.generationValue(); generation != request.SchemaGeneration {
		request.Databases = nil
		request.DatabaseCatalogComplete = false
		request.Catalog = nil
		request.Tables = nil
		request.SchemaGeneration = generation
	}
	return request
}

func (session *aiSession) scheduleContext() {
	if session.contextMeter == nil {
		return
	}
	if session.contextTimer != nil {
		session.contextTimer.Stop()
	}
	session.contextVersion++
	version := session.contextVersion
	request := session.makeRequest(session.input.GetText())
	model := session.model
	application := app.App.Application
	session.contextTimer = time.AfterFunc(100*time.Millisecond, func() {
		if session.ctx.Err() != nil {
			return
		}
		size := ai.MeasureContext(model, request)
		if session.ctx.Err() != nil {
			return
		}
		application.QueueUpdateDraw(func() {
			if !session.active() || session.contextVersion != version {
				return
			}
			session.nextContext = size
			session.refreshContext()
		})
	})
}

func (session *aiSession) observeMetrics(run uint64, metrics ai.RequestMetrics) {
	if session.ctx.Err() != nil {
		return
	}
	app.App.QueueUpdateDraw(func() {
		if !session.active() || session.run != run {
			return
		}
		session.currentMetrics = metrics
		if metrics.Round > session.startedRound {
			session.startedRound = metrics.Round
			session.metricTotals.Calls++
		}
		if metrics.Complete {
			session.lastMetrics = metrics
			if metrics.Round > session.measuredRound {
				session.measuredRound = metrics.Round
				logger.Debug("ai_model_request", map[string]any{"round": metrics.Round, "duration_ms": metrics.Duration.Milliseconds(), "first_token_ms": metrics.FirstToken.Milliseconds(), "first_sql_ms": metrics.FirstSQL.Milliseconds(), "context_bytes": metrics.Context.Bytes, "wire_bytes": metrics.WireBytes, "input_tokens": metrics.Usage.Input, "output_tokens": metrics.Usage.Output, "cached_tokens": metrics.Usage.Cached, "usage_reported": metrics.Usage.InputKnown || metrics.Usage.OutputKnown, "failed": metrics.Failed})
				usage := metrics.Usage
				if usage.InputKnown || usage.OutputKnown {
					session.metricTotals.Reports++
				}
				if usage.InputKnown {
					session.metricTotals.Input += usage.Input
					session.metricTotals.InputKnown = true
				}
				if usage.OutputKnown {
					session.metricTotals.Output += usage.Output
					session.metricTotals.OutputKnown = true
				}
				if usage.CachedKnown {
					session.metricTotals.Cached += usage.Cached
					session.metricTotals.CachedKnown = true
				}
			}
		}
		session.refreshContext()
	})
}

func compactCount(value int) string {
	if value >= 1000 {
		return fmt.Sprintf("%.1fk", float64(value)/1000)
	}
	return fmt.Sprint(value)
}

func reportedCount(value int, known bool) string {
	if !known {
		return "not reported"
	}
	return fmt.Sprintf("%d tokens", value)
}

func (session *aiSession) refreshContext() {
	if session.contextMeter == nil {
		return
	}
	size := session.nextContext
	if session.busy && session.currentMetrics.Round > 0 {
		size = session.currentMetrics.Context
	}
	text := "~" + compactCount(size.Tokens) + " tokens"
	if session.contextWindow > 0 {
		window := strconv.Itoa(session.contextWindow)
		if session.contextWindow >= 1000 {
			window = strconv.FormatFloat(float64(session.contextWindow)/1000, 'f', -1, 64) + "k"
		}
		text = fmt.Sprintf("%.0f%%/%s", 100*float64(size.Tokens)/float64(session.contextWindow), window)
	}
	if size.HistoryOmitted || session.historyOmitted {
		text += " · history trimmed"
	}
	session.contextMeter.SetText(text)
	color := app.Styles.SecondaryTextColor
	if session.contextWindow > 0 && size.Tokens*10 >= session.contextWindow*9 {
		color = app.Styles.ErrorColor
	}
	session.contextMeter.SetTextColor(color)
	var detail strings.Builder
	fmt.Fprintf(&detail, "Estimated context: ~%s tokens", compactCount(size.Tokens))
	if session.contextWindow > 0 {
		fmt.Fprintf(&detail, " (%s)", text)
	}
	detail.WriteString("\n\n")
	last := session.lastMetrics.Usage
	fmt.Fprintf(&detail, "Last request\nInput: %s\nOutput: %s\nCached input: %s\n", reportedCount(last.Input, last.InputKnown), reportedCount(last.Output, last.OutputKnown), reportedCount(last.Cached, last.CachedKnown))
	if last.ReasoningKnown {
		fmt.Fprintf(&detail, "Reasoning: %d tokens\n", last.Reasoning)
	}
	totals := session.metricTotals
	fmt.Fprintf(&detail, "\nSession (%d/%d requests reported)\nInput: %s\nOutput: %s\nCached input: %s", totals.Reports, totals.Calls, reportedCount(totals.Input, totals.InputKnown), reportedCount(totals.Output, totals.OutputKnown), reportedCount(totals.Cached, totals.CachedKnown))
	session.contextDetails.SetText(detail.String())
}

func (session *aiSession) toggleContext() {
	name, _ := session.content.GetFrontPage()
	if name == "context" {
		session.content.SwitchToPage("chat")
		session.contextButton.SetLabel("Ctrl+K Context")
		session.focus(session.input)
	} else {
		session.refreshContext()
		session.content.SwitchToPage("context")
		session.contextButton.SetLabel("Ctrl+K Chat")
		_, view := session.contextTabs.GetFrontPage()
		session.focus(view)
	}
	session.refreshControls()
}

func (session *aiSession) switchContextTab(name string) {
	session.contextTabs.SwitchToTabByReference(name)
	session.refreshControls()
	_, view := session.contextTabs.GetFrontPage()
	session.focus(view)
}

func (session *aiSession) observeRequest(run uint64, round int, body []byte) {
	if session.ctx.Err() != nil {
		return
	}
	var formatted bytes.Buffer
	if json.Indent(&formatted, body, "", "  ") != nil {
		return
	}
	text := formatted.String()
	app.App.QueueUpdateDraw(func() {
		if !session.active() || session.run != run {
			return
		}
		session.requestBody = text
		session.requestRound = round
		session.refreshRequest()
	})
}

func (session *aiSession) refreshRequest() {
	text := "No model request yet. Send a message to inspect its full context."
	if session.requestBody != "" {
		text = fmt.Sprintf("Last sent request · round %d\n\n%s", session.requestRound, session.requestBody)
	}
	session.requestDetails.SetText(text).ScrollToBeginning()
}
