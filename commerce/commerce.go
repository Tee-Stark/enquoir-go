// Package commerce is merchant-delegated commerce: customers, their
// agents and funding methods, mandates, checkouts, grants and
// executions - everything a merchant integration key reaches. Build it
// with enquoir.NewMerchantClient; its resources hang off that client,
// not off this package, directly.
package commerce

import (
	"time"

	"github.com/Tee-Stark/enquoir-go/internal/money"
)

// Currency is one of the three currencies Enquoir supports. It is the
// same type as enquoir.Currency (both alias internal/money.Currency),
// so a value built from either package works interchangeably.
type Currency = money.Currency

const (
	USD = money.USD
	EUR = money.EUR
	GBP = money.GBP
)

// CommerceStatus is whether a customer, agent or funding method may
// take part in commerce.
type CommerceStatus string

const (
	StatusActive   CommerceStatus = "active"
	StatusDisabled CommerceStatus = "disabled"
)

// Page is one page of a cursor-paginated list. NextCursor is empty on
// the last page; pass it back as the next call's ListOptions.Cursor to
// continue.
type Page[T any] struct {
	Data       []T
	NextCursor string
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func parseTimePtr(s *string) (*time.Time, error) {
	if s == nil {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
