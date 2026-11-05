package api

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/alecthomas/assert/v2"
)

func TestAllNode_Get(t *testing.T) {
	client, err := NewClient("http://127.0.0.1:7890")
	assert.NoError(t, err)

	nodes, err := NewAllNode(client).Get()

	assert.NoError(t, err)
	assert.NotEqual(t, 0, len(nodes))
	assert.NotEqual(t, "", nodes[0].Name)
	assert.NotEqual(t, 0, nodes[0].ID)
	t.Logf("nodes: %d, first: %+v", len(nodes), nodes[0])
}

func TestAllNodeHot(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/nodes/all.json" {
			t.Errorf("path = %q, want /nodes/all.json", r.URL.Path)
		}
		writeJSON(t, w, http.StatusOK, []AllNodeInfo{
			{ID: 1, Name: "a", Topics: 10, Stars: 5},
			{ID: 2, Name: "b", Topics: 30, Stars: 1},
			{ID: 3, Name: "c", Topics: 30, Stars: 9},
			{ID: 4, Name: "d", Topics: 20, Stars: 2},
			{ID: 5, Name: "e", Topics: 10, Stars: 7},
		})
	})

	// Hottest first: topics descending, then stars descending.
	wantOrder := []string{"c", "b", "d", "e", "a"}

	nodes, err := NewAllNode(client).Hot(1)
	if err != nil {
		t.Fatalf("Hot: %v", err)
	}
	if len(nodes) != len(wantOrder) {
		t.Fatalf("got %d nodes, want %d", len(nodes), len(wantOrder))
	}
	for i, want := range wantOrder {
		if nodes[i].Name != want {
			t.Errorf("nodes[%d] = %q, want %q", i, nodes[i].Name, want)
		}
	}
}

func TestAllNodeHotCapsPageSize(t *testing.T) {
	all := make([]AllNodeInfo, 0, hotNodesPerPage+5)
	for i := range cap(all) {
		all = append(all, AllNodeInfo{ID: i + 1, Name: "n", Topics: i})
	}
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, all)
	})

	nodes, err := NewAllNode(client).Hot(1)
	if err != nil {
		t.Fatalf("Hot: %v", err)
	}
	if len(nodes) != hotNodesPerPage {
		t.Errorf("got %d nodes, want %d", len(nodes), hotNodesPerPage)
	}
	if nodes[0].Topics != len(all)-1 {
		t.Errorf("first topic = %d, want %d", nodes[0].Topics, len(all)-1)
	}
}

func TestAllNodeGetUsesCache(t *testing.T) {
	requests := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeJSON(t, w, http.StatusOK, []AllNodeInfo{{ID: 1, Name: "a", Topics: 10}})
	})

	cache, err := OpenCache(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("OpenCache: %v", err)
	}
	t.Cleanup(func() { cache.Close() })
	client.Cache = cache

	if _, err := NewAllNode(client).Get(); err != nil {
		t.Fatalf("first Get: %v", err)
	}
	if _, err := NewAllNode(client).Get(); err != nil {
		t.Fatalf("second Get: %v", err)
	}

	if requests != 1 {
		t.Errorf("server saw %d requests, want 1: the second Get should hit the cache", requests)
	}
}

func TestAllNodeGetRefetchesStaleCache(t *testing.T) {
	requests := 0
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		writeJSON(t, w, http.StatusOK, []AllNodeInfo{{ID: 1, Name: "a", Topics: 10}})
	})

	cache, err := OpenCache(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatalf("OpenCache: %v", err)
	}
	t.Cleanup(func() { cache.Close() })
	client.Cache = cache

	if _, err := NewAllNode(client).Get(); err != nil {
		t.Fatalf("first Get: %v", err)
	}

	// Move the clock past the TTL: the cached copy is no longer fresh.
	cache.now = func() time.Time { return time.Now().Add(CacheTTL + time.Minute) }

	if _, err := NewAllNode(client).Get(); err != nil {
		t.Fatalf("second Get: %v", err)
	}
	if requests != 2 {
		t.Errorf("server saw %d requests, want 2: a stale entry must be refetched", requests)
	}
}
