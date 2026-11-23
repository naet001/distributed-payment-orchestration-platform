package payment

import "example.com/payment-platform/internal/repositories"

// Service coordinates payment operations.
type Service struct {
	repository repositories.PaymentRepository
}

// NewService creates a payment service.
func NewService(repository repositories.PaymentRepository) *Service {
	return &Service{
		repository: repository,
	}
}
