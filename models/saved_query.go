package models

// SavedQuery represents a query that the user has saved for later use.
type SavedQuery struct {
	Name  string `toml:"name"`
	Query string `toml:"query"`
}
