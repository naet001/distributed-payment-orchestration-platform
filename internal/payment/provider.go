package payment

import (
	"context"
	"net/url"
)

// TransactionInitializer defines the interface for payment provider transaction initialization.
type TransactionInitializer interface {
	Provider() string

	// InitializeTransaction creates a transaction with the provider and returns
	// the provider's reference and initialization URL (e.g., authorization URL).
	// The callbackURL is where the provider will redirect after payment completion.
	InitializeTransaction(ctx context.Context, reference string, amount int64, currency string, callbackURL string) (providerRef string, authURL *url.URL, err error)
}
