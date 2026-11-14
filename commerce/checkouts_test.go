package commerce

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tee-Stark/enquoir-go/internal/money"
	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

func newTestCheckouts(t *testing.T, handler http.HandlerFunc) *Checkouts {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewCheckouts(transport.New(transport.Config{BaseURL: srv.URL, APIKey: "enq_mk_test"}))
}

func checkoutSnapshotFixture() map[string]any {
	return map[string]any{
		"customer_id": "cus_1", "agent_id": "cag_1", "funding_method_id": "fmb_1",
		"currency": "USD",
		"items": []map[string]any{
			{"product_id": "prod_1", "name": "Widget", "category": "electronics", "quantity": 2, "unit_price_minor": 500},
		},
		"shipping_minor": 100, "tax_minor": 50, "fees_minor": 0, "total_minor": 1150,
	}
}

func checkoutFixture(status string) map[string]any {
	return map[string]any{
		"id": "chk_1", "external_id": "ext-1", "mandate_id": "man_1",
		"status":              status,
		"decision":            map[string]any{"allowed": status == "authorized", "reasons": []map[string]any{}},
		"checkout":            checkoutSnapshotFixture(),
		"checkout_hash":       "sha256:abc",
		"created_at":          "2026-01-01T00:00:00Z",
		"decided_at":          "2026-01-01T00:00:01Z",
		"approval_expires_at": nil,
		"canceled_at":         nil,
		"authorization":       nil,
	}
}

func TestCheckouts_Submit_Success(t *testing.T) {
	var capturedReq []byte
	var capturedPath string
	c := newTestCheckouts(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedReq, _ = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(checkoutFixture("authorized"))
	})

	checkout, err := c.Submit(context.Background(), SubmitCheckoutRequest{
		ExternalID: "ext-1", MandateID: "man_1", CustomerID: "cus_1", AgentID: "cag_1", FundingMethodID: "fmb_1",
		Lines: CheckoutLines{
			Currency: money.USD, TotalMinor: 1150, ShippingMinor: 100, TaxMinor: 50,
			Items: []CheckoutItem{{ProductID: "prod_1", Name: "Widget", Category: "electronics", Quantity: 2, UnitPriceMinor: 500}},
		},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if checkout.ID != "chk_1" || checkout.Status != CheckoutAuthorized || !checkout.Decision.Allowed {
		t.Errorf("Submit = %+v", checkout)
	}
	if len(checkout.Snapshot.Items) != 1 || checkout.Snapshot.Items[0].ProductID != "prod_1" {
		t.Errorf("Submit.Snapshot.Items = %+v", checkout.Snapshot.Items)
	}

	validateRequest(t, http.MethodPost, capturedPath, capturedReq)
	body, _ := json.Marshal(checkoutFixture("authorized"))
	validateResponse(t, http.MethodPost, capturedPath, http.StatusCreated, http.Header{"Content-Type": {"application/json"}}, body)
}

func TestCheckouts_Submit_Denied(t *testing.T) {
	c := newTestCheckouts(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fixture := checkoutFixture("denied")
		fixture["decision"] = map[string]any{
			"allowed": false,
			"reasons": []map[string]any{{"code": "over_budget", "detail": "exceeds remaining budget"}},
		}
		json.NewEncoder(w).Encode(fixture)
	})

	checkout, err := c.Submit(context.Background(), SubmitCheckoutRequest{
		ExternalID: "ext-1", MandateID: "man_1", CustomerID: "cus_1", AgentID: "cag_1", FundingMethodID: "fmb_1",
		Lines: CheckoutLines{Currency: money.USD, TotalMinor: 1150},
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if checkout.Status != CheckoutDenied || checkout.Decision.Allowed {
		t.Errorf("Submit = %+v", checkout)
	}
	if len(checkout.Decision.Reasons) != 1 || checkout.Decision.Reasons[0].Code != ReasonOverBudget {
		t.Errorf("Submit.Decision.Reasons = %+v", checkout.Decision.Reasons)
	}
}

func TestCheckouts_Submit_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"bad request", http.StatusBadRequest, "validation_failed"},
		{"registration conflict", http.StatusConflict, "registration_conflict"},
		{"body too large", http.StatusRequestEntityTooLarge, "body_too_large"},
		{"mandate unknown", http.StatusUnprocessableEntity, "mandate_unknown"},
		{"unauthenticated", http.StatusUnauthorized, "unauthenticated"},
		{"internal error", http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestCheckouts(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := c.Submit(context.Background(), SubmitCheckoutRequest{
				ExternalID: "ext-1", MandateID: "man_1", CustomerID: "cus_1", AgentID: "cag_1", FundingMethodID: "fmb_1",
				Lines: CheckoutLines{Currency: money.USD, TotalMinor: 1150},
			})
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Submit error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestCheckouts_Get_Success(t *testing.T) {
	var capturedPath string
	c := newTestCheckouts(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(checkoutFixture("authorized"))
	})

	checkout, err := c.Get(context.Background(), "chk_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if checkout.ID != "chk_1" {
		t.Errorf("Get = %+v", checkout)
	}
	validateRequest(t, http.MethodGet, capturedPath, nil)
}

func TestCheckouts_Get_WithAuthorization(t *testing.T) {
	c := newTestCheckouts(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fixture := checkoutFixture("authorized")
		fixture["authorization"] = map[string]any{
			"id": "aut_1", "status": "active", "grant": "enq_gr_secret",
			"grant_issued_at": "2026-01-01T00:00:02Z", "created_at": "2026-01-01T00:00:01Z",
			"expires_at": "2026-01-01T00:15:01Z", "redeemed_at": nil, "revoked_at": nil, "execution_id": nil,
		}
		json.NewEncoder(w).Encode(fixture)
	})

	checkout, err := c.Get(context.Background(), "chk_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if checkout.Authorization == nil || checkout.Authorization.Grant != "enq_gr_secret" {
		t.Errorf("Get.Authorization = %+v", checkout.Authorization)
	}
}

func TestCheckouts_Get_Errors(t *testing.T) {
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
			c := newTestCheckouts(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := c.Get(context.Background(), "chk_1")
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Get error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestCheckouts_Cancel_Success(t *testing.T) {
	var capturedPath string
	c := newTestCheckouts(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(checkoutFixture("canceled"))
	})

	checkout, err := c.Cancel(context.Background(), "chk_1")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if checkout.Status != CheckoutCanceled {
		t.Errorf("Cancel = %+v", checkout)
	}
	if capturedPath != "/v1/commerce/checkouts/chk_1/cancel" {
		t.Errorf("path = %q", capturedPath)
	}
	validateRequest(t, http.MethodPost, capturedPath, nil)
}

func TestCheckouts_Cancel_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"not cancelable", http.StatusConflict, "checkout_not_cancelable"},
		{"not found", http.StatusNotFound, "not_found"},
		{"unauthenticated", http.StatusUnauthorized, "unauthenticated"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestCheckouts(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := c.Cancel(context.Background(), "chk_1")
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Cancel error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestCheckouts_IssueGrant_Success(t *testing.T) {
	var capturedPath string
	c := newTestCheckouts(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fixture := checkoutFixture("authorized")
		fixture["authorization"] = map[string]any{
			"id": "aut_1", "status": "active", "grant": "enq_gr_newsecret",
			"grant_issued_at": "2026-01-01T00:05:00Z", "created_at": "2026-01-01T00:00:01Z",
			"expires_at": "2026-01-01T00:15:01Z", "redeemed_at": nil, "revoked_at": nil, "execution_id": nil,
		}
		json.NewEncoder(w).Encode(fixture)
	})

	checkout, err := c.IssueGrant(context.Background(), "chk_1")
	if err != nil {
		t.Fatalf("IssueGrant: %v", err)
	}
	if checkout.Authorization == nil || checkout.Authorization.Grant != "enq_gr_newsecret" {
		t.Errorf("IssueGrant = %+v", checkout.Authorization)
	}
	if capturedPath != "/v1/commerce/checkouts/chk_1/grant" {
		t.Errorf("path = %q", capturedPath)
	}
}

func TestCheckouts_IssueGrant_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"already redeemed", http.StatusConflict, "grant_redeemed"},
		{"not issuable", http.StatusConflict, "grant_not_issuable"},
		{"not found", http.StatusNotFound, "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestCheckouts(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := c.IssueGrant(context.Background(), "chk_1")
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("IssueGrant error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestCheckouts_List_Success(t *testing.T) {
	var capturedPath string
	c := newTestCheckouts(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path + "?" + r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data":        []map[string]any{checkoutFixture("authorized")},
			"next_cursor": nil,
		})
	})

	page, err := c.List(context.Background(), CheckoutListOptions{MandateID: "man_1", Status: CheckoutAuthorized})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Data) != 1 || page.Data[0].ID != "chk_1" {
		t.Errorf("List = %+v", page)
	}
	if capturedPath != "/v1/commerce/checkouts?mandate_id=man_1&status=authorized" {
		t.Errorf("path = %q", capturedPath)
	}
}
