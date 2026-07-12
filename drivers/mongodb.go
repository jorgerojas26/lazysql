package drivers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	// MongoDBDefaultTimeout is the default timeout for MongoDB operations.
	MongoDBDefaultTimeout = 10 * time.Second
	// MongoDBLongTimeout is used for potentially long-running operations like queries.
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
	connectCtx, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(urlstr))
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(connectCtx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db.Client = client
	db.Provider = DriverMongoDB
	return nil
}

// TestConnection validates the connection string without storing the connection.
func (db *MongoDB) TestConnection(urlstr string) error {
	testCtx, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	client, err := mongo.Connect(testCtx, options.Client().ApplyURI(urlstr))
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

// GetDatabases returns all non-system databases on the server.
func (db *MongoDB) GetDatabases() ([]string, error) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	databases, err := db.Client.ListDatabaseNames(ctxWithTimeout, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
	}

	filtered := make([]string, 0, len(databases))
	for _, dbName := range databases {
		if dbName != "admin" && dbName != "local" && dbName != "config" {
			filtered = append(filtered, dbName)
		}
	}

	return filtered, nil
}

// GetCollections returns all non-system collections in a database.
// MongoDB has a flat structure, so the map key is the empty string.
func (db *MongoDB) GetCollections(database string) (map[string][]string, error) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	collections, err := db.Client.Database(database).ListCollectionNames(ctxWithTimeout, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}

	filtered := make([]string, 0, len(collections))
	for _, collName := range collections {
		if !strings.HasPrefix(collName, "system.") {
			filtered = append(filtered, collName)
		}
	}

	return map[string][]string{"": filtered}, nil
}

// GetSchema infers the schema by sampling up to 100 documents from the collection.
func (db *MongoDB) GetSchema(database, collection string) (Schema, error) {
	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), MongoDBDefaultTimeout)
	defer cancel()

	coll := db.Client.Database(database).Collection(collection)

	cursor, err := coll.Find(ctxWithTimeout, bson.M{}, options.Find().SetLimit(100))
	if err != nil {
		return Schema{}, fmt.Errorf("failed to sample documents: %w", err)
	}
	defer cursor.Close(ctxWithTimeout)

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

	fields := make([]SchemaField, 0, len(fieldTypes))
	for fieldName, types := range fieldTypes {
		typeStr := "mixed"
		if len(types) == 1 {
			for t := range types {
				typeStr = t
			}
		}

		fields = append(fields, SchemaField{
			Name: fieldName,
			Type: typeStr,
		})
	}

	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })

	return Schema{Fields: fields}, nil
}

// inferType infers the display type of a document field value.
func inferType(value interface{}) string {
	switch value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case int, int32, int64, float32, float64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}, bson.A:
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

		name, _ := indexSpec["name"].(string)
		unique, _ := indexSpec["unique"].(bool)

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

	bsonFilter, err := db.FormatFilter(filter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to format filter: %w", err)
	}

	totalCount, err := coll.CountDocuments(ctxWithTimeout, bsonFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count documents: %w", err)
	}

	findOpts := options.Find().
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	if sort.Field != "" {
		findOpts.SetSort(db.FormatSort(sort))
	}

	cursor, err := coll.Find(ctxWithTimeout, bsonFilter, findOpts)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find documents: %w", err)
	}
	defer cursor.Close(ctxWithTimeout)

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

// FormatFilter converts an abstract Filter to MongoDB's bson.M format.
func (db *MongoDB) FormatFilter(filter Filter) (bson.M, error) {
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
func (db *MongoDB) FormatSort(sort Sort) bson.D {
	if sort.Field == "" {
		return bson.D{}
	}

	sortOrder := 1
	if sort.Order == "desc" {
		sortOrder = -1
	}

	return bson.D{{Key: sort.Field, Value: sortOrder}}
}

// GetProvider returns the driver type identifier.
func (db *MongoDB) GetProvider() string {
	return db.Provider
}

// SetProvider sets the driver type identifier.
func (db *MongoDB) SetProvider(provider string) {
	db.Provider = provider
}
