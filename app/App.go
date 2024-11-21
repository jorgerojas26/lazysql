package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var App = tview.NewApplication()

type Theme struct {
	SidebarTitleBorderColor string
	tview.Theme
}

var Styles = Theme{
	SidebarTitleBorderColor: "#666A7E",
}

func init() {
	App.EnablePaste(true)

	theme := tview.Theme{
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
	}

	Styles.Theme = theme
	tview.Styles = theme
}
