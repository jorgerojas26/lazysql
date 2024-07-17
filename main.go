package main

import (
	"io"
	"log"
	"os"

	"github.com/go-sql-driver/mysql"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/components"
)

var version = "dev"

func main() {
	err := mysql.SetLogger(log.New(io.Discard, "", 0))
	if err != nil {
		panic(err)
	}

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
