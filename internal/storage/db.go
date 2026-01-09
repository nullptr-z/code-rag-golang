package storage

import (
	"database/sql"
	_ "embed"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// Open opens or creates a SQLite database at the given path
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Enable foreign keys
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, err
	}

	// Initialize schema
	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, err
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Clear removes all data from the database
func (db *DB) Clear() error {
	_, err := db.conn.Exec("DELETE FROM edges; DELETE FROM nodes;")
	return err
}

// Conn returns the underlying database connection for advanced queries
func (db *DB) Conn() *sql.DB {
	return db.conn
}

