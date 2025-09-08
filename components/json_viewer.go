package components

import (
	"encoding/json"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const pageNameJSONViewer = "json_viewer"

type JSONViewer struct {
	*tview.Flex
	TextView         *tview.TextView
	Pages            *tview.Pages
	primitiveToFocus tview.Primitive
}

func NewJSONViewer(pages *tview.Pages) *JSONViewer {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(false)
	textView.SetBorder(true).SetTitle("Row Details (JSON) - Press Esc or q to close")

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(textView, 0, 4, true).
			AddItem(nil, 0, 1, false), 0, 4, true).
		AddItem(nil, 0, 1, false)

	jsonViewer := &JSONViewer{
		Flex:     flex,
		TextView: textView,
		Pages:    pages,
	}

	pages.AddPage(pageNameJSONViewer, jsonViewer, true, false)

	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			jsonViewer.Hide()
		}
		return event
	})

	return jsonViewer
}

func (v *JSONViewer) Show(rowData map[string]string, focus tview.Primitive) {
	v.primitiveToFocus = focus
	jsonData, err := json.MarshalIndent(rowData, "", "  ")
	if err != nil {
		v.TextView.SetText(fmt.Sprintf("Error: %v", err))
	} else {
		v.TextView.SetText(string(jsonData))
	}

	v.Pages.ShowPage(pageNameJSONViewer)
	App.SetFocus(v.TextView)
}

func (v *JSONViewer) Hide() {
	v.Pages.HidePage(pageNameJSONViewer)
	if v.primitiveToFocus != nil {
		App.SetFocus(v.primitiveToFocus)
	}
}
