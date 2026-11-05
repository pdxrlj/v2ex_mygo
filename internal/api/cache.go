package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// CacheTTL is the default freshness window for cached API responses.
// Listings like nodes/all.json change slowly, so a slightly stale copy is
// still worth serving.
const CacheTTL = 6 * time.Hour

// Cache stores API responses in a SQLite file with a per-entry TTL, so a
// large listing such as nodes/all.json is fetched once while fresh rather
// than on every call or app start. Cache errors are best-effort at the
// call sites: a broken cache never blocks a network fetch.
type Cache struct {
	db  *sql.DB
	ttl time.Duration

	now func() time.Time // overridden in tests
}

// DefaultCachePath returns cache.db under the user's cache directory, or a
// file next to the binary when no cache directory is available.
func DefaultCachePath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "cache.db"
	}
	return filepath.Join(dir, "mygo_demo", "cache.db")
}

// OpenCache opens the SQLite cache at path, creating the file, its
// directory and the table if needed.
func OpenCache(path string) (*Cache, error) {
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

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS cache_entries (
		key        TEXT PRIMARY KEY,
		body       BLOB NOT NULL,
		expires_at INTEGER NOT NULL
	)`); err != nil {
		db.Close()
		return nil, err
	}

	return &Cache{db: db, ttl: CacheTTL, now: time.Now}, nil
}

// TTL returns the freshness window Put applies to new entries.
func (c *Cache) TTL() time.Duration {
	return c.ttl
}

// SetTTL sets the freshness window Put applies to new entries. A
// non-positive d restores CacheTTL.
func (c *Cache) SetTTL(d time.Duration) {
	if d <= 0 {
		d = CacheTTL
	}
	c.ttl = d
}

// Get decodes the entry cached under key into dst. It reports false when the
// key is absent or expired, which tells the caller to refetch.
func (c *Cache) Get(key string, dst any) (bool, error) {
	var (
		body      []byte
		expiresAt int64
	)
	err := c.db.QueryRow(`SELECT body, expires_at FROM cache_entries WHERE key = ?`, key).
		Scan(&body, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if !c.now().Before(time.Unix(expiresAt, 0)) {
		return false, nil
	}

	if err := json.Unmarshal(body, dst); err != nil {
		return false, err
	}
	return true, nil
}

// Put stores src under key using the cache TTL.
func (c *Cache) Put(key string, src any) error {
	return c.PutTTL(key, src, c.ttl)
}

// PutTTL stores src under key with an explicit TTL. A non-positive ttl uses
// the cache TTL.
func (c *Cache) PutTTL(key string, src any, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = c.ttl
	}

	body, err := json.Marshal(src)
	if err != nil {
		return err
	}

	_, err = c.db.Exec(`INSERT INTO cache_entries (key, body, expires_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET body = excluded.body, expires_at = excluded.expires_at`,
		key, body, c.now().Add(ttl).Unix())
	return err
}

// Delete removes the entry for key and reports whether one existed.
func (c *Cache) Delete(key string) (bool, error) {
	res, err := c.db.Exec(`DELETE FROM cache_entries WHERE key = ?`, key)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// Clear removes every entry and returns how many were deleted.
func (c *Cache) Clear() (int64, error) {
	res, err := c.db.Exec(`DELETE FROM cache_entries`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PurgeExpired removes entries whose TTL has passed and returns how many
// were deleted. Get already ignores them; this reclaims the space.
func (c *Cache) PurgeExpired() (int64, error) {
	res, err := c.db.Exec(`DELETE FROM cache_entries WHERE expires_at <= ?`, c.now().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Close closes the underlying database file.
func (c *Cache) Close() error {
	return c.db.Close()
}
