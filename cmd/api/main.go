package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	// Create a new Gin router
	router := gin.Default()

	// Define the health check route
	router.GET("/health", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	// Start the HTTP server on port 8080
	router.Run(":8080")
}
