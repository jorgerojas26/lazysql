package drivers

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	// mssql driver
	_ "github.com/microsoft/go-mssqldb"
	"github.com/xo/dburl"

	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
)

type MSSQL struct {
	Connection *sql.DB
	Provider   string
}

func (db *MSSQL) TestConnection(urlstr string) error {
	return db.Connect(urlstr)
}

func (db *MSSQL) Connect(urlstr string) error {
	if urlstr == "" {
		return errors.New("url string can not be empty")
	}

	db.SetProvider(DriverMSSQL)

	var err error

	db.Connection, err = dburl.Open(urlstr)
	if err != nil {
		return err
	}

	if err := db.Connection.Ping(); err != nil {
		return err
	}

	return nil
}

func (db *MSSQL) GetDatabases() ([]string, error) {
	databases := make([]string, 0)

	query := `
		SELECT
			name
		FROM
			sys.databases
	`
	rows, err := db.Connection.Query(query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			return nil, err
		}

		databases = append(databases, database)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return databases, nil
}

func (db *MSSQL) GetTables(database string) (map[string][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	tables := make(map[string][]string)

	query := `
		SELECT
			name
		FROM
			sys.tables
	`
	rows, err := db.Connection.Query(query)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}

		tables[database] = append(tables[database], table)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tables, nil
}

func (db *MSSQL) GetTableColumns(database, table string) ([][]string, error) {
	query := `
		SELECT
			column_name, data_type, is_nullable, column_default
		FROM
			information_schema.columns
		WHERE
			table_catalog = @p1
		AND
			table_name = @p2
		ORDER BY
			ordinal_position
	`
	return db.getTableInformations(query, database, table)
}

func (db *MSSQL) GetConstraints(database, table string) ([][]string, error) {
	query := `
		SELECT
				tc.constraint_name,
				kcu.column_name,
				tc.constraint_type
		FROM
				information_schema.table_constraints AS tc
		JOIN
				information_schema.key_column_usage AS kcu
					ON
							tc.constraint_name = kcu.constraint_name
					AND
							tc.table_schema = kcu.table_schema
		JOIN
				information_schema.constraint_column_usage AS ccu
					ON
							ccu.constraint_name = tc.constraint_name
					AND
							ccu.table_schema = tc.table_schema
		WHERE NOT
				tc.constraint_type = 'FOREIGN KEY'
		AND
				tc.table_catalog = @p1
		AND
				tc.table_name = @p2
	`
	return db.getTableInformations(query, database, table)
}

func (db *MSSQL) GetForeignKeys(database, table string) ([][]string, error) {
	query := `
		SELECT
				tc.constraint_name,
				kcu.column_name,
				tc.constraint_type
		FROM
				information_schema.table_constraints AS tc
		JOIN
				information_schema.key_column_usage AS kcu
					ON
							tc.constraint_name = kcu.constraint_name
					AND
							tc.table_schema = kcu.table_schema
		JOIN
				information_schema.constraint_column_usage AS ccu
					ON
							ccu.constraint_name = tc.constraint_name
					AND
							ccu.table_schema = tc.table_schema
		WHERE
				tc.constraint_type = 'FOREIGN KEY'
		AND
				tc.table_catalog = @p1
		AND
				tc.table_name = @p2
	`
	return db.getTableInformations(query, database, table)
}

func (db *MSSQL) GetIndexes(database, table string) ([][]string, error) {
	query := `
		SELECT
				t.name AS table_name,
				i.name AS index_name,
				i.is_unique AS is_unique,
				i.is_primary_key AS is_primary_key,
				i.type_desc AS index_type,
				c.name AS column_name,
				ic.key_ordinal AS seq_in_index,
				ic.is_included_column AS is_included,
				i.has_filter AS has_filter,
				i.filter_definition AS filter_definition
		FROM
				sys.tables t
		INNER JOIN
				sys.schemas s
					ON
						t.schema_id = s.schema_id
		INNER JOIN
				sys.indexes i
					ON
						t.object_id = i.object_id
		INNER JOIN
				sys.index_columns ic
					ON
						i.object_id = ic.object_id
					AND
						i.index_id = ic.index_id
		INNER JOIN
				sys.columns c
					ON
						t.object_id = c.object_id
					AND
						ic.column_id = c.column_id
		WHERE
				DB_NAME() = @p1
		AND
				t.name = @p2
		ORDER BY
				i.type_desc
	`
	return db.getTableInformations(query, database, table)
}

func (db *MSSQL) GetRecords(database, table, where, sort string, offset, limit int) ([][]string, int, error) {
    if database == "" {
        return nil, 0, errors.New("database name is required")
    }

    if table == "" {
        return nil, 0, errors.New("table name is required")
    }

    if limit == 0 {
        limit = DefaultRowLimit
    }

    results := make([][]string, 0)

    query := "SELECT * FROM "
    query += fmt.Sprintf("[%s]", table)

    if where != "" {
        query += fmt.Sprintf(" %s", where)
    }

    // Since in MSSQL, ORDER BY is mandatory when using pagination
    if sort == "" {
        sort = "(SELECT NULL)"
    }
    query += fmt.Sprintf(" ORDER BY %s OFFSET @p1 ROWS FETCH NEXT @p2 ROWS ONLY", sort)

    rows, err := db.Connection.Query(query, offset, limit)
    if err != nil {
        return nil, 0, err
    }

    defer rows.Close()

    columns, err := rows.Columns()
    if err != nil {
        return nil, 0, err
    }

    results = append(results, columns)

    for rows.Next() {
        rowValues := make([]any, len(columns))

        for i := range columns {
            rowValues[i] = new(sql.RawBytes)
        }

        if err := rows.Scan(rowValues...); err != nil {
            return nil, 0, err
        }

        var row []string
        for _, col := range rowValues {
            if col == nil {
                row = append(row, "NULL&")
                continue
            }

            colval := string(*col.(*sql.RawBytes))
            if colval == "" {
                row = append(row, "EMPTY&")
            } else {
                row = append(row, colval)
            }
        }

        results = append(results, row)
    }

    if err := rows.Err(); err != nil {
        return nil, 0, err
    }

    totalRecords := 0
    row := db.Connection.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM [%s]", table))
    if err := row.Scan(&totalRecords); err != nil {
        return nil, 0, err
    }

    return results, totalRecords, nil
}

func (db *MSSQL) UpdateRecord(database, table, column, value, primaryKeyColumnName, primaryKeyValue string) error {
	if database == "" {
		return errors.New("database name is required")
	}

	if table == "" {
		return errors.New("table name is required")
	}

	if column == "" {
		return errors.New("table column is required")
	}

	if primaryKeyColumnName == "" {
		return errors.New("primary key column is required")
	}

	if primaryKeyValue == "" {
		return errors.New("primary key value is required")
	}

	query := "UPDATE "
	query += table
	query += " SET "
	query += column
	query += " = @p1 WHERE "
	query += primaryKeyColumnName
	query += " = @p2"
	_, err := db.Connection.Exec(query, value, primaryKeyValue)

	return err
}

func (db *MSSQL) DeleteRecord(database, table, primaryKeyColumnName, primaryKeyValue string) error {
	if database == "" {
		return errors.New("database name is required")
	}

	if table == "" {
		return errors.New("table name is required")
	}

	if primaryKeyColumnName == "" {
		return errors.New("primary key column is required")
	}

	if primaryKeyValue == "" {
		return errors.New("primary key value is required")
	}

	query := "DELETE FROM "
	query += table
	query += " WHERE "
	query += primaryKeyColumnName
	query += " = @p1"
	_, err := db.Connection.Exec(query, primaryKeyValue)

	return err
}

func (db *MSSQL) ExecuteDMLStatement(query string) (string, error) {
	if query == "" {
		return "", errors.New("query is required")
	}

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

func (db *MSSQL) ExecuteQuery(query string) ([][]string, int, error) {
	if query == "" {
		return nil, 0, errors.New("query can not be empty")
	}

	rows, err := db.Connection.Query(query)
	if err != nil {
		return nil, 0, err
	}

	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, 0, err
	}

	records := make([][]string, 0)
	for rows.Next() {
		rowValues := make([]any, len(columns))

		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		if err := rows.Scan(rowValues...); err != nil {
			return nil, 0, err
		}

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		records = append(records, row)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Prepend the columns to the records.
	results := append([][]string{columns}, records...)

	return results, len(records), nil
}

func (db *MSSQL) ExecutePendingChanges(changes []models.DBDMLChange) error {
	if len(changes) <= 0 {
		return nil
	}

	queries := make([]models.Query, 0)
	errlist := make([]error, 0)

	for _, change := range changes {
		if change.Table == "" {
			errlist = append(errlist, errors.New("table name is required"))
			continue
		}

		columnNames := make([]string, 0)
		valuesPlaceholder := make([]string, 0)
		values := make([]any, 0)

		for i, cell := range change.Values {
			columnNames = append(columnNames, cell.Column)

			switch cell.Type {
			case models.Default:
				valuesPlaceholder = append(valuesPlaceholder, "DEFAULT")
			case models.Null:
				valuesPlaceholder = append(valuesPlaceholder, "NULL")
			default:
				valuesPlaceholder = append(valuesPlaceholder, fmt.Sprintf("@p%d", i+1))
			}
		}

		for _, cell := range change.Values {
			switch cell.Type {
			case models.Empty:
				values = append(values, "")
			case models.String:
				values = append(values, cell.Value)
			}
		}

		switch change.Type {
		case models.DMLInsertType:
			queryStr := fmt.Sprintf(
				"INSERT INTO %s (%s) VALUES (%s)",
				change.Table,
				strings.Join(columnNames, ", "),
				strings.Join(valuesPlaceholder, ", "),
			)

			newQuery := models.Query{
				Query: queryStr,
				Args:  values,
			}

			queries = append(queries, newQuery)
		case models.DMLUpdateType:
			queryStr := fmt.Sprintf("UPDATE %s", change.Table)

			for i, column := range columnNames {
				if i == 0 {
					queryStr += fmt.Sprintf(" SET %s = %s", column, valuesPlaceholder[i])
				} else {
					queryStr += fmt.Sprintf(", %s = %s", column, valuesPlaceholder[i])
				}
			}

			args := make([]any, len(values))

			copy(args, values)

			// start counting from valuesPlaceholder
			// then add 1 by 1 on loop
			updateCounterParams := len(valuesPlaceholder)
			for i, pki := range change.PrimaryKeyInfo {
				updateCounterParams++
				if i == 0 {
					queryStr += fmt.Sprintf(" WHERE %s = @p%d", pki.Name, updateCounterParams)
				} else {
					queryStr += fmt.Sprintf(" AND %s = @p%d", pki.Name, updateCounterParams)
				}
				args = append(args, pki.Value)
			}

			newQuery := models.Query{
				Query: queryStr,
				Args:  args,
			}

			// EZ way to log
			// _ = os.WriteFile("/tmp/lazysql", []byte(queryStr+"\n"), 0644)
			queries = append(queries, newQuery)
		case models.DMLDeleteType:
			queryStr := fmt.Sprintf("DELETE FROM %s", change.Table)

			deleteArgs := make([]any, len(change.PrimaryKeyInfo))

			for i, pki := range change.PrimaryKeyInfo {
				if i == 0 {
					queryStr += fmt.Sprintf(" WHERE %s = @p%d", pki.Name, i+1)
				} else {
					queryStr += fmt.Sprintf(" AND %s = @p%d", pki.Name, i+1)
				}
				deleteArgs[i] = pki.Value
			}

			newQuery := models.Query{
				Query: queryStr,
				Args:  deleteArgs,
			}

			queries = append(queries, newQuery)
		}
	}

	// log loop errlist
	if len(errlist) > 0 {
		errmap := make(map[string]any)
		for i, e := range errlist {
			errmap[fmt.Sprintf("%d:", i+1)] = e
		}
		logger.Error("ExecutePendingChanges", errmap)
	}

	return queriesInTransaction(db.Connection, queries)
}

func (db *MSSQL) GetPrimaryKeyColumnNames(database, table string) ([]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	pkColumnName := make([]string, 0)
	query := `
		SELECT
				c.name AS column_name
		FROM
				sys.tables t
		INNER JOIN
			sys.schemas s
				ON
					t.schema_id = s.schema_id
		INNER JOIN
			sys.key_constraints kc
				ON
					t.object_id = kc.parent_object_id
				AND
					kc.type = @p1
		INNER JOIN
			sys.index_columns ic
				ON
					kc.unique_index_id = ic.index_id
				AND
					t.object_id = ic.object_id
		INNER JOIN
			sys.columns c
				ON
					ic.column_id = c.column_id
				AND
					t.object_id = c.object_id
		WHERE
				DB_NAME() = @p2
		AND
				t.name = @p3
	`
	rows, err := db.Connection.Query(query, "PK", database, table)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var colName string
		err = rows.Scan(&colName)
		if err != nil {
			return nil, err
		}

		if rows.Err() != nil {
			return nil, rows.Err()
		}

		pkColumnName = append(pkColumnName, colName)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return pkColumnName, nil
}

func (db *MSSQL) SetProvider(provider string) {
	db.Provider = provider
}

func (db *MSSQL) GetProvider() string {
	return db.Provider
}

// getTableInformations is used for following func:
//
//   - [GetTableColumns]
//   - [GetConstraints]
//   - [GetForeignKeys]
//   - [GetIndexes]
//
// getTableInformations requires following parameter:
//
//   - database name, used for filtering table_catalog
//   - table name, used for filtering table_name
func (db *MSSQL) getTableInformations(query, database, table string) ([][]string, error) {
	if database == "" {
		return nil, errors.New("database name is required")
	}

	if table == "" {
		return nil, errors.New("table name is required")
	}

	if query == "" {
		return nil, errors.New("query can not be empty")
	}

	results := make([][]string, 0)
	rows, err := db.Connection.Query(query, database, table)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results = append(results, columns)

	for rows.Next() {
		rowValues := make([]any, len(columns))

		for i := range columns {
			rowValues[i] = new(sql.RawBytes)
		}

		if err := rows.Scan(rowValues...); err != nil {
			return nil, err
		}

		var row []string
		for _, col := range rowValues {
			row = append(row, string(*col.(*sql.RawBytes)))
		}

		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}
