package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds the application configuration.
type Config struct {
	// Port is the HTTP server port.
	Port int
}

// Load loads the configuration from environment variables.
// It first attempts to load a .env file, then reads from the environment.
// Returns a Config pointer or an error if validation fails.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	portStr := os.Getenv("PORT")
	if portStr == "" {
		portStr = "8080"
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	if port < 1 || port > 65535 {
		return nil, os.ErrInvalid
	}

	return &Config{Port: port}, nil
}
