package drivers

import (
	"context"
	"database/sql"
	"database/sql/driver"
)

// editorConnection is implemented by both a pool and a reserved connection.
type editorConnection interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func switchedEditorConnection(pool *sql.DB, use string) (editorConnection, func(), error) {
	ctx := context.Background()
	conn, err := pool.Conn(ctx)
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() {
		// Close alone returns the changed session to the pool. MySQL's session
		// reset does not restore its default database. Discard it instead, even
		// after a failed query, so unrelated operations cannot use the wrong DB.
		// ErrBadConn is the database/sql contract for discarding a Raw connection.
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()
	}
	if _, err := conn.ExecContext(ctx, use); err != nil {
		cleanup()
		return nil, nil, err
	}
	return conn, cleanup, nil
}
