package app

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

type Config struct {
	ConfigFile  string
	AppConfig   *models.AppConfig   `toml:"application"`
	Connections []models.Connection `toml:"database"`
	Keymaps     models.KeymapConfig `toml:"keymap"`
}

func defaultConfig() *Config {
	return &Config{
		AppConfig: &models.AppConfig{
			DefaultPageSize:              300,
			SidebarOverlay:               false,
			MaxQueryHistoryPerConnection: 100,
			TreeWidth:                    30,
			JSONViewerWordWrap:           false,
			EnterOpensJSONViewer:         false,
			ConfirmOnQuit:                true,
		},
	}
}

func GetConfigPath() (string, error) {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		configDir = dir
	}
	return configDir, nil
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

// FindLocalConfig walks up from CWD to find a `.lazysql.toml` file.
// It stops at the git repository root (`.git` directory/file).
func FindLocalConfig() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check for .lazysql.toml in current directory
		configPath := filepath.Join(dir, ".lazysql.toml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		// Check if we've reached the git repo root (.git file or directory).
		// This stops the search from going above the git boundary.
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return "", nil // reached git root without finding config
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}

	return "", nil
}

// mergeMaps recursively merges local map into global map.
// Nested maps are merged recursively, arrays are appended, scalar values are overridden by local.
func mergeMaps(global, local map[string]any) map[string]any {
	result := make(map[string]any, len(global))
	for k, v := range global {
		result[k] = v
	}
	for k, localVal := range local {
		globalVal, exists := result[k]
		if !exists {
			result[k] = localVal
			continue
		}
		result[k] = mergeValues(globalVal, localVal)
	}
	return result
}

// mergeValues handles the recursive merge of two values.
// Maps are merged recursively, arrays and scalars are replaced by local.
func mergeValues(globalVal, localVal any) any {
	globalMap, globalIsMap := globalVal.(map[string]any)
	localMap, localIsMap := localVal.(map[string]any)
	if globalIsMap && localIsMap {
		return mergeMaps(globalMap, localMap)
	}
	// Arrays and scalars: local replaces global entirely.
	// This means [[database]] in local config replaces global connections,
	// not appends to them.
	return localVal
}

func LoadConfig(configFile string) error {
	// Load global config
	file, err := os.ReadFile(configFile)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	expanded := expandEnvVars(string(file))

	var globalMap map[string]any
	if err := toml.Unmarshal([]byte(expanded), &globalMap); err != nil {
		return err
	}

	// Load local config if it exists
	localConfigPath, err := FindLocalConfig()
	if err != nil {
		return err
	}

	mergedMap := globalMap
	if localConfigPath != "" {
		localFile, err := os.ReadFile(localConfigPath)
		if err != nil {
			return err
		}

		localExpanded := expandEnvVars(string(localFile))

		var localMap map[string]any
		if err := toml.Unmarshal([]byte(localExpanded), &localMap); err != nil {
			return err
		}

		mergedMap = mergeMaps(globalMap, localMap)
	}

	// Marshal merged map back to TOML and unmarshal into App.config.
	// Unmarshaling directly into App.config preserves default values from
	// defaultConfig() for fields not present in the config files.
	mergedBytes, err := toml.Marshal(mergedMap)
	if err != nil {
		return err
	}

	if err := toml.Unmarshal(mergedBytes, App.config); err != nil {
		return err
	}

	for i, conn := range App.config.Connections {
		App.config.Connections[i].URL = parseConfigURL(&conn)
	}

	if err := ApplyKeymapConfig(App.config.Keymaps); err != nil {
		return err
	}

	return nil
}

// expandEnvVars expands environment variables in the format ${env:VAR_NAME}.
// Variables without the "env:" prefix (e.g., ${port}) are left unchanged
// to maintain compatibility with dynamic variables used at connection time.
func expandEnvVars(s string) string {
	return os.Expand(s, func(key string) string {
		if envKey, found := strings.CutPrefix(key, "env:"); found {
			return os.Getenv(envKey)
		}
		// Keep non-env variables unchanged (e.g., ${port})
		return "${" + key + "}"
	})
}

func (c *Config) SaveConnections(connections []models.Connection) error {
	c.Connections = connections

	if err := os.MkdirAll(filepath.Dir(c.ConfigFile), 0o755); err != nil {
		return err
	}

	file, err := os.Create(c.ConfigFile)
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
