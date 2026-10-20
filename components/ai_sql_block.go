package components

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rivo/uniseg"

	"github.com/jorgerojas26/lazysql/app"
)

func chatSQLBlock(sql string, width int) string {
	if width < 8 {
		return tview.Escape(sql)
	}
	inner := width - 4
	styles := tokenizeSQL(sql, app.Styles.PrimaryTextColor, app.Styles.PrimitiveBackgroundColor)
	var block, line, segment strings.Builder
	fmt.Fprintf(&block, "[%s]╭─ SQL %s╮[-]\n", app.Styles.SecondaryTextColor, strings.Repeat("─", width-8))
	cells, offset := 0, 0
	color := tcell.ColorDefault
	flushSegment := func() {
		if segment.Len() == 0 {
			return
		}
		fmt.Fprintf(&line, "[%s]%s", color, tview.Escape(segment.String()))
		segment.Reset()
	}
	flush := func() {
		flushSegment()
		fmt.Fprintf(&block, "[%s]│[-] %s[-]%s [%s]│[-]\n", app.Styles.SecondaryTextColor, line.String(), strings.Repeat(" ", max(0, inner-cells)), app.Styles.SecondaryTextColor)
		line.Reset()
		cells = 0
		color = tcell.ColorDefault
	}
	graphemes := uniseg.NewGraphemes(sql)
	for graphemes.Next() {
		cluster := graphemes.Str()
		next := offset + utf8.RuneCountInString(cluster)
		if cluster == "\n" || cluster == "\r\n" {
			flush()
			offset = next
			continue
		}
		fg := app.Styles.PrimaryTextColor
		if offset < len(styles) {
			fg, _, _ = styles[offset].Decompose()
		}
		size := graphemes.Width()
		if cluster == "\t" {
			size = 4 - cells%4
			cluster = strings.Repeat(" ", size)
		}
		if cells+size > inner && cells > 0 {
			flush()
		}
		if fg != color {
			flushSegment()
			color = fg
		}
		segment.WriteString(cluster)
		cells += size
		offset = next
	}
	flush()
	fmt.Fprintf(&block, "[%s]╰%s╯[-]", app.Styles.SecondaryTextColor, strings.Repeat("─", width-2))
	return block.String()
}
