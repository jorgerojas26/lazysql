package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strings"
)

type chatGPTClient struct {
	model   string
	service *chatGPTService
}

func (service *chatGPTService) request(ctx context.Context, method, path string, body any) (*http.Response, error) {
	token, err := service.activeToken(ctx)
	if err != nil {
		return nil, err
	}
	var input io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if path == "/responses" {
			recordWireBytes(ctx, len(data))
		}
		if err != nil {
			return nil, errors.New("cannot prepare ChatGPT request")
		}
		input = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, service.api+path, input)
	if err != nil {
		return nil, errors.New("cannot prepare ChatGPT request")
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("User-Agent", "lazysql/ai-sql")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "text/event-stream")
	}
	response, err := service.http.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("cannot reach ChatGPT; check your connection and retry")
	}
	if response.StatusCode != http.StatusOK {
		defer response.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(response.Body, 64*1024))
		failure := oauthRequestError(response.StatusCode, data).(*chatGPTError)
		id := response.Header.Get("x-request-id")
		if len(id) < 128 {
			failure.RequestID = id
		}
		return nil, failure
	}
	return response, nil
}

func (service *chatGPTService) models(ctx context.Context) ([]Model, error) {
	response, err := service.request(ctx, http.MethodGet, "/models", nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024+1))
	if err != nil || len(data) > 1024*1024 {
		return nil, errors.New("invalid ChatGPT model catalog")
	}
	var catalog struct {
		Models []struct {
			Slug          string `json:"slug"`
			ContextWindow int    `json:"context_window"`
			Name          string `json:"display_name"`
			Visibility    string `json:"visibility"`
		} `json:"models"`
	}
	if json.Unmarshal(data, &catalog) != nil {
		return nil, errors.New("invalid ChatGPT model catalog")
	}
	seen := map[string]bool{}
	var models []Model
	for _, model := range catalog.Models {
		if model.Visibility != "list" || model.Slug == "" || seen[model.Slug] {
			continue
		}
		seen[model.Slug] = true
		name := model.Name
		if name == "" {
			name = model.Slug
		}
		models = append(models, Model{ID: model.Slug, Name: name, ContextWindow: model.ContextWindow})
	}
	if len(models) == 0 {
		return nil, errors.New("no ChatGPT models available for this account; check plan access")
	}
	return models, nil
}

func ChatGPTModels(ctx context.Context) ([]Model, error) {
	service, err := newChatGPTService()
	if err != nil {
		return nil, err
	}
	return service.models(ctx)
}

func (client *chatGPTClient) Complete(ctx context.Context, prompt string) (string, error) {
	if client.model == "" {
		return "", errors.New("select a ChatGPT model with lazysql ai login")
	}
	response, err := client.service.request(ctx, http.MethodPost, "/responses", map[string]any{
		"model": client.model, "instructions": sqlAssistant,
		"input": []map[string]string{{"role": "user", "content": prompt}},
		"store": false, "stream": true,
	})
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	output, err := readChatGPTStream(response.Body)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	return output, err
}

func (client *chatGPTClient) Step(ctx context.Context, instructions string, messages []Message, tools []Tool) (Turn, error) {
	return client.StreamStep(ctx, instructions, messages, tools, nil)
}

func (client *chatGPTClient) StreamStep(ctx context.Context, instructions string, messages []Message, tools []Tool, delta func(string)) (Turn, error) {
	if client.model == "" {
		return Turn{}, errors.New("select a ChatGPT model with lazysql ai login")
	}
	body := toolRequestBody("responses", client.model, instructions, messages, tools)
	body["stream"] = true
	delete(body, "max_output_tokens")
	response, err := client.service.request(ctx, http.MethodPost, "/responses", body)
	if err != nil {
		return Turn{}, err
	}
	defer response.Body.Close()
	turn, err := readResponsesStream(response.Body, "ChatGPT", delta)
	if ctx.Err() != nil {
		return Turn{}, ctx.Err()
	}
	return turn, err
}

func readChatGPTStream(reader io.Reader) (string, error) {
	turn, err := readChatGPTTurn(reader)
	if err != nil {
		return "", err
	}
	if len(turn.Calls) > 0 {
		return "", errors.New("unexpected ChatGPT tool call")
	}
	return turn.Text, nil
}

func readChatGPTTurn(reader io.Reader) (Turn, error) {
	return readResponsesStream(reader, "ChatGPT", nil)
}

func readResponsesStream(reader io.Reader, provider string, delta func(string)) (Turn, error) {
	emit := func(text string) {
		if delta != nil && text != "" {
			delta(text)
		}
	}
	scanner := bufio.NewScanner(io.LimitReader(reader, 4*1024*1024))
	scanner.Buffer(make([]byte, 4096), 512*1024)
	var eventData strings.Builder
	var output strings.Builder
	var turn Turn
	items := map[int]json.RawMessage{}
	completedItems := map[int]bool{}
	itemIndexes := map[string]int{}
	itemBytes := 0
	storeItem := func(index int, raw json.RawMessage) error {
		size := itemBytes - len(items[index]) + len(raw)
		if index < 0 || index > 63 || size > MaxResponseBytes*2 {
			return errors.New(provider + " tool response is too large")
		}
		itemBytes = size
		items[index] = raw
		return nil
	}
	process := func() (bool, error) {
		raw := strings.TrimSpace(eventData.String())
		eventData.Reset()
		if raw == "" {
			return false, nil
		}
		if raw == "[DONE]" {
			return false, errors.New(provider + " stream ended without completion; retry")
		}
		var event struct {
			Type        string          `json:"type"`
			Delta       string          `json:"delta"`
			Code        string          `json:"code"`
			Item        json.RawMessage `json:"item"`
			ItemID      string          `json:"item_id"`
			OutputIndex int             `json:"output_index"`
			Arguments   *string         `json:"arguments"`
			Text        string          `json:"text"`
			Error       struct {
				Code string `json:"code"`
			} `json:"error"`
			Response struct {
				Status string `json:"status"`
				Error  struct {
					Code string `json:"code"`
				} `json:"error"`
				Output []struct {
					Type    string `json:"type"`
					Content []struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"content"`
				} `json:"output"`
			} `json:"response"`
		}
		if json.Unmarshal([]byte(raw), &event) != nil {
			return false, errors.New("invalid " + provider + " stream event")
		}
		switch event.Type {
		case "response.output_item.added", "response.output_item.done":
			var item struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			}
			if json.Unmarshal(event.Item, &item) != nil || item.Type == "" {
				return false, errors.New("invalid " + provider + " output item")
			}
			if err := storeItem(event.OutputIndex, event.Item); err != nil {
				return false, err
			}
			if item.ID != "" {
				itemIndexes[item.ID] = event.OutputIndex
			}
			completedItems[event.OutputIndex] = event.Type == "response.output_item.done"
		case "response.function_call_arguments.done":
			index := event.OutputIndex
			if found, ok := itemIndexes[event.ItemID]; ok {
				index = found
			}
			var item map[string]json.RawMessage
			if json.Unmarshal(items[index], &item) != nil || string(item["type"]) != `"function_call"` || event.Arguments == nil || !json.Valid([]byte(*event.Arguments)) {
				return false, errors.New("invalid " + provider + " tool arguments")
			}
			item["arguments"], _ = json.Marshal(*event.Arguments)
			item["status"] = json.RawMessage(`"completed"`)
			raw, err := json.Marshal(item)
			if err != nil {
				return false, errors.New("invalid " + provider + " tool arguments")
			}
			if err := storeItem(index, raw); err != nil {
				return false, err
			}
			completedItems[index] = true
		case "response.output_text.done":
			if output.Len() == 0 {
				if len(event.Text) > MaxResponseBytes {
					return false, errors.New(provider + " response is too large")
				}
				output.WriteString(event.Text)
				emit(event.Text)
			}
		case "response.output_text.delta":
			if output.Len()+len(event.Delta) > MaxResponseBytes {
				return false, errors.New(provider + " response is too large; narrow the request")
			}
			output.WriteString(event.Delta)
			emit(event.Delta)
		case "response.failed", "error":
			code := event.Response.Error.Code
			if code == "" {
				code = event.Error.Code
			}
			if code == "" {
				code = event.Code
			}
			if provider == "ChatGPT" {
				return false, &chatGPTError{Code: code}
			}
			return false, errors.New(provider + " generation failed")
		case "response.incomplete":
			return false, errors.New(provider + " response was incomplete; narrow the request or retry")
		case "response.completed":
			if event.Response.Status != "" && event.Response.Status != "completed" {
				return false, errors.New(provider + " response did not complete")
			}
			var envelope struct {
				Response json.RawMessage `json:"response"`
			}
			if json.Unmarshal([]byte(raw), &envelope) != nil {
				return false, errors.New("invalid " + provider + " completion")
			}
			response := envelope.Response
			usage := decodeUsage("responses", response)
			if event.Response.Status == "" {
				fields := map[string]json.RawMessage{}
				if len(response) > 0 && string(response) != "null" {
					if json.Unmarshal(response, &fields) != nil {
						return false, errors.New("invalid " + provider + " completion")
					}
				}
				if fields == nil {
					fields = map[string]json.RawMessage{}
				}
				fields["status"] = json.RawMessage(`"completed"`)
				response, _ = json.Marshal(fields)
			}
			if len(event.Response.Output) == 0 && len(items) > 0 {
				indexes := make([]int, 0, len(items))
				for index := range items {
					indexes = append(indexes, index)
				}
				sort.Ints(indexes)
				var assembled []json.RawMessage
				for _, index := range indexes {
					var item struct {
						Type string `json:"type"`
					}
					if json.Unmarshal(items[index], &item) != nil {
						return false, errors.New("invalid " + provider + " output item")
					}
					if !completedItems[index] {
						if item.Type == "function_call" {
							return false, errors.New(provider + " tool call was incomplete; retry")
						}
						continue
					}
					assembled = append(assembled, items[index])
				}
				response, _ = json.Marshal(map[string]any{"status": "completed", "output": assembled})
			}
			parsed, parseErr := decodeToolTurn("responses", response)
			parsed.Usage = usage
			turn.Usage = usage
			if parseErr == nil {
				turn = parsed
				if len(turn.Calls) > 0 {
					return true, nil
				}
				if output.Len() == 0 {
					output.WriteString(turn.Text)
					emit(turn.Text)
				}
			}
			if output.Len() == 0 {
				for _, item := range event.Response.Output {
					if item.Type == "message" {
						for _, part := range item.Content {
							if part.Type == "output_text" {
								if output.Len()+len(part.Text) > MaxResponseBytes {
									return false, errors.New(provider + " response is too large")
								}
								output.WriteString(part.Text)
								emit(part.Text)
							}
						}
					}
				}
			}
			if strings.TrimSpace(output.String()) == "" {
				return false, errors.New(provider + " returned no SQL output")
			}
			return true, nil
		}
		return false, nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			done, err := process()
			if err != nil {
				return Turn{}, err
			}
			if done {
				turn.Text = output.String()
				return turn, nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if eventData.Len()+len(line) > 512*1024 {
				return Turn{}, errors.New(provider + " stream event is too large")
			}
			eventData.WriteString(strings.TrimPrefix(line, "data:"))
			eventData.WriteByte('\n')
		}
	}
	if eventData.Len() > 0 {
		done, err := process()
		if err != nil {
			return Turn{}, err
		}
		if done {
			turn.Text = output.String()
			return turn, nil
		}
	}
	return Turn{}, errors.New(provider + " connection ended before completion; retry (partial SQL was discarded)")
}
