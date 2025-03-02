package drivers

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/xo/dburl"

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
	db.SetProvider(DriverMySQL)

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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return databases, nil
}

func (db *MySQL) GetTables(database string) (map[string][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	rows, err := db.Connection.Query(fmt.Sprintf("SHOW TABLES FROM `%s`", database))
	if err != nil {
		return nil, err
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
	if err := rows.Err(); err != nil {
		return nil, err
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
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
	defer paginatedRows.Close()

	columns, err := paginatedRows.Columns()
	if err != nil {
		return nil, 0, err
	}

	paginatedResults = append(paginatedResults, columns)

	for paginatedRows.Next() {
		nullStringSlice := make([]sql.NullString, len(columns))

		rowValues := make([]interface{}, len(columns))
		for i := range nullStringSlice {
			rowValues[i] = &nullStringSlice[i]
		}

		err = paginatedRows.Scan(rowValues...)
		if err != nil {
			return nil, 0, err
		}

		var row []string
		for _, col := range nullStringSlice {
			if col.Valid {
				if col.String == "" {
					row = append(row, "EMPTY&")
				} else {
					row = append(row, col.String)
				}
			} else {
				row = append(row, "NULL&")
			}
		}

		paginatedResults = append(paginatedResults, row)
	}
	if err := paginatedRows.Err(); err != nil {
		return nil, 0, err
	}
	// close to release the connection
	if err := paginatedRows.Close(); err != nil {
		return nil, 0, err
	}

	countQuery := "SELECT COUNT(*) FROM "
	countQuery += fmt.Sprintf("`%s`.", database)
	countQuery += fmt.Sprintf("`%s`", table)
	row := db.Connection.QueryRow(countQuery)
	if err := row.Scan(&totalRecords); err != nil {
		return nil, 0, err
	}

	return paginatedResults, totalRecords, nil
}

func (db *MySQL) ExecuteQuery(query string) ([][]string, int, error) {
	rows, err := db.Connection.Query(query)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, 0, err
	}

	records := make([][]string, 0)
	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		err = rows.Scan(rowValues...)
		if err != nil {
			return nil, 0, err
		}

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		records = append(records, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Prepend the columns to the records.
	results := append([][]string{columns}, records...)

	return results, len(records), nil
}

func (db *MySQL) UpdateRecord(database, table, column, value, primaryKeyColumnName, primaryKeyValue string) error {
	query := "UPDATE "
	query += db.formatTableName(database, table)
	query += fmt.Sprintf(" SET %s = ? WHERE %s = ?", column, primaryKeyColumnName)

	_, err := db.Connection.Exec(query, fmt.Sprintf("'%s'", value), primaryKeyValue)

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

func (db *MySQL) ExecutePendingChanges(changes []models.DBDMLChange) error {
	var queries []models.Query

	for _, change := range changes {

		formattedTableName := db.formatTableName(change.Database, change.Table)

		switch change.Type {

		case models.DMLInsertType:
			queries = append(queries, buildInsertQuery(formattedTableName, change.Values, db))
		case models.DMLUpdateType:
			queries = append(queries, buildUpdateQuery(formattedTableName, change.Values, change.PrimaryKeyInfo, db))
		case models.DMLDeleteType:
			queries = append(queries, buildDeleteQuery(formattedTableName, change.PrimaryKeyInfo, db))
		}
	}

	return queriesInTransaction(db.Connection, queries)
}

func (db *MySQL) GetPrimaryKeyColumnNames(database, table string) (primaryKeyColumnName []string, err error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	rows, err := db.Connection.Query(`
	SELECT column_name
	FROM information_schema.key_column_usage
	WHERE table_schema = ? AND table_name = ? AND constraint_name = ?
	`, database, table, "PRIMARY")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var colName string
		err = rows.Scan(&colName)
		if err != nil {
			return nil, err
		}

		if rows.Err() != nil {
			return nil, rows.Err()
		}

		primaryKeyColumnName = append(primaryKeyColumnName, colName)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return primaryKeyColumnName, nil
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

func (db *MySQL) FormatArg(arg interface{}) string {
	switch v := arg.(type) {

	case int, int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%f", v)
	case string:
		return fmt.Sprintf("'%s'", v)
	case []byte:
		return fmt.Sprintf("'%s'", string(v))
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (db *MySQL) FormatReference(reference string) string {
	return fmt.Sprintf("`%s`", reference)
}

func (db *MySQL) FormatPlaceholder(_ int) string {
	return "?"
}

func (db *MySQL) DMLChangeToQueryString(change models.DBDMLChange) (string, error) {
	var queryStr string

	formattedTableName := db.formatTableName(change.Database, change.Table)

	columnNames, values := getColNamesAndArgsAsString(change.Values, db)

	switch change.Type {
	case models.DMLInsertType:
		queryStr = buildInsertQueryString(formattedTableName, columnNames, values)
	case models.DMLUpdateType:
		queryStr = buildUpdateQueryString(formattedTableName, columnNames, values, change.PrimaryKeyInfo, db)
	case models.DMLDeleteType:
		queryStr = buildDeleteQueryString(formattedTableName, change.PrimaryKeyInfo, db)

	}

	return queryStr, nil
}
