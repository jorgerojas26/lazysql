package helpers

import (
	"strings"

	"github.com/xo/dburl"
)

func ParseConnectionString(url string) (*dburl.URL, error) {
	return dburl.Parse(url)
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
