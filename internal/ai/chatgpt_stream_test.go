package ai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jorgerojas26/lazysql/models"
)

func TestChatGPTStreamRequiresCompletionAndDiscardsFailedPartialSQL(t *testing.T) {
	partial := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial SQL\"}\n\n"
	for _, ending := range []string{
		"",
		"data: [DONE]\n\n",
		"data: {\"type\":\"response.incomplete\"}\n\n",
		"data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"subscription_sharing_usage_limit_exceeded\"}}}\n\n",
		"data: {\"type\":\"error\",\"code\":\"subscription_sharing_usage_unavailable\"}\n\n",
	} {
		output, err := readResponsesStream(strings.NewReader(partial+ending), "ChatGPT", nil)
		if err == nil || output.Text != "" {
			t.Fatalf("partial SQL escaped after failure: %q %v", output.Text, err)
		}
	}
	_, err := readResponsesStream(strings.NewReader(partial+"data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"subscription_sharing_usage_limit_exceeded\"}}}\n\n"), "ChatGPT", nil)
	if !strings.Contains(err.Error(), ChatGPTUsageURL) {
		t.Fatal("usage limit must link to ChatGPT usage settings")
	}
}

func TestChatGPTStreamMultilineEventsFinalOutputAndBounds(t *testing.T) {
	for _, stream := range []string{
		": heartbeat\n\nevent: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\ndata: \"delta\":\"SELECT 1;\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n",
		"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"SELECT 1;\"}]}]}}\n\n",
	} {
		output, err := readResponsesStream(strings.NewReader(stream), "ChatGPT", nil)
		if err != nil || output.Text != "SELECT 1;" {
			t.Fatalf("stream = %q %v", output.Text, err)
		}
	}
	oversized := fmt.Sprintf("data: {\"type\":\"response.output_text.delta\",\"delta\":\"%s\"}\n\n", strings.Repeat("x", MaxResponseBytes+1))
	if output, err := readResponsesStream(strings.NewReader(oversized), "ChatGPT", nil); err == nil || output.Text != "" {
		t.Fatal("unbounded stream output accepted")
	}
}

func TestNativeChatGPTHTTPFailuresAreSanitizedAndNeverChangeBilling(t *testing.T) {
	fixture := newChatGPTFixture(t)
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		status int
		body   string
	}{
		{401, `{"detail":"private credential and SQL"}`},
		{403, `{"error":{"code":"subscription_sharing_user_not_eligible","message":"private prompt"}}`},
		{429, `{"error":{"code":"subscription_sharing_usage_limit_exceeded"}}`},
		{503, `{"error":{"code":"subscription_sharing_usage_unavailable"}}`},
	} {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			calls++
			writer.Header().Set("x-request-id", "request-fixture")
			writer.WriteHeader(test.status)
			io.WriteString(writer, test.body)
		}))
		fixture.service.api = server.URL
		_, err := (&chatGPTClient{model: "model", service: fixture.service}).Step(context.Background(), discoveryInstructions, []Message{{Role: "user", Text: "private prompt"}}, metadataTools())
		server.Close()
		var failure *chatGPTError
		if !errors.As(err, &failure) || failure.Status != test.status || failure.RequestID != "request-fixture" || calls != 1 || strings.Contains(err.Error(), "private") {
			t.Fatalf("request failure = %v calls=%d", err, calls)
		}
		store, storeErr := readChatGPTStore(fixture.service.path)
		if storeErr != nil || store.Accounts[0].RefreshToken == "" {
			t.Fatal("temporary admission error erased login")
		}
	}
}

func TestNativeChatGPTModelAndInferenceUseOnlySavedOAuthCredentials(t *testing.T) {
	fixture := newChatGPTFixture(t)
	t.Setenv("OPENAI_API_KEY", "separately-billed-secret")
	t.Setenv("CODEX_API_KEY", "other-secret")
	if _, err := fixture.service.login(context.Background(), "", fixture.present(t, false)); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer access-one" {
			t.Error("request did not use saved OAuth access token")
		}
		io.WriteString(writer, `{"models":[{"slug":"account-model","display_name":"Account Model","visibility":"list"}]}`)
	}))
	defer server.Close()
	fixture.service.api = server.URL
	models, err := fixture.service.models(context.Background())
	if err != nil || len(models) != 1 {
		t.Fatalf("model discovery=%v %v", models, err)
	}
}

func TestChatGPTCredentialsAreNotCreatedWhenAIIsDisabled(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", directory)
	if _, err := New(models.AIConfig{Provider: "chatgpt", Enabled: false}); err == nil {
		t.Fatal("disabled ChatGPT was initialized")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatal("disabled AI created credential state")
	}
}
