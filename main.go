package main

import (
	"io"
	"log"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/components"

	"github.com/go-sql-driver/mysql"
)

func main() {
	mysql.SetLogger(log.New(io.Discard, "", 0))

	if err := app.App.SetRoot(components.MainPages, true).Run(); err != nil {
		panic(err)
	}
}
