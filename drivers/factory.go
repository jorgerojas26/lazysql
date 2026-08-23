package drivers

import (
	"fmt"
)

// NewSQLDriver creates a SQL driver instance for the given provider.
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

// NewNoSQLDriver creates a NoSQL driver instance for the given provider.
func NewNoSQLDriver(provider string) (NoSQLDriver, error) {
	switch provider {
	case DriverMongoDB:
		return &MongoDB{}, nil
	default:
		return nil, fmt.Errorf("unsupported NoSQL driver: %s", provider)
	}
}

// IsNoSQLProvider reports whether the provider is a NoSQL database.
func IsNoSQLProvider(provider string) bool {
	return provider == DriverMongoDB
}

// TestConnection tests a connection for both SQL and NoSQL providers.
func TestConnection(provider, connectionString string) error {
	if IsNoSQLProvider(provider) {
		driver, err := NewNoSQLDriver(provider)
		if err != nil {
			return err
		}
		return driver.TestConnection(connectionString)
	}

	driver, err := NewSQLDriver(provider)
	if err != nil {
		return err
	}
	return driver.TestConnection(connectionString)
}
