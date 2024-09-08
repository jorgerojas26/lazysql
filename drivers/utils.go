package drivers

import (
	"database/sql"
	"errors"

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
