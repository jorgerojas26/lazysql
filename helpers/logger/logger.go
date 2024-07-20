package logger

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
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
	if level < l.level {
		return
	}

	logMessage := logMessage{
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     level.String(),
		Message:   msg,
		Data:      data,
	}

	logData, err := json.Marshal(logMessage)
	if err != nil {
		fmt.Println("Error marshaling log message:", err)
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		// maybe add another way to log, I did not want to add fmt.Println since this is a TUI app
		return
	}

	l.file.Write(logData)
	l.file.Write([]byte("\n"))
}

func (l *logger) SetFile(filename string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		l.file.Close()
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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

func Info(msg string, data map[string]any) {
	logInstance.log(slog.LevelInfo, msg, data)
}

func Warn(msg string, data map[string]any) {
	logInstance.log(slog.LevelWarn, msg, data)
}

func Error(msg string, data map[string]any) {
	logInstance.log(slog.LevelError, msg, data)
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
