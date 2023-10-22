package main

import (
	"io"
	"lazysql/app"
	"lazysql/components"
	"log"

	"github.com/go-sql-driver/mysql"
)

func main() {
	mysql.SetLogger(log.New(io.Discard, "", 0))

	if err := app.App.SetRoot(components.MainPages, true).Run(); err != nil {
		panic(err)
	}
}
