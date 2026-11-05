package api

import (
	"path/filepath"
	"testing"
	"time"
)

// openTestCache returns a Cache backed by a file in the test's temp dir.
func openTestCache(t *testing.T) *Cache {
	t.Helper()

	cache, err := OpenCache(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("OpenCache: %v", err)
	}
	t.Cleanup(func() { cache.Close() })
	return cache
}

func TestCacheGetPut(t *testing.T) {
	cache := openTestCache(t)

	var stored map[string]int
	if ok, err := cache.Get("missing", &stored); err != nil || ok {
		t.Fatalf("Get(missing) = %v, %v; want false, nil", ok, err)
	}

	if err := cache.Put("key", map[string]int{"v": 1}); err != nil {
		t.Fatalf("Put: %v", err)
	}

	var loaded map[string]int
	if ok, err := cache.Get("key", &loaded); err != nil || !ok {
		t.Fatalf("Get(key) = %v, %v; want true, nil", ok, err)
	}
	if loaded["v"] != 1 {
		t.Errorf("loaded = %+v, want v:1", loaded)
	}
}

func TestCacheSetTTL(t *testing.T) {
	cache := openTestCache(t)

	if cache.TTL() != CacheTTL {
		t.Errorf("TTL = %v, want %v", cache.TTL(), CacheTTL)
	}

	cache.SetTTL(time.Minute)
	if cache.TTL() != time.Minute {
		t.Errorf("TTL = %v, want 1m", cache.TTL())
	}

	// A non-positive TTL restores the default rather than expiring at once.
	cache.SetTTL(0)
	if cache.TTL() != CacheTTL {
		t.Errorf("TTL = %v, want %v after reset", cache.TTL(), CacheTTL)
	}
}

func TestCacheTTLExpiryAndPurge(t *testing.T) {
	cache := openTestCache(t)
	start := time.Now()
	cache.now = func() time.Time { return start }

	if err := cache.PutTTL("short", "s", time.Minute); err != nil {
		t.Fatalf("PutTTL short: %v", err)
	}
	if err := cache.PutTTL("long", "l", time.Hour); err != nil {
		t.Fatalf("PutTTL long: %v", err)
	}

	cache.now = func() time.Time { return start.Add(2 * time.Minute) }

	var s string
	if ok, err := cache.Get("short", &s); err != nil || ok {
		t.Fatalf("Get(short) = %v, %v; want false, nil after its TTL", ok, err)
	}
	var l string
	if ok, err := cache.Get("long", &l); err != nil || !ok {
		t.Fatalf("Get(long) = %v, %v; want true, nil while fresh", ok, err)
	}

	removed, err := cache.PurgeExpired()
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	// The long entry is still there after the purge.
	if ok, err := cache.Get("long", &l); err != nil || !ok {
		t.Fatalf("Get(long) after purge = %v, %v; want true, nil", ok, err)
	}
}

func TestCacheDeleteAndClear(t *testing.T) {
	cache := openTestCache(t)

	if err := cache.Put("a", "1"); err != nil {
		t.Fatalf("Put a: %v", err)
	}
	if err := cache.Put("b", "2"); err != nil {
		t.Fatalf("Put b: %v", err)
	}

	ok, err := cache.Delete("a")
	if err != nil || !ok {
		t.Fatalf("Delete(a) = %v, %v; want true, nil", ok, err)
	}
	ok, err = cache.Delete("a")
	if err != nil || ok {
		t.Fatalf("Delete(a) again = %v, %v; want false, nil", ok, err)
	}

	var s string
	if hit, err := cache.Get("a", &s); err != nil || hit {
		t.Fatalf("Get(a) = %v, %v; want false, nil after Delete", hit, err)
	}
	if hit, err := cache.Get("b", &s); err != nil || !hit {
		t.Fatalf("Get(b) = %v, %v; want true, nil", hit, err)
	}

	cleared, err := cache.Clear()
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if cleared != 1 {
		t.Errorf("cleared = %d, want 1", cleared)
	}
	if hit, err := cache.Get("b", &s); err != nil || hit {
		t.Fatalf("Get(b) after Clear = %v, %v; want false, nil", hit, err)
	}
}
