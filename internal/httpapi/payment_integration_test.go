package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"example.com/payment-platform/internal/models"
	"example.com/payment-platform/internal/payment"
	"example.com/payment-platform/internal/repositories"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestCreatePaymentHTTPWithPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration test")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&models.Payment{}))

	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { require.NoError(t, tx.Rollback().Error) })

	gin.SetMode(gin.TestMode)
	repository := repositories.NewGormPaymentRepository(databaseForTest{db: tx})
	router := NewGinHandler(payment.NewService(repository)).SetupRoutes()
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	requestBody := `{"amount":1250,"currency":"USD","provider":"stripe"}`
	response, err := http.Post(server.URL+"/payments", "application/json", strings.NewReader(requestBody))
	require.NoError(t, err)
	defer response.Body.Close()

	require.Equal(t, http.StatusCreated, response.StatusCode)
	var created models.Payment
	require.NoError(t, json.NewDecoder(response.Body).Decode(&created))
	require.Regexp(t, `^PAY_[0-9a-f]{32}$`, created.Reference)
	require.Equal(t, int64(1250), created.Amount)
	require.Equal(t, "USD", created.Currency)
	require.Equal(t, models.PaymentStatusPending, created.Status)
	require.Equal(t, "stripe", created.Provider)
	stored, err := repository.FindByReference(created.Reference)
	require.NoError(t, err)
	require.Equal(t, created.Reference, stored.Reference)
	require.Equal(t, models.PaymentStatusPending, stored.Status)

	invalidResponse, err := http.Post(server.URL+"/payments", "application/json", strings.NewReader(`{"amount":0,"currency":"USD","provider":"stripe"}`))
	require.NoError(t, err)
	defer invalidResponse.Body.Close()
	require.Equal(t, http.StatusBadRequest, invalidResponse.StatusCode)
}

type databaseForTest struct {
	db *gorm.DB
}

func (d databaseForTest) GetDB() *gorm.DB {
	return d.db
}
