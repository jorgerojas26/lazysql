package helpers

import "testing"

func TestBinaryDisplayValue(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		want   string
		binary bool
	}{
		{name: "uuid bytes", value: "\x11\xee\x8c\x5b\x00\x42\x0a\xbc\xde\xf0\x12\x34\x56\x78\x9a\xbc", want: "0x11EE8C5B00420ABCDEF0123456789ABC", binary: true},
		{name: "invalid utf8", value: "caf\xe9", want: "0x636166E9", binary: true},
		{name: "nul byte", value: "a\x00b", want: "0x610062", binary: true},
		{name: "delete control", value: "a\x7f", want: "0x617F", binary: true},
		{name: "plain text", value: "hello", binary: false},
		{name: "unicode text", value: "héllo 世界", binary: false},
		{name: "multiline text", value: "line one\nline two\r\n\tindented", binary: false},
		{name: "empty", value: "", binary: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, binary := BinaryDisplayValue(tt.value)
			if binary != tt.binary || got != tt.want {
				t.Fatalf("BinaryDisplayValue(%q) = (%q, %v), want (%q, %v)", tt.value, got, binary, tt.want, tt.binary)
			}
		})
	}
}

func TestParseBinaryDisplayValue(t *testing.T) {
	tests := []struct {
		text string
		want string
		ok   bool
	}{
		{text: "0x636166E9", want: "caf\xe9", ok: true},
		{text: "0X00ff", want: "\x00\xff", ok: true},
		{text: "0x", want: "", ok: true},
		{text: "0x123", ok: false},
		{text: "0xZZ", ok: false},
		{text: "636166", ok: false},
		{text: "hello", ok: false},
	}

	for _, tt := range tests {
		got, ok := ParseBinaryDisplayValue(tt.text)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseBinaryDisplayValue(%q) = (%q, %v), want (%q, %v)", tt.text, got, ok, tt.want, tt.ok)
		}
	}
}

func TestBinaryDisplayValueRoundTrip(t *testing.T) {
	raw := "\x00\x01\xfe\xff binary"
	text, ok := BinaryDisplayValue(raw)
	if !ok {
		t.Fatalf("BinaryDisplayValue(%q) reported text", raw)
	}
	got, ok := ParseBinaryDisplayValue(text)
	if !ok || got != raw {
		t.Fatalf("ParseBinaryDisplayValue(%q) = (%q, %v), want (%q, true)", text, got, ok, raw)
	}
}

func TestEncodeBinaryDisplayValue(t *testing.T) {
	for _, raw := range []string{"", "AB", "\t\n\r", "\x00\xff", "0x41"} {
		text := EncodeBinaryDisplayValue(raw)
		got, ok := ParseBinaryDisplayValue(text)
		if !ok || got != raw {
			t.Fatalf("round trip %q = %q, %v", raw, got, ok)
		}
	}
}
