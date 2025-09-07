package drivers

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	// import postgresql driver
	_ "github.com/lib/pq"
	"github.com/xo/dburl"

	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
)

type Postgres struct {
	Connection       *sql.DB
	Provider         string
	CurrentDatabase  string
	PreviousDatabase string
	Urlstr           string
}

const (
	defaultPort = "5432"
)

func (db *Postgres) TestConnection(urlstr string) error {
	return db.Connect(urlstr)
}

func (db *Postgres) Connect(urlstr string) error {
	db.SetProvider(DriverPostgres)

	connection, err := dburl.Open(urlstr)
	if err != nil {
		return err
	}

	db.Connection = connection

	err = db.Connection.Ping()
	if err != nil {
		return err
	}

	db.Urlstr = urlstr

	// Get the current database.
	rows := db.Connection.QueryRow("SELECT current_database();")

	database := ""
	err = rows.Scan(&database)
	if err != nil {
		return err
	}

	db.CurrentDatabase = database
	db.PreviousDatabase = database

	return nil
}

func (db *Postgres) GetDatabases() ([]string, error) {
	rows, err := db.Connection.Query("SELECT datname FROM pg_database;")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var database string
		err := rows.Scan(&database)
		if err != nil {
			return nil, err
		}
		databases = append(databases, database)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return databases, nil
}

func (db *Postgres) GetTables(database string) (map[string][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if database != db.CurrentDatabase {
		err := db.SwitchDatabase(database)
		if err != nil {
			return nil, err
		}

		defer func() {
			if err != nil {
				_ = db.SwitchDatabase(db.PreviousDatabase)
			}
		}()
	}

	query := "SELECT table_name, table_schema FROM information_schema.tables WHERE table_catalog = $1"
	rows, err := db.Connection.Query(query, database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make(map[string][]string)
	for rows.Next() {
		var (
			tableName   string
			tableSchema string
		)
		if err := rows.Scan(&tableName, &tableSchema); err != nil {
			return nil, err
		}

		tables[tableSchema] = append(tables[tableSchema], tableName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tables, nil
}

func (db *Postgres) GetTableColumns(database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if table == "" {
		return nil, errors.New("table name is required")
	}

	splitTableString := strings.Split(table, ".")

	if len(splitTableString) == 1 {
		return nil, errors.New("table must be in the format schema.table")
	}

	if database != db.CurrentDatabase {
		err := db.SwitchDatabase(database)
		if err != nil {
			return nil, err
		}

		defer func() {
			if err != nil {
				_ = db.SwitchDatabase(db.PreviousDatabase)
			}
		}()
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	query := "SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_catalog = $1 AND table_schema = $2 AND table_name = $3 ORDER by ordinal_position"

	rows, err := db.Connection.Query(query, database, tableSchema, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results := [][]string{columns}
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

func (db *Postgres) GetConstraints(database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if table == "" {
		return nil, errors.New("table name is required")
	}

	splitTableString := strings.Split(table, ".")
	if len(splitTableString) == 1 {
		return nil, errors.New("table must be in the format schema.table")
	}

	if database != db.CurrentDatabase {
		err := db.SwitchDatabase(database)
		if err != nil {
			return nil, err
		}

		defer func() {
			if err != nil {
				_ = db.SwitchDatabase(db.PreviousDatabase)
			}
		}()
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	rows, err := db.Connection.Query(fmt.Sprintf(`
        SELECT
            tc.constraint_name,
            kcu.column_name,
            tc.constraint_type
        FROM
            information_schema.table_constraints AS tc
            JOIN information_schema.key_column_usage AS kcu ON tc.constraint_name = kcu.constraint_name
            AND tc.table_schema = kcu.table_schema
            JOIN information_schema.constraint_column_usage AS ccu ON ccu.constraint_name = tc.constraint_name
            AND ccu.table_schema = tc.table_schema
        WHERE
            NOT tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = '%s'
            AND tc.table_name = '%s'
            `, tableSchema, tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	constraints := [][]string{columns}
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

		constraints = append(constraints, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return constraints, nil
}

func (db *Postgres) GetForeignKeys(database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if table == "" {
		return nil, errors.New("table name is required")
	}

	splitTableString := strings.Split(table, ".")
	if len(splitTableString) == 1 {
		return nil, errors.New("table must be in the format schema.table")
	}

	if database != db.CurrentDatabase {
		err := db.SwitchDatabase(database)
		if err != nil {
			return nil, err
		}

		defer func() {
			if err != nil {
				_ = db.SwitchDatabase(db.PreviousDatabase)
			}
		}()
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	rows, err := db.Connection.Query(fmt.Sprintf(`
        SELECT
            tc.constraint_name,
            kcu.column_name,
            ccu.table_name AS foreign_table_name,
            ccu.column_name AS foreign_column_name
        FROM
            information_schema.table_constraints AS tc
            JOIN information_schema.key_column_usage AS kcu ON tc.constraint_name = kcu.constraint_name
            AND tc.table_schema = kcu.table_schema
            JOIN information_schema.constraint_column_usage AS ccu ON ccu.constraint_name = tc.constraint_name
            AND ccu.table_schema = tc.table_schema
        WHERE
            tc.constraint_type = 'FOREIGN KEY'
          	AND tc.table_schema = '%s'
            AND tc.table_name = '%s'
  `, tableSchema, tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	foreignKeys := [][]string{columns}
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

		foreignKeys = append(foreignKeys, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return foreignKeys, nil
}

func (db *Postgres) GetIndexes(database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if table == "" {
		return nil, errors.New("table name is required")
	}

	splitTableString := strings.Split(table, ".")
	if len(splitTableString) == 1 {
		return nil, errors.New("table must be in the format schema.table")
	}

	if database != db.CurrentDatabase {
		err := db.SwitchDatabase(database)
		if err != nil {
			return nil, err
		}

		defer func() {
			if err != nil {
				_ = db.SwitchDatabase(db.PreviousDatabase)
			}
		}()
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	rows, err := db.Connection.Query(fmt.Sprintf(`
        SELECT
            i.relname AS index_name,
            a.attname AS column_name,
            am.amname AS type
        FROM
            pg_namespace n,
            pg_class t,
            pg_class i,
            pg_index ix,
            pg_attribute a,
            pg_am am
        WHERE
            t.oid = ix.indrelid
            and i.oid = ix.indexrelid
            and a.attrelid = t.oid
            and a.attnum = ANY(ix.indkey)
            and t.relkind = 'r'
            and am.oid = i.relam
          	and n.oid = t.relnamespace
            and n.nspname = '%s'
            and t.relname = '%s'
        ORDER BY
            t.relname,
            i.relname
  `, tableSchema, tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	indexes := [][]string{columns}
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

		indexes = append(indexes, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return indexes, nil
}

func (db *Postgres) GetRecords(database, table, where, sort string, offset, limit int) (records [][]string, totalRecords int, queryString string, err error) {
	if database == "" {
		return nil, 0, "", errors.New("database name is required")
	}
	if table == "" {
		return nil, 0, "", errors.New("table name is required")
	}

	formattedTableName, err := db.formatTableName(table)
	if err != nil {
		return nil, 0, "", err
	}

	if database != db.CurrentDatabase {
		err := db.SwitchDatabase(database)
		if err != nil {
			return nil, 0, "", err
		}

		defer func() {
			if err != nil {
				_ = db.SwitchDatabase(db.PreviousDatabase)
			}
		}()
	}

	queryString = "SELECT * FROM "
	queryString += formattedTableName

	if where != "" {
		queryString += fmt.Sprintf(" %s", where)
	}

	if sort != "" {
		queryString += fmt.Sprintf(" ORDER BY %s", sort)
	}

	queryString += " LIMIT $1 OFFSET $2"

	if limit == 0 {
		limit = DefaultRowLimit
	}

	paginatedRows, err := db.Connection.Query(queryString, limit, offset)
	if err != nil {
		return nil, 0, queryString, err
	}
	defer paginatedRows.Close()

	columns, columnsError := paginatedRows.Columns()
	if columnsError != nil {
		return nil, 0, queryString, columnsError
	}

	records = [][]string{columns}
	for paginatedRows.Next() {
		nullStringSlice := make([]sql.NullString, len(columns))

		rowValues := make([]any, len(columns))
		for i := range nullStringSlice {
			rowValues[i] = &nullStringSlice[i]
		}

		if err := paginatedRows.Scan(rowValues...); err != nil {
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

		records = append(records, row)
	}

	if err := paginatedRows.Err(); err != nil {
		return nil, 0, queryString, err
	}
	// close to release the connection
	if err := paginatedRows.Close(); err != nil {
		return nil, 0, queryString, err
	}

	countQuery := "SELECT COUNT(*) FROM "
	countQuery += formattedTableName

	if where != "" {
		countQuery += fmt.Sprintf(" %s", where)
	}

	countRow := db.Connection.QueryRow(countQuery)

	if err := countRow.Scan(&totalRecords); err != nil {
		return records, 0, queryString, err
	}

	// Replace the limit and offset with actual values in the query string
	queryString = strings.Replace(queryString, "$1", strconv.Itoa(limit), 1)
	queryString = strings.Replace(queryString, "$2", strconv.Itoa(offset), 1)

	return records, totalRecords, queryString, nil
}

func (db *Postgres) UpdateRecord(database, table, column, value, primaryKeyColumnName, primaryKeyValue string) error {
	if database == "" {
		return errors.New("database name is required")
	}
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

	formattedTableName, formatErr := db.formatTableName(table)

	if formatErr != nil {
		return formatErr
	}

	switchDatabaseOnError := false
	if database != db.CurrentDatabase {
		err := db.SwitchDatabase(database)
		if err != nil {
			return err
		}
		switchDatabaseOnError = true
	}

	query := "UPDATE "
	query += formattedTableName
	query += fmt.Sprintf(" SET \"%s\" = $1 WHERE \"%s\" = $2", column, primaryKeyColumnName)

	_, err := db.Connection.Exec(query, value, primaryKeyValue)
	if err != nil && switchDatabaseOnError {
		err = db.SwitchDatabase(db.PreviousDatabase)
	}

	return err
}

func (db *Postgres) DeleteRecord(database, table, primaryKeyColumnName, primaryKeyValue string) error {
	if database == "" {
		return errors.New("database name is required")
	}
	if table == "" {
		return errors.New("table name is required")
	}
	if primaryKeyColumnName == "" {
		return errors.New("primary key column name is required")
	}
	if primaryKeyValue == "" {
		return errors.New("primary key value is required")
	}

	formattedTableName, formatErr := db.formatTableName(table)
	if formatErr != nil {
		return formatErr
	}

	switchDatabaseOnError := false
	if database != db.CurrentDatabase {
		err := db.SwitchDatabase(database)
		if err != nil {
			return err
		}
		switchDatabaseOnError = true
	}

	query := "DELETE FROM "
	query += formattedTableName
	query += fmt.Sprintf(" WHERE \"%s\" = $1", primaryKeyColumnName)

	_, err := db.Connection.Exec(query, primaryKeyValue)
	if err != nil && switchDatabaseOnError {
		err = db.SwitchDatabase(db.PreviousDatabase)
	}

	return err
}

func (db *Postgres) ExecuteDMLStatement(query string) (result string, err error) {
	res, err := db.Connection.Exec(query)
	if err != nil {
		return result, err
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return result, err
	}
	return fmt.Sprintf("%d rows affected", rowsAffected), nil
}

func (db *Postgres) ExecuteQuery(query string) ([][]string, int, error) {
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

func (db *Postgres) ExecutePendingChanges(changes []models.DBDMLChange) error {
	var queries []models.Query

	for _, change := range changes {

		formattedTableName, formatErr := db.formatTableName(change.Table)
		if formatErr != nil {
			return formatErr
		}

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

func (db *Postgres) GetPrimaryKeyColumnNames(database, table string) ([]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}
	if table == "" {
		return nil, errors.New("table name is required")
	}

	splitTableString := strings.Split(table, ".")
	if len(splitTableString) != 2 {
		return nil, errors.New("table must be in the format schema.table")
	}

	schemaName := splitTableString[0]
	tableName := splitTableString[1]

	if database != db.CurrentDatabase {
		err := db.SwitchDatabase(database)
		if err != nil {
			return nil, err
		}

		defer func() {
			if err != nil {
				_ = db.SwitchDatabase(db.PreviousDatabase)
			}
		}()
	}

	row, err := db.Connection.Query(`
		SELECT
			a.attname AS column_name
		FROM
			pg_index i
			JOIN pg_class c ON c.oid = i.indrelid
			JOIN pg_attribute a ON a.attrelid = c.oid
				AND a.attnum = ANY (i.indkey)
			JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE
			relname = $2 AND nspname = $1 AND indisprimary
	`, schemaName, tableName)
	if err != nil {
		logger.Error("GetPrimaryKeyColumnNames", map[string]any{"error": err.Error()})
		return nil, err
	}

	defer row.Close()

	var primaryKeyColumnName []string
	for row.Next() {
		var colName string
		err = row.Scan(&colName)
		if err != nil {
			return nil, err
		}

		if row.Err() != nil {
			return nil, row.Err()
		}

		primaryKeyColumnName = append(primaryKeyColumnName, colName)
	}

	if row.Err() != nil {
		return nil, row.Err()
	}

	return primaryKeyColumnName, nil
}

func (db *Postgres) SetProvider(provider string) {
	db.Provider = provider
}

func (db *Postgres) GetProvider() string {
	return db.Provider
}

func (db *Postgres) SwitchDatabase(database string) error {
	parsedConn, err := dburl.Parse(db.Urlstr)
	if err != nil {
		return err
	}

	user := parsedConn.User.Username()
	password, _ := parsedConn.User.Password()
	host := parsedConn.Host
	port := parsedConn.Port()
	dbname := parsedConn.Path

	if port == "" {
		port = defaultPort
	}

	if dbname == "" {
		dbname = database
	}

	connection, err := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=%s password=%s dbname='%s' sslmode=disable", host, port, user, password, dbname))
	if err != nil {
		return err
	}

	err = db.Connection.Close()
	if err != nil {
		return err
	}

	db.Connection = connection
	db.PreviousDatabase = db.CurrentDatabase
	db.CurrentDatabase = database

	return nil
}

func (db *Postgres) formatTableName(table string) (string, error) {
	splitTableString := strings.Split(table, ".")

	if len(splitTableString) == 1 {
		return "", errors.New("table must be in the format schema.table")
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	return fmt.Sprintf("\"%s\".\"%s\"", tableSchema, tableName), nil
}

func (db *Postgres) FormatArg(arg any, colType models.CellValueType) any {
	if colType == models.Null {
		return sql.NullString{
			String: "",
			Valid:  false,
		}
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
			trimmed := strings.TrimRight(s, "0")
			if strings.HasSuffix(trimmed, ".") {
				trimmed += "0"
			}
			return trimmed
		case string:
			return v
		case []byte:
			return string(v)
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

func (db *Postgres) FormatArgForQueryString(arg any) string {
	switch v := arg.(type) {
	case string:
		if v == "NULL" || v == "DEFAULT" {
			return v
		}
		escaped := strings.ReplaceAll(v, "'", "''")
		return "'" + escaped + "'"
	case sql.NullString:
		if !v.Valid {
			return "NULL"
		}
		escaped := strings.ReplaceAll(v.String, "'", "''")
		return "'" + escaped + "'"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (db *Postgres) FormatReference(reference string) string {
	return fmt.Sprintf("\"%s\"", reference)
}

func (db *Postgres) FormatPlaceholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

func (db *Postgres) DMLChangeToQueryString(change models.DBDMLChange) (string, error) {
	var queryStr string

	formattedTableName, err := db.formatTableName(change.Table)
	if err != nil {
		return "", err
	}

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
