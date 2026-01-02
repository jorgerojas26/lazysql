package drivers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// MongoDBDefaultTimeout is the default timeout for MongoDB operations
	MongoDBDefaultTimeout = 10 * time.Second
	// MongoDBLongTimeout is used for potentially long-running operations like queries
	MongoDBLongTimeout = 30 * time.Second
)

// MongoDB implements the NoSQLDriver interface for MongoDB databases.
type MongoDB struct {
	Client   *mongo.Client
	Provider string
}

// Connect establishes a connection to MongoDB using the provided connection string.
// Connection string format: mongodb://[username:password@]host[:port][/database][?options]
func (db *MongoDB) Connect(urlstr string) error {
	if urlstr == "" {
		return errors.New("connection string is required")
	}

	connectCtx, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	clientOptions := options.Client().ApplyURI(urlstr)
	client, err := mongo.Connect(connectCtx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping the database to verify connection
	if err := client.Ping(connectCtx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db.Client = client
	db.Provider = DriverMongoDB
	return nil
}

// TestConnection validates the connection string without storing the connection.
func (db *MongoDB) TestConnection(urlstr string) error {
	if urlstr == "" {
		return errors.New("connection string is required")
	}

	testCtx, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	clientOptions := options.Client().ApplyURI(urlstr)
	client, err := mongo.Connect(testCtx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer func() {
		_ = client.Disconnect(testCtx)
	}()

	if err := client.Ping(testCtx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	return nil
}

// Disconnect closes the MongoDB client connection and cleans up resources.
func (db *MongoDB) Disconnect() error {
	if db.Client == nil {
		return nil // Already disconnected or never connected
	}

	disconnectCtx, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	if err := db.Client.Disconnect(disconnectCtx); err != nil {
		return fmt.Errorf("failed to disconnect from MongoDB: %w", err)
	}

	db.Client = nil
	return nil
}

// GetDatabases returns a list of all databases on the MongoDB server.
func (db *MongoDB) GetDatabases() ([]string, error) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	databases, err := db.Client.ListDatabaseNames(ctxWithTimeout, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}

	// Filter out system databases
	filtered := make([]string, 0, len(databases))
	for _, dbName := range databases {
		if dbName != "admin" && dbName != "local" && dbName != "config" {
			filtered = append(filtered, dbName)
		}
	}

	return filtered, nil
}

// GetCollections returns all collections in a database.
// MongoDB has a flat structure, so the map key is empty string.
func (db *MongoDB) GetCollections(database string) (map[string][]string, error) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	mongoDB := db.Client.Database(database)
	collections, err := mongoDB.ListCollectionNames(ctxWithTimeout, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	// Filter out system collections
	filtered := make([]string, 0, len(collections))
	for _, collName := range collections {
		if !strings.HasPrefix(collName, "system.") {
			filtered = append(filtered, collName)
		}
	}

	// Return flat structure (no grouping)
	return map[string][]string{"": filtered}, nil
}

// GetSchema infers the schema by sampling documents from the collection.
func (db *MongoDB) GetSchema(database, collection string) (Schema, error) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	coll := db.Client.Database(database).Collection(collection)

	// Sample up to 100 documents to infer schema
	cursor, err := coll.Find(ctxWithTimeout, bson.M{}, options.Find().SetLimit(100))
	if err != nil {
		return Schema{}, fmt.Errorf("failed to sample documents: %w", err)
	}
	defer cursor.Close(ctxWithTimeout)

	// Collect all field names and types
	fieldTypes := make(map[string]map[string]bool) // field -> set of types seen

	for cursor.Next(ctxWithTimeout) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}

		for key, value := range doc {
			if fieldTypes[key] == nil {
				fieldTypes[key] = make(map[string]bool)
			}
			fieldTypes[key][inferType(value)] = true
		}
	}

	// Convert to SchemaField slice
	fields := make([]SchemaField, 0, len(fieldTypes))
	for fieldName, types := range fieldTypes {
		// If multiple types seen, use the most common or "mixed"
		typeStr := "mixed"
		if len(types) == 1 {
			for t := range types {
				typeStr = t
				break
			}
		}

		fields = append(fields, SchemaField{
			Name: fieldName,
			Type: typeStr,
		})
	}

	return Schema{Fields: fields}, nil
}

// inferType infers the type of a value.
func inferType(value interface{}) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case string:
		return "string"
	case int, int32, int64, float32, float64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}, bson.M:
		return "object"
	default:
		return "unknown"
	}
}

// GetIndexes returns all indexes on a collection.
func (db *MongoDB) GetIndexes(database, collection string) ([]Index, error) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	coll := db.Client.Database(database).Collection(collection)
	cursor, err := coll.Indexes().List(ctxWithTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to list indexes: %w", err)
	}
	defer cursor.Close(ctxWithTimeout)

	var indexes []Index
	for cursor.Next(ctxWithTimeout) {
		var indexSpec bson.M
		if err := cursor.Decode(&indexSpec); err != nil {
			continue
		}

		// Extract index information
		name, _ := indexSpec["name"].(string)
		unique, _ := indexSpec["unique"].(bool)

		// Extract field names from key spec
		var fields []string
		if keySpec, ok := indexSpec["key"].(bson.M); ok {
			for field := range keySpec {
				fields = append(fields, field)
			}
		}

		indexes = append(indexes, Index{
			Name:   name,
			Fields: fields,
			Type:   "btree", // MongoDB default
			Unique: unique,
		})
	}

	return indexes, nil
}

// GetDocuments retrieves documents with filtering, sorting, and pagination.
func (db *MongoDB) GetDocuments(database, collection string, filter Filter, sort Sort, offset, limit int) ([]Document, int, error) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), MongoDBLongTimeout)
	defer cancel()

	coll := db.Client.Database(database).Collection(collection)

	// Convert abstract filter to MongoDB bson.M
	bsonFilter, err := db.FormatFilter(filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to format filter: %w", err)
	}

	// Get total count
	totalCount, err := coll.CountDocuments(ctxWithTimeout, bsonFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	// Build find options
	findOpts := options.Find().
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	// Add sorting if specified
	if sort.Field != "" {
		bsonSort, err := db.FormatSort(sort)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to format sort: %w", err)
		}
		findOpts.SetSort(bsonSort)
	}

	// Execute query
	cursor, err := coll.Find(ctxWithTimeout, bsonFilter, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find documents: %w", err)
	}
	defer cursor.Close(ctxWithTimeout)

	// Decode results
	var documents []Document
	for cursor.Next(ctxWithTimeout) {
		var doc Document
		if err := cursor.Decode(&doc); err != nil {
			return nil, 0, fmt.Errorf("failed to decode document: %w", err)
		}
		documents = append(documents, doc)
	}

	if err := cursor.Err(); err != nil {
		return nil, 0, fmt.Errorf("cursor error: %w", err)
	}

	return documents, int(totalCount), nil
}

// InsertDocument inserts a single document into a collection.
func (db *MongoDB) InsertDocument(database, collection string, doc Document) error {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	coll := db.Client.Database(database).Collection(collection)
	_, err := coll.InsertOne(ctxWithTimeout, doc)
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	return nil
}

// UpdateDocument updates documents matching the filter.
func (db *MongoDB) UpdateDocument(database, collection string, filter Filter, update Document) error {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	coll := db.Client.Database(database).Collection(collection)

	// Convert filter
	bsonFilter, err := db.FormatFilter(filter)
	if err != nil {
		return fmt.Errorf("failed to format filter: %w", err)
	}

	// Wrap update in $set operator
	updateDoc := bson.M{"$set": update}

	// Update all matching documents
	_, err = coll.UpdateMany(ctxWithTimeout, bsonFilter, updateDoc)
	if err != nil {
		return fmt.Errorf("failed to update documents: %w", err)
	}

	return nil
}

// DeleteDocument deletes documents matching the filter.
func (db *MongoDB) DeleteDocument(database, collection string, filter Filter) error {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	coll := db.Client.Database(database).Collection(collection)

	// Convert filter
	bsonFilter, err := db.FormatFilter(filter)
	if err != nil {
		return fmt.Errorf("failed to format filter: %w", err)
	}

	// Delete all matching documents
	_, err = coll.DeleteMany(ctxWithTimeout, bsonFilter)
	if err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}

	return nil
}

// ExecuteQuery executes a raw MongoDB query (aggregation pipeline as JSON string).
func (db *MongoDB) ExecuteQuery(database, collection, query string) ([]Document, error) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), MongoDBLongTimeout)
	defer cancel()

	coll := db.Client.Database(database).Collection(collection)

	// Parse the query string as BSON (simple find queries - can be extended for aggregation)
	var filter bson.M
	if err := bson.UnmarshalExtJSON([]byte(query), true, &filter); err != nil {
		return nil, fmt.Errorf("failed to parse query: %w", err)
	}

	cursor, err := coll.Find(ctxWithTimeout, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer cursor.Close(ctxWithTimeout)

	var documents []Document
	if err := cursor.All(ctxWithTimeout, &documents); err != nil {
		return nil, fmt.Errorf("failed to decode results: %w", err)
	}

	return documents, nil
}

// FormatFilter converts an abstract Filter to MongoDB's bson.M format.
func (db *MongoDB) FormatFilter(filter Filter) (interface{}, error) {
	if len(filter) == 0 {
		return bson.M{}, nil
	}

	bsonFilter := bson.M{}

	for field, condition := range filter {
		switch condition.Operator {
		case "eq":
			bsonFilter[field] = condition.Value
		case "ne":
			bsonFilter[field] = bson.M{"$ne": condition.Value}
		case "gt":
			bsonFilter[field] = bson.M{"$gt": condition.Value}
		case "gte":
			bsonFilter[field] = bson.M{"$gte": condition.Value}
		case "lt":
			bsonFilter[field] = bson.M{"$lt": condition.Value}
		case "lte":
			bsonFilter[field] = bson.M{"$lte": condition.Value}
		case "in":
			bsonFilter[field] = bson.M{"$in": condition.Value}
		case "nin":
			bsonFilter[field] = bson.M{"$nin": condition.Value}
		case "contains":
			// Text search using regex
			bsonFilter[field] = bson.M{"$regex": condition.Value, "$options": "i"}
		case "regex":
			bsonFilter[field] = bson.M{"$regex": condition.Value}
		default:
			return nil, fmt.Errorf("unsupported operator: %s", condition.Operator)
		}
	}

	return bsonFilter, nil
}

// FormatSort converts an abstract Sort to MongoDB's bson.D format.
func (db *MongoDB) FormatSort(sort Sort) (interface{}, error) {
	if sort.Field == "" {
		return bson.D{}, nil
	}

	sortOrder := 1 // ascending
	if sort.Order == "desc" {
		sortOrder = -1
	}

	return bson.D{{Key: sort.Field, Value: sortOrder}}, nil
}

// FormatIdentifier formats a collection/field name (MongoDB doesn't require special formatting).
func (db *MongoDB) FormatIdentifier(name string) string {
	return name
}

// GetProvider returns the driver type identifier.
func (db *MongoDB) GetProvider() string {
	return db.Provider
}

// SetProvider sets the driver type identifier.
func (db *MongoDB) SetProvider(provider string) {
	db.Provider = provider
}
