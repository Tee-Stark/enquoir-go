package commerce

import (
	"context"
	"net/http"
	"time"

	"github.com/Tee-Stark/enquoir-go/internal/money"
	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

// ExecutionStatus is a redeemed grant's one payment's outcome - the
// merchant's own word, checked against the execution.
type ExecutionStatus string

const (
	ExecutionPending   ExecutionStatus = "pending"
	ExecutionSucceeded ExecutionStatus = "succeeded"
	ExecutionFailed    ExecutionStatus = "failed"
	ExecutionUnknown   ExecutionStatus = "unknown"
)

// ExecutionFundingMethod is the funding method an execution charges -
// just enough to identify it, not the full FundingMethod record.
type ExecutionFundingMethod struct {
	ID                      string
	Provider                string
	MerchantFundingMethodID string
	ProviderMethodReference string
	Brand                   string
	LastFour                string
}

// Execution is the one payment a redeemed grant permits. Status is the
// merchant's own reported outcome (see Executions.ReportOutcome, added
// alongside the Executions resource), never something Enquoir verifies
// with the PSP itself.
type Execution struct {
	ID                       string
	AuthorizationID          string
	CheckoutID               string
	ExternalCheckoutID       string
	CheckoutHash             string
	MandateID                string
	AmountMinor              int64
	Currency                 money.Currency
	FundingMethod            ExecutionFundingMethod
	Status                   ExecutionStatus
	ProviderPaymentReference string
	FailureReason            string
	// IdempotencyKey: send it as your PSP request's own idempotency
	// key, so a retried charge can't charge twice.
	IdempotencyKey string
	// ReportDeadline: still pending past this, the execution is
	// presumed unknown.
	ReportDeadline time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	FinishedAt     *time.Time
}

type executionFundingMethodWire struct {
	ID                      string  `json:"id"`
	Provider                string  `json:"provider"`
	MerchantFundingMethodID *string `json:"merchant_funding_method_id"`
	ProviderMethodReference *string `json:"provider_method_reference"`
	Brand                   string  `json:"brand"`
	LastFour                *string `json:"last_four"`
}

type executionWire struct {
	ID                       string                     `json:"id"`
	AuthorizationID          string                     `json:"authorization_id"`
	CheckoutID               string                     `json:"checkout_id"`
	ExternalCheckoutID       string                     `json:"external_checkout_id"`
	CheckoutHash             string                     `json:"checkout_hash"`
	MandateID                string                     `json:"mandate_id"`
	AmountMinor              int64                      `json:"amount_minor"`
	Currency                 string                     `json:"currency"`
	FundingMethod            executionFundingMethodWire `json:"funding_method"`
	Status                   string                     `json:"status"`
	ProviderPaymentReference *string                    `json:"provider_payment_reference"`
	FailureReason            *string                    `json:"failure_reason"`
	IdempotencyKey           string                     `json:"idempotency_key"`
	ReportDeadline           string                     `json:"report_deadline"`
	CreatedAt                string                     `json:"created_at"`
	UpdatedAt                string                     `json:"updated_at"`
	FinishedAt               *string                    `json:"finished_at"`
}

func (w executionWire) toExecution() (*Execution, error) {
	reportDeadline, err := parseTime(w.ReportDeadline)
	if err != nil {
		return nil, err
	}
	createdAt, err := parseTime(w.CreatedAt)
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseTime(w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	finishedAt, err := parseTimePtr(w.FinishedAt)
	if err != nil {
		return nil, err
	}

	return &Execution{
		ID:                 w.ID,
		AuthorizationID:    w.AuthorizationID,
		CheckoutID:         w.CheckoutID,
		ExternalCheckoutID: w.ExternalCheckoutID,
		CheckoutHash:       w.CheckoutHash,
		MandateID:          w.MandateID,
		AmountMinor:        w.AmountMinor,
		Currency:           money.Currency(w.Currency),
		FundingMethod: ExecutionFundingMethod{
			ID:                      w.FundingMethod.ID,
			Provider:                w.FundingMethod.Provider,
			MerchantFundingMethodID: stringOrEmpty(w.FundingMethod.MerchantFundingMethodID),
			ProviderMethodReference: stringOrEmpty(w.FundingMethod.ProviderMethodReference),
			Brand:                   w.FundingMethod.Brand,
			LastFour:                stringOrEmpty(w.FundingMethod.LastFour),
		},
		Status:                   ExecutionStatus(w.Status),
		ProviderPaymentReference: stringOrEmpty(w.ProviderPaymentReference),
		FailureReason:            stringOrEmpty(w.FailureReason),
		IdempotencyKey:           w.IdempotencyKey,
		ReportDeadline:           reportDeadline,
		CreatedAt:                createdAt,
		UpdatedAt:                updatedAt,
		FinishedAt:               finishedAt,
	}, nil
}

// Grants redeems one-time checkout grants before charging.
type Grants struct {
	transport *transport.Client
}

// NewGrants builds a Grants resource over an existing transport.
// Callers use enquoir.NewMerchantClient instead of this directly.
func NewGrants(t *transport.Client) *Grants {
	return &Grants{transport: t}
}

// RedeemGrantRequest names the grant to redeem and the checkout it was
// issued for - both required, and checked against each other.
type RedeemGrantRequest struct {
	// Grant is the enq_gr_... secret from Checkouts.IssueGrant.
	Grant              string
	ExternalCheckoutID string
	CheckoutHash       string
}

// Redeem redeems a grant before charging: checks it is this
// merchant's, unredeemed, unrevoked, unexpired, for this checkout and
// version, and its mandate still active, then creates the one
// execution. Repeating it - even after the grant's 15-minute window -
// returns that same execution, never a second.
func (g *Grants) Redeem(ctx context.Context, req RedeemGrantRequest) (*Execution, error) {
	wire := struct {
		Grant              string `json:"grant"`
		ExternalCheckoutID string `json:"external_checkout_id"`
		CheckoutHash       string `json:"checkout_hash"`
	}{
		Grant:              req.Grant,
		ExternalCheckoutID: req.ExternalCheckoutID,
		CheckoutHash:       req.CheckoutHash,
	}

	var out executionWire
	if err := g.transport.Do(ctx, http.MethodPost, "/v1/commerce/grants/redeem", wire, &out); err != nil {
		return nil, err
	}
	return out.toExecution()
}
