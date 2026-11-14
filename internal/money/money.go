// Package money is Enquoir's Currency and Money types, shared by the
// root package and commerce so neither has to duplicate them or import
// the other. The root package and commerce both re-export Currency as
// a type alias, so a caller never imports this package directly.
package money

// Currency is one of the three currencies Enquoir supports.
type Currency string

const (
	USD Currency = "USD"
	EUR Currency = "EUR"
	GBP Currency = "GBP"
)

// Money carries an amount exactly as the wire sent it: integer minor
// units alongside the decimal string the API also sends, never
// reformatted or re-derived - there is no safe reason for the SDK to
// touch money's own representation.
type Money struct {
	MinorUnits int64
	Currency   Currency
	// Amount is major units as a decimal string, e.g. "10.50".
	Amount string
}

// Wire is Money's exact JSON shape, exported so other packages can
// embed it in their own wire structs without redeclaring the fields.
type Wire struct {
	MinorUnits int64  `json:"minor_units"`
	Currency   string `json:"currency"`
	Amount     string `json:"amount"`
}

func (w Wire) ToMoney() Money {
	return Money{MinorUnits: w.MinorUnits, Currency: Currency(w.Currency), Amount: w.Amount}
}
