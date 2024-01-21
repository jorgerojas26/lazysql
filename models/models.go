package models

import (
	"github.com/google/uuid"
	"github.com/rivo/tview"
)

type Connection struct {
	Name     string
	Provider string
	User     string
	Password string
	Host     string
	Port     string
	Query    string
	DBName   string
	DSN      string
}

type StateChange struct {
	Value interface{}
	Key   string
}

type ConnectionPages struct {
	*tview.Flex
	*tview.Pages
}

type DbDmlChange struct {
	Type   string
	Table  string
	Column string
	Value  string
	RowId  string
	Option int
}

type DbInsert struct {
	Table   string
	Columns []string
	Values  []string
	Option  int
	RowId   uuid.UUID
}
