package main

import (
	"io"
	"log"
	"os"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/components"

	"github.com/go-sql-driver/mysql"
)

var version = "dev"

func main() {
	mysql.SetLogger(log.New(io.Discard, "", 0))

	// check if "version" arg is passed
	argsWithProg := os.Args

	if len(argsWithProg) > 1 {
		switch argsWithProg[1] {
		case "version":
			println("LazySQL version: ", version)
			os.Exit(0)
		}
	}

	if err := app.App.
		SetRoot(components.MainPages, true).
		EnableMouse(true).
		Run(); err != nil {
		panic(err)
	}
}
