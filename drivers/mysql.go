package drivers

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql" // driver
	"github.com/xo/dburl"

	"github.com/jorgerojas26/lazysql/models"
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
	rows, err := db.conn.Query("SHOW DATABASES")
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var databases []string

	for rows.Next() {
		var database string
		if scanErr := rows.Scan(&database); scanErr != nil {
			return nil, scanErr
		}

		if database != "information_schema" && database != "mysql" && database != "performance_schema" && database != "sys" {
			databases = append(databases, database)
		}
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return databases, nil
}

func (db *MySQL) GetTables(database string) ([]string, error) {
	rows, err := db.conn.Query("SHOW TABLES FROM " + database)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var tables []string

	for rows.Next() {
		var table string
		if scanErr := rows.Scan(&table); scanErr != nil {
			return nil, scanErr
		}

		tables = append(tables, table)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return tables, nil
}

// TODO: refactor
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

		_ = rows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}

	if rows.Err() != nil {
		return
	}

	return
}

func (db *MySQL) GetTableConstraints(table string) (results [][]string) {
	const query = `SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME 
			 FROM information_schema.KEY_COLUMN_USAGE 
		        WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`

	return db.getTableInfo(table, query)
}

func (db *MySQL) GetTableForeignKeys(table string) (results [][]string) {
	const query = `SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_COLUMN_NAME, REFERENCED_TABLE_NAME 
			 FROM information_schema.KEY_COLUMN_USAGE 
			WHERE REFERENCED_TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME = ?`

	return db.getTableInfo(table, query)
}

func (db *MySQL) getTableInfo(table, query string) (results [][]string) {
	splitTableString := strings.Split(table, ".")
	database := splitTableString[0]
	tableName := splitTableString[1]

	rows, _ := db.conn.Query(query, database, tableName) // TODO: handle error
	defer rows.Close()

	columns, _ := rows.Columns()

	results = append(results, columns)

	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		rows.Scan(rowValues...) // TODO: handle error

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

		rows.Scan(rowValues...) // TODO: handle error

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}

	return
}

// TODO: refactor
func (db *MySQL) GetRecords(table, where, sort string, offset, limit int, appendColumns bool) (results [][]string, err error) {
	defaultLimit := 2

	if limit != 0 {
		defaultLimit = limit
	}

	query := fmt.Sprintf("SELECT * FROM %s s LIMIT %d, %d", table, offset, defaultLimit)

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

	defer rows.Close()

	db.lastExecutedQuery = query

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

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return
}

// Get paginated records
// TODO: refactor
func (db *MySQL) GetPaginatedRecords(table, where, sort string, offset, limit int, appendColumns bool) (paginatedResults [][]string, totalRecords int, err error) {
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
		return paginatedResults, totalRecords, err
	}

	defer paginatedRows.Close()

	queryWithoutLimit := fmt.Sprintf("SELECT COUNT(*) FROM %s %s", table, where)

	if scanErr := db.conn.QueryRow(queryWithoutLimit).Scan(&totalRecords); scanErr != nil {
		return paginatedResults, totalRecords, scanErr
	}

	columns, _ := paginatedRows.Columns()

	if appendColumns {
		paginatedResults = append(paginatedResults, columns)
	}

	for paginatedRows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		if scanErr := paginatedRows.Scan(rowValues...); scanErr != nil {
			return paginatedResults, totalRecords, scanErr
		}

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		paginatedResults = append(paginatedResults, row)
	}

	if paginatedRows.Err() != nil {
		return paginatedResults, totalRecords, err
	}

	return paginatedResults, totalRecords, nil
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

		if scanErr := rows.Scan(rowValues...); scanErr != nil {
			return results, scanErr
		}

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}

	if rows.Err() != nil {
		return results, err
	}

	return results, nil
}

func (db *MySQL) UpdateRecord(table, column, value, id string) error {
	query := fmt.Sprintf("UPDATE %s SET %s = ? WHERE id = ?", table, column)
	_, err := db.conn.Exec(query, value, id)

	return err
}

func (db *MySQL) DeleteRecord(table, id string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = ?", table)
	_, err := db.conn.Exec(query, id)

	return err
}

func (db *MySQL) ExecuteDMLQuery(query string) (result string, err error) {
	res, err := db.conn.Exec(query)
	if err != nil {
		return result, err
	}

	rowsAffected, _ := res.RowsAffected()

	return fmt.Sprintf("%d rows affected", rowsAffected), nil
}

func (db *MySQL) GetLastExecutedQuery() string {
	return db.lastExecutedQuery
}

// TODO: refactor
func (db *MySQL) ExecutePendingChanges(changes []models.DbDmlChange, inserts []models.DbInsert) error {
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
		query = fmt.Sprintf("%s %s WHERE id = %q", statementType, delete.Table, delete.RowID)

		if query != "" {
			queries = append(queries, query)
		}
	}

	for _, insert := range inserts {
		values := make([]string, 0, len(insert.Values))

		for _, value := range insert.Values {
			_, err := strconv.ParseFloat(value, 64)
			if !strings.EqualFold(value, "default") && err != nil {
				values = append(values, fmt.Sprintf("%q", value))
			} else {
				values = append(values, value)
			}
		}

		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", insert.Table, strings.Join(insert.Columns, ", "), strings.Join(values, ", "))

		queries = append(queries, query)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
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
