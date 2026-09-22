package app

import (
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// runtimeThemeRenderer recolors already-created primitives after they draw.
// tview copies global styles into primitives at construction time, so updating
// tview.Styles alone is not enough for an in-place theme switch.
type runtimeThemeRenderer struct {
	mu      sync.RWMutex
	sources []*Theme
	target  *Theme
	fg      map[tcell.Color]tcell.Color
	bg      map[tcell.Color]tcell.Color
}

var themeRenderer runtimeThemeRenderer

// BeginThemePreview records the theme used by currently existing primitives.
func (a *Application) BeginThemePreview() {
	themeRenderer.addSource(Styles)
}

// PreviewTheme applies a theme in memory and recolors the current UI on its
// next draw. The caller decides whether to persist or restore it.
func (a *Application) PreviewTheme(cfg ThemeConfig) error {
	theme, err := NewTheme(cfg)
	if err != nil {
		return err
	}

	// Async UI work may create or restyle primitives while the picker is open.
	// Remember every displayed theme so those colors can also be translated when
	// the user moves again or cancels the preview.
	themeRenderer.addSource(Styles)
	Styles = theme
	tview.Styles = theme.Theme
	themeRenderer.setTarget(theme)
	return nil
}

func (r *runtimeThemeRenderer) addSource(theme *Theme) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, source := range r.sources {
		if source == theme {
			return
		}
	}
	r.sources = append(r.sources, theme)
}

func (r *runtimeThemeRenderer) setTarget(target *Theme) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.target = target
	r.fg = make(map[tcell.Color]tcell.Color)
	r.bg = make(map[tcell.Color]tcell.Color)

	targetForegrounds := themeForegrounds(target)
	targetBackgrounds := themeBackgrounds(target)
	for _, source := range r.sources {
		for index, color := range themeForegrounds(source) {
			r.fg[color] = targetForegrounds[index]
		}
		for index, color := range themeBackgrounds(source) {
			r.bg[color] = targetBackgrounds[index]
		}
	}

	// Primitives created after a theme was applied already contain target
	// colors and must pass through unchanged.
	for _, color := range targetForegrounds {
		r.fg[color] = color
	}
	for _, color := range targetBackgrounds {
		r.bg[color] = color
	}
}

func (r *runtimeThemeRenderer) recolor(screen tcell.Screen) {
	r.mu.RLock()
	if r.target == nil {
		r.mu.RUnlock()
		return
	}
	fgMap := r.fg
	bgMap := r.bg
	r.mu.RUnlock()

	width, height := screen.Size()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			mainc, combc, style, cellWidth := screen.GetContent(x, y)
			if cellWidth == 0 {
				continue
			}
			fg, bg, _ := style.Decompose()
			newFg, fgChanged := fgMap[fg]
			newBg, bgChanged := bgMap[bg]
			if !fgChanged && !bgChanged {
				continue
			}
			if fgChanged {
				style = style.Foreground(newFg)
			}
			if bgChanged {
				style = style.Background(newBg)
			}
			screen.SetContent(x, y, mainc, combc, style)
		}
	}
}

// The order is from low to high priority when multiple semantic roles share
// the same source color.
func themeForegrounds(theme *Theme) []tcell.Color {
	return []tcell.Color{
		theme.GraphicsColor,
		theme.BorderColor,
		theme.TitleColor,
		theme.ErrorColor,
		theme.ReadOnlyColor,
		theme.AutocompleteSeparatorColor,
		theme.AutocompleteDescriptionColor,
		theme.AutocompleteTextColor,
		theme.EditorStatusBarTextColor,
		theme.JSONKeyColor,
		theme.JSONStringColor,
		theme.JSONBooleanColor,
		theme.JSONNullColor,
		theme.JSONNumberColor,
		theme.SQLKeywordColor,
		theme.SQLStringColor,
		theme.SQLNumberColor,
		theme.SQLCommentColor,
		theme.SQLFunctionColor,
		theme.SQLOperatorColor,
		theme.SQLTypeColor,
		theme.SQLBooleanColor,
		theme.SQLParameterColor,
		theme.ContrastSecondaryTextColor,
		theme.InverseTextColor,
		theme.TertiaryTextColor,
		theme.SecondaryTextColor,
		theme.PrimaryTextColor,
	}
}

func themeBackgrounds(theme *Theme) []tcell.Color {
	return []tcell.Color{
		theme.ErrorColor,
		theme.InverseTextColor,
		theme.PrimaryTextColor,
		theme.SecondaryTextColor,
		theme.EditorSelectionColor,
		theme.EditorStatusBarBackgroundColor,
		theme.AutocompleteBackgroundColor,
		theme.AutocompleteSelectedColor,
		theme.TableChangeColor,
		theme.TableInsertColor,
		theme.TableDeleteColor,
		theme.TableMarkedColor,
		theme.MoreContrastBackgroundColor,
		theme.ContrastBackgroundColor,
		theme.PrimitiveBackgroundColor,
	}
}
