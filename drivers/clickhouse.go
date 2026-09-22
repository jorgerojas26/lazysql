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
	PoolConfig models.ConnectionPoolConfig
}

func (db *ClickHouse) TestConnection(ctx context.Context, urlstr string) error {
	return db.Connect(ctx, urlstr)
}

func (db *ClickHouse) Connect(ctx context.Context, urlstr string) (err error) {
	db.SetProvider(DriverClickHouse)

	db.Connection, err = dburl.Open(urlstr)
	if err != nil {
		return err
	}

	if err := applyConnectionPoolConfig(db.Connection, db.PoolConfig); err != nil {
		_ = db.Connection.Close()
		return err
	}
	if err := db.Connection.PingContext(contextOrBackground(ctx)); err != nil {
		_ = db.Connection.Close()
		return err
	}
	return nil
}

func (db *ClickHouse) GetDatabases(ctx context.Context) ([]string, error) {
	rows, err := db.Connection.QueryContext(contextOrBackground(ctx), "SELECT name FROM system.databases WHERE name NOT IN ('system', 'information_schema', 'INFORMATION_SCHEMA') ORDER BY name")
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

func (db *ClickHouse) GetTables(ctx context.Context, database string) (map[string][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	rows, err := db.Connection.QueryContext(contextOrBackground(ctx), "SELECT name FROM system.tables WHERE database = ? AND NOT is_temporary ORDER BY name", database)
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

func (db *ClickHouse) GetTableColumns(ctx context.Context, database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	query := "SELECT name, type, default_kind, default_expression, is_in_partition_key, is_in_sorting_key, is_in_primary_key, comment FROM system.columns WHERE database = ? AND table = ? ORDER BY position"

	return db.queryMetadata(ctx, query, database, table)
}

// GetConstraints returns the table engine and keys. ClickHouse has no
// PRIMARY/UNIQUE/FOREIGN KEY constraints; the partition, sorting and primary
// keys are what define how a table is organized.
func (db *ClickHouse) GetConstraints(ctx context.Context, database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	query := "SELECT engine, partition_key, sorting_key, primary_key, sampling_key FROM system.tables WHERE database = ? AND name = ?"

	return db.queryMetadata(ctx, query, database, table)
}

// GetForeignKeys returns only the header row because ClickHouse does not
// support foreign keys.
func (db *ClickHouse) GetForeignKeys(_ context.Context, database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	return [][]string{{"table_name", "column_name", "constraint_name", "referenced_column_name", "referenced_table_name"}}, nil
}

// GetReferencingTables returns only the shared header row because ClickHouse
// does not support foreign keys.
func (db *ClickHouse) GetReferencingTables(_ context.Context, database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	return [][]string{append([]string(nil), ReferencingTablesHeader...)}, nil
}

// GetIndexes returns the data skipping indexes of the table.
func (db *ClickHouse) GetIndexes(ctx context.Context, database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	query := "SELECT name, type_full AS type, expr, granularity FROM system.data_skipping_indices WHERE database = ? AND table = ?"

	return db.queryMetadata(ctx, query, database, table)
}

func (db *ClickHouse) GetRecords(ctx context.Context, database, table, where, sort string, offset, limit int) (PageResult, error) {
	if table == "" {
		return PageResult{}, errors.New("table name is required")
	}

	if database == "" {
		return PageResult{}, errors.New("database name is required")
	}

	pageSize, fetchLimit := pageSizeAndFetchLimit(limit)

	formattedTableName := db.formatTableName(database, table)

	queryString := "SELECT * FROM " + formattedTableName

	if where != "" {
		queryString += fmt.Sprintf(" %s", where)
	}

	if sort != "" {
		queryString += fmt.Sprintf(" ORDER BY %s", sort)
	}

	queryString += fmt.Sprintf(" LIMIT %d OFFSET %d", fetchLimit, offset)

	rows, err := db.Connection.QueryContext(contextOrBackground(ctx), queryString)
	if err != nil {
		return PageResult{Query: queryString}, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return PageResult{Query: queryString}, err
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return PageResult{Query: queryString}, err
	}

	paginatedResults := [][]string{columns}

	for rows.Next() {
		values, nulls, err := scanClickHouseRow(rows, columnTypes)
		if err != nil {
			return PageResult{Query: queryString}, err
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
		return PageResult{Query: queryString}, err
	}
	// close to release the connection
	if err := rows.Close(); err != nil {
		return PageResult{Query: queryString}, err
	}

	return newPageResult(paginatedResults, queryString, pageSize), nil
}

func (db *ClickHouse) ExecuteQuery(ctx context.Context, _, query string) ([][]string, int, error) {
	rows, err := db.Connection.QueryContext(contextOrBackground(ctx), query)
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

func (db *ClickHouse) UpdateRecord(ctx context.Context, database, table, column, value, primaryKeyColumnName, primaryKeyValue string) error {
	change := models.DBDMLChange{
		Type:     models.DMLUpdateType,
		Database: database,
		Table:    table,
		Values: []models.CellValue{{
			Column: column,
			Value:  value,
			Type:   models.String,
		}},
		PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: primaryKeyColumnName, Value: primaryKeyValue}},
	}

	query, err := db.buildChangeQuery(ctx, change, false)
	if err != nil {
		return err
	}

	_, err = db.Connection.ExecContext(clickHouseMutationContext(ctx), query.Query, query.Args...)
	return err
}

func (db *ClickHouse) DeleteRecord(ctx context.Context, database, table, primaryKeyColumnName, primaryKeyValue string) error {
	change := models.DBDMLChange{
		Type:           models.DMLDeleteType,
		Database:       database,
		Table:          table,
		PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: primaryKeyColumnName, Value: primaryKeyValue}},
	}

	query, err := db.buildChangeQuery(ctx, change, false)
	if err != nil {
		return err
	}

	_, err = db.Connection.ExecContext(clickHouseMutationContext(ctx), query.Query, query.Args...)
	return err
}

func (db *ClickHouse) ExecuteDMLStatement(ctx context.Context, _, query string) (string, error) {
	res, err := db.Connection.ExecContext(contextOrBackground(ctx), query)
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
// multi-statement transactions, so a partial execution error reports how many
// changes the caller must remove before allowing a retry.
func (db *ClickHouse) ExecutePendingChanges(ctx context.Context, changes []models.DBDMLChange) error {
	queries := make([]models.Query, 0, len(changes))
	for _, change := range changes {
		query, err := db.buildChangeQuery(ctx, change, false)
		if err != nil {
			return err
		}
		queries = append(queries, query)
	}

	ctx = clickHouseMutationContext(ctx)
	for i, query := range queries {
		if _, err := db.Connection.ExecContext(ctx, query.Query, query.Args...); err != nil {
			if i > 0 {
				return &PartialExecutionError{Applied: i, Err: err}
			}
			return err
		}
	}

	return nil
}

// GetPrimaryKeyColumnNames returns the columns of the table's primary key.
// Unlike other databases, ClickHouse primary keys are not unique, so edits
// and deletes made through the table view apply to every row sharing the same
// primary key values.
func (db *ClickHouse) GetPrimaryKeyColumnNames(ctx context.Context, database, table string) ([]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	rows, err := db.Connection.QueryContext(contextOrBackground(ctx), "SELECT name FROM system.columns WHERE database = ? AND table = ? AND is_in_primary_key ORDER BY position", database, table)
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
	return db.FormatReference(database) + "." + db.FormatReference(table)
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
	return "`" + strings.ReplaceAll(reference, "`", "``") + "`"
}

func (db *ClickHouse) FormatPlaceholder(_ int) string {
	return "?"
}

func (db *ClickHouse) DMLChangeToQueryString(change models.DBDMLChange) (string, error) {
	ctx := context.Background()
	query, err := db.buildChangeQuery(ctx, change, true)
	if err != nil {
		return "", err
	}
	return query.Query, nil
}

func (db *ClickHouse) buildChangeQuery(ctx context.Context, change models.DBDMLChange, preview bool) (models.Query, error) {
	columnTypes, err := db.getColumnTypes(ctx, change.Database, change.Table)
	if err != nil {
		return models.Query{}, err
	}

	formattedTableName := db.formatTableName(change.Database, change.Table)
	args := make([]any, 0, len(change.Values)+len(change.PrimaryKeyInfo))
	// clickhouse-go mistakes map literals ({'key':value}) for native named
	// query parameters when positional arguments are also present. Inline all
	// safely formatted values whenever a composite literal is used.
	inlineValues := preview || changeUsesCompositeType(change, columnTypes)
	valueSQL := func(value any, valueType models.CellValueType, column string) (string, error) {
		return db.clickHouseSQLValue(value, valueType, columnTypes[column], inlineValues, &args)
	}

	switch change.Type {
	case models.DMLInsertType:
		columns := make([]string, 0, len(change.Values))
		values := make([]string, 0, len(change.Values))
		for _, value := range change.Values {
			if value.Type == models.Default {
				continue
			}
			expression, err := valueSQL(value.Value, value.Type, value.Column)
			if err != nil {
				return models.Query{}, err
			}
			columns = append(columns, db.FormatReference(value.Column))
			values = append(values, expression)
		}
		if len(columns) == 0 {
			return models.Query{}, errors.New("insert requires at least one non-default value")
		}
		return models.Query{
			Query: "INSERT INTO " + formattedTableName + " (" + strings.Join(columns, ", ") + ") VALUES (" + strings.Join(values, ", ") + ")",
			Args:  args,
		}, nil

	case models.DMLUpdateType:
		if len(change.Values) == 0 {
			return models.Query{}, errors.New("update requires at least one value")
		}
		if len(change.PrimaryKeyInfo) == 0 {
			return models.Query{}, errors.New("update requires primary key values")
		}

		assignments := make([]string, 0, len(change.Values))
		for _, value := range change.Values {
			expression, err := valueSQL(value.Value, value.Type, value.Column)
			if err != nil {
				return models.Query{}, err
			}
			assignments = append(assignments, db.FormatReference(value.Column)+" = "+expression)
		}
		where, err := db.clickHouseWhere(change.PrimaryKeyInfo, columnTypes, inlineValues, &args)
		if err != nil {
			return models.Query{}, err
		}
		return models.Query{
			Query: "ALTER TABLE " + formattedTableName + " UPDATE " + strings.Join(assignments, ", ") + " WHERE " + where,
			Args:  args,
		}, nil

	case models.DMLDeleteType:
		if len(change.PrimaryKeyInfo) == 0 {
			return models.Query{}, errors.New("delete requires primary key values")
		}
		where, err := db.clickHouseWhere(change.PrimaryKeyInfo, columnTypes, inlineValues, &args)
		if err != nil {
			return models.Query{}, err
		}
		return models.Query{
			Query: "ALTER TABLE " + formattedTableName + " DELETE WHERE " + where,
			Args:  args,
		}, nil
	}

	return models.Query{}, fmt.Errorf("unsupported DML change type %d", change.Type)
}

func changeUsesCompositeType(change models.DBDMLChange, columnTypes map[string]string) bool {
	for _, value := range change.Values {
		if isClickHouseCompositeType(columnTypes[value.Column]) {
			return true
		}
	}
	for _, primaryKey := range change.PrimaryKeyInfo {
		if isClickHouseCompositeType(columnTypes[primaryKey.Name]) {
			return true
		}
	}
	return false
}

func (db *ClickHouse) clickHouseWhere(primaryKeyInfo []models.PrimaryKeyInfo, columnTypes map[string]string, preview bool, args *[]any) (string, error) {
	where := make([]string, 0, len(primaryKeyInfo))
	for _, primaryKey := range primaryKeyInfo {
		expression, err := db.clickHouseSQLValue(primaryKey.Value, models.String, columnTypes[primaryKey.Name], preview, args)
		if err != nil {
			return "", err
		}
		where = append(where, db.FormatReference(primaryKey.Name)+" = "+expression)
	}
	return strings.Join(where, " AND "), nil
}

func (db *ClickHouse) clickHouseSQLValue(value any, valueType models.CellValueType, databaseType string, preview bool, args *[]any) (string, error) {
	switch valueType {
	case models.Null:
		return "NULL", nil
	case models.Default:
		return "DEFAULT", nil
	}

	if isClickHouseCompositeType(databaseType) {
		literal := strings.TrimSpace(fmt.Sprint(value))
		expression, err := clickHouseCompositeExpression(literal, databaseType)
		if err != nil {
			return "", err
		}
		return "CAST(" + expression + " AS " + databaseType + ")", nil
	}

	if preview {
		if valueType == models.Empty {
			return "''", nil
		}
		return db.FormatArgForQueryString(value), nil
	}

	*args = append(*args, db.FormatArg(value, valueType))
	return db.FormatPlaceholder(len(*args)), nil
}

func clickHouseCompositeExpression(literal, databaseType string) (string, error) {
	typeName, typeArgs := clickHouseTypeParts(databaseType)
	var opening, closing byte
	switch typeName {
	case "Array":
		opening, closing = '[', ']'
	case "Map":
		opening, closing = '{', '}'
	case "Tuple":
		opening, closing = '(', ')'
	default:
		return literal, nil
	}

	if len(literal) < 2 || literal[0] != opening || literal[len(literal)-1] != closing {
		return "", fmt.Errorf("value for %s must be a ClickHouse literal enclosed by %c and %c", databaseType, opening, closing)
	}

	items, err := splitClickHouseLiteralList(literal[1 : len(literal)-1])
	if err != nil {
		return "", fmt.Errorf("invalid %s literal: %w", databaseType, err)
	}

	switch typeName {
	case "Map":
		if len(typeArgs) != 2 {
			return "", fmt.Errorf("invalid ClickHouse map type %s", databaseType)
		}
		mapArgs := make([]string, 0, len(items)*2)
		for _, item := range items {
			key, value, err := splitClickHouseMapEntry(item)
			if err != nil {
				return "", fmt.Errorf("invalid %s literal: %w", databaseType, err)
			}
			key, err = clickHouseCompositeExpression(strings.TrimSpace(key), typeArgs[0])
			if err != nil {
				return "", err
			}
			value, err = clickHouseCompositeExpression(strings.TrimSpace(value), typeArgs[1])
			if err != nil {
				return "", err
			}
			mapArgs = append(mapArgs, key, value)
		}
		return "map(" + strings.Join(mapArgs, ", ") + ")", nil

	case "Array":
		if len(typeArgs) == 1 {
			for i, item := range items {
				items[i], err = clickHouseCompositeExpression(strings.TrimSpace(item), typeArgs[0])
				if err != nil {
					return "", err
				}
			}
		}
		return "[" + strings.Join(items, ", ") + "]", nil

	case "Tuple":
		for i, item := range items {
			if i >= len(typeArgs) {
				break
			}
			items[i], err = clickHouseCompositeExpression(strings.TrimSpace(item), clickHouseTupleElementType(typeArgs[i]))
			if err != nil {
				return "", err
			}
		}
		return "(" + strings.Join(items, ", ") + ")", nil
	}

	return literal, nil
}

func splitClickHouseLiteralList(body string) ([]string, error) {
	if strings.TrimSpace(body) == "" {
		return nil, nil
	}

	items := make([]string, 0, 2)
	start := 0
	var round, square, curly int
	quoted, escaped := false, false
	for i := 0; i < len(body); i++ {
		char := body[i]
		if quoted {
			if escaped {
				escaped = false
				continue
			}
			switch char {
			case '\\':
				escaped = true
			case '\'':
				quoted = false
			}
			continue
		}

		switch char {
		case '\'':
			quoted = true
		case '(':
			round++
		case ')':
			round--
		case '[':
			square++
		case ']':
			square--
		case '{':
			curly++
		case '}':
			curly--
		case ',':
			if round == 0 && square == 0 && curly == 0 {
				items = append(items, strings.TrimSpace(body[start:i]))
				start = i + 1
			}
		}
		if round < 0 || square < 0 || curly < 0 {
			return nil, errors.New("unbalanced delimiters")
		}
	}
	if quoted || round != 0 || square != 0 || curly != 0 {
		return nil, errors.New("unbalanced delimiters or quotes")
	}
	items = append(items, strings.TrimSpace(body[start:]))
	return items, nil
}

func splitClickHouseMapEntry(entry string) (string, string, error) {
	quoted, escaped := false, false
	var round, square, curly int
	for i := 0; i < len(entry); i++ {
		char := entry[i]
		if quoted {
			if escaped {
				escaped = false
				continue
			}
			switch char {
			case '\\':
				escaped = true
			case '\'':
				quoted = false
			}
			continue
		}
		switch char {
		case '\'':
			quoted = true
		case '(':
			round++
		case ')':
			round--
		case '[':
			square++
		case ']':
			square--
		case '{':
			curly++
		case '}':
			curly--
		case ':':
			if round == 0 && square == 0 && curly == 0 {
				return entry[:i], entry[i+1:], nil
			}
		}
	}
	return "", "", errors.New("map entry has no top-level colon")
}

func isClickHouseCompositeType(databaseType string) bool {
	typeName, _ := clickHouseTypeParts(databaseType)
	return typeName == "Array" || typeName == "Map" || typeName == "Tuple"
}

func (db *ClickHouse) getColumnTypes(ctx context.Context, database, table string) (map[string]string, error) {
	columnTypes := make(map[string]string)
	// Keeping query generation usable without a connection is convenient for
	// tests and callers that only preview scalar values.
	if db.Connection == nil {
		return columnTypes, nil
	}

	rows, err := db.Connection.QueryContext(contextOrBackground(ctx), "SELECT name, type FROM system.columns WHERE database = ? AND table = ?", database, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name, columnType string
		if err := rows.Scan(&name, &columnType); err != nil {
			return nil, err
		}
		columnTypes[name] = columnType
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columnTypes, nil
}

func (db *ClickHouse) GetFunctions(_ context.Context, _ string) (map[string][]string, error) {
	return nil, errors.New("not implemented")
}

func (db *ClickHouse) GetProcedures(_ context.Context, _ string) (map[string][]string, error) {
	return nil, errors.New("not implemented")
}

func (db *ClickHouse) GetViews(_ context.Context, _ string) (map[string][]string, error) {
	return nil, errors.New("not implemented")
}

func (db *ClickHouse) SupportsProgramming() bool {
	return false
}

func (db *ClickHouse) UseSchemas() bool {
	return false
}

func (db *ClickHouse) GetFunctionDefinition(_ context.Context, _ string, _ string) (string, error) {
	return "", errors.New("not implemented")
}

func (db *ClickHouse) GetProcedureDefinition(_ context.Context, _ string, _ string) (string, error) {
	return "", errors.New("not implemented")
}

func (db *ClickHouse) GetViewDefinition(_ context.Context, _ string, _ string) (string, error) {
	return "", errors.New("not implemented")
}

// queryMetadata runs a metadata query and returns the column names followed
// by the rows as strings.
func (db *ClickHouse) queryMetadata(ctx context.Context, query string, args ...any) ([][]string, error) {
	rows, err := db.Connection.QueryContext(contextOrBackground(ctx), query, args...)
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

func clickHouseMutationContext(ctx context.Context) context.Context {
	return clickhouse.Context(contextOrBackground(ctx), clickhouse.WithSettings(clickhouse.Settings{
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
	return formatClickHouseValue(value, databaseType, false)
}

func formatClickHouseValue(value any, databaseType string, nested bool) (string, bool) {
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
	typeName, typeArgs := clickHouseTypeParts(databaseType)

	switch val := value.(type) {
	case string:
		if nested {
			return clickHouseQuoteString(val), false
		}
		return val, false
	case time.Time:
		formatted := val.Format("2006-01-02 15:04:05.999999999")
		if typeName == "Date" || typeName == "Date32" {
			formatted = val.Format(time.DateOnly)
		}
		if nested {
			formatted = clickHouseQuoteString(formatted)
		}
		return formatted, false
	case fmt.Stringer:
		formatted := val.String()
		if nested && clickHouseTypeNeedsQuotes(typeName) {
			formatted = clickHouseQuoteString(formatted)
		}
		return formatted, false
	}

	// Types such as big.Int (Int128/Int256) implement fmt.Stringer on the
	// pointer receiver only.
	ptr := reflect.New(v.Type())
	ptr.Elem().Set(v)
	if stringer, ok := ptr.Interface().(fmt.Stringer); ok {
		formatted := stringer.String()
		if nested && clickHouseTypeNeedsQuotes(typeName) {
			formatted = clickHouseQuoteString(formatted)
		}
		return formatted, false
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		elements := make([]string, v.Len())
		for i := range elements {
			elementType := ""
			switch typeName {
			case "Array":
				if len(typeArgs) > 0 {
					elementType = typeArgs[0]
				}
			case "Tuple":
				if i < len(typeArgs) {
					elementType = clickHouseTupleElementType(typeArgs[i])
				}
			}
			formatted, isNull := formatClickHouseValue(v.Index(i).Interface(), elementType, true)
			if isNull {
				formatted = "NULL"
			}
			elements[i] = formatted
		}
		if typeName == "Tuple" {
			return "(" + strings.Join(elements, ",") + ")", false
		}
		return "[" + strings.Join(elements, ",") + "]", false

	case reflect.Map:
		keyType, valueType := "", ""
		if typeName == "Map" && len(typeArgs) == 2 {
			keyType, valueType = typeArgs[0], typeArgs[1]
		}
		entries := make([]string, 0, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			key, _ := formatClickHouseValue(iter.Key().Interface(), keyType, true)
			mapValue, isNull := formatClickHouseValue(iter.Value().Interface(), valueType, true)
			if isNull {
				mapValue = "NULL"
			}
			entries = append(entries, key+":"+mapValue)
		}
		sort.Strings(entries)
		return "{" + strings.Join(entries, ",") + "}", false
	}

	return fmt.Sprintf("%v", value), false
}

func clickHouseTypeNeedsQuotes(typeName string) bool {
	switch typeName {
	case "String", "FixedString", "UUID", "IPv4", "IPv6", "Enum8", "Enum16", "Date", "Date32", "DateTime", "DateTime64":
		return true
	default:
		return false
	}
}

// clickHouseTypeParts unwraps Nullable and LowCardinality and returns the base
// type name and its top-level arguments.
func clickHouseTypeParts(databaseType string) (string, []string) {
	databaseType = strings.TrimSpace(databaseType)
	for {
		name, args := splitClickHouseType(databaseType)
		if (name == "Nullable" || name == "LowCardinality") && len(args) == 1 {
			databaseType = args[0]
			continue
		}
		return name, args
	}
}

func splitClickHouseType(databaseType string) (string, []string) {
	open := strings.IndexByte(databaseType, '(')
	if open < 0 || !strings.HasSuffix(databaseType, ")") {
		return strings.TrimSpace(databaseType), nil
	}

	name := strings.TrimSpace(databaseType[:open])
	body := databaseType[open+1 : len(databaseType)-1]
	args := make([]string, 0, 2)
	start, depth := 0, 0
	quoted, escaped := false, false
	for i := 0; i < len(body); i++ {
		char := body[i]
		if quoted {
			if escaped {
				escaped = false
				continue
			}
			switch char {
			case '\\':
				escaped = true
			case '\'':
				quoted = false
			}
			continue
		}
		switch char {
		case '\'':
			quoted = true
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(body[start:i]))
				start = i + 1
			}
		}
	}
	args = append(args, strings.TrimSpace(body[start:]))
	return name, args
}

func clickHouseTupleElementType(tupleElement string) string {
	tupleElement = strings.TrimSpace(tupleElement)
	depth := 0
	for i, char := range tupleElement {
		switch char {
		case '(':
			depth++
		case ')':
			depth--
		case ' ':
			if depth == 0 {
				return strings.TrimSpace(tupleElement[i+1:])
			}
		}
	}
	return tupleElement
}

func (db *ClickHouse) GetEstimatedRowCount(ctx context.Context, database, table string) (*int64, error) {
	if database == "" || table == "" {
		return nil, errors.New("database and table names are required")
	}
	var count sql.NullInt64
	err := db.Connection.QueryRowContext(contextOrBackground(ctx), "SELECT total_rows FROM system.tables WHERE database = ? AND name = ?", database, table).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !count.Valid || count.Int64 < 0 {
		return nil, nil
	}
	return &count.Int64, nil
}
func (db *ClickHouse) GetExactRowCount(ctx context.Context, database, table, where string) (int64, error) {
	if database == "" || table == "" {
		return 0, errors.New("database and table names are required")
	}
	query := "SELECT count() FROM " + db.formatTableName(database, table)
	if where != "" {
		query += " " + where
	}
	var count int64
	err := db.Connection.QueryRowContext(contextOrBackground(ctx), query).Scan(&count)
	return count, err
}
func (db *ClickHouse) StreamQuery(ctx context.Context, _, query string, maxRows int, onBatch func(QueryBatch) error) (QueryStreamResult, error) {
	return streamQueryWithScanner(ctx, db.Connection, query, maxRows, onBatch, func(rows *sql.Rows, _ int) ([]string, error) {
		types, err := rows.ColumnTypes()
		if err != nil {
			return nil, err
		}
		values, nulls, err := scanClickHouseRow(rows, types)
		for i := range values {
			if nulls[i] {
				values[i] = "NULL"
			}
		}
		return values, err
	})
}
