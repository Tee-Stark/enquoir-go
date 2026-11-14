package enquoir

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

func problemHandler(status int, code, detail string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]any{
			"type": "about:blank", "title": http.StatusText(status),
			"status": status, "detail": detail, "code": code,
		})
	}
}

func newTestPayments(t *testing.T, handler http.HandlerFunc) (*Payments, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Payments{transport: transport.New(transport.Config{BaseURL: srv.URL, APIKey: "enq_sk_test"})}, srv
}

func TestPayments_Submit_Success(t *testing.T) {
	var capturedReq []byte
	var capturedPath string
	p, _ := newTestPayments(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedReq, _ = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"key": "intent_abc", "state": "PROPOSED", "outcome": "started",
		})
	})

	resp, err := p.Submit(context.Background(), SubmitIntentRequest{
		Payee: "bnf_123", Amount: "10.50", Currency: USD, Reference: "invoice-1",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if resp.Key != "intent_abc" || resp.State != StateProposed || resp.Outcome != OutcomeStarted {
		t.Errorf("Submit response = %+v, want key=intent_abc state=PROPOSED outcome=started", resp)
	}

	validateRequest(t, http.MethodPost, capturedPath, capturedReq)
	validateResponse(t, http.MethodPost, capturedPath, http.StatusOK, http.Header{"Content-Type": {"application/json"}},
		[]byte(`{"key":"intent_abc","state":"PROPOSED","outcome":"started"}`))
}

func TestPayments_Submit_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"body too large", http.StatusRequestEntityTooLarge, "body_too_large"},
		{"forbidden", http.StatusForbidden, "email_unverified"},
		{"validation failed", http.StatusBadRequest, "validation_failed"},
		{"unauthenticated", http.StatusUnauthorized, "unauthenticated"},
		{"payee unknown", http.StatusUnprocessableEntity, "payee_unknown"},
		{"internal error", http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := newTestPayments(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := p.Submit(context.Background(), SubmitIntentRequest{
				Payee: "bnf_123", Amount: "10.50", Currency: USD, Reference: "invoice-1",
			})
			if !IsCode(err, tt.code) {
				t.Errorf("Submit error code = %v, want %q", err, tt.code)
			}
		})
	}
}

func TestPayments_Get_Success(t *testing.T) {
	var capturedPath string
	p, _ := newTestPayments(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"key": "intent_abc", "state": "APPLIED",
			"events": []map[string]any{
				{"sequence_number": 1, "type": "PROPOSED", "payload": "{}"},
				{"sequence_number": 2, "type": "APPLIED", "payload": "{}"},
			},
		})
	})

	detail, err := p.Get(context.Background(), "intent_abc")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if detail.Key != "intent_abc" || detail.State != StateApplied {
		t.Errorf("Get = %+v, want key=intent_abc state=APPLIED", detail)
	}
	if len(detail.Events) != 2 || detail.Events[1].Type != StateApplied {
		t.Errorf("Get events = %+v", detail.Events)
	}

	validateRequest(t, http.MethodGet, capturedPath, nil)
	validateResponse(t, http.MethodGet, capturedPath, http.StatusOK, http.Header{"Content-Type": {"application/json"}},
		[]byte(`{"key":"intent_abc","state":"APPLIED","events":[{"sequence_number":1,"type":"PROPOSED","payload":"{}"},{"sequence_number":2,"type":"APPLIED","payload":"{}"}]}`))
}

func TestPayments_Get_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"unauthenticated", http.StatusUnauthorized, "unauthenticated"},
		{"not found", http.StatusNotFound, "not_found"},
		{"internal error", http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _ := newTestPayments(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := p.Get(context.Background(), "intent_abc")
			if !IsCode(err, tt.code) {
				t.Errorf("Get error code = %v, want %q", err, tt.code)
			}
		})
	}
}
