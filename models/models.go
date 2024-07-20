package models

import (
	"github.com/rivo/tview"
)

type Connection struct {
	Name     string
	Provider string
	DBName   string
	URL      string
}

type StateChange struct {
	Value interface{}
	Key   string
}

type ConnectionPages struct {
	*tview.Flex
	*tview.Pages
}

type (
	CellValueType int8
	DmlType       int8
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
	Type   CellValueType
	Column string
	Value  interface{}
}

const (
	DmlUpdateType DmlType = iota
	DmlDeleteType
	DmlInsertType
)

type DbDmlChange struct {
	Type                 DmlType
	Table                string
	Values               []CellValue
	PrimaryKeyColumnName string
	PrimaryKeyValue      string
}

type DatabaseTableColumn struct {
	Field   string
	Type    string
	Null    string
	Key     string
	Default string
	Extra   string
}
