package commerce

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Tee-Stark/enquoir-go/internal/money"
	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

// CheckoutStatus is where a checkout stands.
type CheckoutStatus string

const (
	CheckoutAuthorized       CheckoutStatus = "authorized"
	CheckoutAwaitingApproval CheckoutStatus = "awaiting_customer_approval"
	CheckoutDenied           CheckoutStatus = "denied"
	CheckoutRejected         CheckoutStatus = "rejected"
	CheckoutCanceled         CheckoutStatus = "canceled"
	// CheckoutExpired means the checkout awaited the customer past
	// ApprovalExpiresAt, or its grant went unredeemed past its
	// authorization's expiry.
	CheckoutExpired CheckoutStatus = "expired"
	// CheckoutRedeemed means its grant was redeemed; payment is
	// pending or unknown.
	CheckoutRedeemed      CheckoutStatus = "redeemed"
	CheckoutRevoked       CheckoutStatus = "revoked"
	CheckoutPaid          CheckoutStatus = "paid"
	CheckoutPaymentFailed CheckoutStatus = "payment_failed"
)

// DecisionReasonCode is why a checkout fell outside its mandate - a
// mandate-level reason (the customer's own consent doesn't allow this)
// or a Policy-prefixed one (the organisation's spending policy
// doesn't, decided before the mandate is ever evaluated). The two
// never appear together on one checkout.
type DecisionReasonCode string

const (
	ReasonMandateNotActive            DecisionReasonCode = "mandate_not_active"
	ReasonCustomerMismatch            DecisionReasonCode = "customer_mismatch"
	ReasonAgentMismatch               DecisionReasonCode = "agent_mismatch"
	ReasonFundingMethodMismatch       DecisionReasonCode = "funding_method_mismatch"
	ReasonCurrencyMismatch            DecisionReasonCode = "currency_mismatch"
	ReasonOverTransactionLimit        DecisionReasonCode = "over_transaction_limit"
	ReasonCategoryNotAllowed          DecisionReasonCode = "category_not_allowed"
	ReasonOverTransactionCount        DecisionReasonCode = "over_transaction_count"
	ReasonOverBudget                  DecisionReasonCode = "over_budget"
	ReasonPolicyOverTransactionLimit  DecisionReasonCode = "policy_over_transaction_limit"
	ReasonPolicyOver24hTotal          DecisionReasonCode = "policy_over_24h_total"
	ReasonPolicyOver7dTotal           DecisionReasonCode = "policy_over_7d_total"
	ReasonPolicyOver30dTotal          DecisionReasonCode = "policy_over_30d_total"
	ReasonPolicyOverHourlyCount       DecisionReasonCode = "policy_over_hourly_count"
	ReasonPolicyPayeeBlocked          DecisionReasonCode = "policy_payee_blocked"
	ReasonPolicyPayeeNotAllowed       DecisionReasonCode = "policy_payee_not_allowed"
	ReasonPolicyCategoryBlocked       DecisionReasonCode = "policy_category_blocked"
	ReasonPolicyNoCurrencyRule        DecisionReasonCode = "policy_no_currency_rule"
	ReasonPolicyOverApprovalThreshold DecisionReasonCode = "policy_over_approval_threshold"
)

// AuthorizationStatus is where a checkout's authorization stands.
type AuthorizationStatus string

const (
	AuthorizationActive   AuthorizationStatus = "active"
	AuthorizationRedeemed AuthorizationStatus = "redeemed"
	AuthorizationExpired  AuthorizationStatus = "expired"
	AuthorizationRevoked  AuthorizationStatus = "revoked"
)

// CheckoutItem is one line item: a product, its category (checked
// against the mandate's allowed categories), quantity and unit price
// in minor units.
type CheckoutItem struct {
	ProductID      string
	Name           string
	Category       string
	Quantity       int64
	UnitPriceMinor int64
}

// CheckoutLines is a checkout's priced contents, in integer minor
// units: TotalMinor must equal the items (quantity times unit price)
// plus ShippingMinor, TaxMinor and FeesMinor, and be positive.
type CheckoutLines struct {
	Currency      money.Currency
	Items         []CheckoutItem
	ShippingMinor int64
	TaxMinor      int64
	FeesMinor     int64
	TotalMinor    int64
}

// CheckoutSnapshot is everything a checkout submission froze: the
// parties named alongside CheckoutLines's priced contents.
type CheckoutSnapshot struct {
	CustomerID      string
	AgentID         string
	FundingMethodID string
	CheckoutLines
}

// DecisionReason is one reason a denied checkout fell outside its
// mandate.
type DecisionReason struct {
	Code   DecisionReasonCode
	Detail string
}

// Decision is a checkout's evaluation result. Allowed is true only
// when Status is CheckoutAuthorized; Reasons lists every way a denied
// checkout fell outside its mandate.
type Decision struct {
	Allowed bool
	Reasons []DecisionReason
}

// Authorization is a checkout's grant of spending authority, once
// authorized.
type Authorization struct {
	ID     string
	Status AuthorizationStatus
	// Grant is the one-time secret (enq_gr_...) - present only in the
	// response that issued or reissued it, never on a later read.
	Grant         string
	GrantIssuedAt *time.Time
	CreatedAt     time.Time
	// ExpiresAt is 15 minutes after the checkout was authorized.
	ExpiresAt   time.Time
	RedeemedAt  *time.Time
	RevokedAt   *time.Time
	ExecutionID string
}

// Checkout is one purchase a merchant's agent proposed, and its
// decision against the named mandate.
type Checkout struct {
	ID           string
	ExternalID   string
	MandateID    string
	Status       CheckoutStatus
	Decision     Decision
	Checkout     CheckoutSnapshot
	CheckoutHash string
	CreatedAt    time.Time
	DecidedAt    *time.Time
	// ApprovalExpiresAt is set only while AwaitingApproval.
	ApprovalExpiresAt *time.Time
	CanceledAt        *time.Time
	// Authorization is nil until the checkout is authorized.
	Authorization *Authorization
}

type checkoutItemWire struct {
	ProductID      string `json:"product_id"`
	Name           string `json:"name"`
	Category       string `json:"category"`
	Quantity       int64  `json:"quantity"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
}

func (w checkoutItemWire) toCheckoutItem() CheckoutItem {
	return CheckoutItem{
		ProductID:      w.ProductID,
		Name:           w.Name,
		Category:       w.Category,
		Quantity:       w.Quantity,
		UnitPriceMinor: w.UnitPriceMinor,
	}
}

type checkoutSnapshotWire struct {
	CustomerID      string             `json:"customer_id"`
	AgentID         string             `json:"agent_id"`
	FundingMethodID string             `json:"funding_method_id"`
	Currency        string             `json:"currency"`
	Items           []checkoutItemWire `json:"items"`
	ShippingMinor   int64              `json:"shipping_minor"`
	TaxMinor        int64              `json:"tax_minor"`
	FeesMinor       int64              `json:"fees_minor"`
	TotalMinor      int64              `json:"total_minor"`
}

func (w checkoutSnapshotWire) toCheckoutSnapshot() CheckoutSnapshot {
	items := make([]CheckoutItem, len(w.Items))
	for i, it := range w.Items {
		items[i] = it.toCheckoutItem()
	}
	return CheckoutSnapshot{
		CustomerID:      w.CustomerID,
		AgentID:         w.AgentID,
		FundingMethodID: w.FundingMethodID,
		CheckoutLines: CheckoutLines{
			Currency:      money.Currency(w.Currency),
			Items:         items,
			ShippingMinor: w.ShippingMinor,
			TaxMinor:      w.TaxMinor,
			FeesMinor:     w.FeesMinor,
			TotalMinor:    w.TotalMinor,
		},
	}
}

type decisionReasonWire struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

type decisionWire struct {
	Allowed bool                 `json:"allowed"`
	Reasons []decisionReasonWire `json:"reasons"`
}

func (w decisionWire) toDecision() Decision {
	reasons := make([]DecisionReason, len(w.Reasons))
	for i, r := range w.Reasons {
		reasons[i] = DecisionReason{Code: DecisionReasonCode(r.Code), Detail: r.Detail}
	}
	return Decision{Allowed: w.Allowed, Reasons: reasons}
}

type authorizationWire struct {
	ID            string  `json:"id"`
	Status        string  `json:"status"`
	Grant         *string `json:"grant"`
	GrantIssuedAt *string `json:"grant_issued_at"`
	CreatedAt     string  `json:"created_at"`
	ExpiresAt     string  `json:"expires_at"`
	RedeemedAt    *string `json:"redeemed_at"`
	RevokedAt     *string `json:"revoked_at"`
	ExecutionID   *string `json:"execution_id"`
}

func (w authorizationWire) toAuthorization() (*Authorization, error) {
	createdAt, err := parseTime(w.CreatedAt)
	if err != nil {
		return nil, err
	}
	expiresAt, err := parseTime(w.ExpiresAt)
	if err != nil {
		return nil, err
	}
	grantIssuedAt, err := parseTimePtr(w.GrantIssuedAt)
	if err != nil {
		return nil, err
	}
	redeemedAt, err := parseTimePtr(w.RedeemedAt)
	if err != nil {
		return nil, err
	}
	revokedAt, err := parseTimePtr(w.RevokedAt)
	if err != nil {
		return nil, err
	}
	return &Authorization{
		ID:            w.ID,
		Status:        AuthorizationStatus(w.Status),
		Grant:         stringOrEmpty(w.Grant),
		GrantIssuedAt: grantIssuedAt,
		CreatedAt:     createdAt,
		ExpiresAt:     expiresAt,
		RedeemedAt:    redeemedAt,
		RevokedAt:     revokedAt,
		ExecutionID:   stringOrEmpty(w.ExecutionID),
	}, nil
}

type checkoutWire struct {
	ID                string               `json:"id"`
	ExternalID        string               `json:"external_id"`
	MandateID         string               `json:"mandate_id"`
	Status            string               `json:"status"`
	Decision          decisionWire         `json:"decision"`
	Checkout          checkoutSnapshotWire `json:"checkout"`
	CheckoutHash      string               `json:"checkout_hash"`
	CreatedAt         string               `json:"created_at"`
	DecidedAt         *string              `json:"decided_at"`
	ApprovalExpiresAt *string              `json:"approval_expires_at"`
	CanceledAt        *string              `json:"canceled_at"`
	Authorization     *authorizationWire   `json:"authorization"`
}

func (w checkoutWire) toCheckout() (*Checkout, error) {
	createdAt, err := parseTime(w.CreatedAt)
	if err != nil {
		return nil, err
	}
	decidedAt, err := parseTimePtr(w.DecidedAt)
	if err != nil {
		return nil, err
	}
	approvalExpiresAt, err := parseTimePtr(w.ApprovalExpiresAt)
	if err != nil {
		return nil, err
	}
	canceledAt, err := parseTimePtr(w.CanceledAt)
	if err != nil {
		return nil, err
	}

	var auth *Authorization
	if w.Authorization != nil {
		auth, err = w.Authorization.toAuthorization()
		if err != nil {
			return nil, err
		}
	}

	return &Checkout{
		ID:                w.ID,
		ExternalID:        w.ExternalID,
		MandateID:         w.MandateID,
		Status:            CheckoutStatus(w.Status),
		Decision:          w.Decision.toDecision(),
		Checkout:          w.Checkout.toCheckoutSnapshot(),
		CheckoutHash:      w.CheckoutHash,
		CreatedAt:         createdAt,
		DecidedAt:         decidedAt,
		ApprovalExpiresAt: approvalExpiresAt,
		CanceledAt:        canceledAt,
		Authorization:     auth,
	}, nil
}

// Checkouts submits checkouts for evaluation against a mandate, and
// reads, cancels and issues grants for them.
type Checkouts struct {
	transport *transport.Client
}

// NewCheckouts builds a Checkouts resource over an existing transport.
// Callers use enquoir.NewMerchantClient instead of this directly.
func NewCheckouts(t *transport.Client) *Checkouts {
	return &Checkouts{transport: t}
}

// SubmitCheckoutRequest is a checkout to evaluate, priced by the
// merchant's own server - never prices an agent asserts.
type SubmitCheckoutRequest struct {
	ExternalID      string
	MandateID       string
	CustomerID      string
	AgentID         string
	FundingMethodID string
	Checkout        CheckoutLines
}

// Submit submits a checkout for evaluation against MandateID. The
// decision is recorded and returned whether it passed or not - only a
// mandate that is not this merchant's returns an error; every other
// failure is a recorded CheckoutDenied checkout with its Decision.Reasons.
func (c *Checkouts) Submit(ctx context.Context, req SubmitCheckoutRequest) (*Checkout, error) {
	wire := struct {
		ExternalID      string `json:"external_id"`
		MandateID       string `json:"mandate_id"`
		CustomerID      string `json:"customer_id"`
		AgentID         string `json:"agent_id"`
		FundingMethodID string `json:"funding_method_id"`
		Checkout        struct {
			Currency      string             `json:"currency"`
			Items         []checkoutItemWire `json:"items"`
			ShippingMinor int64              `json:"shipping_minor"`
			TaxMinor      int64              `json:"tax_minor"`
			FeesMinor     int64              `json:"fees_minor"`
			TotalMinor    int64              `json:"total_minor"`
		} `json:"checkout"`
	}{
		ExternalID:      req.ExternalID,
		MandateID:       req.MandateID,
		CustomerID:      req.CustomerID,
		AgentID:         req.AgentID,
		FundingMethodID: req.FundingMethodID,
	}
	wire.Checkout.Currency = string(req.Checkout.Currency)
	wire.Checkout.ShippingMinor = req.Checkout.ShippingMinor
	wire.Checkout.TaxMinor = req.Checkout.TaxMinor
	wire.Checkout.FeesMinor = req.Checkout.FeesMinor
	wire.Checkout.TotalMinor = req.Checkout.TotalMinor
	wire.Checkout.Items = make([]checkoutItemWire, len(req.Checkout.Items))
	for i, it := range req.Checkout.Items {
		wire.Checkout.Items[i] = checkoutItemWire{
			ProductID:      it.ProductID,
			Name:           it.Name,
			Category:       it.Category,
			Quantity:       it.Quantity,
			UnitPriceMinor: it.UnitPriceMinor,
		}
	}

	var out checkoutWire
	if err := c.transport.Do(ctx, http.MethodPost, "/v1/commerce/checkouts", wire, &out); err != nil {
		return nil, err
	}
	return out.toCheckout()
}

// Get reads one checkout and its decision by Enquoir's ID.
func (c *Checkouts) Get(ctx context.Context, id string) (*Checkout, error) {
	var out checkoutWire
	path := "/v1/commerce/checkouts/" + url.PathEscape(id)
	if err := c.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toCheckout()
}

// Cancel cancels an authorized checkout, or one awaiting the customer,
// releasing what it counted against the mandate. Cancelling a
// cancelled checkout succeeds and changes nothing.
func (c *Checkouts) Cancel(ctx context.Context, id string) (*Checkout, error) {
	var out checkoutWire
	path := "/v1/commerce/checkouts/" + url.PathEscape(id) + "/cancel"
	if err := c.transport.Do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toCheckout()
}

// IssueGrant issues the checkout's first grant, or reissues a
// replacement for a lost one - the earlier secret stops working, and
// the new one's expiry stays the authorization's. The returned
// Checkout's Authorization.Grant holds the secret; it is never shown
// again after this response.
func (c *Checkouts) IssueGrant(ctx context.Context, id string) (*Checkout, error) {
	var out checkoutWire
	path := "/v1/commerce/checkouts/" + url.PathEscape(id) + "/grant"
	if err := c.transport.Do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toCheckout()
}

// CheckoutListOptions filters and pages List.
type CheckoutListOptions struct {
	MandateID  string
	CustomerID string
	Status     CheckoutStatus
	Limit      int
	Cursor     string
}

func (o CheckoutListOptions) query() url.Values {
	q := url.Values{}
	if o.MandateID != "" {
		q.Set("mandate_id", o.MandateID)
	}
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

// List lists the merchant's checkouts, newest first.
func (c *Checkouts) List(ctx context.Context, opts CheckoutListOptions) (*Page[Checkout], error) {
	path := "/v1/commerce/checkouts"
	if q := opts.query(); len(q) > 0 {
		path += "?" + q.Encode()
	}

	var out struct {
		Data       []checkoutWire `json:"data"`
		NextCursor *string        `json:"next_cursor"`
	}
	if err := c.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}

	page := &Page[Checkout]{NextCursor: stringOrEmpty(out.NextCursor)}
	for _, w := range out.Data {
		checkout, err := w.toCheckout()
		if err != nil {
			return nil, err
		}
		page.Data = append(page.Data, *checkout)
	}
	return page, nil
}
