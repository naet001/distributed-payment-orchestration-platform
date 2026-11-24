package httpapi

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

// Responder writes HTTP responses and logs server-side causes for errors.
type Responder struct {
	logger *slog.Logger
}

// NewResponder creates a response writer with an injected logger.
func NewResponder(logger *slog.Logger) *Responder {
	return &Responder{logger: logger}
}

// Success writes a success response to the client.
func (r *Responder) Success(context *gin.Context, status int, body any) {
	context.JSON(status, body)
}

// Error logs the underlying cause, when present, and writes a safe error message.
func (r *Responder) Error(context *gin.Context, status int, message string, cause error) {
	if cause != nil {
		r.logger.Error("HTTP request failed", slog.Int("status", status), slog.String("error", cause.Error()))
	}
	context.JSON(status, gin.H{"error": message})
}
