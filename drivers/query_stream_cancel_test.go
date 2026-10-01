package drivers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"
)

// This result blocks in Next until its query context is canceled, as a server
// can do while producing the next row. Rows.Close alone cannot interrupt Next:
// database/sql holds the connection lock while Next is in progress.
type cancelStreamConnector struct{ waiting chan struct{} }
type cancelStreamConn struct{ waiting chan struct{} }
type cancelStreamRows struct {
	ctx     context.Context
	waiting chan struct{}
	sent    bool
}

func (c cancelStreamConnector) Connect(context.Context) (driver.Conn, error) {
	return &cancelStreamConn{waiting: c.waiting}, nil
}
func (c cancelStreamConnector) Driver() driver.Driver { return c }
func (c cancelStreamConnector) Open(string) (driver.Conn, error) {
	return c.Connect(context.Background())
}
func (*cancelStreamConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unsupported") }
func (*cancelStreamConn) Begin() (driver.Tx, error)           { return nil, errors.New("unsupported") }
func (*cancelStreamConn) Close() error                        { return nil }
func (c *cancelStreamConn) QueryContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	return &cancelStreamRows{ctx: ctx, waiting: c.waiting}, nil
}
func (*cancelStreamRows) Columns() []string { return []string{"id"} }
func (*cancelStreamRows) Close() error      { return nil }
func (r *cancelStreamRows) Next(values []driver.Value) error {
	if !r.sent {
		r.sent = true
		values[0] = int64(1)
		return nil
	}
	close(r.waiting)
	<-r.ctx.Done()
	return r.ctx.Err()
}

func TestStreamQueryCallbackFailureCancelsBlockedReader(t *testing.T) {
	waiting := make(chan struct{})
	db := sql.OpenDB(cancelStreamConnector{waiting: waiting})
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	want := errors.New("export disk full")
	finished := make(chan error, 1)
	go func() {
		_, err := streamQuery(ctx, db, "SELECT id", 0, func(QueryBatch) error { <-waiting; return want })
		finished <- err
	}()
	select {
	case err := <-finished:
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	case <-time.After(time.Second):
		cancel()
		<-finished
		t.Fatal("callback failure did not cancel the blocked database reader")
	}
	if got := db.Stats().InUse; got != 0 {
		t.Fatalf("leaked %d connections", got)
	}
}
