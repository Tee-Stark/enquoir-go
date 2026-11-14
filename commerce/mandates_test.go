package commerce

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Tee-Stark/enquoir-go/internal/money"
	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

func newTestMandates(t *testing.T, handler http.HandlerFunc) *Mandates {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewMandates(transport.New(transport.Config{BaseURL: srv.URL, APIKey: "enq_mk_test"}))
}

func mandateTermsFixture() map[string]any {
	return map[string]any{
		"merchant":       map[string]any{"id": "mrc_1", "name": "Acme"},
		"customer_id":    "cus_1",
		"agent":          map[string]any{"id": "cag_1", "name": "Shopper Bot"},
		"funding_method": map[string]any{"id": "fmb_1", "brand": "visa", "last_four": "4242"},
		"approval_mode":  "autonomous",
		"scope": map[string]any{
			"currency": "USD", "max_transaction_minor": 10000, "max_cumulative_minor": 100000,
			"max_transactions": 10, "allowed_categories": []string{"electronics"},
		},
		"expires_at": "2026-04-01T00:00:00Z",
		"terms_hash": "sha256:abc",
	}
}

func mandateRequestFixture() map[string]any {
	return map[string]any{
		"id": "mrq_1", "external_id": "ext-1", "status": "pending",
		"terms":      mandateTermsFixture(),
		"created_at": "2026-01-01T00:00:00Z", "expires_at": "2026-01-01T01:00:00Z",
		"decided_at": nil, "mandate_id": nil,
	}
}

func mandateFixture() map[string]any {
	return map[string]any{
		"id": "man_1", "request_id": "mrq_1", "consent_record_id": "con_1",
		"status": "active", "terms": mandateTermsFixture(),
		"approved_at": "2026-01-01T00:05:00Z", "revoked_at": nil, "revocation_expires_at": nil,
		"usage": map[string]any{
			"currency": "USD", "purchases": 2, "purchases_left": 8,
			"spent_minor": 5000, "paid_minor": 4000, "held_minor": 1000, "remaining_minor": 95000,
		},
	}
}

func TestMandates_Request_Success(t *testing.T) {
	var capturedReq []byte
	var capturedPath string
	m := newTestMandates(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedReq, _ = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(mandateRequestFixture())
	})

	req, err := m.Request(context.Background(), RequestMandateRequest{
		ExternalID:      "ext-1",
		CustomerID:      "cus_1",
		AgentID:         "cag_1",
		FundingMethodID: "fmb_1",
		ApprovalMode:    Autonomous,
		Scope: MandateScope{
			Currency: money.USD, MaxTransactionMinor: 10000, MaxCumulativeMinor: 100000,
			MaxTransactions: 10, AllowedCategories: []string{"electronics"},
		},
		ExpiresAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if req.ID != "mrq_1" || req.Status != RequestPending || req.Terms.Scope.Currency != money.USD {
		t.Errorf("Request = %+v", req)
	}
	if req.Terms.FundingMethod.LastFour != "4242" {
		t.Errorf("Request.Terms.FundingMethod = %+v", req.Terms.FundingMethod)
	}

	validateRequest(t, http.MethodPost, capturedPath, capturedReq)
	body, _ := json.Marshal(mandateRequestFixture())
	validateResponse(t, http.MethodPost, capturedPath, http.StatusCreated, http.Header{"Content-Type": {"application/json"}}, body)
}

func TestMandates_Request_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"bad request", http.StatusBadRequest, "validation_failed"},
		{"registration conflict", http.StatusConflict, "registration_conflict"},
		{"body too large", http.StatusRequestEntityTooLarge, "body_too_large"},
		{"agent unknown", http.StatusUnprocessableEntity, "agent_unknown"},
		{"funding method disabled", http.StatusUnprocessableEntity, "funding_method_disabled"},
		{"unauthenticated", http.StatusUnauthorized, "unauthenticated"},
		{"internal error", http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMandates(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := m.Request(context.Background(), RequestMandateRequest{
				ExternalID: "ext-1", CustomerID: "cus_1", AgentID: "cag_1", FundingMethodID: "fmb_1",
				ApprovalMode: Autonomous,
				Scope:        MandateScope{Currency: money.USD, MaxTransactionMinor: 10000, MaxCumulativeMinor: 100000},
				ExpiresAt:    time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			})
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Request error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestMandates_GetRequest_Success(t *testing.T) {
	var capturedPath string
	m := newTestMandates(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mandateRequestFixture())
	})

	req, err := m.GetRequest(context.Background(), "mrq_1")
	if err != nil {
		t.Fatalf("GetRequest: %v", err)
	}
	if req.ID != "mrq_1" {
		t.Errorf("GetRequest = %+v", req)
	}
	validateRequest(t, http.MethodGet, capturedPath, nil)
}

func TestMandates_GetRequest_Errors(t *testing.T) {
	m := newTestMandates(t, problemHandler(http.StatusNotFound, "not_found", "boom"))
	_, err := m.GetRequest(context.Background(), "mrq_missing")
	if code := apiErrorCode(err); code != "not_found" {
		t.Errorf("GetRequest error code = %q, want not_found", code)
	}
}

func TestMandates_Get_Success(t *testing.T) {
	var capturedPath string
	m := newTestMandates(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mandateFixture())
	})

	mandate, err := m.Get(context.Background(), "man_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if mandate.ID != "man_1" || mandate.Status != MandateActive {
		t.Errorf("Get = %+v", mandate)
	}
	if mandate.Usage.PurchasesLeft != 8 || mandate.Usage.SpentMinor != 5000 {
		t.Errorf("Get.Usage = %+v", mandate.Usage)
	}
	validateRequest(t, http.MethodGet, capturedPath, nil)
	body, _ := json.Marshal(mandateFixture())
	validateResponse(t, http.MethodGet, capturedPath, http.StatusOK, http.Header{"Content-Type": {"application/json"}}, body)
}

func TestMandates_Get_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"not found", http.StatusNotFound, "not_found"},
		{"unauthenticated", http.StatusUnauthorized, "unauthenticated"},
		{"internal error", http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestMandates(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := m.Get(context.Background(), "man_1")
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Get error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestMandates_Revoke_Success(t *testing.T) {
	var capturedPath string
	m := newTestMandates(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fixture := mandateFixture()
		fixture["status"] = "revocation_pending"
		expires := "2026-01-01T01:00:00Z"
		fixture["revocation_expires_at"] = expires
		json.NewEncoder(w).Encode(fixture)
	})

	mandate, err := m.Revoke(context.Background(), "man_1")
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if mandate.Status != MandateRevocationPending || mandate.RevocationExpiresAt == nil {
		t.Errorf("Revoke = %+v", mandate)
	}
	if capturedPath != "/v1/commerce/mandates/man_1/revoke" {
		t.Errorf("path = %q", capturedPath)
	}
	validateRequest(t, http.MethodPost, capturedPath, nil)
}

func TestMandates_Revoke_Errors(t *testing.T) {
	m := newTestMandates(t, problemHandler(http.StatusNotFound, "not_found", "boom"))
	_, err := m.Revoke(context.Background(), "man_missing")
	if code := apiErrorCode(err); code != "not_found" {
		t.Errorf("Revoke error code = %q, want not_found", code)
	}
}

func TestMandates_List_Success(t *testing.T) {
	var capturedPath string
	m := newTestMandates(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path + "?" + r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data":        []map[string]any{mandateFixture()},
			"next_cursor": nil,
		})
	})

	page, err := m.List(context.Background(), MandateListOptions{CustomerID: "cus_1", Status: MandateActive})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Data) != 1 || page.Data[0].ID != "man_1" {
		t.Errorf("List = %+v", page)
	}
	if capturedPath != "/v1/commerce/mandates?customer_id=cus_1&status=active" {
		t.Errorf("path = %q", capturedPath)
	}
}
