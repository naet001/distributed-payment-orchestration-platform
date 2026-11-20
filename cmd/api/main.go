package main

import (
	"log"
	"strconv"

	"example.com/payment-platform/internal/config"
	"example.com/payment-platform/internal/httpapi"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	handler := httpapi.NewGinHandler()
	router := handler.SetupRoutes()
	router.Run(":" + strconv.Itoa(cfg.Port))
}
