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
	payments  *payment.Service
	responder *Responder
}

// NewGinHandler creates a new GinHandler instance.
func NewGinHandler(payments *payment.Service, responder *Responder) Handler {
	return &GinHandler{payments: payments, responder: responder}
}

// SetupRoutes configures the Gin router with all application routes.
func (h *GinHandler) SetupRoutes() *gin.Engine {
	router := gin.Default()

	router.GET("/health", func(context *gin.Context) {
		h.responder.Success(context, http.StatusOK, gin.H{"status": "ok"})
	})

	router.POST("/payments", h.createPayment)
	return router
}

func (h *GinHandler) createPayment(context *gin.Context) {
	var request payment.CreateRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		h.responder.Error(context, http.StatusBadRequest, "invalid payment request", nil)
		return
	}

	created, err := h.payments.Create(request)
	if err != nil {
		if errors.Is(err, payment.ErrInvalidPayment) {
			h.responder.Error(context, http.StatusBadRequest, "invalid payment request", nil)
			return
		}
		h.responder.Error(context, http.StatusInternalServerError, "failed to create payment", err)
		return
	}

	h.responder.Success(context, http.StatusCreated, created)
}
