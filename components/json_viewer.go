package components

import (
	"encoding/json"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

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
	textView.SetBorder(true).SetTitle("Press Esc or q to close")

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
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' || event.Key() == tcell.KeyEnter {
			jsonViewer.Hide()
			return nil
		}
		return event
	})

	return jsonViewer
}

func (v *JSONViewer) Show(rowData map[string]string, focus tview.Primitive) {
	v.primitiveToFocus = focus

	structuredRowData := make(map[string]interface{})

	for key, value := range rowData {
		var jsonData interface{}
		err := json.Unmarshal([]byte(value), &jsonData)
		if err == nil {
			structuredRowData[key] = jsonData
		} else {
			structuredRowData[key] = value
		}
	}

	jsonData, err := json.MarshalIndent(structuredRowData, "", "  ")
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
