// Package repo holds the app's data repositories. Each repository takes the
// SQLite handle from internal/database and owns one table.
package repo

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mygo_demo/internal/database"
)

// errEmptyNodeName is returned when a favorite operation gets no node name.
var errEmptyNodeName = errors.New("empty node name")

// Favorite is one favorited node. NodeTitle is kept so the favorites list
// renders without calling the API.
type Favorite struct {
	NodeName  string
	NodeTitle string
	CreatedAt time.Time
}

// Favorites is the repository of the user's favorite nodes.
type Favorites struct {
	db *sql.DB
}

// NewFavorites returns a favorites repository backed by conn.
func NewFavorites(conn *database.DB) *Favorites {
	return &Favorites{db: conn.SQL()}
}

// AddFavorite favorites a node. Adding one that is already favorited only
// updates its title, so the call is safe to repeat.
func (r *Favorites) AddFavorite(name, title string) error {
	if name == "" {
		return errEmptyNodeName
	}

	_, err := r.db.Exec(`INSERT INTO favorites (node_name, node_title, created_at)
		VALUES (?, ?, ?)
		ON CONFLICT(node_name) DO UPDATE SET node_title = excluded.node_title`,
		name, title, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("add favorite %q: %w", name, err)
	}
	return nil
}

// RemoveFavorite drops a favorite. Removing one that does not exist is not
// an error, so callers do not have to check first.
func (r *Favorites) RemoveFavorite(name string) error {
	if name == "" {
		return errEmptyNodeName
	}

	if _, err := r.db.Exec(`DELETE FROM favorites WHERE node_name = ?`, name); err != nil {
		return fmt.Errorf("remove favorite %q: %w", name, err)
	}
	return nil
}

// IsFavorite reports whether the node is favorited.
func (r *Favorites) IsFavorite(name string) (bool, error) {
	var exists int
	err := r.db.QueryRow(`SELECT 1 FROM favorites WHERE node_name = ?`, name).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check favorite %q: %w", name, err)
	}
	return true, nil
}

// ToggleFavorite flips the favorite state of a node and returns the new
// state.
func (r *Favorites) ToggleFavorite(name, title string) (bool, error) {
	favorited, err := r.IsFavorite(name)
	if err != nil {
		return false, err
	}
	if favorited {
		return false, r.RemoveFavorite(name)
	}
	return true, r.AddFavorite(name, title)
}

// Favorites returns every favorited node, most recently added first.
func (r *Favorites) Favorites() ([]Favorite, error) {
	rows, err := r.db.Query(`SELECT node_name, node_title, created_at
		FROM favorites ORDER BY created_at DESC, node_name`)
	if err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	defer rows.Close()

	var out []Favorite
	for rows.Next() {
		var (
			name    string
			title   string
			created int64
		)
		if err := rows.Scan(&name, &title, &created); err != nil {
			return nil, fmt.Errorf("scan favorite: %w", err)
		}
		out = append(out, Favorite{
			NodeName:  name,
			NodeTitle: title,
			CreatedAt: time.Unix(created, 0),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list favorites: %w", err)
	}
	return out, nil
}
