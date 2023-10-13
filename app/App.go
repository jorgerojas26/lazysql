package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var (
	App               = tview.NewApplication()
	InactiveTextColor = tcell.ColorDarkGray
	FocusTextColor    = tcell.ColorWhite.TrueColor()
	ActiveTextColor   = tcell.ColorCadetBlue
)
