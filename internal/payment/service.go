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
)

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
