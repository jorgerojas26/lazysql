package ai

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
)

func readSSE(reader io.Reader, event func([]byte) (bool, error), completed ...func() bool) error {
	scanner := bufio.NewScanner(io.LimitReader(reader, 4*1024*1024))
	scanner.Buffer(make([]byte, 4096), 512*1024)
	var data strings.Builder
	process := func() (bool, error) {
		raw := strings.TrimSpace(data.String())
		data.Reset()
		if raw == "" {
			return false, nil
		}
		return event([]byte(raw))
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			done, err := process()
			if err != nil {
				return err
			}
			if done {
				return nil
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			if data.Len()+len(line) > 512*1024 {
				return errors.New("AI stream event is too large")
			}
			data.WriteString(strings.TrimPrefix(line, "data:"))
			data.WriteByte('\n')
		}
	}
	if data.Len() > 0 {
		done, err := process()
		if err != nil {
			return err
		}
		if done {
			return nil
		}
	}
	if scanner.Err() == nil && len(completed) > 0 && completed[0]() {
		return nil
	}
	return errors.New("AI stream ended before completion; partial output was discarded")
}

func readOpenCodeStream(reader io.Reader, format string, delta func(string)) (Turn, error) {
	var output strings.Builder
	emit := func(text string) error {
		if output.Len()+len(text) > MaxResponseBytes {
			return errors.New("AI response is too large")
		}
		output.WriteString(text)
		if delta != nil && text != "" {
			delta(text)
		}
		return nil
	}
	blocks := map[int]map[string]any{}
	complete := map[int]bool{}
	arguments := map[int]string{}
	var parts []json.RawMessage
	var reasoning strings.Builder
	finish := ""
	finished := false
	usage := Usage{}
	var result Turn
	assemble := func() error {
		indexes := make([]int, 0, len(blocks))
		for index := range blocks {
			indexes = append(indexes, index)
		}
		slices.Sort(indexes)
		var payload any
		switch format {
		case "anthropic":
			content := []any{}
			for _, index := range indexes {
				if !complete[index] {
					return errors.New("AI tool block was incomplete")
				}
				block := blocks[index]
				if block["type"] == "tool_use" && arguments[index] != "" {
					if !json.Valid([]byte(arguments[index])) {
						return errors.New("invalid AI tool arguments")
					}
					block["input"] = json.RawMessage(arguments[index])
				}
				content = append(content, block)
			}
			payload = map[string]any{"stop_reason": finish, "content": content}
		case "gemini":
			payload = map[string]any{"candidates": []any{map[string]any{"finishReason": finish, "content": map[string]any{"role": "model", "parts": parts}}}}
		default:
			calls := []any{}
			for _, index := range indexes {
				calls = append(calls, blocks[index])
			}
			message := map[string]any{"role": "assistant", "content": output.String()}
			if len(calls) > 0 {
				message["tool_calls"] = calls
			}
			if reasoning.Len() > 0 {
				message["reasoning_content"] = reasoning.String()
			}
			payload = map[string]any{"choices": []any{map[string]any{"finish_reason": finish, "message": message}}}
		}
		data, err := json.Marshal(payload)
		if err != nil || len(data) > MaxResponseBytes*2 {
			return errors.New("AI streamed response is too large or invalid")
		}
		result, err = decodeToolTurn(format, data)
		if err == nil {
			result.Text = output.String()
		}
		return err
	}
	err := readSSE(reader, func(raw []byte) (bool, error) {
		if string(raw) == "[DONE]" {
			if finished {
				return true, nil
			}
			return false, errors.New("AI stream ended without a completed response")
		}
		if len(raw) > MaxResponseBytes*2 {
			return false, errors.New("AI stream event is too large")
		}
		usage.Merge(decodeUsage(format, raw))
		switch format {
		case "anthropic":
			var event struct {
				Message json.RawMessage `json:"message"`
				Type    string          `json:"type"`
				Index   int             `json:"index"`
				Block   map[string]any  `json:"content_block"`
				Delta   struct {
					Type      string `json:"type"`
					Text      string `json:"text"`
					JSON      string `json:"partial_json"`
					Thinking  string `json:"thinking"`
					Signature string `json:"signature"`
					Stop      string `json:"stop_reason"`
				} `json:"delta"`
			}
			if json.Unmarshal(raw, &event) != nil {
				return false, errors.New("invalid AI stream event")
			}
			if event.Index < 0 || event.Index > 63 {
				return false, errors.New("too many AI content blocks")
			}
			if event.Type == "message_start" {
				usage.Merge(decodeUsage("anthropic", event.Message))
			}
			switch event.Type {
			case "error":
				return false, errors.New("OpenCode Go generation failed")
			case "content_block_start":
				if event.Block == nil {
					return false, errors.New("invalid AI content block")
				}
				blocks[event.Index] = event.Block
				if event.Block["type"] == "text" {
					if text, ok := event.Block["text"].(string); ok {
						if err := emit(text); err != nil {
							return false, err
						}
					}
				}
			case "content_block_delta":
				block := blocks[event.Index]
				if block == nil {
					return false, errors.New("AI content block is missing")
				}
				appendField := func(key, value string) error {
					previous, _ := block[key].(string)
					if len(previous)+len(value) > MaxResponseBytes*2 {
						return errors.New("AI content block is too large")
					}
					block[key] = previous + value
					return nil
				}
				switch event.Delta.Type {
				case "text_delta":
					if err := appendField("text", event.Delta.Text); err != nil {
						return false, err
					}
					if err := emit(event.Delta.Text); err != nil {
						return false, err
					}
				case "input_json_delta":
					arguments[event.Index] += event.Delta.JSON
					if len(arguments[event.Index]) > MaxResponseBytes {
						return false, errors.New("AI tool arguments are too large")
					}
				case "thinking_delta":
					if err := appendField("thinking", event.Delta.Thinking); err != nil {
						return false, err
					}
				case "signature_delta":
					if err := appendField("signature", event.Delta.Signature); err != nil {
						return false, err
					}
				}
			case "content_block_stop":
				complete[event.Index] = true
			case "message_delta":
				if event.Delta.Stop != "" {
					finish = event.Delta.Stop
				}
			case "message_stop":
				return true, assemble()
			}
		case "gemini":
			var chunk struct {
				Error      json.RawMessage `json:"error"`
				Candidates []struct {
					Finish  string `json:"finishReason"`
					Content struct {
						Parts []json.RawMessage `json:"parts"`
					} `json:"content"`
				} `json:"candidates"`
			}
			if json.Unmarshal(raw, &chunk) != nil || len(chunk.Error) > 0 {
				return false, errors.New("invalid AI stream response")
			}
			if len(chunk.Candidates) == 0 {
				return false, nil
			}
			candidate := chunk.Candidates[0]
			for _, rawPart := range candidate.Content.Parts {
				var part struct {
					Text    string `json:"text"`
					Thought bool   `json:"thought"`
				}
				if json.Unmarshal(rawPart, &part) != nil {
					return false, errors.New("invalid AI content part")
				}
				var fields map[string]json.RawMessage
				if json.Unmarshal(rawPart, &fields) != nil {
					return false, errors.New("invalid AI content part")
				}
				merged := false
				_, hasText := fields["text"]
				if hasText && len(fields) <= 2 && (len(fields) == 1 || fields["thought"] != nil) && len(parts) > 0 {
					var previous map[string]json.RawMessage
					if json.Unmarshal(parts[len(parts)-1], &previous) == nil && previous["text"] != nil && len(previous) <= 2 && (len(previous) == 1 || previous["thought"] != nil) {
						var oldText string
						var oldThought bool
						json.Unmarshal(previous["text"], &oldText)
						json.Unmarshal(previous["thought"], &oldThought)
						if oldThought == part.Thought && len(oldText)+len(part.Text) <= MaxResponseBytes {
							previous["text"], _ = json.Marshal(oldText + part.Text)
							parts[len(parts)-1], _ = json.Marshal(previous)
							merged = true
						}
					}
				}
				if !merged {
					parts = append(parts, rawPart)
				}
				if len(parts) > 1024 {
					return false, errors.New("too many AI content parts")
				}
				if !part.Thought {
					if err := emit(part.Text); err != nil {
						return false, err
					}
				}
			}
			if candidate.Finish != "" {
				finish = candidate.Finish
				err := assemble()
				finished = err == nil
				return false, err
			}
		default:
			var chunk struct {
				Error   json.RawMessage `json:"error"`
				Choices []struct {
					Finish string `json:"finish_reason"`
					Delta  struct {
						Content   string `json:"content"`
						Reasoning string `json:"reasoning_content"`
						Calls     []struct {
							Index    int    `json:"index"`
							ID       string `json:"id"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if json.Unmarshal(raw, &chunk) != nil || len(chunk.Error) > 0 {
				return false, errors.New("invalid AI stream response")
			}
			if len(chunk.Choices) == 0 {
				return false, nil
			}
			choice := chunk.Choices[0]
			if err := emit(choice.Delta.Content); err != nil {
				return false, err
			}
			if reasoning.Len()+len(choice.Delta.Reasoning) > MaxResponseBytes {
				return false, errors.New("AI reasoning response is too large")
			}
			reasoning.WriteString(choice.Delta.Reasoning)
			for _, call := range choice.Delta.Calls {
				if call.Index < 0 || call.Index > 63 {
					return false, errors.New("too many AI tool calls")
				}
				block := blocks[call.Index]
				if block == nil {
					block = map[string]any{"type": "function", "id": "", "function": map[string]any{"name": "", "arguments": ""}}
					blocks[call.Index] = block
				}
				if call.ID != "" {
					block["id"] = call.ID
				}
				function := block["function"].(map[string]any)
				function["name"] = function["name"].(string) + call.Function.Name
				arguments[call.Index] += call.Function.Arguments
				if len(arguments[call.Index]) > MaxResponseBytes {
					return false, errors.New("AI tool arguments are too large")
				}
				function["arguments"] = arguments[call.Index]
			}
			if choice.Finish != "" {
				finish = choice.Finish
				err := assemble()
				finished = err == nil
				return false, err
			}
		}
		return false, nil
	}, func() bool { return finished })
	if err != nil {
		return Turn{}, err
	}
	result.Usage = usage
	return result, nil
}
