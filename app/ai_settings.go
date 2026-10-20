package app

import (
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
