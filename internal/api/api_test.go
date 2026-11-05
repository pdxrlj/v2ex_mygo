package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-resty/resty/v2"
)

// newTestClient returns a Client pointed at an in-process test server, so
// tests do not need a token, a proxy, or network access.
func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Client{
		Key:     "test-token",
		Client:  resty.New().SetBaseURL(server.URL),
		SiteURL: server.URL,
	}
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(t *testing.T, w http.ResponseWriter, status int, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Errorf("write json response: %v", err)
	}
}
