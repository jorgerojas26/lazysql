package components

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/models"
)

func InitFromArg(connectionString string) error {
	parsed, err := helpers.ParseConnectionString(connectionString)
	if err != nil {
		return errors.New(fmt.Sprintf("Could not parse connection string: %s", err))
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
		return errors.New(fmt.Sprintf("Could not connect to database %s: %s", connectionString, err))
	}
	MainPages.AddAndSwitchToPage(connection.URL, NewHomePage(connection, newDbDriver).Flex, true)
	return nil
}
