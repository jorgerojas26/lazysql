package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jorgerojas26/lazysql/models"
)

const openCodeEndpoint = "https://opencode.ai/inference/go/openai/v1/chat/completions"

type openCodeClient struct {
	thinking       string
	thinkingMode   string
	thinkingBudget int
	model          string
	format         string
	key            string
	session        string
	endpoint       string
	http           *http.Client
}

func newOpenCode(config models.AIConfig) (*openCodeClient, error) {
	if config.Model == "" {
		return nil, errors.New("select an OpenCode Go model with lazysql ai login")
	}
	endpoint, err := openCodeEndpointFor(config.Model, config.APIFormat)
	if err != nil {
		return nil, err
	}
	env := config.APIKeyEnv
	if env == "" {
		env = "OPENCODE_API_KEY"
	}
	key := os.Getenv(env)
	if key == "" && (config.APIKeyEnv == "" || config.APIKeyEnv == "OPENCODE_API_KEY") {
		var err error
		key, err = LoadOpenCodeKey()
		if err != nil {
			return nil, err
		}
	}
	if key == "" {
		return nil, fmt.Errorf("run lazysql ai login or set %s to your OpenCode Go subscription API key", env)
	}
	return &openCodeClient{thinking: config.Thinking, thinkingMode: config.ThinkingMode, thinkingBudget: config.ThinkingBudget, model: config.Model, format: config.APIFormat, key: key, session: uuid.NewString(), endpoint: endpoint, http: &http.Client{
		Timeout: 90 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errors.New("AI endpoint redirects are not allowed")
		},
	}}, nil
}

func (client *openCodeClient) Complete(ctx context.Context, prompt string) (string, error) {
	data, err := client.send(ctx, client.requestBody(prompt))
	if err != nil {
		return "", err
	}
	return decodeOpenCodeResponse(client.format, data)
}

func (client *openCodeClient) Step(ctx context.Context, instructions string, messages []Message, tools []Tool) (Turn, error) {
	data, err := client.send(ctx, toolRequestBody(client.format, client.model, instructions, messages, tools))
	if err != nil {
		return Turn{}, err
	}
	return decodeToolTurn(client.format, data)
}

func (client *openCodeClient) StreamStep(ctx context.Context, instructions string, messages []Message, tools []Tool, delta func(string)) (Turn, error) {
	payload := toolRequestBody(client.format, client.model, instructions, messages, tools)
	endpoint := client.endpoint
	if client.format == "gemini" {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return Turn{}, errors.New("invalid model endpoint")
		}
		parsed.Path = strings.Replace(parsed.Path, ":generateContent", ":streamGenerateContent", 1)
		parsed.RawPath = ""
		query := parsed.Query()
		query.Set("alt", "sse")
		parsed.RawQuery = query.Encode()
		endpoint = parsed.String()
	} else {
		payload["stream"] = true
	}
	response, err := client.post(ctx, payload, endpoint)
	if err != nil {
		return Turn{}, err
	}
	defer response.Body.Close()
	var turn Turn
	if strings.Contains(response.Header.Get("Content-Type"), "application/json") {
		data, readErr := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes*2+1))
		if readErr != nil {
			return Turn{}, errors.New("could not read OpenCode Go response")
		}
		turn, err = decodeToolTurn(client.format, data)
		if err == nil && delta != nil {
			delta(turn.Text)
		}
	} else if client.format == "responses" {
		turn, err = readResponsesStream(response.Body, "OpenCode Go", delta)
	} else {
		turn, err = readOpenCodeStream(response.Body, client.format, delta)
	}
	if ctx.Err() != nil {
		return Turn{}, ctx.Err()
	}
	return turn, err
}

func (client *openCodeClient) send(ctx context.Context, payload map[string]any) ([]byte, error) {
	response, err := client.post(ctx, payload, client.endpoint)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, MaxResponseBytes*2+1))
	if err != nil {
		return nil, errors.New("could not read OpenCode Go response")
	}
	if len(data) > MaxResponseBytes*2 {
		return nil, errors.New("OpenCode Go response is too large")
	}
	return data, nil
}

func (client *openCodeClient) post(ctx context.Context, payload map[string]any, endpoint string) (*http.Response, error) {
	applyThinking(payload, client.format, client.thinking, client.thinkingMode, client.thinkingBudget)
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	recordRequest(ctx, body)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if payload["stream"] == true || client.format == "gemini" {
		request.Header.Set("Accept", "text/event-stream")
	}
	switch client.format {
	case "anthropic":
		request.Header.Set("x-api-key", client.key)
		request.Header.Set("anthropic-version", "2023-06-01")
	case "gemini":
		request.Header.Set("x-goog-api-key", client.key)
	default:
		request.Header.Set("Authorization", "Bearer "+client.key)
	}
	request.Header.Set("User-Agent", "lazysql/ai-sql")
	request.Header.Set("x-opencode-session", client.session)
	response, err := client.http.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("could not reach OpenCode Go; check your connection and retry")
	}
	if response.StatusCode == http.StatusOK {
		return response, nil
	}
	response.Body.Close()
	switch response.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, errors.New("OpenCode Go authentication failed; check your subscription API key")
	case http.StatusPaymentRequired, http.StatusTooManyRequests:
		return nil, errors.New("OpenCode Go subscription limit reached; check your plan and retry later (no billing fallback)")
	default:
		return nil, fmt.Errorf("OpenCode Go request failed (HTTP %d); check model availability and retry", response.StatusCode)
	}
}
