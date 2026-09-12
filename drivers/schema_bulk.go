package drivers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// newBulkColumnResults creates ready result buckets for every requested table.
// A bucket with only headers represents a successful lookup of a table with no
// visible columns and keeps the shared cache's ready/empty distinction intact.
func newBulkColumnResults(tables []string, headers []string) map[string][][]string {
	results := make(map[string][][]string, len(tables))
	for _, table := range tables {
		results[table] = [][]string{append([]string(nil), headers...)}
	}
	return results
}

func bulkResultKey(results map[string][][]string, name string) string {
	if _, ok := results[name]; ok {
		return name
	}
	lowerName := strings.ToLower(name)
	for existing := range results {
		if strings.ToLower(existing) == lowerName {
			return existing
		}
	}
	return ""
}

// GetTableColumnsBulk loads MySQL column metadata for all requested tables in
// one information_schema query. The result shape matches GetTableColumns so a
// bulk result can be shared with the Records metadata surface unchanged.
func (db *MySQL) GetTableColumnsBulk(ctx context.Context, database string, tables []string) (map[string][][]string, error) {
	ctx = contextOrBackground(ctx)
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if len(tables) == 0 {
		return map[string][][]string{}, nil
	}

	const headers = "Field,Type,Collation,Null,Key,Default,Extra,Privileges,Comment"
	columnHeaders := strings.Split(headers, ",")
	results := newBulkColumnResults(tables, columnHeaders)
	placeholders := make([]string, len(tables))
	args := make([]any, 0, len(tables)+1)
	args = append(args, database)
	for i, table := range tables {
		placeholders[i] = "?"
		args = append(args, table)
	}

	query := `SELECT TABLE_NAME, COLUMN_NAME, COLUMN_TYPE, COLLATION_NAME,
		IS_NULLABLE, COLUMN_KEY, COLUMN_DEFAULT, EXTRA, PRIVILEGES, COLUMN_COMMENT
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME IN (` + strings.Join(placeholders, ", ") + `)
		ORDER BY TABLE_NAME, ORDINAL_POSITION`
	rows, err := db.Connection.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			tableName, columnName, columnType, collation sql.NullString
			isNullable, columnKey, columnDefault, extra  sql.NullString
			privileges, comment                          sql.NullString
		)
		if err := rows.Scan(
			&tableName, &columnName, &columnType, &collation,
			&isNullable, &columnKey, &columnDefault, &extra,
			&privileges, &comment,
		); err != nil {
			return nil, err
		}

		key := bulkResultKey(results, tableName.String)
		if key == "" {
			continue
		}
		results[key] = append(results[key], []string{
			columnName.String,
			columnType.String,
			collation.String,
			isNullable.String,
			columnKey.String,
			columnDefault.String,
			extra.String,
			privileges.String,
			comment.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetTableColumnsBulk loads PostgreSQL columns for the requested schema.table
// names with one information_schema query. The schema-qualified result keys
// preserve the key format required by the existing PostgreSQL driver API.
func (db *Postgres) GetTableColumnsBulk(ctx context.Context, database string, tables []string) (map[string][][]string, error) {
	ctx = contextOrBackground(ctx)
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if len(tables) == 0 {
		return map[string][][]string{}, nil
	}

	const headers = "column_name,data_type,is_nullable,column_default,comment"
	columnHeaders := strings.Split(headers, ",")
	results := newBulkColumnResults(tables, columnHeaders)
	filters := make([]string, 0, len(tables))
	args := make([]any, 0, len(tables)*2+1)
	args = append(args, database)
	for _, table := range tables {
		schema, tableName, ok := strings.Cut(table, ".")
		if !ok || schema == "" || tableName == "" || strings.Contains(tableName, ".") {
			return nil, fmt.Errorf("table must be in the format schema.table: %s", table)
		}
		schemaArg := len(args) + 1
		args = append(args, schema)
		tableArg := len(args) + 1
		args = append(args, tableName)
		filters = append(filters, fmt.Sprintf("(c.table_schema = $%d AND c.table_name = $%d)", schemaArg, tableArg))
	}

	query := `SELECT c.table_schema, c.table_name, c.column_name, c.data_type,
		c.is_nullable, c.column_default, COALESCE(pd.description, '') AS comment
		FROM information_schema.columns c
		LEFT JOIN pg_class pc ON pc.relname = c.table_name
			AND pc.relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = c.table_schema)
		LEFT JOIN pg_namespace pn ON pn.nspname = c.table_schema AND pn.oid = pc.relnamespace
		LEFT JOIN pg_description pd ON pd.objoid = pc.oid AND pd.objsubid = c.ordinal_position
		WHERE c.table_catalog = $1 AND (` + strings.Join(filters, " OR ") + `)
		ORDER BY c.table_schema, c.table_name, c.ordinal_position`

	conn, needsClose, err := db.connectionFor(ctx, database)
	if err != nil {
		return nil, err
	}
	if needsClose {
		defer conn.Close()
	}
	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			schema, tableName, columnName, dataType sql.NullString
			isNullable, columnDefault, comment      sql.NullString
		)
		if err := rows.Scan(&schema, &tableName, &columnName, &dataType, &isNullable, &columnDefault, &comment); err != nil {
			return nil, err
		}

		key := bulkResultKey(results, schema.String+"."+tableName.String)
		if key == "" {
			continue
		}
		results[key] = append(results[key], []string{
			columnName.String,
			dataType.String,
			isNullable.String,
			columnDefault.String,
			comment.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetTableColumnsBulk loads MSSQL columns for all requested tables after
// switching to the requested database. MSSQL's existing API uses the current
// schema, so table names remain bare just like GetTableColumns.
func (db *MSSQL) GetTableColumnsBulk(ctx context.Context, database string, tables []string) (map[string][][]string, error) {
	ctx = contextOrBackground(ctx)
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if len(tables) == 0 {
		return map[string][][]string{}, nil
	}

	columnHeaders := []string{"column_name", "data_type", "is_nullable", "column_default", "comment"}
	results := newBulkColumnResults(tables, columnHeaders)
	placeholders := make([]string, len(tables))
	args := make([]any, len(tables))
	for i, table := range tables {
		placeholders[i] = fmt.Sprintf("@p%d", i+1)
		args[i] = table
	}

	query := db.databasePrefix(database) + `
		SELECT t.name AS table_name,
			c.name AS column_name,
			ty.name AS data_type,
			c.is_nullable,
			def.definition AS column_default,
			ISNULL(ep.value, '') AS comment
		FROM sys.tables t
		INNER JOIN sys.columns c ON t.object_id = c.object_id
		INNER JOIN sys.types ty ON c.system_type_id = ty.system_type_id
		LEFT JOIN sys.default_constraints def ON c.default_object_id = def.parent_column_id
		LEFT JOIN sys.extended_properties ep ON ep.major_id = c.object_id
			AND ep.minor_id = c.column_id AND ep.name = 'MS_Description'
		WHERE t.name IN (` + strings.Join(placeholders, ", ") + `)
			AND ty.name <> 'sysname'
		ORDER BY t.name, c.column_id;
	`
	rows, err := db.Connection.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			tableName, columnName, dataType    sql.NullString
			isNullable, columnDefault, comment sql.NullString
		)
		if err := rows.Scan(&tableName, &columnName, &dataType, &isNullable, &columnDefault, &comment); err != nil {
			return nil, err
		}

		key := bulkResultKey(results, tableName.String)
		if key == "" {
			continue
		}
		results[key] = append(results[key], []string{
			columnName.String,
			dataType.String,
			isNullable.String,
			columnDefault.String,
			comment.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// GetTableColumnsBulk uses SQLite's table-valued PRAGMA to fetch the requested
// tables in one catalog query. Older SQLite builds that do not expose this
// pragma simply remain eligible for the shared loader's lazy path.
func (db *SQLite) GetTableColumnsBulk(ctx context.Context, _ string, tables []string) (map[string][][]string, error) {
	ctx = contextOrBackground(ctx)
	if len(tables) == 0 {
		return map[string][][]string{}, nil
	}

	columnHeaders := []string{"name", "type", "notnull", "dflt_value", "pk"}
	results := newBulkColumnResults(tables, columnHeaders)
	placeholders := make([]string, len(tables))
	args := make([]any, len(tables))
	for i, table := range tables {
		placeholders[i] = "?"
		args[i] = table
	}

	query := `SELECT m.name, p.name, p.type, p."notnull", p.dflt_value, p.pk
		FROM sqlite_master AS m, pragma_table_info(m.name) AS p
		WHERE m.type = 'table' AND m.name IN (` + strings.Join(placeholders, ", ") + `)
		ORDER BY m.name, p.cid`
	rows, err := db.Connection.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			tableName, columnName, dataType   sql.NullString
			notNull, defaultValue, primaryKey sql.NullString
		)
		if err := rows.Scan(&tableName, &columnName, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}

		key := bulkResultKey(results, tableName.String)
		if key == "" {
			continue
		}
		results[key] = append(results[key], []string{
			columnName.String,
			dataType.String,
			notNull.String,
			defaultValue.String,
			primaryKey.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
