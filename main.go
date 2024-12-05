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
	flag.Usage = func() {
		f := flag.CommandLine.Output()
		fmt.Fprintln(f, "lazysql")
		fmt.Fprintln(f, "")
		fmt.Fprintf(f, "Usage:  %s [options] [connection_url]\n\n", os.Args[0])
		fmt.Fprintln(f, "  connection_url")
		fmt.Fprintln(f, "        database URL to connect to. Omit to start in picker mode")
		fmt.Fprintln(f, "")
		fmt.Fprintln(f, "Options:")
		flag.PrintDefaults()
	}
	printVersion := flag.Bool("version", false, "Show version")
	logLevel := flag.String("loglevel", "info", "Log level")
	logFile := flag.String("logfile", "", "Log file")
	flag.Parse()

	if *printVersion {
		println("LazySQL version: ", version)
		os.Exit(0)
	}

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

	args := flag.Args()

	switch len(args) {
	case 0:
		// nothing to do. Launch into the connection picker.
	case 1:
		err := components.InitFromArg(args[0])
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("Only a single connection is allowed")
	}

	if err = app.App.Run(components.MainPages); err != nil {
		log.Fatalf("Error running app: %v", err)
	}
}
