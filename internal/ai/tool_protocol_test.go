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
)

func TestNativeToolProtocolsRoundTripDiscoveryAndPreserveReplay(t *testing.T) {
	for _, test := range []struct{ format, reply, final, marker string }{
		{"chat-completions", `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":null,"reasoning_content":"preserve-reasoning","tool_calls":[{"id":"call1","type":"function","function":{"name":"list_tables","arguments":"{}"}}]}}]}`, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":%s}}]}`, "preserve-reasoning"},
		{"responses", `{"status":"completed","output":[{"type":"reasoning","id":"r1","encrypted_content":"preserve-encrypted","summary":[]},{"type":"function_call","id":"fc1","call_id":"call1","name":"list_tables","arguments":"{}"}]}`, `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":%s}]}]}`, "preserve-encrypted"},
		{"anthropic", `{"stop_reason":"tool_use","content":[{"type":"thinking","thinking":"private","signature":"preserve-signature"},{"type":"tool_use","id":"call1","name":"list_tables","input":{}}]}`, `{"stop_reason":"end_turn","content":[{"type":"text","text":%s}]}`, "preserve-signature"},
		{"gemini", `{"candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"functionCall":{"name":"list_tables","args":{}},"thoughtSignature":"preserve-thought"}]}}]}`, `{"candidates":[{"finishReason":"STOP","content":{"role":"model","parts":[{"text":%s}]}}]}`, "preserve-thought"},
	} {
		t.Run(test.format, func(t *testing.T) {
			calls := 0
			bodies := make(chan string, 2)
			var captured []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				data, _ := io.ReadAll(r.Body)
				bodies <- string(data)
				var body map[string]any
				if json.Unmarshal(data, &body) != nil || body["tools"] == nil {
					t.Error("missing native tool definitions")
				}
				if !strings.Contains(string(data), "describe_tables") || strings.Contains(string(data), `"name":"execute_sql"`) {
					t.Error("wrong tool allowlist")
				}
				if calls == 1 {
					io.WriteString(w, test.reply)
					return
				}
				if !strings.Contains(string(data), test.marker) || !strings.Contains(string(data), "public.orders") {
					t.Errorf("tool replay or result missing: %s", data)
				}
				text, _ := json.Marshal(generatedResult)
				fmt.Fprintf(w, test.final, string(text))
			}))
			defer server.Close()
			client := &openCodeClient{format: test.format, model: "model", key: "key", endpoint: server.URL, http: server.Client()}
			result, err := DiscoverSQLStream(context.Background(), client, Request{Prompt: "Orders"}, func(context.Context, string, json.RawMessage) (any, error) {
				return map[string]any{"tables": []string{"public.orders"}}, nil
			}, nil, DiscoveryOptions{ObserveRequest: func(round int, body []byte) {
				if round != len(captured)+1 {
					t.Errorf("wrong captured request round: %d", round)
				}
				captured = append(captured, string(body))
			}})
			if err != nil || result.SQL == "" || calls != 2 {
				t.Fatalf("native discovery: %+v %v (%d calls)", result, err, calls)
			}
			if len(captured) != 2 {
				t.Fatalf("captured %d request bodies", len(captured))
			}
			for _, body := range captured {
				if body != <-bodies {
					t.Fatal("captured request differs from provider payload")
				}
				if strings.Contains(body, "Bearer") || strings.Contains(body, "x-api-key") {
					t.Fatal("request inspection includes authentication headers")
				}
			}
		})
	}
}

func TestNativeToolProtocolsRejectIncompleteResponses(t *testing.T) {
	for _, test := range []struct{ format, reply string }{
		{"chat-completions", `{"choices":[{"finish_reason":"length","message":{"tool_calls":[]}}]}`},
		{"chat-completions", `{"choices":[{"finish_reason":"tool_calls","message":{"content":"partial"}}]}`},
		{"responses", `{"status":"incomplete","output":[{"type":"function_call","call_id":"1","name":"list_tables","arguments":"{}"}]}`},
		{"anthropic", `{"stop_reason":"max_tokens","content":[{"type":"tool_use","id":"1","name":"list_tables","input":{}}]}`},
		{"gemini", `{"candidates":[{"finishReason":"MAX_TOKENS","content":{"parts":[{"functionCall":{"name":"list_tables","args":{}}}]}}]}`},
	} {
		if _, err := decodeToolTurn(test.format, []byte(test.reply)); err == nil {
			t.Fatalf("accepted incomplete %s", test.format)
		}
	}
}

func TestChatGPTToolStreamRequiresCompletedResponseAndPreservesCalls(t *testing.T) {
	reply := `{"status":"completed","output":[{"type":"reasoning","encrypted_content":"encrypted","summary":[]},{"type":"function_call","call_id":"call1","name":"describe_tables","arguments":"{\"names\":[\"public.orders\"]}"}]}`
	stream := "data: {\"type\":\"response.completed\",\"response\":" + reply + "}\n\n"
	turn, err := readChatGPTTurn(strings.NewReader(stream))
	if err != nil || len(turn.Calls) != 1 || turn.Calls[0].Name != "describe_tables" || !strings.Contains(string(turn.Replay), "encrypted") {
		t.Fatalf("stream turn: %+v %v", turn, err)
	}
	for _, ending := range []string{"", "data: [DONE]\n\n", "data: {\"type\":\"response.incomplete\"}\n\n"} {
		partial := "data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"name\":\"list_tables\"}}\n\n"
		if _, err := readChatGPTTurn(strings.NewReader(partial + ending)); err == nil {
			t.Fatal("partial tool call accepted")
		}
	}
}

func TestChatGPTNativeDiscoveryUsesStreamingToolProtocolAndSavedCredentials(t *testing.T) {
	fixture := newChatGPTFixture(t)
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
		t.Fatal(err)
	}
	calls := 0
	bodies := make(chan string, 2)
	var captured []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/responses" || r.Header.Get("Authorization") != "Bearer access-one" {
			t.Error("wrong native endpoint or OAuth credentials")
		}
		data, _ := io.ReadAll(r.Body)
		bodies <- string(data)
		var body map[string]any
		if json.Unmarshal(data, &body) != nil || body["stream"] != true || body["store"] != false || body["tools"] == nil || body["max_output_tokens"] != nil {
			t.Error("invalid native streaming tool request")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if calls == 1 {
			io.WriteString(w, `data: {"type":"response.output_item.done","output_index":0,"item":{"type":"reasoning","encrypted_content":"keep-me","summary":[]}}

data: {"type":"response.output_item.done","output_index":1,"item":{"type":"function_call","call_id":"call1","name":"list_tables","arguments":"{}"}}

data: {"type":"response.completed","response":{"status":"completed"}}

`)
			return
		}
		if !strings.Contains(string(data), "keep-me") || !strings.Contains(string(data), "function_call_output") || !strings.Contains(string(data), "public.orders") {
			t.Error("native follow-up lost reasoning or tool result")
		}
		text, _ := json.Marshal(generatedResult)
		fmt.Fprintf(w, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":%s}]}]}}\n\n", text)
	}))
	defer server.Close()
	fixture.service.api = server.URL
	client := &chatGPTClient{model: "account-first", service: fixture.service}
	result, err := DiscoverSQLStream(context.Background(), client, Request{Prompt: "Orders"}, func(context.Context, string, json.RawMessage) (any, error) {
		return map[string]any{"tables": []string{"public.orders"}}, nil
	}, nil, DiscoveryOptions{ObserveRequest: func(round int, body []byte) {
		if round != len(captured)+1 {
			t.Errorf("wrong ChatGPT request round: %d", round)
		}
		captured = append(captured, string(body))
	}})
	if err != nil || result.SQL == "" || calls != 2 {
		t.Fatalf("ChatGPT native discovery: %+v %v (%d calls)", result, err, calls)
	}
	if len(captured) != 2 {
		t.Fatalf("captured %d ChatGPT requests", len(captured))
	}
	for _, body := range captured {
		if body != <-bodies || strings.Contains(body, "access-one") {
			t.Fatal("ChatGPT request inspection differs from wire payload or includes credentials")
		}
	}
}

func TestChatGPTToolItemsSurviveSparseCompletion(t *testing.T) {
	stream := `data: {"type":"response.output_item.added","output_index":0,"item":{"id":"reason1","type":"reasoning","summary":[]}}

data: {"type":"response.output_item.done","output_index":0,"item":{"id":"reason1","type":"reasoning","encrypted_content":"keep-encrypted","summary":[]}}

data: {"type":"response.output_item.added","output_index":1,"item":{"id":"fc1","type":"function_call","call_id":"call1","name":"list_tables","arguments":""}}

data: {"type":"response.function_call_arguments.delta","output_index":1,"item_id":"fc1","delta":"{\"search\":"}

data: {"type":"response.function_call_arguments.delta","output_index":1,"item_id":"fc1","delta":"\"users\"}"}

data: {"type":"response.function_call_arguments.done","output_index":1,"item_id":"fc1","arguments":"{\"search\":\"users\"}"}

data: {"type":"response.output_item.done","output_index":1,"item":{"id":"fc1","type":"function_call","call_id":"call1","name":"list_tables","arguments":"{\"search\":\"users\"}","status":"completed"}}

data: {"type":"response.completed","response":{"status":"completed"}}

`
	turn, err := readChatGPTTurn(strings.NewReader(stream))
	if err != nil || len(turn.Calls) != 1 || turn.Calls[0].Name != "list_tables" || string(turn.Calls[0].Arguments) != `{"search":"users"}` || !strings.Contains(string(turn.Replay), "keep-encrypted") {
		t.Fatalf("sparse tool completion: %+v %v", turn, err)
	}
}

func TestChatGPTSparseToolCompletionVariants(t *testing.T) {
	for _, test := range []struct {
		name, stream string
		wantErr      bool
	}{
		{"arguments done without output item done", `data: {"type":"response.output_item.added","output_index":2,"item":{"id":"fc1","type":"function_call","status":"in_progress","call_id":"call1","name":"list_tables","arguments":""}}

data: {"type":"response.function_call_arguments.done","item_id":"fc1","arguments":"{}"}

data: {"type":"response.completed","response":{"status":"completed"}}

`, false},
		{"final output without redundant status", `data: {"type":"response.completed","response":{"output":[{"type":"function_call","call_id":"call1","name":"list_tables","arguments":"{}"}]}}

`, false},
		{"tool arguments never completed", `data: {"type":"response.output_item.added","output_index":0,"item":{"id":"fc1","type":"function_call","call_id":"call1","name":"list_tables","arguments":""}}

data: {"type":"response.function_call_arguments.delta","item_id":"fc1","output_index":0,"delta":"{}"}

data: {"type":"response.completed","response":{"status":"completed"}}

`, true},
		{"completed tool followed by failed response", `data: {"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","call_id":"call1","name":"list_tables","arguments":"{}"}}

data: {"type":"response.failed","response":{"error":{"code":"server_error"}}}

`, true},
		{"completed tool followed by incomplete response", `data: {"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","call_id":"call1","name":"list_tables","arguments":"{}"}}

data: {"type":"response.completed","response":{"status":"incomplete"}}

`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			turn, err := readChatGPTTurn(strings.NewReader(test.stream))
			if test.wantErr {
				if err == nil || len(turn.Calls) != 0 {
					t.Fatal("incomplete tool call escaped")
				}
				return
			}
			if err != nil || len(turn.Calls) != 1 || turn.Calls[0].ID != "call1" || string(turn.Calls[0].Arguments) != "{}" {
				t.Fatalf("tool completion: %+v %v", turn, err)
			}
			if strings.Contains(string(turn.Replay), "in_progress") {
				t.Fatal("unfinished tool state was replayed")
			}
		})
	}
}

func TestChatGPTSparseTextCompletionAndParallelToolOrdering(t *testing.T) {
	for _, stream := range []string{
		"data: {\"type\":\"response.output_text.done\",\"text\":\"SELECT 1;\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n",
		"data: {\"type\":\"response.output_item.done\",\"output_index\":1,\"item\":{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"SELECT 1;\"}]}}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n",
	} {
		output, err := readChatGPTStream(strings.NewReader(stream))
		if err != nil || output != "SELECT 1;" {
			t.Fatalf("sparse SQL text: %q %v", output, err)
		}
	}
	stream := `data: {"type":"response.output_item.done","output_index":2,"item":{"type":"function_call","call_id":"second","name":"describe_tables","arguments":"{\"names\":[\"public.users\"]}"}}

data: {"type":"response.output_item.done","output_index":1,"item":{"type":"function_call","call_id":"first","name":"get_relationships","arguments":"{\"names\":[\"public.users\"]}"}}

data: {"type":"response.completed","response":{"status":"completed"}}

`
	turn, err := readChatGPTTurn(strings.NewReader(stream))
	if err != nil || len(turn.Calls) != 2 || turn.Calls[0].ID != "first" || turn.Calls[1].ID != "second" {
		t.Fatalf("parallel call order: %+v %v", turn, err)
	}
}
