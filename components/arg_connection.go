package components

import (
	"fmt"
	"strings"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/models"
)

func InitFromArg(connectionString string, readOnly bool) error {
	parsed, err := helpers.ParseConnectionString(connectionString)
	if err != nil {
		return fmt.Errorf("could not parse connection string: %s", err)
	}
	DBName := strings.Split(parsed.Normalize(",", "NULL", 0), ",")[3]

	if DBName == "NULL" {
		DBName = ""
	}

	connection := models.Connection{
		Name:     "",
		Provider: parsed.Driver,
		DBName:   DBName,
		URL:      connectionString,
		ReadOnly: readOnly,
	}

	// NoSQL databases require separate UI components which are not implemented yet
	if drivers.IsNoSQLProvider(connection.Provider) {
		return fmt.Errorf("noSQL database connections via command-line arguments are not yet supported, please use the connection management UI")
	}

	newDBDriver, err := drivers.NewSQLDriver(connection.Provider)
	if err != nil {
		return fmt.Errorf("could not create database driver: %w", err)
	}

	err = newDBDriver.Connect(connection.URL)
	if err != nil {
		return fmt.Errorf("could not connect to database %s: %s", connectionString, err)
	}
	mainPages.AddAndSwitchToPage(connection.URL, NewHomePage(connection, newDBDriver).Flex, true)

	return nil
}
