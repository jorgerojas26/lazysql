package components

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/internal/ai"
)

func TestAIContextDetailsDistinguishEstimatesUsageAndHistoryOmission(t *testing.T) {
	session := newAILayoutSession(t)
	request := session.makeRequest("Include email")
	request.HistoryTruncated = true
	request.History = []ai.ChatMessage{{Role: "user", Text: strings.Repeat("old", 15000)}, {Role: "assistant", SQL: "SELECT 1;"}, {Role: "user", Text: "Latest"}, {Role: "assistant", SQL: "SELECT id FROM users;"}}
	session.nextContext = ai.MeasureContext("unknown-model", request)
	session.lastMetrics = ai.RequestMetrics{Complete: true, WireBytes: 1234, Usage: ai.Usage{Input: 100, Output: 20, Cached: 80, InputKnown: true, OutputKnown: true, CachedKnown: true}}
	session.metricTotals = aiMetricTotals{Calls: 4, Reports: 2, Input: 200, Output: 40, Cached: 160}
	session.refreshContext()
	meter := session.contextMeter.GetText(false)
	detail := session.contextDetails.GetText(false)
	for _, text := range []string{"~", "history trimmed"} {
		if !strings.Contains(meter, text) {
			t.Fatalf("context indicator omitted %q: %s", text, meter)
		}
	}
	for _, text := range []string{"Model context window: unknown", "Input: 100 tokens", "Cached input: 80 tokens (included in input)", "Usage reports: 2/4", "Earlier conversation omitted: true", "headers excluded", "byte estimate"} {
		if !strings.Contains(detail, text) {
			t.Fatalf("context details omitted %q: %s", text, detail)
		}
	}
	if strings.Contains(meter, "%") {
		t.Fatal("unknown model limit was shown as a percentage")
	}
	session.contextWindow = 128000
	session.refreshContext()
	if !strings.Contains(session.contextMeter.GetText(false), "/128k") || !strings.Contains(session.contextDetails.GetText(false), "future output also needs room") {
		t.Fatal("known model limit is not distinguished from input estimate")
	}
	session.capture(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone))
	if name, _ := session.content.GetFrontPage(); name != "context" || app.App.GetFocus() != session.contextDetails {
		t.Fatal("Ctrl+K did not open context details")
	}
	session.capture(tcell.NewEventKey(tcell.KeyCtrlK, 0, tcell.ModNone))
	if name, _ := session.content.GetFrontPage(); name != "chat" || app.App.GetFocus() != session.input {
		t.Fatal("Ctrl+K did not return to chat")
	}
}

func TestAIContextMeterUsesActiveRequestWithoutLosingNextDraft(t *testing.T) {
	session := newAILayoutSession(t)
	session.nextContext = ai.ContextSize{Bytes: 1000, Tokens: 250}
	session.currentMetrics = ai.RequestMetrics{Round: 2, Context: ai.ContextSize{Bytes: 4000, Tokens: 1000}}
	session.busy = true
	session.refreshContext()
	if !strings.Contains(session.contextMeter.GetText(false), "~1.0k tokens") {
		t.Fatalf("wrong active context: %s", session.contextMeter.GetText(false))
	}
	session.busy = false
	session.refreshContext()
	if !strings.Contains(session.contextMeter.GetText(false), "~250 tokens") {
		t.Fatal("idle context did not return to next request")
	}
}

func TestAIContextMeterShowsModelPercentageAndWindow(t *testing.T) {
	session := newAILayoutSession(t)
	session.contextWindow = 500000
	session.nextContext = ai.ContextSize{Bytes: 120000, Tokens: 25000}
	session.refreshContext()
	if text := session.contextMeter.GetText(false); text != "5%/500k" {
		t.Fatalf("context meter: %q", text)
	}
	session.nextContext.Tokens = 450000
	session.refreshContext()
	if text := session.contextMeter.GetText(false); text != "90%/500k" {
		t.Fatalf("context meter near model limit: %q", text)
	}
	if strings.Contains(session.contextDetails.GetText(false), "96000") {
		t.Fatal("details still show a byte limit")
	}
}
