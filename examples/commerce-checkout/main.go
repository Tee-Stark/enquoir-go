// commerce-checkout runs one checkout end to end against an existing,
// active mandate: submit for evaluation, issue the grant, redeem it,
// and report the outcome.
//
// It needs an active mandate ID, not a fresh signup: the mandate
// itself only becomes active once the customer approves it from an
// emailed one-time consent link, a flow this SDK deliberately doesn't
// expose (v1 is API-key-only; the consent endpoints take no API key).
// Request one with Mandates.Request, have the customer approve it
// from the dashboard or the emailed link, then pass its ID here.
//
// Usage:
//
//	ENQUOIR_API_KEY=enq_mk_... ENQUOIR_MANDATE_ID=man_... go run ./examples/commerce-checkout
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Tee-Stark/enquoir-go"
	"github.com/Tee-Stark/enquoir-go/commerce"
)

func main() {
	apiKey := os.Getenv("ENQUOIR_API_KEY")
	if apiKey == "" {
		log.Fatal("set ENQUOIR_API_KEY to a merchant key (enq_mk_...)")
	}
	mandateID := os.Getenv("ENQUOIR_MANDATE_ID")
	if mandateID == "" {
		log.Fatal("set ENQUOIR_MANDATE_ID to an active mandate's ID (see this file's doc comment)")
	}

	var opts []enquoir.Option
	if base := os.Getenv("ENQUOIR_BASE_URL"); base != "" {
		opts = append(opts, enquoir.WithBaseURL(base))
	}
	client := enquoir.NewMerchantClient(apiKey, opts...)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	mandate, err := client.Mandates.Get(ctx, mandateID)
	if err != nil {
		log.Fatalf("get mandate: %v", err)
	}
	if mandate.Status != commerce.MandateActive {
		log.Fatalf("mandate %s is %s, not active - approve it first", mandate.ID, mandate.Status)
	}
	log.Printf("mandate %s: active, scope=%d %s max per transaction",
		mandate.ID, mandate.Terms.Scope.MaxTransactionMinor, mandate.Terms.Scope.Currency)

	checkout, err := client.Checkouts.Submit(ctx, commerce.SubmitCheckoutRequest{
		ExternalID:      "order-1001",
		MandateID:       mandate.ID,
		CustomerID:      mandate.Terms.CustomerID,
		AgentID:         mandate.Terms.Agent.ID,
		FundingMethodID: mandate.Terms.FundingMethod.ID,
		Lines: commerce.CheckoutLines{
			Currency: mandate.Terms.Scope.Currency,
			Items: []commerce.CheckoutItem{
				{ProductID: "widget-1", Name: "Widget", Category: "electronics", Quantity: 1, UnitPriceMinor: 1500},
			},
			ShippingMinor: 0,
			TaxMinor:      0,
			FeesMinor:     0,
			TotalMinor:    1500,
		},
	})
	if err != nil {
		log.Fatalf("submit checkout: %v", err)
	}
	log.Printf("checkout %s: status=%s allowed=%v", checkout.ID, checkout.Status, checkout.Decision.Allowed)

	if checkout.Status != commerce.CheckoutAuthorized {
		for _, reason := range checkout.Decision.Reasons {
			log.Printf("  reason: %s - %s", reason.Code, reason.Detail)
		}
		if checkout.Status == commerce.CheckoutAwaitingApproval {
			log.Print("checkout is awaiting the customer's per-transaction approval - another flow this SDK doesn't expose")
		}
		return
	}

	checkout, err = client.Checkouts.IssueGrant(ctx, checkout.ID)
	if err != nil {
		log.Fatalf("issue grant: %v", err)
	}
	log.Printf("checkout %s: grant issued, expires %s", checkout.ID, checkout.Authorization.ExpiresAt)

	execution, err := client.Grants.Redeem(ctx, commerce.RedeemGrantRequest{
		Grant:              checkout.Authorization.Grant,
		ExternalCheckoutID: checkout.ExternalID,
		CheckoutHash:       checkout.CheckoutHash,
	})
	if err != nil {
		log.Fatalf("redeem grant: %v", err)
	}
	log.Printf("execution %s: redeemed, idempotency_key=%s", execution.ID, execution.IdempotencyKey)

	// Charge execution.AmountMinor of execution.Currency through your
	// own PSP here, using execution.IdempotencyKey as that call's own
	// idempotency key, before reporting the outcome.

	result, err := client.Executions.ReportOutcome(ctx, execution.ID, commerce.ReportOutcomeRequest{
		Outcome:                  commerce.OutcomeSucceeded,
		Provider:                 execution.FundingMethod.Provider,
		ProviderPaymentReference: "pi_example_123",
		AmountMinor:              execution.AmountMinor,
		Currency:                 execution.Currency,
	})
	if err != nil {
		log.Fatalf("report outcome: %v", err)
	}
	log.Printf("execution %s: outcome recorded (%s), status=%s", result.ID, result.Result, result.Status)
}
