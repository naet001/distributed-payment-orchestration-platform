package payment

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"example.com/payment-platform/internal/models"
	"example.com/payment-platform/internal/repositories"
)

var (
	ErrInvalidPayment         = errors.New("invalid payment")
	ErrInvalidPaymentStatus   = errors.New("invalid payment status")
	ErrInvalidTransition      = errors.New("invalid payment status transition")
	ErrProviderMismatch       = errors.New("payment provider does not match configured initializer")
	ErrUnknownPaystackStatus  = errors.New("unknown Paystack status")
	ErrPaystackResultMismatch = errors.New("Paystack result does not match payment")
)

// PaystackResult represents a verified Paystack transaction status.
type PaystackResult struct {
	Status    string
	Reference string
	Amount    int64
	Currency  string
}

// CreateRequest contains the fields a caller may set when creating a payment.
type CreateRequest struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Provider string `json:"provider"`
}

// Service coordinates payment operations.
type Service struct {
	repository      repositories.PaymentRepository
	transactionInit TransactionInitializer
}

// NewService creates a payment service.
func NewService(repository repositories.PaymentRepository) *Service {
	return &Service{repository: repository}
}

// NewServiceWithInitializer creates a payment service with a transaction initializer.
func NewServiceWithInitializer(repository repositories.PaymentRepository, initializer TransactionInitializer) *Service {
	return &Service{
		repository:      repository,
		transactionInit: initializer,
	}
}

// TransitionStatus applies a valid lifecycle transition to a payment.
func (s *Service) TransitionStatus(reference string, next models.PaymentStatus) (*models.Payment, error) {
	if !validPaymentStatus(next) {
		return nil, ErrInvalidPaymentStatus
	}

	current, err := s.repository.FindByReference(reference)
	if err != nil {
		return nil, fmt.Errorf("find payment: %w", err)
	}
	if current.Status == next {
		return current, nil
	}
	if !allowedTransition(current.Status, next) {
		return nil, ErrInvalidTransition
	}

	updated, err := s.repository.TransitionStatus(current.ID, current.Status, next)
	if err != nil {
		return nil, fmt.Errorf("transition payment status: %w", err)
	}
	if !updated {
		return nil, ErrInvalidTransition
	}
	current.Status = next
	return current, nil
}

func validPaymentStatus(status models.PaymentStatus) bool {
	switch status {
	case models.PaymentStatusPending, models.PaymentStatusSucceeded, models.PaymentStatusFailed, models.PaymentStatusCancelled:
		return true
	default:
		return false
	}
}

func allowedTransition(current, next models.PaymentStatus) bool {
	return current == models.PaymentStatusPending &&
		(next == models.PaymentStatusSucceeded || next == models.PaymentStatusFailed || next == models.PaymentStatusCancelled)
}

// ApplyPaystackResult applies a verified Paystack result to a payment.
// It only changes the payment status for known, validated results.
func (s *Service) ApplyPaystackResult(reference string, result PaystackResult) (*models.Payment, error) {
	payment, err := s.repository.FindByReference(reference)
	if err != nil {
		return nil, fmt.Errorf("find payment: %w", err)
	}

	// Validate the result matches the payment
	if result.Reference != payment.Reference || result.Amount != payment.Amount || result.Currency != payment.Currency {
		return nil, ErrPaystackResultMismatch
	}

	// Map Paystack status to internal status
	var next models.PaymentStatus
	switch result.Status {
	case "pending":
		next = models.PaymentStatusPending
	case "success":
		next = models.PaymentStatusSucceeded
	case "failed":
		next = models.PaymentStatusFailed
	default:
		return nil, ErrUnknownPaystackStatus
	}

	// Apply the transition (idempotent for repeated terminal results)
	return s.TransitionStatus(reference, next)
}

// Create validates and stores a pending payment.
func (s *Service) Create(request CreateRequest) (*models.Payment, error) {
	request.Currency = strings.ToUpper(strings.TrimSpace(request.Currency))
	request.Provider = strings.TrimSpace(request.Provider)
	if request.Amount <= 0 || utf8.RuneCountInString(request.Currency) != 3 || request.Provider == "" || utf8.RuneCountInString(request.Provider) > 30 {
		return nil, ErrInvalidPayment
	}

	reference, err := newReference()
	if err != nil {
		return nil, fmt.Errorf("generate payment reference: %w", err)
	}

	payment := &models.Payment{
		Reference: reference,
		Amount:    request.Amount,
		Currency:  request.Currency,
		Status:    models.PaymentStatusPending,
		Provider:  request.Provider,
	}
	if err := s.repository.Create(payment); err != nil {
		return nil, fmt.Errorf("save payment: %w", err)
	}
	return payment, nil
}

func newReference() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "PAY_" + hex.EncodeToString(value[:]), nil
}

// ErrNoTransactionInitializer is returned when trying to initialize a transaction
// without a configured initializer.
var ErrNoTransactionInitializer = errors.New("no transaction initializer configured")

// InitializeTransaction initializes a transaction with the configured provider.
// It returns the provider's reference and authorization URL.
// The payment must already exist with a pending status.
func (s *Service) InitializeTransaction(ctx context.Context, reference string, callbackURL string) (providerRef string, authURL *url.URL, err error) {
	if s.transactionInit == nil {
		return "", nil, ErrNoTransactionInitializer
	}

	// Get the payment to validate it exists and get amount/currency
	payment, err := s.repository.FindByReference(reference)
	if err != nil {
		return "", nil, fmt.Errorf("find payment: %w", err)
	}

	if payment.Status != models.PaymentStatusPending {
		return "", nil, fmt.Errorf("cannot initialize transaction for non-pending payment: %w", ErrInvalidPaymentStatus)
	}
	if payment.Provider != s.transactionInit.Provider() {
		return "", nil, fmt.Errorf("%w: payment=%q initializer=%q", ErrProviderMismatch, payment.Provider, s.transactionInit.Provider())
	}

	// Initialize transaction with provider
	providerRef, authURL, err = s.transactionInit.InitializeTransaction(
		ctx,
		reference,
		payment.Amount,
		payment.Currency,
		callbackURL,
	)
	if err != nil {
		return "", nil, fmt.Errorf("initialize transaction: %w", err)
	}

	payment.ProviderReference = &providerRef
	if err := s.repository.Update(payment); err != nil {
		return "", nil, fmt.Errorf("save provider reference: %w", err)
	}

	return providerRef, authURL, nil
}
