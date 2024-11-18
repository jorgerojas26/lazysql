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
	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
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
			connectionString := argsWithProg[1]
			parsed, err := helpers.ParseConnectionString(connectionString)
			if err != nil {
				fmt.Printf("Could not parse connection string: %s\n", err)
				os.Exit(1)
			}
			connection := models.Connection{
				Name:     connectionString,
				Provider: parsed.Driver,
				DBName:   connectionString,
				URL:      connectionString,
			}
			newDbDriver := &drivers.SQLite{}
			err = newDbDriver.Connect(connection.URL)
			if err != nil {
				fmt.Printf("Could not connect to database %s: %s\n", connectionString, err)
				os.Exit(1)
			}
			components.MainPages.AddPage(connection.URL, components.NewHomePage(connection, newDbDriver).Flex, true, true)
		}
	}

	if err = app.App.Run(components.MainPages); err != nil {
		log.Fatalf("Error running app: %v", err)
	}
}
