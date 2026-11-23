package httpapi

import (
	"errors"
	"net/http"

	"example.com/payment-platform/internal/payment"
	"github.com/gin-gonic/gin"
)

// Handler defines the contract for HTTP route handlers.
type Handler interface {
	SetupRoutes() *gin.Engine
}

// GinHandler implements Handler using the Gin framework.
type GinHandler struct {
	payments *payment.Service
}

// NewGinHandler creates a new GinHandler instance.
func NewGinHandler(payments *payment.Service) Handler {
	return &GinHandler{payments: payments}
}

// SetupRoutes configures the Gin router with all application routes.
func (h *GinHandler) SetupRoutes() *gin.Engine {
	router := gin.Default()

	router.GET("/health", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	router.POST("/payments", h.createPayment)
	return router
}

func (h *GinHandler) createPayment(context *gin.Context) {
	var request payment.CreateRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		context.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment request"})
		return
	}

	created, err := h.payments.Create(request)
	if err != nil {
		if errors.Is(err, payment.ErrInvalidPayment) {
			context.JSON(http.StatusBadRequest, gin.H{"error": "invalid payment request"})
			return
		}
		context.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create payment"})
		return
	}

	context.JSON(http.StatusCreated, created)
}
