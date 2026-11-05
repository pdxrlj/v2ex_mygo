package api

import (
	"errors"
	"net/http"
	"testing"

	"github.com/go-resty/resty/v2"
)

func TestRetryCondition(t *testing.T) {
	tests := []struct {
		name string
		resp *resty.Response
		err  error
		want bool
	}{
		{"network error", nil, errors.New("dial failed"), true},
		{"no response", nil, nil, false},
		{"internal server error", responseWithStatus(http.StatusInternalServerError), nil, true},
		{"bad gateway", responseWithStatus(http.StatusBadGateway), nil, true},
		{"rate limited", responseWithStatus(http.StatusTooManyRequests), nil, false},
		{"unauthorized", responseWithStatus(http.StatusUnauthorized), nil, false},
		{"not found", responseWithStatus(http.StatusNotFound), nil, false},
		{"success", responseWithStatus(http.StatusOK), nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := retryCondition(tt.resp, tt.err); got != tt.want {
				t.Errorf("retryCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewClientRequiresToken(t *testing.T) {
	t.Setenv("V2EX_TOKEN", "")

	if _, err := NewClient(); err == nil {
		t.Fatal("expected an error when V2EX_TOKEN is empty")
	}
}

func responseWithStatus(status int) *resty.Response {
	return &resty.Response{RawResponse: &http.Response{StatusCode: status}}
}
