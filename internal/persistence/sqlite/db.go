package sqlite

import (
	"database/sql"
	"fmt"
	"runtime"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// NewDB opens a sqlite database connection and runs the initial schema setup.
func NewDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	configureAndroidPool(db)

	// Enable foreign keys
	if _, err := db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Apply schema
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	return db, nil
}

// gomobile + sqlite3_dotlk cannot share WAL across pooled connections.
// Cap the pool before the first Exec so schema and later Sync share one conn.
func configureAndroidPool(db *sql.DB) {
	if runtime.GOOS != "android" {
		return
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
}
