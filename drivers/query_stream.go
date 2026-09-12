package drivers

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"
)

const (
	queryStreamBatchSize  = 100
	queryStreamFlushEvery = 50 * time.Millisecond
)

// QueryBatch is a database-neutral batch of rows returned by a streamed query.
// Drivers emit column names with every batch so consumers do not need to know
// anything about the driver's database library.
type QueryBatch struct {
	Columns []string
	Rows    [][]string
}

// QueryStreamResult describes the rows consumed by StreamQuery. Columns is
// populated even when the query returns no rows. Truncated means the stream
// found one lookahead row beyond maxRows; that row is never included in a
// QueryBatch.
type QueryStreamResult struct {
	Columns   []string
	Rows      int
	Truncated bool
}

// QueryStreamer is the driver-facing contract used by interactive SQL editor
// queries. It deliberately contains no terminal UI types: the caller owns
// rendering and applies backpressure by returning from onBatch only after a
// batch has been handled.
type QueryStreamer interface {
	StreamQuery(ctx context.Context, query string, maxRows int, onBatch func(QueryBatch) error) (QueryStreamResult, error)
}

var errNilQueryBatchHandler = errors.New("query stream batch handler is nil")

// streamQuery incrementally reads rows from a database/sql result. A finite
// maxRows consumes at most maxRows rows for display plus one lookahead row to
// determine whether the result is truncated. maxRows == 0 means unlimited.
func streamQuery(ctx context.Context, connection *sql.DB, query string, maxRows int, onBatch func(QueryBatch) error) (QueryStreamResult, error) {
	if onBatch == nil {
		return QueryStreamResult{}, errNilQueryBatchHandler
	}
	if maxRows < 0 {
		return QueryStreamResult{}, errors.New("query stream max rows cannot be negative")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if connection == nil {
		return QueryStreamResult{}, errors.New("database connection is nil")
	}

	rows, err := connection.QueryContext(ctx, query)
	if err != nil {
		return QueryStreamResult{}, err
	}

	columns, err := rows.Columns()
	if err != nil {
		_ = rows.Close()
		return QueryStreamResult{}, err
	}

	result := QueryStreamResult{Columns: append([]string(nil), columns...)}
	batch := make([][]string, 0, queryStreamBatchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}

		current := QueryBatch{
			Columns: result.Columns,
			Rows:    batch,
		}
		if err := onBatch(current); err != nil {
			return err
		}
		batch = make([][]string, 0, queryStreamBatchSize)
		return nil
	}

	// flushBeforeReturn makes rows received before a scan/driver error visible
	// even when the batch did not reach the normal size or time threshold.
	finish := func(streamErr error) (QueryStreamResult, error) {
		if err := flush(); err != nil {
			return result, err
		}
		return result, streamErr
	}

	// database/sql exposes a blocking Next method, so row acquisition runs in a
	// small reader goroutine. The unbuffered row channel preserves backpressure;
	// the caller can still flush a partial batch on the timer while the reader
	// waits for a slow next row.
	type readResult struct {
		err       error
		truncated bool
	}
	rowCh := make(chan []string)
	readerDone := make(chan readResult, 1)
	stopReaderCh := make(chan struct{})
	var stopReaderOnce sync.Once
	stopReader := func() {
		stopReaderOnce.Do(func() {
			close(stopReaderCh)
			// Closing Rows unblocks drivers that do not independently notice
			// context cancellation while Next is waiting for network data.
			_ = rows.Close()
		})
	}

	go func() {
		defer rows.Close()
		sendResult := func(read readResult) {
			// Signal completion only after the connection has been released.
			_ = rows.Close()
			readerDone <- read
		}
		readRows := 0
		for {
			if err := ctx.Err(); err != nil {
				sendResult(readResult{err: err})
				return
			}
			if !rows.Next() {
				sendResult(readResult{err: rows.Err()})
				return
			}

			// Calling Next once after maxRows is intentional: the current row is
			// the lookahead used to distinguish an exact result from truncation.
			// Do not scan or emit it.
			if maxRows > 0 && readRows >= maxRows {
				sendResult(readResult{truncated: true})
				return
			}

			row, err := scanQueryRow(rows, len(columns))
			if err != nil {
				sendResult(readResult{err: err})
				return
			}
			readRows++

			select {
			case rowCh <- row:
			case <-ctx.Done():
				sendResult(readResult{err: ctx.Err()})
				return
			case <-stopReaderCh:
				return
			}
		}
	}()

	timer := time.NewTimer(queryStreamFlushEvery)
	defer timer.Stop()
	resetTimer := func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(queryStreamFlushEvery)
	}

	for {
		select {
		case row := <-rowCh:
			batch = append(batch, row)
			result.Rows++
			if len(batch) >= queryStreamBatchSize {
				if err := flush(); err != nil {
					stopReader()
					return result, err
				}
				resetTimer()
			}

		case read := <-readerDone:
			if read.err != nil {
				return finish(read.err)
			}
			result.Truncated = read.truncated
			return finish(nil)

		case <-timer.C:
			if len(batch) > 0 {
				if err := flush(); err != nil {
					stopReader()
					return result, err
				}
			}
			timer.Reset(queryStreamFlushEvery)

		case <-ctx.Done():
			stopReader()
			return finish(ctx.Err())
		}
	}
}

func scanQueryRow(rows *sql.Rows, columnCount int) ([]string, error) {
	values := make([]sql.RawBytes, columnCount)
	arguments := make([]any, columnCount)
	for i := range values {
		arguments[i] = &values[i]
	}

	if err := rows.Scan(arguments...); err != nil {
		return nil, err
	}

	row := make([]string, columnCount)
	for i, value := range values {
		if value == nil {
			row[i] = "NULL"
			continue
		}
		row[i] = string(value)
	}
	return row, nil
}
