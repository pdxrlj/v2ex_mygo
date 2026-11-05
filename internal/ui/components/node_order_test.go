package components

import (
	"path/filepath"
	"slices"
	"testing"

	"mygo_demo/internal/database"
	"mygo_demo/internal/database/repo"
)

func TestMoveNames(t *testing.T) {
	names := []string{"a", "b", "c", "d"}

	tests := []struct {
		name string
		rows []int
		to   int
		want []string
	}{
		{"first to the end", []int{0}, 4, []string{"b", "c", "d", "a"}},
		{"last to the top", []int{3}, 0, []string{"d", "a", "b", "c"}},
		{"before the next row is a no-op", []int{1}, 2, []string{"a", "b", "c", "d"}},
		{"several move together", []int{0, 2}, 3, []string{"b", "a", "c", "d"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := move(names, tt.rows, tt.to)
			if !slices.Equal(got, tt.want) {
				t.Errorf("move(%v, %v, %d) = %v, want %v", names, tt.rows, tt.to, got, tt.want)
			}
		})
	}
}

func TestPinnedSectionHeight(t *testing.T) {
	tests := []struct {
		rows int
		want float32
	}{
		{0, 0},
		{1, nodeRowHeight},
		{4, 4*nodeRowHeight + 3*nodeRowGap},
		{8, 4*nodeRowHeight + 3*nodeRowGap},
	}

	for _, tt := range tests {
		if got := pinnedSectionHeight(tt.rows); got != tt.want {
			t.Errorf("pinnedSectionHeight(%d) = %v, want %v", tt.rows, got, tt.want)
		}
	}
}

func TestNodeLayoutRoundTrip(t *testing.T) {
	conn, err := database.Open(filepath.Join(t.TempDir(), "mygo.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	settings := repo.NewSettings(conn)
	if got := loadNodeLayout(settings); len(got.Pins) != 0 || len(got.Order) != 0 {
		t.Fatalf("fresh layout = %+v, want empty", got)
	}

	want := nodeLayout{
		Pins:  []string{"go", "qna"},
		Order: []string{"python", "macos"},
	}
	if err := saveNodeLayout(settings, want); err != nil {
		t.Fatalf("saveNodeLayout: %v", err)
	}

	got := loadNodeLayout(settings)
	if got.Version != nodeLayoutVersion {
		t.Errorf("version = %d, want %d", got.Version, nodeLayoutVersion)
	}
	if !slices.Equal(got.Pins, want.Pins) || !slices.Equal(got.Order, want.Order) {
		t.Errorf("layout = %+v, want %+v", got, want)
	}
}

func TestNodeLayoutBrokenJSON(t *testing.T) {
	conn, err := database.Open(filepath.Join(t.TempDir(), "mygo.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	settings := repo.NewSettings(conn)
	if err := settings.Set(repo.NodeLayoutKey, "{not json"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	if got := loadNodeLayout(settings); len(got.Pins) != 0 || len(got.Order) != 0 {
		t.Errorf("broken layout = %+v, want empty", got)
	}
}
