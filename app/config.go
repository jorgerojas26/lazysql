package app

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/jorgerojas26/lazysql/models"
)

type Config struct {
	AppConfig   *models.AppConfig   `toml:"application"`
	Connections []models.Connection `toml:"database"`
}

func defaultConfig() *Config {
	return &Config{
		AppConfig: &models.AppConfig{
			DefaultPageSize: 300,
		},
	}
}

func LoadConfig() error {
	file, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "lazysql", "config.toml"))
	if err != nil {
		return err
	}
	return toml.Unmarshal(file, App.config)
}

func (c *Config) SaveConnections(connections []models.Connection) error {
	c.Connections = connections

	directoriesPath := filepath.Join(os.Getenv("HOME"), ".config", "lazysql")
	configFilePath := filepath.Join(directoriesPath, "config.toml")

	err := os.MkdirAll(directoriesPath, 0o755)
	if err != nil {
		return err
	}

	file, err := os.Create(configFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return toml.NewEncoder(file).Encode(c)
}
