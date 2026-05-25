package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jorgerojas26/lazysql/app"
)

const (
	sessionDirName       = "session"
	lazysqlConfigDirName = "lazysql"
	sessionFileExtension = ".json"
)

type Session struct {
	Database string `json:"database"`
	Table    string `json:"table"`
}

func getSessionFilePath(connectionIdentifier string) (string, error) {
	configDir, err := app.GetConfigPath()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}

	sessionDir := filepath.Join(configDir, lazysqlConfigDirName, sessionDirName)

	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create session directory: %w", err)
	}

	return filepath.Join(sessionDir, sanitizeFilename(connectionIdentifier)+sessionFileExtension), nil
}

func sanitizeFilename(name string) string {
	if name == "" {
		return "default_connection"
	}
	reg := regexp.MustCompile(`[<>:"/\\|?*\s]+`)
	sanitized := reg.ReplaceAllString(name, "_")
	reg = regexp.MustCompile("[^a-zA-Z0-9_.-]+")
	sanitized = reg.ReplaceAllString(sanitized, "_")
	const maxLength = 100
	if len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
	}
	return strings.ToLower(sanitized)
}

func Save(connectionIdentifier, database, table string) error {
	if database == "" || table == "" {
		return nil
	}

	filePath, err := getSessionFilePath(connectionIdentifier)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(Session{Database: database, Table: table}, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	return os.WriteFile(filePath, data, 0o600)
}

func Load(connectionIdentifier string) (Session, error) {
	filePath, err := getSessionFilePath(connectionIdentifier)
	if err != nil {
		return Session{}, err
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return Session{}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return Session{}, fmt.Errorf("failed to read session file: %w", err)
	}

	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return Session{}, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return s, nil
}
