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
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			blank()
			continue
		}
		if marker, language := markdownFence(trimmed); marker != "" {
			start := index + 1
			for index++; index < len(lines); index++ {
				candidate := strings.TrimSpace(lines[index])
				if strings.HasPrefix(candidate, marker) && strings.Trim(candidate, marker[:1]) == "" {
					break
				}
			}
			code := strings.Join(lines[start:index], "\n")
			blank()
			switch strings.ToLower(language) {
			case "sql", "mysql", "postgres", "postgresql", "sqlite", "sqlserver", "mssql", "clickhouse":
				output = append(output, chatSQLBlock(code, width))
			default:
				for _, codeLine := range strings.Split(code, "\n") {
					wrapped := tview.WordWrap(tview.Escape(codeLine), max(1, width-2))
					if len(wrapped) == 0 {
						wrapped = []string{""}
					}
					for _, part := range wrapped {
						output = append(output, "│ "+markdownStyle(app.Styles.SecondaryTextColor, "")+part+"[-:-:-]")
					}
				}
			}
			blank()
			continue
		}
		if hashes := len(trimmed) - len(strings.TrimLeft(trimmed, "#")); hashes > 0 && hashes <= 6 && len(trimmed) > hashes && (trimmed[hashes] == ' ' || trimmed[hashes] == '\t') {
			blank()
			output = append(output, markdownInline(strings.TrimSpace(trimmed[hashes:]), app.Styles.SecondaryTextColor, "b", 0))
			blank()
			continue
		}
		if prefix, content, ok := markdownList(line); ok {
			formatted := markdownInline(content, app.Styles.PrimaryTextColor, "", 0)
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
		if strings.HasPrefix(trimmed, "> ") {
			output = append(output, "│ "+markdownInline(trimmed[2:], app.Styles.PrimaryTextColor, "", 0))
			continue
		}
		output = append(output, markdownInline(line, app.Styles.PrimaryTextColor, "", 0))
	}
	return strings.Trim(strings.Join(output, "\n"), "\n") + "[-:-:-]"
}

func markdownFence(line string) (string, string) {
	if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
		return "", ""
	}
	length := 0
	for length < len(line) && line[length] == line[0] {
		length++
	}
	if length < 3 {
		return "", ""
	}
	language := strings.TrimSpace(line[length:])
	if line[0] == '`' && strings.ContainsRune(language, '`') {
		return "", ""
	}
	return line[:length], language
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

func markdownWord(character byte) bool {
	return character >= 128 || character == '_' || character >= '0' && character <= '9' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

func markdownInline(source string, color tcell.Color, attributes string, depth int) string {
	base := markdownStyle(color, attributes)
	if depth >= 8 {
		return base + tview.Escape(source)
	}
	var output, literal strings.Builder
	output.WriteString(base)
	flush := func() { output.WriteString(tview.Escape(literal.String())); literal.Reset() }
	for index := 0; index < len(source); {
		character := source[index]
		if character == '\\' && index+1 < len(source) && strings.ContainsRune("\\`*_#[]()+-~", rune(source[index+1])) {
			literal.WriteByte(source[index+1])
			index += 2
			continue
		}
		if character != '`' && character != '*' && character != '_' || character == '_' && index > 0 && markdownWord(source[index-1]) {
			literal.WriteByte(character)
			index++
			continue
		}
		length := 1
		if character == '`' {
			for index+length < len(source) && source[index+length] == '`' {
				length++
			}
		} else if index+1 < len(source) && source[index+1] == character {
			length = 2
			if index+2 < len(source) && source[index+2] == character {
				length = 3
			}
		}
		marker := source[index : index+length]
		start, end := index+length, -1
		if character != '`' && (start == len(source) || source[start] == ' ' || source[start] == '\t') {
			literal.WriteString(marker)
			index += length
			continue
		}
		for offset := start; offset < len(source); {
			match := strings.Index(source[offset:], marker)
			if match < 0 {
				break
			}
			candidate := offset + match
			if candidate > start && source[candidate-1] != '\\' && (character != '_' || candidate+length == len(source) || !markdownWord(source[candidate+length])) {
				end = candidate
				break
			}
			offset = candidate + length
		}
		if end < 0 || strings.TrimSpace(source[start:end]) == "" {
			literal.WriteString(marker)
			index += length
			continue
		}
		flush()
		if character == '`' {
			output.WriteString(markdownStyle(app.Styles.SecondaryTextColor, attributes))
			output.WriteString(tview.Escape(source[start:end]))
		} else {
			next := attributes
			flag := "i"
			if length == 2 {
				flag = "b"
			}
			if length == 3 {
				flag = "bi"
			}
			if !strings.Contains(next, flag) {
				next += flag
			}
			output.WriteString(markdownInline(source[start:end], color, next, depth+1))
		}
		output.WriteString("[-:-:-]" + base)
		index = end + length
	}
	flush()
	return output.String()
}
