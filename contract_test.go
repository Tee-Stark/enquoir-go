package enquoir

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
	"github.com/Tee-Stark/enquoir-go/openapi"
)

var (
	specOnce   sync.Once
	specDoc    *openapi3.T
	specRouter routers.Router
	specErr    error
)

// loadSpec parses and validates the vendored spec once per test binary -
// the same contract every resource method's table-driven test checks
// its request and response against.
func loadSpec(t *testing.T) (*openapi3.T, routers.Router) {
	t.Helper()
	specOnce.Do(func() {
		loader := openapi3.NewLoader()
		specDoc, specErr = loader.LoadFromData(openapi.Spec)
		if specErr != nil {
			return
		}
		if specErr = specDoc.Validate(context.Background()); specErr != nil {
			return
		}
		// Match requests by path alone; a test's base URL is never one
		// of the spec's own servers.
		specDoc.Servers = nil
		specRouter, specErr = legacy.NewRouter(specDoc)
	})
	if specErr != nil {
		t.Fatalf("openapi/openapi.yaml: %v", specErr)
	}
	return specDoc, specRouter
}

func TestSpec_IsValidOpenAPI31(t *testing.T) {
	doc, _ := loadSpec(t)
	if !doc.IsOpenAPI31OrLater() {
		t.Errorf("openapi = %q, want 3.1.x", doc.OpenAPI)
	}
}

// validateRequest checks one outgoing request - method, path, and JSON
// body - against the vendored spec's operation for that route. Every
// resource method's table-driven test calls this on the request it sent.
func validateRequest(t *testing.T, method, path string, body []byte) {
	t.Helper()
	_, router := loadSpec(t)

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, "http://sdk.test"+path, bodyReader)
	if err != nil {
		t.Fatalf("validateRequest: build request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	route, params, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("contract: %s %s is not in openapi/openapi.yaml: %v", method, path, err)
	}
	input := &openapi3filter.RequestValidationInput{
		Request:    req,
		PathParams: params,
		Route:      route,
		Options:    &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
	}
	if err := openapi3filter.ValidateRequest(context.Background(), input); err != nil {
		t.Errorf("contract: %s %s does not match openapi/openapi.yaml: %v\nbody: %s", method, path, err, string(body))
	}
}

// validateResponse checks one received response - status, headers and
// JSON body - against the vendored spec's operation for that route.
func validateResponse(t *testing.T, method, path string, status int, header http.Header, body []byte) {
	t.Helper()
	_, router := loadSpec(t)

	req, err := http.NewRequest(method, "http://sdk.test"+path, nil)
	if err != nil {
		t.Fatalf("validateResponse: build request: %v", err)
	}
	route, params, err := router.FindRoute(req)
	if err != nil {
		t.Fatalf("contract: %s %s is not in openapi/openapi.yaml: %v", method, path, err)
	}
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: params,
			Route:      route,
			Options:    &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc},
		},
		Status:  status,
		Header:  header,
		Options: &openapi3filter.Options{IncludeResponseStatus: true},
	}
	input.SetBodyBytes(body)
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		t.Errorf("contract: %s %s -> %d does not match openapi/openapi.yaml: %v\nbody: %s", method, path, status, err, string(body))
	}
}

// TestHarness_TransportRequestAndResponseMatchSpec proves the contract
// harness works end-to-end against a real documented route, before any
// resource method exists to exercise it: it drives internal/transport
// directly, the same way every later Treasury/Commerce method will.
func TestHarness_TransportRequestAndResponseMatchSpec(t *testing.T) {
	const intentKey = "4f6a1b2c3d4e5f60718293a4b5c6d7e8"

	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"key":   intentKey,
			"state": "APPLIED",
			"events": []map[string]any{
				{"sequence_number": 1, "type": "PROPOSED", "payload": "{}"},
			},
		})
	}))
	defer srv.Close()

	c := transport.New(transport.Config{BaseURL: srv.URL, APIKey: "enq_sk_test"})

	var out struct {
		Key    string `json:"key"`
		State  string `json:"state"`
		Events []struct {
			SequenceNumber int    `json:"sequence_number"`
			Type           string `json:"type"`
			Payload        string `json:"payload"`
		} `json:"events"`
	}
	path := "/v1/intents/" + intentKey
	if err := c.Do(context.Background(), http.MethodGet, path, nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}

	validateRequest(t, gotMethod, gotPath, nil)

	respBody, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal response for validation: %v", err)
	}
	validateResponse(t, gotMethod, gotPath, http.StatusOK,
		http.Header{"Content-Type": []string{"application/json"}}, respBody)
}
