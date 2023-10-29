package drivers

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/xo/dburl"
)

type MySql struct {
	conn              *sql.DB
	connectionString  string
	lastExecutedQuery string
}

func (db *MySql) TestConnection() error {
	var err error

	db.conn, err = dburl.Open(db.connectionString)
	if err != nil {
		return err
	}

	err = db.conn.Ping()

	if err != nil {
		return err
	}

	return nil
}

func (db *MySql) ParseConnectionString(url string) (*dburl.URL, error) {
	return dburl.Parse(url)
}

func (db *MySql) Connect() error {
	var err error

	db.conn, err = dburl.Open(db.connectionString)
	// db.connection.SetConnMaxLifetime(time.Second * 120)
	// db.conn.SetMaxIdleConns(0)
	if err != nil {
		return err
	}

	err = db.conn.Ping()
	if err != nil {
		return err
	}

	// Start polling the database for deadlocks
	go func() {
		for {
			time.Sleep(30 * time.Second)
			db.conn.Ping()
		}
	}()
	// End polling the database for deadlocks

	return nil
}

func (db *MySql) SetConnectionString(connectionString string) {
	db.connectionString = connectionString
}

func (db *MySql) GetConnectionString() string {
	return db.connectionString
}

func (db *MySql) GetDatabases() ([]string, error) {
	var databases []string

	rows, err := db.conn.Query("SHOW DATABASES")
	if err != nil {
		return databases, err
	}

	for rows.Next() {
		var database string
		rows.Scan(&database)
		if database != "information_schema" && database != "mysql" && database != "performance_schema" && database != "sys" {
			databases = append(databases, database)
		}
	}

	return databases, nil
}

func (db *MySql) GetTables(database string) ([]string, error) {
	var tables []string

	rows, err := db.conn.Query("SHOW TABLES FROM " + database)
	if err != nil {
		return tables, err
	}

	for rows.Next() {
		var table string
		rows.Scan(&table)
		tables = append(tables, table)
	}

	return tables, nil
}

func (db *MySql) DescribeTable(table string) (results [][]string) {
	rows, _ := db.conn.Query("DESCRIBE " + table)
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

func (db *MySql) GetTableConstraints(table string) (results [][]string) {
	splitTableString := strings.Split(table, ".")
	database := splitTableString[0]
	tableName := splitTableString[1]

	rows, _ := db.conn.Query(fmt.Sprintf("SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE where TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s'", database, tableName))

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

func (db *MySql) GetTableForeignKeys(table string) (results [][]string) {
	splitTableString := strings.Split(table, ".")
	database := splitTableString[0]
	tableName := splitTableString[1]

	rows, _ := db.conn.Query(fmt.Sprintf("SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_COLUMN_NAME, REFERENCED_TABLE_NAME FROM information_schema.KEY_COLUMN_USAGE where REFERENCED_TABLE_SCHEMA = '%s' AND REFERENCED_TABLE_NAME = '%s'", database, tableName))
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

func (db *MySql) GetTableIndexes(table string) (results [][]string) {
	rows, _ := db.conn.Query("SHOW INDEX FROM " + table) // TODO: handle error
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

func (db *MySql) GetRecords(table string, where string, sort string, offset int, limit int, appendColumns bool) (results [][]string, err error) {
	defaultLimit := 100

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

	rows, err := db.conn.Query(query)
	if err != nil {
		return results, err
	}
	db.lastExecutedQuery = query

	defer rows.Close()

	columns, _ := rows.Columns()

	if appendColumns {
		results = append(results, columns)
	}

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

// Get paginated records
func (db *MySql) GetPaginatedRecords(table string, where string, sort string, offset int, limit int, appendColumns bool) (paginatedResults [][]string, totalRecords int, err error) {
	defaultLimit := 300

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

	paginatedRows, err := db.conn.Query(query)
	if err != nil {
		totalRecords = 0
		return paginatedResults, totalRecords, err
	}

	queryWithoutLimit := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", table, where)

	rows := db.conn.QueryRow(queryWithoutLimit)
	if err != nil {
		totalRecords = 0
		return paginatedResults, totalRecords, err
	}

	rows.Scan(&totalRecords)

	defer paginatedRows.Close()

	columns, _ := paginatedRows.Columns()

	if appendColumns {
		paginatedResults = append(paginatedResults, columns)
	}

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

func (db *MySql) QueryPaginatedRecords(query string) (results [][]string, err error) {
	rows, err := db.conn.Query(query)
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

func (db *MySql) UpdateRecord(table string, column string, value string, id string) error {
	query := fmt.Sprintf("UPDATE %s SET %s = \"%s\" WHERE id = \"%s\"", table, column, value, id)
	_, err := db.conn.Exec(query)

	return err
}

func (db *MySql) DeleteRecord(table string, id string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = \"%s\"", table, id)
	_, err := db.conn.Exec(query)

	return err
}

func (db *MySql) ExecuteDMLQuery(query string) (result string, err error) {
	res, error := db.conn.Exec(query)

	if error != nil {
		return result, error
	} else {
		rowsAffected, _ := res.RowsAffected()

		return fmt.Sprintf("%d rows affected", rowsAffected), error
	}
}

func (db *MySql) GetLastExecutedQuery() string {
	return db.lastExecutedQuery
}

//export the database
var MySQL MySql = MySql{}
