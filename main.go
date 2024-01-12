package main

import (
	"io"
	"log"

	"github.com/go-sql-driver/mysql"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/components"
)

func main() {
	if err := mysql.SetLogger(log.New(io.Discard, "", 0)); err != nil {
		panic(err)
	}

	if err := app.App.SetRoot(components.MainPages, true).Run(); err != nil {
		panic(err)
	}
}
