// Package enquoir is a Go client for the Enquoir API. TreasuryClient
// submits treasury payments with an agent key; MerchantClient runs
// merchant-delegated commerce with a merchant key. Treasury and commerce
// keys are not interchangeable server-side, so each client type only
// offers what its key can do - a caller never sees a method their key
// would be refused on.
package enquoir

import (
	"net/http"
	"time"

	"github.com/Tee-Stark/enquoir-go/commerce"
	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

// defaultBaseURL is Enquoir's local dev API. Every other environment,
// including any hosted one, must be set explicitly with WithBaseURL -
// there is no production default to guess at yet.
const defaultBaseURL = "http://localhost:8090"

// Option configures a TreasuryClient or MerchantClient.
type Option func(*transport.Config)

// WithBaseURL overrides the default local dev API address.
func WithBaseURL(url string) Option {
	return func(c *transport.Config) { c.BaseURL = url }
}

// WithHTTPClient overrides the default http.Client, for custom transports,
// proxying, or test doubles.
func WithHTTPClient(client *http.Client) Option {
	return func(c *transport.Config) { c.HTTPClient = client }
}

// WithTimeout sets a per-request timeout. Ignored when WithHTTPClient is
// also given, since that client's own timeout applies instead.
func WithTimeout(d time.Duration) Option {
	return func(c *transport.Config) { c.Timeout = d }
}

func newConfig(apiKey string, opts []Option) transport.Config {
	cfg := transport.Config{
		BaseURL: defaultBaseURL,
		APIKey:  apiKey,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// TreasuryClient submits payments and registers beneficiaries for one
// agent - everything an agent key (enq_sk_...) can reach.
type TreasuryClient struct {
	transport *transport.Client

	Payments      *Payments
	Beneficiaries *Beneficiaries
}

// NewTreasuryClient builds a TreasuryClient authenticating with the given
// agent key.
func NewTreasuryClient(apiKey string, opts ...Option) *TreasuryClient {
	t := transport.New(newConfig(apiKey, opts))
	return &TreasuryClient{
		transport:     t,
		Payments:      &Payments{transport: t},
		Beneficiaries: &Beneficiaries{transport: t},
	}
}

// MerchantClient runs merchant-delegated commerce for one merchant
// integration - everything a merchant key (enq_mk_...) can reach.
type MerchantClient struct {
	transport *transport.Client

	Customers      *commerce.Customers
	MerchantAgents *commerce.MerchantAgents
	FundingMethods *commerce.FundingMethods
}

// NewMerchantClient builds a MerchantClient authenticating with the given
// merchant integration key.
func NewMerchantClient(apiKey string, opts ...Option) *MerchantClient {
	t := transport.New(newConfig(apiKey, opts))
	return &MerchantClient{
		transport:      t,
		Customers:      commerce.NewCustomers(t),
		MerchantAgents: commerce.NewMerchantAgents(t),
		FundingMethods: commerce.NewFundingMethods(t),
	}
}
