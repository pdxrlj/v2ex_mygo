package api

import (
	"fmt"

	"github.com/go-resty/resty/v2"
)

// v2Response is the envelope shared by all V2EX API 2.0 endpoints.
type v2Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Pagination is returned by the paginated list endpoints. The API does not
// echo the requested page, so callers must track it themselves.
type Pagination struct {
	PerPage int `json:"per_page"`
	Total   int `json:"total"`
	Pages   int `json:"pages"`
}

// check reports a non-2xx status or a success:false response as an error.
// apiErr carries the envelope resty decodes for failed requests (SetError).
func (body v2Response) check(resp *resty.Response, apiErr v2Response) error {
	if resp.IsError() {
		msg := firstNonEmpty(apiErr.Message, body.Message, resp.Status())
		return fmt.Errorf("v2ex api error (%d): %s", resp.StatusCode(), msg)
	}
	if !body.Success {
		msg := firstNonEmpty(apiErr.Message, body.Message, "unknown error")
		return fmt.Errorf("v2ex api error: %s", msg)
	}
	return nil
}

// firstNonEmpty returns the first non-empty string, or "" if none is set.
func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
