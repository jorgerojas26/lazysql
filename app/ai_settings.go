package app

import (
	"errors"

	"github.com/pelletier/go-toml/v2"

	"github.com/jorgerojas26/lazysql/models"
)

func ReadAISettings(configFile string) (models.AIConfig, error) {
	table, err := readConfigTable(configFile)
	if err != nil {
		return models.AIConfig{}, err
	}
	if _, ok := table["ai"]; !ok {
		return models.AIConfig{}, nil
	}
	data, err := toml.Marshal(map[string]any{"ai": table["ai"]})
	if err != nil {
		return models.AIConfig{}, err
	}
	var config struct {
		AI models.AIConfig `toml:"ai"`
	}
	if err := toml.Unmarshal(data, &config); err != nil {
		return models.AIConfig{}, err
	}
	return config.AI, nil
}

func UpdateAISettings(configFile string, changes map[string]any) error {
	table, err := readConfigTable(configFile)
	if err != nil {
		return err
	}
	settings, _ := table["ai"].(map[string]any)
	if settings == nil {
		settings = map[string]any{}
	}
	for key, value := range changes {
		if value == nil {
			delete(settings, key)
		} else {
			settings[key] = value
		}
	}
	return saveConfigTable(configFile, table, "ai", settings)
}

func (application *Application) SaveAISelection(config models.AIConfig) error {
	path := application.config.activeConfigFile()
	if path == "" {
		return errors.New("no configuration file is available to save the selection")
	}
	changes := map[string]any{
		"provider":             config.Provider,
		"model":                config.Model,
		"api_format":           config.APIFormat,
		"context_window":       config.ContextWindow,
		"thinking":             config.Thinking,
		"thinking_mode":        config.ThinkingMode,
		"thinking_budget":      config.ThinkingBudget,
		"thinking_preferences": nil,
	}
	if err := UpdateAISettings(path, changes); err != nil {
		return err
	}
	config.Enabled = application.config.AI.Enabled
	application.config.AI = config
	return nil
}
