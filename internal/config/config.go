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
	// Database configuration
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
}

// Load loads the configuration from environment variables.
// It first attempts to load a .env file, then reads from the environment.
// Returns a Config pointer or an error if validation fails.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Load and validate PORT
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

	// Load database configuration
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}

	dbPortStr := os.Getenv("DB_PORT")
	if dbPortStr == "" {
		dbPortStr = "5432"
	}

	dbPort, err := strconv.Atoi(dbPortStr)
	if err != nil {
		return nil, err
	}

	if dbPort < 1 || dbPort > 65535 {
		return nil, os.ErrInvalid
	}

	cfg := &Config{
		Port:      port,
		DBHost:    dbHost,
		DBPort:    dbPort,
		DBUser:    os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:    os.Getenv("DB_NAME"),
		DBSSLMode: os.Getenv("DB_SSL_MODE"),
	}

	return cfg, nil
}
