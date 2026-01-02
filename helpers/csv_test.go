package helpers

import (
	"os"
	"path/filepath"
	"strings"
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

func TestCSVWriter(t *testing.T) {
	t.Run("Commit creates final file", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_commit.csv")

		writer, err := NewCSVWriter(filePath)
		if err != nil {
			t.Fatalf("NewCSVWriter failed: %v", err)
		}

		records := [][]string{
			{"id", "name", "value"},
			{"1", "Alice", "100"},
			{"2", "Bob", "200"},
		}

		err = writer.WriteRecords(records, true)
		if err != nil {
			t.Fatalf("WriteRecords failed: %v", err)
		}

		if writer.RowCount() != 2 {
			t.Fatalf("Expected RowCount 2, got %d", writer.RowCount())
		}

		err = writer.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		expected := "id,name,value\n1,Alice,100\n2,Bob,200\n"
		if string(content) != expected {
			t.Fatalf("Content mismatch:\nexpected: %q\ngot: %q", expected, string(content))
		}

		// Check no temp files remain
		entries, _ := os.ReadDir(tempDir)
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".lazysql_export_") {
				t.Fatalf("Temp file should not remain: %s", entry.Name())
			}
		}
	})

	t.Run("Abort removes temp file", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_abort.csv")

		writer, err := NewCSVWriter(filePath)
		if err != nil {
			t.Fatalf("NewCSVWriter failed: %v", err)
		}

		records := [][]string{
			{"id", "name"},
			{"1", "Alice"},
		}

		err = writer.WriteRecords(records, true)
		if err != nil {
			t.Fatalf("WriteRecords failed: %v", err)
		}

		writer.Abort()

		// Final file should not exist
		if _, err := os.Stat(filePath); !os.IsNotExist(err) {
			t.Fatalf("Final file should not exist after Abort")
		}

		// Temp file should not exist
		entries, _ := os.ReadDir(tempDir)
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".lazysql_export_") {
				t.Fatalf("Temp file should be removed after Abort: %s", entry.Name())
			}
		}
	})

	t.Run("Abort after Commit is safe", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_abort_after_commit.csv")

		writer, err := NewCSVWriter(filePath)
		if err != nil {
			t.Fatalf("NewCSVWriter failed: %v", err)
		}

		records := [][]string{{"header"}, {"value"}}
		_ = writer.WriteRecords(records, true)
		_ = writer.Commit()

		// Abort after Commit should be no-op
		writer.Abort()

		// File should still exist
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Fatalf("File should exist after Abort following Commit")
		}
	})

	t.Run("Write multiple batches", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_multi_batch.csv")

		writer, err := NewCSVWriter(filePath)
		if err != nil {
			t.Fatalf("NewCSVWriter failed: %v", err)
		}
		defer writer.Abort()

		// First batch with header
		batch1 := [][]string{
			{"id", "name"},
			{"1", "Alice"},
			{"2", "Bob"},
		}
		err = writer.WriteRecords(batch1, true)
		if err != nil {
			t.Fatalf("WriteRecords batch1 failed: %v", err)
		}

		// Second batch without header
		batch2 := [][]string{
			{"id", "name"},
			{"3", "Charlie"},
			{"4", "Diana"},
		}
		err = writer.WriteRecords(batch2, false)
		if err != nil {
			t.Fatalf("WriteRecords batch2 failed: %v", err)
		}

		if writer.RowCount() != 4 {
			t.Fatalf("Expected RowCount 4, got %d", writer.RowCount())
		}

		err = writer.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		expected := "id,name\n1,Alice\n2,Bob\n3,Charlie\n4,Diana\n"
		if string(content) != expected {
			t.Fatalf("Content mismatch:\nexpected: %q\ngot: %q", expected, string(content))
		}
	})

	t.Run("Write with NULL markers", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_null_markers.csv")

		writer, err := NewCSVWriter(filePath)
		if err != nil {
			t.Fatalf("NewCSVWriter failed: %v", err)
		}
		defer writer.Abort()

		records := [][]string{
			{"id", "name"},
			{"1", "NULL&"},
			{"2", "EMPTY&"},
		}

		err = writer.WriteRecords(records, true)
		if err != nil {
			t.Fatalf("WriteRecords failed: %v", err)
		}

		err = writer.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		expected := "id,name\n1,\n2,\n"
		if string(content) != expected {
			t.Fatalf("Content mismatch:\nexpected: %q\ngot: %q", expected, string(content))
		}
	})

	t.Run("Write empty records", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_empty.csv")

		writer, err := NewCSVWriter(filePath)
		if err != nil {
			t.Fatalf("NewCSVWriter failed: %v", err)
		}
		defer writer.Abort()

		err = writer.WriteRecords([][]string{}, true)
		if err != nil {
			t.Fatalf("WriteRecords failed: %v", err)
		}

		if writer.RowCount() != 0 {
			t.Fatalf("Expected RowCount 0, got %d", writer.RowCount())
		}

		err = writer.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
	})

	t.Run("Write header only", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "test_header_only.csv")

		writer, err := NewCSVWriter(filePath)
		if err != nil {
			t.Fatalf("NewCSVWriter failed: %v", err)
		}
		defer writer.Abort()

		records := [][]string{
			{"id", "name", "value"},
		}

		err = writer.WriteRecords(records, true)
		if err != nil {
			t.Fatalf("WriteRecords failed: %v", err)
		}

		if writer.RowCount() != 0 {
			t.Fatalf("Expected RowCount 0, got %d", writer.RowCount())
		}

		err = writer.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("Failed to read file: %v", err)
		}

		expected := "id,name,value\n"
		if string(content) != expected {
			t.Fatalf("Content mismatch:\nexpected: %q\ngot: %q", expected, string(content))
		}
	})

	t.Run("Creates parent directories", func(t *testing.T) {
		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "nested", "dir", "test.csv")

		writer, err := NewCSVWriter(filePath)
		if err != nil {
			t.Fatalf("NewCSVWriter failed: %v", err)
		}
		defer writer.Abort()

		records := [][]string{{"header"}, {"value"}}
		err = writer.WriteRecords(records, true)
		if err != nil {
			t.Fatalf("WriteRecords failed: %v", err)
		}

		err = writer.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Fatalf("File was not created: %v", err)
		}
	})

	t.Run("Invalid path", func(t *testing.T) {
		// Try to create a file in a non-existent root path
		_, err := NewCSVWriter("/nonexistent_root_path_xxx/test.csv")
		if err == nil {
			t.Fatalf("Expected error for invalid path, got nil")
		}
	})
}
