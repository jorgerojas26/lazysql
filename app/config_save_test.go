package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"github.com/jorgerojas26/lazysql/models"
)

const credentialedGlobalConfig = `
[application]
DefaultPageSize = 50

[[database]]
Name = "prod"
URL = "postgres://user:pass@host/db"
`

// loadProjectConfig writes a global config and a project .lazysql.toml, loads
// them from the project directory and returns both paths.
func loadProjectConfig(t *testing.T, global, local string) (string, string) {
	t.Helper()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	tmpDir := t.TempDir()
	globalPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(global), 0o600); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(filepath.Join(projectDir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	localPath := filepath.Join(projectDir, ".lazysql.toml")
	if err := os.WriteFile(localPath, []byte(local), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(projectDir); err != nil {
		t.Fatal(err)
	}

	App.config = &Config{ConfigFile: globalPath}
	t.Cleanup(func() {
		App.config = defaultConfig()
		_ = ApplyTheme(ThemeConfig{})
	})
	if err := LoadConfig(globalPath); err != nil {
		t.Fatal(err)
	}
	return globalPath, localPath
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func readTable(t *testing.T, path string) map[string]any {
	t.Helper()
	table := map[string]any{}
	if err := toml.Unmarshal([]byte(readFile(t, path)), &table); err != nil {
		t.Fatal(err)
	}
	return table
}

func assertNoMergedContent(t *testing.T, path string) {
	t.Helper()
	content := readFile(t, path)
	for _, leaked := range []string{"database", "application", "ConfigFile", "LocalConfigFile", "user:pass"} {
		if strings.Contains(content, leaked) {
			t.Errorf("%s contains %q:\n%s", filepath.Base(path), leaked, content)
		}
	}
}

func TestSaveThemePresetDoesNotWriteGlobalConfigIntoLocalFile(t *testing.T) {
	// Ctrl-E is the default binding, so loading this does not change global keymaps.
	local := "# project theme\n[theme]\nPreset = \"nord\"\n\n[keymap.Home]\nSwitchToEditorView = \"Ctrl-E\"\n"
	globalPath, localPath := loadProjectConfig(t, credentialedGlobalConfig, local)

	if err := App.SaveThemePreset("dracula"); err != nil {
		t.Fatal(err)
	}

	assertNoMergedContent(t, localPath)
	table := readTable(t, localPath)
	theme, _ := table["theme"].(map[string]any)
	if theme["Preset"] != "dracula" {
		t.Errorf("local theme = %v, want Preset dracula", table["theme"])
	}
	keymap, _ := table["keymap"].(map[string]any)
	home, _ := keymap["Home"].(map[string]any)
	if home["SwitchToEditorView"] != "Ctrl-E" {
		t.Errorf("local keymap = %v, want the existing binding kept", table["keymap"])
	}
	if got := readFile(t, globalPath); got != credentialedGlobalConfig {
		t.Errorf("global config changed:\n%s", got)
	}
}

func TestSaveConnectionsWithLocalFileWithoutConnectionsWritesGlobalFile(t *testing.T) {
	local := "# project theme\n[theme]\nPreset = \"nord\"\n"
	globalPath, localPath := loadProjectConfig(t, credentialedGlobalConfig, local)

	connections := append(App.Connections(), models.Connection{Name: "staging", URL: "postgres://user:pass@staging/db"})
	if err := App.SaveConnections(connections); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, localPath); got != local {
		t.Errorf("local config changed:\n%s", got)
	}

	table := readTable(t, globalPath)
	databases, _ := table["database"].([]any)
	if len(databases) != 2 {
		t.Fatalf("global database = %v, want prod and staging", table["database"])
	}
	application, _ := table["application"].(map[string]any)
	if application["DefaultPageSize"] != int64(50) {
		t.Errorf("global application = %v, want DefaultPageSize kept", table["application"])
	}
	if _, ok := table["theme"]; ok {
		t.Errorf("global config gained the local [theme]: %v", table["theme"])
	}
}

func TestSaveConnectionsWithLocalConnectionsWritesLocalFile(t *testing.T) {
	local := "[theme]\nPreset = \"nord\"\n\n[[database]]\nName = \"dev\"\nURL = \"sqlite://dev.db\"\n"
	globalPath, localPath := loadProjectConfig(t, credentialedGlobalConfig, local)

	connections := append(App.Connections(), models.Connection{Name: "test", URL: "sqlite://test.db"})
	if err := App.SaveConnections(connections); err != nil {
		t.Fatal(err)
	}

	content := readFile(t, localPath)
	for _, leaked := range []string{"application", "ConfigFile", "LocalConfigFile", "user:pass", "prod"} {
		if strings.Contains(content, leaked) {
			t.Errorf("local config contains %q:\n%s", leaked, content)
		}
	}
	table := readTable(t, localPath)
	if databases, _ := table["database"].([]any); len(databases) != 2 {
		t.Errorf("local database = %v, want dev and test", table["database"])
	}
	if _, ok := table["theme"]; !ok {
		t.Error("local [theme] was dropped")
	}
	if got := readFile(t, globalPath); got != credentialedGlobalConfig {
		t.Errorf("global config changed:\n%s", got)
	}

	// Deleting every local connection keeps an empty list, so the global
	// connections do not reappear in this project.
	if err := App.SaveConnections(nil); err != nil {
		t.Fatal(err)
	}
	table = readTable(t, localPath)
	if databases, ok := table["database"].([]any); !ok || len(databases) != 0 {
		t.Errorf("local database = %#v, want an empty list", table["database"])
	}
}

func TestSaveGlobalConfigKeepsOwnContentAndDropsPathKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	stale := "ConfigFile = '/old/config.toml'\nLocalConfigFile = '/old/.lazysql.toml'\n\n" +
		"[application]\nDefaultPageSize = 50\n\n[[database]]\nName = \"prod\"\nURL = \"${env:PROD_URL}\"\n"
	if err := os.WriteFile(path, []byte(stale), 0o600); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.ConfigFile = path

	if err := config.SaveThemePreset("light"); err != nil {
		t.Fatal(err)
	}

	content := readFile(t, path)
	if strings.Contains(content, "ConfigFile") {
		t.Errorf("stale path keys were kept:\n%s", content)
	}
	if !strings.Contains(content, "${env:PROD_URL}") {
		t.Errorf("unrelated connection was not kept as written:\n%s", content)
	}
	if strings.Contains(content, "TreeWidth") {
		t.Errorf("application defaults were written into the file:\n%s", content)
	}
	table := readTable(t, path)
	theme, _ := table["theme"].(map[string]any)
	if theme["Preset"] != "light" {
		t.Errorf("theme = %v, want Preset light", table["theme"])
	}
}
