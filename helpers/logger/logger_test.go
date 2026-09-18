package logger

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDebugPerformanceLogsAreStructuredAndRedacted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "performance.jsonl")
	if err := SetFile(path); err != nil {
		t.Fatalf("SetFile() error = %v", err)
	}
	SetLevel(slog.LevelDebug)
	t.Cleanup(func() {
		logInstance.mu.Lock()
		if logInstance.file != nil {
			_ = logInstance.file.Close()
			logInstance.file = nil
		}
		logInstance.level = slog.LevelInfo
		logInstance.mu.Unlock()
	})

	DebugOperation("fetch_records", time.Now(), map[string]any{
		"database": "app",
		"table":    "users",
		"query":    "SELECT password FROM users",
		"password": "do-not-write-me",
		"error":    errors.New("postgres://admin:dsn-secret@host/db password=inline-secret"),
		"rows":     3,
	})
	DebugFirstUsefulResult("records", time.Now(), map[string]any{
		"database": "app",
		"table":    "users",
		"rows":     3,
	})

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	lines := make([]string, 0, 2)
	for _, line := range strings.Split(strings.TrimSpace(string(contents)), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != 2 {
		t.Fatalf("log line count = %d, want 2: %s", len(lines), contents)
	}

	var operation struct {
		Message string         `json:"message"`
		Data    map[string]any `json:"additional_info"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &operation); err != nil {
		t.Fatalf("unmarshal operation log: %v", err)
	}
	if operation.Message != "performance_operation" {
		t.Fatalf("operation message = %q, want performance_operation", operation.Message)
	}
	if operation.Data["operation"] != "fetch_records" || operation.Data["database"] != "app" {
		t.Fatalf("operation fields = %#v", operation.Data)
	}
	if _, ok := operation.Data["duration"]; !ok {
		t.Fatalf("operation fields do not include duration: %#v", operation.Data)
	}
	if operation.Data["query"] != "[redacted]" || operation.Data["password"] != "[redacted]" {
		t.Fatalf("sensitive fields were not redacted: %#v", operation.Data)
	}
	if string(contents) == "" ||
		strings.Contains(string(contents), "do-not-write-me") ||
		strings.Contains(string(contents), "SELECT password") ||
		strings.Contains(string(contents), "dsn-secret") ||
		strings.Contains(string(contents), "inline-secret") {
		t.Fatalf("sensitive log content was written: %s", contents)
	}

	var firstResult struct {
		Message string         `json:"message"`
		Data    map[string]any `json:"additional_info"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &firstResult); err != nil {
		t.Fatalf("unmarshal first-result log: %v", err)
	}
	if firstResult.Data["event"] != "first_useful_result" || firstResult.Data["surface"] != "records" {
		t.Fatalf("first-result fields = %#v", firstResult.Data)
	}
}
