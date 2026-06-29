package drivers

import (
	"context"
	"fmt"
)

// NewSQLDriver creates a new SQL driver instance based on the provider string.
// This centralizes driver instantiation to avoid code duplication across components.
func NewSQLDriver(provider string) (Driver, error) {
	switch provider {
	case DriverMySQL:
		return &MySQL{}, nil
	case DriverPostgres:
		return &Postgres{}, nil
	case DriverSqlite:
		return &SQLite{}, nil
	case DriverMSSQL:
		return &MSSQL{}, nil
	default:
		return nil, fmt.Errorf("unsupported SQL driver: %s", provider)
	}
}

// NewNoSQLDriver creates a new NoSQL driver instance based on the provider string.
// This centralizes driver instantiation to avoid code duplication across components.
func NewNoSQLDriver(provider string) (NoSQLDriver, error) {
	switch provider {
	case DriverMongoDB:
		return &MongoDB{}, nil
	default:
		return nil, fmt.Errorf("unsupported NoSQL driver: %s", provider)
	}
}

// IsNoSQLProvider checks if a provider string represents a NoSQL database.
// This centralizes the logic for determining SQL vs NoSQL routing.
func IsNoSQLProvider(provider string) bool {
	switch provider {
	case DriverMongoDB:
		return true
	default:
		return false
	}
}

// TestConnection is a unified helper that tests connections for both SQL and NoSQL drivers.
// It routes to the appropriate driver type. Both SQL and NoSQL drivers currently don't
// accept context from callers (context parameter kept for future compatibility).
func TestConnection(_ context.Context, provider, connectionString string) error {
	if IsNoSQLProvider(provider) {
		driver, err := NewNoSQLDriver(provider)
		if err != nil {
			return err
		}
		return driver.TestConnection(connectionString)
	}

	// SQL drivers don't accept context (legacy pattern)
	driver, err := NewSQLDriver(provider)
	if err != nil {
		return err
	}
	return driver.TestConnection(connectionString)
}
