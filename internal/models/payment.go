package models

import (
	"gorm.io/gorm"
)

// PaymentStatus represents the current state of a payment.
type PaymentStatus string

// Payment status constants.
const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusCompleted PaymentStatus = "completed"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusRefunded  PaymentStatus = "refunded"
)

// Payment represents a payment transaction.
type Payment struct {
	gorm.Model
	Reference string        `gorm:"uniqueIndex;not null"`
	Amount    int64         `gorm:"not null"`
	Currency  string        `gorm:"size:3;not null"`
	Status    PaymentStatus `gorm:"type:varchar(20);not null"`
	Provider  string        `gorm:"not null"`
}
