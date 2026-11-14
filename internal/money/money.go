// Package money is Enquoir's Currency type, shared by the root
// package and commerce so neither has to duplicate it or import the
// other. Both re-export it as a type alias, so a caller never imports
// this package directly.
package money

// Currency is one of the three currencies Enquoir supports.
type Currency string

const (
	USD Currency = "USD"
	EUR Currency = "EUR"
	GBP Currency = "GBP"
)
