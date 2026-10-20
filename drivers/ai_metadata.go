package drivers

import "context"

func (db *MySQL) GetOutgoingForeignKeys(ctx context.Context, database, table string) ([][]string, error) {
	return db.queryForeignKeys(contextOrBackground(ctx), `
 SELECT CONSTRAINT_NAME AS constraint_name, COLUMN_NAME AS column_name,
 REFERENCED_TABLE_NAME AS referenced_table, REFERENCED_COLUMN_NAME AS referenced_column
 FROM information_schema.KEY_COLUMN_USAGE
 WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND REFERENCED_TABLE_SCHEMA = ?
 ORDER BY CONSTRAINT_NAME, ORDINAL_POSITION`, database, table, database)
}
