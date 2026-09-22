package components

import (
	"testing"

	"github.com/jorgerojas26/lazysql/models"
)

func TestShowAllDatabasesChecked(t *testing.T) {
	testCases := []struct {
		name     string
		conn     models.Connection
		expected bool
	}{
		{
			name: "DBName pinned -> unchecked, regardless of URL",
			conn: models.Connection{
				URL:    "postgres://user:pass@localhost:5432/mydb",
				DBName: "mydb",
			},
			expected: false,
		},
		{
			name: "DBName empty, URL has embedded database -> checked",
			conn: models.Connection{
				URL: "postgres://user:pass@localhost:5432/mydb",
			},
			expected: true,
		},
		{
			name: "DBName empty, URL has no embedded database -> unchecked",
			conn: models.Connection{
				URL: "postgres://user:pass@localhost:5432/",
			},
			expected: false,
		},
		{
			name: "DBName empty, MSSQL URL with embedded database -> checked",
			conn: models.Connection{
				URL: "mssql://user:pass@localhost/DADOSADV",
			},
			expected: true,
		},
		{
			name: "invalid URL -> unchecked (fail safe)",
			conn: models.Connection{
				URL: "not a valid url :: at all",
			},
			expected: false,
		},
		{
			name: "empty connection -> unchecked",
			conn: models.Connection{
				URL: "",
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := showAllDatabasesChecked(tc.conn)
			if got != tc.expected {
				t.Errorf("showAllDatabasesChecked(%+v) = %v, want %v", tc.conn, got, tc.expected)
			}
		})
	}
}
