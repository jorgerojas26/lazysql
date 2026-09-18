package drivers

import (
	"database/sql"

	"github.com/jorgerojas26/lazysql/models"
)

func applyConnectionPoolConfig(connection *sql.DB, config models.ConnectionPoolConfig) error {
	effective, err := config.Normalize()
	if err != nil {
		return err
	}

	connection.SetMaxOpenConns(effective.MaxOpenConnections)
	connection.SetMaxIdleConns(effective.MaxIdleConnections)
	return nil
}

func applySQLitePoolConfig(connection *sql.DB) {
	connection.SetMaxOpenConns(1)
	connection.SetMaxIdleConns(1)
}
