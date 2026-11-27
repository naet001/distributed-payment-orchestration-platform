package main

import (
	"log"
	"log/slog"
	"os"
	"strconv"
	"time"

	"example.com/payment-platform/internal/config"
	"example.com/payment-platform/internal/database"
	"example.com/payment-platform/internal/httpapi"
	"example.com/payment-platform/internal/models"
	"example.com/payment-platform/internal/payment"
	"example.com/payment-platform/internal/repositories"
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

	paymentRepo := repositories.NewGormPaymentRepository(db)
	paystackClient := payment.NewPaystackClient(payment.PaystackConfig{
		SecretKey: cfg.PaystackSecretKey,
		Timeout:   time.Duration(cfg.PaystackTimeout) * time.Second,
	})
	paymentService := payment.NewService(paymentRepo, paystackClient).
		WithTransactionVerifier(paystackClient)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	handler := httpapi.NewGinHandler(paymentService, httpapi.NewResponder(logger))
	router := handler.SetupRoutes()
	router.Run(":" + strconv.Itoa(cfg.Port))
}
