package drivers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/xo/dburl"

	"github.com/jorgerojas26/lazysql/models"
)

// ClickHouse has no row-level UPDATE/DELETE in the traditional sense, so edits
// made in the table view are executed as mutations
// (ALTER TABLE ... UPDATE/DELETE). Mutations are asynchronous by default;
// mutations_sync makes the statement wait until the change is visible so the
// refreshed table shows the new data.
const clickHouseMutationsSync = 2

type ClickHouse struct {
	Connection *sql.DB
	Provider   string
}

func (db *ClickHouse) TestConnection(urlstr string) error {
	return db.Connect(urlstr)
}

func (db *ClickHouse) Connect(urlstr string) (err error) {
	db.SetProvider(DriverClickHouse)

	db.Connection, err = dburl.Open(urlstr)
	if err != nil {
		return err
	}

	return db.Connection.Ping()
}

func (db *ClickHouse) GetDatabases() ([]string, error) {
	rows, err := db.Connection.Query("SELECT name FROM system.databases WHERE name NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA') ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			return nil, err
		}
		databases = append(databases, database)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return databases, nil
}

func (db *ClickHouse) GetTables(database string) (map[string][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	rows, err := db.Connection.Query("SELECT name FROM system.tables WHERE database = ? AND NOT is_temporary ORDER BY name", database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make(map[string][]string)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables[database] = append(tables[database], table)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tables, nil
}

func (db *ClickHouse) GetTableColumns(database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	query := "SELECT name, type, default_kind, default_expression, is_in_partition_key, is_in_sorting_key, is_in_primary_key, comment FROM system.columns WHERE database = ? AND table = ? ORDER BY position"

	return db.queryMetadata(query, database, table)
}

// GetConstraints returns the table engine and keys. ClickHouse has no
// PRIMARY/UNIQUE/FOREIGN KEY constraints; the partition, sorting and primary
// keys are what define how a table is organized.
func (db *ClickHouse) GetConstraints(database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	query := "SELECT engine, partition_key, sorting_key, primary_key, sampling_key FROM system.tables WHERE database = ? AND name = ?"

	return db.queryMetadata(query, database, table)
}

// GetForeignKeys returns only the header row because ClickHouse does not
// support foreign keys.
func (db *ClickHouse) GetForeignKeys(database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	return [][]string{{"table_name", "column_name", "constraint_name", "referenced_column_name", "referenced_table_name"}}, nil
}

// GetIndexes returns the data skipping indexes of the table.
func (db *ClickHouse) GetIndexes(database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	query := "SELECT name, type_full AS type, expr, granularity FROM system.data_skipping_indices WHERE database = ? AND table = ?"

	return db.queryMetadata(query, database, table)
}

func (db *ClickHouse) GetRecords(database, table, where, sort string, offset, limit int) (paginatedResults [][]string, totalRecords int, queryString string, err error) {
	if table == "" {
		return nil, 0, "", errors.New("table name is required")
	}

	if database == "" {
		return nil, 0, "", errors.New("database name is required")
	}

	if limit == 0 {
		limit = DefaultRowLimit
	}

	formattedTableName := db.formatTableName(database, table)

	queryString = "SELECT * FROM " + formattedTableName

	if where != "" {
		queryString += fmt.Sprintf(" %s", where)
	}

	if sort != "" {
		queryString += fmt.Sprintf(" ORDER BY %s", sort)
	}

	queryString += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	rows, err := db.Connection.Query(queryString)
	if err != nil {
		return nil, 0, queryString, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, 0, queryString, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, 0, queryString, err
	}

	paginatedResults = append(paginatedResults, columns)

	for rows.Next() {
		values, nulls, err := scanClickHouseRow(rows, columnTypes)
		if err != nil {
			return nil, 0, queryString, err
		}

		row := make([]string, 0, len(values))
		for i, value := range values {
			switch {
			case nulls[i]:
				row = append(row, "NULL&")
			case value == "":
				row = append(row, "EMPTY&")
			default:
				row = append(row, value)
			}
		}

		paginatedResults = append(paginatedResults, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, queryString, err
	}
	// close to release the connection
	if err := rows.Close(); err != nil {
		return nil, 0, queryString, err
	}

	countQuery := "SELECT count() FROM " + formattedTableName
	if where != "" {
		countQuery += fmt.Sprintf(" %s", where)
	}
	if err := db.Connection.QueryRow(countQuery).Scan(&totalRecords); err != nil {
		return paginatedResults, 0, queryString, err
	}

	return paginatedResults, totalRecords, queryString, nil
}

func (db *ClickHouse) ExecuteQuery(query string) ([][]string, int, error) {
	rows, err := db.Connection.Query(query)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, 0, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, 0, err
	}

	records := make([][]string, 0)
	for rows.Next() {
		row, nulls, err := scanClickHouseRow(rows, columnTypes)
		if err != nil {
			return nil, 0, err
		}

		for i := range row {
			if nulls[i] {
				row[i] = "NULL"
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

func (db *ClickHouse) UpdateRecord(database, table, column, value, primaryKeyColumnName, primaryKeyValue string) error {
	query := "ALTER TABLE "
	query += db.formatTableName(database, table)
	query += fmt.Sprintf(" UPDATE %s = ? WHERE %s = ?", db.FormatReference(column), db.FormatReference(primaryKeyColumnName))

	_, err := db.Connection.ExecContext(clickHouseMutationContext(), query, value, primaryKeyValue)

	return err
}

func (db *ClickHouse) DeleteRecord(database, table, primaryKeyColumnName, primaryKeyValue string) error {
	query := "ALTER TABLE "
	query += db.formatTableName(database, table)
	query += fmt.Sprintf(" DELETE WHERE %s = ?", db.FormatReference(primaryKeyColumnName))

	_, err := db.Connection.ExecContext(clickHouseMutationContext(), query, primaryKeyValue)

	return err
}

func (db *ClickHouse) ExecuteDMLStatement(query string) (string, error) {
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

// ExecutePendingChanges runs the changes one by one. ClickHouse has no
// multi-statement transactions, so changes that ran before a failing one are
// not rolled back.
func (db *ClickHouse) ExecutePendingChanges(changes []models.DBDMLChange) error {
	var queries []models.Query

	for _, change := range changes {
		formattedTableName := db.formatTableName(change.Database, change.Table)

		switch change.Type {
		case models.DMLInsertType:
			queries = append(queries, buildInsertQuery(formattedTableName, change.Values, db))
		case models.DMLUpdateType:
			query := buildUpdateQuery(formattedTableName, change.Values, change.PrimaryKeyInfo, db)
			query.Query = toClickHouseMutation(query.Query, formattedTableName)
			queries = append(queries, query)
		case models.DMLDeleteType:
			query := buildDeleteQuery(formattedTableName, change.PrimaryKeyInfo, db)
			query.Query = toClickHouseMutation(query.Query, formattedTableName)
			queries = append(queries, query)
		}
	}

	ctx := clickHouseMutationContext()
	for _, query := range queries {
		if _, err := db.Connection.ExecContext(ctx, query.Query, query.Args...); err != nil {
			return err
		}
	}

	return nil
}

// GetPrimaryKeyColumnNames returns the columns of the table's primary key.
// Unlike other databases, ClickHouse primary keys are not unique, so edits
// and deletes made through the table view apply to every row sharing the same
// primary key values.
func (db *ClickHouse) GetPrimaryKeyColumnNames(database, table string) ([]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	rows, err := db.Connection.Query("SELECT name FROM system.columns WHERE database = ? AND table = ? AND is_in_primary_key ORDER BY position", database, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var primaryKeyColumnNames []string
	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			return nil, err
		}
		primaryKeyColumnNames = append(primaryKeyColumnNames, colName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return primaryKeyColumnNames, nil
}

func (db *ClickHouse) SetProvider(provider string) {
	db.Provider = provider
}

func (db *ClickHouse) GetProvider() string {
	return db.Provider
}

func (db *ClickHouse) formatTableName(database, table string) string {
	return fmt.Sprintf("`%s`.`%s`", database, table)
}

func (db *ClickHouse) FormatArg(arg any, colType models.CellValueType) any {
	if colType == models.Null {
		return nil
	}

	if colType == models.Empty {
		return ""
	}

	return fmt.Sprintf("%v", arg)
}

func (db *ClickHouse) FormatArgForQueryString(arg any) string {
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
		return clickHouseQuoteString(v)
	case []byte:
		return clickHouseQuoteString(string(v))
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (db *ClickHouse) FormatReference(reference string) string {
	return fmt.Sprintf("`%s`", reference)
}

func (db *ClickHouse) FormatPlaceholder(_ int) string {
	return "?"
}

func (db *ClickHouse) DMLChangeToQueryString(change models.DBDMLChange) (string, error) {
	var queryStr string

	formattedTableName := db.formatTableName(change.Database, change.Table)

	columnNames, values := getColNamesAndArgsAsString(change.Values)

	switch change.Type {
	case models.DMLInsertType:
		queryStr = buildInsertQueryString(formattedTableName, columnNames, values, db)
	case models.DMLUpdateType:
		queryStr = toClickHouseMutation(buildUpdateQueryString(formattedTableName, columnNames, values, change.PrimaryKeyInfo, db), formattedTableName)
	case models.DMLDeleteType:
		queryStr = toClickHouseMutation(buildDeleteQueryString(formattedTableName, change.PrimaryKeyInfo, db), formattedTableName)
	}

	return queryStr, nil
}

func (db *ClickHouse) GetFunctions(_ string) (map[string][]string, error) {
	return nil, errors.New("not implemented")
}

func (db *ClickHouse) GetProcedures(_ string) (map[string][]string, error) {
	return nil, errors.New("not implemented")
}

func (db *ClickHouse) GetViews(_ string) (map[string][]string, error) {
	return nil, errors.New("not implemented")
}

func (db *ClickHouse) SupportsProgramming() bool {
	return false
}

func (db *ClickHouse) UseSchemas() bool {
	return false
}

func (db *ClickHouse) GetFunctionDefinition(_ string, _ string) (string, error) {
	return "", errors.New("not implemented")
}

func (db *ClickHouse) GetProcedureDefinition(_ string, _ string) (string, error) {
	return "", errors.New("not implemented")
}

func (db *ClickHouse) GetViewDefinition(_ string, _ string) (string, error) {
	return "", errors.New("not implemented")
}

// queryMetadata runs a metadata query and returns the column names followed
// by the rows as strings.
func (db *ClickHouse) queryMetadata(query string, args ...any) ([][]string, error) {
	rows, err := db.Connection.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	results := [][]string{columns}

	for rows.Next() {
		row, _, err := scanClickHouseRow(rows, columnTypes)
		if err != nil {
			return nil, err
		}

		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// toClickHouseMutation rewrites the UPDATE/DELETE statements produced by the
// shared query builders into ClickHouse mutations.
func toClickHouseMutation(query, formattedTableName string) string {
	updatePrefix := "UPDATE " + formattedTableName + " SET "
	deletePrefix := "DELETE FROM " + formattedTableName + " "

	switch {
	case strings.HasPrefix(query, updatePrefix):
		return "ALTER TABLE " + formattedTableName + " UPDATE " + strings.TrimPrefix(query, updatePrefix)
	case strings.HasPrefix(query, deletePrefix):
		return "ALTER TABLE " + formattedTableName + " DELETE " + strings.TrimPrefix(query, deletePrefix)
	}

	return query
}

func clickHouseMutationContext() context.Context {
	return clickhouse.Context(context.Background(), clickhouse.WithSettings(clickhouse.Settings{
		"mutations_sync": clickHouseMutationsSync,
	}))
}

func clickHouseQuoteString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "'", `\'`)
	return "'" + s + "'"
}

// scanClickHouseRow scans a row and converts every value to its display
// string. ClickHouse returns types such as arrays, maps and nullable pointers
// that cannot be scanned into sql.NullString or sql.RawBytes, so values are
// scanned as native Go values first. The second return value reports which
// values are NULL.
func scanClickHouseRow(rows *sql.Rows, columnTypes []*sql.ColumnType) ([]string, []bool, error) {
	values := make([]any, len(columnTypes))
	pointers := make([]any, len(columnTypes))
	for i := range values {
		pointers[i] = &values[i]
	}

	if err := rows.Scan(pointers...); err != nil {
		return nil, nil, err
	}

	row := make([]string, len(values))
	nulls := make([]bool, len(values))
	for i, value := range values {
		row[i], nulls[i] = clickHouseValueToString(value, columnTypes[i].DatabaseTypeName())
	}

	return row, nulls, nil
}

// clickHouseValueToString converts a value returned by the ClickHouse driver
// to the string ClickHouse itself would print, so it can be used again in
// queries. The second return value reports whether the value is NULL.
func clickHouseValueToString(value any, databaseType string) (string, bool) {
	if value == nil {
		return "", true
	}

	v := reflect.ValueOf(value)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "", true
		}
		v = v.Elem()
	}
	value = v.Interface()

	switch val := value.(type) {
	case string:
		return val, false
	case time.Time:
		if isClickHouseDateType(databaseType) {
			return val.Format(time.DateOnly), false
		}
		return val.Format("2006-01-02 15:04:05.999999999"), false
	case fmt.Stringer:
		return val.String(), false
	}

	// Types such as big.Int (Int128/Int256) implement fmt.Stringer on the
	// pointer receiver only.
	ptr := reflect.New(v.Type())
	ptr.Elem().Set(v)
	if stringer, ok := ptr.Interface().(fmt.Stringer); ok {
		return stringer.String(), false
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		elements := make([]string, v.Len())
		for i := range elements {
			elements[i] = clickHouseLiteral(v.Index(i).Interface())
		}
		if strings.HasPrefix(databaseType, "Tuple(") {
			return "(" + strings.Join(elements, ",") + ")", false
		}
		return "[" + strings.Join(elements, ",") + "]", false
	case reflect.Map:
		entries := make([]string, 0, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			entries = append(entries, clickHouseLiteral(iter.Key().Interface())+":"+clickHouseLiteral(iter.Value().Interface()))
		}
		sort.Strings(entries)
		return "{" + strings.Join(entries, ",") + "}", false
	}

	return fmt.Sprintf("%v", value), false
}

// clickHouseLiteral formats a value nested in an Array or Map the way
// ClickHouse prints it, e.g. ['a','b'] or {'k':1}.
func clickHouseLiteral(value any) string {
	str, isNull := clickHouseValueToString(value, "")
	if isNull {
		return "NULL"
	}

	v := reflect.ValueOf(value)
	for v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.String:
		return clickHouseQuoteString(str)
	case reflect.Struct:
		if _, ok := v.Interface().(time.Time); ok {
			return clickHouseQuoteString(str)
		}
	}

	return str
}

// isClickHouseDateType reports whether the type is Date or Date32, possibly
// wrapped in Nullable or LowCardinality.
func isClickHouseDateType(databaseType string) bool {
	for _, wrapper := range []string{"Nullable(", "LowCardinality("} {
		if strings.HasPrefix(databaseType, wrapper) {
			databaseType = strings.TrimSuffix(strings.TrimPrefix(databaseType, wrapper), ")")
		}
	}

	return databaseType == "Date" || databaseType == "Date32"
}
