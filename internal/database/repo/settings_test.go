package repo

import (
	"path/filepath"
	"testing"

	"mygo_demo/internal/database"
)

// TestSettingsSetAndGet checks the key-value store and that an explicitly
// empty value stays distinguishable from an unset key.
func TestSettingsSetAndGet(t *testing.T) {
	conn, err := database.Open(filepath.Join(t.TempDir(), "mygo.db"))
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	settings := NewSettings(conn)

	if value, ok, err := settings.Get(ProxyKey); err != nil || ok || value != "" {
		t.Fatalf("Get before Set = %q, %v, %v; want empty, false, nil", value, ok, err)
	}

	if err := settings.Set(ProxyKey, "http://127.0.0.1:7890"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	value, ok, err := settings.Get(ProxyKey)
	if err != nil || !ok || value != "http://127.0.0.1:7890" {
		t.Errorf("Get = %q, %v, %v; want the saved proxy", value, ok, err)
	}

	if err := settings.Set(ProxyKey, ""); err != nil {
		t.Fatalf("Set empty: %v", err)
	}
	value, ok, err = settings.Get(ProxyKey)
	if err != nil || !ok || value != "" {
		t.Errorf("Get after clearing = %q, %v, %v; want empty but set", value, ok, err)
	}
}
