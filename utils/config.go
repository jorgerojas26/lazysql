package utils

import (
	"github.com/pelletier/go-toml/v2"
	"os"
	"path/filepath"
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

	file, err := os.Create(filepath.Join(os.Getenv("HOME"), ".config", "lazysql", "config.toml"))

	if err != nil {
		return
	}

	defer file.Close()

	err = toml.NewEncoder(file).Encode(config)

	return
}
