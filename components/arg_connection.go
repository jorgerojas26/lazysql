package components

import (
	"fmt"
	"strings"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/models"
)

func InitFromArg(connectionString string) error {
	parsed, err := helpers.ParseConnectionString(connectionString)
	if err != nil {
		return fmt.Errorf("Could not parse connection string: %s", err)
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
	}

	var newDBDriver drivers.Driver
	switch connection.Provider {
	case drivers.DriverMySQL:
		newDBDriver = &drivers.MySQL{}
	case drivers.DriverPostgres:
		newDBDriver = &drivers.Postgres{}
	case drivers.DriverSqlite:
		newDBDriver = &drivers.SQLite{}
	}

	err = newDBDriver.Connect(connection.URL)
	if err != nil {
		return fmt.Errorf("Could not connect to database %s: %s", connectionString, err)
	}
	MainPages.AddAndSwitchToPage(connection.URL, NewHomePage(connection, newDBDriver).Flex, true)

	return nil
}
