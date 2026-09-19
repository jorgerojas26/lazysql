package app

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/pelletier/go-toml/v2"
	"github.com/rivo/tview"
)

// The default preset must reproduce the colors lazysql used before themes
// existed: the tview.Theme set in init() and the literals in components.
func TestDefaultThemeMatchesOriginalColors(t *testing.T) {
	want := Theme{
		Theme: tview.Theme{
			PrimitiveBackgroundColor:    tcell.ColorDefault,
			ContrastBackgroundColor:     tcell.ColorBlue,
			MoreContrastBackgroundColor: tcell.ColorGreen,
			BorderColor:                 tcell.ColorWhite,
			TitleColor:                  tcell.ColorWhite,
			GraphicsColor:               tcell.ColorGray,
			PrimaryTextColor:            tcell.ColorDefault.TrueColor(),
			SecondaryTextColor:          tcell.ColorYellow,
			TertiaryTextColor:           tcell.ColorGreen,
			InverseTextColor:            tcell.ColorWhite,
			ContrastSecondaryTextColor:  tcell.ColorBlack,
		},
		SidebarTitleBorderColor: tcell.GetColor("#666A7E"),
		ErrorColor:              tcell.ColorRed,
		ReadOnlyColor:           tcell.ColorLightBlue,

		TableChangeColor: tcell.ColorOrange,
		TableInsertColor: tcell.ColorDarkGreen,
		TableDeleteColor: tcell.ColorRed,
		TableMarkedColor: tcell.ColorSteelBlue,

		EditorSelectionColor:           tcell.ColorDarkCyan,
		EditorStatusBarBackgroundColor: tcell.ColorDarkSlateGray,
		EditorStatusBarTextColor:       tcell.ColorWhite,

		AutocompleteBackgroundColor:  tcell.ColorDarkSlateGray,
		AutocompleteTextColor:        tcell.ColorWhite,
		AutocompleteSelectedColor:    tcell.ColorDodgerBlue,
		AutocompleteDescriptionColor: tcell.ColorLightGray,
		AutocompleteSeparatorColor:   tcell.ColorGray,

		SQLKeywordColor:   tcell.ColorDodgerBlue,
		SQLStringColor:    tcell.ColorOrange,
		SQLNumberColor:    tcell.ColorLimeGreen,
		SQLCommentColor:   tcell.ColorGray,
		SQLFunctionColor:  tcell.ColorMediumPurple,
		SQLOperatorColor:  tcell.ColorDarkOrange,
		SQLTypeColor:      tcell.ColorDarkCyan,
		SQLBooleanColor:   tcell.ColorOrangeRed,
		SQLParameterColor: tcell.ColorGold,

		JSONKeyColor:     tcell.GetColor("#73B5AE"),
		JSONStringColor:  tcell.GetColor("#3BC285"),
		JSONBooleanColor: tcell.GetColor("#d3869b"),
		JSONNullColor:    tcell.GetColor("#458588"),
		JSONNumberColor:  tcell.GetColor("#83a598"),
	}

	got, err := NewTheme(ThemeConfig{})
	if err != nil {
		t.Fatalf("NewTheme() error = %v", err)
	}
	if *got != want {
		t.Errorf("default theme = %+v\nwant %+v", *got, want)
	}
}

func TestThemePresetsAreComplete(t *testing.T) {
	keys := (&Theme{}).colors()

	for name, colors := range ThemePresets {
		t.Run(name, func(t *testing.T) {
			if len(colors) != len(keys) {
				t.Errorf("preset has %d colors, want %d", len(colors), len(keys))
			}
			for key := range keys {
				if _, ok := colors[key]; !ok {
					t.Errorf("preset is missing %s", key)
				}
			}

			theme, err := NewTheme(ThemeConfig{Preset: name})
			if err != nil {
				t.Fatalf("NewTheme() error = %v", err)
			}
			// The results table tells pending changes apart by background.
			pending := map[tcell.Color]string{}
			for key, color := range map[string]tcell.Color{
				"TableChange": theme.TableChangeColor,
				"TableInsert": theme.TableInsertColor,
				"TableDelete": theme.TableDeleteColor,
			} {
				if other, ok := pending[color]; ok {
					t.Errorf("%s and %s share the same color", key, other)
				}
				pending[color] = key
			}
		})
	}
}

func TestNewThemeOverrides(t *testing.T) {
	theme, err := NewTheme(ThemeConfig{
		Preset: "Light",
		Colors: map[string]string{
			"Border":                    "#666A7E",
			"Title":                     "Black",
			"PrimaryText":               "default",
			"autocompleteselected":      "dodgerblue",
			"EditorStatusBarBackground": "lightgray",
			"SQLKeyword":                "#005F87",
		},
	})
	if err != nil {
		t.Fatalf("NewTheme() error = %v", err)
	}

	light, err := NewTheme(ThemeConfig{Preset: "light"})
	if err != nil {
		t.Fatalf("NewTheme() error = %v", err)
	}

	checks := []struct {
		name string
		got  tcell.Color
		want tcell.Color
	}{
		{"Border", theme.BorderColor, tcell.NewHexColor(0x666A7E)},
		{"Title", theme.TitleColor, tcell.ColorBlack},
		{"PrimaryText", theme.PrimaryTextColor, tcell.ColorDefault},
		{"AutocompleteSelected", theme.AutocompleteSelectedColor, tcell.ColorDodgerBlue},
		{"EditorStatusBarBackground", theme.EditorStatusBarBackgroundColor, tcell.ColorLightGray},
		{"SQLKeyword", theme.SQLKeywordColor, tcell.NewHexColor(0x005F87)},
		// Not overridden: comes from the preset.
		{"SecondaryText", theme.SecondaryTextColor, light.SecondaryTextColor},
		{"SQLString", theme.SQLStringColor, light.SQLStringColor},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestNewThemeErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ThemeConfig
		wantErr string
	}{
		{
			name:    "unknown preset",
			cfg:     ThemeConfig{Preset: "monokai"},
			wantErr: `unknown theme preset "monokai"`,
		},
		{
			name:    "unknown color key",
			cfg:     ThemeConfig{Colors: map[string]string{"Bordr": "red"}},
			wantErr: `unknown theme color "Bordr"`,
		},
		{
			name:    "invalid color name",
			cfg:     ThemeConfig{Colors: map[string]string{"Border": "not-a-color"}},
			wantErr: `theme color Border: invalid color "not-a-color"`,
		},
		{
			name:    "invalid hex",
			cfg:     ThemeConfig{Colors: map[string]string{"Title": "#12345"}},
			wantErr: `theme color Title: invalid color "#12345"`,
		},
		{
			name:    "empty value",
			cfg:     ThemeConfig{Colors: map[string]string{"Title": ""}},
			wantErr: `theme color Title: invalid color ""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewTheme(tt.cfg)
			if err == nil {
				t.Fatal("NewTheme() error = nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("NewTheme() error = %q, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfigAppliesTheme(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	origConfig := App.config
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
		App.config = origConfig
		if err := ApplyTheme(ThemeConfig{}); err != nil {
			t.Error(err)
		}
	})

	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}

	globalConfig := `
[theme]
Preset = "dracula"

[theme.Colors]
Border = "red"
Title = "blue"
`
	globalPath := filepath.Join(tmpDir, "config.toml")
	if err := os.WriteFile(globalPath, []byte(globalConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	// The local config only overrides one color; the preset and the other
	// global override must survive the merge.
	localConfig := `
[theme.Colors]
Title = "green"
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".lazysql.toml"), []byte(localConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	App.config = &Config{ConfigFile: globalPath}
	if err := LoadConfig(globalPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	dracula, err := NewTheme(ThemeConfig{Preset: "dracula"})
	if err != nil {
		t.Fatal(err)
	}
	if Styles.PrimitiveBackgroundColor != dracula.PrimitiveBackgroundColor {
		t.Errorf("PrimitiveBackgroundColor = %v, want dracula's %v", Styles.PrimitiveBackgroundColor, dracula.PrimitiveBackgroundColor)
	}
	if Styles.BorderColor != tcell.ColorRed {
		t.Errorf("BorderColor = %v, want red (global override)", Styles.BorderColor)
	}
	if Styles.TitleColor != tcell.ColorGreen {
		t.Errorf("TitleColor = %v, want green (local override)", Styles.TitleColor)
	}
	if tview.Styles != Styles.Theme {
		t.Error("tview.Styles was not updated")
	}

	App.config = &Config{ConfigFile: globalPath}
	if err := os.WriteFile(filepath.Join(tmpDir, ".lazysql.toml"), []byte("[theme]\nPreset = \"nope\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := LoadConfig(globalPath); err == nil {
		t.Error("LoadConfig() with an unknown preset: error = nil")
	}
}

// Saving connections must not add a [theme] section to a config without one.
func TestThemeNotWrittenWhenUnset(t *testing.T) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(&Config{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "theme") {
		t.Errorf("encoded config contains a theme section:\n%s", buf.String())
	}
}
