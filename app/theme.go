package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DefaultThemePreset is the preset used when [theme] has no Preset.
const DefaultThemePreset = "default"

// ThemeConfig is the [theme] section of the config file.
type ThemeConfig struct {
	// Preset is the name of a built-in theme, see ThemePresets.
	Preset string `toml:"Preset,omitempty"`
	// Colors overrides individual colors of the preset. Keys are the
	// names listed in Theme.colors, values are anything tcell.GetColor
	// accepts (a color name, "#RRGGBB" or "default").
	Colors map[string]string `toml:"Colors,omitempty"`
}

type Theme struct {
	tview.Theme

	SidebarTitleBorderColor tcell.Color
	ErrorColor              tcell.Color
	ReadOnlyColor           tcell.Color

	// Background of rows/cells with pending changes in the results table.
	TableChangeColor tcell.Color
	TableInsertColor tcell.Color
	TableDeleteColor tcell.Color
	TableMarkedColor tcell.Color

	EditorSelectionColor           tcell.Color
	EditorStatusBarBackgroundColor tcell.Color
	EditorStatusBarTextColor       tcell.Color

	AutocompleteBackgroundColor  tcell.Color
	AutocompleteTextColor        tcell.Color
	AutocompleteSelectedColor    tcell.Color
	AutocompleteDescriptionColor tcell.Color
	AutocompleteSeparatorColor   tcell.Color

	SQLKeywordColor   tcell.Color
	SQLStringColor    tcell.Color
	SQLNumberColor    tcell.Color
	SQLCommentColor   tcell.Color
	SQLFunctionColor  tcell.Color
	SQLOperatorColor  tcell.Color
	SQLTypeColor      tcell.Color
	SQLBooleanColor   tcell.Color
	SQLParameterColor tcell.Color

	JSONKeyColor     tcell.Color
	JSONStringColor  tcell.Color
	JSONBooleanColor tcell.Color
	JSONNullColor    tcell.Color
	JSONNumberColor  tcell.Color
}

// colors maps each config key to the Theme field it sets.
func (t *Theme) colors() map[string]*tcell.Color {
	return map[string]*tcell.Color{
		"PrimitiveBackground":    &t.PrimitiveBackgroundColor,
		"ContrastBackground":     &t.ContrastBackgroundColor,
		"MoreContrastBackground": &t.MoreContrastBackgroundColor,
		"Border":                 &t.BorderColor,
		"Title":                  &t.TitleColor,
		"Graphics":               &t.GraphicsColor,
		"PrimaryText":            &t.PrimaryTextColor,
		"SecondaryText":          &t.SecondaryTextColor,
		"TertiaryText":           &t.TertiaryTextColor,
		"InverseText":            &t.InverseTextColor,
		"ContrastSecondaryText":  &t.ContrastSecondaryTextColor,

		"SidebarTitleBorder": &t.SidebarTitleBorderColor,
		"Error":              &t.ErrorColor,
		"ReadOnly":           &t.ReadOnlyColor,

		"TableChange": &t.TableChangeColor,
		"TableInsert": &t.TableInsertColor,
		"TableDelete": &t.TableDeleteColor,
		"TableMarked": &t.TableMarkedColor,

		"EditorSelection":           &t.EditorSelectionColor,
		"EditorStatusBarBackground": &t.EditorStatusBarBackgroundColor,
		"EditorStatusBarText":       &t.EditorStatusBarTextColor,

		"AutocompleteBackground":  &t.AutocompleteBackgroundColor,
		"AutocompleteText":        &t.AutocompleteTextColor,
		"AutocompleteSelected":    &t.AutocompleteSelectedColor,
		"AutocompleteDescription": &t.AutocompleteDescriptionColor,
		"AutocompleteSeparator":   &t.AutocompleteSeparatorColor,

		"SQLKeyword":   &t.SQLKeywordColor,
		"SQLString":    &t.SQLStringColor,
		"SQLNumber":    &t.SQLNumberColor,
		"SQLComment":   &t.SQLCommentColor,
		"SQLFunction":  &t.SQLFunctionColor,
		"SQLOperator":  &t.SQLOperatorColor,
		"SQLType":      &t.SQLTypeColor,
		"SQLBoolean":   &t.SQLBooleanColor,
		"SQLParameter": &t.SQLParameterColor,

		"JSONKey":     &t.JSONKeyColor,
		"JSONString":  &t.JSONStringColor,
		"JSONBoolean": &t.JSONBooleanColor,
		"JSONNull":    &t.JSONNullColor,
		"JSONNumber":  &t.JSONNumberColor,
	}
}

// set parses each value with tcell.GetColor and stores it in the matching
// field. Keys are matched case-insensitively.
func (t *Theme) set(values map[string]string) error {
	fields := map[string]*tcell.Color{}
	for key, field := range t.colors() {
		fields[strings.ToLower(key)] = field
	}

	seen := make(map[string]string, len(values))
	for key, value := range values {
		normalizedKey := strings.ToLower(key)
		if previousKey, ok := seen[normalizedKey]; ok {
			return fmt.Errorf("duplicate theme color keys %q and %q (keys are case-insensitive)", previousKey, key)
		}
		seen[normalizedKey] = key

		field, ok := fields[normalizedKey]
		if !ok {
			return fmt.Errorf("unknown theme color %q", key)
		}
		color, err := parseColor(value)
		if err != nil {
			return fmt.Errorf("theme color %s: %w", key, err)
		}
		*field = color
	}
	return nil
}

func parseColor(value string) (tcell.Color, error) {
	name := strings.ToLower(strings.TrimSpace(value))
	color := tcell.GetColor(name)
	// GetColor returns ColorDefault for anything it does not recognize.
	if color == tcell.ColorDefault && name != "default" {
		return color, fmt.Errorf("invalid color %q (use a color name, #RRGGBB or \"default\")", value)
	}
	return color, nil
}

// NewTheme builds the theme for a [theme] config: the preset's colors with
// the overrides from Colors applied on top.
func NewTheme(cfg ThemeConfig) (*Theme, error) {
	preset := cfg.Preset
	if preset == "" {
		preset = DefaultThemePreset
	}
	colors, ok := ThemePresets[strings.ToLower(preset)]
	if !ok {
		return nil, fmt.Errorf("unknown theme preset %q (available: %s)", cfg.Preset, strings.Join(ThemePresetNames(), ", "))
	}

	theme := &Theme{}
	// Every preset lists every color, but start from the default one so a
	// preset missing a key never leaves a color unset.
	if err := theme.set(ThemePresets[DefaultThemePreset]); err != nil {
		return nil, err
	}
	if err := theme.set(colors); err != nil {
		return nil, err
	}
	if err := theme.set(cfg.Colors); err != nil {
		return nil, err
	}
	return theme, nil
}

// ApplyTheme makes the theme described by cfg the active one.
// It must run before any component is created.
func ApplyTheme(cfg ThemeConfig) error {
	theme, err := NewTheme(cfg)
	if err != nil {
		return err
	}
	Styles = theme
	tview.Styles = theme.Theme
	return nil
}

// ThemePresetNames returns the names of the built-in presets, sorted.
func ThemePresetNames() []string {
	names := make([]string, 0, len(ThemePresets))
	for name := range ThemePresets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
