package drivers

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	// mssql driver
	_ "github.com/microsoft/go-mssqldb"
	"github.com/xo/dburl"

	"github.com/google/uuid"
	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
)

type MSSQL struct {
	Connection *sql.DB
	Provider   string
}

func (db *MSSQL) TestConnection(urlstr string) error {
	return db.Connect(urlstr)
}

func (db *MSSQL) Connect(urlstr string) error {
	if urlstr == "" {
		return errors.New("url string can not be empty")
	}

	db.SetProvider(DriverMSSQL)

	var err error

	db.Connection, err = dburl.Open(urlstr)
	if err != nil {
		return err
	}

	if err := db.Connection.Ping(); err != nil {
		return err
	}

	return nil
}

func (db *MSSQL) GetDatabases() ([]string, error) {
	databases := make([]string, 0)

	query := `
		SELECT
			name
		FROM
			sys.databases
	`
	rows, err := db.Connection.Query(query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

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

func (db *MSSQL) GetTables(database string) (map[string][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	tables := make(map[string][]string)

	query := `SELECT name FROM sys.tables`
	rows, err := db.Connection.Query(query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

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

func (db *MSSQL) GetTableColumns(database, table string) ([][]string, error) {
	query := `
        SELECT
            c.name AS column_name,
            t.name AS data_type,
            c.is_nullable,
            def.definition AS column_default
        FROM sys.columns c
        INNER JOIN sys.types t ON c.system_type_id = t.system_type_id
        LEFT JOIN sys.default_constraints def ON c.default_object_id = def.parent_column_id
        WHERE c.object_id = OBJECT_ID(@p2)
        AND t.name <> 'sysname'
        ORDER BY c.column_id;
    `
	return db.getTableInformations(query, database, table, "")
}

func (db *MSSQL) GetConstraints(_, table string) ([][]string, error) {
	currentSchema, err := db.getCurrentSchema()
	if err != nil {
		return nil, err
	}

	query := `
        SELECT 
            kc.name AS constraint_name,
            c.name AS column_name,
            kc.type_desc AS constraint_type
        FROM sys.key_constraints kc
        INNER JOIN sys.tables t 
            ON kc.parent_object_id = t.object_id
        INNER JOIN sys.schemas s 
            ON t.schema_id = s.schema_id
        INNER JOIN sys.index_columns ic 
            ON kc.unique_index_id = ic.index_id 
            AND kc.parent_object_id = ic.object_id
        INNER JOIN sys.columns c 
            ON ic.column_id = c.column_id 
            AND ic.object_id = c.object_id
        WHERE s.name = @p1
          AND t.name = @p2
          AND kc.type IN ('PK', 'UQ')  -- Primary keys and unique constraints
    `
	return db.getTableInformations(query, currentSchema, table, "")
}

func (db *MSSQL) GetForeignKeys(database, table string) ([][]string, error) {
	query := `
        SELECT 
            fk.name AS constraint_name,
            c.name AS column_name,
            DB_NAME(DB_ID(@p1)) AS current_database,
            OBJECT_SCHEMA_NAME(fk.referenced_object_id, DB_ID(@p1)) + '.' + 
            OBJECT_NAME(fk.referenced_object_id, DB_ID(@p1)) AS referenced_table,
            rc.name AS referenced_column,
            fk.delete_referential_action_desc AS delete_rule,
            fk.update_referential_action_desc AS update_rule
        FROM sys.foreign_keys fk
        INNER JOIN sys.foreign_key_columns fkc 
            ON fk.object_id = fkc.constraint_object_id
        INNER JOIN sys.columns c 
            ON fkc.parent_column_id = c.column_id 
            AND fkc.parent_object_id = c.object_id
        INNER JOIN sys.columns rc 
            ON fkc.referenced_column_id = rc.column_id 
            AND fkc.referenced_object_id = rc.object_id
        INNER JOIN sys.tables t 
            ON fk.parent_object_id = t.object_id
        INNER JOIN sys.schemas s 
            ON t.schema_id = s.schema_id
        WHERE t.name = @p2
          AND DB_NAME(DB_ID(@p1)) = @p1
    `
	return db.getTableInformations(query, database, table, "")
}

func (db *MSSQL) GetIndexes(database, table string) ([][]string, error) {
	currentSchema, err := db.getCurrentSchema()
	if err != nil {
		return nil, err
	}

	query := `
        SELECT
            t.name AS table_name,
            i.name AS index_name,
            CAST(i.is_unique AS BIT) AS is_unique,
            CAST(i.is_primary_key AS BIT) AS is_primary_key,
            i.type_desc AS index_type,
            c.name AS column_name,
            ic.key_ordinal AS seq_in_index,
            CAST(ic.is_included_column AS BIT) AS is_included,
            CAST(i.has_filter AS BIT) AS has_filter,
            i.filter_definition
        FROM sys.tables t
        INNER JOIN sys.schemas s 
            ON t.schema_id = s.schema_id
        INNER JOIN sys.databases d 
            ON d.name = @p1
        INNER JOIN sys.indexes i 
            ON t.object_id = i.object_id
        INNER JOIN sys.index_columns ic 
            ON i.object_id = ic.object_id 
            AND i.index_id = ic.index_id
        INNER JOIN sys.columns c 
            ON ic.column_id = c.column_id 
            AND t.object_id = c.object_id
        WHERE t.name = @p2
          AND s.name = @p3
          AND DB_ID(@p1) = d.database_id
        ORDER BY i.type_desc
    `
	return db.getTableInformations(query, database, table, currentSchema)
}

func (db *MSSQL) GetRecords(database, table, where, sort string, offset, limit int) ([][]string, int, error) {
	if database == "" {
		return nil, 0, errors.New("database name is required")
	}

	if table == "" {
		return nil, 0, errors.New("table name is required")
	}

	if limit == 0 {
		limit = DefaultRowLimit
	}

	results := make([][]string, 0)

	query := "SELECT * FROM "
	query += db.FormatReference(table)

	if where != "" {
		query += fmt.Sprintf(" %s", where)
	}

	// Since in MSSQL, ORDER BY is mandatory when using pagination
	if sort == "" {
		sort = "(SELECT NULL)"
	}

	query += fmt.Sprintf(" ORDER BY %s OFFSET @p1 ROWS FETCH NEXT @p2 ROWS ONLY", sort)

	rows, err := db.Connection.Query(query, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, 0, err
	}

	results = append(results, columns)

	for rows.Next() {
		rowValues := make([]any, len(columns))

		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		if err := rows.Scan(rowValues...); err != nil {
			return nil, 0, err
		}
		// Get column types to identify UNIQUEIDENTIFIER
		columnTypes, err := rows.ColumnTypes()
		if err != nil {
			return nil, 0, err
		}
		var row []string
		for i, col := range rowValues {
			if col == nil {
				row = append(row, "NULL&")
				continue
			}

			rawBytes := *col.(*sql.RawBytes)
			colType := columnTypes[i].DatabaseTypeName()

			if colType == "UNIQUEIDENTIFIER" {
				// Convert binary GUID to standard string format
				guid, err := uuid.FromBytes(rawBytes)
				if err != nil {
					return nil, 0, fmt.Errorf("failed to parse GUID: %w", err)
				}
				row = append(row, guid.String())
			} else {
				// Handle non-GUID columns as strings
				colval := string(rawBytes)
				if colval == "" {
					row = append(row, "EMPTY&")
				} else {
					row = append(row, colval)
				}
			}
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	totalRecords := 0
	row := db.Connection.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", db.FormatReference(table)))
	if err := row.Scan(&totalRecords); err != nil {
		return nil, 0, err
	}

	return results, totalRecords, nil
}

func (db *MSSQL) UpdateRecord(database, table, column, value, primaryKeyColumnName, primaryKeyValue string) error {
	if database == "" {
		return errors.New("database name is required")
	}

	if table == "" {
		return errors.New("table name is required")
	}

	if column == "" {
		return errors.New("table column is required")
	}

	if primaryKeyColumnName == "" {
		return errors.New("primary key column is required")
	}

	if primaryKeyValue == "" {
		return errors.New("primary key value is required")
	}

	query := "UPDATE "
	query += table
	query += " SET "
	query += column
	query += " = @p1 WHERE "
	query += primaryKeyColumnName
	query += " = @p2"
	_, err := db.Connection.Exec(query, value, primaryKeyValue)

	return err
}

func (db *MSSQL) DeleteRecord(database, table, primaryKeyColumnName, primaryKeyValue string) error {
	if database == "" {
		return errors.New("database name is required")
	}

	if table == "" {
		return errors.New("table name is required")
	}

	if primaryKeyColumnName == "" {
		return errors.New("primary key column is required")
	}

	if primaryKeyValue == "" {
		return errors.New("primary key value is required")
	}

	query := "DELETE FROM "
	query += table
	query += " WHERE "
	query += primaryKeyColumnName
	query += " = @p1"
	_, err := db.Connection.Exec(query, primaryKeyValue)

	return err
}

func (db *MSSQL) ExecuteDMLStatement(query string) (string, error) {
	if query == "" {
		return "", errors.New("query is required")
	}

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

func (db *MSSQL) ExecuteQuery(query string) ([][]string, int, error) {
	if query == "" {
		return nil, 0, errors.New("query can not be empty")
	}

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

		if err := rows.Scan(rowValues...); err != nil {
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

func (db *MSSQL) ExecutePendingChanges(changes []models.DBDMLChange) error {
	var queries []models.Query

	for _, change := range changes {

		formattedTableName := db.FormatReference(change.Table)

		switch change.Type {

		case models.DMLInsertType:
			queries = append(queries, buildInsertQuery(formattedTableName, change.Values, db))
		case models.DMLUpdateType:
			queries = append(queries, buildUpdateQuery(formattedTableName, change.Values, change.PrimaryKeyInfo, db))
		case models.DMLDeleteType:
			queries = append(queries, buildDeleteQuery(formattedTableName, change.PrimaryKeyInfo, db))
		}
	}

	logger.Info("queries", map[string]any{"queries": queries})

	return queriesInTransaction(db.Connection, queries)
}

func (db *MSSQL) GetPrimaryKeyColumnNames(database, table string) ([]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	currentSchema, err := db.getCurrentSchema()
	if err != nil {
		return nil, err
	}

	pkColumnName := make([]string, 0)
	query := `SELECT
			c.name AS column_name
		FROM
			sys.tables t
		INNER JOIN
			sys.schemas s
				ON t.schema_id = s.schema_id
		INNER JOIN
			sys.key_constraints kc
				ON t.object_id = kc.parent_object_id
				AND kc.type = @p1
		INNER JOIN
			sys.index_columns ic
				ON kc.unique_index_id = ic.index_id
				AND t.object_id = ic.object_id
		INNER JOIN
			sys.columns c
				ON ic.column_id = c.column_id
				AND t.object_id = c.object_id
		WHERE 
			s.name = @p2
			AND t.name = @p3
		ORDER BY ic.key_ordinal`
	rows, err := db.Connection.Query(query, "PK", currentSchema, table)
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

		pkColumnName = append(pkColumnName, colName)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return pkColumnName, nil
}

func (db *MSSQL) SetProvider(provider string) {
	db.Provider = provider
}

func (db *MSSQL) GetProvider() string {
	return db.Provider
}

// getTableInformations is used for following func:
//
//   - [GetTableColumns]
//   - [GetConstraints]
//   - [GetForeignKeys]
//   - [GetIndexes]
//
// getTableInformations requires following parameter:
//
//   - database name, used for filtering table_catalog
//   - table name, used for filtering table_name
func (db *MSSQL) getTableInformations(query, database, table, schema string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	if query == "" {
		return nil, errors.New("query can not be empty")
	}

	results := make([][]string, 0)

	args := []any{database, table}

	if schema != "" {
		args = append(args, schema)
	}

	rows, err := db.Connection.Query(query, args...)
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

func (db *MSSQL) FormatArg(arg any) string {
	if arg == "NULL" || arg == "DEFAULT" {
		return fmt.Sprintf("%v", arg)
	}

	switch v := arg.(type) {

	case int, int64:
		return fmt.Sprintf("%v", v)
	case float64:
		return fmt.Sprintf("%v", v)
	case string:
		escaped := strings.ReplaceAll(v, "'", "''")
		return fmt.Sprintf("'%s'", escaped)
	case []byte:
		return fmt.Sprintf("0x%x", v)
	case nil:
		return "NULL"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (db *MSSQL) FormatReference(reference string) string {
	return fmt.Sprintf("[%s]", reference)
}

func (db *MSSQL) FormatPlaceholder(index int) string {
	return fmt.Sprintf("@p%d", index)
}

func (db *MSSQL) DMLChangeToQueryString(change models.DBDMLChange) (string, error) {
	var queryStr string

	formattedTableName := db.FormatReference(change.Table)

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

func (db *MSSQL) getCurrentSchema() (string, error) {
	query := "SELECT SCHEMA_NAME() AS CurrentSchema"
	row := db.Connection.QueryRow(query)

	var currentSchema string
	err := row.Scan(&currentSchema)
	if err != nil {
		return "", err
	}

	return currentSchema, nil
}
