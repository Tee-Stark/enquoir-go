package enquoir

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

func newTestBeneficiaries(t *testing.T, handler http.HandlerFunc) *Beneficiaries {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Beneficiaries{transport: transport.New(transport.Config{BaseURL: srv.URL, APIKey: "enq_sk_test"})}
}

func validRegisterRequest() RegisterBeneficiaryRequest {
	return RegisterBeneficiaryRequest{
		Name:            "Acme Supplier",
		AccountNumber:   "000123456789",
		RoutingNumber:   "021000021",
		AccountCategory: Checking,
		Address: Address{
			Street:   "1 Main St",
			City:     "Seattle",
			State:    "WA",
			Postcode: "98101",
		},
	}
}

func TestBeneficiaries_Register_Success(t *testing.T) {
	var capturedReq []byte
	var capturedPath string
	b := newTestBeneficiaries(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedReq, _ = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": "awx_ben_1", "key": "bnf_abc"})
	})

	resp, err := b.Register(context.Background(), validRegisterRequest())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if resp.ID != "awx_ben_1" || resp.Key != "bnf_abc" {
		t.Errorf("Register response = %+v, want id=awx_ben_1 key=bnf_abc", resp)
	}

	validateRequest(t, http.MethodPost, capturedPath, capturedReq)
	validateResponse(t, http.MethodPost, capturedPath, http.StatusOK, http.Header{"Content-Type": {"application/json"}},
		[]byte(`{"id":"awx_ben_1","key":"bnf_abc"}`))
}

func TestBeneficiaries_Register_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"body too large", http.StatusRequestEntityTooLarge, "body_too_large"},
		{"forbidden", http.StatusForbidden, "email_unverified"},
		{"validation failed", http.StatusBadRequest, "validation_failed"},
		{"unauthenticated", http.StatusUnauthorized, "unauthenticated"},
		{"rail rejected", http.StatusUnprocessableEntity, "rail_rejected"},
		{"internal error", http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newTestBeneficiaries(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := b.Register(context.Background(), validRegisterRequest())
			if !IsCode(err, tt.code) {
				t.Errorf("Register error code = %v, want %q", err, tt.code)
			}
		})
	}
}
