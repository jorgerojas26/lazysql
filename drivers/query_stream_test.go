package drivers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func queryStreamRows(count int) *sqlmock.Rows {
	rows := sqlmock.NewRows([]string{"id"})
	for i := 1; i <= count; i++ {
		rows.AddRow(i)
	}
	return rows
}

func TestSQLiteStreamQueryFlushesBeforeCompleteResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id").WillReturnRows(queryStreamRows(150))

	streamer := &SQLite{Connection: db}
	var batches int
	result, err := streamer.StreamQuery(context.Background(), "SELECT id", 0, func(batch QueryBatch) error {
		batches++
		if batches == 1 && len(batch.Rows) != queryStreamBatchSize {
			t.Fatalf("first batch has %d rows, want %d", len(batch.Rows), queryStreamBatchSize)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("StreamQuery() error = %v", err)
	}
	if result.Rows != 150 {
		t.Fatalf("StreamQuery() consumed %d rows, want 150", result.Rows)
	}
	if batches != 2 {
		t.Fatalf("got %d batches, want 2", batches)
	}
	if result.Truncated {
		t.Fatal("unlimited stream was marked truncated")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type slowStreamDriver struct{}

type slowStreamConn struct{}

type slowStreamRows struct {
	ctx   context.Context
	index int
}

func (slowStreamDriver) Open(string) (driver.Conn, error)   { return &slowStreamConn{}, nil }
func (*slowStreamConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not supported") }
func (*slowStreamConn) Close() error                        { return nil }
func (*slowStreamConn) Begin() (driver.Tx, error)           { return nil, errors.New("not supported") }
func (conn *slowStreamConn) QueryContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	return &slowStreamRows{ctx: ctx}, nil
}
func (*slowStreamRows) Columns() []string { return []string{"id"} }
func (*slowStreamRows) Close() error      { return nil }
func (rows *slowStreamRows) Next(dest []driver.Value) error {
	if rows.index == 0 {
		dest[0] = int64(1)
		rows.index++
		return nil
	}
	if rows.index == 1 {
		select {
		case <-time.After(200 * time.Millisecond):
		case <-rows.ctx.Done():
			return rows.ctx.Err()
		}
		dest[0] = int64(2)
		rows.index++
		return nil
	}
	return io.EOF
}

var registerSlowStreamDriver sync.Once

func TestSQLiteStreamQueryFlushesSlowBatchByTime(t *testing.T) {
	const driverName = "lazysql-slow-stream-test"
	// database/sql permits each driver name to be registered once.
	registerSlowStreamDriver.Do(func() { sql.Register(driverName, slowStreamDriver{}) })
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	streamer := &SQLite{Connection: db}
	firstBatch := make(chan struct{})
	finished := make(chan struct{})
	var result QueryStreamResult
	var streamErr error
	go func() {
		result, streamErr = streamer.StreamQuery(ctx, "SELECT id", 0, func(batch QueryBatch) error {
			if len(batch.Rows) != 1 {
				t.Errorf("slow first batch has %d rows, want 1", len(batch.Rows))
			}
			close(firstBatch)
			return nil
		})
		close(finished)
	}()

	select {
	case <-firstBatch:
	case <-time.After(2 * time.Second):
		t.Fatal("timer did not flush a partial batch while the next row was slow")
	}
	select {
	case <-finished:
		t.Fatal("stream completed before the delayed row was released")
	default:
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled slow stream did not finish")
	}
	if !errors.Is(streamErr, context.Canceled) {
		t.Fatalf("stream error = %v, want context.Canceled (result=%+v)", streamErr, result)
	}
}

func TestSQLiteStreamQueryCapUsesOneLookaheadRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id").WillReturnRows(queryStreamRows(4))

	streamer := &SQLite{Connection: db}
	var emitted [][]string
	result, err := streamer.StreamQuery(context.Background(), "SELECT id", 2, func(batch QueryBatch) error {
		emitted = append(emitted, batch.Rows...)
		return nil
	})
	if err != nil {
		t.Fatalf("StreamQuery() error = %v", err)
	}
	if result.Rows != 2 {
		t.Fatalf("StreamQuery() consumed %d displayed rows, want 2", result.Rows)
	}
	if !result.Truncated {
		t.Fatal("result with a lookahead row was not marked truncated")
	}
	if len(emitted) != 2 || emitted[0][0] != "1" || emitted[1][0] != "2" {
		t.Fatalf("emitted rows = %v, want [[1] [2]]", emitted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteStreamQueryCancellationPropagates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectQuery("SELECT id").WillReturnRows(queryStreamRows(150))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamer := &SQLite{Connection: db}
	result, err := streamer.StreamQuery(ctx, "SELECT id", 0, func(batch QueryBatch) error {
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("StreamQuery() error = %v, want context.Canceled", err)
	}
	if result.Rows != queryStreamBatchSize {
		t.Fatalf("StreamQuery() consumed %d rows before cancellation, want %d", result.Rows, queryStreamBatchSize)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteStreamQueryPreservesRowsBeforeDriverError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2).RowError(1, errors.New("connection lost"))
	mock.ExpectQuery("SELECT id").WillReturnRows(rows)

	streamer := &SQLite{Connection: db}
	var emitted [][]string
	result, err := streamer.StreamQuery(context.Background(), "SELECT id", 0, func(batch QueryBatch) error {
		emitted = append(emitted, batch.Rows...)
		return nil
	})
	if err == nil || err.Error() != "connection lost" {
		t.Fatalf("StreamQuery() error = %v, want connection lost", err)
	}
	if result.Rows != 1 || len(emitted) != 1 {
		t.Fatalf("result rows/emitted = %d/%d, want 1/1", result.Rows, len(emitted))
	}
}

func TestSQLiteStreamQueryRejectsNilBatchHandler(t *testing.T) {
	streamer := &SQLite{}
	_, err := streamer.StreamQuery(context.Background(), "SELECT 1", 0, nil)
	if !errors.Is(err, errNilQueryBatchHandler) {
		t.Fatalf("StreamQuery() error = %v, want nil-handler error", err)
	}
}
