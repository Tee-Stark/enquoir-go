# enquoir-go

A Go client for [Enquoir](https://github.com/Tee-Stark/enquoir), the safe
control and observability layer for agentic finance.

```go
import "github.com/Tee-Stark/enquoir-go"
```

Two client types, one per API key:

- **`TreasuryClient`** - an agent key (`enq_sk_...`). Submits payment intents
  and registers the beneficiaries they pay.
- **`MerchantClient`** - a merchant integration key (`enq_mk_...`).
  Merchant-delegated commerce: customers, agents, funding methods, mandates,
  checkouts, grants and executions.

A `TreasuryClient` never exposes a commerce method, and a `MerchantClient`
never exposes a treasury one - the type you hold only offers what its key can
do, matching the API itself (an agent key is refused on commerce routes and a
merchant key on treasury routes).

## Install

```sh
go get github.com/Tee-Stark/enquoir-go
```

## Treasury: submit a payment

```go
client := enquoir.NewTreasuryClient(os.Getenv("ENQUOIR_API_KEY"))

beneficiary, err := client.Beneficiaries.Register(ctx, enquoir.RegisterBeneficiaryRequest{
	Name:            "Acme Supplier",
	AccountNumber:   "000123456789",
	RoutingNumber:   "021000021",
	AccountCategory: enquoir.Checking,
	Address: enquoir.Address{
		Street: "1 Main St", City: "Seattle", State: "WA", Postcode: "98101",
	},
})

submitted, err := client.Payments.Submit(ctx, enquoir.SubmitIntentRequest{
	Payee:     beneficiary.ID,
	Amount:    "10.50",
	Currency:  enquoir.USD,
	Reference: "invoice-1001",
})

for {
	detail, err := client.Payments.Get(ctx, submitted.Key)
	if detail.State.Terminal() {
		break // APPLIED or BLOCKED - not FAILED, which can still move on
	}
	time.Sleep(2 * time.Second)
}
```

Submitting the same intent again - same payee, amount, currency and reference
- returns the same key and starts nothing new, however long after the first
submission. Full runnable version: [`examples/submit-payment`](examples/submit-payment).

## Commerce: run a checkout

```go
client := enquoir.NewMerchantClient(os.Getenv("ENQUOIR_API_KEY"))

checkout, err := client.Checkouts.Submit(ctx, commerce.SubmitCheckoutRequest{
	ExternalID:      "order-1001",
	MandateID:       mandate.ID,
	CustomerID:      mandate.Terms.CustomerID,
	AgentID:         mandate.Terms.Agent.ID,
	FundingMethodID: mandate.Terms.FundingMethod.ID,
	Lines: commerce.CheckoutLines{
		Currency:   mandate.Terms.Scope.Currency,
		Items:      []commerce.CheckoutItem{{ProductID: "widget-1", Name: "Widget", Category: "electronics", Quantity: 1, UnitPriceMinor: 1500}},
		TotalMinor: 1500,
	},
})
if checkout.Status != commerce.CheckoutAuthorized {
	// checkout.Decision.Reasons says why: a mandate-level reason, or a
	// Policy-prefixed one from the organisation's own spending policy.
	return
}

checkout, err = client.Checkouts.IssueGrant(ctx, checkout.ID)

execution, err := client.Grants.Redeem(ctx, commerce.RedeemGrantRequest{
	Grant:              checkout.Authorization.Grant, // the one-time secret; shown only here
	ExternalCheckoutID: checkout.ExternalID,
	CheckoutHash:       checkout.CheckoutHash,
})

// Charge execution.AmountMinor of execution.Currency through your own PSP,
// using execution.IdempotencyKey as that call's own idempotency key, then:

result, err := client.Executions.ReportOutcome(ctx, execution.ID, commerce.ReportOutcomeRequest{
	Outcome:                  commerce.OutcomeSucceeded,
	Provider:                 execution.FundingMethod.Provider,
	ProviderPaymentReference: "pi_...",
	AmountMinor:              execution.AmountMinor,
	Currency:                 execution.Currency,
})
```

A checkout's mandate has to be active first, which needs the customer's
consent - an emailed one-time link, approved from Enquoir's own dashboard or
consent page. That flow takes no API key, so it's outside this SDK's scope on
purpose; everything after an active mandate exists is what's shown above.
Full runnable version (reads an existing mandate's ID from the environment):
[`examples/commerce-checkout`](examples/commerce-checkout).

Registering the customers, agents and funding methods a mandate names is the
same shape as treasury's beneficiaries:

```go
customer, err := client.Customers.Register(ctx, commerce.RegisterCustomerRequest{
	ExternalID: "cust-42", Email: "buyer@example.com",
})
agent, err := client.MerchantAgents.Register(ctx, commerce.RegisterMerchantAgentRequest{
	ExternalID: "shopping-bot-1", Name: "Shopping Bot",
})
funding, err := client.FundingMethods.Register(ctx, commerce.RegisterFundingMethodRequest{
	CustomerID: customer.ID, Provider: "stripe", MerchantFundingMethodID: "mfm_1",
	Display: commerce.FundingMethodDisplay{Brand: "visa", LastFour: "4242"},
})
mandateRequest, err := client.Mandates.Request(ctx, commerce.RequestMandateRequest{
	ExternalID: "mandate-1", CustomerID: customer.ID, AgentID: agent.ID, FundingMethodID: funding.ID,
	ApprovalMode: commerce.Autonomous,
	Scope: commerce.MandateScope{Currency: enquoir.USD, MaxTransactionMinor: 10000, MaxCumulativeMinor: 100000},
	ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
})
// mandateRequest.Status is "pending" until the customer approves it.
```

## Errors

Every method returns an `*enquoir.APIError` on failure - Enquoir's own
[RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) Problem response, with a
stable `Code` meant to be branched on:

```go
_, err := client.Payments.Submit(ctx, req)
if enquoir.IsCode(err, "payee_unknown") {
	// register the beneficiary first
}
```

`Code`'s full set of values is documented per-operation in Enquoir's
[OpenAPI spec](openapi/openapi.yaml).

## Design

- **Hand-written wire types, contract-tested against the spec.** No code
  generation: every request/response type is a plain Go struct. A
  [`contract_test.go`](contract_test.go) harness validates every table-driven
  test's actual request and response against the vendored
  [`openapi.yaml`](openapi/openapi.yaml) with
  [`kin-openapi`](https://github.com/getkin/kin-openapi) - the same library
  the backend's own contract test uses - so a hand-written type can't quietly
  drift from the API it's meant to match.
- **No floats for money.** Treasury amounts are a decimal string
  (`"10.50"`); commerce amounts are integer minor units plus a currency code.
- **No blind retries.** Only `GET` is retried (capped, exponential backoff,
  on network errors and 5xx) - a mutating call is sent once, matching the
  backend's own rule that an unclear outcome must be looked up, never guessed
  at by resending.

## Development

```sh
go build ./...
go vet ./...
go test ./...
gofmt -l .
```

[`scripts/sync-openapi.sh`](scripts/sync-openapi.sh) re-vendors `openapi.yaml`
from a sibling checkout of the backend (`../enquoir` by default);
`scripts/sync-openapi.sh --check` fails if the vendored copy has drifted.

## License

[MIT](LICENSE)
