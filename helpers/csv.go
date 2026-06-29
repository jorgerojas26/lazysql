package helpers

import (
	"bufio"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
)

// CSVWriter supports streaming CSV writing with atomic file creation.
// It writes to a temporary file and renames it to the final path on Commit().
type CSVWriter struct {
	file           *os.File
	bufferedWriter *bufio.Writer
	csvWriter      *csv.Writer
	columnCount    int
	rowCount       int
	cleanedRecord  []string // reusable slice to reduce allocations
	tempPath       string
	finalPath      string
	done           bool
}

// NewCSVWriter creates a new CSVWriter that writes to a temporary file.
// Call Commit() to finalize the file, or Abort() to discard it.
func NewCSVWriter(filePath string) (*CSVWriter, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	tempFile, err := os.CreateTemp(dir, ".lazysql_export_*.tmp")
	if err != nil {
		return nil, err
	}

	bufferedWriter := bufio.NewWriterSize(tempFile, 64*1024)
	csvWriter := csv.NewWriter(bufferedWriter)

	return &CSVWriter{
		file:           tempFile,
		bufferedWriter: bufferedWriter,
		csvWriter:      csvWriter,
		tempPath:       tempFile.Name(),
		finalPath:      filePath,
	}, nil
}

// WriteRecords writes records to CSV.
// If includeHeader is true, records[0] is written as the header.
// If includeHeader is false, records[0] (header) is skipped and only records[1:] are written.
func (w *CSVWriter) WriteRecords(records [][]string, includeHeader bool) error {
	if len(records) == 0 {
		return nil
	}

	// Initialize column count and reusable slice on first write
	if w.columnCount == 0 {
		w.columnCount = len(records[0])
		w.cleanedRecord = make([]string, w.columnCount)
	}

	writeRow := func(record []string) error {
		for i := range w.cleanedRecord {
			if i < len(record) {
				w.cleanedRecord[i] = CleanCellValue(record[i])
			} else {
				w.cleanedRecord[i] = ""
			}
		}
		return w.csvWriter.Write(w.cleanedRecord)
	}

	if includeHeader {
		if err := writeRow(records[0]); err != nil {
			return err
		}
	}
	for _, record := range records[1:] {
		if err := writeRow(record); err != nil {
			return err
		}
		w.rowCount++
	}

	return nil
}

// Commit flushes, closes the temp file, and renames it to the final path.
// After Commit, the CSVWriter should not be used.
func (w *CSVWriter) Commit() error {
	if w.done {
		return nil
	}

	w.csvWriter.Flush()
	if err := w.csvWriter.Error(); err != nil {
		w.Abort()
		return err
	}
	if err := w.bufferedWriter.Flush(); err != nil {
		w.Abort()
		return err
	}

	w.done = true
	if err := w.file.Close(); err != nil {
		_ = os.Remove(w.tempPath)
		return err
	}

	return os.Rename(w.tempPath, w.finalPath)
}

// Abort closes the temp file and removes it.
// Safe to call multiple times or after Commit.
func (w *CSVWriter) Abort() {
	if w.done {
		return
	}
	w.done = true

	_ = w.file.Close()
	_ = os.Remove(w.tempPath)
}

// RowCount returns the number of data rows written (excluding header)
func (w *CSVWriter) RowCount() int {
	return w.rowCount
}

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
