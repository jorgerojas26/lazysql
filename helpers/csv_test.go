package helpers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanCellValue(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NULL marker",
			input:    "NULL&",
			expected: "",
		},
		{
			name:     "EMPTY marker",
			input:    "EMPTY&",
			expected: "",
		},
		{
			name:     "DEFAULT marker",
			input:    "DEFAULT&",
			expected: "",
		},
		{
			name:     "Regular string",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "String ending with ampersand but not a marker",
			input:    "test&",
			expected: "test&",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "String with NULL inside",
			input:    "contains NULL text",
			expected: "contains NULL text",
		},
		{
			name:     "Ampersand only",
			input:    "&",
			expected: "&",
		},
		{
			name:     "NULLABLE (not NULL&)",
			input:    "NULLABLE",
			expected: "NULLABLE",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := CleanCellValue(tc.input)
			if result != tc.expected {
				t.Fatalf("CleanCellValue(%q): expected %q, got %q", tc.input, tc.expected, result)
			}
		})
	}
}

func TestExportToCSV(t *testing.T) {
	t.Run("Export simple records", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_export.csv")

		records := [][]string{
			{"id", "name", "value"},
			{"1", "Alice", "100"},
			{"2", "Bob", "200"},
		}

		err := ExportToCSV(records, filePath)
		if err != nil {
			t.Fatalf("ExportToCSV failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read exported file: %v", err)
		}

		expected := "id,name,value\n1,Alice,100\n2,Bob,200\n"
		if string(content) != expected {
			t.Fatalf("Exported content mismatch:\nexpected: %q\ngot: %q", expected, string(content))
		}
	})

	t.Run("Export with NULL markers", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_null.csv")

		records := [][]string{
			{"id", "name"},
			{"1", "NULL&"},
			{"2", "EMPTY&"},
		}

		err := ExportToCSV(records, filePath)
		if err != nil {
			t.Fatalf("ExportToCSV failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read exported file: %v", err)
		}

		expected := "id,name\n1,\n2,\n"
		if string(content) != expected {
			t.Fatalf("Exported content mismatch:\nexpected: %q\ngot: %q", expected, string(content))
		}
	})

	t.Run("Export with special characters", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_special.csv")

		records := [][]string{
			{"id", "description"},
			{"1", "Hello, World"},
			{"2", "Line with \"quotes\""},
		}

		err := ExportToCSV(records, filePath)
		if err != nil {
			t.Fatalf("ExportToCSV failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read exported file: %v", err)
		}

		// CSV escapes commas and quotes
		expected := "id,description\n1,\"Hello, World\"\n2,\"Line with \"\"quotes\"\"\"\n"
		if string(content) != expected {
			t.Fatalf("Exported content mismatch:\nexpected: %q\ngot: %q", expected, string(content))
		}
	})

	t.Run("Export empty records", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_empty.csv")

		records := [][]string{}

		err := ExportToCSV(records, filePath)
		if err != nil {
			t.Fatalf("ExportToCSV failed: %v", err)
		}

		// File should not be created for empty records
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Fatalf("Expected file to not exist for empty records, but it exists")
		}
	})

	t.Run("Creates parent directories", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "nested", "dir", "test.csv")

		records := [][]string{{"header"}, {"value"}}

		err := ExportToCSV(records, filePath)
		if err != nil {
			t.Fatalf("ExportToCSV failed: %v", err)
		}

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Fatalf("File was not created: %v", err)
		}
	})
}
