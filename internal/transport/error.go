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
	code := e.Code
	if code == "" {
		// An intermediary's non-Problem response: see newError.
		code = "non_problem_response"
	}
	if e.Field != "" {
		return fmt.Sprintf("enquoir: %s: %s (field %q, status %d)", code, e.Detail, e.Field, e.Status)
	}
	return fmt.Sprintf("enquoir: %s: %s (status %d)", code, e.Detail, e.Status)
}

// maxNonProblemBodyInError caps how much of a non-Problem body (an
// intermediary's HTML error page, say) ends up in Detail.
const maxNonProblemBodyInError = 500

func newError(status int, header http.Header, body []byte) error {
	var problem struct {
		Type   string `json:"type"`
		Title  string `json:"title"`
		Status int    `json:"status"`
		Detail string `json:"detail"`
		Code   string `json:"code"`
		Field  string `json:"field"`
	}
	// Code is required on every real Problem; an empty one means the
	// body either failed to decode as JSON at all, or decoded cleanly
	// into something that merely isn't a Problem (a proxy's own JSON
	// error body, `{"message":"..."}` say, or a bare `null`) - both
	// are a load balancer or intermediary's own 5xx, not Enquoir's.
	// This is exactly the transient failure GET retries exist for, so
	// it still comes back as an *Error - not some other type
	// retryable() and IsCode can't recognize - carrying what Status it
	// can and the raw body, truncated, so nothing is silently lost.
	if err := json.Unmarshal(body, &problem); err != nil || problem.Code == "" {
		detail := string(body)
		if len(detail) > maxNonProblemBodyInError {
			detail = detail[:maxNonProblemBodyInError] + "..."
		}
		return &Error{
			Status:     status,
			Detail:     detail,
			RetryAfter: retryAfterHeader(header),
		}
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
