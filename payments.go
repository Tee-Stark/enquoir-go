package enquoir

import (
	"context"
	"net/http"
	"net/url"

	"github.com/Tee-Stark/enquoir-go/internal/money"
	"github.com/Tee-Stark/enquoir-go/internal/transport"
)

// Currency is one of the three currencies Enquoir supports.
type Currency = money.Currency

const (
	USD = money.USD
	EUR = money.EUR
	GBP = money.GBP
)

// PaymentState is where a payment, or one step of its history, stands in
// its lifecycle. APPLIED and BLOCKED are final; FAILED is deliberately
// not - a reconciler may still re-authorize a failed attempt, or give
// up and block it.
type PaymentState string

const (
	StateProposed         PaymentState = "PROPOSED"
	StateAwaitingApproval PaymentState = "AWAITING_APPROVAL"
	StateAuthorized       PaymentState = "AUTHORIZED"
	StateInFlight         PaymentState = "IN_FLIGHT"
	StateUnknown          PaymentState = "UNKNOWN"
	StateApplied          PaymentState = "APPLIED"
	StateFailed           PaymentState = "FAILED"
	StateBlocked          PaymentState = "BLOCKED"
)

// Terminal reports whether nothing more will happen to a payment in
// this state. Only APPLIED and BLOCKED are terminal - notably not
// FAILED, which can still move on. A poll loop should stop on this,
// not on a hand-picked set of states.
func (s PaymentState) Terminal() bool {
	return s == StateApplied || s == StateBlocked
}

// SubmitOutcome says what a Submit call actually did.
type SubmitOutcome string

const (
	// OutcomeStarted means a new payment began.
	OutcomeStarted SubmitOutcome = "started"
	// OutcomeAttached means the same intent was submitted moments ago and
	// its run is starting; this call joined it.
	OutcomeAttached SubmitOutcome = "attached"
	// OutcomeDeduplicated means the intent already has a recorded
	// history; nothing new started, State is where it stands.
	OutcomeDeduplicated SubmitOutcome = "deduplicated"
)

// Payments submits treasury payment intents and reads their state and
// history, for one agent.
type Payments struct {
	transport *transport.Client
}

// SubmitIntentRequest states a payment to make. Submitting the same
// intent again - same Payee, Amount, Currency and Reference - returns
// the same key and starts nothing new, however long after the first
// submission; Reason is recorded but not part of that key, so rewording
// it on a retry still joins the original payment.
type SubmitIntentRequest struct {
	// Payee is a beneficiary ID from Beneficiaries.Register.
	Payee string
	// Amount is major units as a decimal string with at most two
	// decimals, e.g. "10.50".
	Amount   string
	Currency Currency
	// Reference is the caller's own reference, such as an invoice number.
	Reference string
	// Reason is free text, recorded but not part of the intent key.
	Reason string
}

type SubmitIntentResponse struct {
	// Key is the intent key. The same intent always gets the same key.
	Key     string
	State   PaymentState
	Outcome SubmitOutcome
}

// Submit states a payment intent. Enquoir derives a stable key from its
// payee, amount, currency and reference, checks policy, and executes it
// through a durable workflow - this call only starts or joins that; use
// Get to follow it to a terminal state.
func (p *Payments) Submit(ctx context.Context, req SubmitIntentRequest) (*SubmitIntentResponse, error) {
	wire := struct {
		Payee     string `json:"payee"`
		Amount    string `json:"amount"`
		Currency  string `json:"currency"`
		Reference string `json:"reference"`
		Reason    string `json:"reason,omitempty"`
	}{
		Payee:     req.Payee,
		Amount:    req.Amount,
		Currency:  string(req.Currency),
		Reference: req.Reference,
		Reason:    req.Reason,
	}

	var out struct {
		Key     string `json:"key"`
		State   string `json:"state"`
		Outcome string `json:"outcome"`
	}
	if err := p.transport.Do(ctx, http.MethodPost, "/v1/intents", wire, &out); err != nil {
		return nil, err
	}
	return &SubmitIntentResponse{
		Key:     out.Key,
		State:   PaymentState(out.State),
		Outcome: SubmitOutcome(out.Outcome),
	}, nil
}

// Event is one step in an intent's hash-chained history.
type Event struct {
	SequenceNumber int
	Type           PaymentState
	Payload        string
}

// IntentDetail is an intent's current state and full history - enough to
// poll a payment to completion, not the richer record the dashboard
// shows (that view is session-only).
type IntentDetail struct {
	Key    string
	State  PaymentState
	Events []Event
}

// Get reads an intent's state and history by its key. A key belonging to
// another organisation is reported the same as one that never existed.
func (p *Payments) Get(ctx context.Context, key string) (*IntentDetail, error) {
	var out struct {
		Key    string `json:"key"`
		State  string `json:"state"`
		Events []struct {
			SequenceNumber int    `json:"sequence_number"`
			Type           string `json:"type"`
			Payload        string `json:"payload"`
		} `json:"events"`
	}
	path := "/v1/intents/" + url.PathEscape(key)
	if err := p.transport.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}

	detail := &IntentDetail{Key: out.Key, State: PaymentState(out.State)}
	for _, e := range out.Events {
		detail.Events = append(detail.Events, Event{
			SequenceNumber: e.SequenceNumber,
			Type:           PaymentState(e.Type),
			Payload:        e.Payload,
		})
	}
	return detail, nil
}
