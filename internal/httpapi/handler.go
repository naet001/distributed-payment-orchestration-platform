package httpapi

import (
	"errors"
	"net/http"
	"net/url"

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
	router.POST("/payments/:reference/initialize", h.initializeTransaction)
	router.POST("/payments/:reference/verify", h.verifyPayment)
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

// initializeTransactionRequest represents the request body for transaction initialization.
type initializeTransactionRequest struct {
	CallbackURL string `json:"callback_url" binding:"required,url"`
}

// initializeTransactionResponse represents the response for successful transaction initialization.
type initializeTransactionResponse struct {
	AuthorizationURL string `json:"authorization_url"`
	ProviderReference string `json:"provider_reference"`
}

func (h *GinHandler) initializeTransaction(context *gin.Context) {
	reference := context.Param("reference")
	if reference == "" {
		h.responder.Error(context, http.StatusBadRequest, "payment reference is required", nil)
		return
	}

	var request initializeTransactionRequest
	if err := context.ShouldBindJSON(&request); err != nil {
		h.responder.Error(context, http.StatusBadRequest, "invalid request: callback_url is required", nil)
		return
	}

	// Validate callback URL
	callbackURL, err := url.ParseRequestURI(request.CallbackURL)
	if err != nil || callbackURL.Scheme == "" || callbackURL.Host == "" {
		h.responder.Error(context, http.StatusBadRequest, "invalid callback_url", nil)
		return
	}

	providerRef, authURL, err := h.payments.InitializeTransaction(context.Request.Context(), reference, request.CallbackURL)
	if err != nil {
		h.handlePaymentError(context, err)
		return
	}

	h.responder.Success(context, http.StatusOK, initializeTransactionResponse{
		AuthorizationURL:   authURL.String(),
		ProviderReference: providerRef,
	})
}

// verifyPaymentResponse represents the response for payment verification.
type verifyPaymentResponse struct {
	Reference string `json:"reference"`
	Status    string `json:"status"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
}

func (h *GinHandler) verifyPayment(context *gin.Context) {
	reference := context.Param("reference")
	if reference == "" {
		h.responder.Error(context, http.StatusBadRequest, "payment reference is required", nil)
		return
	}

	payment, err := h.payments.VerifyPayment(context.Request.Context(), reference)
	if err != nil {
		h.handlePaymentError(context, err)
		return
	}

	h.responder.Success(context, http.StatusOK, verifyPaymentResponse{
		Reference: payment.Reference,
		Status:    string(payment.Status),
		Amount:    payment.Amount,
		Currency:  payment.Currency,
	})
}

func (h *GinHandler) handlePaymentError(context *gin.Context, err error) {
	switch {
	case errors.Is(err, payment.ErrInvalidPayment):
		h.responder.Error(context, http.StatusBadRequest, "invalid payment", nil)
	case errors.Is(err, payment.ErrInvalidPaymentStatus):
		h.responder.Error(context, http.StatusBadRequest, "invalid payment status", nil)
	case errors.Is(err, payment.ErrInvalidTransition):
		h.responder.Error(context, http.StatusConflict, "invalid payment status transition", nil)
	case errors.Is(err, payment.ErrProviderMismatch):
		h.responder.Error(context, http.StatusBadRequest, "payment provider mismatch", nil)
	case errors.Is(err, payment.ErrUnknownPaystackStatus):
		h.responder.Error(context, http.StatusBadGateway, "unknown payment provider status", nil)
	case errors.Is(err, payment.ErrPaystackResultMismatch):
		h.responder.Error(context, http.StatusConflict, "payment provider result mismatch", nil)
	default:
		// Check for not found errors
		if isNotFoundError(err) {
			h.responder.Error(context, http.StatusNotFound, "payment not found", nil)
			return
		}
		h.responder.Error(context, http.StatusInternalServerError, "payment operation failed", err)
	}
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	const findPaymentPrefix = "find payment: "
	return len(errStr) > 0 && (errStr == "payment not found" || 
		(len(errStr) >= len(findPaymentPrefix) && errStr[:len(findPaymentPrefix)] == findPaymentPrefix))
}
