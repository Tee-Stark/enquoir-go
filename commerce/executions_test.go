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

func newTestExecutions(t *testing.T, handler http.HandlerFunc) *Executions {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewExecutions(transport.New(transport.Config{BaseURL: srv.URL, APIKey: "enq_mk_test"}))
}

func executionDetailFixture() map[string]any {
	fixture := executionFixture()
	fixture["history"] = []map[string]any{
		{
			"sequence": 1, "kind": "redeemed", "source": "direct", "outcome": nil,
			"status_after": "pending", "provider_payment_reference": nil, "provider_event_id": nil,
			"amount_minor": nil, "currency": nil, "detail": nil, "at": "2026-01-01T00:00:00Z",
		},
	}
	return fixture
}

func executionReportResultFixture(result string) map[string]any {
	fixture := executionFixture()
	fixture["status"] = "succeeded"
	fixture["result"] = result
	return fixture
}

func TestExecutions_Get_Success(t *testing.T) {
	var capturedPath string
	e := newTestExecutions(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(executionDetailFixture())
	})

	detail, err := e.Get(context.Background(), "exe_1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if detail.ID != "exe_1" || len(detail.History) != 1 || detail.History[0].Kind != HistoryRedeemed {
		t.Errorf("Get = %+v", detail)
	}
	validateRequest(t, http.MethodGet, capturedPath, nil)
	body, _ := json.Marshal(executionDetailFixture())
	validateResponse(t, http.MethodGet, capturedPath, http.StatusOK, http.Header{"Content-Type": {"application/json"}}, body)
}

func TestExecutions_Get_Errors(t *testing.T) {
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
			e := newTestExecutions(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := e.Get(context.Background(), "exe_1")
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Get error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestExecutions_List_Success(t *testing.T) {
	var capturedPath string
	e := newTestExecutions(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path + "?" + r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data":        []map[string]any{executionFixture()},
			"next_cursor": nil,
		})
	})

	page, err := e.List(context.Background(), ExecutionListOptions{Status: ExecutionUnknown, MandateID: "man_1"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(page.Data) != 1 || page.Data[0].ID != "exe_1" {
		t.Errorf("List = %+v", page)
	}
	if capturedPath != "/v1/commerce/executions?mandate_id=man_1&status=unknown" {
		t.Errorf("path = %q", capturedPath)
	}
}

func TestExecutions_ReportOutcome_Success(t *testing.T) {
	var capturedReq []byte
	var capturedPath string
	e := newTestExecutions(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedReq, _ = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(executionReportResultFixture("applied"))
	})

	result, err := e.ReportOutcome(context.Background(), "exe_1", ReportOutcomeRequest{
		Outcome: OutcomeSucceeded, Provider: "stripe", ProviderPaymentReference: "pi_123",
		AmountMinor: 1150, Currency: money.USD,
	})
	if err != nil {
		t.Fatalf("ReportOutcome: %v", err)
	}
	if result.Result != ReportApplied || result.Status != ExecutionSucceeded {
		t.Errorf("ReportOutcome = %+v", result)
	}
	if capturedPath != "/v1/commerce/executions/exe_1/outcome" {
		t.Errorf("path = %q", capturedPath)
	}

	validateRequest(t, http.MethodPost, capturedPath, capturedReq)
	body, _ := json.Marshal(executionReportResultFixture("applied"))
	validateResponse(t, http.MethodPost, capturedPath, http.StatusOK, http.Header{"Content-Type": {"application/json"}}, body)
}

func TestExecutions_ReportOutcome_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"bad request", http.StatusBadRequest, "validation_failed"},
		{"outcome conflict", http.StatusConflict, "outcome_conflict"},
		{"reconciliation required", http.StatusConflict, "reconciliation_required"},
		{"event conflict", http.StatusConflict, "event_conflict"},
		{"body too large", http.StatusRequestEntityTooLarge, "body_too_large"},
		{"report mismatch", http.StatusUnprocessableEntity, "report_mismatch"},
		{"not found", http.StatusNotFound, "not_found"},
		{"unauthenticated", http.StatusUnauthorized, "unauthenticated"},
		{"internal error", http.StatusInternalServerError, "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutions(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := e.ReportOutcome(context.Background(), "exe_1", ReportOutcomeRequest{
				Outcome: OutcomeSucceeded, Provider: "stripe", ProviderPaymentReference: "pi_123",
				AmountMinor: 1150, Currency: money.USD,
			})
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("ReportOutcome error code = %q, want %q", code, tt.code)
			}
		})
	}
}

func TestExecutions_Reconcile_Success(t *testing.T) {
	var capturedReq []byte
	var capturedPath string
	e := newTestExecutions(t, func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedReq, _ = readAll(r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(executionReportResultFixture("applied"))
	})

	result, err := e.Reconcile(context.Background(), "exe_1", ReconcileRequest{
		Outcome: OutcomeSucceeded, Provider: "stripe", ProviderPaymentReference: "pi_123",
		AmountMinor: 1150, Currency: money.USD,
	})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if result.Result != ReportApplied {
		t.Errorf("Reconcile = %+v", result)
	}
	if capturedPath != "/v1/commerce/executions/exe_1/reconcile" {
		t.Errorf("path = %q", capturedPath)
	}

	validateRequest(t, http.MethodPost, capturedPath, capturedReq)
}

func TestExecutions_Reconcile_Errors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		code   string
	}{
		{"bad request", http.StatusBadRequest, "validation_failed"},
		{"outcome conflict", http.StatusConflict, "outcome_conflict"},
		{"report mismatch", http.StatusUnprocessableEntity, "report_mismatch"},
		{"not found", http.StatusNotFound, "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestExecutions(t, problemHandler(tt.status, tt.code, "boom"))
			_, err := e.Reconcile(context.Background(), "exe_1", ReconcileRequest{
				Outcome: OutcomeSucceeded, Provider: "stripe", ProviderPaymentReference: "pi_123",
				AmountMinor: 1150, Currency: money.USD,
			})
			if code := apiErrorCode(err); code != tt.code {
				t.Errorf("Reconcile error code = %q, want %q", code, tt.code)
			}
		})
	}
}
