package helpers

import (
	"github.com/xo/dburl"

	"github.com/jorgerojas26/lazysql/commands"
)

func ParseConnectionString(url string) (*dburl.URL, error) {
	return dburl.Parse(url)
}

func ContainsCommand(commands []commands.Command, command commands.Command) bool {
	for _, cmd := range commands {
		if cmd == command {
			return true
		}
	}
	return false
}
