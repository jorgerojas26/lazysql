package main

import (
	"flag"
	"io"
	"log"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/components"
	"github.com/jorgerojas26/lazysql/helpers/logger"

	"github.com/go-sql-driver/mysql"
)

func main() {
	rawLogLvl := flag.String("loglvl", "info", "Log level")
	logFile := flag.String("logfile", "", "Log file")
	flag.Parse()

	logLvl, err := logger.ParseLogLevel(*rawLogLvl)
	if err != nil {
		panic(err)
	}
	logger.SetLevel(logLvl)
	logger.SetFile(*logFile)
	logger.Info("Starting LazySQL...", nil)

	mysql.SetLogger(log.New(io.Discard, "", 0))

	if err := app.App.
		SetRoot(components.MainPages, true).
		EnableMouse(true).
		Run(); err != nil {
		panic(err)
	}
}
