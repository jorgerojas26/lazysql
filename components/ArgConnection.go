package components

import (
	"fmt"
	"os"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/models"
)

func InitFromArg(connectionString string) {
	parsed, err := helpers.ParseConnectionString(connectionString)
	if err != nil {
		fmt.Printf("Could not parse connection string: %s\n", err)
		os.Exit(1)
	}
	connection := models.Connection{
		Name:     connectionString,
		Provider: parsed.Driver,
		DBName:   connectionString,
		URL:      connectionString,
	}
	var newDbDriver drivers.Driver
	switch connection.Provider {
	case drivers.DriverMySQL:
		newDbDriver = &drivers.MySQL{}
	case drivers.DriverPostgres:
		newDbDriver = &drivers.Postgres{}
	case drivers.DriverSqlite:
		newDbDriver = &drivers.SQLite{}
	}
	err = newDbDriver.Connect(connection.URL)

	if err != nil {
		fmt.Printf("Could not connect to database %s: %s\n", connectionString, err)
		os.Exit(1)
	}
	MainPages.AddPage(connection.URL, NewHomePage(connection, newDbDriver).Flex, true, true)
}
