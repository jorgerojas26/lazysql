package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jorgerojas26/lazysql/models"
)

func TestThinkingControlsMapToNativeRequestFields(t *testing.T) {
	for _, test := range []struct {
		name, format, level, mode string
		budget                    int
		want                      map[string]any
	}{
		{"responses", "responses", "high", "effort", 0, map[string]any{"reasoning": map[string]string{"effort": "high"}}},
		{"chat", "chat-completions", "low", "effort", 0, map[string]any{"reasoning_effort": "low"}},
		{"anthropic-effort", "anthropic", "medium", "effort", 0, map[string]any{"thinking": map[string]string{"type": "adaptive"}, "output_config": map[string]string{"effort": "medium"}}},
		{"anthropic-budget", "anthropic", "high", "budget", 16384, map[string]any{"thinking": map[string]any{"type": "enabled", "budget_tokens": 16384}, "max_tokens": 24576}},
		{"anthropic-off", "anthropic", "none", "toggle", 0, map[string]any{"thinking": map[string]string{"type": "disabled"}}},
		{"gemini-effort", "gemini", "low", "effort", 0, map[string]any{"generationConfig": map[string]any{"thinkingConfig": map[string]string{"thinkingLevel": "low"}}}},
		{"gemini-budget", "gemini", "medium", "budget", 4096, map[string]any{"generationConfig": map[string]any{"thinkingConfig": map[string]int{"thinkingBudget": 4096}, "maxOutputTokens": 12288}}},
		{"gemini-off", "gemini", "none", "toggle", 0, map[string]any{"generationConfig": map[string]any{"thinkingConfig": map[string]int{"thinkingBudget": 0}}}},
		{"gemini-on", "gemini", "on", "toggle", 1024, map[string]any{"generationConfig": map[string]any{"thinkingConfig": map[string]int{"thinkingBudget": -1}}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := map[string]any{}
			applyThinking(body, test.format, test.level, test.mode, test.budget)
			if !reflect.DeepEqual(body, test.want) {
				t.Fatalf("thinking payload: %#v, want %#v", body, test.want)
			}
		})
	}
	for _, format := range []string{"responses", "anthropic", "gemini", "chat-completions"} {
		body := toolRequestBody(format, "model", "instructions", nil, nil)
		before := toolRequestBody(format, "model", "instructions", nil, nil)
		applyThinking(body, format, "auto", "effort", 0)
		if !reflect.DeepEqual(body, before) {
			t.Fatal("Auto changed provider defaults")
		}
	}
}

func TestThinkingLevelsUseAdvertisedCapabilitiesOnly(t *testing.T) {
	model := Model{Format: "responses"}
	model.setThinking([]ReasoningOption{{Type: "effort", Values: []string{"low", "high"}}}, 32768)
	if !reflect.DeepEqual(model.ThinkingLevels, []ThinkingLevel{{ID: "low"}, {ID: "high"}}) {
		t.Fatalf("invented reasoning levels: %+v", model.ThinkingLevels)
	}
	config := models.AIConfig{}
	if err := model.ConfigureThinking(&config, "high"); err != nil || config.Thinking != "high" || config.ThinkingMode != "effort" {
		t.Fatalf("thinking selection: %+v %v", config, err)
	}
	if err := model.ConfigureThinking(&config, "none"); err == nil {
		t.Fatal("Off was allowed without capability metadata")
	}
	if err := model.ConfigureThinking(&config, "auto"); err != nil || config.Thinking != "" {
		t.Fatal("Auto did not restore provider default")
	}
	unknown := Model{Format: "chat-completions"}
	unknown.setThinking(nil, 0)
	if len(unknown.ThinkingLevels) != 0 {
		t.Fatal("guessed capabilities for an unknown model")
	}
	budget := Model{Format: "anthropic"}
	budget.setThinking([]ReasoningOption{{Type: "budget_tokens", Min: 1024, Max: 12000}, {Type: "toggle"}}, 16384)
	if len(budget.ThinkingLevels) != 4 || budget.ThinkingLevels[0].ID != "none" || budget.ThinkingLevels[3].Budget != 8192 {
		t.Fatalf("budget levels did not respect model limits: %+v", budget)
	}
}

func TestAvailableModelsOnlyLoadsAuthenticatedProvidersAndKeepsPartialResults(t *testing.T) {
	calls := 0
	catalog, err := availableModels(context.Background(), map[string]string{"chatgpt": "Logged in", "opencode-go": "Not logged in"}, func(_ context.Context, provider string) ([]Model, error) {
		calls++
		if provider != "chatgpt" {
			t.Fatal("queried an unauthenticated provider")
		}
		return []Model{{ID: "m", Name: "Model"}}, nil
	})
	if err != nil || calls != 1 || len(catalog) != 1 || catalog[0].Provider != "chatgpt" {
		t.Fatalf("authenticated catalog: %+v %v", catalog, err)
	}
	catalog, err = availableModels(context.Background(), map[string]string{"chatgpt": "Logged in", "opencode-go": "Logged in"}, func(_ context.Context, provider string) ([]Model, error) {
		if provider == "chatgpt" {
			return nil, errors.New("private backend error")
		}
		return []Model{{ID: "go-model", Name: "Go model"}}, nil
	})
	if len(catalog) != 1 || err == nil || !strings.Contains(err.Error(), "chatgpt") || strings.Contains(err.Error(), "private") {
		t.Fatalf("partial catalog: %+v %v", catalog, err)
	}
}

func TestChatGPTCatalogAdvertisesThinkingAndClientSendsSelection(t *testing.T) {
	fixture := newChatGPTFixture(t)
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-one" {
			t.Error("model catalog/request did not use the saved account")
		}
		if request.URL.Path == "/models" {
			io.WriteString(writer, `{"models":[{"slug":"account-model","display_name":"Account Model","visibility":"list","context_window":400000,"supported_reasoning_levels":[{"effort":"low"},{"effort":"high"}]},{"slug":"hidden","visibility":"hidden"}]}`)
			return
		}
		var body struct {
			Model     string `json:"model"`
			Reasoning struct {
				Effort string `json:"effort"`
			} `json:"reasoning"`
		}
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "account-model" || body.Reasoning.Effort != "high" {
			t.Errorf("ChatGPT ignored model/thinking selection: %+v", body)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		text, _ := json.Marshal(generatedResult)
		fmt.Fprintf(writer, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":%s}]}]}}\n\n", text)
	}))
	defer server.Close()
	fixture.service.api = server.URL
	catalog, err := fixture.service.models(context.Background())
	if err != nil || len(catalog) != 1 || catalog[0].ThinkingMode != "effort" || len(catalog[0].ThinkingLevels) != 2 || catalog[0].ContextWindow != 400000 {
		t.Fatalf("ChatGPT capabilities: %+v %v", catalog, err)
	}
	client := &chatGPTClient{model: "account-model", thinking: "high", service: fixture.service}
	if _, err := client.Step(context.Background(), "instructions", []Message{{Role: "user", Text: "Prompt"}}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Complete(context.Background(), "Prompt"); err != nil {
		t.Fatal(err)
	}
}

func TestOpenCodeRequestsKeepSelectedThinkingOnTheWire(t *testing.T) {
	for _, test := range []struct{ format, final string }{
		{"chat-completions", `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":%s}}]}`},
		{"responses", `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":%s}]}]}`},
		{"anthropic", `{"stop_reason":"end_turn","content":[{"type":"text","text":%s}]}`},
		{"gemini", `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"text":%s}]}}]}`},
	} {
		t.Run(test.format, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				switch test.format {
				case "responses":
					if body["reasoning"].(map[string]any)["effort"] != "high" {
						t.Error("missing Responses reasoning effort")
					}
				case "anthropic":
					if body["output_config"].(map[string]any)["effort"] != "high" || body["thinking"].(map[string]any)["type"] != "adaptive" {
						t.Error("missing Anthropic reasoning configuration")
					}
				case "gemini":
					if body["generationConfig"].(map[string]any)["thinkingConfig"].(map[string]any)["thinkingLevel"] != "high" {
						t.Error("missing Gemini thinking level")
					}
				default:
					if body["reasoning_effort"] != "high" {
						t.Error("missing compatible reasoning effort")
					}
				}
				writer.Header().Set("Content-Type", "application/json")
				text, _ := json.Marshal(generatedResult)
				fmt.Fprintf(writer, test.final, text)
			}))
			defer server.Close()
			client := &openCodeClient{model: "selected", format: test.format, thinking: "high", thinkingMode: "effort", key: "secret", endpoint: server.URL, http: server.Client()}
			if _, err := client.Step(context.Background(), "instructions", []Message{{Role: "user", Text: "Prompt"}}, nil); err != nil {
				t.Fatal(err)
			}
			if _, err := client.StreamStep(context.Background(), "instructions", []Message{{Role: "user", Text: "Prompt"}}, nil, func(string) {}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
