package helpers

import (
	"encoding/hex"
	"strings"
	"unicode/utf8"
)

// BinaryDisplayValue returns the text shown for a raw cell value that is not
// printable text, such as a BINARY(16) UUID: a 0x-prefixed hex string, the way
// MySQL clients show binary data. A value counts as binary when it is not valid
// UTF-8 or contains control characters other than tab, newline and carriage
// return. It returns false for values that are shown as they are.
func BinaryDisplayValue(value string) (string, bool) {
	if utf8.ValidString(value) && !strings.ContainsFunc(value, isBinaryControlRune) {
		return "", false
	}
	return "0x" + strings.ToUpper(hex.EncodeToString([]byte(value))), true
}

// ParseBinaryDisplayValue decodes a 0x-prefixed hex string, as shown by
// BinaryDisplayValue, back into the raw value it encodes.
func ParseBinaryDisplayValue(text string) (string, bool) {
	digits, found := strings.CutPrefix(text, "0x")
	if !found {
		digits, found = strings.CutPrefix(text, "0X")
	}
	if !found || digits == "" {
		return "", false
	}
	decoded, err := hex.DecodeString(digits)
	if err != nil {
		return "", false
	}
	return string(decoded), true
}

func isBinaryControlRune(r rune) bool {
	switch r {
	case '\t', '\n', '\r':
		return false
	}
	return r < 0x20 || r == 0x7f
}
