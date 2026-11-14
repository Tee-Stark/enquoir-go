package commerce

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

func newTestFundingMethods(t *testing.T, handler http.HandlerFunc) *FundingMethods {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewFundingMethods(transport.New(transport.Config{BaseURL: srv.URL, APIKey: "enq_mk_test"}))
}

func fundingMethodFixture() map[string]any {
	return map[string]any{
		"id": "fmb_1", "customer_id": "cus_1",
		"merchant_funding_method_id": "mfm_1", "provider_method_reference": nil,
		"provider": "stripe",
		"display":  map[string]any{"brand": "visa", "last_four": "4242"},
		"status":   "active", "created_at": "2026-01-01T00:00:00Z", "disabled_at": nil,
	}
}

func TestFundingMethods_Register_Success(t *testing.T) {
	var capturedReq []byte
	var capturedPath string
	f := newTestFundingMethods(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedReq, _ = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(fundingMethodFixture())
	})

	fm, err := f.Register(context.Background(), RegisterFundingMethodRequest{
		CustomerID:              "cus_1",
		Provider:                "stripe",
		MerchantFundingMethodID: "mfm_1",
		Display:                 FundingMethodDisplay{Brand: "visa", LastFour: "4242"},
	})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if fm.ID != "fmb_1" || fm.Display.Brand != "visa" || fm.ProviderMethodReference != "" {
		t.Errorf("Register = %+v", fm)
	}

	validateRequest(t, http.MethodPost, capturedPath, capturedReq)
	body, _ := json.Marshal(fundingMethodFixture())
	validateResponse(t, http.MethodPost, capturedPath, http.StatusCreated, http.Header{"Content-Type": {"application/json"}}, body)
}

func TestFundingMethods_Register_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"bad request", http.StatusBadRequest, "validation_failed"},
		{"registration conflict", http.StatusConflict, "registration_conflict"},
		{"body too large", http.StatusRequestEntityTooLarge, "body_too_large"},
		{"customer unknown", http.StatusUnprocessableEntity, "customer_unknown"},
		{"customer disabled", http.StatusUnprocessableEntity, "customer_disabled"},
		{"unauthenticated", http.StatusUnauthorized, "unauthenticated"},
		{"internal error", http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newTestFundingMethods(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := f.Register(context.Background(), RegisterFundingMethodRequest{
				CustomerID: "cus_1", Provider: "stripe", MerchantFundingMethodID: "mfm_1",
				Display: FundingMethodDisplay{Brand: "visa"},
			})
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Register error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestFundingMethods_Get_Success(t *testing.T) {
	var capturedPath string
	f := newTestFundingMethods(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fundingMethodFixture())
	})

	fm, err := f.Get(context.Background(), "fmb_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fm.ID != "fmb_1" {
		t.Errorf("Get = %+v", fm)
	}
	validateRequest(t, http.MethodGet, capturedPath, nil)
}

func TestFundingMethods_Get_Errors(t *testing.T) {
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
			f := newTestFundingMethods(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := f.Get(context.Background(), "fmb_1")
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Get error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestFundingMethods_DisableEnable_Success(t *testing.T) {
	for _, tt := range []struct {
		name   string
		call   func(*FundingMethods) (*FundingMethod, error)
		suffix string
	}{
		{"disable", func(f *FundingMethods) (*FundingMethod, error) { return f.Disable(context.Background(), "fmb_1") }, "/disable"},
		{"enable", func(f *FundingMethods) (*FundingMethod, error) { return f.Enable(context.Background(), "fmb_1") }, "/enable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string
			f := newTestFundingMethods(t, func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(fundingMethodFixture())
			})
			if _, err := tt.call(f); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if capturedPath != "/v1/commerce/funding-methods/fmb_1"+tt.suffix {
				t.Errorf("path = %q, want suffix %q", capturedPath, tt.suffix)
			}
			validateRequest(t, http.MethodPost, capturedPath, nil)
		})
	}
}

func TestFundingMethods_ListForCustomer_Success(t *testing.T) {
	var capturedPath string
	f := newTestFundingMethods(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data":        []map[string]any{fundingMethodFixture()},
			"next_cursor": nil,
		})
	})

	page, err := f.ListForCustomer(context.Background(), "cus_1", FundingMethodListOptions{Status: StatusActive})
	if err != nil {
		t.Fatalf("ListForCustomer: %v", err)
	}
	if len(page.Data) != 1 || page.Data[0].ID != "fmb_1" {
		t.Errorf("ListForCustomer = %+v", page)
	}
	if capturedPath != "/v1/commerce/customers/cus_1/funding-methods" {
		t.Errorf("path = %q", capturedPath)
	}
}

func TestFundingMethods_ListForCustomer_Errors(t *testing.T) {
	f := newTestFundingMethods(t, problemHandler(http.StatusNotFound, "not_found", "boom"))
	_, err := f.ListForCustomer(context.Background(), "cus_missing", FundingMethodListOptions{})
	if code := apiErrorCode(err); code != "not_found" {
		t.Errorf("ListForCustomer error code = %q, want not_found", code)
	}
}
