package drivers

// Document represents a document in a NoSQL database as a map of field names to values.
// This allows for flexible, schema-less document structures with nested data.
type Document map[string]interface{}

// Filter represents database-agnostic filter conditions for querying documents.
// Each key is a field name, and the value is a FilterCondition describing how to filter that field.
type Filter map[string]FilterCondition

// FilterCondition represents a single filter operation on a field.
type FilterCondition struct {
	Operator string      // Operator: "eq", "ne", "gt", "gte", "lt", "lte", "in", "nin", "contains", "regex"
	Value    interface{} // The value to compare against
}

// Sort represents sorting configuration for query results.
type Sort struct {
	Field string // Field name to sort by
	Order string // Sort order: "asc" or "desc"
}

// Schema represents the inferred schema of a NoSQL collection.
// Since NoSQL databases are schema-less, this is derived from sampling documents.
type Schema struct {
	Fields []SchemaField // List of fields found in the collection
}

// SchemaField represents metadata about a field in a document.
type SchemaField struct {
	Name   string        // Field name
	Type   string        // Inferred type: "string", "number", "boolean", "object", "array", "null"
	Nested []SchemaField // For object types, nested fields
}

// Index represents an index on a NoSQL collection.
type Index struct {
	Name   string   // Index name
	Fields []string // Fields included in the index
	Type   string   // Index type: "btree", "hash", "text", "geo", etc.
	Unique bool     // Whether the index enforces uniqueness
}

// NoSQLDriver defines the interface that all NoSQL database drivers must implement.
// This interface abstracts differences between MongoDB, Redis, DynamoDB, Cassandra, etc.
type NoSQLDriver interface {
	// Connection Management

	// Connect establishes a connection to the NoSQL database using the provided connection string.
	// The connection string format is database-specific (e.g., "mongodb://user:pass@host:port/database").
	Connect(urlstr string) error

	// TestConnection validates that a connection can be established without storing the connection.
	// Useful for testing connection strings in the UI before saving.
	TestConnection(urlstr string) error

	// Disconnect closes the database connection and cleans up resources.
	// Should be called when the connection is no longer needed to prevent resource leaks.
	Disconnect() error

	// Metadata Operations

	// GetDatabases returns a list of all databases available on the server.
	GetDatabases() ([]string, error)

	// GetCollections returns collections grouped by namespace/category.
	// The map key represents a grouping (empty string for flat structure).
	// For MongoDB: map[""][]string (flat list of collections)
	// For Redis: map[namespace][]string (keys grouped by namespace)
	GetCollections(database string) (map[string][]string, error)

	// GetSchema infers the schema of a collection by sampling documents.
	// Returns the fields found and their inferred types.
	GetSchema(database, collection string) (Schema, error)

	// GetIndexes returns all indexes defined on a collection.
	GetIndexes(database, collection string) ([]Index, error)

	// Data Operations

	// GetDocuments retrieves documents from a collection with filtering, sorting, and pagination.
	// Returns: documents slice, total count (for pagination), error
	GetDocuments(database, collection string, filter Filter, sort Sort, offset, limit int) ([]Document, int, error)

	// InsertDocument inserts a single document into a collection.
	// The document's _id field will be auto-generated if not provided (database-specific).
	InsertDocument(database, collection string, doc Document) error

	// UpdateDocument updates documents matching the filter with the provided update document.
	// The exact update semantics are database-specific (e.g., MongoDB's $set operator).
	UpdateDocument(database, collection string, filter Filter, update Document) error

	// DeleteDocument deletes all documents matching the filter.
	DeleteDocument(database, collection string, filter Filter) error

	// ExecuteQuery executes a raw database-specific query and returns the results.
	// For MongoDB: MQL aggregation pipeline as JSON string
	// For Redis: Redis commands
	// Returns: result documents, error
	ExecuteQuery(database, collection, query string) ([]Document, error)

	// Formatting Methods (for dialect abstraction)

	// FormatFilter converts an abstract Filter to the database-specific filter format.
	// For MongoDB: returns bson.M
	// For Redis: returns pattern string
	// For DynamoDB: returns KeyConditionExpression
	FormatFilter(filter Filter) (interface{}, error)

	// FormatSort converts an abstract Sort to the database-specific sort format.
	// For MongoDB: returns bson.D
	// For Redis: may return empty (limited sorting support)
	FormatSort(sort Sort) (interface{}, error)

	// FormatIdentifier formats a collection/field name according to database conventions.
	// For MongoDB: typically no modification needed
	// For Redis: may add namespace prefix
	FormatIdentifier(name string) string

	// Provider Metadata

	// GetProvider returns the driver type identifier (e.g., "mongodb", "redis").
	GetProvider() string

	// SetProvider sets the driver type identifier.
	SetProvider(provider string)
}
