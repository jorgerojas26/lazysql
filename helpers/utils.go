package helpers

import (
	"github.com/xo/dburl"
)

func ParseConnectionString(url string) (*dburl.URL, error) {
	return dburl.Parse(url)
}
