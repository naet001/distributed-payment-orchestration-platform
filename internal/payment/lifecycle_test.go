package payment

import (
	"errors"
	"testing"

	"example.com/payment-platform/internal/models"
)

type lifecycleRepository struct {
	payment models.Payment
}

func (r *lifecycleRepository) Create(payment *models.Payment) error {
	payment.ID = 1
	r.payment = *payment
	return nil
}

func (r *lifecycleRepository) FindByID(id uint) (*models.Payment, error) {
	if id != r.payment.ID {
		return nil, errors.New("payment not found")
	}
	return &r.payment, nil
}

func (r *lifecycleRepository) FindByReference(reference string) (*models.Payment, error) {
	if reference != r.payment.Reference {
		return nil, errors.New("payment not found")
	}
	return &r.payment, nil
}

func (r *lifecycleRepository) Update(payment *models.Payment) error {
	r.payment = *payment
	return nil
}

func (r *lifecycleRepository) TransitionStatus(id uint, expected, next models.PaymentStatus) (bool, error) {
	if id != r.payment.ID || r.payment.Status != expected {
		return false, nil
	}
	r.payment.Status = next
	return true, nil
}

func (r *lifecycleRepository) List() ([]models.Payment, error) {
	return nil, nil
}

func TestApplyPaystackResult(t *testing.T) {
	tests := []struct {
		name       string
		result     PaystackResult
		wantStatus models.PaymentStatus
		wantError  error
	}{
		{
			name:       "pending result keeps payment pending",
			result:     PaystackResult{Status: "pending", Reference: "PAY_reference", Amount: 1000, Currency: "USD"},
			wantStatus: models.PaymentStatusPending,
		},
		{
			name:       "verified success marks payment succeeded",
			result:     PaystackResult{Status: "success", Reference: "PAY_reference", Amount: 1000, Currency: "USD"},
			wantStatus: models.PaymentStatusSucceeded,
		},
		{
			name:       "verified failure marks payment failed",
			result:     PaystackResult{Status: "failed", Reference: "PAY_reference", Amount: 1000, Currency: "USD"},
			wantStatus: models.PaymentStatusFailed,
		},
		{
			name:       "unknown result leaves payment pending",
			result:     PaystackResult{Status: "processing_later", Reference: "PAY_reference", Amount: 1000, Currency: "USD"},
			wantStatus: models.PaymentStatusPending,
			wantError:  ErrUnknownPaystackStatus,
		},
		{
			name:       "success with mismatched amount leaves payment pending",
			result:     PaystackResult{Status: "success", Reference: "PAY_reference", Amount: 999, Currency: "USD"},
			wantStatus: models.PaymentStatusPending,
			wantError:  ErrPaystackResultMismatch,
		},
		{
			name:       "success with mismatched reference leaves payment pending",
			result:     PaystackResult{Status: "success", Reference: "PAY_other", Amount: 1000, Currency: "USD"},
			wantStatus: models.PaymentStatusPending,
			wantError:  ErrPaystackResultMismatch,
		},
		{
			name:       "success with mismatched currency leaves payment pending",
			result:     PaystackResult{Status: "success", Reference: "PAY_reference", Amount: 1000, Currency: "EUR"},
			wantStatus: models.PaymentStatusPending,
			wantError:  ErrPaystackResultMismatch,
		},
		{
			name:       "repeated success is idempotent",
			result:     PaystackResult{Status: "success", Reference: "PAY_reference", Amount: 1000, Currency: "USD"},
			wantStatus: models.PaymentStatusSucceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &lifecycleRepository{payment: models.Payment{
				ID: 1, Reference: "PAY_reference", Amount: 1000, Currency: "USD",
				Status: models.PaymentStatusPending, Provider: "paystack",
			}}
			service := NewService(repository, &mockTransactionInitializer{})
			if tt.name == "repeated success is idempotent" {
				repository.payment.Status = models.PaymentStatusSucceeded
			}

			_, err := service.ApplyPaystackResult("PAY_reference", tt.result)
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("ApplyPaystackResult() error = %v, want %v", err, tt.wantError)
			}
			if got := repository.payment.Status; got != tt.wantStatus {
				t.Fatalf("stored status = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}

func TestPaymentLifecycle(t *testing.T) {
	tests := []struct {
		name        string
		transitions []models.PaymentStatus
		wantFinal   models.PaymentStatus
		wantError   bool
	}{
		{name: "pending remains pending", transitions: []models.PaymentStatus{models.PaymentStatusPending}, wantFinal: models.PaymentStatusPending},
		{name: "pending succeeds", transitions: []models.PaymentStatus{models.PaymentStatusSucceeded}, wantFinal: models.PaymentStatusSucceeded},
		{name: "pending fails", transitions: []models.PaymentStatus{models.PaymentStatusFailed}, wantFinal: models.PaymentStatusFailed},
		{name: "pending is cancelled", transitions: []models.PaymentStatus{models.PaymentStatusCancelled}, wantFinal: models.PaymentStatusCancelled},
		{name: "terminal status can be repeated", transitions: []models.PaymentStatus{models.PaymentStatusSucceeded, models.PaymentStatusSucceeded}, wantFinal: models.PaymentStatusSucceeded},
		{name: "succeeded cannot return to pending", transitions: []models.PaymentStatus{models.PaymentStatusSucceeded, models.PaymentStatusPending}, wantFinal: models.PaymentStatusSucceeded, wantError: true},
		{name: "succeeded cannot become failed", transitions: []models.PaymentStatus{models.PaymentStatusSucceeded, models.PaymentStatusFailed}, wantFinal: models.PaymentStatusSucceeded, wantError: true},
		{name: "failed cannot become succeeded", transitions: []models.PaymentStatus{models.PaymentStatusFailed, models.PaymentStatusSucceeded}, wantFinal: models.PaymentStatusFailed, wantError: true},
		{name: "cancelled cannot become succeeded", transitions: []models.PaymentStatus{models.PaymentStatusCancelled, models.PaymentStatusSucceeded}, wantFinal: models.PaymentStatusCancelled, wantError: true},
		{name: "unknown status is rejected", transitions: []models.PaymentStatus{models.PaymentStatus("unknown")}, wantFinal: models.PaymentStatusPending, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repository := &lifecycleRepository{}
			service := NewService(repository, &mockTransactionInitializer{})
			created, err := service.Create(CreateRequest{Amount: 1000, Currency: "USD", Provider: "paystack"})
			if err != nil {
				t.Fatalf("Create() error = %v, want nil", err)
			}

			for i, next := range tt.transitions {
				_, err := service.TransitionStatus(created.Reference, next)
				if err != nil {
					if !tt.wantError || i != len(tt.transitions)-1 {
						t.Fatalf("TransitionStatus(%q) error = %v, want nil", next, err)
					}
					break
				}
				if tt.wantError && i == len(tt.transitions)-1 {
					t.Fatalf("TransitionStatus(%q) error = nil, want error", next)
				}
			}

			if repository.payment.Status != tt.wantFinal {
				t.Fatalf("stored status = %q, want %q", repository.payment.Status, tt.wantFinal)
			}
		})
	}
}
