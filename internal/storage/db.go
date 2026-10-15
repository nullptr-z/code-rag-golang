package storage

import (
	"database/sql"
	_ "embed"
	"fmt"

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

// Publish copies a completed database into the live database in one
// transaction. Keeping the live file and connection intact lets readers see
// either the old or the new index, including long-running MCP servers.
func Publish(tempPath, path string) error {
	db, err := Open(path)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()

	if _, err := tx.Exec("ATTACH DATABASE ? AS source", tempPath); err != nil {
		return fmt.Errorf("attach temporary database: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM edges; DELETE FROM nodes;"); err != nil {
		return fmt.Errorf("clear live database: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO nodes (id, kind, name, package, file, line, signature, doc)
		SELECT id, kind, name, package, file, line, signature, doc FROM source.nodes`); err != nil {
		return fmt.Errorf("publish nodes: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO edges (id, from_id, to_id, kind, call_site_file, call_site_line)
		SELECT id, from_id, to_id, kind, call_site_file, call_site_line FROM source.edges`); err != nil {
		return fmt.Errorf("publish edges: %w", err)
	}
	if _, err := tx.Exec("DETACH DATABASE source"); err != nil {
		return fmt.Errorf("detach temporary database: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit published database: %w", err)
	}
	rollback = false
	return nil
}

// Conn returns the underlying database connection for advanced queries
func (db *DB) Conn() *sql.DB {
	return db.conn
}
