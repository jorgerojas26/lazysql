// Command lazysql-benchmark runs the local network-performance contract
// matrix without connecting to a database.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/jorgerojas26/lazysql/internal/benchmarks"
)

func main() {
	format := flag.String("format", "table", "output format: table or json")
	rttValue := flag.String("rtt", "0ms,50ms,100ms", "comma-separated artificial RTT durations")
	flag.Parse()

	rtts, err := benchmarks.ParseRTTs(*rttValue)
	if err != nil {
		fail(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	results, err := benchmarks.RunMatrix(ctx, rtts)
	if err != nil {
		fail(err)
	}

	switch *format {
	case "json":
		err = benchmarks.WriteJSON(os.Stdout, results)
	case "table":
		err = benchmarks.WriteTable(os.Stdout, results)
	default:
		fail(fmt.Errorf("unknown output format %q", *format))
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
