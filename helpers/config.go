package helpers

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/jorgerojas26/lazysql/drivers"
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

	for idx, conn := range config.Connections {
		config.Connections[idx].URL = ParseConfigURL(&conn)
	}

	return config, nil
}

// ParseConfigURL will manually parse config url if url empty
//
// it main purpose is for handling username & password with special characters
//
// only sqlserver for now
func ParseConfigURL(conn *models.Connection) string {
	if conn.URL != "" {
		return conn.URL
	}

	// only sqlserver for now
	if conn.Provider != drivers.DriverMSSQL {
		return conn.URL
	}

	user := url.QueryEscape(conn.Username)
	pass := url.QueryEscape(conn.Password)

	return fmt.Sprintf(
		"%s://%s:%s@%s:%s?database=%s%s",
		conn.Provider,
		user,
		pass,
		conn.Hostname,
		conn.Port,
		conn.DBName,
		conn.URLParams,
	)
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
