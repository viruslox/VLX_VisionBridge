package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

var driverName = "sqlite3"

// InitDB initializes a SQLite connection pool.
func InitDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set reasonable pool limits
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}
