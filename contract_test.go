package enquoir

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tee-Stark/enquoir-go/internal/contracttest"
	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

func TestSpec_IsValidOpenAPI31(t *testing.T) {
	doc, _ := contracttest.LoadSpec(t)
	if !doc.IsOpenAPI31OrLater() {
		t.Errorf("openapi = %q, want 3.1.x", doc.OpenAPI)
	}
}

// validateRequest and validateResponse are thin wrappers so this
// package's other test files read the same way they did before the
// harness moved to internal/contracttest for commerce to share.
func validateRequest(t *testing.T, method, path string, body []byte) {
	contracttest.ValidateRequest(t, method, path, body)
}

func validateResponse(t *testing.T, method, path string, status int, header http.Header, body []byte) {
	contracttest.ValidateResponse(t, method, path, status, header, body)
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
