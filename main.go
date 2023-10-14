package main

import (
	"io"
	"lazysql/app"
	"lazysql/pages"
	"log"

	"github.com/go-sql-driver/mysql"
)

func main() {
	mysql.SetLogger(log.New(io.Discard, "", 0))
	if err := app.App.SetRoot(pages.AllPages, true).Run(); err != nil {
		panic(err)
	}
}
