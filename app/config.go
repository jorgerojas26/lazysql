package app

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

	err = toml.Unmarshal(file, App.config)
	if err != nil {
		return err
	}

	for i, conn := range App.config.Connections {
		App.config.Connections[i].URL = parseConfigURL(&conn)
	}

	return nil
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

// parseConfigURL automatically generates the URL from the connection struct
// if the URL is empty. It is useful for handling usernames and passwords with
// special characters. NOTE: Only MSSQL is supported for now!
func parseConfigURL(conn *models.Connection) string {
	if conn.URL != "" {
		return conn.URL
	}

	// Only MSSQL is supported for now.
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
