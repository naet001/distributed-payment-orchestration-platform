package payment

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"example.com/payment-platform/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRepository implements repositories.PaymentRepository for testing
type mockRepository struct {
	payments    map[string]*models.Payment
	updateErr   error
	updateCalls int
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		payments: make(map[string]*models.Payment),
	}
}

func (m *mockRepository) Create(payment *models.Payment) error {
	m.payments[payment.Reference] = payment
	return nil
}

func (m *mockRepository) FindByID(id uint) (*models.Payment, error) {
	for _, p := range m.payments {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, errors.New("payment not found")
}

func (m *mockRepository) FindByReference(reference string) (*models.Payment, error) {
	payment, ok := m.payments[reference]
	if !ok {
		return nil, errors.New("payment not found")
	}
	copy := *payment
	return &copy, nil
}

func (m *mockRepository) Update(payment *models.Payment) error {
	m.updateCalls++
	if m.updateErr != nil {
		return m.updateErr
	}
	copy := *payment
	m.payments[payment.Reference] = &copy
	return nil
}

func (m *mockRepository) TransitionStatus(id uint, expected, next models.PaymentStatus) (bool, error) {
	// Simplified for testing
	return true, nil
}

func (m *mockRepository) List() ([]models.Payment, error) {
	var result []models.Payment
	for _, p := range m.payments {
		result = append(result, *p)
	}
	return result, nil
}

// mockTransactionInitializer implements TransactionInitializer for testing
type mockTransactionInitializer struct {
	providerRef string
	authURL     *url.URL
	err         error
	calls       int
}

func (m *mockTransactionInitializer) Provider() string {
	return "paystack"
}

func (m *mockTransactionInitializer) InitializeTransaction(
	ctx context.Context,
	reference string,
	amount int64,
	currency string,
	callbackURL string,
) (providerRef string, authURL *url.URL, err error) {
	m.calls++
	return m.providerRef, m.authURL, m.err
}

func TestService_InitializeTransaction_Success(t *testing.T) {
	// Setup mock repository with a pending payment
	repo := newMockRepository()
	payment := &models.Payment{
		Reference: "PAY_TEST_REF",
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "paystack",
	}
	repo.Create(payment)

	// Setup mock transaction initializer
	authURL, _ := url.Parse("https://paystack.com/pay/abc123")
	initializer := &mockTransactionInitializer{
		providerRef: "ACC_123456",
		authURL:     authURL,
		err:         nil,
	}

	// Create service with both repository and initializer
	service := NewService(repo, initializer)

	// Call InitializeTransaction
	ctx := context.Background()
	providerRef, returnedURL, err := service.InitializeTransaction(
		ctx,
		"PAY_TEST_REF",
		"https://example.com/callback",
	)

	// Verify results
	require.NoError(t, err)
	assert.Equal(t, "ACC_123456", providerRef)
	assert.Equal(t, authURL, returnedURL)
	stored, err := repo.FindByReference("PAY_TEST_REF")
	require.NoError(t, err)
	require.NotNil(t, stored.ProviderReference)
	assert.Equal(t, "ACC_123456", *stored.ProviderReference)
	assert.Equal(t, 1, repo.updateCalls)
}

func TestService_InitializeTransaction_RepositoryErrorDoesNotReturnCheckout(t *testing.T) {
	repo := newMockRepository()
	repo.Create(&models.Payment{
		Reference: "PAY_TEST_REF",
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "paystack",
	})
	repo.updateErr = errors.New("database unavailable")
	authURL, _ := url.Parse("https://paystack.com/pay/abc123")
	initializer := &mockTransactionInitializer{providerRef: "ACC_123456", authURL: authURL}
	service := NewService(repo, initializer)

	providerRef, returnedURL, err := service.InitializeTransaction(context.Background(), "PAY_TEST_REF", "https://example.com/callback")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "save provider reference")
	assert.Empty(t, providerRef)
	assert.Nil(t, returnedURL)
}

func TestService_InitializeTransaction_RejectsMismatchedProvider(t *testing.T) {
	repo := newMockRepository()
	payment := &models.Payment{
		Reference: "PAY_TEST_REF",
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "stripe",
	}
	repo.Create(payment)
	initializer := &mockTransactionInitializer{providerRef: "ACC_123456"}
	service := NewService(repo, initializer)

	_, _, err := service.InitializeTransaction(context.Background(), "PAY_TEST_REF", "https://example.com/callback")

	require.ErrorIs(t, err, ErrProviderMismatch)
	assert.Zero(t, initializer.calls, "must not initialize a payment with a different provider")
}

func TestService_InitializeTransaction_PaymentNotFound(t *testing.T) {
	// Setup empty mock repository
	repo := newMockRepository()

	// Setup mock transaction initializer
	initializer := &mockTransactionInitializer{}

	// Create service
	service := NewService(repo, initializer)

	// Call InitializeTransaction with non-existent reference
	ctx := context.Background()
	_, _, err := service.InitializeTransaction(
		ctx,
		"NON_EXISTENT_REF",
		"https://example.com/callback",
	)

	// Verify error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find payment")
}

func TestService_InitializeTransaction_NonPendingPayment(t *testing.T) {
	// Setup mock repository with a succeeded payment
	repo := newMockRepository()
	payment := &models.Payment{
		Reference: "PAY_TEST_REF",
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusSucceeded,
		Provider:  "paystack",
	}
	repo.Create(payment)

	// Setup mock transaction initializer
	initializer := &mockTransactionInitializer{}

	// Create service
	service := NewService(repo, initializer)

	// Call InitializeTransaction
	ctx := context.Background()
	_, _, err := service.InitializeTransaction(
		ctx,
		"PAY_TEST_REF",
		"https://example.com/callback",
	)

	// Verify error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot initialize transaction for non-pending payment")
}

func TestService_InitializeTransaction_ProviderError(t *testing.T) {
	// Setup mock repository with a pending payment
	repo := newMockRepository()
	payment := &models.Payment{
		Reference: "PAY_TEST_REF",
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "paystack",
	}
	repo.Create(payment)

	// Setup mock transaction initializer that returns an error
	initializer := &mockTransactionInitializer{
		err: errors.New("provider error"),
	}

	// Create service
	service := NewService(repo, initializer)

	// Call InitializeTransaction
	ctx := context.Background()
	_, _, err := service.InitializeTransaction(
		ctx,
		"PAY_TEST_REF",
		"https://example.com/callback",
	)

	// Verify error
	require.Error(t, err)
	assert.Contains(t, err.Error(), "initialize transaction")
	assert.Contains(t, err.Error(), "provider error")
}

func TestNewService(t *testing.T) {
	repo := newMockRepository()
	initializer := &mockTransactionInitializer{}

	service := NewService(repo, initializer)

	assert.NotNil(t, service)
	assert.Equal(t, repo, service.repository)
	assert.Equal(t, initializer, service.transactionInit)
}
