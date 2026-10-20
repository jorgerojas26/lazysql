package ai

func applyThinking(body map[string]any, format, level, mode string, budget int) {
	if level == "" || level == "auto" {
		return
	}
	switch format {
	case "responses":
		body["reasoning"] = map[string]string{"effort": level}
	case "anthropic":
		switch {
		case level == "none":
			body["thinking"] = map[string]string{"type": "disabled"}
		case mode == "effort":
			body["thinking"] = map[string]string{"type": "adaptive"}
			body["output_config"] = map[string]string{"effort": level}
		default:
			body["thinking"] = map[string]any{"type": "enabled", "budget_tokens": max(1024, budget)}
			body["max_tokens"] = max(8192, budget+8192)
		}
	case "gemini":
		generation, _ := body["generationConfig"].(map[string]any)
		if generation == nil {
			generation = map[string]any{}
			body["generationConfig"] = generation
		}
		switch {
		case level == "none":
			generation["thinkingConfig"] = map[string]int{"thinkingBudget": 0}
		case mode == "effort":
			generation["thinkingConfig"] = map[string]string{"thinkingLevel": level}
		case mode == "toggle":
			generation["thinkingConfig"] = map[string]int{"thinkingBudget": -1}
		default:
			generation["thinkingConfig"] = map[string]int{"thinkingBudget": budget}
			generation["maxOutputTokens"] = max(8192, budget+8192)
		}
	default:
		body["reasoning_effort"] = level
	}
}
