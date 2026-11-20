package main

import (
	"example.com/payment-platform/internal/httpapi"
)

func main() {
	router := httpapi.SetupRouter()
	router.Run(":8080")
}
