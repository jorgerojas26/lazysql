package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
)

const (
	historyDirName       = "history"
	lazysqlConfigDirName = "lazysql" // This should match your application's config directory name
	historyFileExtension = ".json"
)

// GetAppConfigDir returns the application's configuration directory.
func GetAppConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
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

// GetHistoryFilePath constructs the full path for a connection's history file.
// connectionIdentifier should be a unique name or ID for the connection.
func GetHistoryFilePath(connectionIdentifier string) (string, error) {
	appConfigDir, err := GetAppConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get app config dir: %w", err)
	}

	historyDirPath := filepath.Join(appConfigDir, historyDirName)

	// Ensure the history directory exists
	if err := os.MkdirAll(historyDirPath, 0700); err != nil { // 0700: rwx for user only
		return "", fmt.Errorf("failed to create history directory %s: %w", historyDirPath, err)
	}

	sanitizedIdentifier := SanitizeFilename(connectionIdentifier)
	if sanitizedIdentifier == "" { // Should be handled by SanitizeFilename, but as a safeguard
		sanitizedIdentifier = "default_connection"
	}

	return filepath.Join(historyDirPath, sanitizedIdentifier+historyFileExtension), nil
}

// ReadHistory reads query history items from the specified file.
// The 'limit' parameter is not strictly enforced in this read function yet,
// but is available for future use (e.g., if the modal itself doesn't limit or sort).
// The QueryHistoryModal currently sorts all items passed to it.
func ReadHistory(filePath string, _ int) ([]models.QueryHistoryItem, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logger.Info("History file does not exist, returning empty history.", map[string]any{"path": filePath})
		return []models.QueryHistoryItem{}, nil // No history file yet, return empty.
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read history file %s: %w", filePath, err)
	}

	if len(data) == 0 { // Empty file
		logger.Info("History file is empty.", map[string]any{"path": filePath})
		return []models.QueryHistoryItem{}, nil
	}

	var items []models.QueryHistoryItem
	if err := json.Unmarshal(data, &items); err != nil {
		// It's possible the file is corrupted. Log and consider returning empty or error.
		logger.Error("Failed to unmarshal history data, file might be corrupted.", map[string]any{"path": filePath, "error": err})
		// Depending on desired UX, could return empty to allow app to continue, or error to signal issue.
		// For now, return empty so the app doesn't crash, but log the error.
		return []models.QueryHistoryItem{}, fmt.Errorf("failed to unmarshal history data from %s: %w", filePath, err)
	}

	// The QueryHistoryModal will sort these items by timestamp.
	// If a limit were to be applied here, it should be after sorting.
	// Example:
	// sort.SliceStable(items, func(i, j int) bool {
	// 	return items[i].Timestamp.After(items[j].Timestamp)
	// })
	// if limit > 0 && len(items) > limit {
	// 	items = items[:limit]
	// }

	return items, nil
}

// AddQueryToHistory adds a query to the history for the given connection.
// It ensures the history does not exceed the configured limit and avoids immediate duplicates.
func AddQueryToHistory(connectionIdentifier string, queryText string) error {
	if strings.TrimSpace(queryText) == "" {
		logger.Info("Attempted to add empty query to history, skipping.", map[string]any{"connection": connectionIdentifier})
		return nil // Don't add empty or whitespace-only queries
	}

	historyFilePath, err := GetHistoryFilePath(connectionIdentifier)
	if err != nil {
		return fmt.Errorf("failed to get history file path for AddQueryToHistory: %w", err)
	}

	items, err := ReadHistory(historyFilePath, 0) // Limit is managed on write
	if err != nil {
		// If ReadHistory failed (e.g. corrupted JSON), it returns an error and empty items.
		// We log this and proceed with an empty list, effectively overwriting corrupted history.
		logger.Warn("Error reading existing history, will proceed as if history is empty or create new.", map[string]any{"path": historyFilePath, "error": err, "connection": connectionIdentifier})
		items = []models.QueryHistoryItem{} // Reset to ensure we don't append to potentially problematic data
	}

	// Avoid adding duplicate of the most recent query by updating its timestamp
	// Sort by timestamp descending to easily check the latest
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Timestamp.After(items[j].Timestamp)
	})

	if len(items) > 0 && items[0].QueryText == queryText {
		logger.Info("Query is identical to the most recent history entry, updating timestamp.", map[string]any{"connection": connectionIdentifier})
		items[0].Timestamp = time.Now().UTC()
	} else {
		newItem := models.QueryHistoryItem{
			QueryText: queryText,
			Timestamp: time.Now().UTC(),
		}
		items = append(items, newItem) // Add as a new entry
	}

	// Re-sort by timestamp descending (newest first) after potential addition or timestamp update
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Timestamp.After(items[j].Timestamp)
	})

	// Enforce history limit
	limit := app.App.Config().MaxQueryHistoryPerConnection
	if limit <= 0 { // Ensure a positive limit, fallback to a default
		limit = 100
		logger.Info("MaxQueryHistoryPerConnection is not set or invalid, using default limit.", map[string]any{"defaultLimit": limit, "connection": connectionIdentifier})
	}

	if len(items) > limit {
		items = items[:limit] // Keep only the newest 'limit' items
	}

	// Write updated history back to file
	data, err := json.MarshalIndent(items, "", "  ") // Use MarshalIndent for readability
	if err != nil {
		return fmt.Errorf("failed to marshal history items for %s: %w", connectionIdentifier, err)
	}

	// Ensure directory exists (GetHistoryFilePath should do this, but good for robustness before write)
	historyDir := filepath.Dir(historyFilePath)
	if err := os.MkdirAll(historyDir, 0700); err != nil { // 0700: rwx for user only
		return fmt.Errorf("failed to ensure history directory exists %s for %s: %w", historyDir, connectionIdentifier, err)
	}

	err = os.WriteFile(historyFilePath, data, 0600) // 0600: rw for user only
	if err != nil {
		return fmt.Errorf("failed to write history file %s for %s: %w", historyFilePath, connectionIdentifier, err)
	}

	logger.Info("Query successfully added/updated in history.", map[string]any{"connection": connectionIdentifier, "path": historyFilePath})
	return nil
}
