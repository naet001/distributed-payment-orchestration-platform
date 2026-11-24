package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/payment-platform/internal/models"
	"example.com/payment-platform/internal/payment"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type inMemoryPaymentRepository struct {
	payments  []models.Payment
	createErr error
}

func (r *inMemoryPaymentRepository) Create(p *models.Payment) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.payments = append(r.payments, *p)
	return nil
}

func (*inMemoryPaymentRepository) FindByID(uint) (*models.Payment, error) {
	return nil, errors.New("not implemented")
}

func (*inMemoryPaymentRepository) FindByReference(string) (*models.Payment, error) {
	return nil, errors.New("not implemented")
}

func (*inMemoryPaymentRepository) Update(*models.Payment) error { return errors.New("not implemented") }
func (*inMemoryPaymentRepository) List() ([]models.Payment, error) {
	return nil, errors.New("not implemented")
}

func TestCreatePayment(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repository := &inMemoryPaymentRepository{}
	router := NewGinHandler(payment.NewService(repository), NewResponder(newTestLogger())).SetupRoutes()

	request := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(`{"amount":1250,"currency":"USD","provider":"stripe"}`))
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
	require.Equal(t, created.Reference, repository.payments[0].Reference)
}

func TestCreatePaymentHidesAndLogsSaveError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const underlyingError = "database unavailable"
	repository := &inMemoryPaymentRepository{createErr: errors.New(underlyingError)}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	router := NewGinHandler(payment.NewService(repository), NewResponder(logger)).SetupRoutes()

	request := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(`{"amount":1250,"currency":"USD","provider":"stripe"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.JSONEq(t, `{"error":"failed to create payment"}`, response.Body.String())
	require.NotContains(t, response.Body.String(), underlyingError)
	require.Contains(t, logs.String(), underlyingError)
}

func TestCreatePaymentRejectsInvalidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, body := range []string{
		`{"amount":0,"currency":"USD","provider":"stripe"}`,
		`{"amount":1250,"currency":"US","provider":"stripe"}`,
		`{"amount":1250,"currency":"USD"}`,
		`{"amount":1250,"currency":"USD","provider":"provider-name-that-is-more-than-thirty-characters"}`,
	} {
		t.Run(body, func(t *testing.T) {
			repository := &inMemoryPaymentRepository{}
			router := NewGinHandler(payment.NewService(repository), NewResponder(newTestLogger())).SetupRoutes()

			request := httptest.NewRequest(http.MethodPost, "/payments", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			require.Equal(t, http.StatusBadRequest, response.Code)
			require.Empty(t, repository.payments)
		})
	}
}
