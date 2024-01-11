package drivers

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jorgerojas26/lazysql/models"

	_ "github.com/go-sql-driver/mysql"
	"github.com/xo/dburl"
)

type MySQL struct {
	conn              *sql.DB
	connectionString  string
	lastExecutedQuery string
}

func (db *MySQL) TestConnection() error {
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

func (db *MySQL) Connect() error {
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

func (db *MySQL) SetConnectionString(connectionString string) {
	db.connectionString = connectionString
}

func (db *MySQL) GetConnectionString() string {
	return db.connectionString
}

func (db *MySQL) GetDatabases() ([]string, error) {
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

func (db *MySQL) GetTables(database string) ([]string, error) {
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

func (db *MySQL) DescribeTable(table string) (results [][]string) {
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

func (db *MySQL) GetTableConstraints(table string) (results [][]string) {
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

func (db *MySQL) GetTableForeignKeys(table string) (results [][]string) {
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

func (db *MySQL) GetTableIndexes(table string) (results [][]string) {
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

func (db *MySQL) GetRecords(table string, where string, sort string, offset int, limit int, appendColumns bool) (results [][]string, err error) {
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
func (db *MySQL) GetPaginatedRecords(table string, where string, sort string, offset int, limit int, appendColumns bool) (paginatedResults [][]string, totalRecords int, err error) {
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

func (db *MySQL) QueryPaginatedRecords(query string) (results [][]string, err error) {
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

func (db *MySQL) UpdateRecord(table string, column string, value string, id string) error {
	query := fmt.Sprintf("UPDATE %s SET %s = \"%s\" WHERE id = \"%s\"", table, column, value, id)
	_, err := db.conn.Exec(query)

	return err
}

func (db *MySQL) DeleteRecord(table string, id string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = \"%s\"", table, id)
	_, err := db.conn.Exec(query)

	return err
}

func (db *MySQL) ExecuteDMLQuery(query string) (result string, err error) {
	res, error := db.conn.Exec(query)

	if error != nil {
		return result, error
	} else {
		rowsAffected, _ := res.RowsAffected()

		return fmt.Sprintf("%d rows affected", rowsAffected), error
	}
}

func (db *MySQL) GetLastExecutedQuery() string {
	return db.lastExecutedQuery
}

func (db *MySQL) ExecutePendingChanges(changes []models.DbDmlChange, inserts []models.DbInsert) (err error) {
	queries := make([]string, 0, len(changes)+len(inserts))

	// This will hold grouped changes by their RowId and Table
	groupedUpdated := make(map[string][]models.DbDmlChange)
	groupedDeletes := make([]models.DbDmlChange, 0, len(changes))

	// Group changes by RowId and Table
	for _, change := range changes {
		if change.Type == "UPDATE" {
			key := fmt.Sprintf("%s|%s", change.Table, change.RowID)
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
		rowID := splitted[1]

		for _, change := range changes {
			columns = append(columns, fmt.Sprintf("%s='%s'", change.Column, change.Value))
		}

		// Merge all column updates
		updateClause := strings.Join(columns, ", ")

		query := fmt.Sprintf("UPDATE %s SET %s WHERE id = '%s';", table, updateClause, rowID)

		queries = append(queries, query)
	}

	for _, delete := range groupedDeletes {
		statementType := ""
		query := ""

		statementType = "DELETE FROM"
		query = fmt.Sprintf("%s %s WHERE id = \"%s\"", statementType, delete.Table, delete.RowID)

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

	tx, error := db.conn.Begin()
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

	// fmt.Println("executing query", query)
	// _, err = tx.Exec(query)
	//
	// if err != nil {
	// 	err := tx.Rollback()
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	// err = tx.Commit()

	return err
}

func (db *MySQL) GetUpdateQuery(table string, column string, value string, whereCol string, whereVal string) string {
	return fmt.Sprintf("UPDATE %s SET %s = \"%s\" WHERE %s = \"%s\"", table, column, value, whereCol, whereVal)
}
