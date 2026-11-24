package payment

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"example.com/payment-platform/internal/models"
	"example.com/payment-platform/internal/repositories"
)

var (
	ErrInvalidPayment       = errors.New("invalid payment")
	ErrInvalidPaymentStatus = errors.New("invalid payment status")
	ErrInvalidTransition    = errors.New("invalid payment status transition")
	ErrUnknownPaystackStatus = errors.New("unknown Paystack status")
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
	repository repositories.PaymentRepository
}

// NewService creates a payment service.
func NewService(repository repositories.PaymentRepository) *Service {
	return &Service{repository: repository}
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
