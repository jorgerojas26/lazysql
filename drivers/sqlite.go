package drivers

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	// import sqlite driver
	_ "modernc.org/sqlite"

	"github.com/jorgerojas26/lazysql/models"
)

type SQLite struct {
	Connection *sql.DB
	Provider   string
}

func (db *SQLite) TestConnection(urlstr string) (err error) {
	return db.Connect(urlstr)
}

func (db *SQLite) Connect(urlstr string) (err error) {
	db.SetProvider("sqlite3")

	db.Connection, err = sql.Open("sqlite", urlstr)
	if err != nil {
		return err
	}

	err = db.Connection.Ping()
	if err != nil {
		return err
	}

	return nil
}

func (db *SQLite) GetDatabases() ([]string, error) {
	var databases []string

	rows, err := db.Connection.Query("SELECT file FROM pragma_database_list WHERE name='main'")
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var database string
		err := rows.Scan(&database)
		if err != nil {
			return nil, err
		}

		split := strings.Split(database, "/")
		dbName := split[len(split)-1]

		databases = append(databases, dbName)
	}

	return databases, nil
}

func (db *SQLite) GetTables(database string) (map[string][]string, error) {
	rows, err := db.Connection.Query("SELECT name FROM sqlite_master WHERE type='table'")

	tables := make(map[string][]string)

	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var table string
		err = rows.Scan(&table)
		if err != nil {
			return nil, err
		}

		tables[database] = append(tables[database], table)
	}

	return tables, nil
}

func (db *SQLite) GetTableColumns(_, table string) (results [][]string, err error) {
	rows, err := db.Connection.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results = append(results, columns[1:])

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		err = rows.Scan(rowValues...)
		if err != nil {
			return nil, err
		}

		var row []string

		for _, col := range rowValues {
			if col == nil {
				row = append(row, "NULL")
			} else {
				row = append(row, string(*col.(*sql.RawBytes)))
			}
		}

		results = append(results, row[1:])
	}

	return
}

func (db *SQLite) GetConstraints(table string) (results [][]string, err error) {
	rows, err := db.Connection.Query("SELECT sql FROM sqlite_master WHERE type='table' AND name = '" + table + "'")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results = append(results, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		err = rows.Scan(rowValues...)
		if err != nil {
			return nil, err
		}

		var row []string
		for _, col := range rowValues {
			if col == nil {
				row = append(row, "NULL")
			} else {
				row = append(row, string(*col.(*sql.RawBytes)))
			}
		}

		results = append(results, row)
	}

	return
}

func (db *SQLite) GetForeignKeys(table string) (results [][]string, err error) {
	rows, err := db.Connection.Query("PRAGMA foreign_key_list(" + table + ")")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results = append(results, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		err = rows.Scan(rowValues...)
		if err != nil {
			return nil, err
		}

		var row []string
		for _, col := range rowValues {
			if col == nil {
				row = append(row, "NULL")
			} else {
				row = append(row, string(*col.(*sql.RawBytes)))
			}
		}

		results = append(results, row)
	}

	return
}

func (db *SQLite) GetIndexes(table string) (results [][]string, err error) {
	rows, err := db.Connection.Query("PRAGMA index_list(" + table + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results = append(results, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		err = rows.Scan(rowValues...)
		if err != nil {
			return nil, err
		}

		var row []string
		for _, col := range rowValues {
			if col == nil {
				row = append(row, "NULL")
			} else {
				row = append(row, string(*col.(*sql.RawBytes)))
			}
		}

		results = append(results, row)
	}

	return
}

func (db *SQLite) GetRecords(table, where, sort string, offset, limit int) (paginatedResults [][]string, totalRecords int, err error) {
	defaultLimit := 300

	isPaginationEnabled := offset >= 0 && limit >= 0

	if limit != 0 {
		defaultLimit = limit
	}

	query := fmt.Sprintf("SELECT * FROM %s s LIMIT %d,%d", table, offset, defaultLimit)

	if where != "" {
		query = fmt.Sprintf("SELECT * FROM %s %s LIMIT %d,%d", table, where, offset, defaultLimit)
	}

	if sort != "" {
		query = fmt.Sprintf("SELECT * FROM %s %s ORDER BY %s LIMIT %d,%d", table, where, sort, offset, defaultLimit)
	}

	paginatedRows, err := db.Connection.Query(query)
	if err != nil {
		return nil, 0, err
	}

	if isPaginationEnabled {
		queryWithoutLimit := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", table, where)

		rows := db.Connection.QueryRow(queryWithoutLimit)
		if err != nil {
			return nil, 0, err
		}

		err = rows.Scan(&totalRecords)
		if err != nil {
			return nil, 0, err
		}

		defer paginatedRows.Close()
	}

	columns, err := paginatedRows.Columns()
	if err != nil {
		return nil, 0, err
	}

	paginatedResults = append(paginatedResults, columns)

	for paginatedRows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		err = paginatedRows.Scan(rowValues...)
		if err != nil {
			return nil, 0, err
		}

		var row []string
		for _, col := range rowValues {
			if col == nil {
				row = append(row, "NULL")
			} else {
				row = append(row, string(*col.(*sql.RawBytes)))
			}
		}

		paginatedResults = append(paginatedResults, row)

	}

	return
}

func (db *SQLite) ExecuteQuery(query string) (results [][]string, err error) {
	rows, err := db.Connection.Query(query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results = append(results, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		err = rows.Scan(rowValues...)
		if err != nil {
			return nil, err
		}

		var row []string
		for _, col := range rowValues {
			if col == nil {
				row = append(row, "NULL")
			} else {
				row = append(row, string(*col.(*sql.RawBytes)))
			}
		}

		results = append(results, row)

	}

	return
}

func (db *SQLite) UpdateRecord(table, column, value, primaryKeyColumnName, primaryKeyValue string) error {
	query := fmt.Sprintf("UPDATE %s SET %s = \"%s\" WHERE %s = %s;", table, column, value, primaryKeyColumnName, primaryKeyValue)
	_, err := db.Connection.Exec(query)

	return err
}

func (db *SQLite) DeleteRecord(table, primaryKeyColumnName, primaryKeyValue string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = \"%s\"", table, primaryKeyColumnName, primaryKeyValue)
	_, err := db.Connection.Exec(query)

	return err
}

func (db *SQLite) ExecuteDMLStatement(query string) (result string, err error) {
	res, err := db.Connection.Exec(query)
	if err != nil {
		return "", err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d rows affected", rowsAffected), nil
}

func (db *SQLite) ExecutePendingChanges(changes []models.DbDmlChange, inserts []models.DbInsert) (err error) {
	queries := make([]string, 0, len(changes)+len(inserts))

	// This will hold grouped changes by their RowId and Table
	groupedUpdated := make(map[string][]models.DbDmlChange)
	groupedDeletes := make([]models.DbDmlChange, 0, len(changes))

	// Group changes by RowId and Table
	for _, change := range changes {
		if change.Type == "UPDATE" {
			key := fmt.Sprintf("%s|%s|%s", change.Table, change.PrimaryKeyColumnName, change.PrimaryKeyValue)
			groupedUpdated[key] = append(groupedUpdated[key], change)
		} else if change.Type == "DELETE" {
			groupedDeletes = append(groupedDeletes, change)
		}
	}

	// Combine individual changes to SQL statements
	for key, changes := range groupedUpdated {
		columns := []string{}

		// Split key into table and rowId
		splitted := strings.Split(key, "|")
		table := splitted[0]
		primaryKeyColumnName := splitted[1]
		primaryKeyValue := splitted[2]

		for _, change := range changes {
			columns = append(columns, fmt.Sprintf("%s='%s'", change.Column, change.Value))
		}

		// Merge all column updates
		updateClause := strings.Join(columns, ", ")

		query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = '%s';", table, updateClause, primaryKeyColumnName, primaryKeyValue)

		queries = append(queries, query)
	}

	for _, delete := range groupedDeletes {
		statementType := ""
		query := ""

		statementType = "DELETE FROM"

		query = fmt.Sprintf("%s %s WHERE %s = \"%s\"", statementType, delete.Table, delete.PrimaryKeyColumnName, delete.PrimaryKeyValue)

		if query != "" {
			queries = append(queries, query)
		}
	}

	for _, insert := range inserts {
		values := make([]string, 0, len(insert.Values))

		columnsToBeInserted := insert.Columns

		for _, value := range insert.Values {
			_, err := strconv.ParseFloat(value, 64)

			if err != nil {
				values = append(values, fmt.Sprintf("\"%s\"", value))
			} else {
				values = append(values, value)
			}
		}

		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", insert.Table, strings.Join(columnsToBeInserted, ", "), strings.Join(values, ", "))

		queries = append(queries, query)
	}

	tx, err := db.Connection.Begin()
	if err != nil {
		return err
	}

	for _, query := range queries {
		_, err = tx.Exec(query)
		if err != nil {
			return errors.Join(err, tx.Rollback())
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (db *SQLite) SetProvider(provider string) {
	db.Provider = provider
}

func (db *SQLite) GetProvider() string {
	return db.Provider
}
