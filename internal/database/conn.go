// Package database stores the app's local state in a SQLite file.
//
// Only data the V2EX API does not provide lives here: the user's favorite
// nodes. Everything fetched from the API stays in internal/api.
package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// schema is applied on every Open. The statements are idempotent, so
// upgrading simply adds whatever is missing.
const schema = `
CREATE TABLE IF NOT EXISTS favorites (
	node_name  TEXT PRIMARY KEY,
	node_title TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS favorites_created_at ON favorites (created_at DESC);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL DEFAULT ''
);
`

var (
	// conn is the shared handle returned by Init, nil until it succeeds.
	conn *DB

	connOnce sync.Once
	connErr  error
)

// Init opens the shared database file exactly once and returns it. Later
// calls return the same handle and the same error, so the SQLite file is
// opened a single time per process even if several callers race.
//
// A failure leaves conn nil; callers decide whether persistence is
// optional.
func Init() (*DB, error) {
	connOnce.Do(func() {
		conn, connErr = Open(DefaultPath())
	})
	return conn, connErr
}

// DB is the app's SQLite database handle.
type DB struct {
	db *sql.DB
}

// DefaultPath returns mygo.db under the user's config directory, or a file
// in the working directory when no config directory is available.
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "mygo.db"
	}
	return filepath.Join(dir, "mygo_demo", "mygo.db")
}

// Open opens the SQLite database at path, creating the file, its directory
// and the schema when needed.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// SQLite allows one writer at a time; a single connection keeps the
	// database/sql pool from turning concurrent access into SQLITE_BUSY.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &DB{db: db}, nil
}

// SQL exposes the underlying handle so repos can run their own queries.
func (c *DB) SQL() *sql.DB {
	return c.db
}

// Close closes the underlying database file.
func (c *DB) Close() error {
	return c.db.Close()
}
