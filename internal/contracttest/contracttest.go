// Package contracttest is the contract-validation harness shared by
// every package's tests: it loads the vendored OpenAPI spec once per
// test binary and checks a request or response against it with
// kin-openapi/openapi3filter - the same library and pattern the
// backend's own contract_test.go uses. Exported so both the root
// package and commerce can use one harness instead of two copies.
package contracttest

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/legacy"

	"github.com/Tee-Stark/enquoir-go/openapi"
)

var (
	specOnce   sync.Once
	specDoc    *openapi3.T
	specRouter routers.Router
	specErr    error
)

// LoadSpec parses and validates the vendored spec once per test binary.
func LoadSpec(t *testing.T) (*openapi3.T, routers.Router) {
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

// ValidateRequest checks one outgoing request - method, path, and JSON
// body - against the vendored spec's operation for that route.
func ValidateRequest(t *testing.T, method, path string, body []byte) {
	t.Helper()
	_, router := LoadSpec(t)

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, "http://sdk.test"+path, bodyReader)
	if err != nil {
		t.Fatalf("ValidateRequest: build request: %v", err)
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

// ValidateResponse checks one received response - status, headers and
// JSON body - against the vendored spec's operation for that route.
func ValidateResponse(t *testing.T, method, path string, status int, header http.Header, body []byte) {
	t.Helper()
	_, router := LoadSpec(t)

	req, err := http.NewRequest(method, "http://sdk.test"+path, nil)
	if err != nil {
		t.Fatalf("ValidateResponse: build request: %v", err)
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
