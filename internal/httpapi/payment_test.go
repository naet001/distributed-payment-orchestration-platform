package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"example.com/payment-platform/internal/models"
	"example.com/payment-platform/internal/payment"
	"example.com/payment-platform/internal/repositories"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type inMemoryPaymentRepository struct {
	payments       map[string]*models.Payment
	createErr      error
	updateErr      error
	transitionErr  error
}

type unusedTransactionInitializer struct{}

func (unusedTransactionInitializer) Provider() string { return "paystack" }
func (unusedTransactionInitializer) InitializeTransaction(context.Context, string, int64, string, string, string) (string, *url.URL, error) {
	return "", nil, errors.New("unexpected transaction initialization")
}

type mockTransactionInitializer struct {
	providerRef string
	authURL     *url.URL
	err         error
}

func (m *mockTransactionInitializer) Provider() string { return "paystack" }
func (m *mockTransactionInitializer) InitializeTransaction(context.Context, string, int64, string, string, string) (string, *url.URL, error) {
	return m.providerRef, m.authURL, m.err
}

type mockTransactionVerifier struct {
	status   string
	ref      string
	amount   int64
	currency string
	err      error
}

func (m *mockTransactionVerifier) Provider() string { return "paystack" }
func (m *mockTransactionVerifier) VerifyTransaction(context.Context, string) (string, string, int64, string, error) {
	return m.status, m.ref, m.amount, m.currency, m.err
}

func newTestPaymentService(repository repositories.PaymentRepository, initializer payment.TransactionInitializer, verifier payment.TransactionVerifier) *payment.Service {
	s := payment.NewService(repository, initializer)
	if verifier != nil {
		s = s.WithTransactionVerifier(verifier)
	}
	return s
}

func newTestRepository() *inMemoryPaymentRepository {
	return &inMemoryPaymentRepository{
		payments: make(map[string]*models.Payment),
	}
}

func (r *inMemoryPaymentRepository) Create(p *models.Payment) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.payments[p.Reference] = p
	return nil
}

func (r *inMemoryPaymentRepository) FindByID(id uint) (*models.Payment, error) {
	for _, p := range r.payments {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, errors.New("payment not found")
}

func (r *inMemoryPaymentRepository) FindByReference(ref string) (*models.Payment, error) {
	p, ok := r.payments[ref]
	if !ok {
		return nil, errors.New("payment not found")
	}
	copy := *p
	return &copy, nil
}

func (r *inMemoryPaymentRepository) Update(p *models.Payment) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	r.payments[p.Reference] = p
	return nil
}

func (r *inMemoryPaymentRepository) TransitionStatus(id uint, expected, next models.PaymentStatus) (bool, error) {
	if r.transitionErr != nil {
		return false, r.transitionErr
	}
	for _, p := range r.payments {
		if p.ID == id && p.Status == expected {
			p.Status = next
			return true, nil
		}
	}
	return false, nil
}

func (r *inMemoryPaymentRepository) List() ([]models.Payment, error) {
	var result []models.Payment
	for _, p := range r.payments {
		result = append(result, *p)
	}
	return result, nil
}

func TestCreatePayment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := newTestRepository()
	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, nil), NewResponder(newTestLogger())).SetupRoutes()

	request := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(`{"amount":1250,"currency":"USD","provider":"stripe","email":"customer@example.com"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusCreated, response.Code)
	var created models.Payment
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &created))
	require.Regexp(t, `^PAY_[0-9a-f]{32}$`, created.Reference)
	require.Equal(t, int64(1250), created.Amount)
	require.Equal(t, "USD", created.Currency)
	require.Equal(t, "stripe", created.Provider)
	require.Equal(t, models.PaymentStatusPending, created.Status)
	require.Len(t, repository.payments, 1)
	require.Equal(t, created.Reference, repository.payments[created.Reference].Reference)
}

func TestCreatePaymentHidesAndLogsSaveError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const underlyingError = "database unavailable"
	repository := newTestRepository()
	repository.createErr = errors.New(underlyingError)
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, nil), NewResponder(logger)).SetupRoutes()

	request := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(`{"amount":1250,"currency":"USD","provider":"stripe","email":"customer@example.com"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.JSONEq(t, `{"error":"failed to create payment"}`, response.Body.String())
	require.NotContains(t, response.Body.String(), underlyingError)
	require.Contains(t, logs.String(), underlyingError)
}

func TestCreatePaymentRequiresCustomerEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := newTestRepository()
	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, nil), NewResponder(newTestLogger())).SetupRoutes()

	request := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(`{"amount":1250,"currency":"USD","provider":"paystack"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Empty(t, repository.payments)
}

func TestCreatePaymentRejectsInvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, body := range []string{
		`{"amount":0,"currency":"USD","provider":"stripe"}`,
		`{"amount":1250,"currency":"US","provider":"stripe"}`,
		`{"amount":1250,"currency":"USD"}`,
		`{"amount":1250,"currency":"USD","provider":"paystack"}`,
		`{"amount":1250,"currency":"USD","provider":"provider-name-that-is-more-than-thirty-characters"}`,
	} {
		t.Run(body, func(t *testing.T) {
			repository := newTestRepository()
			router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, nil), NewResponder(newTestLogger())).SetupRoutes()

			request := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusBadRequest, response.Code)
			require.Empty(t, repository.payments)
		})
	}
}

func TestInitializeTransaction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup repository with a pending payment
	repository := newTestRepository()
	reference := "PAY_TEST_REF"
	payment := &models.Payment{
		Reference: reference,
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "paystack",
		Email:     "customer@example.com",
	}
	require.NoError(t, repository.Create(payment))

	// Setup mock initializer
	authURL, _ := url.Parse("https://paystack.com/pay/abc123")
	initializer := &mockTransactionInitializer{
		providerRef: "ACC_123456",
		authURL:     authURL,
		err:         nil,
	}

	router := NewGinHandler(newTestPaymentService(repository, initializer, nil), NewResponder(newTestLogger())).SetupRoutes()

	// Make request
	body := `{"callback_url":"https://example.com/callback"}`
	request := httptest.NewRequest(http.MethodPost, "/payments/"+reference+"/initialize", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	var resp struct {
		AuthorizationURL   string `json:"authorization_url"`
		ProviderReference string `json:"provider_reference"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	require.Equal(t, "https://paystack.com/pay/abc123", resp.AuthorizationURL)
	require.Equal(t, "ACC_123456", resp.ProviderReference)

	// Verify provider reference was stored
	stored, err := repository.FindByReference(reference)
	require.NoError(t, err)
	require.NotNil(t, stored.ProviderReference)
	require.Equal(t, "ACC_123456", *stored.ProviderReference)
}

func TestInitializeTransaction_RequiresCallbackURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := newTestRepository()
	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, nil), NewResponder(newTestLogger())).SetupRoutes()

	body := `{}`
	request := httptest.NewRequest(http.MethodPost, "/payments/PAY_TEST/initialize", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestInitializeTransaction_RejectsInvalidCallbackURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := newTestRepository()
	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, nil), NewResponder(newTestLogger())).SetupRoutes()

	body := `{"callback_url":"not-a-valid-url"}`
	request := httptest.NewRequest(http.MethodPost, "/payments/PAY_TEST/initialize", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestInitializeTransaction_RejectsNonHTTPSCallbackURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := newTestRepository()
	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, nil), NewResponder(newTestLogger())).SetupRoutes()

	body := `{"callback_url":"http://example.com/callback"}`
	request := httptest.NewRequest(http.MethodPost, "/payments/PAY_TEST/initialize", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "HTTPS")
}

func TestInitializeTransaction_PaymentNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := newTestRepository()
	initializer := &mockTransactionInitializer{}
	router := NewGinHandler(newTestPaymentService(repository, initializer, nil), NewResponder(newTestLogger())).SetupRoutes()

	body := `{"callback_url":"https://example.com/callback"}`
	request := httptest.NewRequest(http.MethodPost, "/payments/NON_EXISTENT/initialize", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNotFound, response.Code)
}

func TestInitializeTransaction_RejectsNonPendingPayment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := newTestRepository()
	payment := &models.Payment{
		Reference: "PAY_TEST_REF",
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusSucceeded,
		Provider:  "paystack",
		Email:     "customer@example.com",
	}
	require.NoError(t, repository.Create(payment))

	initializer := &mockTransactionInitializer{}
	router := NewGinHandler(newTestPaymentService(repository, initializer, nil), NewResponder(newTestLogger())).SetupRoutes()

	body := `{"callback_url":"https://example.com/callback"}`
	request := httptest.NewRequest(http.MethodPost, "/payments/PAY_TEST_REF/initialize", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestInitializeTransaction_RejectsMismatchedProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := newTestRepository()
	payment := &models.Payment{
		Reference: "PAY_TEST_REF",
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "stripe",
		Email:     "customer@example.com",
	}
	require.NoError(t, repository.Create(payment))

	initializer := &mockTransactionInitializer{}
	router := NewGinHandler(newTestPaymentService(repository, initializer, nil), NewResponder(newTestLogger())).SetupRoutes()

	body := `{"callback_url":"https://example.com/callback"}`
	request := httptest.NewRequest(http.MethodPost, "/payments/PAY_TEST_REF/initialize", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestInitializeTransaction_ProviderError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := newTestRepository()
	payment := &models.Payment{
		Reference: "PAY_TEST_REF",
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "paystack",
		Email:     "customer@example.com",
	}
	require.NoError(t, repository.Create(payment))

	initializer := &mockTransactionInitializer{err: errors.New("provider error")}
	router := NewGinHandler(newTestPaymentService(repository, initializer, nil), NewResponder(newTestLogger())).SetupRoutes()

	body := `{"callback_url":"https://example.com/callback"}`
	request := httptest.NewRequest(http.MethodPost, "/payments/PAY_TEST_REF/initialize", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestVerifyPayment(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup repository with a pending payment
	repository := newTestRepository()
	reference := "PAY_TEST_REF"
	payment := &models.Payment{
		Reference: reference,
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "paystack",
		Email:     "customer@example.com",
	}
	require.NoError(t, repository.Create(payment))

	// Setup mock verifier
	verifier := &mockTransactionVerifier{
		status:   "success",
		ref:      reference,
		amount:   100000,
		currency: "NGN",
		err:      nil,
	}

	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, verifier), NewResponder(newTestLogger())).SetupRoutes()

	// Make request
	request := httptest.NewRequest(http.MethodPost, "/payments/"+reference+"/verify", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	var resp struct {
		Reference string `json:"reference"`
		Status    string `json:"status"`
		Amount    int64  `json:"amount"`
		Currency  string `json:"currency"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	require.Equal(t, reference, resp.Reference)
	require.Equal(t, "succeeded", resp.Status)
	require.Equal(t, int64(100000), resp.Amount)
	require.Equal(t, "NGN", resp.Currency)

	// Verify payment status was updated
	stored, err := repository.FindByReference(reference)
	require.NoError(t, err)
	require.Equal(t, models.PaymentStatusSucceeded, stored.Status)
}

func TestVerifyPayment_PendingResult(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := newTestRepository()
	reference := "PAY_TEST_REF"
	payment := &models.Payment{
		Reference: reference,
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "paystack",
		Email:     "customer@example.com",
	}
	require.NoError(t, repository.Create(payment))

	verifier := &mockTransactionVerifier{
		status:   "pending",
		ref:      reference,
		amount:   100000,
		currency: "NGN",
		err:      nil,
	}

	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, verifier), NewResponder(newTestLogger())).SetupRoutes()

	request := httptest.NewRequest(http.MethodPost, "/payments/"+reference+"/verify", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	var resp struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	require.Equal(t, "pending", resp.Status)

	stored, err := repository.FindByReference(reference)
	require.NoError(t, err)
	require.Equal(t, models.PaymentStatusPending, stored.Status)
}

func TestVerifyPayment_FailedResult(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := newTestRepository()
	reference := "PAY_TEST_REF"
	payment := &models.Payment{
		Reference: reference,
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "paystack",
		Email:     "customer@example.com",
	}
	require.NoError(t, repository.Create(payment))

	verifier := &mockTransactionVerifier{
		status:   "failed",
		ref:      reference,
		amount:   100000,
		currency: "NGN",
		err:      nil,
	}

	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, verifier), NewResponder(newTestLogger())).SetupRoutes()

	request := httptest.NewRequest(http.MethodPost, "/payments/"+reference+"/verify", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	var resp struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &resp))
	require.Equal(t, "failed", resp.Status)

	stored, err := repository.FindByReference(reference)
	require.NoError(t, err)
	require.Equal(t, models.PaymentStatusFailed, stored.Status)
}

func TestVerifyPayment_UnknownStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := newTestRepository()
	reference := "PAY_TEST_REF"
	payment := &models.Payment{
		Reference: reference,
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "paystack",
		Email:     "customer@example.com",
	}
	require.NoError(t, repository.Create(payment))

	verifier := &mockTransactionVerifier{
		status:   "processing_later",
		ref:      reference,
		amount:   100000,
		currency: "NGN",
		err:      nil,
	}

	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, verifier), NewResponder(newTestLogger())).SetupRoutes()

	request := httptest.NewRequest(http.MethodPost, "/payments/"+reference+"/verify", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadGateway, response.Code)
}

func TestVerifyPayment_ProviderError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := newTestRepository()
	reference := "PAY_TEST_REF"
	payment := &models.Payment{
		Reference: reference,
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "paystack",
		Email:     "customer@example.com",
	}
	require.NoError(t, repository.Create(payment))

	verifier := &mockTransactionVerifier{
		err: errors.New("provider error"),
	}

	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, verifier), NewResponder(newTestLogger())).SetupRoutes()

	request := httptest.NewRequest(http.MethodPost, "/payments/"+reference+"/verify", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code)
}

func TestVerifyPayment_MismatchedProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := newTestRepository()
	reference := "PAY_TEST_REF"
	payment := &models.Payment{
		Reference: reference,
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "stripe",
		Email:     "customer@example.com",
	}
	require.NoError(t, repository.Create(payment))

	verifier := &mockTransactionVerifier{}
	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, verifier), NewResponder(newTestLogger())).SetupRoutes()

	request := httptest.NewRequest(http.MethodPost, "/payments/"+reference+"/verify", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusBadRequest, response.Code)
}

func TestVerifyPayment_PaymentNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := newTestRepository()
	verifier := &mockTransactionVerifier{}
	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, verifier), NewResponder(newTestLogger())).SetupRoutes()

	request := httptest.NewRequest(http.MethodPost, "/payments/NON_EXISTENT/verify", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusNotFound, response.Code)
}

func TestVerifyPayment_ResultMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	repository := newTestRepository()
	reference := "PAY_TEST_REF"
	payment := &models.Payment{
		Reference: reference,
		Amount:    100000,
		Currency:  "NGN",
		Status:    models.PaymentStatusPending,
		Provider:  "paystack",
		Email:     "customer@example.com",
	}
	require.NoError(t, repository.Create(payment))

	verifier := &mockTransactionVerifier{
		status:   "success",
		ref:      "PAY_OTHER",
		amount:   100000,
		currency: "NGN",
		err:      nil,
	}

	router := NewGinHandler(newTestPaymentService(repository, unusedTransactionInitializer{}, verifier), NewResponder(newTestLogger())).SetupRoutes()

	request := httptest.NewRequest(http.MethodPost, "/payments/"+reference+"/verify", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusConflict, response.Code)
}
