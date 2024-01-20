package drivers

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/jorgerojas26/lazysql/helpers"
	"github.com/jorgerojas26/lazysql/models"
	"github.com/xo/dburl"

	_ "github.com/lib/pq"
)

type Postgres struct {
	Connection *sql.DB
	Provider   string
}

func (db *Postgres) Connect(urlstr string) (err error) {
	parsed, _ := helpers.ParseConnectionString(urlstr)

	db.SetProvider(parsed.Driver)

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

func (db *Postgres) TestConnection(urlstr string) error {
	return db.Connect(urlstr)
}

func (db *Postgres) GetDatabases() (databases []string, err error) {
	rows, err := db.Connection.Query("SELECT datname FROM pg_database;")
	if err != nil {
		return databases, err
	}

	for rows.Next() {
		var database string
		err := rows.Scan(&database)
		if err != nil {
			return databases, err
		}
		databases = append(databases, database)
	}

	return databases, nil
}

// TODO: Implement GetSchemas function
func (db *Postgres) GetTables(database string) (tables map[string][]string, err error) {
	tables = make(map[string][]string)

	rows, err := db.Connection.Query(fmt.Sprintf("SELECT table_name, table_schema FROM information_schema.tables WHERE table_catalog = '%s'", database))
	if err != nil {
		return tables, err
	}

	for rows.Next() {
		var tableName string
		var tableSchema string

		rows.Scan(&tableName, &tableSchema)

		tables[tableSchema] = append(tables[tableSchema], tableName)

	}

	return tables, nil
}

func (db *Postgres) GetTableColumns(database, table string) (results [][]string, error error) {
	tableName := strings.Split(table, ".")[1]
	rows, err := db.Connection.Query(fmt.Sprintf("SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_catalog = '%s' AND table_name = '%s'", database, tableName))
	if err != nil {
		return results, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return results, err
	}

	results = append(results, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		rows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}

	return
}

func (db *Postgres) GetConstraints(table string) (constraints [][]string, error error) {
	splitTableString := strings.Split(table, ".")
	tableName := splitTableString[1]

	rows, err := db.Connection.Query(fmt.Sprintf(`
  SELECT
            tc.constraint_name,
            kcu.column_name
        FROM
            information_schema.table_constraints AS tc
            JOIN information_schema.key_column_usage AS kcu ON tc.constraint_name = kcu.constraint_name
            AND tc.table_schema = kcu.table_schema
            JOIN information_schema.constraint_column_usage AS ccu ON ccu.constraint_name = tc.constraint_name
            AND ccu.table_schema = tc.table_schema
        WHERE
            NOT tc.constraint_type = 'FOREIGN KEY'
            AND tc.table_name = '%s'
            `, tableName))
	if err != nil {
		return constraints, err
	}

	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return constraints, err
	}

	constraints = append(constraints, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		rows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		constraints = append(constraints, row)
	}

	return
}

func (db *Postgres) GetForeignKeys(table string) (foreignKeys [][]string, error error) {
	splitTableString := strings.Split(table, ".")
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
            AND tc.table_name = '%s'
  `, tableName))
	if err != nil {
		return foreignKeys, err
	}

	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return foreignKeys, err
	}

	foreignKeys = append(foreignKeys, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		rows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		foreignKeys = append(foreignKeys, row)
	}

	return
}

func (db *Postgres) GetIndexes(table string) (indexes [][]string, error error) {
	rows, err := db.Connection.Query(fmt.Sprintf(`
  SELECT
            i.relname AS index_name,
            a.attname AS column_name,
            am.amname AS type
        FROM
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
            and t.relname = '%s'
        ORDER BY
            t.relname,
            i.relname
  `, table))
	if err != nil {
		return indexes, err
	}
	defer rows.Close()

	columns, _ := rows.Columns()

	indexes = append(indexes, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		rows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		indexes = append(indexes, row)
	}

	return
}

func (db *Postgres) GetRecords(table, where, sort string, offset, limit int) (records [][]string, totalRecords int, err error) {
	defaultLimit := 300

	splitTableString := strings.Split(table, ".")
	tableName := splitTableString[1]

	isPaginationEnabled := offset >= 0 && limit >= 0

	if limit != 0 {
		defaultLimit = limit
	}

	query := fmt.Sprintf("SELECT * FROM %s s LIMIT %d OFFSET %d", tableName, defaultLimit, offset)

	if where != "" {
		query = fmt.Sprintf("SELECT * FROM %s %s LIMIT %d OFFSET %d", tableName, where, defaultLimit, offset)
	}

	if sort != "" {
		query = fmt.Sprintf("SELECT * FROM %s %s ORDER BY %s LIMIT %d OFFSET %d", tableName, where, sort, defaultLimit, offset)
	}

	paginatedRows, err := db.Connection.Query(query)
	if err != nil {
		return records, totalRecords, err
	}

	if isPaginationEnabled {
		queryWithoutLimit := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", tableName, where)

		rows := db.Connection.QueryRow(queryWithoutLimit)

		if err != nil {
			return records, totalRecords, err
		}

		rows.Scan(&totalRecords)

		defer paginatedRows.Close()
	}

	columns, _ := paginatedRows.Columns()

	records = append(records, columns)

	for paginatedRows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		paginatedRows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		records = append(records, row)

	}

	return
}

// TODO: Rewrites this logic to use the primary key instead of the id
func (db *Postgres) UpdateRecord(table, column, value, id string) (err error) {
	query := fmt.Sprintf("UPDATE %s SET %s = '%s' WHERE id = '%s'", table, column, value, id)
	_, err = db.Connection.Exec(query)

	return err
}

// TODO: Rewrites this logic to use the primary key instead of the id
func (db *Postgres) DeleteRecord(table, id string) (err error) {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = '%s'", table, id)
	_, err = db.Connection.Exec(query)

	return err
}

func (db *Postgres) ExecuteDMLStatement(query string) (result string, err error) {
	res, error := db.Connection.Exec(query)

	if error != nil {
		return result, error
	} else {
		rowsAffected, _ := res.RowsAffected()

		return fmt.Sprintf("%d rows affected", rowsAffected), error
	}
}

func (db *Postgres) ExecuteQuery(query string) (results [][]string, err error) {
	rows, err := db.Connection.Query(query)
	if err != nil {
		return results, err
	}

	defer rows.Close()

	columns, _ := rows.Columns()

	results = append(results, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		rows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)

	}

	return
}

func (db *Postgres) ExecutePendingChanges(changes []models.DbDmlChange, inserts []models.DbInsert) (err error) {
	queries := make([]string, 0, len(changes)+len(inserts))

	// This will hold grouped changes by their RowId and Table
	groupedUpdated := make(map[string][]models.DbDmlChange)
	groupedDeletes := make([]models.DbDmlChange, 0, len(changes))

	// Group changes by RowId and Table
	for _, change := range changes {
		if change.Type == "UPDATE" {
			tableName := strings.Split(change.Table, ".")[1]
			key := fmt.Sprintf("%s|%s", tableName, change.RowId)
			groupedUpdated[key] = append(groupedUpdated[key], change)
		} else if change.Type == "DELETE" {
			groupedDeletes = append(groupedDeletes, change)
		}
	}

	// Combine individual changes to SQL statements
	for key, changes := range groupedUpdated {
		columns := []string{}

		// Split key into table and rowId
		splitted := strings.Split(key, "|")
		table := splitted[0]
		rowId := splitted[1]

		for _, change := range changes {
			columns = append(columns, fmt.Sprintf("%s='%s'", change.Column, change.Value))
		}

		// Merge all column updates
		updateClause := strings.Join(columns, ", ")

		query := fmt.Sprintf("UPDATE %s SET %s WHERE id = '%s';", table, updateClause, rowId)

		queries = append(queries, query)
	}

	for _, delete := range groupedDeletes {
		statementType := ""
		query := ""

		statementType = "DELETE FROM"
		tableName := strings.Split(delete.Table, ".")[1]
		query = fmt.Sprintf("%s %s WHERE id = \"%s\"", statementType, tableName, delete.RowId)

		if query != "" {
			queries = append(queries, query)
		}
	}

	for _, insert := range inserts {
		values := make([]string, 0, len(insert.Values))

		for _, value := range insert.Values {
			_, error := strconv.ParseFloat(value, 64)

			if strings.ToLower(value) != "default" && error != nil {
				values = append(values, fmt.Sprintf("'%s'", value))
			} else {
				values = append(values, value)
			}
		}

		tableName := strings.Split(insert.Table, ".")[1]
		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, strings.Join(insert.Columns, ", "), strings.Join(values, ", "))

		queries = append(queries, query)
	}

	tx, error := db.Connection.Begin()
	if error != nil {
		return error
	}

	for _, query := range queries {
		_, err = tx.Exec(query)

		if err != nil {
			tx.Rollback()

			return err
		}
	}

	err = tx.Commit()

	if err != nil {
		return err
	}

	return err
}

func (db *Postgres) SetProvider(provider string) {
	db.Provider = provider
}

func (db *Postgres) GetProvider() string {
	return db.Provider
}
