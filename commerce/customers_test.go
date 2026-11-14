package commerce

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

func newTestCustomers(t *testing.T, handler http.HandlerFunc) *Customers {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewCustomers(transport.New(transport.Config{BaseURL: srv.URL, APIKey: "enq_mk_test"}))
}

func TestCustomers_Register_Success(t *testing.T) {
	var capturedReq []byte
	var capturedPath string
	c := newTestCustomers(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedReq, _ = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "cus_1", "external_id": "ext-1", "email": "a@example.com",
			"status": "active", "created_at": "2026-01-01T00:00:00Z", "disabled_at": nil,
		})
	})

	cust, err := c.Register(context.Background(), RegisterCustomerRequest{ExternalID: "ext-1", Email: "a@example.com"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if cust.ID != "cus_1" || cust.Status != StatusActive || cust.DisabledAt != nil {
		t.Errorf("Register = %+v", cust)
	}

	validateRequest(t, http.MethodPost, capturedPath, capturedReq)
	validateResponse(t, http.MethodPost, capturedPath, http.StatusCreated, http.Header{"Content-Type": {"application/json"}},
		[]byte(`{"id":"cus_1","external_id":"ext-1","email":"a@example.com","status":"active","created_at":"2026-01-01T00:00:00Z","disabled_at":null}`))
}

func TestCustomers_Register_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"bad request", http.StatusBadRequest, "validation_failed"},
		{"registration conflict", http.StatusConflict, "registration_conflict"},
		{"body too large", http.StatusRequestEntityTooLarge, "body_too_large"},
		{"unauthenticated", http.StatusUnauthorized, "unauthenticated"},
		{"internal error", http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestCustomers(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := c.Register(context.Background(), RegisterCustomerRequest{ExternalID: "ext-1", Email: "a@example.com"})
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Register error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestCustomers_Get_Success(t *testing.T) {
	var capturedPath string
	c := newTestCustomers(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "cus_1", "external_id": "ext-1", "email": "a@example.com",
			"status": "active", "created_at": "2026-01-01T00:00:00Z", "disabled_at": nil,
		})
	})

	cust, err := c.Get(context.Background(), "cus_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if cust.ID != "cus_1" {
		t.Errorf("Get = %+v", cust)
	}
	validateRequest(t, http.MethodGet, capturedPath, nil)
}

func TestCustomers_Get_Errors(t *testing.T) {
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
			c := newTestCustomers(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := c.Get(context.Background(), "cus_1")
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Get error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestCustomers_DisableEnable_Success(t *testing.T) {
	for _, tt := range []struct {
		name   string
		call   func(*Customers) (*Customer, error)
		suffix string
	}{
		{"disable", func(c *Customers) (*Customer, error) { return c.Disable(context.Background(), "cus_1") }, "/disable"},
		{"enable", func(c *Customers) (*Customer, error) { return c.Enable(context.Background(), "cus_1") }, "/enable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string
			c := newTestCustomers(t, func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{
					"id": "cus_1", "external_id": "ext-1", "email": "a@example.com",
					"status": "active", "created_at": "2026-01-01T00:00:00Z", "disabled_at": nil,
				})
			})
			if _, err := tt.call(c); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if capturedPath != "/v1/commerce/customers/cus_1"+tt.suffix {
				t.Errorf("path = %q, want suffix %q", capturedPath, tt.suffix)
			}
			validateRequest(t, http.MethodPost, capturedPath, nil)
		})
	}
}

func TestCustomers_List_Success(t *testing.T) {
	var capturedPath string
	c := newTestCustomers(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path + "?" + r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "cus_1", "external_id": "ext-1", "email": "a@example.com", "status": "active", "created_at": "2026-01-01T00:00:00Z", "disabled_at": nil},
			},
			"next_cursor": "cur_abc",
		})
	})

	page, err := c.List(context.Background(), CustomerListOptions{Status: StatusActive, Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Data) != 1 || page.Data[0].ID != "cus_1" || page.NextCursor != "cur_abc" {
		t.Errorf("List = %+v", page)
	}
	if capturedPath != "/v1/commerce/customers?limit=10&status=active" && capturedPath != "/v1/commerce/customers?status=active&limit=10" {
		t.Errorf("path = %q", capturedPath)
	}
}

// apiErrorCode extracts a *transport.Error's Code, the same shape
// enquoir.IsCode checks, without importing the root package (which
// would create an import cycle: root imports commerce).
func apiErrorCode(err error) string {
	if apiErr, ok := err.(*transport.Error); ok {
		return apiErr.Code
	}
	return ""
}
