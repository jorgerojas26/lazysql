package drivers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xo/dburl"

	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
)

type MySQL struct {
	Connection *sql.DB
	Provider   string
	PoolConfig models.ConnectionPoolConfig
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

	if err = applyConnectionPoolConfig(db.Connection, db.PoolConfig); err != nil {
		_ = db.Connection.Close()
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

	query := "SHOW FULL COLUMNS FROM "
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

	query := "SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?"

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
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// MySQL 5.6/5.7 expose the InnoDB dictionary with the INNODB_SYS_* names;
// MySQL 8 uses the corresponding INNODB_* tables. Both avoid the
// instance-wide KEY_COLUMN_USAGE lookup for the common InnoDB case.
const (
	mysqlForeignKeysLegacyFastPathQuery = `
SELECT
    SUBSTRING_INDEX(f.FOR_NAME, '/', -1) AS TABLE_NAME,
    fc.FOR_COL_NAME AS COLUMN_NAME,
    SUBSTRING_INDEX(f.ID, '/', -1) AS CONSTRAINT_NAME,
    fc.REF_COL_NAME AS REFERENCED_COLUMN_NAME,
    SUBSTRING_INDEX(f.REF_NAME, '/', -1) AS REFERENCED_TABLE_NAME
FROM information_schema.INNODB_SYS_FOREIGN AS f
INNER JOIN information_schema.INNODB_SYS_FOREIGN_COLS AS fc
    ON f.ID = fc.ID
WHERE f.REF_NAME = CONCAT(?, '/', ?)
ORDER BY f.ID, fc.POS`
	mysqlForeignKeysFastPathQuery = `
SELECT
    SUBSTRING_INDEX(f.FOR_NAME, '/', -1) AS TABLE_NAME,
    fc.FOR_COL_NAME AS COLUMN_NAME,
    SUBSTRING_INDEX(f.ID, '/', -1) AS CONSTRAINT_NAME,
    fc.REF_COL_NAME AS REFERENCED_COLUMN_NAME,
    SUBSTRING_INDEX(f.REF_NAME, '/', -1) AS REFERENCED_TABLE_NAME
FROM information_schema.INNODB_FOREIGN AS f
INNER JOIN information_schema.INNODB_FOREIGN_COLS AS fc
    ON f.ID = fc.ID
WHERE f.REF_NAME = CONCAT(?, '/', ?)
ORDER BY f.ID, fc.POS`
	mysqlForeignKeysFallbackQuery = "SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_COLUMN_NAME, REFERENCED_TABLE_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE REFERENCED_TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME = ?"
)

var mysqlForeignKeysFastPathQueries = []struct {
	name  string
	query string
}{
	{name: "innodb_sys_foreign", query: mysqlForeignKeysLegacyFastPathQuery},
	{name: "innodb_foreign", query: mysqlForeignKeysFastPathQuery},
}

func (db *MySQL) GetForeignKeys(ctx context.Context, database, table string) (results [][]string, err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	started := time.Now()
	fallback := "none"
	fastPath := "none"
	var fastPathErrors []string
	defer func() {
		data := map[string]any{
			"operation":     "get_foreign_keys",
			"driver":        DriverMySQL,
			"database":      database,
			"table":         table,
			"duration":      time.Since(started).String(),
			"fast_path":     fastPath,
			"fallback":      fallback,
			"fallback_used": fallback != "none",
		}
		if len(fastPathErrors) > 0 {
			data["fast_path_error"] = strings.Join(fastPathErrors, "; ")
		}
		if err != nil {
			data["error"] = err.Error()
		}
		logger.Debug("Loaded MySQL foreign-key metadata", data)
	}()

	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	for _, fastPathQuery := range mysqlForeignKeysFastPathQueries {
		results, err = db.queryForeignKeys(ctx, fastPathQuery.query, database, table)
		if err == nil {
			fastPath = fastPathQuery.name
			return results, nil
		}
		fastPathErrors = append(fastPathErrors, fastPathQuery.name+": "+err.Error())
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	fallback = "key_column_usage"
	return db.queryForeignKeys(ctx, mysqlForeignKeysFallbackQuery, database, table)
}

func (db *MySQL) queryForeignKeys(ctx context.Context, query string, args ...any) (results [][]string, err error) {
	rows, err := db.Connection.QueryContext(ctx, query, args...)
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

		if err := rows.Scan(rowValues...); err != nil {
			return nil, err
		}

		row := make([]string, 0, len(rowValues))
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
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (db *MySQL) GetRecords(ctx context.Context, database, table, where, sort string, offset, limit int) (PageResult, error) {
	if table == "" {
		return PageResult{}, errors.New("table name is required")
	}

	if database == "" {
		return PageResult{}, errors.New("database name is required")
	}

	pageSize, fetchLimit := pageSizeAndFetchLimit(limit)
	queryString := "SELECT * FROM "
	queryString += db.formatTableName(database, table)

	if where != "" {
		queryString += fmt.Sprintf(" %s", where)
	}

	if sort != "" {
		queryString += fmt.Sprintf(" ORDER BY %s", sort)
	}

	queryString += " LIMIT ?, ?"

	paginatedRows, err := db.Connection.QueryContext(ctx, queryString, offset, fetchLimit)
	if err != nil {
		return PageResult{Query: queryString}, err
	}
	defer paginatedRows.Close()

	columns, err := paginatedRows.Columns()
	if err != nil {
		return PageResult{Query: queryString}, err
	}

	paginatedResults := [][]string{columns}

	for paginatedRows.Next() {
		nullStringSlice := make([]sql.NullString, len(columns))

		rowValues := make([]any, len(columns))
		for i := range nullStringSlice {
			rowValues[i] = &nullStringSlice[i]
		}

		err = paginatedRows.Scan(rowValues...)
		if err != nil {
			return PageResult{Query: queryString}, err
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
		return PageResult{Query: queryString}, err
	}
	// close to release the connection
	if err := paginatedRows.Close(); err != nil {
		return PageResult{Query: queryString}, err
	}

	// Replace the limit and offset with actual values in the query string.
	queryString = strings.Replace(queryString, "?", strconv.Itoa(offset), 1)
	queryString = strings.Replace(queryString, "?", strconv.Itoa(fetchLimit), 1)

	return newPageResult(paginatedResults, queryString, pageSize), nil
}

func (db *MySQL) GetEstimatedRowCount(ctx context.Context, database, table string) (*int64, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if table == "" {
		return nil, errors.New("table name is required")
	}

	var estimate sql.NullInt64
	err := db.Connection.QueryRowContext(ctx, `
		SELECT TABLE_ROWS
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
	`, database, table).Scan(&estimate)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !estimate.Valid || estimate.Int64 < 0 {
		return nil, nil
	}

	return &estimate.Int64, nil
}

func (db *MySQL) GetExactRowCount(ctx context.Context, database, table, where string) (int64, error) {
	if database == "" {
		return 0, errors.New("database name is required")
	}
	if table == "" {
		return 0, errors.New("table name is required")
	}

	query := "SELECT COUNT(*) FROM " + db.formatTableName(database, table)
	if where != "" {
		query += " " + where
	}

	var count int64
	if err := db.Connection.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// StreamQuery incrementally emits interactive SQL results and honors context
// cancellation through database/sql.
func (db *MySQL) StreamQuery(ctx context.Context, query string, maxRows int, onBatch func(QueryBatch) error) (QueryStreamResult, error) {
	return streamQuery(ctx, db.Connection, query, maxRows, onBatch)
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

	rows, err := db.Connection.Query("SELECT column_name FROM information_schema.key_column_usage WHERE table_schema = ? AND table_name = ? AND constraint_name = ?", database, table, "PRIMARY")
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

func (db *MySQL) FormatArg(arg any, colType models.CellValueType) any {
	if colType == models.Null {
		return sql.NullString{
			String: "",
			Valid:  false,
		}
	}

	if colType == models.Default {
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
			s := fmt.Sprintf("%f", v)
			s = strings.TrimRight(s, "0")
			if strings.HasSuffix(s, ".") {
				s += "0"
			}
			return s
		case string:
			return v
		case []byte:
			return "'" + string(v) + "'"
		default:
			return fmt.Sprintf("%v", v)
		}
	}

	return fmt.Sprintf("%v", arg)
}

func (db *MySQL) FormatArgForQueryString(arg any) string {
	if arg == "NULL" || arg == "DEFAULT" {
		return fmt.Sprintf("%v", arg)
	}

	switch v := arg.(type) {
	case int, int64:
		return fmt.Sprintf("%d", v)
	case float64, float32:
		s := fmt.Sprintf("%f", v)
		trimmed := strings.TrimRight(s, "0")
		if strings.HasSuffix(trimmed, ".") {
			trimmed += "0"
		}
		return trimmed
	case string:
		escaped := strings.ReplaceAll(v, "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	case []byte:
		escaped := strings.ReplaceAll(string(v), "'", "''")
		return fmt.Sprintf("'%s'", escaped)
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

func (db *MySQL) GetFunctions(_ string) (map[string][]string, error) {
	return nil, errors.New("not implemented")
}

func (db *MySQL) GetProcedures(_ string) (map[string][]string, error) {
	return nil, errors.New("not implemented")
}

func (db *MySQL) GetViews(_ string) (map[string][]string, error) {
	return nil, errors.New("not implemented")
}

func (db *MySQL) SupportsProgramming() bool {
	return false
}

func (db *MySQL) UseSchemas() bool {
	return false
}

func (db *MySQL) GetFunctionDefinition(_ string, _ string) (string, error) {
	return "", errors.New("not implemented")
}

func (db *MySQL) GetProcedureDefinition(_ string, _ string) (string, error) {
	return "", errors.New("not implemented")
}

func (db *MySQL) GetViewDefinition(_ string, _ string) (string, error) {
	return "", errors.New("not implemented")
}
