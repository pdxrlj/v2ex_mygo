package api

import (
	"net/http"
	"strings"
	"testing"
)

func TestNodeGet(t *testing.T) {
	var path string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		writeJSON(t, w, http.StatusOK, nodeResponse{
			v2Response: v2Response{Success: true, Message: "Node details found"},
			Result:     NodeInfo{ID: 90, Name: "python", Title: "Python", Topics: 16297},
		})
	})

	info, err := NewNode(client).Get("python")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if path != "/v2/nodes/python" {
		t.Errorf("path = %q, want /v2/nodes/python", path)
	}
	if info.ID != 90 || info.Name != "python" || info.Title != "Python" {
		t.Errorf("unexpected node: %+v", info)
	}
}

func TestNodeGetAPIError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusNotFound, v2Response{Message: "Node not found"})
	})

	_, err := NewNode(client).Get("missing")
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"404", "Node not found"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestNodeGetSuccessFalse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, v2Response{Message: "Node unavailable"})
	})

	_, err := NewNode(client).Get("python")
	if err == nil || !strings.Contains(err.Error(), "Node unavailable") {
		t.Fatalf("err = %v, want message Node unavailable", err)
	}
}

func TestNodeGetEmptyName(t *testing.T) {
	if _, err := NewNode(&Client{}).Get(""); err == nil {
		t.Fatal("expected an error for an empty node name")
	}
}

func TestNodeTopics(t *testing.T) {
	var path, page string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		page = r.URL.Query().Get("p")
		writeJSON(t, w, http.StatusOK, nodeTopicsResponse{
			v2Response: v2Response{Success: true, Message: "Node topics found"},
			Result: []TopicInfo{
				{ID: 1, Title: "first"},
				{ID: 2, Title: "second"},
			},
			Pagination: Pagination{PerPage: 20, Total: 16297, Pages: 815},
		})
	})

	topics, pagination, err := NewNode(client).Topics("python", 2)
	if err != nil {
		t.Fatalf("Topics: %v", err)
	}
	if path != "/v2/nodes/python/topics" {
		t.Errorf("path = %q, want /v2/nodes/python/topics", path)
	}
	if page != "2" {
		t.Errorf("p = %q, want 2", page)
	}
	if len(topics) != 2 || topics[0].ID != 1 || topics[1].Title != "second" {
		t.Errorf("unexpected topics: %+v", topics)
	}
	if pagination.PerPage != 20 || pagination.Total != 16297 || pagination.Pages != 815 {
		t.Errorf("unexpected pagination: %+v", pagination)
	}
}

func TestNodeTopicsClampsPage(t *testing.T) {
	var page string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		page = r.URL.Query().Get("p")
		writeJSON(t, w, http.StatusOK, nodeTopicsResponse{
			v2Response: v2Response{Success: true},
		})
	})

	if _, _, err := NewNode(client).Topics("python", 0); err != nil {
		t.Fatalf("Topics: %v", err)
	}
	if page != "1" {
		t.Errorf("p = %q, want 1", page)
	}
}

func TestNodeTopicsEmptyName(t *testing.T) {
	if _, _, err := NewNode(&Client{}).Topics("", 1); err == nil {
		t.Fatal("expected an error for an empty node name")
	}
}
