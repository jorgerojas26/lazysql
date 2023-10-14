package utils

import (
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Connection struct {
	Name     string
	Provider string
	User     string
	Password string
	Host     string
	Port     string
}

type Config struct {
	Connections []Connection `toml:"database"`
}

func LoadConfig() (config Config, err error) {
	file, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "lazysql", "config.toml"))

	if err != nil {
		return
	}

	err = toml.Unmarshal(file, &config)

	return

}

func LoadConnections() (databases []Connection, err error) {
	config, err := LoadConfig()

	if err != nil {
		return
	}

	databases = config.Connections

	return
}

func SaveConnectionConfig(databases []Connection) (err error) {
	config := Config{Connections: databases}

	directoriesPath := filepath.Join(os.Getenv("HOME"), ".config", "lazysql")
	configFilePath := filepath.Join(directoriesPath, "config.toml")

	err = os.MkdirAll(directoriesPath, 0755)

	if err != nil {
		return
	}

	file, err := os.OpenFile(configFilePath, os.O_RDWR|os.O_CREATE, 0755)

	if err != nil {
		return
	}

	defer file.Close()

	err = toml.NewEncoder(file).Encode(config)

	return
}
