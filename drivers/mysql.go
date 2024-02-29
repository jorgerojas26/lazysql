package drivers

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/jorgerojas26/lazysql/models"

	_ "github.com/go-sql-driver/mysql"
	"github.com/xo/dburl"
)

type MySQL struct {
	Connection  *sql.DB
	Provider    string
	PrimaryKeys map[string]string
}

func (db *MySQL) TestConnection(urlstr string) (err error) {
	return db.Connect(urlstr)
}

func (db *MySQL) Connect(urlstr string) (err error) {
	db.SetProvider("mysql")

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
		return databases, err
	}

	for rows.Next() {
		var database string
		err := rows.Scan(&database)
		if err != nil {
			return databases, err
		}
		if database != "information_schema" && database != "mysql" && database != "performance_schema" && database != "sys" {
			databases = append(databases, database)
		}
	}

	return databases, nil
}

func (db *MySQL) GetTables(database string) (map[string][]string, error) {
	rows, err := db.Connection.Query("SHOW TABLES FROM " + database)

	tables := make(map[string][]string)

	if err != nil {
		return tables, err
	}

	for rows.Next() {
		var table string
		rows.Scan(&table)

		tables[database] = append(tables[database], table)
	}

	db.PrimaryKeys = make(map[string]string, len(tables))

	return tables, nil
}

func (db *MySQL) GetTableColumns(database, table string) (results [][]string, err error) {
	rows, err := db.Connection.Query("DESCRIBE " + table)
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

func (db *MySQL) GetConstraints(table string) (results [][]string, err error) {
	splitTableString := strings.Split(table, ".")
	database := splitTableString[0]
	tableName := splitTableString[1]

	rows, err := db.Connection.Query(fmt.Sprintf("SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE where TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s'", database, tableName))
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

func (db *MySQL) GetForeignKeys(table string) (results [][]string, err error) {
	splitTableString := strings.Split(table, ".")
	database := splitTableString[0]
	tableName := splitTableString[1]

	rows, err := db.Connection.Query(fmt.Sprintf("SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_COLUMN_NAME, REFERENCED_TABLE_NAME FROM information_schema.KEY_COLUMN_USAGE where REFERENCED_TABLE_SCHEMA = '%s' AND REFERENCED_TABLE_NAME = '%s'", database, tableName))
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

func (db *MySQL) GetIndexes(table string) (results [][]string, err error) {
	rows, err := db.Connection.Query("SHOW INDEX FROM " + table)
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

func (db *MySQL) GetRecords(table, where, sort string, offset, limit int) (paginatedResults [][]string, totalRecords int, err error) {
	defaultLimit := 300

	isPaginationEnabled := offset >= 0 && limit >= 0

	if limit != 0 {
		defaultLimit = limit
	}

	query := fmt.Sprintf("SELECT * FROM %s s LIMIT %d,%d", table, offset, defaultLimit)

	if where != "" {
		query = fmt.Sprintf("SELECT * FROM %s %s LIMIT %d,%d", table, where, offset, defaultLimit)
	}

	if sort != "" {
		query = fmt.Sprintf("SELECT * FROM %s %s ORDER BY %s LIMIT %d,%d", table, where, sort, offset, defaultLimit)
	}

	paginatedRows, err := db.Connection.Query(query)
	if err != nil {
		return paginatedResults, totalRecords, err
	}

	if isPaginationEnabled {
		queryWithoutLimit := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", table, where)

		rows := db.Connection.QueryRow(queryWithoutLimit)

		if err != nil {
			return paginatedResults, totalRecords, err
		}

		rows.Scan(&totalRecords)

		defer paginatedRows.Close()
	}

	columns, _ := paginatedRows.Columns()

	paginatedResults = append(paginatedResults, columns)

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

		paginatedResults = append(paginatedResults, row)

	}

	return
}

func (db *MySQL) ExecuteQuery(query string) (results [][]string, err error) {
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

func (db *MySQL) UpdateRecord(table, column, value, id string) error {
	primaryKey := db.GetPrimaryKey(table)
	query := fmt.Sprintf("UPDATE %s SET %s = \"%s\" WHERE %s = \"%s\"", table, column, value, primaryKey, id)
	_, err := db.Connection.Exec(query)

	return err
}

func (db *MySQL) DeleteRecord(table, id string) error {
	primaryKey := db.GetPrimaryKey(table)
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = \"%s\"", table, primaryKey, id)
	_, err := db.Connection.Exec(query)

	return err
}

func (db *MySQL) ExecuteDMLStatement(query string) (result string, err error) {
	res, error := db.Connection.Exec(query)

	if error != nil {
		return result, error
	} else {
		rowsAffected, _ := res.RowsAffected()

		return fmt.Sprintf("%d rows affected", rowsAffected), error
	}
}

func (db *MySQL) ExecutePendingChanges(changes []models.DbDmlChange, inserts []models.DbInsert) (err error) {
	queries := make([]string, 0, len(changes)+len(inserts))

	// This will hold grouped changes by their RowId and Table
	groupedUpdated := make(map[string][]models.DbDmlChange)
	groupedDeletes := make([]models.DbDmlChange, 0, len(changes))

	// Group changes by RowId and Table
	for _, change := range changes {
		if change.Type == "UPDATE" {
			key := fmt.Sprintf("%s|%s", change.Table, change.RowId)
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

		primaryKey := db.GetPrimaryKey(table)
		query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = '%s';", table, updateClause, primaryKey, rowId)

		queries = append(queries, query)
	}

	for _, delete := range groupedDeletes {
		statementType := ""
		query := ""

		statementType = "DELETE FROM"
		primaryKey := db.GetPrimaryKey(delete.Table)
		query = fmt.Sprintf("%s %s WHERE %s = \"%s\"", statementType, delete.Table, primaryKey, delete.RowId)

		if query != "" {
			queries = append(queries, query)
		}
	}

	for _, insert := range inserts {
		values := make([]string, 0, len(insert.Values))

		for _, value := range insert.Values {
			_, error := strconv.ParseFloat(value, 64)

			if strings.ToLower(value) != "default" && error != nil {
				values = append(values, fmt.Sprintf("\"%s\"", value))
			} else {
				values = append(values, value)
			}
		}

		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", insert.Table, strings.Join(insert.Columns, ", "), strings.Join(values, ", "))

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

func (db *MySQL) SetProvider(provider string) {
	db.Provider = provider
}

func (db *MySQL) GetProvider() string {
	return db.Provider
}

func (db *MySQL) GetPrimaryKey(table string) string {
	primaryKey, ok := db.PrimaryKeys[table]
	if ok {
		return primaryKey
	}

	type Row struct {
		Field   string
		Type    string
		Null    string
		Key     string
		Default sql.NullString
		Extra   string
	}

	query := fmt.Sprintf("show columns from %s where `Key` = \"PRI\";", table)

	var row Row
	queryRow := db.Connection.QueryRow(query)
	err := queryRow.Scan(
		&row.Field,
		&row.Type,
		&row.Null,
		&row.Key,
		&row.Default,
		&row.Extra,
	)

	if err == nil {
		db.PrimaryKeys[table] = row.Field

		return row.Field
	}

	return "id"
}
