package transport

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Error is Enquoir's RFC 9457 Problem response. Code is stable and meant to
// be branched on; Detail is for people and may change.
type Error struct {
	Type       string
	Title      string
	Status     int
	Detail     string
	Code       string
	Field      string
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("enquoir: %s: %s (field %q, status %d)", e.Code, e.Detail, e.Field, e.Status)
	}
	return fmt.Sprintf("enquoir: %s: %s (status %d)", e.Code, e.Detail, e.Status)
}

func newError(status int, header http.Header, body []byte) error {
	var problem struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Detail string `json:"detail"`
		Code   string `json:"code"`
		Field  string `json:"field"`
	}
	if err := json.Unmarshal(body, &problem); err != nil {
		return fmt.Errorf("enquoir: status %d, body not a Problem: %s", status, string(body))
	}
	return &Error{
		Type:       problem.Type,
		Title:      problem.Title,
		Status:     status,
		Detail:     problem.Detail,
		Code:       problem.Code,
		Field:      problem.Field,
		RetryAfter: retryAfterHeader(header),
	}
}
