package drivers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestEditorDatabaseIsolation(t *testing.T) {
	for _, provider := range []string{"mysql", "mssql"} {
		for _, operation := range []string{"query", "dml", "stream"} {
			for _, failure := range []string{"", "use", "execute", "scan"} {
				t.Run(provider+"/"+operation+"/"+failure, func(t *testing.T) {
					pool, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
					if err != nil {
						t.Fatal(err)
					}
					defer pool.Close()
					database, use := "target`; --", "USE `target``; --`"
					var db Driver = &MySQL{Connection: pool, CurrentDatabase: "original"}
					if provider == "mssql" {
						database, use = "target]; --", "USE [target]]; --]"
						db = &MSSQL{Connection: pool, CurrentDatabase: "original"}
					}
					expectedErr := errors.New("test failure")
					switchDB := mock.ExpectExec(use)
					if failure == "use" {
						switchDB.WillReturnError(expectedErr)
					} else {
						switchDB.WillReturnResult(sqlmock.NewResult(0, 0))
						if operation != "dml" {
							q := mock.ExpectQuery("SELECT 1")
							if failure == "execute" {
								q.WillReturnError(expectedErr)
							} else {
								rows := sqlmock.NewRows([]string{"value"}).AddRow(1)
								if failure == "scan" {
									rows.RowError(0, expectedErr)
								}
								q.WillReturnRows(rows).RowsWillBeClosed()
							}
						} else {
							q := mock.ExpectExec("UPDATE items SET value = 1")
							if failure == "execute" || failure == "scan" {
								q.WillReturnError(expectedErr)
							} else {
								q.WillReturnResult(sqlmock.NewResult(0, 1))
							}
						}
					}
					// The physical connection must be discarded, not returned with USE state.
					mock.ExpectClose()
					switch operation {
					case "stream":
						_, err = db.(QueryStreamer).StreamQuery(context.Background(), database, "SELECT 1", 1, func(QueryBatch) error { return nil })
					case "query":
						_, _, err = db.ExecuteQuery(context.Background(), database, "SELECT 1")
					default:
						_, err = db.ExecuteDMLStatement(context.Background(), database, "UPDATE items SET value = 1")
					}
					if failure == "" && err != nil {
						t.Fatal(err)
					}
					if failure != "" && !errors.Is(err, expectedErr) {
						t.Fatalf("got %v, want %v", err, expectedErr)
					}
					if err := mock.ExpectationsWereMet(); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestEditorDMLDefaultDatabase(t *testing.T) {
	for _, database := range []string{"", "original"} {
		for _, provider := range []string{"mysql", "mssql", "postgres"} {
			t.Run(provider+"/"+database, func(t *testing.T) {
				pool, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
				if err != nil {
					t.Fatal(err)
				}
				defer pool.Close()
				var db Driver
				switch provider {
				case "mysql":
					db = &MySQL{Connection: pool, CurrentDatabase: "original"}
				case "mssql":
					db = &MSSQL{Connection: pool, CurrentDatabase: "original"}
				case "postgres":
					db = &Postgres{Connection: pool, CurrentDatabase: "original"}
				}
				mock.ExpectExec("UPDATE items SET value = 1").WillReturnResult(sqlmock.NewResult(0, 2))
				result, err := db.ExecuteDMLStatement(context.Background(), database, "UPDATE items SET value = 1")
				if err != nil || result != "2 rows affected" {
					t.Fatalf("got %q, %v", result, err)
				}
				if err := mock.ExpectationsWereMet(); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestEditorStreamCancellationDuringDatabaseSelection(t *testing.T) {
	pool, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	mock.ExpectExec("USE").WillDelayFor(time.Second).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectClose()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	db := &MySQL{Connection: pool, CurrentDatabase: "original"}
	_, err = db.StreamQuery(ctx, "other", "SELECT 1", 1, func(QueryBatch) error { return nil })
	if err == nil || ctx.Err() == nil {
		t.Fatalf("selection did not honor cancellation: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	if pool.Stats().InUse != 0 {
		t.Fatal("canceled selection leaked a connection")
	}
}
