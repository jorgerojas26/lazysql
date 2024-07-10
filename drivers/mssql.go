package drivers

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/jorgerojas26/lazysql/helpers/log"
	"github.com/jorgerojas26/lazysql/models"
	_ "github.com/microsoft/go-mssqldb"
	"github.com/xo/dburl"
)

var _ Driver = &MsSql{}

type MsSql struct {
	Connection *sql.DB
	Provider   string

	// Used to store the last database used
	// todo: i think the interface should be altered, so that always the database of question is passed
	// this is more like a quick and dirty. The driver _assumes_ that the last database viewed is the one to be used
	lastDatabase string
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
	log.Debug("Getting tables", map[string]any{"driver": db.Provider, "database": database})
	db.lastDatabase = database
	rows, err := db.Connection.Query(fmt.Sprintf("USE %s; SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE = 'BASE TABLE' AND TABLE_CATALOG = @database", database), sql.Named("database", database))
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

		if _, ok := tables[database]; !ok {
			tables[database] = make([]string, 0)
		}

		tables[database] = append(tables[database], table)
	}

	return tables, nil
}

func (db *MsSql) GetTableColumns(database, table string) (results [][]string, err error) {
	log.Debug("Getting columns", map[string]any{"driver": db.Provider, "table": table, "database": database})

	// todo: the database should be passed as a parameter
	if strings.Contains(table, ".") {
		splitted := strings.Split(table, ".")
		database = splitted[0]
		table = splitted[1]
	}

	query := ""

	if database != "" {
		db.lastDatabase = database
		query = fmt.Sprintf("USE %s;\n", database)
	}

	query += `
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
		log.Error("failed to get columns", map[string]any{"driver": db.Provider, "table": table, "database": database, "error": err})
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
	log.Debug("Getting constraints", map[string]any{"driver": db.Provider, "table": table})

	// todo: the database should be passed as a parameter
	var database string
	if strings.Contains(table, ".") {
		splitted := strings.Split(table, ".")
		database = splitted[0]
		table = splitted[1]
	}

	query := ""

	if database != "" {
		db.lastDatabase = database
		query = fmt.Sprintf("USE %s;\n", database)
	}

	query += `
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
		log.Error("failed to get constraints", map[string]any{"driver": db.Provider, "table": table, "error": err})
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
	log.Debug("Getting foreign keys", map[string]any{"driver": db.Provider, "table": table})

	// todo: the database should be passed as a parameter
	var database string
	if strings.Contains(table, ".") {
		splitted := strings.Split(table, ".")
		database = splitted[0]
		table = splitted[1]
	}

	query := ""

	if database != "" {
		db.lastDatabase = database
		query = fmt.Sprintf("USE %s;\n", database)
	}

	query += `
		SELECT 
			FK.name, 
			C.name, 
			T.name 
		FROM sys.foreign_keys FK 
		JOIN sys.tables T 
		ON FK.parent_object_id = T.object_id 
		JOIN sys.columns C 
		ON FK.parent_object_id = C.object_id 
		WHERE T.name = @table;`

	rows, err := db.Connection.Query(query, sql.Named("table", table))
	if err != nil {
		log.Error("failed to get foreign keys", map[string]any{"driver": db.Provider, "table": table, "error": err})
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
	log.Debug("Getting indexes", map[string]any{"driver": db.Provider, "table": table})

	// todo: the database should be passed as a parameter
	var database string
	if strings.Contains(table, ".") {
		splitted := strings.Split(table, ".")
		database = splitted[0]
		table = splitted[1]
	}

	query := ""

	if database != "" {
		db.lastDatabase = database
		query = fmt.Sprintf("USE %s;\n", database)
	}

	query += "SELECT name, type_desc FROM sys.indexes WHERE object_id = OBJECT_ID(@table);"

	rows, err := db.Connection.Query(query, sql.Named("table", table))
	if err != nil {
		log.Error("failed to get indexes", map[string]any{"driver": db.Provider, "table": table, "error": err})
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
	log.Debug("Getting records", map[string]any{"driver": db.Provider, "table": table, "where": where, "sort": sort, "offset": offset, "limit": limit})
	defaultLimit := 300

	isPaginationEnabled := offset >= 0 && limit >= 0

	// todo: the database should be passed as a parameter
	var database string
	if strings.Contains(table, ".") {
		splitted := strings.Split(table, ".")
		database = splitted[0]
		table = splitted[1]
	}

	query := ""

	if database != "" {
		db.lastDatabase = database
		query = fmt.Sprintf("USE %s;\n", database)
	}

	query += fmt.Sprintf("SELECT * FROM %s", table)
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
		log.Error("failed to get records", map[string]any{"driver": db.Provider, "table": table, "where": where, "sort": sort, "offset": offset, "limit": limit, "error": err, "sql": query})
		return paginatedResults, totalRecords, err
	}
	defer paginatedRows.Close()

	if isPaginationEnabled {
		queryWithoutLimit := fmt.Sprintf("USE %s; SELECT COUNT(*) FROM %s", database, table)
		if where != "" {
			queryWithoutLimit += fmt.Sprintf(" WHERE %s", where)
		}

		rows := db.Connection.QueryRow(queryWithoutLimit)
		if err != nil {
			log.Error("failed to get total records", map[string]any{"driver": db.Provider, "table": table, "where": where, "sort": sort, "offset": offset, "limit": limit, "error": err, "sql": queryWithoutLimit})
			return paginatedResults, totalRecords, err
		}

		if err := rows.Scan(&totalRecords); err != nil {
			log.Error("failed to scan total records", map[string]any{"driver": db.Provider, "table": table, "where": where, "sort": sort, "offset": offset, "limit": limit, "error": err})
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
	log.Debug("Executing query", map[string]any{"driver": db.Provider, "query": query})

	query = fmt.Sprintf("USE %s; %s", db.lastDatabase, query)
	rows, err := db.Connection.Query(query)
	if err != nil {
		log.Error("failed to execute query", map[string]any{"driver": db.Provider, "query": query, "error": err})
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
	query := fmt.Sprintf("USE %s; UPDATE %s SET %s = @value WHERE %s = @primaryKeyValue", db.lastDatabase, table, column, primaryKeyColumnName)
	_, err := db.Connection.Exec(query, sql.Named("value", value), sql.Named("primaryKeyValue", primaryKeyValue))
	if err != nil {
		return err
	}

	return nil
}

func (db *MsSql) DeleteRecord(table, primaryKeyColumnName, primaryKeyValue string) error {
	var database string
	if strings.Contains(table, ".") {
		splitted := strings.Split(table, ".")
		database = splitted[0]
		table = splitted[1]
	}
	if database != "" {
		db.lastDatabase = database
	}

	query := fmt.Sprintf("USE %s; DELETE FROM %s WHERE %s = @primaryKeyValue", db.lastDatabase, table, primaryKeyColumnName)
	_, err := db.Connection.Exec(query, sql.Named("primaryKeyValue", primaryKeyValue))
	if err != nil {
		return err
	}

	return nil
}

func (db *MsSql) ExecuteDMLStatement(query string) (string, error) {
	log.Debug("Executing DML statement", map[string]any{"driver": db.Provider, "query": query})
	query = fmt.Sprintf("USE %s; %s", db.lastDatabase, query)
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
	return withTransaction(db.Connection, func(tx *sql.Tx) error {
		// Group changes
		groupedUpdated := make(map[string][]models.DbDmlChange)
		groupedDeletes := make([]models.DbDmlChange, 0, len(changes))
		for _, change := range changes {
			if change.Type == "UPDATE" {
				key := fmt.Sprintf("%s|%s|%s", change.Table, change.PrimaryKeyColumnName, change.PrimaryKeyValue)
				groupedUpdated[key] = append(groupedUpdated[key], change)
			} else if change.Type == "DELETE" {
				groupedDeletes = append(groupedDeletes, change)
			}
		}

		// Execute updates
		for key, changes := range groupedUpdated {
			columns := []string{}

			// Split key into table and rowId
			splitted := strings.Split(key, "|")
			table := splitted[0]
			primaryKeyColumnName := splitted[1]
			primaryKeyValue := splitted[2]

			for _, change := range changes {
				columns = append(columns, fmt.Sprintf("%s='%s'", change.Column, change.Value))
			}
			updateClause := strings.Join(columns, ", ")

			query := fmt.Sprintf("USE %s; UPDATE %s SET %s WHERE %s = '%s';", db.lastDatabase, table, updateClause, primaryKeyColumnName, primaryKeyValue)
			_, err := tx.Exec(query)
			if err != nil {
				return err
			}
		}

		// Execute deletes
		for _, delete := range groupedDeletes {
			query := fmt.Sprintf("USE %s; DELETE FROM %s WHERE %s = '%s';", db.lastDatabase, delete.Table, delete.PrimaryKeyColumnName, delete.PrimaryKeyValue)
			_, err := tx.Exec(query)
			if err != nil {
				return err
			}
		}

		// Execute inserts
		for _, insert := range inserts {
			values := make([]string, 0, len(insert.Values))
			for _, value := range insert.Values {
				_, err := strconv.ParseFloat(value, 64)
				if err != nil {
					values = append(values, fmt.Sprintf("'%s'", value))
				} else {
					values = append(values, value)
				}
			}

			query := fmt.Sprintf("USE %s; INSERT INTO %s (%s) VALUES (%s);", db.lastDatabase, insert.Table, strings.Join(insert.Columns, ", "), strings.Join(values, ", "))
			_, err := tx.Exec(query)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (db *MsSql) SetProvider(provider string) {
	db.Provider = provider
}

func (db *MsSql) GetProvider() string {
	return db.Provider
}
