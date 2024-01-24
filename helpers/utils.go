package helpers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/jorgerojas26/lazysql/models"
	"github.com/xo/dburl"
)

func ParseConnectionString(url string) (*dburl.URL, error) {
	return dburl.Parse(url)
}

func EscapeConnectionString(urlstr string) string {
	connectionString := urlstr

	splitConnection := strings.Split(connectionString, "://")

	if len(splitConnection) > 1 {
		if strings.Contains(connectionString, "?") {
			splitPath := strings.Split(splitConnection[1], "?")

			connectionString = splitConnection[0] + "://" + url.PathEscape(splitPath[0]) + "?" + splitPath[1]
		} else {
			connectionString = splitConnection[0] + "://" + url.PathEscape(splitConnection[1])
		}
	}
	return connectionString
}

func ConnectionToURL(connection *models.Connection) string {
	connectionUrl := fmt.Sprintf("%s://%s:%s@%s:%s", connection.Provider, connection.User, connection.Password, connection.Host, connection.Port)

	queryParams := connection.Query
	dbNamePath := connection.DBName

	if connection.Provider == "sqlite3" {
		connectionUrl = fmt.Sprintf("file:%s", connection.DSN)
	} else {
		if dbNamePath != "" {
			connectionUrl = fmt.Sprintf("%s/%s", connectionUrl, dbNamePath)
		}

		if queryParams != "" {
			connectionUrl = fmt.Sprintf("%s?%s", connectionUrl, queryParams)
		}

	}
	return connectionUrl
}

func GetDBName(url string) string {
	return strings.Split(url, "/")[1]
}

func ParsedDBName(path string) string {
	dbName := ""

	if path != "" {
		dbName = GetDBName(path)
	}

	return dbName
}
