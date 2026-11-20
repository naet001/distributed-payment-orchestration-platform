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

	router := httpapi.SetupRouter()
	router.Run(":" + strconv.Itoa(cfg.Port))
}
