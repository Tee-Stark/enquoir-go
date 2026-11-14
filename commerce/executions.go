package commerce

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

// ExecutionHistoryKind is what one step in an execution's history was.
type ExecutionHistoryKind string

const (
	HistoryRedeemed  ExecutionHistoryKind = "redeemed"
	HistoryReported  ExecutionHistoryKind = "reported"
	HistoryDuplicate ExecutionHistoryKind = "duplicate"
	HistoryIgnored   ExecutionHistoryKind = "ignored"
	HistoryRefused   ExecutionHistoryKind = "refused"
	HistorySwept     ExecutionHistoryKind = "swept"
)

// ReportedOutcome is what a merchant, or a verified PSP webhook,
// reported for an execution.
type ReportedOutcome string

const (
	OutcomeSucceeded ReportedOutcome = "succeeded"
	OutcomeFailed    ReportedOutcome = "failed"
	OutcomeUnknown   ReportedOutcome = "unknown"
)

// ReportResult is what recording a report actually did.
type ReportResult string

const (
	ReportApplied   ReportResult = "applied"
	ReportDuplicate ReportResult = "duplicate"
	// ReportIgnored means the report was older than what is already
	// recorded.
	ReportIgnored ReportResult = "ignored"
)

// ExecutionEvent is one step in an execution's append-only history.
// Source, Outcome, ProviderPaymentReference, ProviderEventID, Currency
// and Detail are empty, and AmountMinor nil, when not applicable to
// that step's Kind.
type ExecutionEvent struct {
	Sequence int64
	Kind     ExecutionHistoryKind
	// Source is "direct", "webhook", "reconciliation" or "sweep".
	Source                   string
	Outcome                  string
	StatusAfter              ExecutionStatus
	ProviderPaymentReference string
	ProviderEventID          string
	AmountMinor              *int64
	Currency                 string
	// Detail is why a report was refused, or the reported failure
	// reason.
	Detail string
	At     time.Time
}

// ExecutionDetail is an execution and every report on it, in order.
type ExecutionDetail struct {
	Execution
	History []ExecutionEvent
}

// ExecutionReportResult is an execution after recording a report
// against it.
type ExecutionReportResult struct {
	Execution
	Result ReportResult
}

type executionEventWire struct {
	Sequence                 int64   `json:"sequence"`
	Kind                     string  `json:"kind"`
	Source                   *string `json:"source"`
	Outcome                  *string `json:"outcome"`
	StatusAfter              string  `json:"status_after"`
	ProviderPaymentReference *string `json:"provider_payment_reference"`
	ProviderEventID          *string `json:"provider_event_id"`
	AmountMinor              *int64  `json:"amount_minor"`
	Currency                 *string `json:"currency"`
	Detail                   *string `json:"detail"`
	At                       string  `json:"at"`
}

func (w executionEventWire) toExecutionEvent() (*ExecutionEvent, error) {
	at, err := parseTime(w.At)
	if err != nil {
		return nil, err
	}
	return &ExecutionEvent{
		Sequence:                 w.Sequence,
		Kind:                     ExecutionHistoryKind(w.Kind),
		Source:                   stringOrEmpty(w.Source),
		Outcome:                  stringOrEmpty(w.Outcome),
		StatusAfter:              ExecutionStatus(w.StatusAfter),
		ProviderPaymentReference: stringOrEmpty(w.ProviderPaymentReference),
		ProviderEventID:          stringOrEmpty(w.ProviderEventID),
		AmountMinor:              w.AmountMinor,
		Currency:                 stringOrEmpty(w.Currency),
		Detail:                   stringOrEmpty(w.Detail),
		At:                       at,
	}, nil
}

type executionDetailWire struct {
	executionWire
	History []executionEventWire `json:"history"`
}

func (w executionDetailWire) toExecutionDetail() (*ExecutionDetail, error) {
	exec, err := w.executionWire.toExecution()
	if err != nil {
		return nil, err
	}
	history := make([]ExecutionEvent, len(w.History))
	for i, ev := range w.History {
		parsed, err := ev.toExecutionEvent()
		if err != nil {
			return nil, err
		}
		history[i] = *parsed
	}
	return &ExecutionDetail{Execution: *exec, History: history}, nil
}

type executionReportResultWire struct {
	executionWire
	Result string `json:"result"`
}

func (w executionReportResultWire) toExecutionReportResult() (*ExecutionReportResult, error) {
	exec, err := w.executionWire.toExecution()
	if err != nil {
		return nil, err
	}
	return &ExecutionReportResult{Execution: *exec, Result: ReportResult(w.Result)}, nil
}

// Executions reads executions and records the merchant's own, or a
// verified PSP webhook's, report of what happened to the one payment
// each permits.
type Executions struct {
	transport *transport.Client
}

// NewExecutions builds an Executions resource over an existing
// transport. Callers use enquoir.NewMerchantClient instead of this
// directly.
func NewExecutions(t *transport.Client) *Executions {
	return &Executions{transport: t}
}

// Get reads one execution and its full history, in order.
func (e *Executions) Get(ctx context.Context, id string) (*ExecutionDetail, error) {
	var out executionDetailWire
	path := "/v1/commerce/executions/" + url.PathEscape(id)
	if err := e.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toExecutionDetail()
}

// ExecutionListOptions filters and pages List. Zero values mean "no
// filter" or "use the server's default." Status ExecutionUnknown is
// the reconciliation queue - executions whose outcome needs a PSP
// lookup and a Reconcile call.
type ExecutionListOptions struct {
	Status    ExecutionStatus
	MandateID string
	Limit     int
	Cursor    string
}

func (o ExecutionListOptions) query() url.Values {
	q := url.Values{}
	if o.Status != "" {
		q.Set("status", string(o.Status))
	}
	if o.MandateID != "" {
		q.Set("mandate_id", o.MandateID)
	}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Cursor != "" {
		q.Set("cursor", o.Cursor)
	}
	return q
}

// List lists the merchant's executions, newest first.
func (e *Executions) List(ctx context.Context, opts ExecutionListOptions) (*Page[Execution], error) {
	path := "/v1/commerce/executions"
	if q := opts.query(); len(q) > 0 {
		path += "?" + q.Encode()
	}

	var out struct {
		Data       []executionWire `json:"data"`
		NextCursor *string         `json:"next_cursor"`
	}
	if err := e.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}

	page := &Page[Execution]{NextCursor: stringOrEmpty(out.NextCursor)}
	for _, w := range out.Data {
		exec, err := w.toExecution()
		if err != nil {
			return nil, err
		}
		page.Data = append(page.Data, *exec)
	}
	return page, nil
}

// ReportOutcomeRequest is the merchant's own report after charging, or
// - with ProviderEventID set - a verified PSP webhook's report. The
// outcome is the merchant's word; Enquoir checks it against the
// execution, but a direct report can never resolve ExecutionUnknown -
// use Reconcile for that.
type ReportOutcomeRequest struct {
	Outcome  ReportedOutcome
	Provider string
	// ProviderPaymentReference is the PSP's payment ID; required when
	// Outcome is OutcomeSucceeded.
	ProviderPaymentReference string
	// ProviderEventID, when set, makes this a verified PSP webhook
	// report instead of a direct one.
	ProviderEventID string
	AmountMinor     int64
	Currency        Currency
	// FailureReason is e.g. the PSP's decline code - never card data.
	FailureReason string
}

// ReportOutcome reports a payment's outcome: your own report after
// charging, or a verified PSP webhook's.
func (e *Executions) ReportOutcome(ctx context.Context, id string, req ReportOutcomeRequest) (*ExecutionReportResult, error) {
	wire := struct {
		Outcome                  string `json:"outcome"`
		Provider                 string `json:"provider"`
		ProviderPaymentReference string `json:"provider_payment_reference,omitempty"`
		ProviderEventID          string `json:"provider_event_id,omitempty"`
		AmountMinor              int64  `json:"amount_minor"`
		Currency                 string `json:"currency"`
		FailureReason            string `json:"failure_reason,omitempty"`
	}{
		Outcome:                  string(req.Outcome),
		Provider:                 req.Provider,
		ProviderPaymentReference: req.ProviderPaymentReference,
		ProviderEventID:          req.ProviderEventID,
		AmountMinor:              req.AmountMinor,
		Currency:                 string(req.Currency),
		FailureReason:            req.FailureReason,
	}

	var out executionReportResultWire
	path := "/v1/commerce/executions/" + url.PathEscape(id) + "/outcome"
	if err := e.transport.Do(ctx, http.MethodPost, path, wire, &out); err != nil {
		return nil, err
	}
	return out.toExecutionReportResult()
}

// ReconcileRequest is what a lookup at the PSP found, to resolve an
// ExecutionUnknown outcome. A finished payment can't be contradicted.
type ReconcileRequest struct {
	Outcome                  ReportedOutcome
	Provider                 string
	ProviderPaymentReference string
	AmountMinor              int64
	Currency                 Currency
	FailureReason            string
}

// Reconcile reports what a PSP lookup found, resolving an
// ExecutionUnknown outcome.
func (e *Executions) Reconcile(ctx context.Context, id string, req ReconcileRequest) (*ExecutionReportResult, error) {
	wire := struct {
		Outcome                  string `json:"outcome"`
		Provider                 string `json:"provider"`
		ProviderPaymentReference string `json:"provider_payment_reference,omitempty"`
		AmountMinor              int64  `json:"amount_minor"`
		Currency                 string `json:"currency"`
		FailureReason            string `json:"failure_reason,omitempty"`
	}{
		Outcome:                  string(req.Outcome),
		Provider:                 req.Provider,
		ProviderPaymentReference: req.ProviderPaymentReference,
		AmountMinor:              req.AmountMinor,
		Currency:                 string(req.Currency),
		FailureReason:            req.FailureReason,
	}

	var out executionReportResultWire
	path := "/v1/commerce/executions/" + url.PathEscape(id) + "/reconcile"
	if err := e.transport.Do(ctx, http.MethodPost, path, wire, &out); err != nil {
		return nil, err
	}
	return out.toExecutionReportResult()
}
