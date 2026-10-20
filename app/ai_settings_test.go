package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAISettingsPreserveRawConfigurationWithoutMergedValues(t *testing.T) {
	global, local := loadProjectConfig(t, "[ai]\nenabled = false\nprovider = 'chatgpt'\nfuture_option = 'keep'\n\n[[database]]\nName = 'production'\nURL = 'postgres://${env:DB_USER}:${env:DB_PASSWORD}@host/db'\n", "[ai]\nmodel = 'local-model'\n")
	if err := UpdateAISettings(global, map[string]any{"provider": "opencode-go", "model": "test-model"}); err != nil {
		t.Fatal(err)
	}
	content := readFile(t, global)
	if !strings.Contains(content, "${env:DB_PASSWORD}") || strings.Contains(content, "local-model") {
		t.Fatal("saving AI settings expanded credentials or copied merged local config")
	}
	table := readTable(t, global)
	ai := table["ai"].(map[string]any)
	if ai["enabled"] != false || ai["future_option"] != "keep" || ai["provider"] != "opencode-go" {
		t.Fatalf("AI settings changed unexpectedly: %v", ai)
	}
	if strings.Contains(readFile(t, local), "opencode-go") {
		t.Fatal("global settings modified project configuration")
	}
	if err := UpdateAISettings(global, map[string]any{"enabled": true, "model": nil}); err != nil {
		t.Fatal(err)
	}
	config, err := ReadAISettings(global)
	if err != nil || !config.Enabled || config.Model != "" {
		t.Fatalf("updated AI settings = %+v %v", config, err)
	}
}

func TestAISettingsReadDoesNotInitializeAppOrValidateKeymaps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	text := "[ai]\nprovider = 'chatgpt'\n[keymap.Editor]\nGenerateSQL = 'Ctrl-x'\n"
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := ReadAISettings(path)
	if err != nil || config.Provider != "chatgpt" || config.Enabled {
		t.Fatalf("authentication should work independently of application keymaps: %+v %v", config, err)
	}
}

func TestMissingAISettingsRemainDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	config, err := ReadAISettings(path)
	if err != nil || config.Enabled {
		t.Fatal("missing config should not enable AI")
	}
	if err := UpdateAISettings(path, map[string]any{"provider": "chatgpt"}); err != nil {
		t.Fatal(err)
	}
	config, err = ReadAISettings(path)
	if err != nil || config.Enabled || config.Provider != "chatgpt" {
		t.Fatal("provider-only setup enabled AI")
	}
}
