package main

import (
	"flag"
	"io"
	"log"
	"os"

	"github.com/go-sql-driver/mysql"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/components"
	"github.com/jorgerojas26/lazysql/helpers/logger"
)

var version = "dev"

func main() {
	rawLogLvl := flag.String("loglvl", "info", "Log level")
	logFile := flag.String("logfile", "", "Log file")
	flag.Parse()

	logLvl, parseError := logger.ParseLogLevel(*rawLogLvl)
	if parseError != nil {
		panic(parseError)
	}
	logger.SetLevel(logLvl)

	if *logFile != "" {
		fileError := logger.SetFile(*logFile)
		if fileError != nil {
			panic(fileError)
		}
	}

	logger.Info("Starting LazySQL...", nil)

	mysqlError := mysql.SetLogger(log.New(io.Discard, "", 0))
	if mysqlError != nil {
		panic(mysqlError)
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
