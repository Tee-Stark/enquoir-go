package commerce

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

func newTestMerchantAgents(t *testing.T, handler http.HandlerFunc) *MerchantAgents {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewMerchantAgents(transport.New(transport.Config{BaseURL: srv.URL, APIKey: "enq_mk_test"}))
}

func TestMerchantAgents_Register_Success(t *testing.T) {
	var capturedReq []byte
	var capturedPath string
	a := newTestMerchantAgents(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedReq, _ = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"id": "cag_1", "external_id": "ext-1", "name": "Shopper Bot",
			"status": "active", "created_at": "2026-01-01T00:00:00Z", "disabled_at": nil,
		})
	})

	agent, err := a.Register(context.Background(), RegisterMerchantAgentRequest{ExternalID: "ext-1", Name: "Shopper Bot"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if agent.ID != "cag_1" || agent.Name != "Shopper Bot" {
		t.Errorf("Register = %+v", agent)
	}

	validateRequest(t, http.MethodPost, capturedPath, capturedReq)
	validateResponse(t, http.MethodPost, capturedPath, http.StatusCreated, http.Header{"Content-Type": {"application/json"}},
		[]byte(`{"id":"cag_1","external_id":"ext-1","name":"Shopper Bot","status":"active","created_at":"2026-01-01T00:00:00Z","disabled_at":null}`))
}

func TestMerchantAgents_Register_Errors(t *testing.T) {
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
			a := newTestMerchantAgents(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := a.Register(context.Background(), RegisterMerchantAgentRequest{ExternalID: "ext-1", Name: "Shopper Bot"})
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Register error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestMerchantAgents_Get_Success(t *testing.T) {
	var capturedPath string
	a := newTestMerchantAgents(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id": "cag_1", "external_id": "ext-1", "name": "Shopper Bot",
			"status": "active", "created_at": "2026-01-01T00:00:00Z", "disabled_at": nil,
		})
	})

	agent, err := a.Get(context.Background(), "cag_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if agent.ID != "cag_1" {
		t.Errorf("Get = %+v", agent)
	}
	validateRequest(t, http.MethodGet, capturedPath, nil)
}

func TestMerchantAgents_Get_Errors(t *testing.T) {
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
			a := newTestMerchantAgents(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := a.Get(context.Background(), "cag_1")
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Get error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestMerchantAgents_DisableEnable_Success(t *testing.T) {
	for _, tt := range []struct {
		name   string
		call   func(*MerchantAgents) (*MerchantAgent, error)
		suffix string
	}{
		{"disable", func(a *MerchantAgents) (*MerchantAgent, error) { return a.Disable(context.Background(), "cag_1") }, "/disable"},
		{"enable", func(a *MerchantAgents) (*MerchantAgent, error) { return a.Enable(context.Background(), "cag_1") }, "/enable"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var capturedPath string
			a := newTestMerchantAgents(t, func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{
					"id": "cag_1", "external_id": "ext-1", "name": "Shopper Bot",
					"status": "active", "created_at": "2026-01-01T00:00:00Z", "disabled_at": nil,
				})
			})
			if _, err := tt.call(a); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}
			if capturedPath != "/v1/commerce/agents/cag_1"+tt.suffix {
				t.Errorf("path = %q, want suffix %q", capturedPath, tt.suffix)
			}
			validateRequest(t, http.MethodPost, capturedPath, nil)
		})
	}
}

func TestMerchantAgents_List_Success(t *testing.T) {
	a := newTestMerchantAgents(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "cag_1", "external_id": "ext-1", "name": "Shopper Bot", "status": "active", "created_at": "2026-01-01T00:00:00Z", "disabled_at": nil},
			},
			"next_cursor": nil,
		})
	})

	page, err := a.List(context.Background(), MerchantAgentListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Data) != 1 || page.Data[0].ID != "cag_1" || page.NextCursor != "" {
		t.Errorf("List = %+v", page)
	}
}
