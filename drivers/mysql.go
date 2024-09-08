package drivers

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/xo/dburl"

	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
)

type MySQL struct {
	Connection *sql.DB
	Provider   string
}

func (db *MySQL) TestConnection(urlstr string) (err error) {
	return db.Connect(urlstr)
}

func (db *MySQL) Connect(urlstr string) (err error) {
	db.SetProvider(MySQLDriver)

	db.Connection, err = dburl.Open(urlstr)
	if err != nil {
		return err
	}

	err = db.Connection.Ping()
	if err != nil {
		return err
	}

	return nil
}

func (db *MySQL) GetDatabases() ([]string, error) {
	var databases []string

	rows, err := db.Connection.Query("SHOW DATABASES")
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
		if database != "information_schema" && database != "mysql" && database != "performance_schema" && database != "sys" {
			databases = append(databases, database)
		}
	}

	return databases, nil
}

func (db *MySQL) GetTables(database string) (map[string][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	rows, err := db.Connection.Query(fmt.Sprintf("SHOW TABLES FROM `%s`", database))

	rowsErr := rows.Err()
	if rowsErr != nil {
		return nil, rowsErr
	}

	defer rows.Close()

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

func (db *MySQL) GetTableColumns(database, table string) (results [][]string, err error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	query := "DESCRIBE "
	query += db.formatTableName(database, table)

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
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}

	return
}

func (db *MySQL) GetConstraints(database, table string) (results [][]string, err error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	query := "SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE where TABLE_SCHEMA = ? AND TABLE_NAME = ?"

	rows, err := db.Connection.Query(query, database, table)
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
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}

	return
}

func (db *MySQL) GetForeignKeys(database, table string) (results [][]string, err error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	query := "SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_COLUMN_NAME, REFERENCED_TABLE_NAME FROM information_schema.KEY_COLUMN_USAGE where REFERENCED_TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME = ?"

	rows, err := db.Connection.Query(query, database, table)
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
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}

	return
}

func (db *MySQL) GetIndexes(database, table string) (results [][]string, err error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	query := "SHOW INDEX FROM "
	query += db.formatTableName(database, table)

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
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}

	return
}

func (db *MySQL) GetRecords(database, table, where, sort string, offset, limit int) (paginatedResults [][]string, totalRecords int, err error) {
	if table == "" {
		return nil, 0, errors.New("table name is required")
	}

	if database == "" {
		return nil, 0, errors.New("database name is required")
	}

	if limit == 0 {
		limit = DefaultRowLimit
	}

	query := "SELECT * FROM "
	query += db.formatTableName(database, table)

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
	countQuery += fmt.Sprintf("`%s`.", database)
	countQuery += fmt.Sprintf("`%s`", table)

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

func (db *MySQL) ExecuteQuery(query string) (results [][]string, err error) {
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
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)

	}

	return
}

func (db *MySQL) UpdateRecord(database, table, column, value, primaryKeyColumnName, primaryKeyValue string) error {
	query := "UPDATE "
	query += db.formatTableName(database, table)
	query += fmt.Sprintf(" SET %s = ? WHERE %s = ?", column, primaryKeyColumnName)

	_, err := db.Connection.Exec(query, value, primaryKeyValue)

	return err
}

func (db *MySQL) DeleteRecord(database, table, primaryKeyColumnName, primaryKeyValue string) error {
	query := "DELETE FROM "
	query += db.formatTableName(database, table)
	query += fmt.Sprintf(" WHERE %s = ?", primaryKeyColumnName)
	_, err := db.Connection.Exec(query, primaryKeyValue)

	return err
}

func (db *MySQL) ExecuteDMLStatement(query string) (result string, err error) {
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

func (db *MySQL) ExecutePendingChanges(changes []models.DbDmlChange) (err error) {
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
			queryStr += db.formatTableName(change.Database, change.Table)
			queryStr += fmt.Sprintf(" (%s) VALUES (%s)", strings.Join(columnNames, ", "), strings.Join(valuesPlaceholder, ", "))

			newQuery := models.Query{
				Query: queryStr,
				Args:  values,
			}

			query = append(query, newQuery)
		case models.DmlUpdateType:
			queryStr := "UPDATE "
			queryStr += db.formatTableName(change.Database, change.Table)

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
			queryStr += db.formatTableName(change.Database, change.Table)
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

func (db *MySQL) SetProvider(provider string) {
	db.Provider = provider
}

func (db *MySQL) GetProvider() string {
	return db.Provider
}

func (db *MySQL) formatTableName(database, table string) string {
	return fmt.Sprintf("`%s`.`%s`", database, table)
}
