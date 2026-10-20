package ai

import (
	"errors"
	"slices"
	"strings"

	"github.com/jorgerojas26/lazysql/models"
)

type ThinkingLevel struct {
	ID     string
	Budget int
}

type ReasoningOption struct {
	Type   string   `json:"type"`
	Values []string `json:"values"`
	Min    int      `json:"min"`
	Max    int      `json:"max"`
}

type Model struct {
	ContextWindow  int
	ID             string
	Name           string
	Format         string
	Provider       string
	ThinkingMode   string
	ThinkingLevels []ThinkingLevel
}

func (model Model) Key() string {
	return model.Provider + "/" + model.ID
}

func ModelMatches(model Model, query string) bool {
	return strings.Contains(strings.ToLower(model.Name+" "+model.ID+" "+model.Format+" "+model.Provider), strings.ToLower(strings.TrimSpace(query)))
}

func (model *Model) setThinking(options []ReasoningOption, output int) {
	for _, option := range options {
		if option.Type != "effort" {
			continue
		}
		for _, value := range option.Values {
			if value == "" || value == "default" {
				continue
			}
			if !slices.ContainsFunc(model.ThinkingLevels, func(level ThinkingLevel) bool { return level.ID == value }) {
				model.ThinkingLevels = append(model.ThinkingLevels, ThinkingLevel{ID: value})
			}
		}
		if len(model.ThinkingLevels) > 0 {
			model.ThinkingMode = "effort"
		}
	}
	if len(model.ThinkingLevels) > 0 {
		if model.Format == "anthropic" || model.Format == "gemini" {
			for _, option := range options {
				if option.Type == "toggle" && !slices.ContainsFunc(model.ThinkingLevels, func(level ThinkingLevel) bool { return level.ID == "none" }) {
					model.ThinkingLevels = append([]ThinkingLevel{{ID: "none"}}, model.ThinkingLevels...)
					break
				}
			}
		}
		return
	}
	for _, option := range options {
		if option.Type == "budget_tokens" && (model.Format == "anthropic" || model.Format == "gemini") {
			model.ThinkingMode = "budget"
			upper := 16384
			if option.Max > 0 {
				upper = min(upper, option.Max)
			}
			if output > 0 {
				upper = min(upper, max(0, output-8192))
			}
			for _, level := range []ThinkingLevel{{ID: "low", Budget: 1024}, {ID: "medium", Budget: 4096}, {ID: "high", Budget: 16384}} {
				level.Budget = min(upper, max(option.Min, level.Budget))
				if level.Budget >= max(1024, option.Min) {
					if len(model.ThinkingLevels) == 0 || model.ThinkingLevels[len(model.ThinkingLevels)-1].Budget != level.Budget {
						model.ThinkingLevels = append(model.ThinkingLevels, level)
					}
				}
			}
			break
		}
	}
	for _, option := range options {
		if option.Type == "toggle" && (model.Format == "anthropic" || model.Format == "gemini") {
			model.ThinkingLevels = append([]ThinkingLevel{{ID: "none"}}, model.ThinkingLevels...)
			if model.ThinkingMode == "" {
				model.ThinkingMode = "toggle"
				model.ThinkingLevels = append(model.ThinkingLevels, ThinkingLevel{ID: "on", Budget: 1024})
			}
		}
	}
}

func (model Model) ConfigureThinking(config *models.AIConfig, level string) error {
	config.Thinking, config.ThinkingMode, config.ThinkingBudget = "", "", 0
	if level == "" || level == "auto" {
		return nil
	}
	for _, option := range model.ThinkingLevels {
		if option.ID == level {
			config.Thinking, config.ThinkingMode, config.ThinkingBudget = option.ID, model.ThinkingMode, option.Budget
			return nil
		}
	}
	return errors.New("this model does not support that thinking level")
}
