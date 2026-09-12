package models

import (
	"fmt"
	"time"

	"github.com/rivo/tview"
)

const (
	DefaultMaxOpenConnections  = 8
	DefaultMaxIdleConnections  = 8
	DefaultExactCountThreshold = 50000
	DefaultExactCountTimeoutMS = 200
	DefaultMaxQueryRows        = 1000
)

type AppConfig struct {
	DefaultPageSize              int
	DisableSidebar               bool
	SidebarOverlay               bool
	MaxQueryHistoryPerConnection int
	TreeWidth                    int
	JSONViewerWordWrap           bool
	EnterOpensJSONViewer         bool
	ConfirmOnQuit                bool
	MaxOpenConnections           int `toml:"max_open_connections"`
	MaxIdleConnections           int `toml:"max_idle_connections"`
	ExactCountThreshold          int `toml:"exact_count_threshold"`
	ExactCountTimeoutMS          int `toml:"exact_count_timeout_ms"`
	MaxQueryRows                 int `toml:"max_query_rows"`
}

type ConnectionPoolConfig struct {
	MaxOpenConnections int
	MaxIdleConnections int
}

// Normalize applies LazySQL's safe defaults and validates the effective pool
// limits before they are passed to database/sql.
func (config ConnectionPoolConfig) Normalize() (ConnectionPoolConfig, error) {
	if config.MaxOpenConnections < 0 {
		return ConnectionPoolConfig{}, fmt.Errorf("max_open_connections cannot be negative")
	}
	if config.MaxIdleConnections < 0 {
		return ConnectionPoolConfig{}, fmt.Errorf("max_idle_connections cannot be negative")
	}

	if config.MaxOpenConnections == 0 {
		config.MaxOpenConnections = DefaultMaxOpenConnections
	}
	if config.MaxIdleConnections == 0 {
		config.MaxIdleConnections = DefaultMaxIdleConnections
	}

	if config.MaxIdleConnections > config.MaxOpenConnections {
		return ConnectionPoolConfig{}, fmt.Errorf("max_idle_connections (%d) cannot exceed max_open_connections (%d)", config.MaxIdleConnections, config.MaxOpenConnections)
	}

	return config, nil
}

type Connection struct {
	Name string

	// either use this directly
	URL string `toml:",omitempty"`

	// or parse manually
	Provider  string `toml:",omitempty"`
	Username  string `toml:",omitempty"`
	Password  string `toml:",omitempty"`
	Hostname  string `toml:",omitempty"`
	Port      string `toml:",omitempty"`
	DBName    string `toml:",omitempty"`
	URLParams string `toml:",omitempty"`

	ReadOnly bool `toml:",omitempty"`

	// Pool limits are optional per-connection overrides. A nil value inherits
	// the application setting; an explicit zero uses LazySQL's safe default.
	MaxOpenConnections *int `toml:"max_open_connections,omitempty"`
	MaxIdleConnections *int `toml:"max_idle_connections,omitempty"`

	// Schemas filters the schemas shown in the tree (PostgreSQL/MSSQL only).
	// If empty, all schemas are shown.
	Schemas []string `toml:",omitempty"`

	Commands []*Command `toml:",omitempty"`
}

// EffectiveConnectionPool resolves optional connection overrides against the
// application settings and applies LazySQL's safe defaults.
func (config *AppConfig) EffectiveConnectionPool(connection Connection) (ConnectionPoolConfig, error) {
	pool := ConnectionPoolConfig{
		MaxOpenConnections: config.MaxOpenConnections,
		MaxIdleConnections: config.MaxIdleConnections,
	}
	if connection.MaxOpenConnections != nil {
		pool.MaxOpenConnections = *connection.MaxOpenConnections
	}
	if connection.MaxIdleConnections != nil {
		pool.MaxIdleConnections = *connection.MaxIdleConnections
	}

	return pool.Normalize()
}

type KeymapConfig map[string]map[string]string

type Command struct {
	Command      string
	WaitForPort  string
	SaveOutputTo string
	Timeout      int // Timeout in seconds for command to start (default: 5)
}

type StateChange struct {
	Value interface{}
	Key   string
}

type ConnectionPages struct {
	*tview.Grid
	*tview.Pages
}

type (
	CellValueType int8
	DMLType       int8
)

// This is not a direct map of the database types, but rather a way to represent them in the UI.
// So the String type is a representation of the cell value in the UI table and the others are
// just a representation of the values that you can put in the database but not in the UI as a string of characters.
const (
	Empty CellValueType = iota
	Null
	Default
	String
)

type CellValue struct {
	Value            any
	Column           string
	TableColumnIndex int
	TableRowIndex    int
	Type             CellValueType
}

const (
	DMLUpdateType DMLType = iota
	DMLDeleteType
	DMLInsertType
)

type PrimaryKeyInfo struct {
	Name  string
	Value any
}

func (pki PrimaryKeyInfo) Equal(other PrimaryKeyInfo) bool {
	return pki.Name == other.Name && pki.Value == other.Value
}

type DBDMLChange struct {
	Database       string
	Table          string
	PrimaryKeyInfo []PrimaryKeyInfo
	Values         []CellValue
	Type           DMLType
}

type DatabaseTableColumn struct {
	Field   string
	Type    string
	Null    string
	Key     string
	Default string
	Extra   string
}

type Query struct {
	Query string
	Args  []interface{}
}

type SidebarEditingCommitParams struct {
	ColumnName string
	NewValue   string
	Type       CellValueType
}

// QueryHistoryItem represents a single entry in the query history.
type QueryHistoryItem struct {
	QueryText string
	Timestamp time.Time
}
