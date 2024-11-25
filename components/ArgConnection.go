package components

import (
	"fmt"
	"os"
	"strings"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/models"
)

func InitFromArg(connectionString string) {
	parsed, err := helpers.ParseConnectionString(connectionString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not parse connection string: %s\n", err)
		os.Exit(1)
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
		fmt.Fprintf(os.Stderr, "Could not connect to database %s: %s\n", connectionString, err)
		os.Exit(1)
	}
	MainPages.AddAndSwitchToPage(connection.URL, NewHomePage(connection, newDbDriver).Flex, true)
}
