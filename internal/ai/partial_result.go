package ai

import (
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func partialString(input string) (string, int, bool) {
	if len(input) == 0 || input[0] != '"' {
		return "", 0, false
	}
	var text strings.Builder
	for i := 1; i < len(input); {
		if input[i] == '"' {
			return text.String(), i + 1, true
		}
		if input[i] == '\\' {
			if i+1 >= len(input) {
				return text.String(), len(input), false
			}
			switch input[i+1] {
			case '"', '\\', '/':
				text.WriteByte(input[i+1])
				i += 2
			case 'n':
				text.WriteByte('\n')
				i += 2
			case 'r':
				text.WriteByte('\r')
				i += 2
			case 't':
				text.WriteByte('\t')
				i += 2
			case 'b':
				text.WriteByte('\b')
				i += 2
			case 'f':
				text.WriteByte('\f')
				i += 2
			case 'u':
				if i+6 > len(input) {
					return text.String(), len(input), false
				}
				value, err := strconv.ParseUint(input[i+2:i+6], 16, 16)
				if err != nil {
					return text.String(), len(input), false
				}
				r := rune(value)
				if utf16.IsSurrogate(r) {
					if i+12 > len(input) || input[i+6:i+8] != `\u` {
						return text.String(), len(input), false
					}
					low, err := strconv.ParseUint(input[i+8:i+12], 16, 16)
					if err != nil {
						return text.String(), len(input), false
					}
					r = utf16.DecodeRune(r, rune(low))
					if r == utf8.RuneError {
						return text.String(), len(input), false
					}
					i += 6
				}
				text.WriteRune(r)
				i += 6
			default:
				return text.String(), len(input), false
			}
		} else {
			if input[i] < 32 || !utf8.FullRuneInString(input[i:]) {
				return text.String(), len(input), false
			}
			r, size := utf8.DecodeRuneInString(input[i:])
			text.WriteRune(r)
			i += size
		}
	}
	return text.String(), len(input), false
}

func partialResult(input string) Result {
	input = strings.TrimSpace(input)
	input = strings.TrimPrefix(input, "```json\n")
	var result Result
	if len(input) == 0 || input[0] != '{' {
		return result
	}
	input = input[1:]
	for {
		input = strings.TrimSpace(input)
		key, length, complete := partialString(input)
		if !complete {
			return result
		}
		input = strings.TrimSpace(input[length:])
		if len(input) == 0 || input[0] != ':' {
			return result
		}
		input = strings.TrimSpace(input[1:])
		value, length, complete := partialString(input)
		switch key {
		case "sql":
			result.SQL = value
		case "database":
			result.Database = value
		case "message":
			result.Message = value
		case "clarification":
			result.Clarification = value
		case "missing":
			result.Missing = value
		case "warning":
			result.Warning = value
		}
		if !complete {
			return result
		}
		input = strings.TrimSpace(input[length:])
		if len(input) == 0 || input[0] != ',' {
			return result
		}
		input = input[1:]
	}
}
