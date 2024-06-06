package drivers

import (
	"database/sql"
	"fmt"

	"github.com/jorgerojas26/lazysql/models"
	_ "github.com/microsoft/go-mssqldb"
	"github.com/xo/dburl"
)

var _ Driver = &MsSql{}

type MsSql struct {
	Connection *sql.DB
	Provider   string
}

func (db *MsSql) TestConnection(urlstr string) (err error) {
	return db.Connect(urlstr)
}

func (db *MsSql) Connect(urlstr string) (err error) {
	db.SetProvider("mssql")

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

func (db *MsSql) GetDatabases() ([]string, error) {
	var databases []string

	rows, err := db.Connection.Query("SELECT name FROM sys.databases")
	if err != nil {
		return databases, err
	}
	defer rows.Close()

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

func (db *MsSql) GetTables(database string) (map[string][]string, error) {
	rows, err := db.Connection.Query("SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE = 'BASE TABLE' AND TABLE_CATALOG = @database", sql.Named("database", database))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make(map[string][]string)
	for rows.Next() {
		var table string
		err := rows.Scan(&table)
		if err != nil {
			return nil, err
		}

		tables[table] = []string{}
	}

	return tables, nil
}

func (db *MsSql) GetTableColumns(database, table string) (results [][]string, err error) {
	query := `
		SELECT 
			COLUMN_NAME, 
			DATA_TYPE, 
			IS_NULLABLE, 
			COLUMN_DEFAULT, 
			CHARACTER_MAXIMUM_LENGTH, 
			NUMERIC_PRECISION, 
			NUMERIC_SCALE 
		FROM 
			INFORMATION_SCHEMA.COLUMNS 
		WHERE 
			TABLE_NAME = @table 
			AND TABLE_CATALOG = @database 
			AND TABLE_SCHEMA = 'dbo';
	`

	rows, err := db.Connection.Query(query, sql.Named("table", table), sql.Named("database", database))
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

		if err := rows.Scan(rowValues...); err != nil {
			return results, err
		}

		var row []string
		for _, col := range rowValues {
			if col == nil {
				row = append(row, "NULL")
			} else {
				row = append(row, string(*col.(*sql.RawBytes)))
			}
		}

		results = append(results, row)
	}

	return
}

func (db *MsSql) GetConstraints(table string) ([][]string, error) {
	query := `
	SELECT 
		tc.CONSTRAINT_NAME, 
		tc.CONSTRAINT_TYPE, 
		ccu.COLUMN_NAME
	FROM 
		INFORMATION_SCHEMA.TABLE_CONSTRAINTS AS tc
	JOIN 
		INFORMATION_SCHEMA.CONSTRAINT_COLUMN_USAGE AS ccu
	ON 
		tc.CONSTRAINT_NAME = ccu.CONSTRAINT_NAME
	WHERE 
		tc.TABLE_NAME = @table 
		AND tc.TABLE_SCHEMA = 'dbo';
`
	rows, err := db.Connection.Query(query, sql.Named("table", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var constraints [][]string
	for rows.Next() {
		var constraintName, columnName, constraintType string
		err := rows.Scan(&constraintName, &columnName, &constraintType)
		if err != nil {
			return nil, err
		}

		constraints = append(constraints, []string{constraintName, columnName, constraintType})
	}

	return constraints, nil
}

func (db *MsSql) GetForeignKeys(table string) ([][]string, error) {
	rows, err := db.Connection.Query("SELECT FK.name, C.name, T.name FROM sys.foreign_keys FK JOIN sys.tables T ON FK.parent_object_id = T.object_id JOIN sys.columns C ON FK.parent_object_id = C.object_id WHERE T.name = @table", sql.Named("table", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var foreignKeys [][]string
	for rows.Next() {
		var fkName, columnName, refTableName string
		err := rows.Scan(&fkName, &columnName, &refTableName)
		if err != nil {
			return nil, err
		}

		foreignKeys = append(foreignKeys, []string{fkName, columnName, refTableName})
	}

	return foreignKeys, nil
}

func (db *MsSql) GetIndexes(table string) ([][]string, error) {
	rows, err := db.Connection.Query("SELECT name, type_desc FROM sys.indexes WHERE object_id = OBJECT_ID(@table)", sql.Named("table", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes [][]string
	for rows.Next() {
		var indexName, indexType string
		err := rows.Scan(&indexName, &indexType)
		if err != nil {
			return nil, err
		}

		indexes = append(indexes, []string{indexName, indexType})
	}

	return indexes, nil
}

func (db *MsSql) GetRecords(table, where, sort string, offset, limit int) (paginatedResults [][]string, totalRecords int, err error) {
	defaultLimit := 300

	isPaginationEnabled := offset >= 0 && limit >= 0

	query := fmt.Sprintf("SELECT * FROM %s", table)
	if where != "" {
		query += fmt.Sprintf(" WHERE %s", where)
	}
	if sort != "" {
		query += fmt.Sprintf(" ORDER BY %s", sort)
	} else if isPaginationEnabled {
		// OFFSET / FETCH NEXT requires an ORDER BY clause
		query += " ORDER BY (SELECT NULL)"
	}

	if isPaginationEnabled {
		if limit == 0 {
			limit = defaultLimit
		}
		query += fmt.Sprintf(" OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, limit)
	}

	paginatedRows, err := db.Connection.Query(query)
	if err != nil {
		return paginatedResults, totalRecords, err
	}
	defer paginatedRows.Close()

	if isPaginationEnabled {
		queryWithoutLimit := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
		if where != "" {
			queryWithoutLimit += fmt.Sprintf(" WHERE %s", where)
		}

		rows := db.Connection.QueryRow(queryWithoutLimit)
		if err != nil {
			return paginatedResults, totalRecords, err
		}

		if err := rows.Scan(&totalRecords); err != nil {
			return paginatedResults, totalRecords, err
		}
	}

	columns, err := paginatedRows.Columns()
	if err != nil {
		return paginatedResults, totalRecords, err
	}

	paginatedResults = append(paginatedResults, columns)

	for paginatedRows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		if err := paginatedRows.Scan(rowValues...); err != nil {
			return paginatedResults, totalRecords, err
		}

		var row []string
		for _, col := range rowValues {
			if col == nil {
				row = append(row, "NULL")
			} else {
				row = append(row, string(*col.(*sql.RawBytes)))
			}
		}

		paginatedResults = append(paginatedResults, row)
	}

	return
}

func (db *MsSql) ExecuteQuery(query string) ([][]string, error) {
	rows, err := db.Connection.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results [][]string
	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		rows.Scan(rowValues...)

		var row []string
		for _, col := range rowValues {
			if col == nil {
				row = append(row, "NULL")
			} else {
				row = append(row, string(*col.(*sql.RawBytes)))
			}
		}

		results = append(results, row)
	}

	return results, nil
}

func (db *MsSql) UpdateRecord(table, column, value, primaryKeyColumnName, primaryKeyValue string) error {
	query := fmt.Sprintf("UPDATE %s SET %s = @value WHERE %s = @primaryKeyValue", table, column, primaryKeyColumnName)
	_, err := db.Connection.Exec(query, sql.Named("value", value), sql.Named("primaryKeyValue", primaryKeyValue))
	if err != nil {
		return err
	}

	return nil
}

func (db *MsSql) DeleteRecord(table, primaryKeyColumnName, primaryKeyValue string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = @primaryKeyValue", table, primaryKeyColumnName)
	_, err := db.Connection.Exec(query, sql.Named("primaryKeyValue", primaryKeyValue))
	if err != nil {
		return err
	}

	return nil
}

func (db *MsSql) ExecuteDMLStatement(query string) (string, error) {
	res, err := db.Connection.Exec(query)
	if err != nil {
		return "", err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%d rows affected", rowsAffected), nil
}

func (db *MsSql) ExecutePendingChanges(changes []models.DbDmlChange, inserts []models.DbInsert) error {
	// TODO: Implement this method
	return nil
}

func (db *MsSql) SetProvider(provider string) {
	db.Provider = provider
}

func (db *MsSql) GetProvider() string {
	return db.Provider
}
