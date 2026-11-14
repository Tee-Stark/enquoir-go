// Package transport is the shared HTTP layer both TreasuryClient and
// MerchantClient build on: request signing, JSON encoding, capped retries
// on reads, and turning a Problem response into an *Error.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Config configures a Client.
type Config struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// Client is the shared HTTP layer one TreasuryClient or MerchantClient
// builds on.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New builds a Client from cfg.
func New(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}
	return &Client{
		// Every path this client builds already starts with "/", so a
		// trailing slash on BaseURL (a realistic WithBaseURL input)
		// would otherwise double up into "//v1/..." and 404.
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:     cfg.APIKey,
		httpClient: httpClient,
	}
}

const (
	// maxReadAttempts is the total number of tries a GET gets - the
	// first try plus up to maxReadAttempts-1 retries.
	maxReadAttempts = 3
	retryBaseDelay  = 200 * time.Millisecond
)

// Do sends one API call. GET requests get capped retries on network errors
// and 5xx; every other method is sent once - matching the backend's own
// "no blind retries" rule for mutating calls. body is marshaled as the
// request's JSON, when non-nil; out is populated from the response's JSON,
// when non-nil.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return fmt.Errorf("enquoir: encode request: %w", err)
		}
	}

	attempts := 1
	if method == http.MethodGet {
		attempts = maxReadAttempts
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			delay := retryBaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		err := c.do(ctx, method, path, payload, out)
		if err == nil {
			return nil
		}
		lastErr = err

		if !retryable(err) {
			return err
		}
	}
	return lastErr
}

func (c *Client) do(ctx context.Context, method, path string, payload []byte, out any) error {
	var bodyReader io.Reader
	if payload != nil {
		bodyReader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("enquoir: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return &networkError{err: err}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("enquoir: read response: %w", err)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("enquoir: decode response: %w", err)
			}
		}
		return nil
	}

	return newError(resp.StatusCode, resp.Header, respBody)
}

// networkError marks a transport-level failure (dial, TLS, timeout) as
// retryable; an Error built from a 4xx/5xx response decides for itself.
type networkError struct{ err error }

func (e *networkError) Error() string { return "enquoir: request failed: " + e.err.Error() }
func (e *networkError) Unwrap() error { return e.err }

func retryable(err error) bool {
	if _, ok := err.(*networkError); ok {
		return true
	}
	if apiErr, ok := err.(*Error); ok {
		return apiErr.Status >= 500
	}
	return false
}

func retryAfterHeader(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}
	secs, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return time.Duration(secs) * time.Second
}
