package drivers

import (
	"testing"
)

func TestMySQL_FormatArg(t *testing.T) {
	db := &MySQL{}

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
			name:     "Int64 argument",
			arg:      int64(1234567890),
			expected: "1234567890",
		},
		{
			name:     "Float argument",
			arg:      123.45,
			expected: "123.450000",
		},
		{
			name:     "String argument",
			arg:      "hello",
			expected: "'hello'",
		},
		{
			name:     "Byte array argument",
			arg:      []byte("world"),
			expected: "'world'",
		},
		{
			name:     "Default argument",
			arg:      true,
			expected: "true",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := db.FormatArg(tc.arg)
			if actual != tc.expected {
				t.Errorf("expected %q, but got %q", tc.expected, actual)
			}
		})
	}
}
