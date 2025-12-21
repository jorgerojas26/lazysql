package app

import (
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
