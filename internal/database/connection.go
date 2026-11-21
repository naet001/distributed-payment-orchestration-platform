package database

import (
	"fmt"
	"log"

	"example.com/payment-platform/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DB is the GORM database connection interface.
type DB interface {
	GetDB() *gorm.DB
}

// GormDB wraps a GORM database connection.
type GormDB struct {
	conn *gorm.DB
}

// NewGormDB creates a new GORM database connection from config.
func NewGormDB(cfg *config.Config) (*GormDB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBName,
		cfg.DBSSLMode,
	)

	conn, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("failed to connect to database: %v", err)
		return nil, fmt.Errorf("database connection failed: %w", err)
	}

	return &GormDB{conn: conn}, nil
}

// GetDB returns the underlying GORM database connection.
func (db *GormDB) GetDB() *gorm.DB {
	return db.conn
}

// Close closes the database connection.
func (db *GormDB) Close() error {
	sqlDB, err := db.conn.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
