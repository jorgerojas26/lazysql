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
	ConfigFile string
	AppConfig  *models.AppConfig `toml:"application"`
}

type DatabaseConfig struct {
	DatabaseConfigFile string
	Connections        []models.Connection `toml:"database"`
}

func defaultConfig() *Config {
	return &Config{
		AppConfig: &models.AppConfig{
			DefaultPageSize: 300,
			SidebarOverlay:  false,
		},
	}
}

func defaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		Connections: []models.Connection{},
	}
}

func DefaultConfigFile() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		configDir = dir
	}
	return filepath.Join(configDir, "lazysql", "config.toml"), nil
}

func DefaultDatabaseConfigFile() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		configDir = dir
	}
	return filepath.Join(configDir, "lazysql", "database.toml"), nil
}

func LoadConfig(configFile string) error {
	App.config.ConfigFile = configFile

	file, err := os.ReadFile(configFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	err = toml.Unmarshal(file, App.config)
	if err != nil {
		return err
	}

	return nil
}

func LoadDatabaseConfig(databaseConfigFile string) error {
	App.databaseConfig.DatabaseConfigFile = databaseConfigFile

	file, err := os.ReadFile(databaseConfigFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	err = toml.Unmarshal(file, App.databaseConfig)
	if err != nil {
		return err
	}

	for i, conn := range App.databaseConfig.Connections {
		App.databaseConfig.Connections[i].URL = parseConfigURL(&conn)
	}

	return nil
}

func (dc *DatabaseConfig) SaveConnections(connections []models.Connection) error {
	dc.Connections = connections

	if err := os.MkdirAll(filepath.Dir(dc.DatabaseConfigFile), 0o755); err != nil {
		return err
	}

	file, err := os.Create(dc.DatabaseConfigFile)
	if err != nil {
		return err
	}
	defer file.Close()

	return toml.NewEncoder(file).Encode(dc)
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
