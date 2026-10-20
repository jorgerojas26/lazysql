package ai

import (
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

const sqlAssistant = "You are a SQL generation assistant. Follow the requested JSON output format. Do not call tools."

func openCodeEndpointFor(model, format string) (string, error) {
	switch format {
	case "", "chat-completions":
		return openCodeEndpoint, nil
	case "responses":
		return "https://opencode.ai/inference/go/openai/v1/responses", nil
	case "anthropic":
		return "https://opencode.ai/inference/go/anthropic/v1/messages", nil
	case "gemini":
		return "https://opencode.ai/inference/go/google/v1beta/models/" + url.PathEscape(model) + ":generateContent", nil
	default:
		return "", errors.New("unknown OpenCode Go api_format; choose a model with lazysql ai login")
	}
}

func (client *openCodeClient) requestBody(prompt string) map[string]any {
	switch client.format {
	case "responses":
		return map[string]any{"model": client.model, "instructions": sqlAssistant, "input": prompt, "max_output_tokens": 8192, "stream": false, "store": false}
	case "anthropic":
		return map[string]any{"model": client.model, "system": sqlAssistant, "messages": []map[string]string{{"role": "user", "content": prompt}}, "max_tokens": 8192, "stream": false}
	case "gemini":
		return map[string]any{
			"systemInstruction": map[string]any{"parts": []map[string]string{{"text": sqlAssistant}}},
			"contents":          []map[string]any{{"role": "user", "parts": []map[string]string{{"text": prompt}}}},
			"generationConfig":  map[string]any{"maxOutputTokens": 8192, "responseMimeType": "application/json"},
		}
	default:
		return map[string]any{"model": client.model, "messages": []map[string]string{{"role": "system", "content": sqlAssistant}, {"role": "user", "content": prompt}}, "stream": false, "max_tokens": 8192}
	}
}

func decodeOpenCodeResponse(format string, data []byte) (string, error) {
	incomplete := errors.New("OpenCode Go did not finish the response; narrow the request or retry")
	invalid := errors.New("OpenCode Go returned an invalid response")
	var texts []string
	switch format {
	case "responses":
		var result struct {
			Status string `json:"status"`
			Output []struct {
				Type    string `json:"type"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"output"`
		}
		if json.Unmarshal(data, &result) != nil {
			return "", invalid
		}
		if result.Status != "completed" {
			return "", incomplete
		}
		for _, item := range result.Output {
			if item.Type == "message" {
				for _, part := range item.Content {
					if part.Type == "output_text" {
						texts = append(texts, part.Text)
					}
				}
			}
		}
	case "anthropic":
		var result struct {
			StopReason string `json:"stop_reason"`
			Content    []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(data, &result) != nil {
			return "", invalid
		}
		if result.StopReason != "end_turn" {
			return "", incomplete
		}
		for _, part := range result.Content {
			if part.Type == "text" {
				texts = append(texts, part.Text)
			}
		}
	case "gemini":
		var result struct {
			Candidates []struct {
				FinishReason string `json:"finishReason"`
				Content      struct {
					Parts []struct {
						Text    string `json:"text"`
						Thought bool   `json:"thought"`
					} `json:"parts"`
				} `json:"content"`
			} `json:"candidates"`
		}
		if json.Unmarshal(data, &result) != nil || len(result.Candidates) == 0 {
			return "", invalid
		}
		if result.Candidates[0].FinishReason != "STOP" {
			return "", incomplete
		}
		for _, part := range result.Candidates[0].Content.Parts {
			if !part.Thought && part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
	default:
		var result struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if json.Unmarshal(data, &result) != nil || len(result.Choices) == 0 {
			return "", invalid
		}
		if result.Choices[0].FinishReason != "stop" {
			return "", incomplete
		}
		texts = append(texts, result.Choices[0].Message.Content)
	}
	output := strings.TrimSpace(strings.Join(texts, "\n"))
	if output == "" {
		return "", invalid
	}
	return output, nil
}
