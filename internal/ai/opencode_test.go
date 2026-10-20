package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jorgerojas26/lazysql/models"
)

func TestOpenCodeGoRequestAndSession(t *testing.T) {
	var session string
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer subscription-key" || !strings.HasPrefix(request.Header.Get("User-Agent"), "lazysql/") {
			t.Error("missing provider identity or subscription authentication")
		}
		id := request.Header.Get("x-opencode-session")
		if id == "" || (session != "" && session != id) {
			t.Error("session ID must be stable across inference and generation")
		}
		session = id
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["model"] != "test-model" || body["stream"] != false {
			t.Errorf("unexpected request: %v", body)
		}
		if _, ok := body["tools"]; ok {
			t.Error("SQL generation must not expose tools")
		}
		io.WriteString(writer, `{"choices":[{"message":{"content":"[\"orders\"]"},"finish_reason":"stop"}]}`)
	}))
	defer server.Close()
	t.Setenv("OPENCODE_API_KEY", "subscription-key")
	client, err := newOpenCode(models.AIConfig{Model: "test-model"})
	if err != nil {
		t.Fatal(err)
	}
	client.endpoint = server.URL
	for i := 0; i < 2; i++ {
		if output, err := client.Complete(context.Background(), "generate SQL"); err != nil || output != `["orders"]` {
			t.Fatalf("complete = %q %v", output, err)
		}
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestOpenCodeFailureDoesNotLeakResponseOrRetryBilling(t *testing.T) {
	for _, code := range []int{401, 403, 402, 429, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				calls++
				writer.WriteHeader(code)
				io.WriteString(writer, "private schema and secret API key")
			}))
			defer server.Close()
			client := &openCodeClient{endpoint: server.URL, http: server.Client()}
			_, err := client.Complete(context.Background(), "request")
			if err == nil || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "secret") || calls != 1 {
				t.Fatalf("unsafe failure: calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestOpenCodeCancellationAndTruncation(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/truncated" {
			io.WriteString(writer, `{"choices":[{"message":{"content":"incomplete"},"finish_reason":"length"}]}`)
			return
		}
		select {
		case <-request.Context().Done():
		case <-release:
		}
	}))
	defer server.Close()
	defer close(release)
	client := &openCodeClient{endpoint: server.URL, http: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := client.Complete(ctx, "request"); err == nil {
		t.Fatal("canceled request should fail")
	}
	client.endpoint = server.URL + "/truncated"
	if _, err := client.Complete(context.Background(), "request"); err == nil {
		t.Fatal("truncated response should fail")
	}
}

func TestOpenCodeConfiguration(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("OPENCODE_API_KEY", "")
	if _, err := newOpenCode(models.AIConfig{Model: "model"}); err == nil {
		t.Fatal("missing key accepted")
	}
	if _, err := newOpenCode(models.AIConfig{}); err == nil {
		t.Fatal("missing model accepted")
	}
	t.Setenv("CUSTOM_GO_KEY", "key")
	client, err := newOpenCode(models.AIConfig{Model: "model", APIKeyEnv: "CUSTOM_GO_KEY"})
	if err != nil || client.key != "key" {
		t.Fatalf("custom environment key = %v %v", client, err)
	}
	if client.endpoint != openCodeEndpoint {
		t.Fatal("must use subscription endpoint")
	}
}
