package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestPartialResultDecodesOnlyReadableFields(t *testing.T) {
	for _, test := range []struct{ input, sql, message string }{
		{`{"message":"Working`, "", "Working"},
		{`{"sql":"SELECT id\nFROM users`, "SELECT id\nFROM users", ""},
		{`{"sql":"SELECT 'a\\nb'`, `SELECT 'a\nb'`, ""},
		{`{"sql":"SELECT '\uD83D`, "SELECT '", ""},
		{`{"sql":"SELECT '\uD83D\uDE42'`, "SELECT '🙂'", ""},
		{`{"database":"app","message":"Here","sql":"SELECT \\"`, `SELECT \`, "Here"},
		{`raw non-JSON response`, "", ""},
	} {
		result := partialResult(test.input)
		if result.SQL != test.sql || result.Message != test.message {
			t.Fatalf("partial %q: %+v", test.input, result)
		}
	}
	full := `{"message":"Olá 🙂","sql":"SELECT 'café', '\\n';","database":"app"}`
	for end := 0; end <= len(full); end++ {
		result := partialResult(full[:end])
		if !utf8.ValidString(result.Message) || !utf8.ValidString(result.SQL) {
			t.Fatalf("invalid UTF-8 escaped at prefix %d", end)
		}
	}
}

func writeSSE(writer io.Writer, event any) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(writer, "data: %s\n\n", data)
}

func streamTextEvent(format, text string, finish bool) []any {
	switch format {
	case "responses":
		events := []any{map[string]any{"type": "response.output_text.delta", "delta": text}}
		if finish {
			events = append(events, map[string]any{"type": "response.completed", "response": map[string]any{"status": "completed"}})
		}
		return events
	case "anthropic":
		events := []any{}
		if !finish {
			events = append(events, map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}})
		}
		events = append(events, map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": text}})
		if finish {
			events = append(events, map[string]any{"type": "content_block_stop", "index": 0}, map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": "end_turn"}}, map[string]any{"type": "message_stop"})
		}
		return events
	case "gemini":
		candidate := map[string]any{"content": map[string]any{"role": "model", "parts": []any{map[string]any{"text": text}}}}
		if finish {
			candidate["finishReason"] = "STOP"
		}
		return []any{map[string]any{"candidates": []any{candidate}}}
	default:
		choice := map[string]any{"delta": map[string]any{"content": text}}
		if finish {
			choice["finish_reason"] = "stop"
		}
		return []any{map[string]any{"choices": []any{choice}}}
	}
}

func TestOpenCodeStreamsEveryProtocolBeforeCompletion(t *testing.T) {
	for _, format := range []string{"chat-completions", "responses", "anthropic", "gemini"} {
		t.Run(format, func(t *testing.T) {
			allowFinish := make(chan struct{})
			updates := make(chan string, 8)
			first, second := generatedResult[:35], generatedResult[35:]
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if json.NewDecoder(r.Body).Decode(&body) != nil {
					t.Error("invalid provider request")
				}
				if format == "gemini" {
					if !strings.HasSuffix(r.URL.Path, ":streamGenerateContent") || r.URL.Query().Get("alt") != "sse" {
						t.Error("Gemini did not use streaming endpoint")
					}
				} else if body["stream"] != true {
					t.Error("streaming was not enabled")
				}
				w.Header().Set("Content-Type", "text/event-stream")
				for _, event := range streamTextEvent(format, first, false) {
					writeSSE(w, event)
				}
				w.(http.Flusher).Flush()
				select {
				case <-allowFinish:
				case <-r.Context().Done():
					return
				}
				for _, event := range streamTextEvent(format, second, true) {
					writeSSE(w, event)
				}
			}))
			defer server.Close()
			client := &openCodeClient{model: "model", format: format, key: "key", endpoint: server.URL + "/model:generateContent", http: server.Client()}
			done := make(chan error, 1)
			go func() {
				turn, err := client.StreamStep(context.Background(), "SQL", []Message{{Role: "user", Text: "Users"}}, metadataTools(), func(delta string) { updates <- delta })
				if err == nil && turn.Text != generatedResult {
					err = fmt.Errorf("wrong streamed text: %q", turn.Text)
				}
				done <- err
			}()
			select {
			case update := <-updates:
				if update != first {
					t.Fatalf("first delta = %q", update)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("no delta arrived before completion")
			}
			select {
			case err := <-done:
				t.Fatalf("response completed before final chunk: %v", err)
			default:
			}
			close(allowFinish)
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("stream did not complete")
			}
		})
	}
}

func TestOpenCodeStreamsDiscardTruncatedOutput(t *testing.T) {
	for _, format := range []string{"chat-completions", "anthropic", "gemini"} {
		var stream strings.Builder
		for _, event := range streamTextEvent(format, "partial", false) {
			writeSSE(&stream, event)
		}
		turn, err := readOpenCodeStream(strings.NewReader(stream.String()), format, nil)
		if err == nil || turn.Text != "" || len(turn.Calls) > 0 {
			t.Fatalf("partial %s output escaped: %+v %v", format, turn, err)
		}
	}
}

func TestOpenCodeStreamPreservesToolCallsAndPrivateReplay(t *testing.T) {
	for _, test := range []struct{ format, stream, marker string }{
		{"chat-completions", `data: {"choices":[{"delta":{"reasoning_content":"private-reasoning","tool_calls":[{"index":0,"id":"call1","type":"function","function":{"name":"list_tables","arguments":"{"}}]}}]}

data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"}"}}]},"finish_reason":"tool_calls"}]}

`, "private-reasoning"},
		{"anthropic", `data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"private-reasoning"}}

data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"private-signature"}}

data: {"type":"content_block_stop","index":0}

data: {"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"call1","name":"list_tables","input":{}}}

data: {"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{}"}}

data: {"type":"content_block_stop","index":1}

data: {"type":"message_delta","delta":{"stop_reason":"tool_use"}}

data: {"type":"message_stop"}

`, "private-signature"},
		{"gemini", `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"private-reasoning","thought":true},{"functionCall":{"name":"list_tables","args":{}},"thoughtSignature":"private-signature"}]},"finishReason":"STOP"}]}

`, "private-signature"},
	} {
		var visible strings.Builder
		turn, err := readOpenCodeStream(strings.NewReader(test.stream), test.format, func(text string) { visible.WriteString(text) })
		if err != nil || len(turn.Calls) != 1 || turn.Calls[0].Name != "list_tables" || string(turn.Calls[0].Arguments) != "{}" || !strings.Contains(string(turn.Replay), test.marker) {
			t.Fatalf("%s streamed tools: %+v %v", test.format, turn, err)
		}
		if visible.Len() != 0 {
			t.Fatal("private reasoning or tool arguments were shown as assistant response")
		}
	}
}

func TestChatGPTStreamsVisibleTextBeforeCompletion(t *testing.T) {
	fixture := newChatGPTFixture(t)
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
		t.Fatal(err)
	}
	allowFinish := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSE(w, map[string]any{"type": "response.output_text.delta", "delta": generatedResult[:30]})
		w.(http.Flusher).Flush()
		select {
		case <-allowFinish:
		case <-r.Context().Done():
			return
		}
		writeSSE(w, map[string]any{"type": "response.output_text.delta", "delta": generatedResult[30:]})
		writeSSE(w, map[string]any{"type": "response.completed", "response": map[string]any{"status": "completed"}})
	}))
	defer server.Close()
	fixture.service.api = server.URL
	updates := make(chan string, 4)
	done := make(chan error, 1)
	go func() {
		turn, err := (&chatGPTClient{model: "account-first", service: fixture.service}).StreamStep(context.Background(), "SQL", []Message{{Role: "user", Text: "Users"}}, metadataTools(), func(delta string) { updates <- delta })
		if err == nil && turn.Text != generatedResult {
			err = fmt.Errorf("wrong final text %q", turn.Text)
		}
		done <- err
	}()
	select {
	case first := <-updates:
		if first != generatedResult[:30] {
			t.Fatalf("first chunk = %q", first)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ChatGPT did not stream before completion")
	}
	close(allowFinish)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
