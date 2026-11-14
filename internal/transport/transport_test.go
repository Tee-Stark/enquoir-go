package transport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDo_GETDecodesSuccessBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"key": "abc"})
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, APIKey: "enq_sk_test"})

	var out struct {
		Key string `json:"key"`
	}
	if err := c.Do(context.Background(), http.MethodGet, "/v1/intents/abc", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if out.Key != "abc" {
		t.Errorf("Key = %q, want %q", out.Key, "abc")
	}
}

func TestDo_SetsBearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, APIKey: "enq_sk_test"})
	if err := c.Do(context.Background(), http.MethodGet, "/v1/intents/abc", nil, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if want := "Bearer enq_sk_test"; gotAuth != want {
		t.Errorf("Authorization = %q, want %q", gotAuth, want)
	}
}

func TestDo_PostIsNeverRetried(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]any{
			"type": "about:blank", "title": "Internal Server Error",
			"status": 500, "detail": "boom", "code": "internal_error",
		})
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, APIKey: "k"})
	err := c.Do(context.Background(), http.MethodPost, "/v1/intents", map[string]string{"a": "b"}, nil)
	if err == nil {
		t.Fatal("Do: want error, got nil")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (mutating calls are never retried)", attempts)
	}
}

func TestDo_GETRetriesOn5xxThenSucceeds(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]any{
				"type": "about:blank", "title": "Internal Server Error",
				"status": 500, "detail": "boom", "code": "internal_error",
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"key": "abc"})
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, APIKey: "k"})
	var out struct {
		Key string `json:"key"`
	}
	if err := c.Do(context.Background(), http.MethodGet, "/v1/intents/abc", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
	if out.Key != "abc" {
		t.Errorf("Key = %q, want %q", out.Key, "abc")
	}
}

func TestDo_GETDoesNotRetryOn4xx(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{
			"type": "about:blank", "title": "Not Found",
			"status": 404, "detail": "no such intent", "code": "not_found",
		})
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, APIKey: "k"})
	err := c.Do(context.Background(), http.MethodGet, "/v1/intents/missing", nil, nil)
	if err == nil {
		t.Fatal("Do: want error, got nil")
	}
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (4xx is not retryable)", attempts)
	}
}

func TestDo_GETRetriesOn5xxWithNonProblemBody(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 2 {
			// A load balancer or proxy's own error page, not Enquoir's
			// Problem+JSON - exactly the transient failure GET retries
			// exist for.
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte("<html>502 Bad Gateway</html>"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"key": "abc"})
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, APIKey: "k"})
	var out struct {
		Key string `json:"key"`
	}
	if err := c.Do(context.Background(), http.MethodGet, "/v1/intents/abc", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2 (a non-Problem 5xx body must still be retried)", attempts)
	}
}

func TestDo_NonProblem5xxBodyIsStillAnAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("<html>502 Bad Gateway</html>"))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, APIKey: "k"})
	err := c.Do(context.Background(), http.MethodPost, "/v1/intents", map[string]string{}, nil)

	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("err = %T, want *Error", err)
	}
	if apiErr.Status != http.StatusBadGateway {
		t.Errorf("Status = %d, want %d", apiErr.Status, http.StatusBadGateway)
	}
	if apiErr.Detail != "<html>502 Bad Gateway</html>" {
		t.Errorf("Detail = %q, want the raw body preserved", apiErr.Detail)
	}
}

func TestDo_ValidJSONThatIsNotAProblemIsStillAnAPIError(t *testing.T) {
	// A proxy's own JSON error body (unlike the HTML case above) still
	// decodes successfully - it just has none of Problem's required
	// fields. The body must not be silently dropped just because it
	// happened to parse as JSON.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(`{"message":"Internal server error"}`))
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, APIKey: "k"})
	err := c.Do(context.Background(), http.MethodPost, "/v1/intents", map[string]string{}, nil)

	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("err = %T, want *Error", err)
	}
	if apiErr.Code != "" {
		t.Errorf("Code = %q, want empty (not a real Problem code)", apiErr.Code)
	}
	if apiErr.Detail != `{"message":"Internal server error"}` {
		t.Errorf("Detail = %q, want the raw body preserved", apiErr.Detail)
	}
}

func TestNew_TrimsTrailingSlashFromBaseURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL + "//", APIKey: "k"})
	if err := c.Do(context.Background(), http.MethodGet, "/v1/intents/abc", nil, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if gotPath != "/v1/intents/abc" {
		t.Errorf("request path = %q, want %q (every trailing slash on BaseURL must be trimmed)", gotPath, "/v1/intents/abc")
	}
}

func TestDo_ParsesProblemIntoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]any{
			"type": "about:blank", "title": "Too Many Requests",
			"status": 429, "detail": "slow down", "code": "rate_limited",
		})
	}))
	defer srv.Close()

	c := New(Config{BaseURL: srv.URL, APIKey: "k"})
	err := c.Do(context.Background(), http.MethodPost, "/v1/intents", map[string]string{}, nil)

	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("err = %T, want *Error", err)
	}
	if apiErr.Code != "rate_limited" {
		t.Errorf("Code = %q, want %q", apiErr.Code, "rate_limited")
	}
	if apiErr.RetryAfter.Seconds() != 30 {
		t.Errorf("RetryAfter = %v, want 30s", apiErr.RetryAfter)
	}
}
