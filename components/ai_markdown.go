package components

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

func chatMarkdown(source string, width int) string {
	source = strings.Map(func(character rune) rune {
		if unicode.IsControl(character) && character != '\n' && character != '\t' {
			return -1
		}
		return character
	}, strings.ReplaceAll(source, "\r\n", "\n"))
	lines := strings.Split(source, "\n")
	var output []string
	blank := func() {
		if len(output) > 0 && output[len(output)-1] != "" {
			output = append(output, "")
		}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			blank()
			continue
		}
		if hashes := len(trimmed) - len(strings.TrimLeft(trimmed, "#")); hashes > 0 && hashes <= 6 && len(trimmed) > hashes && (trimmed[hashes] == ' ' || trimmed[hashes] == '\t') {
			blank()
			output = append(output, markdownInline(strings.TrimSpace(trimmed[hashes:]), app.Styles.SecondaryTextColor, "b"))
			blank()
			continue
		}
		if prefix, content, ok := markdownList(line); ok {
			formatted := markdownInline(content, app.Styles.PrimaryTextColor, "")
			indent := tview.TaggedStringWidth(prefix)
			wrapped := tview.WordWrap(formatted, max(1, width-indent))
			for row, part := range wrapped {
				if row == 0 {
					output = append(output, prefix+part)
				} else {
					output = append(output, strings.Repeat(" ", indent)+part)
				}
			}
			continue
		}
		output = append(output, markdownInline(line, app.Styles.PrimaryTextColor, ""))
	}
	return strings.Trim(strings.Join(output, "\n"), "\n") + "[-:-:-]"
}

func markdownList(line string) (string, string, bool) {
	trimmed := strings.TrimLeft(line, " \t")
	indent := strings.ReplaceAll(line[:len(line)-len(trimmed)], "\t", "  ")
	indent = indent[:min(8, len(indent))]
	if len(trimmed) >= 2 && strings.ContainsRune("-*+", rune(trimmed[0])) && (trimmed[1] == ' ' || trimmed[1] == '\t') {
		return indent + "• ", strings.TrimSpace(trimmed[2:]), true
	}
	end := 0
	for end < len(trimmed) && end < 9 && trimmed[end] >= '0' && trimmed[end] <= '9' {
		end++
	}
	if end > 0 && end+1 < len(trimmed) && (trimmed[end] == '.' || trimmed[end] == ')') && (trimmed[end+1] == ' ' || trimmed[end+1] == '\t') {
		return indent + trimmed[:end] + ". ", strings.TrimSpace(trimmed[end+2:]), true
	}
	return "", "", false
}

func markdownStyle(color tcell.Color, attributes string) string {
	if attributes == "" {
		attributes = "-"
	}
	return fmt.Sprintf("[%s::%s]", color, attributes)
}

func markdownInline(source string, color tcell.Color, attributes string) string {
	var output strings.Builder
	output.WriteString(markdownStyle(color, attributes))
	bold := strings.Contains(attributes, "b")
	for len(source) > 0 {
		if source[0] == '\\' && len(source) > 1 && strings.ContainsRune("\\`*_#[]()+-~", rune(source[1])) {
			output.WriteString(tview.Escape(source[1:2]))
			source = source[2:]
			continue
		}
		if strings.HasPrefix(source, "***") {
			output.WriteString("***")
			source = source[3:]
			continue
		}
		if strings.HasPrefix(source, "**") && (bold || strings.Contains(source[2:], "**")) {
			bold = !bold
			attributes = ""
			if bold {
				attributes = "b"
			}
			output.WriteString(markdownStyle(color, attributes))
			source = source[2:]
			continue
		}
		if strings.HasPrefix(source, "``") {
			length := len(source) - len(strings.TrimLeft(source, "`"))
			end := len(source)
			if closing := strings.Index(source[length:], source[:length]); closing >= 0 {
				end = length + closing + length
			}
			output.WriteString(tview.Escape(source[:end]))
			source = source[end:]
			continue
		}
		if source[0] == '`' {
			if end := strings.IndexByte(source[1:], '`'); end >= 0 {
				output.WriteString(markdownStyle(app.Styles.SecondaryTextColor, attributes))
				output.WriteString(tview.Escape(source[1 : end+1]))
				output.WriteString(markdownStyle(color, attributes))
				source = source[end+2:]
				continue
			}
		}
		end := 1
		for end < len(source) && source[end] != '*' && source[end] != '`' && source[end] != '\\' {
			end++
		}
		output.WriteString(tview.Escape(source[:end]))
		source = source[end:]
	}
	return output.String() + "[-:-:-]"
}
