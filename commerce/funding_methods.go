package commerce

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

// FundingMethodDisplay is what a consent screen shows for a saved
// payment method. LastFour is empty for wallets.
type FundingMethodDisplay struct {
	Brand    string
	LastFour string
}

// FundingMethod binds one of a customer's saved payment methods to
// Enquoir's own opaque ID. Enquoir never sees a card number, CVV, bank
// credentials or PSP secret key.
type FundingMethod struct {
	// ID is Enquoir's binding ID (fmb_...); mandates name it.
	ID         string
	CustomerID string
	// MerchantFundingMethodID is the merchant's own reference for the
	// saved method, when one was given.
	MerchantFundingMethodID string
	// ProviderMethodReference is the PSP's own ID for it, when one was
	// given.
	ProviderMethodReference string
	// Provider is the PSP, e.g. "stripe".
	Provider   string
	Display    FundingMethodDisplay
	Status     CommerceStatus
	CreatedAt  time.Time
	DisabledAt *time.Time
}

type fundingMethodWire struct {
	ID                      string  `json:"id"`
	CustomerID              string  `json:"customer_id"`
	MerchantFundingMethodID *string `json:"merchant_funding_method_id"`
	ProviderMethodReference *string `json:"provider_method_reference"`
	Provider                string  `json:"provider"`
	Display                 struct {
		Brand    string  `json:"brand"`
		LastFour *string `json:"last_four"`
	} `json:"display"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
	DisabledAt *string `json:"disabled_at"`
}

func (w fundingMethodWire) toFundingMethod() (*FundingMethod, error) {
	createdAt, err := parseTime(w.CreatedAt)
	if err != nil {
		return nil, err
	}
	disabledAt, err := parseTimePtr(w.DisabledAt)
	if err != nil {
		return nil, err
	}
	return &FundingMethod{
		ID:                      w.ID,
		CustomerID:              w.CustomerID,
		MerchantFundingMethodID: stringOrEmpty(w.MerchantFundingMethodID),
		ProviderMethodReference: stringOrEmpty(w.ProviderMethodReference),
		Provider:                w.Provider,
		Display: FundingMethodDisplay{
			Brand:    w.Display.Brand,
			LastFour: stringOrEmpty(w.Display.LastFour),
		},
		Status:     CommerceStatus(w.Status),
		CreatedAt:  createdAt,
		DisabledAt: disabledAt,
	}, nil
}

// FundingMethods registers and manages customers' funding-method
// bindings.
type FundingMethods struct {
	transport *transport.Client
}

// NewFundingMethods builds a FundingMethods resource over an existing
// transport. Callers use enquoir.NewMerchantClient instead of this
// directly.
func NewFundingMethods(t *transport.Client) *FundingMethods {
	return &FundingMethods{transport: t}
}

// RegisterFundingMethodRequest binds one of CustomerID's saved payment
// methods. At least one of MerchantFundingMethodID and
// ProviderMethodReference is required; a value shaped like a card
// number (12 or more digits in a row) in either is refused.
type RegisterFundingMethodRequest struct {
	CustomerID              string
	Provider                string
	MerchantFundingMethodID string
	ProviderMethodReference string
	Display                 FundingMethodDisplay
}

// Register registers a funding-method binding, or resolves one already
// registered with the same reference(s). Repeating it with another
// customer, provider or display details - or adding or changing a
// reference - is a conflict, as is a reference that names two
// different bindings.
func (f *FundingMethods) Register(ctx context.Context, req RegisterFundingMethodRequest) (*FundingMethod, error) {
	wire := struct {
		CustomerID              string `json:"customer_id"`
		Provider                string `json:"provider"`
		MerchantFundingMethodID string `json:"merchant_funding_method_id,omitempty"`
		ProviderMethodReference string `json:"provider_method_reference,omitempty"`
		Display                 struct {
			Brand    string `json:"brand"`
			LastFour string `json:"last_four,omitempty"`
		} `json:"display"`
	}{
		CustomerID:              req.CustomerID,
		Provider:                req.Provider,
		MerchantFundingMethodID: req.MerchantFundingMethodID,
		ProviderMethodReference: req.ProviderMethodReference,
	}
	wire.Display.Brand = req.Display.Brand
	wire.Display.LastFour = req.Display.LastFour

	var out fundingMethodWire
	if err := f.transport.Do(ctx, http.MethodPost, "/v1/commerce/funding-methods", wire, &out); err != nil {
		return nil, err
	}
	return out.toFundingMethod()
}

// Get reads one funding-method binding by Enquoir's ID.
func (f *FundingMethods) Get(ctx context.Context, id string) (*FundingMethod, error) {
	var out fundingMethodWire
	path := "/v1/commerce/funding-methods/" + url.PathEscape(id)
	if err := f.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toFundingMethod()
}

// Disable disables a binding, when the customer removes the payment
// method, say. Disabling an already-disabled binding succeeds and
// changes nothing.
func (f *FundingMethods) Disable(ctx context.Context, id string) (*FundingMethod, error) {
	var out fundingMethodWire
	path := "/v1/commerce/funding-methods/" + url.PathEscape(id) + "/disable"
	if err := f.transport.Do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toFundingMethod()
}

// Enable enables a binding.
func (f *FundingMethods) Enable(ctx context.Context, id string) (*FundingMethod, error) {
	var out fundingMethodWire
	path := "/v1/commerce/funding-methods/" + url.PathEscape(id) + "/enable"
	if err := f.transport.Do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toFundingMethod()
}

// FundingMethodListOptions filters and pages ListForCustomer.
type FundingMethodListOptions struct {
	Status CommerceStatus
	Limit  int
	Cursor string
}

func (o FundingMethodListOptions) query() url.Values {
	q := url.Values{}
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

// ListForCustomer lists one customer's funding-method bindings, newest
// first. Another merchant's customer is reported the same as one that
// never existed.
func (f *FundingMethods) ListForCustomer(ctx context.Context, customerID string, opts FundingMethodListOptions) (*Page[FundingMethod], error) {
	path := "/v1/commerce/customers/" + url.PathEscape(customerID) + "/funding-methods"
	if q := opts.query(); len(q) > 0 {
		path += "?" + q.Encode()
	}

	var out struct {
		Data       []fundingMethodWire `json:"data"`
		NextCursor *string             `json:"next_cursor"`
	}
	if err := f.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}

	page := &Page[FundingMethod]{NextCursor: stringOrEmpty(out.NextCursor)}
	for _, w := range out.Data {
		fm, err := w.toFundingMethod()
		if err != nil {
			return nil, err
		}
		page.Data = append(page.Data, *fm)
	}
	return page, nil
}
