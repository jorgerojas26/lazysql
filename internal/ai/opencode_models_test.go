package ai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGoModelCatalogIncludesEveryLiveModelAndItsProtocol(t *testing.T) {
	models := `{"data":[{"id":"chat"},{"id":"anthropic"},{"id":"responses"},{"id":"gemini"},{"id":"new-model"},{"id":"chat"}]}`
	metadata := `{"opencode-go":{"npm":"@ai-sdk/openai-compatible","models":{"chat":{"name":"Chat model"},"anthropic":{"name":"Anthropic model","provider":{"npm":"@ai-sdk/anthropic"}},"responses":{"name":"Responses model","provider":{"npm":"@ai-sdk/openai"}},"gemini":{"name":"Gemini model","provider":{"npm":"@ai-sdk/google"}},"retired":{"name":"Not available"}}}}`
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		if request.Header.Get("Authorization") != "" || request.Header.Get("x-api-key") != "" || request.Header.Get("x-goog-api-key") != "" {
			t.Error("public model catalog received credentials")
		}
		if !strings.HasPrefix(request.Header.Get("User-Agent"), "lazysql/") {
			t.Error("missing client identification")
		}
		if request.URL.Path == "/models" {
			io.WriteString(writer, models)
		} else {
			io.WriteString(writer, metadata)
		}
	}))
	defer server.Close()
	result, err := fetchOpenCodeModels(context.Background(), server.Client(), server.URL+"/models", server.URL+"/metadata")
	if err != nil || len(result) != 5 || calls != 2 {
		t.Fatalf("models=%+v err=%v calls=%d", result, err, calls)
	}
	formats := map[string]string{}
	for _, model := range result {
		formats[model.ID] = model.Format
	}
	for id, format := range map[string]string{"chat": "chat-completions", "new-model": "chat-completions", "anthropic": "anthropic", "responses": "responses", "gemini": "gemini"} {
		if formats[id] != format {
			t.Fatalf("%s format = %q", id, formats[id])
		}
	}
	if _, ok := formats["retired"]; ok {
		t.Fatal("retired model exposed as available")
	}
	if result[0].Name != "Anthropic model" {
		t.Fatal("model names are not sorted")
	}
}

func TestGoModelCatalogFailsClearlyInsteadOfUsingAnIncompleteList(t *testing.T) {
	for _, test := range []struct{ models, metadata string }{
		{`{"data":[]}`, `{}`},
		{`not json`, `{}`},
		{`{"data":[{"id":"chat"}]}`, `{}`},
		{`{"data":[{"id":"chat"}]}`, `{"opencode-go":{"npm":"unsupported","models":{}}}`},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request.URL.Path == "/models" {
				io.WriteString(writer, test.models)
			} else {
				io.WriteString(writer, test.metadata)
			}
		}))
		_, err := fetchOpenCodeModels(context.Background(), server.Client(), server.URL+"/models", server.URL+"/metadata")
		server.Close()
		if err == nil {
			t.Fatal("invalid model catalog accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fetchOpenCodeModels(ctx, http.DefaultClient, "https://unused.invalid/models", "https://unused.invalid/meta"); err == nil {
		t.Fatal("catalog cancellation ignored")
	}
}
