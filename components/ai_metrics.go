package components

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

type aiMetricTotals struct{ Calls, Reports, Input, Output, Cached int }

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
				}
				if usage.OutputKnown {
					session.metricTotals.Output += usage.Output
				}
				if usage.CachedKnown {
					session.metricTotals.Cached += usage.Cached
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

func observedDuration(value time.Duration) string {
	if value == 0 {
		return "not observed"
	}
	return value.Round(time.Millisecond).String()
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
	fmt.Fprintf(&detail, "Next request — estimated input\n\n")
	for _, part := range session.nextContext.Parts {
		fmt.Fprintf(&detail, "%-27s ~%s tokens (%d bytes)\n", part.Name, compactCount(part.Tokens), part.Bytes)
	}
	fmt.Fprintf(&detail, "\nEstimate method: %s\nApplication context: %d bytes (exact)\n", session.nextContext.Method, session.nextContext.Bytes)
	if session.contextWindow > 0 {
		fmt.Fprintf(&detail, "Model context window: %d tokens\nPercentages estimate input only; future output also needs room.\n", session.contextWindow)
	} else {
		detail.WriteString("Model context window: unknown (no assumed limit)\n")
	}
	if session.busy && session.currentMetrics.Round > 0 {
		fmt.Fprintf(&detail, "\nActive request: round %d · %d bytes · ~%d tokens\n", session.currentMetrics.Round, session.currentMetrics.Context.Bytes, session.currentMetrics.Context.Tokens)
	}
	last := session.lastMetrics
	fmt.Fprintf(&detail, "\nLast finished model request\n\nInput: %s\nOutput: %s\nCached input: %s (included in input)\n", reportedCount(last.Usage.Input, last.Usage.InputKnown), reportedCount(last.Usage.Output, last.Usage.OutputKnown), reportedCount(last.Usage.Cached, last.Usage.CachedKnown))
	if last.Usage.ReasoningKnown {
		fmt.Fprintf(&detail, "Reasoning: %d tokens (included in output)\n", last.Usage.Reasoning)
	}
	if last.WireBytes > 0 {
		fmt.Fprintf(&detail, "JSON request body: %d bytes (headers excluded)\n", last.WireBytes)
	}
	if last.Complete {
		fmt.Fprintf(&detail, "Turn elapsed: %s\nModel request: %s\nFirst text token: %s\nFirst SQL from send: %s\nSchema tools: %s\n", last.Elapsed.Round(time.Millisecond), last.Duration.Round(time.Millisecond), observedDuration(last.FirstToken), observedDuration(last.FirstSQL), last.Tools.Round(time.Millisecond))
	}
	totals := session.metricTotals
	fmt.Fprintf(&detail, "\nSession totals — reported usage only\n\nModel requests: %d\nUsage reports: %d/%d requests\nInput: %d tokens\nOutput: %d tokens\nCached input: %d tokens\n", totals.Calls, totals.Reports, totals.Calls, totals.Input, totals.Output, totals.Cached)
	omitted := session.nextContext.HistoryOmitted || session.historyOmitted
	if session.currentMetrics.Context.HistoryOmitted && session.busy {
		omitted = true
	}
	fmt.Fprintf(&detail, "\nEarlier conversation omitted: %t\n\nContext percentages and token counts prefixed ~ are estimates, not provider counts.\nCached tokens still occupy context. Session totals are not context size.\nCounting never sends an extra network request.", omitted)
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
		session.focus(session.contextDetails)
	}
	session.refreshControls()
}
