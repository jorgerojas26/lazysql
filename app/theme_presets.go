package app

// ThemePresets are the built-in themes selectable with [theme] Preset.
// Keys are the ones accepted in [theme.Colors].
//
// "default" is the original lazysql look and uses the terminal's own
// background. "light" is meant for light terminal backgrounds. The others
// use the official palettes of their color schemes and paint their own
// background; the pending-change backgrounds of the results table are the
// palette's accent colors blended into the background so text stays readable.
var ThemePresets = map[string]map[string]string{
	"default": {
		"PrimitiveBackground":    "default",
		"ContrastBackground":     "blue",
		"MoreContrastBackground": "green",
		"Border":                 "white",
		"Title":                  "white",
		"Graphics":               "gray",
		"PrimaryText":            "default",
		"SecondaryText":          "yellow",
		"TertiaryText":           "green",
		"InverseText":            "white",
		"ContrastSecondaryText":  "black",

		"SidebarTitleBorder": "#666A7E",
		"Error":              "red",
		"ReadOnly":           "lightblue",

		"TableChange": "orange",
		"TableInsert": "darkgreen",
		"TableDelete": "red",
		"TableMarked": "steelblue",

		"EditorSelection":           "darkcyan",
		"EditorStatusBarBackground": "darkslategray",
		"EditorStatusBarText":       "white",

		"AutocompleteBackground":  "darkslategray",
		"AutocompleteText":        "white",
		"AutocompleteSelected":    "dodgerblue",
		"AutocompleteDescription": "lightgray",
		"AutocompleteSeparator":   "gray",

		"SQLKeyword":   "dodgerblue",
		"SQLString":    "orange",
		"SQLNumber":    "limegreen",
		"SQLComment":   "gray",
		"SQLFunction":  "mediumpurple",
		"SQLOperator":  "darkorange",
		"SQLType":      "darkcyan",
		"SQLBoolean":   "orangered",
		"SQLParameter": "gold",

		"JSONKey":     "#73B5AE",
		"JSONString":  "#3BC285",
		"JSONBoolean": "#D3869B",
		"JSONNull":    "#458588",
		"JSONNumber":  "#83A598",
	},

	// For light terminal backgrounds. Keeps the terminal's background and
	// foreground, and uses dark accents that read on white.
	"light": {
		"PrimitiveBackground":    "default",
		"ContrastBackground":     "#EAEEF2",
		"MoreContrastBackground": "#D0D7DE",
		"Border":                 "#57606A",
		"Title":                  "#24292F",
		"Graphics":               "#8C959F",
		"PrimaryText":            "default",
		"SecondaryText":          "#9A6700",
		"TertiaryText":           "#1A7F37",
		"InverseText":            "#24292F",
		"ContrastSecondaryText":  "#FFFFFF",

		"SidebarTitleBorder": "#8C959F",
		"Error":              "#CF222E",
		"ReadOnly":           "#0969DA",

		"TableChange": "#FFDFB6",
		"TableInsert": "#ACEEBB",
		"TableDelete": "#FFCECB",
		"TableMarked": "#B6E3FF",

		"EditorSelection":           "#ADD6FF",
		"EditorStatusBarBackground": "#D0D7DE",
		"EditorStatusBarText":       "#24292F",

		"AutocompleteBackground":  "#EAEEF2",
		"AutocompleteText":        "#24292F",
		"AutocompleteSelected":    "#B6E3FF",
		"AutocompleteDescription": "#57606A",
		"AutocompleteSeparator":   "#8C959F",

		"SQLKeyword":   "#0550AE",
		"SQLString":    "#953800",
		"SQLNumber":    "#116329",
		"SQLComment":   "#6E7781",
		"SQLFunction":  "#8250DF",
		"SQLOperator":  "#BC4C00",
		"SQLType":      "#1B7C83",
		"SQLBoolean":   "#CF222E",
		"SQLParameter": "#9A6700",

		"JSONKey":     "#0550AE",
		"JSONString":  "#116329",
		"JSONBoolean": "#8250DF",
		"JSONNull":    "#6E7781",
		"JSONNumber":  "#953800",
	},

	// https://draculatheme.com/spec
	"dracula": {
		"PrimitiveBackground":    "#282A36",
		"ContrastBackground":     "#44475A",
		"MoreContrastBackground": "#6272A4",
		"Border":                 "#F8F8F2",
		"Title":                  "#F8F8F2",
		"Graphics":               "#6272A4",
		"PrimaryText":            "#F8F8F2",
		"SecondaryText":          "#F1FA8C",
		"TertiaryText":           "#50FA7B",
		"InverseText":            "#F8F8F2",
		"ContrastSecondaryText":  "#282A36",

		"SidebarTitleBorder": "#6272A4",
		"Error":              "#FF5555",
		"ReadOnly":           "#8BE9FD",

		"TableChange": "#735C49",
		"TableInsert": "#36734E",
		"TableDelete": "#733941",
		"TableMarked": "#6272A4",

		"EditorSelection":           "#44475A",
		"EditorStatusBarBackground": "#44475A",
		"EditorStatusBarText":       "#F8F8F2",

		"AutocompleteBackground":  "#44475A",
		"AutocompleteText":        "#F8F8F2",
		"AutocompleteSelected":    "#6272A4",
		"AutocompleteDescription": "#8BE9FD",
		"AutocompleteSeparator":   "#6272A4",

		"SQLKeyword":   "#FF79C6",
		"SQLString":    "#F1FA8C",
		"SQLNumber":    "#BD93F9",
		"SQLComment":   "#6272A4",
		"SQLFunction":  "#50FA7B",
		"SQLOperator":  "#FF79C6",
		"SQLType":      "#8BE9FD",
		"SQLBoolean":   "#BD93F9",
		"SQLParameter": "#FFB86C",

		"JSONKey":     "#8BE9FD",
		"JSONString":  "#F1FA8C",
		"JSONBoolean": "#BD93F9",
		"JSONNull":    "#BD93F9",
		"JSONNumber":  "#BD93F9",
	},

	// https://github.com/morhetz/gruvbox (dark, medium contrast)
	"gruvbox-dark": {
		"PrimitiveBackground":    "#282828",
		"ContrastBackground":     "#504945",
		"MoreContrastBackground": "#665C54",
		"Border":                 "#EBDBB2",
		"Title":                  "#EBDBB2",
		"Graphics":               "#928374",
		"PrimaryText":            "#EBDBB2",
		"SecondaryText":          "#FABD2F",
		"TertiaryText":           "#B8BB26",
		"InverseText":            "#FBF1C7",
		"ContrastSecondaryText":  "#282828",

		"SidebarTitleBorder": "#7C6F64",
		"Error":              "#FB4934",
		"ReadOnly":           "#83A598",

		"TableChange": "#734723",
		"TableInsert": "#5A5B27",
		"TableDelete": "#72342C",
		"TableMarked": "#458588",

		"EditorSelection":           "#504945",
		"EditorStatusBarBackground": "#3C3836",
		"EditorStatusBarText":       "#EBDBB2",

		"AutocompleteBackground":  "#504945",
		"AutocompleteText":        "#EBDBB2",
		"AutocompleteSelected":    "#458588",
		"AutocompleteDescription": "#A89984",
		"AutocompleteSeparator":   "#928374",

		"SQLKeyword":   "#FB4934",
		"SQLString":    "#B8BB26",
		"SQLNumber":    "#D3869B",
		"SQLComment":   "#928374",
		"SQLFunction":  "#8EC07C",
		"SQLOperator":  "#FE8019",
		"SQLType":      "#FABD2F",
		"SQLBoolean":   "#D3869B",
		"SQLParameter": "#83A598",

		"JSONKey":     "#83A598",
		"JSONString":  "#B8BB26",
		"JSONBoolean": "#FE8019",
		"JSONNull":    "#928374",
		"JSONNumber":  "#D3869B",
	},

	// https://ethanschoonover.com/solarized/
	"solarized-light": {
		"PrimitiveBackground":    "#FDF6E3",
		"ContrastBackground":     "#EEE8D5",
		"MoreContrastBackground": "#93A1A1",
		"Border":                 "#586E75",
		"Title":                  "#586E75",
		"Graphics":               "#93A1A1",
		"PrimaryText":            "#657B83",
		"SecondaryText":          "#B58900",
		"TertiaryText":           "#859900",
		"InverseText":            "#586E75",
		"ContrastSecondaryText":  "#FDF6E3",

		"SidebarTitleBorder": "#93A1A1",
		"Error":              "#DC322F",
		"ReadOnly":           "#268BD2",

		"TableChange": "#EEC3A5",
		"TableInsert": "#D9DA9F",
		"TableDelete": "#F3BBAD",
		"TableMarked": "#BCD6DE",

		"EditorSelection":           "#EEE8D5",
		"EditorStatusBarBackground": "#EEE8D5",
		"EditorStatusBarText":       "#586E75",

		"AutocompleteBackground":  "#EEE8D5",
		"AutocompleteText":        "#586E75",
		"AutocompleteSelected":    "#BCD6DE",
		"AutocompleteDescription": "#657B83",
		"AutocompleteSeparator":   "#93A1A1",

		"SQLKeyword":   "#859900",
		"SQLString":    "#2AA198",
		"SQLNumber":    "#D33682",
		"SQLComment":   "#93A1A1",
		"SQLFunction":  "#268BD2",
		"SQLOperator":  "#CB4B16",
		"SQLType":      "#B58900",
		"SQLBoolean":   "#6C71C4",
		"SQLParameter": "#DC322F",

		"JSONKey":     "#268BD2",
		"JSONString":  "#2AA198",
		"JSONBoolean": "#6C71C4",
		"JSONNull":    "#93A1A1",
		"JSONNumber":  "#D33682",
	},

	// https://www.nordtheme.com/docs/colors-and-palettes
	"nord": {
		"PrimitiveBackground":    "#2E3440",
		"ContrastBackground":     "#434C5E",
		"MoreContrastBackground": "#4C566A",
		"Border":                 "#D8DEE9",
		"Title":                  "#ECEFF4",
		"Graphics":               "#4C566A",
		"PrimaryText":            "#D8DEE9",
		"SecondaryText":          "#88C0D0",
		"TertiaryText":           "#A3BE8C",
		"InverseText":            "#ECEFF4",
		"ContrastSecondaryText":  "#2E3440",

		"SidebarTitleBorder": "#81A1C1",
		"Error":              "#BF616A",
		"ReadOnly":           "#81A1C1",

		"TableChange": "#6F5553",
		"TableInsert": "#5D6B5E",
		"TableDelete": "#684651",
		"TableMarked": "#5E81AC",

		"EditorSelection":           "#434C5E",
		"EditorStatusBarBackground": "#3B4252",
		"EditorStatusBarText":       "#D8DEE9",

		"AutocompleteBackground":  "#3B4252",
		"AutocompleteText":        "#ECEFF4",
		"AutocompleteSelected":    "#5E81AC",
		"AutocompleteDescription": "#D8DEE9",
		"AutocompleteSeparator":   "#616E88",

		"SQLKeyword":   "#81A1C1",
		"SQLString":    "#A3BE8C",
		"SQLNumber":    "#B48EAD",
		"SQLComment":   "#616E88",
		"SQLFunction":  "#88C0D0",
		"SQLOperator":  "#81A1C1",
		"SQLType":      "#8FBCBB",
		"SQLBoolean":   "#81A1C1",
		"SQLParameter": "#EBCB8B",

		"JSONKey":     "#8FBCBB",
		"JSONString":  "#A3BE8C",
		"JSONBoolean": "#81A1C1",
		"JSONNull":    "#81A1C1",
		"JSONNumber":  "#B48EAD",
	},

	// https://github.com/enkia/tokyo-night-vscode-theme
	"tokyo-night": {
		"PrimitiveBackground":    "#1A1B26",
		"ContrastBackground":     "#24283B",
		"MoreContrastBackground": "#414868",
		"Border":                 "#7AA2F7",
		"Title":                  "#C0CAF5",
		"Graphics":               "#565F89",
		"PrimaryText":            "#C0CAF5",
		"SecondaryText":          "#7AA2F7",
		"TertiaryText":           "#9ECE6A",
		"InverseText":            "#A9B1D6",
		"ContrastSecondaryText":  "#1A1B26",

		"SidebarTitleBorder": "#565F89",
		"Error":              "#F7768E",
		"ReadOnly":           "#7DCFFF",

		"TableChange": "#5C4934",
		"TableInsert": "#3D553B",
		"TableDelete": "#5C3545",
		"TableMarked": "#364A73",

		"EditorSelection":           "#33467C",
		"EditorStatusBarBackground": "#24283B",
		"EditorStatusBarText":       "#C0CAF5",

		"AutocompleteBackground":  "#24283B",
		"AutocompleteText":        "#C0CAF5",
		"AutocompleteSelected":    "#364A73",
		"AutocompleteDescription": "#A9B1D6",
		"AutocompleteSeparator":   "#565F89",

		"SQLKeyword":   "#BB9AF7",
		"SQLString":    "#9ECE6A",
		"SQLNumber":    "#FF9E64",
		"SQLComment":   "#565F89",
		"SQLFunction":  "#7AA2F7",
		"SQLOperator":  "#89DDFF",
		"SQLType":      "#2AC3DE",
		"SQLBoolean":   "#FF9E64",
		"SQLParameter": "#E0AF68",

		"JSONKey":     "#7DCFFF",
		"JSONString":  "#9ECE6A",
		"JSONBoolean": "#BB9AF7",
		"JSONNull":    "#565F89",
		"JSONNumber":  "#FF9E64",
	},

	// https://github.com/catppuccin/catppuccin (Mocha)
	"catppuccin-mocha": {
		"PrimitiveBackground":    "#1E1E2E",
		"ContrastBackground":     "#313244",
		"MoreContrastBackground": "#45475A",
		"Border":                 "#CBA6F7",
		"Title":                  "#F5C2E7",
		"Graphics":               "#6C7086",
		"PrimaryText":            "#CDD6F4",
		"SecondaryText":          "#CBA6F7",
		"TertiaryText":           "#A6E3A1",
		"InverseText":            "#BAC2DE",
		"ContrastSecondaryText":  "#1E1E2E",

		"SidebarTitleBorder": "#B4BEFE",
		"Error":              "#F38BA8",
		"ReadOnly":           "#89DCEB",

		"TableChange": "#594936",
		"TableInsert": "#3D513E",
		"TableDelete": "#573846",
		"TableMarked": "#574F73",

		"EditorSelection":           "#45475A",
		"EditorStatusBarBackground": "#313244",
		"EditorStatusBarText":       "#CDD6F4",

		"AutocompleteBackground":  "#313244",
		"AutocompleteText":        "#CDD6F4",
		"AutocompleteSelected":    "#574F73",
		"AutocompleteDescription": "#BAC2DE",
		"AutocompleteSeparator":   "#6C7086",

		"SQLKeyword":   "#CBA6F7",
		"SQLString":    "#A6E3A1",
		"SQLNumber":    "#FAB387",
		"SQLComment":   "#6C7086",
		"SQLFunction":  "#89B4FA",
		"SQLOperator":  "#89DCEB",
		"SQLType":      "#94E2D5",
		"SQLBoolean":   "#FAB387",
		"SQLParameter": "#F9E2AF",

		"JSONKey":     "#89DCEB",
		"JSONString":  "#A6E3A1",
		"JSONBoolean": "#CBA6F7",
		"JSONNull":    "#6C7086",
		"JSONNumber":  "#FAB387",
	},
}
