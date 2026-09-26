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
	// ConfigFile and LocalConfigFile are runtime state, never read from or
	// written to a config file.
	ConfigFile      string              `toml:"-"`
	LocalConfigFile string              `toml:"-"`
	AppConfig       *models.AppConfig   `toml:"application"`
	Connections     []models.Connection `toml:"database"`
	Keymaps         models.KeymapConfig `toml:"keymap"`
	Theme           *ThemeConfig        `toml:"theme,omitempty"`
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
			MaxOpenConnections:           models.DefaultMaxOpenConnections,
			MaxIdleConnections:           models.DefaultMaxIdleConnections,
			ExactCountThreshold:          models.DefaultExactCountThreshold,
			ExactCountTimeoutMS:          models.DefaultExactCountTimeoutMS,
			MaxQueryRows:                 models.DefaultMaxQueryRows,
			SchemaBulkLoadThreshold:      models.DefaultSchemaBulkLoadThreshold,
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
	if App.config == nil {
		App.config = defaultConfig()
	}
	if App.config.AppConfig == nil {
		App.config.AppConfig = defaultConfig().AppConfig
	}

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
		App.config.LocalConfigFile = localConfigPath

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

	if App.config.AppConfig.MaxQueryRows < 0 {
		return fmt.Errorf("invalid application max_query_rows: must be non-negative")
	}
	if App.config.AppConfig.SchemaBulkLoadThreshold < 0 {
		return fmt.Errorf("invalid application schema_bulk_load_threshold: must be non-negative")
	}

	poolConfig, err := App.config.AppConfig.EffectiveConnectionPool(models.Connection{})
	if err != nil {
		return fmt.Errorf("invalid application connection pool configuration: %w", err)
	}
	App.config.AppConfig.MaxOpenConnections = poolConfig.MaxOpenConnections
	App.config.AppConfig.MaxIdleConnections = poolConfig.MaxIdleConnections

	for i, conn := range App.config.Connections {
		if _, err := App.config.AppConfig.EffectiveConnectionPool(conn); err != nil {
			return fmt.Errorf("invalid connection pool configuration for %q: %w", conn.Name, err)
		}
		App.config.Connections[i].URL = parseConfigURL(&conn)
	}

	if err := ApplyKeymapConfig(App.config.Keymaps); err != nil {
		return err
	}

	themeConfig := ThemeConfig{}
	if App.config.Theme != nil {
		themeConfig = *App.config.Theme
	}
	if err := ApplyTheme(themeConfig); err != nil {
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

// SaveConnections writes the connection list back to the file it came from:
// the local config when that file defines [[database]] (it replaces the global
// list), otherwise the global config.
func (c *Config) SaveConnections(connections []models.Connection) error {
	configFile := c.ConfigFile
	toLocal := false
	if c.LocalConfigFile != "" {
		local, err := readConfigTable(c.LocalConfigFile)
		if err != nil {
			return err
		}
		if _, toLocal = local["database"]; toLocal {
			configFile = c.LocalConfigFile
		}
	}

	var value any = connections
	if len(connections) == 0 {
		value = nil
		if toLocal {
			// Keep an empty list so the local file still replaces the global one.
			value = []any{}
		}
	}
	if err := saveConfigKey(configFile, "database", value); err != nil {
		return err
	}
	c.Connections = connections
	return nil
}

// SaveThemePreset writes the preset to the local config when one is in use,
// otherwise to the global config.
func (c *Config) SaveThemePreset(preset string) error {
	theme := &ThemeConfig{Preset: preset}
	if err := saveConfigKey(c.activeConfigFile(), "theme", theme); err != nil {
		return err
	}
	c.Theme = theme
	return nil
}

func (c *Config) activeConfigFile() string {
	if c.LocalConfigFile != "" {
		return c.LocalConfigFile
	}
	return c.ConfigFile
}

// saveConfigKey replaces one top-level key in configFile and keeps the rest
// of that file's own content. It never writes the merged global and local
// configuration, so a save to .lazysql.toml cannot copy global connections
// or settings into it. A nil value removes the key.
func saveConfigKey(configFile, key string, value any) error {
	table, err := readConfigTable(configFile)
	if err != nil {
		return err
	}
	if value == nil {
		delete(table, key)
	} else {
		table[key] = value
	}
	// Older versions wrote these runtime paths into config files.
	delete(table, "ConfigFile")
	delete(table, "LocalConfigFile")

	data, err := toml.Marshal(table)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configFile), 0o700); err != nil {
		return err
	}

	file, err := os.Create(configFile)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

// readConfigTable reads a config file as written on disk, without expanding
// environment variables, so untouched values are saved back unchanged.
func readConfigTable(configFile string) (map[string]any, error) {
	data, err := os.ReadFile(configFile)
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}

	table := map[string]any{}
	if err := toml.Unmarshal(data, &table); err != nil {
		return nil, fmt.Errorf("reading %s: %w", configFile, err)
	}
	return table, nil
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
