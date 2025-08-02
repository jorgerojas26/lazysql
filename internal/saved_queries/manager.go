package saved_queries

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
)

const (
	SavedQueriesDirName  = "saved_queries"
	lazysqlConfigDirName = "lazysql"
	SavedQueriesFileName = "queries.toml"
)

// GetSavedQueriesFilePath returns the path to the saved queries file.
func GetSavedQueriesFilePath() (string, error) {
	configDir, err := app.GetConfigPath()
	if err != nil {
		return "", fmt.Errorf("failed to get user config directory: %w", err)
	}
	appConfigDir := filepath.Join(configDir, lazysqlConfigDirName, SavedQueriesDirName)

	if err := os.MkdirAll(appConfigDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create saved queries directory %s: %w", appConfigDir, err)
	}

	return filepath.Join(appConfigDir, SavedQueriesFileName), nil
}

// ReadSavedQueries reads the saved queries from the TOML file.
func ReadSavedQueries() ([]models.SavedQuery, error) {
	filePath, err := GetSavedQueriesFilePath()
	logger.Info("Reading saved queries from file", map[string]any{"file": filePath})
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []models.SavedQuery{}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read saved queries file %s: %w", filePath, err)
	}

	var savedQueries struct {
		Queries []models.SavedQuery `toml:"queries"`
	}

	if err := toml.Unmarshal(data, &savedQueries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal saved queries from %s: %w", filePath, err)
	}

	return savedQueries.Queries, nil
}

// SaveQuery saves a new query to the TOML file.
func SaveQuery(name, query string) error {
	savedQueries, err := ReadSavedQueries()
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

	return writeSavedQueries(savedQueries)
}

// DeleteSavedQuery deletes a saved query from the TOML file.
func DeleteSavedQuery(name string) error {
	savedQueries, err := ReadSavedQueries()
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

	return writeSavedQueries(newQueries)
}

func writeSavedQueries(queries []models.SavedQuery) error {
	filePath, err := GetSavedQueriesFilePath()
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
