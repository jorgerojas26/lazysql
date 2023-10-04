package drivers

import (
	"database/sql"
	"fmt"
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
			time.Sleep(15 * time.Second)
			db.conn.Ping()
		}
	}()
	// End polling the database for deadlocks

	return nil
}

func (db *MySql) Disconnect() error {
	return db.conn.Close()
}

func (db *MySql) SetConnectionString(connectionString string) {
	db.connectionString = connectionString
}

func (db *MySql) GetConnectionString() string {
	return db.connectionString
}

func (db *MySql) GetConnection() *sql.DB {
	return db.conn
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

	results = append(results, []string{"Name", "Type", "Null", "Key", "Default", "Extra"})

	for rows.Next() {
		var field, type_, null, key, default_, extra string
		rows.Scan(&field, &type_, &null, &key, &default_, &extra)

		results = append(results, []string{field, type_, null, key, default_, extra})

	}

	return
}

func (db *MySql) GetTableConstraints(table string) (results [][]string) {
	rows, _ := db.conn.Query("SELECT COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_COLUMN_NAME, REFERENCED_TABLE_NAME FROM information_schema.KEY_COLUMN_USAGE where TABLE_NAME = " + "'" + table + "'")
	defer rows.Close()

	results = append(results, []string{"COLUMN_NAME", "CONSTRAINT_NAME", "REFERENCED_COLUMN_NAME", "REFERENCED_TABLE_NAME"})

	for rows.Next() {
		var columnName, constraintName, referencedColumnName, referencedTableName string
		rows.Scan(&columnName, &constraintName, &referencedColumnName, &referencedTableName)

		results = append(results, []string{columnName, constraintName, referencedColumnName, referencedTableName})

	}

	return
}

func (db *MySql) GetTableForeignKeys(table string) (results [][]string) {
	rows, _ := db.conn.Query("SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE where REFERENCED_TABLE_NAME = " + "'" + table + "'")
	defer rows.Close()

	results = append(results, []string{"TABLE_NAME", "COLUMN_NAME", "CONSTRAINT_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME"})

	for rows.Next() {
		var tableName, columnName, constraintName, referencedTableName, referencedColumnName string
		rows.Scan(&tableName, &columnName, &constraintName, &referencedTableName, &referencedColumnName)

		results = append(results, []string{tableName, columnName, constraintName, referencedTableName, referencedColumnName})

	}

	return
}

func (db *MySql) GetTableIndexes(table string) (results [][]string) {
	rows, _ := db.conn.Query("SHOW INDEX FROM " + table)
	defer rows.Close()

	results = append(results, []string{"Table", "Non_unique", "Key_name", "Seq_in_index", "Column_name", "Collation", "Cardinality", "Sub_part", "Packed", "Null", "Index_type", "Comment"})

	for rows.Next() {
		var tableName, nonUnique, keyName, seqInIndex, columnName, collation, cardinality, subPart, packed, null, indexType, comment string
		rows.Scan(&tableName, &nonUnique, &keyName, &seqInIndex, &columnName, &collation, &cardinality, &subPart, &packed, &null, &indexType, &comment)

		results = append(results, []string{tableName, nonUnique, keyName, seqInIndex, columnName, collation, cardinality, subPart, packed, null, indexType, comment})

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

func (db *MySql) UpdateRecord(table string, column string, value string, id string) error {
	query := fmt.Sprintf("UPDATE %s SET %s = \"%s\" WHERE id = \"%s\"", table, column, value, id)
	_, err := db.conn.Exec(query)

	return err
}

// lastExecutedQuery GETTER

func (db *MySql) GetLastExecutedQuery() string {
	return db.lastExecutedQuery
}

//export the database
var Database MySql = MySql{}
