package drivers

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/xo/dburl"
)

type MySql struct {
	connection       *sql.DB
	connectionString string
}

func (db *MySql) TestConnection() error {
	var err error

	db.connection, err = dburl.Open(db.connectionString)
	if err != nil {
		return err
	}

	err = db.connection.Ping()

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

	db.connection, err = dburl.Open(db.connectionString)
	db.connection.SetConnMaxIdleTime(10000)
	db.connection.SetConnMaxLifetime(10000)
	if err != nil {
		return err
	}

	err = db.connection.Ping()
	if err != nil {
		return err
	}
	return nil
}

func (db *MySql) Disconnect() error {
	return db.connection.Close()
}

func (db *MySql) SetConnectionString(connectionString string) {

	db.connectionString = connectionString
}

func (db *MySql) GetConnectionString() string {
	return db.connectionString
}

func (db *MySql) GetConnection() *sql.DB {
	return db.connection
}

func (db *MySql) GetDatabases() ([]string, error) {
	var databases []string

	rows, err := db.connection.Query("SHOW DATABASES")
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

	rows, err := db.connection.Query("SHOW TABLES FROM " + database)

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
	rows, _ := db.connection.Query("DESCRIBE " + table)
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
	rows, _ := db.connection.Query("SELECT COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_COLUMN_NAME, REFERENCED_TABLE_NAME FROM information_schema.KEY_COLUMN_USAGE where TABLE_NAME = " + "'" + table + "'")
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
	rows, _ := db.connection.Query("SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE where REFERENCED_TABLE_NAME = " + "'" + table + "'")
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
	rows, _ := db.connection.Query("SHOW INDEX FROM " + table)
	defer rows.Close()

	results = append(results, []string{"Table", "Non_unique", "Key_name", "Seq_in_index", "Column_name", "Collation", "Cardinality", "Sub_part", "Packed", "Null", "Index_type", "Comment"})

	for rows.Next() {
		var tableName, nonUnique, keyName, seqInIndex, columnName, collation, cardinality, subPart, packed, null, indexType, comment string
		rows.Scan(&tableName, &nonUnique, &keyName, &seqInIndex, &columnName, &collation, &cardinality, &subPart, &packed, &null, &indexType, &comment)

		results = append(results, []string{tableName, nonUnique, keyName, seqInIndex, columnName, collation, cardinality, subPart, packed, null, indexType, comment})

	}

	return
}

func (db *MySql) GetTableData(table string, offset int, limit int, appendColumns bool) (results [][]string) {
	defaultLimit := 100

	if limit != 0 {
		defaultLimit = limit
	}

	rows, _ := db.connection.Query("SELECT * FROM " + table + " LIMIT " + fmt.Sprint(offset) + "," + fmt.Sprint(defaultLimit))
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

//export the database
var Database MySql = MySql{}
