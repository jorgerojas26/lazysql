package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGoProtocolsUseCorrectRequestAndResponseFormats(t *testing.T) {
	for _, test := range []struct{ format, header, response string }{
		{"chat-completions", "Authorization", `{"choices":[{"message":{"content":"SELECT 1;"},"finish_reason":"stop"}]}`},
		{"responses", "Authorization", `{"status":"completed","output":[{"type":"reasoning","content":[]},{"type":"message","content":[{"type":"output_text","text":"SELECT 1;"}]}]}`},
		{"anthropic", "x-api-key", `{"stop_reason":"end_turn","content":[{"type":"thinking","thinking":"private reasoning"},{"type":"text","text":"SELECT 1;"}]}`},
		{"gemini", "x-goog-api-key", `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"text":"private reasoning","thought":true},{"text":"SELECT 1;"}]}}]}`},
	} {
		t.Run(test.format, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				expected := "subscription-key"
				if test.header == "Authorization" {
					expected = "Bearer " + expected
				}
				if request.Header.Get(test.header) != expected || request.Header.Get("x-opencode-session") != "session" {
					t.Error("wrong protocol authentication or missing session")
				}
				if test.format == "anthropic" && request.Header.Get("anthropic-version") == "" {
					t.Error("missing Anthropic API version")
				}
				var body map[string]any
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				data, _ := json.Marshal(body)
				if !strings.Contains(string(data), "user request") {
					t.Error("missing request context")
				}
				if body["tools"] == nil {
					t.Error("missing metadata tools")
				}
				switch test.format {
				case "responses":
					if body["store"] != false || body["input"] == nil {
						t.Error("invalid Responses request")
					}
				case "anthropic":
					if body["system"] != discoveryInstructions || body["messages"] == nil {
						t.Error("invalid Messages request")
					}
				case "gemini":
					if body["systemInstruction"] == nil || body["contents"] == nil || body["generationConfig"] == nil {
						t.Error("invalid Gemini request")
					}
				}
				io.WriteString(writer, test.response)
			}))
			defer server.Close()
			client := &openCodeClient{format: test.format, model: "model", key: "subscription-key", session: "session", endpoint: server.URL, http: server.Client()}
			output, err := client.Step(context.Background(), discoveryInstructions, []Message{{Role: "user", Text: "user request"}}, metadataTools())
			if err != nil || output.Text != "SELECT 1;" {
				t.Fatalf("response=%q err=%v", output.Text, err)
			}
		})
	}
}

func TestGoProtocolsRejectPartialResponsesAndUnsupportedFormats(t *testing.T) {
	for _, test := range []struct{ format, response string }{
		{"responses", `{"status":"incomplete","output":[]}`},
		{"anthropic", `{"stop_reason":"max_tokens","content":[]}`},
		{"gemini", `{"candidates":[{"finishReason":"MAX_TOKENS"}]}`},
		{"gemini", `{"candidates":[]}`},
		{"responses", `{"status":"completed","output":[]}`},
		{"anthropic", `{"stop_reason":"tool_use","content":[]}`},
	} {
		if _, err := decodeToolTurn(test.format, []byte(test.response)); err == nil {
			t.Fatalf("accepted partial %s response", test.format)
		}
	}
	if _, err := openCodeEndpointFor("model", "unknown"); err == nil {
		t.Fatal("unknown API format accepted")
	}
	for _, format := range []string{"chat-completions", "responses", "anthropic", "gemini"} {
		address, err := openCodeEndpointFor("model", format)
		if err != nil || !strings.HasPrefix(address, "https://opencode.ai/inference/go/") {
			t.Fatal("API format can escape the Go subscription endpoint")
		}
	}
}
