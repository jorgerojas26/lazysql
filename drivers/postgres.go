package drivers

import (
	"database/sql"
	"errors"
	"fmt"
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

func (db *Postgres) Connect(urlstr string) (err error) {
	db.SetProvider("postgres")

	db.Connection, err = dburl.Open(urlstr)
	if err != nil {
		return err
	}

	err = db.Connection.Ping()
	if err != nil {
		return err
	}

	db.Urlstr = urlstr

	// get current database

	rows := db.Connection.QueryRow("SELECT current_database();")

	database := ""

	err = rows.Scan(&database)

	db.CurrentDatabase = database
	db.PreviousDatabase = database
	if err != nil {
		return err
	}

	return nil
}

func (db *Postgres) GetDatabases() (databases []string, err error) {
	rows, err := db.Connection.Query("SELECT datname FROM pg_database;")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	rowsErr := rows.Err()

	if rowsErr != nil {
		err = rowsErr
		return nil, err
	}

	for rows.Next() {
		var database string
		err := rows.Scan(&database)
		if err != nil {
			return nil, err
		}
		databases = append(databases, database)
	}

	return databases, nil
}

func (db *Postgres) GetTables(database string) (tables map[string][]string, err error) {
	tables = make(map[string][]string)

	logger.Info("GetTables", map[string]any{"database": database})

	if database == "" {
		return nil, errors.New("database name is required")
	}

	if database != db.CurrentDatabase {
		err = db.SwitchDatabase(database)
		if err != nil {
			return nil, err
		}
	}

	defer func() {
		if r := recover(); r != nil {
			_ = db.SwitchDatabase(db.PreviousDatabase)
		}
	}()

	query := "SELECT table_name, table_schema FROM information_schema.tables WHERE table_catalog = $1"
	rows, err := db.Connection.Query(query, database)

	if rows != nil {
		rowsErr := rows.Err()

		if rowsErr != nil {
			err = rowsErr
		}

		defer rows.Close()

		for rows.Next() {
			var tableName string
			var tableSchema string

			err = rows.Scan(&tableName, &tableSchema)

			tables[tableSchema] = append(tables[tableSchema], tableName)

		}

	}

	if err != nil {
		return nil, err
	}

	return tables, nil
}

func (db *Postgres) GetTableColumns(database, table string) (results [][]string, err error) {
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
		err = db.SwitchDatabase(database)
		if err != nil {
			return nil, err
		}
	}

	defer func() {
		if r := recover(); r != nil {
			_ = db.SwitchDatabase(db.PreviousDatabase)
		}
	}()

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	query := "SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_catalog = $1 AND table_schema = $2 AND table_name = $3 ORDER by ordinal_position"

	rows, err := db.Connection.Query(query, database, tableSchema, tableName)

	if rows != nil {

		rowsErr := rows.Err()

		if rowsErr != nil {
			err = rowsErr
		}

		defer rows.Close()

		columns, columnsError := rows.Columns()

		if columnsError != nil {
			err = columnsError
		}

		results = append(results, columns)

		for rows.Next() {
			rowValues := make([]interface{}, len(columns))

			for i := range columns {
				rowValues[i] = new(sql.RawBytes)
			}

			err = rows.Scan(rowValues...)

			var row []string
			for _, col := range rowValues {
				row = append(row, string(*col.(*sql.RawBytes)))
			}

			results = append(results, row)
		}

	}

	if err != nil {
		return nil, err
	}

	return
}

func (db *Postgres) GetConstraints(database, table string) (constraints [][]string, err error) {
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
		err = db.SwitchDatabase(database)
		if err != nil {
			return nil, err
		}
	}

	defer func() {
		if r := recover(); r != nil {
			_ = db.SwitchDatabase(db.PreviousDatabase)
		}
	}()

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

	if rows != nil {

		rowsErr := rows.Err()

		if rowsErr != nil {
			err = rowsErr
		}

		defer rows.Close()

		columns, columnsError := rows.Columns()

		if columnsError != nil {
			err = columnsError
		}

		constraints = append(constraints, columns)

		for rows.Next() {
			rowValues := make([]interface{}, len(columns))
			for i := range columns {
				rowValues[i] = new(sql.RawBytes)
			}

			err = rows.Scan(rowValues...)

			var row []string
			for _, col := range rowValues {
				row = append(row, string(*col.(*sql.RawBytes)))
			}

			constraints = append(constraints, row)
		}
	}

	if err != nil {
		return nil, err
	}

	return
}

func (db *Postgres) GetForeignKeys(database, table string) (foreignKeys [][]string, err error) {
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
		err = db.SwitchDatabase(database)
		if err != nil {
			return nil, err
		}
	}

	defer func() {
		if r := recover(); r != nil {
			_ = db.SwitchDatabase(db.PreviousDatabase)
		}
	}()

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

	if rows != nil {

		rowsErr := rows.Err()

		if rowsErr != nil {
			err = rowsErr
		}

		defer rows.Close()

		columns, columnsError := rows.Columns()

		if columnsError != nil {
			err = columnsError
		}

		foreignKeys = append(foreignKeys, columns)

		for rows.Next() {
			rowValues := make([]interface{}, len(columns))
			for i := range columns {
				rowValues[i] = new(sql.RawBytes)
			}

			err = rows.Scan(rowValues...)

			var row []string
			for _, col := range rowValues {
				row = append(row, string(*col.(*sql.RawBytes)))
			}

			foreignKeys = append(foreignKeys, row)
		}
	}

	if err != nil {
		return nil, err
	}

	return
}

func (db *Postgres) GetIndexes(database, table string) (indexes [][]string, err error) {
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
		err = db.SwitchDatabase(database)
		if err != nil {
			return nil, err
		}
	}

	defer func() {
		if r := recover(); r != nil {
			_ = db.SwitchDatabase(db.PreviousDatabase)
		}
	}()

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

	if rows != nil {

		rowsErr := rows.Err()

		if rowsErr != nil {
			err = rowsErr
		}

		defer rows.Close()

		columns, columnsError := rows.Columns()

		if columnsError != nil {
			err = columnsError
		}

		indexes = append(indexes, columns)

		for rows.Next() {
			rowValues := make([]interface{}, len(columns))
			for i := range columns {
				rowValues[i] = new(sql.RawBytes)
			}

			err = rows.Scan(rowValues...)

			var row []string
			for _, col := range rowValues {
				row = append(row, string(*col.(*sql.RawBytes)))
			}

			indexes = append(indexes, row)
		}
	}

	if err != nil {
		return nil, err
	}

	return
}

func (db *Postgres) GetRecords(database, table, where, sort string, offset, limit int) (records [][]string, totalRecords int, err error) {
	if database == "" {
		return nil, 0, errors.New("database name is required")
	}

	if table == "" {
		return nil, 0, errors.New("table name is required")
	}

	splitTableString := strings.Split(table, ".")

	if len(splitTableString) == 1 {
		return nil, 0, errors.New("table must be in the format schema.table")
	}

	if database != db.CurrentDatabase {
		err = db.SwitchDatabase(database)
		if err != nil {
			return nil, 0, err
		}
	}

	defer func() {
		if r := recover(); r != nil {
			if database != db.PreviousDatabase {
				_ = db.SwitchDatabase(db.PreviousDatabase)
			}
		}
	}()

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	formattedTableName := db.formatTableName(tableSchema, tableName)

	if limit == 0 {
		limit = DefaultRowLimit
	}

	query := "SELECT * FROM "
	query += formattedTableName

	if where != "" {
		query += fmt.Sprintf(" %s", where)
	}

	if sort != "" {
		query += fmt.Sprintf(" ORDER BY %s", sort)
	}

	query += " LIMIT $1 OFFSET $2"

	paginatedRows, err := db.Connection.Query(query, limit, offset)

	if paginatedRows != nil {

		rowsErr := paginatedRows.Err()

		defer paginatedRows.Close()

		if rowsErr != nil {
			err = rowsErr
		}

		countQuery := "SELECT COUNT(*) FROM "
		countQuery += formattedTableName

		rows := db.Connection.QueryRow(countQuery)

		rowsErr = rows.Err()

		if rowsErr != nil {
			err = rowsErr
		}

		err = rows.Scan(&totalRecords)

		columns, columnsError := paginatedRows.Columns()

		if columnsError != nil {
			err = columnsError
		}

		records = append(records, columns)

		for paginatedRows.Next() {
			rowValues := make([]interface{}, len(columns))
			for i := range columns {
				rowValues[i] = new(sql.RawBytes)
			}

			err = paginatedRows.Scan(rowValues...)

			var row []string
			for _, col := range rowValues {
				row = append(row, string(*col.(*sql.RawBytes)))
			}

			records = append(records, row)

		}
	}

	if err != nil {
		return nil, 0, err
	}

	return
}

func (db *Postgres) UpdateRecord(database, table, column, value, primaryKeyColumnName, primaryKeyValue string) (err error) {
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

	splitTableString := strings.Split(table, ".")

	if len(splitTableString) == 1 {
		return errors.New("table must be in the format schema.table")
	}

	switchDatabaseOnError := false

	if database != db.CurrentDatabase {
		err = db.SwitchDatabase(database)
		if err != nil {
			return err
		}
		switchDatabaseOnError = true
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	formattedTableName := db.formatTableName(tableSchema, tableName)

	query := "UPDATE "
	query += formattedTableName
	query += fmt.Sprintf(" SET \"%s\" = $1 WHERE \"%s\" = $2", column, primaryKeyColumnName)

	_, err = db.Connection.Exec(query, value, primaryKeyValue)

	if err != nil && switchDatabaseOnError {
		err = db.SwitchDatabase(db.PreviousDatabase)
	}

	return err
}

func (db *Postgres) DeleteRecord(database, table, primaryKeyColumnName, primaryKeyValue string) (err error) {
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

	splitTableString := strings.Split(table, ".")

	if len(splitTableString) == 1 {
		return errors.New("table must be in the format schema.table")
	}

	switchDatabaseOnError := false

	if database != db.CurrentDatabase {
		err = db.SwitchDatabase(database)
		if err != nil {
			return err
		}
		switchDatabaseOnError = true
	}

	tableSchema := splitTableString[0]
	tableName := splitTableString[1]

	formattedTableName := db.formatTableName(tableSchema, tableName)

	query := "DELETE FROM "
	query += formattedTableName
	query += fmt.Sprintf(" WHERE \"%s\" = $1", primaryKeyColumnName)

	_, err = db.Connection.Exec(query, primaryKeyValue)

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

func (db *Postgres) ExecuteQuery(query string) (results [][]string, err error) {
	rows, err := db.Connection.Query(query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	rowsErr := rows.Err()

	if rowsErr != nil {
		err = rowsErr
	}

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

func (db *Postgres) ExecutePendingChanges(changes []models.DbDmlChange) (err error) {
	var query []models.Query

	for _, change := range changes {
		columnNames := []string{}
		values := []interface{}{}
		valuesPlaceholder := []string{}
		placeholderIndex := 1

		for _, cell := range change.Values {
			switch cell.Type {
			case models.Empty, models.Null, models.String:
				columnNames = append(columnNames, cell.Column)
				valuesPlaceholder = append(valuesPlaceholder, fmt.Sprintf("$%d", placeholderIndex))
				placeholderIndex++
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
			case models.Default:
				break
			}
		}

		splitTableString := strings.Split(change.Table, ".")

		tableSchema := splitTableString[0]
		tableName := splitTableString[1]

		formattedTableName := db.formatTableName(tableSchema, tableName)

		switch change.Type {

		case models.DmlInsertType:

			queryStr := "INSERT INTO " + formattedTableName
			queryStr += fmt.Sprintf(" (%s) VALUES (%s)", strings.Join(columnNames, ", "), strings.Join(valuesPlaceholder, ", "))

			newQuery := models.Query{
				Query: queryStr,
				Args:  values,
			}

			query = append(query, newQuery)
		case models.DmlUpdateType:
			queryStr := "UPDATE " + formattedTableName

			for i, column := range columnNames {
				if i == 0 {
					queryStr += fmt.Sprintf(" SET \"%s\" = $1", column)
				} else {
					queryStr += fmt.Sprintf(", \"%s\" = $%d", column, i+1)
				}
			}

			args := make([]interface{}, len(values))

			copy(args, values)

			queryStr += fmt.Sprintf(" WHERE \"%s\" = $%d", change.PrimaryKeyColumnName, len(columnNames)+1)
			args = append(args, change.PrimaryKeyValue)

			newQuery := models.Query{
				Query: queryStr,
				Args:  args,
			}

			query = append(query, newQuery)
		case models.DmlDeleteType:
			queryStr := "DELETE FROM " + formattedTableName
			queryStr += fmt.Sprintf(" WHERE %s = $1", change.PrimaryKeyColumnName)

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

	db.Connection.Close()
	db.Connection = connection
	db.PreviousDatabase = db.CurrentDatabase
	db.CurrentDatabase = database

	return nil
}

func (db *Postgres) formatTableName(database, table string) string {
	return fmt.Sprintf("\"%s\".\"%s\"", database, table)
}
