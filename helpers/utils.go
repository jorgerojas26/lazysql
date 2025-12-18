package helpers

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/xo/dburl"

	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/drivers"
)

// ParseConnectionString parses database connection strings for both SQL and NoSQL databases.
//
// Design Decision: SQL vs NoSQL Parsing Strategy
// SQL databases (MySQL, Postgres, SQLite, MSSQL) all implement Go's database/sql interface
// and use standardized connection strings that can be parsed by xo/dburl. This provides
// consistent handling of username, password, host, port, and database name extraction.
//
// NoSQL databases (MongoDB, Redis, Cassandra, etc.) do NOT implement database/sql and each
// has its own native driver with unique connection string formats and authentication schemes.
// For example:
// - MongoDB uses official mongo-driver with mongodb:// or mongodb+srv:// schemes
// - Redis uses go-redis with redis:// scheme and different auth patterns
// - Cassandra uses gocql with entirely different connection semantics
//
// Therefore, we separate the parsing logic:
// - SQL: Use dburl for standardized parsing
// - NoSQL: Use database-specific native parsing via their official drivers
//
// This approach is extensible: adding a new NoSQL database requires:
// 1. Add scheme prefix(es) to noSQLPrefixes list in isNoSQLURL()
// 2. Add case to switch statement in parseNoSQLConnection()
// 3. Implement database-specific name extractor function
func ParseConnectionString(url string) (*dburl.URL, error) {
	if isNoSQLURL(url) {
		return parseNoSQLConnection(url)
	}
	return dburl.Parse(url)
}

// isNoSQLURL checks if a connection URL is for a NoSQL database.
//
// Extensibility: To add support for a new NoSQL database, add its connection
// scheme prefix(es) to the noSQLPrefixes list. For example:
// - Redis: "redis://", "rediss://"
// - Cassandra: "cassandra://"
// - DynamoDB: "dynamodb://"
func isNoSQLURL(url string) bool {
	noSQLPrefixes := []string{
		"mongodb://",
		"mongodb+srv://",
	}

	for _, prefix := range noSQLPrefixes {
		if strings.HasPrefix(url, prefix) {
			return true
		}
	}
	return false
}

// parseNoSQLConnection parses NoSQL database connection strings.
//
// Design Decision: Minimal Abstraction Approach
// Unlike SQL databases where we parse the full connection string into components
// (username, password, host, port), NoSQL drivers handle their own connection
// string parsing internally. This function only extracts the minimal metadata
// needed for the UI:
// - Driver name (for routing to correct driver implementation)
// - Database name (for display in connection list)
//
// The full connection string (DSN) is passed unchanged to the native driver,
// which handles all authentication, connection pooling, and driver-specific options.
//
// Extensibility: To add a new NoSQL database:
// 1. Add a case to the switch statement
// 2. Set the driver name (must match constant in drivers/constants.go)
// 3. Implement a database-specific name extraction function
// 4. Handle the default case for unsupported schemes
func parseNoSQLConnection(urlstr string) (*dburl.URL, error) {
	var driver, dbName string

	switch {
	case strings.HasPrefix(urlstr, "mongodb://"), strings.HasPrefix(urlstr, "mongodb+srv://"):
		driver = "mongodb"
		dbName = extractMongoDBName(urlstr)

	default:
		return nil, errors.New("unsupported NoSQL database scheme")
	}

	// Return dburl.URL for consistency with SQL parsing, but note:
	// - Driver field identifies which NoSQLDriver implementation to use
	// - DSN field contains the original connection string for the native driver
	// - Path field (from embedded url.URL) contains the extracted database name for UI display
	// - The embedded url.URL is not fully populated since NoSQL drivers parse their own connection strings
	result := &dburl.URL{
		URL: url.URL{
			Path: dbName,
		},
		Driver: driver,
		DSN:    urlstr,
	}
	return result, nil
}

// extractMongoDBName extracts the database name from a MongoDB connection string.
//
// MongoDB connection string format:
// mongodb://[username:password@]host[:port][/database][?options]
// mongodb+srv://[username:password@]host[/database][?options]
//
// Examples:
// - "mongodb://localhost:27017/mydb" -> "mydb"
// - "mongodb://user:pass@localhost:27017/mydb?authSource=admin" -> "mydb"
// - "mongodb+srv://cluster.mongodb.net/mydb" -> "mydb"
// - "mongodb://localhost:27017" -> "" (no database specified)
func extractMongoDBName(urlstr string) string {
	// Remove scheme prefix to get the connection details
	urlstr = strings.TrimPrefix(urlstr, "mongodb+srv://")
	urlstr = strings.TrimPrefix(urlstr, "mongodb://")

	// Split on first "/" to separate host from database path
	parts := strings.SplitN(urlstr, "/", 2)
	if len(parts) < 2 {
		return "" // No database specified in connection string
	}

	// Extract database name and remove query parameters if present
	dbPart := parts[1]
	if idx := strings.Index(dbPart, "?"); idx != -1 {
		dbPart = dbPart[:idx]
	}

	return dbPart
}

// ExtractDatabaseName extracts the database name from a parsed connection URL.
// SQL databases use dburl's Normalize method, while NoSQL databases use the Path field.
func ExtractDatabaseName(parsed *dburl.URL) string {
	// Check if this is a NoSQL database by checking the driver
	// NoSQL databases have their database name extracted in parseNoSQLConnection
	// and stored in the Path field
	if drivers.IsNoSQLProvider(parsed.Driver) {
		return parsed.Path
	}

	// SQL databases use Normalize to extract database name
	// Format: "driver,host,port,database"
	parts := strings.Split(parsed.Normalize(",", "NULL", 0), ",")
	if len(parts) > 3 && parts[3] != "NULL" {
		return parts[3]
	}
	return ""
}

func ContainsCommand(commands []commands.Command, command commands.Command) bool {
	for _, cmd := range commands {
		if cmd == command {
			return true
		}
	}
	return false
}

// GetFreePort asks the kernel for a free port.
func GetFreePort() (string, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return "", err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return "", err
	}
	defer l.Close()

	return strconv.Itoa(l.Addr().(*net.TCPAddr).Port), nil
}

// WaitForPort waits for a port to be open.
func WaitForPort(ctx context.Context, port string) error {
	dialer := &net.Dialer{
		Timeout: 500 * time.Millisecond,
	}

	for i := 0; i < 10; i++ {
		conn, err := dialer.DialContext(ctx, "tcp", "localhost:"+port)
		if err == nil {
			_ = conn.Close()
			return nil
		}

		select {
		case <-time.After(500 * time.Millisecond):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return errors.New("Timeout waiting for port " + port)
}
