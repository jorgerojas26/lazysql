package components

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/lib"
)

type JSONViewer struct {
	*tview.Flex
	TextView         *tview.TextView
	Pages            *tview.Pages
	primitiveToFocus tview.Primitive
	wrapEnabled      bool
}

func NewJSONViewer(pages *tview.Pages) *JSONViewer {
	wrapEnabled := app.App.Config().JSONViewerWordWrap
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(wrapEnabled)
	textView.SetBorder(true).SetTitle(" JSON Viewer ")

	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(textView, 0, 4, true).
			AddItem(nil, 0, 1, false), 0, 4, true).
		AddItem(nil, 0, 1, false)

	jsonViewer := &JSONViewer{
		Flex:        flex,
		TextView:    textView,
		Pages:       pages,
		wrapEnabled: wrapEnabled,
	}

	pages.AddPage(pageNameJSONViewer, jsonViewer, true, false)

	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		command := app.Keymaps.Group(app.JSONViewerGroup).Resolve(event)

		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyEnter || command == commands.ShowCellJSONViewer || command == commands.ShowRowJSONViewer {
			jsonViewer.Hide()
			return nil
		} else if command == commands.Copy {
			clipboard := lib.NewClipboard()
			err := clipboard.Write(jsonViewer.TextView.GetText(true))
			if err != nil {
				logger.Error("Error copying JSON to clipboard", map[string]any{"error": err.Error()})
			}
			return nil
		} else if command == commands.ToggleJSONViewerWrap {
			jsonViewer.wrapEnabled = !jsonViewer.wrapEnabled
			jsonViewer.TextView.SetWrap(jsonViewer.wrapEnabled)
			return nil
		}
		return event
	})

	return jsonViewer
}

func (v *JSONViewer) Show(rowData map[string]string, focus tview.Primitive) {
	v.primitiveToFocus = focus

	structuredRowData := make(map[string]any)

	for key, value := range rowData {
		var jsonData any
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
		highlightedJSON := colorizeJSON(string(jsonData))
		v.TextView.SetText(highlightedJSON)
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

func colorizeJSON(jsonString string) string {
	var sb strings.Builder
	inString := false

	for i := 0; i < len(jsonString); i++ {
		char := jsonString[i]

		if inString {
			switch char {
			case '\\':
				err := sb.WriteByte(char)
				if err != nil {
					return sb.String()
				}

				if i+1 < len(jsonString) {
					err := sb.WriteByte(jsonString[i+1])
					if err != nil {
						return sb.String()
					}
					i++
				}
			case '"':
				err := sb.WriteByte(char)
				if err != nil {
					return sb.String()
				}
				inString = false
				_, err = sb.WriteString("[-]")
				if err != nil {
					return sb.String()
				}
			default:
				err := sb.WriteByte(char)
				if err != nil {
					return sb.String()
				}
			}
			continue
		}

		switch char {
		case '"':
			inString = true

			// Find the closing quote of the current string
			endQuoteIndex := -1
			for j := i + 1; j < len(jsonString); j++ {
				if jsonString[j] == '"' {
					// Count preceding backslashes to check if quote is escaped
					slashes := 0
					for k := j - 1; k > i; k-- {
						if jsonString[k] == '\\' {
							slashes++
						} else {
							break
						}
					}
					if slashes%2 == 0 {
						endQuoteIndex = j
						break
					}
				}
			}

			isKey := false
			if endQuoteIndex != -1 {
				// Look for a colon after the string
				for j := endQuoteIndex + 1; j < len(jsonString); j++ {
					if unicode.IsSpace(rune(jsonString[j])) {
						continue
					}
					if jsonString[j] == ':' {
						isKey = true
					}
					break
				}
			}

			if isKey {
				_, err := sb.WriteString("[#73B5AE]")
				if err != nil {
					return sb.String()
				}
			} else {
				_, err := sb.WriteString("[#3BC285]")
				if err != nil {
					return sb.String()
				}
			}
			err := sb.WriteByte(char)
			if err != nil {
				return sb.String()
			}

		case 't', 'f': // true, false
			if strings.HasPrefix(jsonString[i:], "true") {
				_, err := sb.WriteString("[#d3869b]true[-]")
				if err != nil {
					return sb.String()
				}
				i += 3
			} else if strings.HasPrefix(jsonString[i:], "false") {
				_, err := sb.WriteString("[#d3869b]false[-]")
				if err != nil {
					return sb.String()
				}
				i += 4
			} else {
				err := sb.WriteByte(char)
				if err != nil {
					return sb.String()
				}
			}
		case 'n': // null
			if strings.HasPrefix(jsonString[i:], "null") {
				_, err := sb.WriteString("[#458588]null[-]")
				if err != nil {
					return sb.String()
				}
				i += 3
			} else {
				err := sb.WriteByte(char)
				if err != nil {
					return sb.String()
				}
			}
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-':
			start := i
			for i+1 < len(jsonString) && (unicode.IsDigit(rune(jsonString[i+1])) || jsonString[i+1] == '.') {
				i++
			}
			_, err := sb.WriteString("[#83a598]")
			if err != nil {
				return sb.String()
			}
			_, err = sb.WriteString(jsonString[start : i+1])
			if err != nil {
				return sb.String()
			}
			_, err = sb.WriteString("[-]")
			if err != nil {
				return sb.String()
			}
		default:
			err := sb.WriteByte(char)
			if err != nil {
				return sb.String()
			}
		}
	}

	return sb.String()
}
