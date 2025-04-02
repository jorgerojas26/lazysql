//go:build !disable_sqlite_import

package drivers

// import sqlite driver
import (
	_ "modernc.org/sqlite"
)
