package main

import (
	"flag"
	"fmt"
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
	logLevel := flag.String("loglevel", "info", "Log level")
	logFile := flag.String("logfile", "", "Log file")
	flag.Parse()

	slogLevel, err := logger.ParseLogLevel(*logLevel)
	if err != nil {
		log.Fatalf("Error parsing log level: %v", err)
	}
	logger.SetLevel(slogLevel)

	if *logFile != "" {
		if err := logger.SetFile(*logFile); err != nil {
			log.Fatalf("Error setting log file: %v", err)
		}
	}

	logger.Info("Starting LazySQL...", nil)

	if err := mysql.SetLogger(log.New(io.Discard, "", 0)); err != nil {
		log.Fatalf("Error setting MySQL logger: %v", err)
	}

	// Check if "version" arg is passed.
	argsWithProg := os.Args

	if len(argsWithProg) > 1 {
		switch argsWithProg[1] {
		case "version":
			fmt.Println("LazySQL version: ", version)
			os.Exit(0)
		default:
			err := components.InitFromArg(argsWithProg[1])
			if err != nil {
				fmt.Fprintln(os.Stderr, err) 
				os.Exit(1)
			}
		}
	}

	if err = app.App.Run(components.MainPages); err != nil {
		log.Fatalf("Error running app: %v", err)
	}
}
