package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/payment-platform/internal/payment"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestHealthEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewGinHandler(payment.NewService(&inMemoryPaymentRepository{}))
	router := handler.SetupRoutes()

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"status":"ok"}`, w.Body.String())
}
