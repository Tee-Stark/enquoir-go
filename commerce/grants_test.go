package commerce

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

func newTestGrants(t *testing.T, handler http.HandlerFunc) *Grants {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewGrants(transport.New(transport.Config{BaseURL: srv.URL, APIKey: "enq_mk_test"}))
}

func executionFixture() map[string]any {
	return map[string]any{
		"id": "exe_1", "authorization_id": "aut_1", "checkout_id": "chk_1",
		"external_checkout_id": "ext-1", "checkout_hash": "sha256:abc", "mandate_id": "man_1",
		"amount_minor": 1150, "currency": "USD",
		"funding_method": map[string]any{
			"id": "fmb_1", "provider": "stripe", "merchant_funding_method_id": "mfm_1",
			"provider_method_reference": nil, "brand": "visa", "last_four": "4242",
		},
		"status": "pending", "provider_payment_reference": nil, "failure_reason": nil,
		"idempotency_key": "idem_1", "report_deadline": "2026-01-01T00:30:00Z",
		"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z", "finished_at": nil,
	}
}

func TestGrants_Redeem_Success(t *testing.T) {
	var capturedReq []byte
	var capturedPath string
	g := newTestGrants(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedReq, _ = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(executionFixture())
	})

	exec, err := g.Redeem(context.Background(), RedeemGrantRequest{
		Grant: "enq_gr_secret", ExternalCheckoutID: "ext-1", CheckoutHash: "sha256:abc",
	})
	if err != nil {
		t.Fatalf("Redeem: %v", err)
	}
	if exec.ID != "exe_1" || exec.Status != ExecutionPending || exec.IdempotencyKey != "idem_1" {
		t.Errorf("Redeem = %+v", exec)
	}
	if exec.FundingMethod.Brand != "visa" {
		t.Errorf("Redeem.FundingMethod = %+v", exec.FundingMethod)
	}

	validateRequest(t, http.MethodPost, capturedPath, capturedReq)
	body, _ := json.Marshal(executionFixture())
	validateResponse(t, http.MethodPost, capturedPath, http.StatusCreated, http.Header{"Content-Type": {"application/json"}}, body)
}

func TestGrants_Redeem_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"bad request", http.StatusBadRequest, "validation_failed"},
		{"redemption mismatch", http.StatusConflict, "redemption_mismatch"},
		{"body too large", http.StatusRequestEntityTooLarge, "body_too_large"},
		{"grant invalid", http.StatusUnprocessableEntity, "grant_invalid"},
		{"grant expired", http.StatusUnprocessableEntity, "grant_expired"},
		{"grant revoked", http.StatusUnprocessableEntity, "grant_revoked"},
		{"mandate not active", http.StatusUnprocessableEntity, "mandate_not_active"},
		{"unauthenticated", http.StatusUnauthorized, "unauthenticated"},
		{"internal error", http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := newTestGrants(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := g.Redeem(context.Background(), RedeemGrantRequest{
				Grant: "enq_gr_secret", ExternalCheckoutID: "ext-1", CheckoutHash: "sha256:abc",
			})
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Redeem error code = %q, want %q", code, tt.code)
			}
		})
	}
}
