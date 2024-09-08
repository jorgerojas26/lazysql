package drivers

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	// import sqlite driver
	_ "modernc.org/sqlite"

	"github.com/jorgerojas26/lazysql/helpers/logger"
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
	db.SetProvider(SQLiteDriver)

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

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, rowsErr
	}

	defer rows.Close()

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
	if database == "" {
		return nil, errors.New("database name is required")
	}

	rows, err := db.Connection.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		return nil, err
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, rowsErr
	}

	defer rows.Close()

	tables := make(map[string][]string)

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
	if table == "" {
		return nil, errors.New("table name is required")
	}

	rows, err := db.Connection.Query(fmt.Sprintf("PRAGMA table_info(%s)", db.formatTableName(table)))
	if err != nil {
		return nil, err
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, rowsErr
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

func (db *SQLite) GetConstraints(_, table string) (results [][]string, err error) {
	if table == "" {
		return nil, errors.New("table name is required")
	}

	query := "SELECT sql FROM sqlite_master "
	query += "WHERE type='table' AND name = ?"

	rows, err := db.Connection.Query(query, table)
	if err != nil {
		return nil, err
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, rowsErr
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

func (db *SQLite) GetForeignKeys(_, table string) (results [][]string, err error) {
	if table == "" {
		return nil, errors.New("table name is required")
	}

	rows, err := db.Connection.Query("PRAGMA foreign_key_list(" + table + ")")
	if err != nil {
		return nil, err
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, rowsErr
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

func (db *SQLite) GetIndexes(_, table string) (results [][]string, err error) {
	if table == "" {
		return nil, errors.New("table name is required")
	}

	rows, err := db.Connection.Query("PRAGMA index_list(" + table + ")")
	if err != nil {
		return nil, err
	}

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, rowsErr
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

func (db *SQLite) GetRecords(_, table, where, sort string, offset, limit int) (paginatedResults [][]string, totalRecords int, err error) {
	if table == "" {
		return nil, 0, errors.New("table name is required")
	}

	if limit == 0 {
		limit = DefaultRowLimit
	}

	query := "SELECT * FROM "
	query += db.formatTableName(table)

	if where != "" {
		query += fmt.Sprintf(" %s", where)
	}

	if sort != "" {
		query += fmt.Sprintf(" ORDER BY %s", sort)
	}

	query += " LIMIT ?, ?"

	paginatedRows, err := db.Connection.Query(query, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	rowsErr := paginatedRows.Err()

	if rowsErr != nil {
		return nil, 0, rowsErr
	}

	defer paginatedRows.Close()

	countQuery := "SELECT COUNT(*) FROM "
	countQuery += db.formatTableName(table)

	rows := db.Connection.QueryRow(countQuery)

	if err != nil {
		return nil, 0, err
	}

	err = rows.Scan(&totalRecords)
	if err != nil {
		return nil, 0, err
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
			row = append(row, string(*col.(*sql.RawBytes)))
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

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, rowsErr
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

func (db *SQLite) UpdateRecord(_, table, column, value, primaryKeyColumnName, primaryKeyValue string) error {
	if table == "" {
		return errors.New("table name is required")
	}

	if column == "" {
		return errors.New("column name is required")
	}

	if value == "" {
		return errors.New("value is required")
	}

	if primaryKeyColumnName == "" {
		return errors.New("primary key column name is required")
	}

	if primaryKeyValue == "" {
		return errors.New("primary key value is required")
	}

	query := "UPDATE "
	query += db.formatTableName(table)
	query += fmt.Sprintf(" SET %s = ? WHERE %s = ?", column, primaryKeyColumnName)

	_, err := db.Connection.Exec(query, value, primaryKeyValue)

	return err
}

func (db *SQLite) DeleteRecord(_, table, primaryKeyColumnName, primaryKeyValue string) error {
	if table == "" {
		return errors.New("table name is required")
	}

	if primaryKeyColumnName == "" {
		return errors.New("primary key column name is required")
	}

	if primaryKeyValue == "" {
		return errors.New("primary key value is required")
	}

	query := "DELETE FROM "
	query += db.formatTableName(table)
	query += fmt.Sprintf(" WHERE %s = ?", primaryKeyColumnName)

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

func (db *SQLite) ExecutePendingChanges(changes []models.DbDmlChange) (err error) {
	var query []models.Query

	for _, change := range changes {
		columnNames := []string{}
		values := []interface{}{}
		valuesPlaceholder := []string{}

		for _, cell := range change.Values {
			switch cell.Type {
			case models.Empty, models.Null, models.String:
				columnNames = append(columnNames, cell.Column)
				valuesPlaceholder = append(valuesPlaceholder, "?")
			}
		}
		logger.Info("Column names", map[string]any{"columnNames": columnNames})

		for _, cell := range change.Values {
			switch cell.Type {
			case models.Empty:
				values = append(values, "")
			case models.Null:
				values = append(values, sql.NullString{})
			case models.String:
				values = append(values, cell.Value)
			}
		}

		switch change.Type {
		case models.DmlInsertType:
			queryStr := "INSERT INTO "
			queryStr += db.formatTableName(change.Table)
			queryStr += fmt.Sprintf(" (%s) VALUES (%s)", strings.Join(columnNames, ", "), strings.Join(valuesPlaceholder, ", "))

			newQuery := models.Query{
				Query: queryStr,
				Args:  values,
			}

			query = append(query, newQuery)
		case models.DmlUpdateType:
			queryStr := "UPDATE "
			queryStr += db.formatTableName(change.Table)

			for i, column := range columnNames {
				if i == 0 {
					queryStr += fmt.Sprintf(" SET `%s` = ?", column)
				} else {
					queryStr += fmt.Sprintf(", `%s` = ?", column)
				}
			}

			args := make([]interface{}, len(values))

			copy(args, values)

			queryStr += fmt.Sprintf(" WHERE %s = ?", change.PrimaryKeyColumnName)
			args = append(args, change.PrimaryKeyValue)

			newQuery := models.Query{
				Query: queryStr,
				Args:  args,
			}

			query = append(query, newQuery)
		case models.DmlDeleteType:
			queryStr := "DELETE FROM "
			queryStr += db.formatTableName(change.Table)
			queryStr += fmt.Sprintf(" WHERE %s = ?", change.PrimaryKeyColumnName)

			newQuery := models.Query{
				Query: queryStr,
				Args:  []interface{}{change.PrimaryKeyValue},
			}

			query = append(query, newQuery)
		}
	}

	trx, err := db.Connection.Begin()
	if err != nil {
		return err
	}

	for _, query := range query {
		logger.Info(query.Query, map[string]any{"args": query.Args})
		_, err := trx.Exec(query.Query, query.Args...)
		if err != nil {
			return err
		}
	}

	err = trx.Commit()
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

func (db *SQLite) formatTableName(table string) string {
	return fmt.Sprintf("`%s`", table)
}
