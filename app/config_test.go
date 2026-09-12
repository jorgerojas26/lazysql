package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jorgerojas26/lazysql/models"
)

func TestConnectionPoolDefaultsAndOverrides(t *testing.T) {
	config := defaultConfig().AppConfig

	if config.MaxOpenConnections != models.DefaultMaxOpenConnections {
		t.Fatalf("MaxOpenConnections = %d, want %d", config.MaxOpenConnections, models.DefaultMaxOpenConnections)
	}
	if config.MaxIdleConnections != models.DefaultMaxIdleConnections {
		t.Fatalf("MaxIdleConnections = %d, want %d", config.MaxIdleConnections, models.DefaultMaxIdleConnections)
	}
	if config.ExactCountThreshold != models.DefaultExactCountThreshold {
		t.Fatalf("ExactCountThreshold = %d, want %d", config.ExactCountThreshold, models.DefaultExactCountThreshold)
	}
	if config.ExactCountTimeoutMS != models.DefaultExactCountTimeoutMS {
		t.Fatalf("ExactCountTimeoutMS = %d, want %d", config.ExactCountTimeoutMS, models.DefaultExactCountTimeoutMS)
	}
	if config.SchemaBulkLoadThreshold != models.DefaultSchemaBulkLoadThreshold {
		t.Fatalf("SchemaBulkLoadThreshold = %d, want %d", config.SchemaBulkLoadThreshold, models.DefaultSchemaBulkLoadThreshold)
	}

	pool, err := config.EffectiveConnectionPool(models.Connection{})
	if err != nil {
		t.Fatalf("EffectiveConnectionPool() error = %v", err)
	}
	if pool.MaxOpenConnections != 8 || pool.MaxIdleConnections != 8 {
		t.Fatalf("default pool = %+v, want 8 open and 8 idle", pool)
	}

	maxOpen, maxIdle := 3, 2
	pool, err = config.EffectiveConnectionPool(models.Connection{
		MaxOpenConnections: &maxOpen,
		MaxIdleConnections: &maxIdle,
	})
	if err != nil {
		t.Fatalf("EffectiveConnectionPool() with overrides error = %v", err)
	}
	if pool.MaxOpenConnections != maxOpen || pool.MaxIdleConnections != maxIdle {
		t.Fatalf("override pool = %+v, want %d open and %d idle", pool, maxOpen, maxIdle)
	}

	config.MaxOpenConnections = 12
	config.MaxIdleConnections = 10
	pool, err = config.EffectiveConnectionPool(models.Connection{})
	if err != nil {
		t.Fatalf("EffectiveConnectionPool() with app values error = %v", err)
	}
	if pool.MaxOpenConnections != 12 || pool.MaxIdleConnections != 10 {
		t.Fatalf("inherited pool = %+v, want 12 open and 10 idle", pool)
	}

	zero := 0
	pool, err = config.EffectiveConnectionPool(models.Connection{
		MaxOpenConnections: &zero,
		MaxIdleConnections: &zero,
	})
	if err != nil {
		t.Fatalf("EffectiveConnectionPool() with zero overrides error = %v", err)
	}
	if pool.MaxOpenConnections != models.DefaultMaxOpenConnections || pool.MaxIdleConnections != models.DefaultMaxIdleConnections {
		t.Fatalf("zero override pool = %+v, want LazySQL defaults", pool)
	}
}

func TestConnectionPoolRejectsIdleAboveOpen(t *testing.T) {
	config := defaultConfig().AppConfig
	config.MaxOpenConnections = 2
	config.MaxIdleConnections = 3

	if _, err := config.EffectiveConnectionPool(models.Connection{}); err == nil {
		t.Fatal("EffectiveConnectionPool() error = nil, want max idle validation error")
	}

	maxOpen, maxIdle := 2, 3
	if _, err := defaultConfig().AppConfig.EffectiveConnectionPool(models.Connection{
		MaxOpenConnections: &maxOpen,
		MaxIdleConnections: &maxIdle,
	}); err == nil {
		t.Fatal("EffectiveConnectionPool() override error = nil, want max idle validation error")
	}
}

func TestExpandEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "expand env var",
			input:    "postgres://${env:DB_USER}:${env:DB_PASSWORD}@localhost/db",
			envVars:  map[string]string{"DB_USER": "admin", "DB_PASSWORD": "secret"},
			expected: "postgres://admin:secret@localhost/db",
		},
		{
			name:     "preserve dynamic variables",
			input:    "postgres://user:pass@localhost:${port}/db",
			envVars:  map[string]string{},
			expected: "postgres://user:pass@localhost:${port}/db",
		},
		{
			name:     "mix env vars and dynamic variables",
			input:    "postgres://${env:DB_USER}:${env:DB_PASSWORD}@localhost:${port}/db",
			envVars:  map[string]string{"DB_USER": "admin", "DB_PASSWORD": "secret"},
			expected: "postgres://admin:secret@localhost:${port}/db",
		},
		{
			name:     "undefined env var becomes empty",
			input:    "postgres://${env:UNDEFINED_VAR}@localhost/db",
			envVars:  map[string]string{},
			expected: "postgres://@localhost/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			result := expandEnvVars(tt.input)
			if result != tt.expected {
				t.Errorf("expandEnvVars(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMergeMaps(t *testing.T) {
	tests := []struct {
		name     string
		global   map[string]any
		local    map[string]any
		expected map[string]any
	}{
		{
			name:     "simple key override",
			global:   map[string]any{"key": "global-value"},
			local:    map[string]any{"key": "local-value"},
			expected: map[string]any{"key": "local-value"},
		},
		{
			name:     "local-only keys",
			global:   map[string]any{"global-key": "value"},
			local:    map[string]any{"local-key": "value"},
			expected: map[string]any{"global-key": "value", "local-key": "value"},
		},
		{
			name:     "global-only keys",
			global:   map[string]any{"global-key": "value"},
			local:    map[string]any{},
			expected: map[string]any{"global-key": "value"},
		},
		{
			name:     "nested map merge",
			global:   map[string]any{"outer": map[string]any{"inner": "global", "shared": "global"}},
			local:    map[string]any{"outer": map[string]any{"inner": "local", "local-only": "value"}},
			expected: map[string]any{"outer": map[string]any{"inner": "local", "shared": "global", "local-only": "value"}},
		},
		{
			name:     "array replacement",
			global:   map[string]any{"arr": []any{"a", "b"}},
			local:    map[string]any{"arr": []any{"c", "d"}},
			expected: map[string]any{"arr": []any{"c", "d"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeMaps(tt.global, tt.local)
			for k, expectedVal := range tt.expected {
				resultVal, exists := result[k]
				if !exists {
					t.Errorf("mergeMaps()[%q] missing, got nil", k)
					continue
				}
				// Compare nested maps or slices
				switch expected := expectedVal.(type) {
				case map[string]any:
					resultMap := resultVal.(map[string]any)
					for mk, mv := range expected {
						if rv, ok := resultMap[mk]; !ok || rv != mv {
							t.Errorf("mergeMaps()[%q][%q] = %v, want %v", k, mk, rv, mv)
						}
					}
				case []any:
					resultSlice := resultVal.([]any)
					if len(resultSlice) != len(expected) {
						t.Errorf("mergeMaps()[%q] len = %d, want %d", k, len(resultSlice), len(expected))
						continue
					}
					for i, v := range expected {
						if resultSlice[i] != v {
							t.Errorf("mergeMaps()[%q][%d] = %v, want %v", k, i, resultSlice[i], v)
						}
					}
				default:
					if resultVal != expectedVal {
						t.Errorf("mergeMaps()[%q] = %v, want %v", k, resultVal, expectedVal)
					}
				}
			}
		})
	}
}

func TestFindLocalConfig(t *testing.T) {
	// Save original directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	tests := []struct {
		name        string
		setup       func(tmpDir string) error
		teardown    func(tmpDir string)
		expectFound bool
		expectPath  string
	}{
		{
			name: "finds config in current directory",
			setup: func(tmpDir string) error {
				if err := os.Chdir(tmpDir); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(tmpDir, ".lazysql.toml"), []byte("test"), 0o600)
			},
			expectFound: true,
		},
		{
			name: "finds config in parent directory",
			setup: func(tmpDir string) error {
				subDir := filepath.Join(tmpDir, "subdir")
				if err := os.MkdirAll(subDir, 0o700); err != nil {
					return err
				}
				if err := os.Chdir(subDir); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(tmpDir, ".lazysql.toml"), []byte("test"), 0o600)
			},
			expectFound: true,
		},
		{
			name: "stops at git boundary - config above repo root not found",
			setup: func(tmpDir string) error {
				// Structure:
				//   tmpDir/.lazysql.toml  ← above git boundary, should NOT be found
				//   tmpDir/repo/.git/     ← git boundary
				//   tmpDir/repo/project/   ← CWD
				repoDir := filepath.Join(tmpDir, "repo")
				projectDir := filepath.Join(repoDir, "project")
				if err := os.MkdirAll(projectDir, 0o700); err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o700); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(tmpDir, ".lazysql.toml"), []byte("test"), 0o600); err != nil {
					return err
				}
				return os.Chdir(projectDir)
			},
			expectFound: false,
		},
		{
			name: "finds config at repo root (same dir as .git)",
			setup: func(tmpDir string) error {
				// Structure:
				//   tmpDir/.git/            ← git boundary
				//   tmpDir/.lazysql.toml   ← config at repo root, SHOULD be found
				//   tmpDir/project/         ← CWD
				if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o700); err != nil {
					return err
				}
				projectDir := filepath.Join(tmpDir, "project")
				if err := os.MkdirAll(projectDir, 0o700); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(tmpDir, ".lazysql.toml"), []byte("test"), 0o600); err != nil {
					return err
				}
				return os.Chdir(projectDir)
			},
			expectFound: true,
		},
		{
			name: "no config found",
			setup: func(tmpDir string) error {
				if err := os.Chdir(tmpDir); err != nil {
					return err
				}
				// Create .git to stop the search before it reaches system temp
				return os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o700)
			},
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir, err := os.MkdirTemp("", "lazysql-test-*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			if err := tt.setup(tmpDir); err != nil {
				t.Fatal(err)
			}

			path, err := FindLocalConfig()
			if err != nil {
				t.Fatalf("FindLocalConfig() error = %v", err)
			}

			if tt.expectFound && path == "" {
				t.Errorf("FindLocalConfig() returned empty string, expected to find config")
			}
			if !tt.expectFound && path != "" {
				t.Errorf("FindLocalConfig() returned %q, expected empty string", path)
			}

			if tt.expectFound && path != "" {
				// Verify the file exists
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Errorf("FindLocalConfig() returned path %q that does not exist", path)
				}
			}
		})
	}
}

func TestLoadConfigConnectionPoolOverrides(t *testing.T) {
	originalConfig := App.config
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		App.config = originalConfig
		_ = os.Chdir(originalDir)
	})

	tmpDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(tmpDir, "config.toml")
	configText := `
[application]
max_open_connections = 12
max_idle_connections = 10

[[database]]
name = "override"
provider = "sqlite3"
url = ":memory:"
max_open_connections = 3
max_idle_connections = 2

[[database]]
name = "inherit"
provider = "sqlite3"
url = ":memory:"

[[database]]
name = "zero"
provider = "sqlite3"
url = ":memory:"
max_open_connections = 0
max_idle_connections = 0
`
	if err := os.WriteFile(configPath, []byte(configText), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	App.config = defaultConfig()
	if err := LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if App.config.AppConfig.MaxOpenConnections != 12 {
		t.Errorf("MaxOpenConnections = %d, want 12", App.config.AppConfig.MaxOpenConnections)
	}
	if App.config.AppConfig.MaxIdleConnections != 10 {
		t.Errorf("MaxIdleConnections = %d, want 10", App.config.AppConfig.MaxIdleConnections)
	}

	override, inherit, zero := App.config.Connections[0], App.config.Connections[1], App.config.Connections[2]
	overridePool, err := App.config.AppConfig.EffectiveConnectionPool(override)
	if err != nil {
		t.Fatalf("override pool error = %v", err)
	}
	if overridePool.MaxOpenConnections != 3 || overridePool.MaxIdleConnections != 2 {
		t.Errorf("override pool = %+v, want 3 open and 2 idle", overridePool)
	}

	inheritPool, err := App.config.AppConfig.EffectiveConnectionPool(inherit)
	if err != nil {
		t.Fatalf("inherited pool error = %v", err)
	}
	if inheritPool.MaxOpenConnections != 12 || inheritPool.MaxIdleConnections != 10 {
		t.Errorf("inherited pool = %+v, want 12 open and 10 idle", inheritPool)
	}

	zeroPool, err := App.config.AppConfig.EffectiveConnectionPool(zero)
	if err != nil {
		t.Fatalf("zero pool error = %v", err)
	}
	if zeroPool.MaxOpenConnections != models.DefaultMaxOpenConnections || zeroPool.MaxIdleConnections != models.DefaultMaxIdleConnections {
		t.Errorf("zero pool = %+v, want LazySQL defaults", zeroPool)
	}
}

func TestLoadConfigRejectsInvalidConnectionPool(t *testing.T) {
	originalConfig := App.config
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		App.config = originalConfig
		_ = os.Chdir(originalDir)
	})

	tmpDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(tmpDir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(tmpDir, "config.toml")
	configText := `
[application]
max_open_connections = 2
max_idle_connections = 3
`
	if err := os.WriteFile(configPath, []byte(configText), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	App.config = defaultConfig()
	if err := LoadConfig(configPath); err == nil {
		t.Fatal("LoadConfig() error = nil, want invalid pool configuration error")
	}

	configText = `
[application]
max_open_connections = 8
max_idle_connections = 8

[[database]]
name = "invalid-connection"
provider = "sqlite3"
url = ":memory:"
max_open_connections = 2
max_idle_connections = 3
`
	if err := os.WriteFile(configPath, []byte(configText), 0o600); err != nil {
		t.Fatal(err)
	}
	App.config = defaultConfig()
	if err := LoadConfig(configPath); err == nil {
		t.Fatal("LoadConfig() error = nil, want invalid per-connection pool configuration error")
	}

	configText = `
[application]
schema_bulk_load_threshold = 0
`
	if err := os.WriteFile(configPath, []byte(configText), 0o600); err != nil {
		t.Fatal(err)
	}
	App.config = defaultConfig()
	if err := LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig() with zero schema threshold error = %v", err)
	}
	if App.config.AppConfig.SchemaBulkLoadThreshold != 0 {
		t.Fatalf("SchemaBulkLoadThreshold = %d, want explicit zero", App.config.AppConfig.SchemaBulkLoadThreshold)
	}

	configText = `
[application]
schema_bulk_load_threshold = -1
`
	if err := os.WriteFile(configPath, []byte(configText), 0o600); err != nil {
		t.Fatal(err)
	}
	App.config = defaultConfig()
	if err := LoadConfig(configPath); err == nil {
		t.Fatal("LoadConfig() error = nil, want invalid schema bulk threshold error")
	}
}

func TestLoadConfigWithLocal(t *testing.T) {
	// Save original directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "lazysql-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .git directory to act as repo boundary
	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}

	// Create global config
	globalConfig := `
[application]
DefaultPageSize = 100
SidebarOverlay = false

[[database]]
name = "global-conn"
hostname = "global-host"
`
	globalPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(globalConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	// Create local config
	localConfig := `
[application]
DefaultPageSize = 500

[[database]]
name = "local-conn"
hostname = "local-host"
`
	localPath := filepath.Join(tmpDir, ".lazysql.toml")
	if err := os.WriteFile(localPath, []byte(localConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	// Change to the temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create a new App config to avoid pollution
	App.config = &Config{
		ConfigFile: globalPath,
	}

	if err := LoadConfig(globalPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify application settings were overridden by local
	if App.config.AppConfig.DefaultPageSize != 500 {
		t.Errorf("App.config.AppConfig.DefaultPageSize = %d, want 500 (local override)", App.config.AppConfig.DefaultPageSize)
	}

	// Verify local connections replace global connections (not appended)
	if len(App.config.Connections) != 1 {
		t.Errorf("len(App.config.Connections) = %d, want 1 (local replaces global)", len(App.config.Connections))
	}

	if len(App.config.Connections) > 0 {
		conn := App.config.Connections[0]
		if conn.Name != "local-conn" {
			t.Errorf("App.config.Connections[0].Name = %q, want %q", conn.Name, "local-conn")
		}
		if conn.Hostname != "local-host" {
			t.Errorf("App.config.Connections[0].Hostname = %q, want %q", conn.Hostname, "local-host")
		}
	}
}

func TestLoadConfigLocalReplacesConnections(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	tmpDir, err := os.MkdirTemp("", "lazysql-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}

	// Global config has two connections
	globalConfig := `
[[database]]
name = "staging"
hostname = "staging.example.com"

[[database]]
name = "production"
hostname = "prod.example.com"
`
	globalPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(globalConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	// Local config defines its own connections — these REPLACE global ones
	localConfig := `
[[database]]
name = "local-dev"
hostname = "localhost"
`
	localPath := filepath.Join(tmpDir, ".lazysql.toml")
	if err := os.WriteFile(localPath, []byte(localConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	App.config = &Config{
		ConfigFile: globalPath,
	}

	if err := LoadConfig(globalPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Local connections replace global connections entirely
	if len(App.config.Connections) != 1 {
		t.Fatalf("len(App.config.Connections) = %d, want 1 (local replaces global)", len(App.config.Connections))
	}

	conn := App.config.Connections[0]
	if conn.Name != "local-dev" {
		t.Errorf("connection name = %q, want %q", conn.Name, "local-dev")
	}
	if conn.Hostname != "localhost" {
		t.Errorf("connection hostname = %q, want %q", conn.Hostname, "localhost")
	}
}

func TestLoadConfigPreservesDefaults(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	tmpDir, err := os.MkdirTemp("", "lazysql-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}

	// Global config only sets one field
	globalConfig := `
[application]
DefaultPageSize = 500
`
	globalPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(globalConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	App.config = defaultConfig()

	if err := LoadConfig(globalPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Overridden field
	if App.config.AppConfig.DefaultPageSize != 500 {
		t.Errorf("DefaultPageSize = %d, want 500", App.config.AppConfig.DefaultPageSize)
	}

	// Fields not in config should keep defaults
	if App.config.AppConfig.TreeWidth != 30 {
		t.Errorf("TreeWidth = %d, want 30 (default)", App.config.AppConfig.TreeWidth)
	}
	if App.config.AppConfig.MaxQueryHistoryPerConnection != 100 {
		t.Errorf("MaxQueryHistoryPerConnection = %d, want 100 (default)", App.config.AppConfig.MaxQueryHistoryPerConnection)
	}
	if App.config.AppConfig.MaxQueryRows != models.DefaultMaxQueryRows {
		t.Errorf("MaxQueryRows = %d, want %d (default)", App.config.AppConfig.MaxQueryRows, models.DefaultMaxQueryRows)
	}
	if App.config.AppConfig.SchemaBulkLoadThreshold != models.DefaultSchemaBulkLoadThreshold {
		t.Errorf("SchemaBulkLoadThreshold = %d, want %d (default)", App.config.AppConfig.SchemaBulkLoadThreshold, models.DefaultSchemaBulkLoadThreshold)
	}
	if App.config.AppConfig.MaxOpenConnections != models.DefaultMaxOpenConnections {
		t.Errorf("MaxOpenConnections = %d, want %d (default)", App.config.AppConfig.MaxOpenConnections, models.DefaultMaxOpenConnections)
	}
	if App.config.AppConfig.MaxIdleConnections != models.DefaultMaxIdleConnections {
		t.Errorf("MaxIdleConnections = %d, want %d (default)", App.config.AppConfig.MaxIdleConnections, models.DefaultMaxIdleConnections)
	}
}
