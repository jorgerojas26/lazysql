package logger

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

type logger struct {
	mu     sync.Mutex
	file   *os.File
	level  slog.Level
	output string
}

type logMessage struct {
	Timestamp string         `json:"timestamp"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Data      map[string]any `json:"additional_info,omitempty"`
}

var logInstance *logger

func init() {
	logInstance = &logger{level: slog.LevelInfo}
}

func (l *logger) log(level slog.Level, msg string, data map[string]any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level || l.file == nil {
		return
	}

	logMessage := logMessage{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level.String(),
		Message:   redactSensitiveText(msg),
		Data:      sanitizeData(data),
	}

	logData, err := json.Marshal(logMessage)
	if err != nil {
		fmt.Println("Error marshaling log message:", err)
		return
	}

	if _, err = l.file.Write(logData); err != nil {
		return
	}

	_, _ = l.file.Write([]byte("\n"))
}

func (l *logger) SetFile(filename string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		err := l.file.Close()
		if err != nil {
			return err
		}
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	l.file = file
	l.output = filename
	return nil
}

func (l *logger) SetLevel(level slog.Level) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.level = level
}

func SetLevel(level slog.Level) {
	logInstance.SetLevel(level)
}

func SetFile(filename string) error {
	return logInstance.SetFile(filename)
}

func Debug(msg string, data map[string]any) {
	logInstance.log(slog.LevelDebug, msg, data)
}

// DebugOperation records a structured duration for a database or other
// performance-sensitive operation. Callers should provide only stable
// identity and outcome fields; SQL text, arguments, and row values are
// redacted by the logger.
func DebugOperation(operation string, started time.Time, data map[string]any) {
	fields := cloneData(data)
	elapsed := time.Since(started)
	fields["operation"] = operation
	fields["duration"] = elapsed.String()
	fields["duration_ms"] = elapsed.Milliseconds()
	Debug("performance_operation", fields)
}

// DebugFirstUsefulResult records the local-only time to the first useful UI
// result. It deliberately uses the same structured schema as operation logs.
func DebugFirstUsefulResult(surface string, started time.Time, data map[string]any) {
	fields := cloneData(data)
	elapsed := time.Since(started)
	fields["event"] = "first_useful_result"
	fields["surface"] = surface
	fields["duration"] = elapsed.String()
	fields["duration_ms"] = elapsed.Milliseconds()
	Debug("first_useful_result", fields)
}

func Info(msg string, data map[string]any) {
	logInstance.log(slog.LevelInfo, msg, data)
}

func Warn(msg string, data map[string]any) {
	logInstance.log(slog.LevelWarn, msg, data)
}

func Error(msg string, data map[string]any) {
	logInstance.log(slog.LevelError, msg, data)
}

func cloneData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return make(map[string]any, 4)
	}

	clone := make(map[string]any, len(data)+4)
	for key, value := range data {
		clone[key] = value
	}
	return clone
}

// sanitizeData keeps debug logging useful without allowing queries, arguments,
// command output, or row values to leak into a local log file.
func sanitizeData(data map[string]any) map[string]any {
	if len(data) == 0 {
		return nil
	}

	safe := make(map[string]any, len(data))
	for key, value := range data {
		if sensitiveField(key) {
			safe[key] = "[redacted]"
			continue
		}
		safe[key] = sanitizeValue(value)
	}
	return safe
}

func sanitizeValue(value any) any {
	switch typed := value.(type) {
	case error:
		return redactSensitiveText(typed.Error())
	case string:
		return redactSensitiveText(typed)
	case map[string]any:
		return sanitizeData(typed)
	case []any:
		values := make([]any, len(typed))
		for i, item := range typed {
			values[i] = sanitizeValue(item)
		}
		return values
	default:
		return value
	}
}

var (
	secretTextPattern    = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[-_]?key|authorization)(\s*[=:]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`)
	urlCredentialPattern = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://[^/\s:@]+:)[^@\s/]+@`)
)

func redactSensitiveText(value string) string {
	value = secretTextPattern.ReplaceAllString(value, "$1$2[redacted]")
	return urlCredentialPattern.ReplaceAllString(value, "$1[redacted]@")
}

func sensitiveField(key string) bool {
	key = strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
	for _, word := range []string{
		"password", "passwd", "secret", "token", "credential", "authorization",
		"query", "sql", "args", "argument", "param", "value", "output", "line", "command",
		"url",
	} {
		if strings.Contains(key, word) {
			return true
		}
	}
	return false
}

func ParseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q", s)
	}
}
