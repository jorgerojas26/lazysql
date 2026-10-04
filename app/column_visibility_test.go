package app

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestHiddenColumnsRoundTripAndIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	application := &Application{config: &Config{ConfigFile: path}}
	cases := []struct {
		connection string
		database   string
		table      string
		hidden     []string
	}{
		{"prod.eu", "app.db", "public.users", []string{"secret", "odd[red]name"}},
		{"staging", "app.db", "public.users", []string{"id"}},
		{"prod.eu", "other", "public.users", []string{"email"}},
		{"prod.eu", "app.db", "archive.users", []string{"payload"}},
	}
	for _, test := range cases {
		if err := application.SaveHiddenColumns(test.connection, test.database, test.table, test.hidden); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	config := &Config{}
	if err := toml.Unmarshal(data, config); err != nil {
		t.Fatal(err)
	}
	reloaded := &Application{config: config}
	for _, test := range cases {
		got := reloaded.HiddenColumns(test.connection, test.database, test.table)
		if !reflect.DeepEqual(got, test.hidden) {
			t.Errorf("%s/%s/%s = %v, want %v", test.connection, test.database, test.table, got, test.hidden)
		}
		got[0] = "mutated"
		if !reflect.DeepEqual(reloaded.HiddenColumns(test.connection, test.database, test.table), test.hidden) {
			t.Error("getter exposed mutable configuration")
		}
	}
	if got := reloaded.HiddenColumns("missing", "db", "table"); len(got) != 0 {
		t.Errorf("new table should have all columns visible, got %v", got)
	}
}

func TestHiddenColumnsLocalOverrideAndShowAll(t *testing.T) {
	global := credentialedGlobalConfig + `
[hidden_columns.prod.db]
"public.users" = ["secret"]
"public.orders" = ["payload"]
`
	globalPath, localPath := loadProjectConfig(t, global, "[theme]\nPreset = \"nord\"\n")
	before := readFile(t, globalPath)
	if got := App.HiddenColumns("prod", "db", "public.users"); !reflect.DeepEqual(got, []string{"secret"}) {
		t.Fatalf("global preference not loaded: %v", got)
	}
	if err := App.SaveHiddenColumns("prod", "db", "public.users", nil); err != nil {
		t.Fatal(err)
	}
	assertNoMergedContent(t, localPath)
	local := readFile(t, localPath)
	if !strings.Contains(local, "public.users") || !strings.Contains(local, "[]") || strings.Contains(local, "public.orders") {
		t.Fatalf("local should contain only an explicit show-all override:\n%s", local)
	}
	if got := readFile(t, globalPath); got != before {
		t.Fatal("global configuration changed")
	}
	App.config = &Config{ConfigFile: globalPath}
	if err := LoadConfig(globalPath); err != nil {
		t.Fatal(err)
	}
	if got := App.HiddenColumns("prod", "db", "public.users"); len(got) != 0 {
		t.Errorf("show all did not override global preference: %v", got)
	}
	if got := App.HiddenColumns("prod", "db", "public.orders"); !reflect.DeepEqual(got, []string{"payload"}) {
		t.Errorf("unrelated inherited preference was lost: %v", got)
	}
}

func TestHiddenColumnsSavePreservesTemplatesAndOtherSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := `[application]
DefaultPageSize = 50
[theme]
Preset = "nord"
[[database]]
Name = "prod"
URL = "${env:DB_URL}"
[hidden_columns.prod.db]
orders = ["payload"]
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	application := &Application{config: &Config{ConfigFile: path}}
	columns := []string{"secret"}
	if err := application.SaveHiddenColumns("prod", "db", "users", columns); err != nil {
		t.Fatal(err)
	}
	columns[0] = "mutated"
	if got := application.HiddenColumns("prod", "db", "users"); !reflect.DeepEqual(got, []string{"secret"}) {
		t.Errorf("save retained caller's slice: %v", got)
	}
	values := readTable(t, path)
	if values["theme"].(map[string]any)["Preset"] != "nord" || values["application"].(map[string]any)["DefaultPageSize"] != int64(50) {
		t.Error("unrelated settings changed")
	}
	if !strings.Contains(readFile(t, path), "${env:DB_URL}") {
		t.Error("credential template lost")
	}
	hidden := values["hidden_columns"].(map[string]any)["prod"].(map[string]any)["db"].(map[string]any)
	if !reflect.DeepEqual(hidden["orders"], []any{"payload"}) {
		t.Errorf("unrelated table preference lost: %v", hidden)
	}
}

func TestHiddenColumnsSaveFailureKeepsMemoryUnchanged(t *testing.T) {
	application := &Application{config: &Config{
		ConfigFile: t.TempDir(),
		HiddenColumns: map[string]map[string]map[string][]string{
			"prod": {"db": {"users": {"secret"}}},
		},
	}}
	if err := application.SaveHiddenColumns("prod", "db", "users", []string{"id"}); err == nil {
		t.Fatal("saving to a directory should fail")
	}
	if got := application.HiddenColumns("prod", "db", "users"); !reflect.DeepEqual(got, []string{"secret"}) {
		t.Errorf("failed save changed active preference: %v", got)
	}
}
