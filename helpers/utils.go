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
// SQL databases use dburl for standardized parsing.
// NoSQL databases use database-specific parsing via their native drivers.
func ParseConnectionString(url string) (*dburl.URL, error) {
	if isNoSQLURL(url) {
		return parseNoSQLConnection(url)
	}
	return dburl.Parse(url)
}

// isNoSQLURL checks if a connection URL is for a NoSQL database.
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
// Extracts driver name and database name for UI display.
// The full connection string is passed unchanged to the native driver.
func parseNoSQLConnection(urlstr string) (*dburl.URL, error) {
	var driver, dbName string

	switch {
	case strings.HasPrefix(urlstr, "mongodb://"), strings.HasPrefix(urlstr, "mongodb+srv://"):
		driver = "mongodb"
		dbName = extractMongoDBName(urlstr)

	default:
		return nil, errors.New("unsupported NoSQL database scheme")
	}

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
// Returns empty string if no database is specified.
func extractMongoDBName(urlstr string) string {
	urlstr = strings.TrimPrefix(urlstr, "mongodb+srv://")
	urlstr = strings.TrimPrefix(urlstr, "mongodb://")

	parts := strings.SplitN(urlstr, "/", 2)
	if len(parts) < 2 {
		return ""
	}

	dbPart := parts[1]
	if idx := strings.Index(dbPart, "?"); idx != -1 {
		dbPart = dbPart[:idx]
	}

	return dbPart
}

// ExtractDatabaseName extracts the database name from a parsed connection URL.
func ExtractDatabaseName(parsed *dburl.URL) string {
	if drivers.IsNoSQLProvider(parsed.Driver) {
		return parsed.Path
	}

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
