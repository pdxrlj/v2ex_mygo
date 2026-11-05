package repo

import (
	"path/filepath"
	"testing"

	"mygo_demo/internal/database"
)

// newTestFavorites returns a repository backed by a database file in the
// test's temp dir.
func newTestFavorites(t *testing.T) *Favorites {
	t.Helper()

	conn, err := database.Open(filepath.Join(t.TempDir(), "mygo.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return NewFavorites(conn)
}

func TestAddAndIsFavorite(t *testing.T) {
	favs := newTestFavorites(t)

	ok, err := favs.IsFavorite("go")
	if err != nil {
		t.Fatalf("IsFavorite before add: %v", err)
	}
	if ok {
		t.Fatal("go is favorited before it was added")
	}

	if err := favs.AddFavorite("go", "Go"); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}

	ok, err = favs.IsFavorite("go")
	if err != nil {
		t.Fatalf("IsFavorite after add: %v", err)
	}
	if !ok {
		t.Fatal("go is not favorited after AddFavorite")
	}
}

func TestAddFavoriteIsIdempotent(t *testing.T) {
	favs := newTestFavorites(t)

	if err := favs.AddFavorite("go", "Go"); err != nil {
		t.Fatalf("first AddFavorite: %v", err)
	}
	if err := favs.AddFavorite("go", "Go 语言"); err != nil {
		t.Fatalf("second AddFavorite: %v", err)
	}

	list, err := favs.Favorites()
	if err != nil {
		t.Fatalf("Favorites: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("got %d favorites, want 1", len(list))
	}
	if list[0].NodeTitle != "Go 语言" {
		t.Errorf("title = %q, want the second one to win", list[0].NodeTitle)
	}
}

func TestRemoveFavorite(t *testing.T) {
	favs := newTestFavorites(t)

	if err := favs.AddFavorite("python", "Python"); err != nil {
		t.Fatalf("AddFavorite: %v", err)
	}

	if err := favs.RemoveFavorite("python"); err != nil {
		t.Fatalf("RemoveFavorite: %v", err)
	}

	ok, err := favs.IsFavorite("python")
	if err != nil {
		t.Fatalf("IsFavorite: %v", err)
	}
	if ok {
		t.Fatal("python is still favorited after RemoveFavorite")
	}

	// Removing a missing favorite is not an error.
	if err := favs.RemoveFavorite("missing"); err != nil {
		t.Fatalf("RemoveFavorite(missing): %v", err)
	}
}

func TestToggleFavorite(t *testing.T) {
	favs := newTestFavorites(t)

	on, err := favs.ToggleFavorite("macos", "macOS")
	if err != nil {
		t.Fatalf("first ToggleFavorite: %v", err)
	}
	if !on {
		t.Fatal("first toggle did not favorite the node")
	}

	off, err := favs.ToggleFavorite("macos", "macOS")
	if err != nil {
		t.Fatalf("second ToggleFavorite: %v", err)
	}
	if off {
		t.Fatal("second toggle did not unfavorite the node")
	}
}

func TestFavoritesOrder(t *testing.T) {
	favs := newTestFavorites(t)

	for _, name := range []string{"go", "python", "macos"} {
		if err := favs.AddFavorite(name, name); err != nil {
			t.Fatalf("AddFavorite(%q): %v", name, err)
		}
	}

	list, err := favs.Favorites()
	if err != nil {
		t.Fatalf("Favorites: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("got %d favorites, want 3", len(list))
	}

	// Rows added within the same second tie on created_at and fall back to
	// the node name, which keeps the order stable between frames.
	for i, want := range []string{"go", "macos", "python"} {
		if list[i].NodeName != want {
			t.Errorf("favorites[%d] = %q, want %q", i, list[i].NodeName, want)
		}
	}
	for _, f := range list {
		if f.CreatedAt.IsZero() {
			t.Errorf("favorite %q has a zero timestamp", f.NodeName)
		}
	}
}

func TestEmptyNodeName(t *testing.T) {
	favs := newTestFavorites(t)

	if err := favs.AddFavorite("", "Go"); err == nil {
		t.Fatal("AddFavorite accepted an empty node name")
	}
	if err := favs.RemoveFavorite(""); err == nil {
		t.Fatal("RemoveFavorite accepted an empty node name")
	}
}
