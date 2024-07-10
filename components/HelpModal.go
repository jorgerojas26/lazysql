package components

import (
	"github.com/gdamore/tcell/v2"
	app "github.com/jorgerojas26/lazysql/app"
	. "github.com/jorgerojas26/lazysql/keymap"
	"github.com/rivo/tview"
)

type HelpModal struct {
	*tview.Flex
}

func NewHelpModal() *HelpModal {

	list := tview.NewList().SetSelectedBackgroundColor(tcell.ColorBlue)

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(list, 0, 5, true)

	flex.SetBorder(true)
	flex.SetTitle("Help")
	flex.SetBorderColor(tcell.ColorGreen)

	//Magic Number gain from trial and error
	screenWidth, screenHeight := 145, 30

	modalWidth := 50
	modalHeight := 20
	x := (screenWidth - modalWidth) / 2
	y := (screenHeight - modalHeight) / 2

	flex.SetRect(x, y, modalWidth, modalHeight)

	r := &HelpModal{flex}

	r.drawgroup(list, "Global", app.Keymaps.Global)
	for k, v := range app.Keymaps.Groups {
		r.drawgroup(list, k, v)
	}
	return r
}
func (modal HelpModal) drawgroup(outtext *tview.List, groupname string, keys Map) {

	outtext.AddItem("", "---"+groupname+"---", rune(0), nil)

	for _, key := range keys {
		outtext.AddItem(key.Key.String()+":"+key.Description, "", rune(0), nil)
	}
}
