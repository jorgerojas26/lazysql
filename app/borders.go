package app

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// BorderConfig is the [borders] section. Empty colors use the active theme.
// An explicitly empty value in a local file also resets a global override.
type BorderConfig struct {
	Style       string `toml:"style,omitempty"`
	NormalColor string `toml:"normal_color,omitempty"`
	FocusColor  string `toml:"focus_color,omitempty"`
}

var activeBorders = BorderConfig{Style: "auto"}
var originalTviewBorders = tview.Borders
var tableNormalMarker = tcell.NewHexColor(0x010203)
var tableFocusMarker = tcell.NewHexColor(0x040506)

// TableBorderMarker lets tables retain their focus role even when a theme uses
// the same color for borders and text.
func TableBorderMarker(focused bool) tcell.Color {
	if focused {
		return tableFocusMarker
	}
	return tableNormalMarker
}

func normalizeBorders(cfg BorderConfig) (BorderConfig, error) {
	cfg.Style = strings.ToLower(strings.TrimSpace(cfg.Style))
	if cfg.Style == "" {
		cfg.Style = "auto"
	}
	switch cfg.Style {
	case "auto", "single", "double":
	default:
		return cfg, fmt.Errorf("invalid border style %q (use auto, single or double)", cfg.Style)
	}
	for _, item := range []struct{ name, value string }{{"normal_color", cfg.NormalColor}, {"focus_color", cfg.FocusColor}} {
		if item.value != "" {
			if _, err := parseColor(item.value); err != nil {
				return cfg, fmt.Errorf("borders %s: %w", item.name, err)
			}
		}
	}
	return cfg, nil
}

func (a *Application) BorderConfig() BorderConfig { return activeBorders }

// PreviewBorders changes only runtime presentation; save methods persist it.
func (a *Application) PreviewBorders(cfg BorderConfig) error {
	validated, err := normalizeBorders(cfg)
	if err != nil {
		return err
	}
	activeBorders = validated
	// Private runes mark only borders emitted by tview. This keeps SQL text
	// containing box-drawing characters untouched by the screen pass.
	if validated.Style == "auto" && validated.NormalColor == "" && validated.FocusColor == "" {
		tview.Borders = originalTviewBorders
	} else {
		tview.Borders.Horizontal, tview.Borders.Vertical = '\ue000', '\ue001'
		tview.Borders.TopLeft, tview.Borders.TopRight = '\ue002', '\ue003'
		tview.Borders.BottomLeft, tview.Borders.BottomRight = '\ue004', '\ue005'
		tview.Borders.LeftT, tview.Borders.RightT = '\ue006', '\ue007'
		tview.Borders.TopT, tview.Borders.BottomT = '\ue008', '\ue009'
		tview.Borders.Cross = '\ue00a'
		tview.Borders.HorizontalFocus, tview.Borders.VerticalFocus = '\ue00b', '\ue00c'
		tview.Borders.TopLeftFocus, tview.Borders.TopRightFocus = '\ue00d', '\ue00e'
		tview.Borders.BottomLeftFocus, tview.Borders.BottomRightFocus = '\ue00f', '\ue010'
	}
	return nil
}

func (a *Application) SaveBorderStyle(style string) error {
	previous := activeBorders
	cfg := activeBorders
	cfg.Style = style
	if err := a.PreviewBorders(cfg); err != nil {
		return err
	}
	if err := a.config.saveBorderField("style", activeBorders.Style); err != nil {
		_ = a.PreviewBorders(previous)
		return err
	}
	a.config.Borders = &cfg
	return nil
}

func (a *Application) SaveBorderColors(normal, focus string) error {
	previous := activeBorders
	cfg := activeBorders
	cfg.NormalColor, cfg.FocusColor = normal, focus
	if err := a.PreviewBorders(cfg); err != nil {
		return err
	}
	if err := a.config.saveBorderFields(map[string]string{"normal_color": normal, "focus_color": focus}); err != nil {
		_ = a.PreviewBorders(previous)
		return err
	}
	a.config.Borders = &cfg
	return nil
}

func NextBorderStyle(style string) string {
	switch style {
	case "auto":
		return "single"
	case "single":
		return "double"
	default:
		return "auto"
	}
}

// drawBorders runs after tview and the theme renderer. tview's original
// normal/focus runes remain the source of focus information even when
// both states are displayed with the same line style.
func drawBorders(screen tcell.Screen) {
	cfg := activeBorders
	normalColor, focusColor := tcell.ColorDefault, tcell.ColorDefault
	if cfg.NormalColor != "" {
		normalColor, _ = parseColor(cfg.NormalColor)
	}
	if cfg.FocusColor != "" {
		focusColor, _ = parseColor(cfg.FocusColor)
	}
	w, h := screen.Size()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ch, comb, style, width := screen.GetContent(x, y)
			if width == 0 {
				continue
			}
			light, double, focused, isBorder := markedBorder(ch)
			if !isBorder {
				fg, _, _ := style.Decompose()
				if fg != tableNormalMarker && fg != tableFocusMarker {
					continue
				}
				var ok bool
				light, double, ok = borderPair(ch)
				if !ok {
					continue
				}
				focused = fg == tableFocusMarker
			}
			fg, _, _ := style.Decompose()
			if fg == tableFocusMarker || fg == tableNormalMarker {
				focused = fg == tableFocusMarker
				if focused {
					style = style.Foreground(Styles.PrimaryTextColor)
				} else {
					style = style.Foreground(Styles.InverseTextColor)
				}
			}
			if focused {
				ch = double
			} else {
				ch = light
			}
			switch cfg.Style {
			case "single":
				ch = light
			case "double":
				ch = double
			}
			if focused && cfg.FocusColor != "" {
				style = style.Foreground(focusColor)
			} else if !focused && cfg.NormalColor != "" {
				style = style.Foreground(normalColor)
			}
			screen.SetContent(x, y, ch, comb, style)
		}
	}
}

func borderPair(ch rune) (rune, rune, bool) {
	// Include table intersections and manually drawn autocomplete borders.
	switch ch {
	case '─', '═':
		return '─', '═', true
	case '│', '║':
		return '│', '║', true
	case '┌', '╔':
		return '┌', '╔', true
	case '┐', '╗':
		return '┐', '╗', true
	case '└', '╚':
		return '└', '╚', true
	case '┘', '╝':
		return '┘', '╝', true
	case '├', '╠':
		return '├', '╠', true
	case '┤', '╣':
		return '┤', '╣', true
	case '┬', '╦':
		return '┬', '╦', true
	case '┴', '╩':
		return '┴', '╩', true
	case '┼', '╬':
		return '┼', '╬', true
	}
	return 0, 0, false
}

func markedBorder(ch rune) (light, double rune, focused, ok bool) {
	switch ch {
	case '\ue000', '\ue00b':
		light, double = '─', '═'
	case '\ue001', '\ue00c':
		light, double = '│', '║'
	case '\ue002', '\ue00d':
		light, double = '┌', '╔'
	case '\ue003', '\ue00e':
		light, double = '┐', '╗'
	case '\ue004', '\ue00f':
		light, double = '└', '╚'
	case '\ue005', '\ue010':
		light, double = '┘', '╝'
	case '\ue006':
		light, double = '├', '╠'
	case '\ue007':
		light, double = '┤', '╣'
	case '\ue008':
		light, double = '┬', '╦'
	case '\ue009':
		light, double = '┴', '╩'
	case '\ue00a':
		light, double = '┼', '╬'
	default:
		return 0, 0, false, false
	}
	return light, double, ch >= '\ue00b', true
}

// BorderRune and BorderForeground cover borders drawn directly by LazySQL.
func BorderRune(ch rune) rune {
	light, double, ok := borderPair(ch)
	if !ok {
		return ch
	}
	if activeBorders.Style == "double" {
		return double
	}
	return light
}

func BorderForeground(base tcell.Color, focused bool) tcell.Color {
	value := activeBorders.NormalColor
	if focused {
		value = activeBorders.FocusColor
	}
	if value == "" {
		return base
	}
	color, _ := parseColor(value)
	return color
}
