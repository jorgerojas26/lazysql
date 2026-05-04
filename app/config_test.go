package app

import (
	"os"
	"path/filepath"
	"testing"
)

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
		os.Chdir(origDir)
	})

	tests := []struct {
		name          string
		setup         func(tmpDir string) error
		teardown     func(tmpDir string)
		expectFound  bool
		expectPath   string
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
				if err := os.MkdirAll(subDir, 0o755); err != nil {
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
				if err := os.MkdirAll(projectDir, 0o755); err != nil {
					return err
				}
				if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
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
				if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755); err != nil {
					return err
				}
				projectDir := filepath.Join(tmpDir, "project")
				if err := os.MkdirAll(projectDir, 0o755); err != nil {
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
				return os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755)
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

func TestLoadConfigWithLocal(t *testing.T) {
	// Save original directory
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chdir(origDir)
	})

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "lazysql-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create .git directory to act as repo boundary
	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create global config
	globalConfig := `
[application]
default_page_size = 100
sidebar_overlay = false

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
default_page_size = 500

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
		os.Chdir(origDir)
	})

	tmpDir, err := os.MkdirTemp("", "lazysql-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755); err != nil {
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
		os.Chdir(origDir)
	})

	tmpDir, err := os.MkdirTemp("", "lazysql-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Global config only sets one field
	globalConfig := `
[application]
default_page_size = 500
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
}
