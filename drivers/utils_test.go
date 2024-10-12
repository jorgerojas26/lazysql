package drivers

import (
	"errors"
	"strings"
	"testing"

	gomock "github.com/DATA-DOG/go-sqlmock"

	"github.com/jorgerojas26/lazysql/models"
)

func Test_queriesInTransaction(t *testing.T) {
	tests := []struct {
		setMockExpectations func(mock gomock.Sqlmock)
		assertErr           func(t *testing.T, err error)
		name                string
		queries             []models.Query
	}{
		{
			name: "successful transaction",
			queries: []models.Query{
				{Query: "SELECT * FROM table"},
			},
			setMockExpectations: func(mock gomock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("SELECT \\* FROM table").WillReturnResult(gomock.NewResult(0, 0))
				mock.ExpectCommit()
			},
		},
		{
			name: "unsuccessful commit",
			queries: []models.Query{
				{Query: "SELECT * FROM table"},
			},
			setMockExpectations: func(mock gomock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("SELECT \\* FROM table").WillReturnResult(gomock.NewResult(0, 0))
				mock.ExpectCommit().WillReturnError(errors.New("commit error"))
			},
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				if !strings.Contains(err.Error(), "commit error") {
					t.Errorf("expected error to contain 'commit error', got %v", err)
				}
			},
		},
		{
			name: "failed query",
			queries: []models.Query{
				{Query: "SELECT * FROM table"},
			},
			setMockExpectations: func(mock gomock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("SELECT \\* FROM table").WillReturnError(errors.New("query error"))
				mock.ExpectRollback()
			},
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				if !strings.Contains(err.Error(), "query error") {
					t.Errorf("expected error to contain 'commit error', got %v", err)
				}
			},
		},
		{
			name: "failed 2nd query of three",
			queries: []models.Query{
				{Query: "SELECT * FROM table"},
				{Query: "SELECT * FROM table"},
				{Query: "SELECT * FROM table"},
			},
			setMockExpectations: func(mock gomock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("SELECT \\* FROM table").WillReturnResult(gomock.NewResult(0, 0))
				mock.ExpectExec("SELECT \\* FROM table").WillReturnError(errors.New("query error"))
				mock.ExpectRollback()
			},
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				if !strings.Contains(err.Error(), "query error") {
					t.Errorf("expected error to contain 'commit error', got %v", err)
				}
			},
		},
		{
			name: "failed query and rollback",
			queries: []models.Query{
				{Query: "SELECT * FROM table"},
			},
			setMockExpectations: func(mock gomock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("SELECT \\* FROM table").WillReturnError(errors.New("query error"))
				mock.ExpectRollback().WillReturnError(errors.New("rollback error"))
			},
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				errMsg := err.Error()
				if !strings.Contains(errMsg, "query error") {
					t.Errorf("expected error to contain 'commit error', got %v", err)
				}
				if !strings.Contains(errMsg, "rollback error") {
					t.Errorf("expected error to contain 'rollback error', got %v", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := gomock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			tt.setMockExpectations(mock)
			queryErr := queriesInTransaction(db, tt.queries)
			if tt.assertErr != nil {
				tt.assertErr(t, queryErr)
			}
		})
	}
}
