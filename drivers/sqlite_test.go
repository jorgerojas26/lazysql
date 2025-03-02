package drivers

import (
	"testing"
)

func TestSQLite_FormatArg(t *testing.T) {
	db := &SQLite{}

	testCases := []struct {
		name     string
		arg      interface{}
		expected string
	}{
		{
			name:     "Integer argument",
			arg:      123,
			expected: "123",
		},
		{
			name:     "String argument",
			arg:      "test string",
			expected: "'test string'",
		},
		{
			name:     "Byte array argument",
			arg:      []byte("byte array"),
			expected: "'byte array'",
		},
		{
			name:     "Float argument",
			arg:      123.45,
			expected: "123.450000",
		},
		{
			name:     "Default argument",
			arg:      true,
			expected: "true",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formattedArg := db.FormatArg(tc.arg)
			if formattedArg != tc.expected {
				t.Fatalf("expected %q, but got %q", tc.expected, formattedArg)
			}
		})
	}
}
