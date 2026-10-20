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

func TestSaveAISelectionPersistsCurrentSelectionWithoutPerModelPreferences(t *testing.T) {
	global, local := loadProjectConfig(t, "[ai]\nenabled = true\nprovider = 'chatgpt'\nmodel = 'global'\n[ai.thinking_preferences]\n'chatgpt/global' = 'high'\n", "[ai]\nmodel = 'local'\nfuture_option = 'keep'\n[ai.thinking_preferences]\n'chatgpt/local' = 'low'\n")
	beforeGlobal := readFile(t, global)
	config := App.AIConfig()
	config.Provider, config.Model, config.APIFormat = "opencode-go", "selected", "responses"
	config.Thinking, config.ThinkingMode, config.ContextWindow = "low", "effort", 400000

	if err := App.SaveAISelection(config); err != nil {
		t.Fatal(err)
	}
	if readFile(t, global) != beforeGlobal {
		t.Fatal("selection modified global config while project config is active")
	}
	text := readFile(t, local)
	if !strings.Contains(text, "future_option = 'keep'") && !strings.Contains(text, "future_option = \"keep\"") {
		t.Fatal("selection removed unrelated AI settings")
	}
	stored, err := ReadAISettings(local)
	if err != nil || stored.Model != "selected" || stored.Thinking != "low" {
		t.Fatalf("saved selection: %+v %v", stored, err)
	}
	if strings.Contains(text, "thinking_preferences") {
		t.Fatal("retained obsolete per-model thinking preferences")
	}
	snapshot := App.AIConfig()
	snapshot.Thinking = "high"
	if App.AIConfig().Thinking != "low" {
		t.Fatal("AI config getter exposed mutable preference state")
	}
}
