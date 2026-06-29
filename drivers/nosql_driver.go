package drivers

// Document represents a document in a NoSQL database as a map of field names to values.
type Document map[string]interface{}

// Filter represents database-agnostic filter conditions for querying documents.
type Filter map[string]FilterCondition

// FilterCondition represents a single filter operation on a field.
type FilterCondition struct {
	Operator string // "eq", "ne", "gt", "gte", "lt", "lte", "in", "nin", "contains", "regex"
	Value    interface{}
}

// Sort represents sorting configuration for query results.
type Sort struct {
	Field string // Field name to sort by
	Order string // "asc" or "desc"
}

// Schema represents the inferred schema of a NoSQL collection.
type Schema struct {
	Fields []SchemaField
}

// SchemaField represents metadata about a field in a document.
type SchemaField struct {
	Name   string
	Type   string        // "string", "number", "boolean", "object", "array", "null"
	Nested []SchemaField // For nested object types
}

// Index represents an index on a NoSQL collection.
type Index struct {
	Name   string
	Fields []string
	Type   string // "btree", "hash", "text", "geo", etc.
	Unique bool
}

// NoSQLDriver defines the interface that all NoSQL database drivers must implement.
type NoSQLDriver interface {
	// Connection Management
	Connect(urlstr string) error
	TestConnection(urlstr string) error
	Disconnect() error

	// Metadata Operations
	GetDatabases() ([]string, error)
	GetCollections(database string) (map[string][]string, error)
	GetSchema(database, collection string) (Schema, error)
	GetIndexes(database, collection string) ([]Index, error)

	// Data Operations
	GetDocuments(database, collection string, filter Filter, sort Sort, offset, limit int) ([]Document, int, error)
	InsertDocument(database, collection string, doc Document) error
	UpdateDocument(database, collection string, filter Filter, update Document) error
	DeleteDocument(database, collection string, filter Filter) error
	ExecuteQuery(database, collection, query string) ([]Document, error)

	// Formatting Methods
	FormatFilter(filter Filter) (interface{}, error)
	FormatSort(sort Sort) (interface{}, error)
	FormatIdentifier(name string) string

	// Provider Metadata
	GetProvider() string
	SetProvider(provider string)
}
