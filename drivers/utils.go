package drivers

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jorgerojas26/lazysql/helpers/logger"
	"github.com/jorgerojas26/lazysql/models"
)

func queriesInTransaction(db *sql.DB, queries []models.Query) (err error) {
	trx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		rErr := trx.Rollback()
		// sql.ErrTxDone is returned when trx.Commit was already called
		if !errors.Is(rErr, sql.ErrTxDone) {
			err = errors.Join(err, rErr)
		}
	}()

	for _, query := range queries {
		logger.Info(query.Query, map[string]any{"args": query.Args})
		if _, err := trx.Exec(query.Query, query.Args...); err != nil {
			return err
		}
	}
	if err := trx.Commit(); err != nil {
		return err
	}
	return nil
}

func buildInsertQueryString(formattedTableName string, columns []string, values []string) string {
	queryStr := "INSERT INTO " + formattedTableName
	queryStr += fmt.Sprintf(" (%s) VALUES (%s)", strings.Join(columns, ", "), strings.Join(values, ", "))

	return queryStr
}

func buildInsertQuery(formattedTableName string, values []models.CellValue, driver Driver) models.Query {
	cols, args := getColNamesAndArgs(values, driver)
	placeholders := buildPlaceholders(values, driver)

	queryStr := "INSERT INTO " + formattedTableName
	queryStr += fmt.Sprintf(" (%s) VALUES (%s)", strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	newQuery := models.Query{
		Query: queryStr,
		Args:  args,
	}

	return newQuery
}

func buildUpdateQueryString(sanitizedTableName string, sanitizedColNames []string, sanitizedArgs []string, primaryKeyInfo []models.PrimaryKeyInfo, driver Driver) string {
	queryStr := "UPDATE " + sanitizedTableName

	sanitizedPrimaryKeyInfo := sanitizePrimaryKeyInfo(primaryKeyInfo, driver)

	for i, sanitizedColName := range sanitizedColNames {
		if i == 0 {
			queryStr += fmt.Sprintf(" SET %s = %s", sanitizedColName, sanitizedArgs[i])
		} else {
			queryStr += fmt.Sprintf(", %s = %s", sanitizedColName, sanitizedArgs[i])
		}
	}

	for i, sanitizedPki := range sanitizedPrimaryKeyInfo {
		if i == 0 {
			queryStr += fmt.Sprintf(" WHERE %s = %s", sanitizedPki.Name, driver.FormatArg(sanitizedPki.Value))
		} else {
			queryStr += fmt.Sprintf(" AND %s = %s", sanitizedPki.Name, driver.FormatArg(sanitizedPki.Value))
		}
	}

	return queryStr
}

func buildUpdateQuery(sanitizedTableName string, values []models.CellValue, primaryKeyInfo []models.PrimaryKeyInfo, driver Driver) models.Query {
	sanitizedCols, sanitizedArgs := getColNamesAndArgs(values, driver)
	sanitizedPrimaryKeyInfo := sanitizePrimaryKeyInfo(primaryKeyInfo, driver)
	placeholders := buildPlaceholders(values, driver)

	queryStr := "UPDATE " + sanitizedTableName

	for i, sanitizedCol := range sanitizedCols {
		placeholder := placeholders[i]
		reference := sanitizedCol
		if i == 0 {
			queryStr += fmt.Sprintf(" SET %s = %s", reference, placeholder)
		} else {
			queryStr += fmt.Sprintf(", %s = %s", reference, placeholder)
		}
	}

	for i, sanitizedPki := range sanitizedPrimaryKeyInfo {
		placeholder := driver.FormatPlaceholder(len(placeholders) + i + 1)
		reference := sanitizedPki.Name

		if i == 0 {
			queryStr += fmt.Sprintf(" WHERE %s = %s", reference, placeholder)
		} else {
			queryStr += fmt.Sprintf(" AND %s = %s", reference, placeholder)
		}
		sanitizedArgs = append(sanitizedArgs, sanitizedPki.Value)
	}

	newQuery := models.Query{
		Query: queryStr,
		Args:  sanitizedArgs,
	}

	return newQuery
}

func buildDeleteQueryString(sanitizedTableName string, primaryKeyInfo []models.PrimaryKeyInfo, driver Driver) string {
	queryStr := "DELETE FROM " + sanitizedTableName

	sanitizedPrimaryKeyInfo := sanitizePrimaryKeyInfo(primaryKeyInfo, driver)

	for i, sanitizedPki := range sanitizedPrimaryKeyInfo {
		if i == 0 {
			queryStr += fmt.Sprintf(" WHERE %s = %s", sanitizedPki.Name, sanitizedPki.Value)
		} else {
			queryStr += fmt.Sprintf(" AND %s = %s", sanitizedPki.Name, sanitizedPki.Value)
		}
	}

	return queryStr
}

func buildDeleteQuery(formattedTableName string, primaryKeyInfo []models.PrimaryKeyInfo, driver Driver) models.Query {
	queryStr := "DELETE FROM " + formattedTableName
	args := make([]interface{}, len(primaryKeyInfo))

	sanitizedPrimaryKeyInfo := sanitizePrimaryKeyInfo(primaryKeyInfo, driver)

	for i, sanitizedPki := range sanitizedPrimaryKeyInfo {
		placeholder := driver.FormatPlaceholder(i + 1)
		reference := sanitizedPki.Name

		if i == 0 {
			queryStr += fmt.Sprintf(" WHERE %s = %s", reference, placeholder)
		} else {
			queryStr += fmt.Sprintf(" AND %s = %s", reference, placeholder)
		}
		args[i] = sanitizedPki.Value
	}

	return models.Query{
		Query: queryStr,
		Args:  args,
	}
}

func sanitizePrimaryKeyInfo(primaryKeyInfo []models.PrimaryKeyInfo, driver Driver) []models.PrimaryKeyInfo {
	sanitizedPrimaryKeyInfo := []models.PrimaryKeyInfo{}

	for _, pki := range primaryKeyInfo {
		sanitizedPrimaryKeyInfo = append(sanitizedPrimaryKeyInfo, models.PrimaryKeyInfo{
			Name:  driver.FormatReference(pki.Name),
			Value: pki.Value,
		})
	}

	return sanitizedPrimaryKeyInfo
}

func getColNamesAndArgsAsString(values []models.CellValue, driver Driver) ([]string, []string) {
	cols := []string{}
	v := []string{}

	for _, cell := range values {
		cols = append(cols, driver.FormatReference(cell.Column))

		switch cell.Type {
		case models.Empty:
			v = append(v, "")
		case models.String:
			v = append(v, driver.FormatArg(cell.Value))
		}
	}

	return cols, v
}

func getColNamesAndArgs(values []models.CellValue, driver Driver) ([]string, []interface{}) {
	cols := []string{}
	v := []interface{}{}

	for _, cell := range values {
		cols = append(cols, driver.FormatReference(cell.Column))

		switch cell.Type {
		case models.Empty:
			v = append(v, "")
		case models.String:
			// This must not be sanitized because it's used as the placeholder arg
			v = append(v, cell.Value)
		}
	}

	return cols, v
}

func buildPlaceholders(values []models.CellValue, driver Driver) []string {
	placeholders := []string{}

	for i, cell := range values {
		switch cell.Type {
		case models.Empty:
			placeholders = append(placeholders, "DEFAULT")
		case models.Null:
			placeholders = append(placeholders, "NULL")
		default:
			placeholders = append(placeholders, driver.FormatPlaceholder(i+1))
		}
	}
	return placeholders
}
