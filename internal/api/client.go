// Package api provides a Go client for the V2EX API.
//
// See https://v2ex.com/help/api for the API documentation.
package api

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
)

// siteURL is the V2EX site root, which also serves the public pages.
const siteURL = "https://www.v2ex.com"

// baseURL is the common prefix of every V2EX API endpoint.
const baseURL = siteURL + "/api"

// Client is a V2EX API client with token authentication and retries on
// transient failures.
type Client struct {
	// Key is the personal access token used for authentication.
	Key string
	*resty.Client

	// SiteURL is the site root, used when reading a public page instead of
	// an API endpoint.
	SiteURL string

	// Cache, when set, stores responses of the large legacy listings so
	// they are not refetched on every call. A nil Cache disables caching.
	Cache *Cache
}

// NewClient returns a V2EX API client. The V2EX_TOKEN environment variable
// (or a .env entry, see loadEnv) must be set. An optional proxy URL, e.g.
// "http://127.0.0.1:7890", routes all requests through the proxy.
func NewClient(proxy ...string) (*Client, error) {
	loadEnv()

	token := os.Getenv("V2EX_TOKEN")
	if token == "" {
		return nil, errors.New("V2EX_TOKEN is not set")
	}

	return NewClientWithToken(token, proxy...)
}

// NewClientWithToken returns a V2EX API client authenticated with token, as
// the settings panel's saved token uses it. An optional proxy URL routes all
// requests through the proxy.
func NewClientWithToken(token string, proxy ...string) (*Client, error) {
	loadEnv()

	if token == "" {
		return nil, errors.New("V2EX_TOKEN is not set")
	}

	client := resty.New().
		SetBaseURL(baseURL).
		SetRetryCount(3).
		SetRetryWaitTime(time.Second).
		SetRetryMaxWaitTime(3*time.Second).
		AddRetryCondition(retryCondition).
		SetHeader("Authorization", "Bearer "+token)

	if len(proxy) > 0 && proxy[0] != "" {
		client = client.SetProxy(proxy[0])
	}

	return &Client{Key: token, Client: client, SiteURL: siteURL}, nil
}

// SetToken replaces the token later requests are authorized with, as the
// settings panel does when its token is applied. An empty token stops the
// requests being authorized at all.
func (c *Client) SetToken(token string) {
	c.Key = token
	if token == "" {
		c.Header.Del("Authorization")
		return
	}
	c.SetHeader("Authorization", "Bearer "+token)
}

// retryCondition decides whether a failed request is retried. Resty retries
// transport errors by default, but adding a custom condition replaces that
// rule, so the error is handled explicitly here.
func retryCondition(resp *resty.Response, err error) bool {
	// Network errors and timeouts are transient, always retry them.
	if err != nil {
		return true
	}
	if resp == nil {
		return false
	}

	// V2EX documents no status-code semantics, so only retry transient server
	// errors. 4xx responses (invalid token, missing resource, rate limit)
	// are not retryable: they persist until the caller intervenes.
	return resp.StatusCode() >= http.StatusInternalServerError
}
