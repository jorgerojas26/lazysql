package app

import (
	"os"
	"path/filepath"
	"testing"
)

func themeConfigFiles(t *testing.T, global, local string) string {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(project, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "config.toml")
	if err := os.WriteFile(path, []byte(global), 0600); err != nil {
		t.Fatal(err)
	}
	if local != "" {
		if err := os.WriteFile(filepath.Join(project, ".lazysql.toml"), []byte(local), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })
	return path
}

func TestReadThemeConfigMergesLocalColorsAndExpandsThemeVariables(t *testing.T) {
	t.Setenv("AUTH_THEME_COLOR", "#123456")
	global := "[theme]\nPreset='dracula'\n[theme.Colors]\nPrimitiveBackground='#102030'\nPrimaryText='#405060'\nSecondaryText='#708090'\n"
	local := "[theme]\nPreset='nord'\n[theme.Colors]\nPrimaryText='${env:AUTH_THEME_COLOR}'\n"
	path := themeConfigFiles(t, global, local)
	config, err := ReadThemeConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.Preset != "nord" || config.Colors["PrimaryText"] != "#123456" || config.Colors["PrimitiveBackground"] != "#102030" || config.Colors["SecondaryText"] != "#708090" {
		t.Fatalf("effective auth theme = %+v", config)
	}
	data, _ := os.ReadFile(path)
	if string(data) != global {
		t.Fatal("reading a theme modified configuration")
	}
}

func TestReadThemeConfigDoesNotLoadUnrelatedApplicationSettings(t *testing.T) {
	path := themeConfigFiles(t, "[theme]\nPreset='dracula'\n[application]\nmax_open_connections=-1\n[keymap.Editor]\nGenerateSQL='Ctrl-x'\n[ai]\nenabled=true\nprovider='opencode-go'\n", "")
	beforeAI := App.AIConfig()
	beforeTheme := Styles
	config, err := ReadThemeConfig(path)
	if err != nil || config.Preset != "dracula" {
		t.Fatalf("theme-only loading validated unrelated settings: %+v %v", config, err)
	}
	if App.AIConfig() != beforeAI || Styles != beforeTheme {
		t.Fatal("reading a theme initialized application configuration or changed runtime state")
	}
}

func TestReadThemeConfigDefaultsAndInvalidTheme(t *testing.T) {
	path := themeConfigFiles(t, "[ai]\nenabled=false\n", "")
	config, err := ReadThemeConfig(path)
	if err != nil || config.Preset != "" || len(config.Colors) != 0 {
		t.Fatalf("missing theme should use defaults: %+v %v", config, err)
	}
	if err := os.WriteFile(path, []byte("[theme]\nPreset='not-a-theme'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config, err = ReadThemeConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewTheme(config); err == nil {
		t.Fatal("invalid configured theme should not silently fall back")
	}
}
