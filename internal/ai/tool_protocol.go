package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func toolRequestBody(format, model, instructions string, history []Message, tools []Tool) map[string]any {
	definitions := []any{}
	messages := []any{}
	for _, tool := range tools {
		switch format {
		case "responses":
			definitions = append(definitions, map[string]any{"type": "function", "name": tool.Name, "description": tool.Description, "parameters": tool.Parameters, "strict": false})
		case "anthropic":
			definitions = append(definitions, map[string]any{"name": tool.Name, "description": tool.Description, "input_schema": tool.Parameters})
		case "gemini":
			definitions = append(definitions, map[string]any{"name": tool.Name, "description": tool.Description, "parametersJsonSchema": tool.Parameters})
		default:
			definitions = append(definitions, map[string]any{"type": "function", "function": map[string]any{"name": tool.Name, "description": tool.Description, "parameters": tool.Parameters}})
		}
	}
	for _, message := range history {
		switch format {
		case "responses":
			if message.Role == "assistant" && len(message.Replay) > 0 {
				var items []json.RawMessage
				if json.Unmarshal(message.Replay, &items) == nil {
					for _, item := range items {
						messages = append(messages, item)
					}
				}
			} else if message.Role == "tool" {
				messages = append(messages, map[string]any{"type": "function_call_output", "call_id": message.CallID, "output": message.Text})
			} else {
				messages = append(messages, map[string]any{"role": message.Role, "content": message.Text})
			}
		case "anthropic":
			if message.Role == "assistant" && len(message.Replay) > 0 {
				messages = append(messages, map[string]any{"role": "assistant", "content": message.Replay})
			} else if message.Role == "tool" {
				block := map[string]any{"type": "tool_result", "tool_use_id": message.CallID, "content": message.Text}
				if len(messages) > 0 {
					previous, ok := messages[len(messages)-1].(map[string]any)
					if ok && previous["role"] == "user" {
						if content, ok := previous["content"].([]any); ok {
							previous["content"] = append(content, block)
							continue
						}
					}
				}
				messages = append(messages, map[string]any{"role": "user", "content": []any{block}})
			} else {
				messages = append(messages, map[string]any{"role": message.Role, "content": message.Text})
			}
		case "gemini":
			if message.Role == "assistant" && len(message.Replay) > 0 {
				messages = append(messages, message.Replay)
			} else if message.Role == "tool" {
				var response any
				if json.Unmarshal([]byte(message.Text), &response) != nil {
					response = map[string]string{"error": "invalid metadata"}
				}
				if _, ok := response.(map[string]any); !ok {
					response = map[string]any{"result": response}
				}
				part := map[string]any{"functionResponse": map[string]any{"name": message.Name, "response": response}}
				if len(messages) > 0 {
					previous, ok := messages[len(messages)-1].(map[string]any)
					if ok && previous["role"] == "user" {
						if parts, ok := previous["parts"].([]any); ok {
							previous["parts"] = append(parts, part)
							continue
						}
					}
				}
				messages = append(messages, map[string]any{"role": "user", "parts": []any{part}})
			} else {
				messages = append(messages, map[string]any{"role": "user", "parts": []any{map[string]string{"text": message.Text}}})
			}
		default:
			if message.Role == "assistant" && len(message.Replay) > 0 {
				messages = append(messages, message.Replay)
			} else if message.Role == "tool" {
				messages = append(messages, map[string]any{"role": "tool", "tool_call_id": message.CallID, "content": message.Text})
			} else {
				messages = append(messages, map[string]any{"role": message.Role, "content": message.Text})
			}
		}
	}
	switch format {
	case "responses":
		return map[string]any{"model": model, "instructions": instructions, "input": messages, "tools": definitions, "include": []string{"reasoning.encrypted_content"}, "store": false, "stream": false, "max_output_tokens": 8192}
	case "anthropic":
		return map[string]any{"model": model, "system": instructions, "messages": messages, "tools": definitions, "max_tokens": 8192, "stream": false}
	case "gemini":
		return map[string]any{"systemInstruction": map[string]any{"parts": []any{map[string]string{"text": instructions}}}, "contents": messages, "tools": []any{map[string]any{"functionDeclarations": definitions}}, "generationConfig": map[string]any{"maxOutputTokens": 8192}}
	default:
		messages = append([]any{map[string]string{"role": "system", "content": instructions}}, messages...)
		return map[string]any{"model": model, "messages": messages, "tools": definitions, "stream": false, "max_tokens": 8192}
	}
}

func decodeToolTurn(format string, data []byte) (Turn, error) {
	invalid := errors.New("AI returned an invalid or incomplete tool response")
	if len(data) > MaxResponseBytes*2 {
		return Turn{}, invalid
	}
	var turn Turn
	var texts []string
	switch format {
	case "responses":
		var response struct {
			Status string          `json:"status"`
			Output json.RawMessage `json:"output"`
		}
		if json.Unmarshal(data, &response) != nil || response.Status != "completed" {
			return Turn{}, invalid
		}
		var items []struct {
			Type      string `json:"type"`
			ID        string `json:"call_id"`
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
			Status    string `json:"status"`
			Content   []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(response.Output, &items) != nil {
			return Turn{}, invalid
		}
		turn.Replay = response.Output
		for _, item := range items {
			if item.Type == "function_call" {
				if item.Status != "" && item.Status != "completed" {
					return Turn{}, invalid
				}
				turn.Calls = append(turn.Calls, ToolCall{ID: item.ID, Name: item.Name, Arguments: json.RawMessage(item.Arguments)})
			}
			if item.Type == "message" {
				for _, part := range item.Content {
					if part.Type == "output_text" {
						texts = append(texts, part.Text)
					}
				}
			}
		}
	case "anthropic":
		var response struct {
			StopReason string          `json:"stop_reason"`
			Content    json.RawMessage `json:"content"`
		}
		if json.Unmarshal(data, &response) != nil || (response.StopReason != "end_turn" && response.StopReason != "tool_use") {
			return Turn{}, invalid
		}
		var items []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		if json.Unmarshal(response.Content, &items) != nil {
			return Turn{}, invalid
		}
		turn.Replay = response.Content
		for _, item := range items {
			if item.Type == "tool_use" {
				turn.Calls = append(turn.Calls, ToolCall{ID: item.ID, Name: item.Name, Arguments: item.Input})
			}
			if item.Type == "text" {
				texts = append(texts, item.Text)
			}
		}
		if (response.StopReason == "tool_use") != (len(turn.Calls) > 0) {
			return Turn{}, invalid
		}
	case "gemini":
		var response struct {
			Candidates []struct {
				FinishReason string          `json:"finishReason"`
				Content      json.RawMessage `json:"content"`
			} `json:"candidates"`
		}
		if json.Unmarshal(data, &response) != nil || len(response.Candidates) == 0 || response.Candidates[0].FinishReason != "STOP" {
			return Turn{}, invalid
		}
		turn.Replay = response.Candidates[0].Content
		var content struct {
			Parts []struct {
				Text    string `json:"text"`
				Thought bool   `json:"thought"`
				Call    *struct {
					Name string          `json:"name"`
					Args json.RawMessage `json:"args"`
				} `json:"functionCall"`
			} `json:"parts"`
		}
		if json.Unmarshal(turn.Replay, &content) != nil {
			return Turn{}, invalid
		}
		for i, part := range content.Parts {
			if part.Call != nil {
				turn.Calls = append(turn.Calls, ToolCall{ID: fmt.Sprintf("call_%d", i), Name: part.Call.Name, Arguments: part.Call.Args})
			}
			if !part.Thought && part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
	default:
		var response struct {
			Choices []struct {
				FinishReason string          `json:"finish_reason"`
				Message      json.RawMessage `json:"message"`
			} `json:"choices"`
		}
		if json.Unmarshal(data, &response) != nil || len(response.Choices) == 0 || (response.Choices[0].FinishReason != "stop" && response.Choices[0].FinishReason != "tool_calls") {
			return Turn{}, invalid
		}
		turn.Replay = response.Choices[0].Message
		var message struct {
			Content string `json:"content"`
			Calls   []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		}
		if json.Unmarshal(turn.Replay, &message) != nil {
			return Turn{}, invalid
		}
		texts = append(texts, message.Content)
		for _, call := range message.Calls {
			if call.Type != "function" {
				return Turn{}, invalid
			}
			turn.Calls = append(turn.Calls, ToolCall{ID: call.ID, Name: call.Function.Name, Arguments: json.RawMessage(call.Function.Arguments)})
		}
		if (response.Choices[0].FinishReason == "tool_calls") != (len(turn.Calls) > 0) {
			return Turn{}, invalid
		}
	}
	turn.Text = strings.TrimSpace(strings.Join(texts, "\n"))
	if len(turn.Text) > MaxResponseBytes || (turn.Text == "" && len(turn.Calls) == 0) {
		return Turn{}, invalid
	}
	return turn, nil
}
