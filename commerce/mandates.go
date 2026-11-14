package commerce

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

// ApprovalMode is whether purchases within a mandate's scope go ahead
// on their own, or each needs the customer's approval.
type ApprovalMode string

const (
	Autonomous     ApprovalMode = "autonomous"
	PerTransaction ApprovalMode = "per_transaction"
)

// MandateRequestStatus is where a mandate request stands with the
// customer.
type MandateRequestStatus string

const (
	RequestPending  MandateRequestStatus = "pending"
	RequestApproved MandateRequestStatus = "approved"
	RequestRejected MandateRequestStatus = "rejected"
	RequestExpired  MandateRequestStatus = "expired"
)

// MandateStatus is where a mandate stands.
type MandateStatus string

const (
	MandateActive            MandateStatus = "active"
	MandateSuspended         MandateStatus = "suspended"
	MandateRevocationPending MandateStatus = "revocation_pending"
	MandateExpired           MandateStatus = "expired"
	MandateRevoked           MandateStatus = "revoked"
)

// NamedRef is a referenced resource's ID and display name.
type NamedRef struct {
	ID   string
	Name string
}

// MandateScope is a mandate request's proposed spending limits, all in
// integer minor units (cents) to match the API - no floats for money.
type MandateScope struct {
	Currency            Currency
	MaxTransactionMinor int64
	MaxCumulativeMinor  int64
	// MaxTransactions is the most purchases allowed; 0 means no limit.
	MaxTransactions int64
	// AllowedCategories is which product categories are allowed; empty
	// means any.
	AllowedCategories []string
}

// MandateFundingMethod is the funding method a mandate's terms freeze
// in - just enough to show a customer what they approved, not the full
// FundingMethod record.
type MandateFundingMethod struct {
	ID       string
	Brand    string
	LastFour string
}

// MandateTerms is exactly what the customer was asked to approve,
// frozen with the request.
type MandateTerms struct {
	Merchant      NamedRef
	CustomerID    string
	Agent         NamedRef
	FundingMethod MandateFundingMethod
	ApprovalMode  ApprovalMode
	Scope         MandateScope
	ExpiresAt     time.Time
	// TermsHash is "sha256:" and the SHA-256 of the canonical snapshot
	// of these terms.
	TermsHash string
}

// MandateRequest is an ask for a customer's mandate, and their decision
// once made.
type MandateRequest struct {
	ID         string
	ExternalID string
	Status     MandateRequestStatus
	Terms      MandateTerms
	CreatedAt  time.Time
	// ExpiresAt is when the request, and its consent link, stop
	// working (1 hour after it was made).
	ExpiresAt time.Time
	DecidedAt *time.Time
	// MandateID is set once approved.
	MandateID string
}

// MandateUsage is what counts against a mandate - present on Get and
// List, absent (the zero value) elsewhere.
type MandateUsage struct {
	Currency  string
	Purchases int64
	// PurchasesLeft is -1 when the mandate has no purchase limit.
	PurchasesLeft int64
	// SpentMinor is PaidMinor plus HeldMinor.
	SpentMinor int64
	PaidMinor  int64
	// HeldMinor is authorized or redeemed, not yet settled (pending or
	// unknown).
	HeldMinor      int64
	RemainingMinor int64
}

// Mandate is a customer's standing authorization for a merchant's
// agent to spend, within MandateTerms.Scope, from one funding method.
type Mandate struct {
	ID        string
	RequestID string
	// ConsentRecordID is the customer's recorded consent.
	ConsentRecordID string
	Status          MandateStatus
	Terms           MandateTerms
	ApprovedAt      time.Time
	RevokedAt       *time.Time
	// RevocationExpiresAt is set while Status is MandateRevocationPending:
	// when the revocation request lapses unconfirmed and the mandate
	// goes back to active.
	RevocationExpiresAt *time.Time
	// Usage is the zero value unless this Mandate came from Get or List.
	Usage MandateUsage
}

type namedRefWire struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (w namedRefWire) toNamedRef() NamedRef {
	return NamedRef{ID: w.ID, Name: w.Name}
}

type mandateTermsWire struct {
	Merchant      namedRefWire `json:"merchant"`
	CustomerID    string       `json:"customer_id"`
	Agent         namedRefWire `json:"agent"`
	FundingMethod struct {
		ID       string  `json:"id"`
		Brand    string  `json:"brand"`
		LastFour *string `json:"last_four"`
	} `json:"funding_method"`
	ApprovalMode string `json:"approval_mode"`
	Scope        struct {
		Currency            string   `json:"currency"`
		MaxTransactionMinor int64    `json:"max_transaction_minor"`
		MaxCumulativeMinor  int64    `json:"max_cumulative_minor"`
		MaxTransactions     *int64   `json:"max_transactions"`
		AllowedCategories   []string `json:"allowed_categories"`
	} `json:"scope"`
	ExpiresAt string `json:"expires_at"`
	TermsHash string `json:"terms_hash"`
}

func (w mandateTermsWire) toMandateTerms() (*MandateTerms, error) {
	expiresAt, err := parseTime(w.ExpiresAt)
	if err != nil {
		return nil, err
	}
	var maxTransactions int64
	if w.Scope.MaxTransactions != nil {
		maxTransactions = *w.Scope.MaxTransactions
	}
	return &MandateTerms{
		Merchant:   w.Merchant.toNamedRef(),
		CustomerID: w.CustomerID,
		Agent:      w.Agent.toNamedRef(),
		FundingMethod: MandateFundingMethod{
			ID:       w.FundingMethod.ID,
			Brand:    w.FundingMethod.Brand,
			LastFour: stringOrEmpty(w.FundingMethod.LastFour),
		},
		ApprovalMode: ApprovalMode(w.ApprovalMode),
		Scope: MandateScope{
			Currency:            Currency(w.Scope.Currency),
			MaxTransactionMinor: w.Scope.MaxTransactionMinor,
			MaxCumulativeMinor:  w.Scope.MaxCumulativeMinor,
			MaxTransactions:     maxTransactions,
			AllowedCategories:   w.Scope.AllowedCategories,
		},
		ExpiresAt: expiresAt,
		TermsHash: w.TermsHash,
	}, nil
}

type mandateRequestWire struct {
	ID         string           `json:"id"`
	ExternalID string           `json:"external_id"`
	Status     string           `json:"status"`
	Terms      mandateTermsWire `json:"terms"`
	CreatedAt  string           `json:"created_at"`
	ExpiresAt  string           `json:"expires_at"`
	DecidedAt  *string          `json:"decided_at"`
	MandateID  *string          `json:"mandate_id"`
}

func (w mandateRequestWire) toMandateRequest() (*MandateRequest, error) {
	terms, err := w.Terms.toMandateTerms()
	if err != nil {
		return nil, err
	}
	createdAt, err := parseTime(w.CreatedAt)
	if err != nil {
		return nil, err
	}
	expiresAt, err := parseTime(w.ExpiresAt)
	if err != nil {
		return nil, err
	}
	decidedAt, err := parseTimePtr(w.DecidedAt)
	if err != nil {
		return nil, err
	}
	return &MandateRequest{
		ID:         w.ID,
		ExternalID: w.ExternalID,
		Status:     MandateRequestStatus(w.Status),
		Terms:      *terms,
		CreatedAt:  createdAt,
		ExpiresAt:  expiresAt,
		DecidedAt:  decidedAt,
		MandateID:  stringOrEmpty(w.MandateID),
	}, nil
}

type mandateUsageWire struct {
	Currency       string `json:"currency"`
	Purchases      int64  `json:"purchases"`
	PurchasesLeft  *int64 `json:"purchases_left"`
	SpentMinor     int64  `json:"spent_minor"`
	PaidMinor      int64  `json:"paid_minor"`
	HeldMinor      int64  `json:"held_minor"`
	RemainingMinor int64  `json:"remaining_minor"`
}

type mandateWire struct {
	ID                  string            `json:"id"`
	RequestID           string            `json:"request_id"`
	ConsentRecordID     string            `json:"consent_record_id"`
	Status              string            `json:"status"`
	Terms               mandateTermsWire  `json:"terms"`
	ApprovedAt          string            `json:"approved_at"`
	RevokedAt           *string           `json:"revoked_at"`
	RevocationExpiresAt *string           `json:"revocation_expires_at"`
	Usage               *mandateUsageWire `json:"usage"`
}

func (w mandateWire) toMandate() (*Mandate, error) {
	terms, err := w.Terms.toMandateTerms()
	if err != nil {
		return nil, err
	}
	approvedAt, err := parseTime(w.ApprovedAt)
	if err != nil {
		return nil, err
	}
	revokedAt, err := parseTimePtr(w.RevokedAt)
	if err != nil {
		return nil, err
	}
	revocationExpiresAt, err := parseTimePtr(w.RevocationExpiresAt)
	if err != nil {
		return nil, err
	}

	var usage MandateUsage
	if w.Usage != nil {
		purchasesLeft := int64(-1)
		if w.Usage.PurchasesLeft != nil {
			purchasesLeft = *w.Usage.PurchasesLeft
		}
		usage = MandateUsage{
			Currency:       w.Usage.Currency,
			Purchases:      w.Usage.Purchases,
			PurchasesLeft:  purchasesLeft,
			SpentMinor:     w.Usage.SpentMinor,
			PaidMinor:      w.Usage.PaidMinor,
			HeldMinor:      w.Usage.HeldMinor,
			RemainingMinor: w.Usage.RemainingMinor,
		}
	}

	return &Mandate{
		ID:                  w.ID,
		RequestID:           w.RequestID,
		ConsentRecordID:     w.ConsentRecordID,
		Status:              MandateStatus(w.Status),
		Terms:               *terms,
		ApprovedAt:          approvedAt,
		RevokedAt:           revokedAt,
		RevocationExpiresAt: revocationExpiresAt,
		Usage:               usage,
	}, nil
}

// Mandates requests mandates from customers, and reads and revokes the
// ones approved.
type Mandates struct {
	transport *transport.Client
}

// NewMandates builds a Mandates resource over an existing transport.
// Callers use enquoir.NewMerchantClient instead of this directly.
func NewMandates(t *transport.Client) *Mandates {
	return &Mandates{transport: t}
}

// RequestMandateRequest asks one customer to approve spending from one
// of their funding methods, by one of the merchant's agents.
type RequestMandateRequest struct {
	ExternalID      string
	CustomerID      string
	AgentID         string
	FundingMethodID string
	ApprovalMode    ApprovalMode
	Scope           MandateScope
	// ExpiresAt must be in the future, at most 90 days ahead; kept to
	// the second.
	ExpiresAt time.Time
}

// Request asks a customer for a mandate, freezing the terms and
// emailing them a one-time consent link (1 hour). Repeating the same
// external ID with the same terms returns the same request without
// sending another email.
func (m *Mandates) Request(ctx context.Context, req RequestMandateRequest) (*MandateRequest, error) {
	wire := struct {
		ExternalID      string `json:"external_id"`
		CustomerID      string `json:"customer_id"`
		AgentID         string `json:"agent_id"`
		FundingMethodID string `json:"funding_method_id"`
		ApprovalMode    string `json:"approval_mode"`
		Scope           struct {
			Currency            string   `json:"currency"`
			MaxTransactionMinor int64    `json:"max_transaction_minor"`
			MaxCumulativeMinor  int64    `json:"max_cumulative_minor"`
			MaxTransactions     int64    `json:"max_transactions,omitempty"`
			AllowedCategories   []string `json:"allowed_categories,omitempty"`
		} `json:"scope"`
		ExpiresAt string `json:"expires_at"`
	}{
		ExternalID:      req.ExternalID,
		CustomerID:      req.CustomerID,
		AgentID:         req.AgentID,
		FundingMethodID: req.FundingMethodID,
		ApprovalMode:    string(req.ApprovalMode),
		ExpiresAt:       req.ExpiresAt.Format(time.RFC3339),
	}
	wire.Scope.Currency = string(req.Scope.Currency)
	wire.Scope.MaxTransactionMinor = req.Scope.MaxTransactionMinor
	wire.Scope.MaxCumulativeMinor = req.Scope.MaxCumulativeMinor
	wire.Scope.MaxTransactions = req.Scope.MaxTransactions
	wire.Scope.AllowedCategories = req.Scope.AllowedCategories

	var out mandateRequestWire
	if err := m.transport.Do(ctx, http.MethodPost, "/v1/commerce/mandate-requests", wire, &out); err != nil {
		return nil, err
	}
	return out.toMandateRequest()
}

// GetRequest reads a mandate request and the customer's decision, if
// any.
func (m *Mandates) GetRequest(ctx context.Context, id string) (*MandateRequest, error) {
	var out mandateRequestWire
	path := "/v1/commerce/mandate-requests/" + url.PathEscape(id)
	if err := m.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toMandateRequest()
}

// Get reads one mandate by Enquoir's ID.
func (m *Mandates) Get(ctx context.Context, id string) (*Mandate, error) {
	var out mandateWire
	path := "/v1/commerce/mandates/" + url.PathEscape(id)
	if err := m.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toMandate()
}

// Revoke asks to revoke a mandate. The mandate reads revocation_pending
// at once and grants nothing from now; only the customer's confirmation
// (an emailed one-time link, 1 hour) actually revokes it. Asking again
// while one is pending, or after revocation, changes and sends
// nothing.
func (m *Mandates) Revoke(ctx context.Context, id string) (*Mandate, error) {
	var out mandateWire
	path := "/v1/commerce/mandates/" + url.PathEscape(id) + "/revoke"
	if err := m.transport.Do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toMandate()
}

// MandateListOptions filters and pages List. Zero values mean "no
// filter" or "use the server's default."
type MandateListOptions struct {
	CustomerID string
	Status     MandateStatus
	Limit      int
	Cursor     string
}

func (o MandateListOptions) query() url.Values {
	q := url.Values{}
	if o.CustomerID != "" {
		q.Set("customer_id", o.CustomerID)
	}
	if o.Status != "" {
		q.Set("status", string(o.Status))
	}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Cursor != "" {
		q.Set("cursor", o.Cursor)
	}
	return q
}

// List lists the merchant's mandates, newest first - what to show a
// customer who asks what they have allowed.
func (m *Mandates) List(ctx context.Context, opts MandateListOptions) (*Page[Mandate], error) {
	path := "/v1/commerce/mandates"
	if q := opts.query(); len(q) > 0 {
		path += "?" + q.Encode()
	}

	var out struct {
		Data       []mandateWire `json:"data"`
		NextCursor *string       `json:"next_cursor"`
	}
	if err := m.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}

	page := &Page[Mandate]{NextCursor: stringOrEmpty(out.NextCursor)}
	for _, w := range out.Data {
		mandate, err := w.toMandate()
		if err != nil {
			return nil, err
		}
		page.Data = append(page.Data, *mandate)
	}
	return page, nil
}
