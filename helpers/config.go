package helpers

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/jorgerojas26/lazysql/models"
)

const configDirPerm = 0o755

type Config struct {
	Connections []models.Connection `toml:"database"`
}

func LoadConfig() (config Config, err error) {
	file, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "lazysql", "config.toml"))
	if err != nil {
		return
	}

	err = toml.Unmarshal(file, &config)

	return
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

	err = os.MkdirAll(directoriesPath, configDirPerm)

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
