package models

import (
	"time"

	"github.com/rivo/tview"
)

type AppConfig struct {
	DefaultPageSize              int
	DisableSidebar               bool
	SidebarOverlay               bool
	MaxQueryHistoryPerConnection int
}

type Connection struct {
	Name string

	// either use this directly
	URL string

	// or parse manually
	Provider  string
	Username  string
	Password  string
	Hostname  string
	Port      string
	DBName    string
	URLParams string

	Commands []*Command
}

type Command struct {
	Command      string
	WaitForPort  string
	SaveOutputTo string
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
