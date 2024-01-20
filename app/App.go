package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var App = tview.NewApplication()

func init() {
	Styles := tview.Theme{
		PrimitiveBackgroundColor:    tcell.ColorDefault,
		ContrastBackgroundColor:     tcell.ColorBlue,
		MoreContrastBackgroundColor: tcell.ColorGreen,
		BorderColor:                 tcell.ColorWhite,
		TitleColor:                  tcell.ColorWhite,
		GraphicsColor:               tcell.ColorWhite,
		PrimaryTextColor:            tcell.ColorWhite.TrueColor(),
		SecondaryTextColor:          tcell.ColorCadetBlue,
		TertiaryTextColor:           tcell.ColorGreen,
		InverseTextColor:            tcell.ColorDarkGray,
		ContrastSecondaryTextColor:  tcell.ColorNavy,
	}

	tview.Styles = Styles
}
