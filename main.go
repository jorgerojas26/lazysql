package main

import (
	"flag"
	"io"
	mysqllog "log"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/components"
	"github.com/jorgerojas26/lazysql/helpers/log"

	"github.com/go-sql-driver/mysql"
)

func main() {
	rawLogLvl := flag.String("loglvl", "info", "Log level")
	logFile := flag.String("logfile", "", "Log file")
	flag.Parse()

	logLvl, err := log.ParseLogLevel(*rawLogLvl)
	if err != nil {
		panic(err)
	}
	log.SetLevel(logLvl)
	log.SetFile(*logFile)
	log.Info("Starting LazySQL...", nil)

	mysql.SetLogger(mysqllog.New(io.Discard, "", 0))

	if err := app.App.
		SetRoot(components.MainPages, true).
		EnableMouse(true).
		Run(); err != nil {
		panic(err)
	}
}
