package commerce

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

// Customer is one of the merchant's customers. Registering it does not
// grant any spending authority - that is a mandate the customer
// consents to separately.
type Customer struct {
	ID         string
	ExternalID string
	// Email is where Enquoir sends consent links; it is never written
	// to the audit log.
	Email      string
	Status     CommerceStatus
	CreatedAt  time.Time
	DisabledAt *time.Time
}

type customerWire struct {
	ID         string  `json:"id"`
	ExternalID string  `json:"external_id"`
	Email      string  `json:"email"`
	Status     string  `json:"status"`
	CreatedAt  string  `json:"created_at"`
	DisabledAt *string `json:"disabled_at"`
}

func (w customerWire) toCustomer() (*Customer, error) {
	createdAt, err := parseTime(w.CreatedAt)
	if err != nil {
		return nil, err
	}
	disabledAt, err := parseTimePtr(w.DisabledAt)
	if err != nil {
		return nil, err
	}
	return &Customer{
		ID:         w.ID,
		ExternalID: w.ExternalID,
		Email:      w.Email,
		Status:     CommerceStatus(w.Status),
		CreatedAt:  createdAt,
		DisabledAt: disabledAt,
	}, nil
}

// Customers registers and manages the merchant's customers.
type Customers struct {
	transport *transport.Client
}

// NewCustomers builds a Customers resource over an existing transport.
// Callers use enquoir.NewMerchantClient instead of this directly.
func NewCustomers(t *transport.Client) *Customers {
	return &Customers{transport: t}
}

type RegisterCustomerRequest struct {
	ExternalID string
	// Email is lower-cased by the server; where consent links go.
	Email string
}

// Register registers a customer, or resolves one already registered
// with the same external ID and email.
func (c *Customers) Register(ctx context.Context, req RegisterCustomerRequest) (*Customer, error) {
	wire := struct {
		ExternalID string `json:"external_id"`
		Email      string `json:"email"`
	}{ExternalID: req.ExternalID, Email: req.Email}

	var out customerWire
	if err := c.transport.Do(ctx, http.MethodPost, "/v1/commerce/customers", wire, &out); err != nil {
		return nil, err
	}
	return out.toCustomer()
}

// Get reads one customer by Enquoir's ID. Another merchant's customer
// is reported the same as one that never existed.
func (c *Customers) Get(ctx context.Context, id string) (*Customer, error) {
	var out customerWire
	path := "/v1/commerce/customers/" + url.PathEscape(id)
	if err := c.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toCustomer()
}

// Disable disables a customer. Disabling an already-disabled customer
// succeeds and changes nothing.
func (c *Customers) Disable(ctx context.Context, id string) (*Customer, error) {
	var out customerWire
	path := "/v1/commerce/customers/" + url.PathEscape(id) + "/disable"
	if err := c.transport.Do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toCustomer()
}

// Enable enables a customer.
func (c *Customers) Enable(ctx context.Context, id string) (*Customer, error) {
	var out customerWire
	path := "/v1/commerce/customers/" + url.PathEscape(id) + "/enable"
	if err := c.transport.Do(ctx, http.MethodPost, path, nil, &out); err != nil {
		return nil, err
	}
	return out.toCustomer()
}

// CustomerListOptions filters and pages List. Zero values mean "no
// filter" or "use the server's default."
type CustomerListOptions struct {
	// ExternalID, when set, returns only the customer with this exact
	// external ID.
	ExternalID string
	Status     CommerceStatus
	// Limit is the page size, 1 to 200; the server defaults to 50.
	Limit int
	// Cursor is the previous page's Page.NextCursor.
	Cursor string
}

func (o CustomerListOptions) query() url.Values {
	q := url.Values{}
	if o.ExternalID != "" {
		q.Set("external_id", o.ExternalID)
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

// List lists the merchant's customers, newest first.
func (c *Customers) List(ctx context.Context, opts CustomerListOptions) (*Page[Customer], error) {
	path := "/v1/commerce/customers"
	if q := opts.query(); len(q) > 0 {
		path += "?" + q.Encode()
	}

	var out struct {
		Data       []customerWire `json:"data"`
		NextCursor *string        `json:"next_cursor"`
	}
	if err := c.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}

	page := &Page[Customer]{NextCursor: stringOrEmpty(out.NextCursor)}
	for _, w := range out.Data {
		cust, err := w.toCustomer()
		if err != nil {
			return nil, err
		}
		page.Data = append(page.Data, *cust)
	}
	return page, nil
}
