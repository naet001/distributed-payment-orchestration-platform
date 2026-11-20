package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler defines the contract for HTTP route handlers.
type Handler interface {
	SetupRoutes() *gin.Engine
}

// GinHandler implements Handler using the Gin framework.
type GinHandler struct{}

// NewGinHandler creates a new GinHandler instance.
func NewGinHandler() Handler {
	return &GinHandler{}
}

// SetupRoutes configures the Gin router with all application routes.
func (h *GinHandler) SetupRoutes() *gin.Engine {
	router := gin.Default()

	router.GET("/health", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	return router
}
