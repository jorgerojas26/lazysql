package drivers

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
	db.SetProvider(DriverSqlite)

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
	if err := rows.Err(); err != nil {
		return nil, err
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

func (db *SQLite) GetTableColumns(_, table string) (results [][]string, err error) {
	if table == "" {
		return nil, errors.New("table name is required")
	}

	rows, err := db.Connection.Query(fmt.Sprintf("PRAGMA table_info(%s)", db.formatTableName(table)))
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
		rowValues := make([]any, len(columns))
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
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
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results = append(results, columns)

	for rows.Next() {
		rowValues := make([]any, len(columns))
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (db *SQLite) GetForeignKeys(_, table string) (results [][]string, err error) {
	if table == "" {
		return nil, errors.New("table name is required")
	}

	formattedTableName := db.formatTableName(table)

	rows, err := db.Connection.Query("PRAGMA foreign_key_list(" + formattedTableName + ")")
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
		rowValues := make([]any, len(columns))
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (db *SQLite) GetIndexes(_, table string) (results [][]string, err error) {
	if table == "" {
		return nil, errors.New("table name is required")
	}

	formattedTableName := db.formatTableName(table)
	rows, err := db.Connection.Query("PRAGMA index_list(" + formattedTableName + ")")
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
		rowValues := make([]any, len(columns))
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
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (db *SQLite) GetRecords(_, table, where, sort string, offset, limit int) (paginatedResults [][]string, totalRecords int, queryString string, err error) {
	if table == "" {
		return nil, 0, "", errors.New("table name is required")
	}

	if limit == 0 {
		limit = DefaultRowLimit
	}

	queryString = "SELECT * FROM "
	queryString += db.formatTableName(table)

	if where != "" {
		queryString += fmt.Sprintf(" %s", where)
	}

	if sort != "" {
		queryString += fmt.Sprintf(" ORDER BY %s", sort)
	}

	queryString += " LIMIT ?, ?"

	paginatedRows, err := db.Connection.Query(queryString, offset, limit)
	if err != nil {
		return nil, 0, queryString, err
	}
	defer paginatedRows.Close()

	columns, err := paginatedRows.Columns()
	if err != nil {
		return nil, 0, queryString, err
	}

	paginatedResults = append(paginatedResults, columns)

	for paginatedRows.Next() {
		nullStringSlice := make([]sql.NullString, len(columns))

		rowValues := make([]any, len(columns))

		for i := range nullStringSlice {
			rowValues[i] = &nullStringSlice[i]
		}

		err = paginatedRows.Scan(rowValues...)
		if err != nil {
			return nil, 0, queryString, err
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
		return nil, 0, queryString, err
	}
	// close to release the connection
	if err := paginatedRows.Close(); err != nil {
		return nil, 0, queryString, err
	}

	countQuery := "SELECT COUNT(*) FROM "
	countQuery += db.formatTableName(table)
	if where != "" { // Add WHERE clause to count query as well if it exists
		countQuery += fmt.Sprintf(" %s", where)
	}
	countRow := db.Connection.QueryRow(countQuery)
	if err := countRow.Scan(&totalRecords); err != nil {
		return paginatedResults, 0, queryString, err
	}

	// Replace the limit and offset with actual values in the query string
	queryString = strings.Replace(queryString, "?, ?", fmt.Sprintf("%d, %d", offset, limit), 1)

	return paginatedResults, totalRecords, queryString, nil
}

func (db *SQLite) ExecuteQuery(query string) ([][]string, int, error) {
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
		rowValues := make([]any, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		err = rows.Scan(rowValues...)
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

		records = append(records, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Prepend the columns to the records.
	results := append([][]string{columns}, records...)

	return results, len(records), nil
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

	_, err := db.Connection.Exec(query, primaryKeyValue)

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

func (db *SQLite) ExecutePendingChanges(changes []models.DBDMLChange) error {
	var queries []models.Query

	for _, change := range changes {

		formattedTableName := db.formatTableName(change.Table)

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

func (db *SQLite) GetPrimaryKeyColumnNames(database, table string) (primaryKeyColumnName []string, err error) {
	columns, err := db.GetTableColumns(database, table)
	if err != nil {
		return nil, err
	}

	indexOfPkColumn := -1

	for i, col := range columns[0] {
		if col == "pk" {
			indexOfPkColumn = i
		}
	}

	for i, col := range columns {
		if i > 0 && col[indexOfPkColumn] != "0" {
			primaryKeyColumnName = append(primaryKeyColumnName, col[0])
		}
	}

	return primaryKeyColumnName, nil
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

func (db *SQLite) FormatArg(arg any, colType models.CellValueType) any {
	if colType == models.Null {
		return sql.NullString{
			String: "",
			Valid:  false,
		}
	}

	if colType == models.Default {
		if arg == nil {
			return "NULL"
		}
		return fmt.Sprintf("%v", arg)
	}

	if colType == models.Empty {
		return ""
	}

	if colType == models.String {
		switch v := arg.(type) {
		case int, int64:
			return fmt.Sprintf("%d", v)
		case float64, float32:
			return fmt.Sprintf("%f", v)
		case string:
			return v
		case []byte:
			return "'" + string(v) + "'"
		case bool:
			if v {
				return "1"
			}

			return "0"
		case nil:
			return sql.NullString{
				String: "",
				Valid:  false,
			}
		default:
			return fmt.Sprintf("%v", v)
		}
	}

	return fmt.Sprintf("%v", arg)
}

func (db *SQLite) FormatArgForQueryString(arg any) string {
	switch v := arg.(type) {
	case string:
		if v == "NULL" || v == "DEFAULT" {
			return v
		}
		escaped := strings.ReplaceAll(v, "'", "''")
		return "'" + escaped + "'"
	case float32, float64:
		return strings.TrimRight(fmt.Sprintf("%f", v), "0")
	case sql.NullString:
		if !v.Valid {
			return "NULL"
		}
		escaped := strings.ReplaceAll(v.String, "'", "''")
		return "'" + escaped + "'"
	case nil:
		return "NULL"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (db *SQLite) FormatReference(reference string) string {
	return fmt.Sprintf("`%s`", reference)
}

func (db *SQLite) FormatPlaceholder(_ int) string {
	return "?"
}

func (db *SQLite) DMLChangeToQueryString(change models.DBDMLChange) (string, error) {
	var queryStr string

	formattedTableName := db.formatTableName(change.Table)

	columnNames, values := getColNamesAndArgsAsString(change.Values)

	switch change.Type {
	case models.DMLInsertType:
		queryStr = buildInsertQueryString(formattedTableName, columnNames, values, db)
	case models.DMLUpdateType:
		queryStr = buildUpdateQueryString(formattedTableName, columnNames, values, change.PrimaryKeyInfo, db)
	case models.DMLDeleteType:
		queryStr = buildDeleteQueryString(formattedTableName, change.PrimaryKeyInfo, db)

	}

	return queryStr, nil
}

func (db *SQLite) GetFunctions(database string) (map[string][]string, error) {
	return nil, nil
}

func (db *SQLite) GetProcedures(database string) (map[string][]string, error) {
	return nil, nil
}

func (db *SQLite) GetViews(database string) (map[string][]string, error) {
	return nil, nil
}

func (db *SQLite) SupportsProgramming() bool {
	return false
}

func (db *SQLite) UseSchemas() bool {
	return false
}

func (db *SQLite) GetFunctionDefinition(database string, name string) (string, error) {
	return "", nil
}

func (db *SQLite) GetProcedureDefinition(database string, name string) (string, error) {
	return "", nil
}

func (db *SQLite) GetViewDefinition(database string, name string) (string, error) {
	return "", nil
}
