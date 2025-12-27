package helpers

import (
	"bufio"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
)

// CleanCellValue removes special markers from cell values (NULL&, EMPTY&, DEFAULT&)
func CleanCellValue(value string) string {
	if cleaned, found := strings.CutSuffix(value, "&"); found {
		switch cleaned {
		case "NULL", "EMPTY", "DEFAULT":
			return ""
		}
	}
	return value
}

// ExportToCSV exports records to a CSV file.
// records should include the header row as the first element.
func ExportToCSV(records [][]string, filePath string) error {
	if len(records) == 0 {
		return nil
	}

	// Ensure the directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use larger buffer to reduce disk I/O (default 4KB -> 64KB)
	bufferedWriter := bufio.NewWriterSize(file, 64*1024)
	writer := csv.NewWriter(bufferedWriter)

	// Reuse slice to reduce heap allocations (all rows have same column count)
	cleanedRecord := make([]string, len(records[0]))

	for _, record := range records {
		for i, cell := range record {
			cleanedRecord[i] = CleanCellValue(cell)
		}

		if err := writer.Write(cleanedRecord); err != nil {
			return err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}

	return bufferedWriter.Flush()
}
