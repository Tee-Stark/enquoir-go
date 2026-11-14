package enquoir

import (
	"context"
	"net/http"

	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

// AccountCategory is a US bank account's type.
type AccountCategory string

const (
	Checking AccountCategory = "Checking"
	Savings  AccountCategory = "Savings"
)

// Address is a US beneficiary address.
type Address struct {
	Street string
	City   string
	// State is a two-letter US state code, such as "WA".
	State string
	// Postcode is a 5-digit ZIP or ZIP+4.
	Postcode string
}

// Beneficiaries registers payees - US company bank accounts paid in USD
// over ACH - for one organisation.
type Beneficiaries struct {
	transport *transport.Client
}

// RegisterBeneficiaryRequest describes a payee to register. Registering
// the same details again returns the same beneficiary rather than a
// duplicate.
type RegisterBeneficiaryRequest struct {
	Name string
	// AccountNumber is 4 to 17 digits.
	AccountNumber string
	// RoutingNumber is a 9-digit ABA routing number with a valid
	// checksum.
	RoutingNumber   string
	AccountCategory AccountCategory
	Address         Address
}

type RegisterBeneficiaryResponse struct {
	// ID is the rail's beneficiary ID. Use it as a payment's Payee.
	ID string
	// Key is Enquoir's own key for these details.
	Key string
}

// Register registers a payee so this organisation's agents can pay it.
// The payee belongs to the registering organisation; the same details
// registered by another organisation are a separate beneficiary.
func (b *Beneficiaries) Register(ctx context.Context, req RegisterBeneficiaryRequest) (*RegisterBeneficiaryResponse, error) {
	wire := struct {
		Name            string `json:"name"`
		AccountNumber   string `json:"account_number"`
		RoutingNumber   string `json:"routing_number"`
		AccountCategory string `json:"account_category"`
		Address         struct {
			Street   string `json:"street"`
			City     string `json:"city"`
			State    string `json:"state"`
			Postcode string `json:"postcode"`
		} `json:"address"`
	}{
		Name:            req.Name,
		AccountNumber:   req.AccountNumber,
		RoutingNumber:   req.RoutingNumber,
		AccountCategory: string(req.AccountCategory),
	}
	wire.Address.Street = req.Address.Street
	wire.Address.City = req.Address.City
	wire.Address.State = req.Address.State
	wire.Address.Postcode = req.Address.Postcode

	var out struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := b.transport.Do(ctx, http.MethodPost, "/v1/beneficiaries", wire, &out); err != nil {
		return nil, err
	}
	return &RegisterBeneficiaryResponse{ID: out.ID, Key: out.Key}, nil
}
