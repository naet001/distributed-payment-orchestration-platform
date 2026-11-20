package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler interface {
	SetupRoutes() *gin.Engine
}

type GinHandler struct{}

func NewGinHandler() Handler {
	return &GinHandler{}
}

func (h *GinHandler) SetupRoutes() *gin.Engine {
	router := gin.Default()

	router.GET("/health", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	return router
}
