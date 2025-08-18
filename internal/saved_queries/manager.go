package saved_queries

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
)

const (
	SavedQueriesDirName       = "saved_queries"
	lazysqlConfigDirName      = "lazysql"
	savedQueriesFileExtension = ".toml"
)

// GetAppConfigDir returns the application's configuration directory.
func GetAppConfigDir() (string, error) {
	configDir, err := app.GetConfigPath()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	return filepath.Join(configDir, lazysqlConfigDirName), nil
}

// SanitizeFilename prepares a string to be used as a part of a filename.
// It replaces non-alphanumeric characters (except _, -, .) with underscores.
func SanitizeFilename(name string) string {
	if name == "" {
		return "default_connection" // Fallback for empty names
	}
	// Replace common problematic characters for filenames
	reg := regexp.MustCompile(`[<>:"/\\|?*\s]+`)
	sanitized := reg.ReplaceAllString(name, "_")

	// Further replace any remaining non-alphanumeric (excluding ., _, -)
	reg = regexp.MustCompile("[^a-zA-Z0-9_.-]+")
	sanitized = reg.ReplaceAllString(sanitized, "_")

	// Limit length to avoid issues with max filename length on some OS
	const maxLength = 100
	if len(sanitized) > maxLength {
		sanitized = sanitized[:maxLength]
	}
	return strings.ToLower(sanitized)
}

// GetSavedQueriesFilePath returns the path to the saved queries file for a specific connection.
func GetSavedQueriesFilePath(connectionIdentifier string) (string, error) {
	appConfigDir, err := GetAppConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get app config dir: %w", err)
	}

	savedQueriesDirPath := filepath.Join(appConfigDir, SavedQueriesDirName)

	// Ensure the saved queries directory exists
	if err := os.MkdirAll(savedQueriesDirPath, 0700); err != nil { // 0700: rwx for user only
		return "", fmt.Errorf("failed to create saved queries directory %s: %w", savedQueriesDirPath, err)
	}

	sanitizedIdentifier := SanitizeFilename(connectionIdentifier)
	if sanitizedIdentifier == "" {
		sanitizedIdentifier = "default_connection"
	}

	return filepath.Join(savedQueriesDirPath, sanitizedIdentifier+savedQueriesFileExtension), nil
}

// ReadSavedQueries reads the saved queries from the TOML file for a specific connection.
func ReadSavedQueries(connectionIdentifier string) ([]models.SavedQuery, error) {
	filePath, err := GetSavedQueriesFilePath(connectionIdentifier)
	if err != nil {
		return nil, err
	}
	logger.Info("Reading saved queries from file", map[string]any{"file": filePath})

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []models.SavedQuery{}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read saved queries file %s: %w", filePath, err)
	}

	if len(data) == 0 {
		return []models.SavedQuery{}, nil
	}

	var savedQueries struct {
		Queries []models.SavedQuery `toml:"queries"`
	}

	if err := toml.Unmarshal(data, &savedQueries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal saved queries from %s: %w", filePath, err)
	}

	return savedQueries.Queries, nil
}

// SaveQuery saves a new query to the TOML file for a specific connection.
func SaveQuery(connectionIdentifier, name, query string) error {
	savedQueries, err := ReadSavedQueries(connectionIdentifier)
	if err != nil {
		return err
	}

	for _, q := range savedQueries {
		if q.Name == name {
			return fmt.Errorf("a query with the name '%s' already exists", name)
		}
	}

	newQuery := models.SavedQuery{Name: name, Query: query}
	savedQueries = append(savedQueries, newQuery)

	return writeSavedQueries(connectionIdentifier, savedQueries)
}

// DeleteSavedQuery deletes a saved query from the TOML file for a specific connection.
func DeleteSavedQuery(connectionIdentifier, name string) error {
	savedQueries, err := ReadSavedQueries(connectionIdentifier)
	if err != nil {
		return err
	}

	var newQueries []models.SavedQuery
	for _, q := range savedQueries {
		if q.Name != name {
			newQueries = append(newQueries, q)
		}
	}

	if len(newQueries) == len(savedQueries) {
		return fmt.Errorf("a query with the name '%s' does not exist", name)
	}

	return writeSavedQueries(connectionIdentifier, newQueries)
}

func writeSavedQueries(connectionIdentifier string, queries []models.SavedQuery) error {
	filePath, err := GetSavedQueriesFilePath(connectionIdentifier)
	if err != nil {
		return err
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create saved queries file %s: %w", filePath, err)
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	return encoder.Encode(struct {
		Queries []models.SavedQuery `toml:"queries"`
	}{Queries: queries})
}
