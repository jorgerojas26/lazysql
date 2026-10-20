package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestContextMeasurementsIncludeAllSuppliedContext(t *testing.T) {
	request := Request{Dialect: "mysql", Database: "app", Prompt: "Last ten users", EditorSQL: "SELECT id FROM users;", SelectedSQL: "id", Schemas: []string{"public"}, Tables: []Table{{Name: "users", Columns: []Column{{Name: "id", Type: "bigint"}}}}, History: []ChatMessage{{Role: "user", Text: "Users"}, {Role: "assistant", SQL: "SELECT id FROM users;"}}}
	size := MeasureContext("unknown-model", request)
	encoded, _ := json.Marshal(request)
	messages, _ := json.Marshal([]Message{{Role: "user", Text: string(encoded)}})
	tools, _ := json.Marshal(metadataTools())
	expected := len(discoveryInstructions) + len(messages) + len(tools)
	if size.Bytes != expected || size.Tokens != (expected+3)/4 {
		t.Fatalf("whole context estimate: %+v, expected %d bytes", size, expected)
	}
	shorter := request
	shorter.Prompt = "x"
	if MeasureContext("unknown-model", shorter).Bytes >= size.Bytes {
		t.Fatal("draft size is not measured")
	}
}

func TestLocalTokenizerDoesNotRequireNetwork(t *testing.T) {
	tokens := countLocal("gpt-4o", "SELECT id FROM users;")
	if tokens != 5 {
		t.Fatalf("local tokenizer: %d", tokens)
	}
	tokens = countLocal("unknown-provider-model", "abcd")
	if tokens != 1 {
		t.Fatalf("unknown model fallback: %d", tokens)
	}
}

func TestProviderUsageDistinguishesActualAndUnknownCounts(t *testing.T) {
	tests := []struct {
		format, body          string
		input, output, cached int
	}{
		{"responses", `{"usage":{"input_tokens":100,"output_tokens":20,"input_tokens_details":{"cached_tokens":60}}}`, 100, 20, 60},
		{"chat-completions", `{"usage":{"prompt_tokens":100,"completion_tokens":20,"prompt_tokens_details":{"cached_tokens":60}}}`, 100, 20, 60},
		{"anthropic", `{"usage":{"input_tokens":10,"output_tokens":20,"cache_read_input_tokens":60,"cache_creation_input_tokens":30}}`, 100, 20, 60},
		{"gemini", `{"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":15,"thoughtsTokenCount":5,"cachedContentTokenCount":60}}`, 100, 20, 60},
	}
	for _, test := range tests {
		usage := decodeUsage(test.format, []byte(test.body))
		if !usage.InputKnown || !usage.OutputKnown || !usage.CachedKnown || usage.Input != test.input || usage.Output != test.output || usage.Cached != test.cached {
			t.Fatalf("%s usage: %+v", test.format, usage)
		}
	}
	unknown := decodeUsage("responses", []byte(`{}`))
	if unknown.InputKnown || unknown.OutputKnown || unknown.CachedKnown {
		t.Fatal("missing usage was shown as actual zero")
	}
	zero := decodeUsage("responses", []byte(`{"usage":{"input_tokens":0,"output_tokens":0,"input_tokens_details":{"cached_tokens":0}}}`))
	if !zero.InputKnown || !zero.OutputKnown || !zero.CachedKnown {
		t.Fatal("reported zero was discarded")
	}
}

func TestStreamingUsageAfterTerminalTextIsRetained(t *testing.T) {
	var stream strings.Builder
	for _, event := range streamTextEvent("chat-completions", generatedResult, true) {
		writeSSE(&stream, event)
	}
	writeSSE(&stream, map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": 123, "completion_tokens": 45, "prompt_tokens_details": map[string]int{"cached_tokens": 80}}})
	stream.WriteString("data: [DONE]\n\n")
	turn, err := readOpenCodeStream(strings.NewReader(stream.String()), "chat-completions", nil)
	if err != nil || turn.Usage.Input != 123 || turn.Usage.Output != 45 || turn.Usage.Cached != 80 {
		t.Fatalf("stream usage: %+v %v", turn.Usage, err)
	}
	response := map[string]any{"type": "response.completed", "response": map[string]any{"status": "completed", "usage": map[string]int{"input_tokens": 77, "output_tokens": 12}, "output": []any{map[string]any{"type": "message", "content": []any{map[string]string{"type": "output_text", "text": generatedResult}}}}}}
	stream.Reset()
	writeSSE(&stream, response)
	turn, err = readResponsesStream(strings.NewReader(stream.String()), "ChatGPT", nil)
	if err != nil || turn.Usage.Input != 77 || turn.Usage.Output != 12 {
		t.Fatalf("Responses usage: %+v %v", turn.Usage, err)
	}
}

func TestDiscoveryReportsEachRequestWithoutTrimmingHistory(t *testing.T) {
	events := []RequestMetrics{}
	client := stepFunc(func(ctx context.Context, _ string, _ []Message, _ []Tool) (Turn, error) {
		recordRequest(ctx, make([]byte, 1234))
		return Turn{Text: generatedResult, Usage: Usage{Input: 100, Output: 20, InputKnown: true, OutputKnown: true}}, nil
	})
	request := Request{Prompt: "Users", History: []ChatMessage{{Role: "user", Text: strings.Repeat("x", 40000)}, {Role: "assistant", SQL: "SELECT 1;"}, {Role: "user", Text: "Latest"}, {Role: "assistant", SQL: "SELECT id FROM users;"}}}
	result, err := DiscoverSQLStream(context.Background(), client, request, func(context.Context, string, json.RawMessage) (any, error) { return nil, nil }, func(Result) {}, DiscoveryOptions{Model: "unknown-model", Observe: func(metrics RequestMetrics) { events = append(events, metrics) }})
	if err != nil || result.SQL == "" || len(events) != 2 || events[0].Complete || !events[1].Complete || events[1].Context.HistoryOmitted || events[1].WireBytes != 1234 || events[1].Usage.Input != 100 {
		t.Fatalf("request measurements: %+v %v", events, err)
	}
}
