package helpers

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/jorgerojas26/lazysql/models"
)

type Config struct {
	Connections []models.Connection `toml:"database"`
}

func LoadConfig() (Config, error) {
	config := Config{}

	file, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "lazysql", "config.toml"))
	if err != nil {
		return config, err
	}

	err = toml.Unmarshal(file, &config)
	if err != nil {
		return config, err
	}

	for idx, cfg := range config.Connections {
		cfg.ParseURL()
		config.Connections[idx].URL = cfg.URL // can be better than this
	}

	return config, nil
}

func LoadConnections() (connections []models.Connection, err error) {
	config, err := LoadConfig()
	if err != nil {
		return
	}

	connections = config.Connections

	return
}

func SaveConnectionConfig(connections []models.Connection) (err error) {
	config := Config{Connections: connections}

	directoriesPath := filepath.Join(os.Getenv("HOME"), ".config", "lazysql")
	configFilePath := filepath.Join(directoriesPath, "config.toml")

	err = os.MkdirAll(directoriesPath, 0755)
	if err != nil {
		return err
	}

	file, err := os.Create(configFilePath)
	if err != nil {
		return err
	}

	defer file.Close()

	err = toml.NewEncoder(file).Encode(config)

	return err
}
