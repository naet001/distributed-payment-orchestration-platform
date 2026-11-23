package main

import (
	"log"
	"strconv"

	"example.com/payment-platform/internal/config"
	"example.com/payment-platform/internal/database"
	"example.com/payment-platform/internal/httpapi"
	"example.com/payment-platform/internal/models"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := database.NewGormDB(cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Println("database connection established")
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("error closing database: %v", err)
		}
	}()

	err = db.GetDB().AutoMigrate(&models.Payment{})
	if err != nil {
		log.Fatalf("failed to migrate database: %v", err)
	}
	log.Println("database migration completed")

	// paymentRepo := repositories.NewGormPaymentRepository(db)
	handler := httpapi.NewGinHandler()
	router := handler.SetupRoutes()
	router.Run(":" + strconv.Itoa(cfg.Port))
}
