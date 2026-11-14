// submit-payment registers a payee, submits a payment intent for it, and
// polls until the payment reaches a terminal state.
//
// Usage:
//
//	ENQUOIR_API_KEY=enq_sk_... go run ./examples/submit-payment
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/Tee-Stark/enquoir-go"
)

func main() {
	apiKey := os.Getenv("ENQUOIR_API_KEY")
	if apiKey == "" {
		log.Fatal("set ENQUOIR_API_KEY to an agent key (enq_sk_...)")
	}

	var opts []enquoir.Option
	if base := os.Getenv("ENQUOIR_BASE_URL"); base != "" {
		opts = append(opts, enquoir.WithBaseURL(base))
	}
	client := enquoir.NewTreasuryClient(apiKey, opts...)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	beneficiary, err := client.Beneficiaries.Register(ctx, enquoir.RegisterBeneficiaryRequest{
		Name:            "Acme Supplier",
		AccountNumber:   "000123456789",
		RoutingNumber:   "021000021",
		AccountCategory: enquoir.Checking,
		Address: enquoir.Address{
			Street:   "1 Main St",
			City:     "Seattle",
			State:    "WA",
			Postcode: "98101",
		},
	})
	if err != nil {
		log.Fatalf("register beneficiary: %v", err)
	}
	log.Printf("beneficiary registered: id=%s key=%s", beneficiary.ID, beneficiary.Key)

	submitted, err := client.Payments.Submit(ctx, enquoir.SubmitIntentRequest{
		Payee:     beneficiary.ID,
		Amount:    "10.50",
		Currency:  enquoir.USD,
		Reference: "invoice-1001",
		Reason:    "submit-payment example",
	})
	if err != nil {
		log.Fatalf("submit intent: %v", err)
	}
	log.Printf("intent %s: outcome=%s state=%s", submitted.Key, submitted.Outcome, submitted.State)

	for {
		detail, err := client.Payments.Get(ctx, submitted.Key)
		if err != nil {
			log.Fatalf("get intent: %v", err)
		}
		log.Printf("intent %s: state=%s", detail.Key, detail.State)

		if detail.State.Terminal() {
			log.Printf("intent %s reached a terminal state: %s", detail.Key, detail.State)
			return
		}

		select {
		case <-ctx.Done():
			log.Fatal("timed out waiting for a terminal state")
		case <-time.After(2 * time.Second):
		}
	}
}
