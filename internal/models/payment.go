package models

import "time"

type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"
	PaymentStatusSucceeded PaymentStatus = "succeeded"
	PaymentStatusFailed    PaymentStatus = "failed"
	PaymentStatusCancelled PaymentStatus = "cancelled"
)

type Payment struct {
	ID                uint          `gorm:"primaryKey"`
	Reference         string        `gorm:"type:varchar(64);not null;uniqueIndex"`
	Amount            int64         `gorm:"not null;check:amount > 0"`
	Currency          string        `gorm:"type:char(3);not null"`
	Status            PaymentStatus `gorm:"type:varchar(20);not null;index"`
	Provider          string        `gorm:"type:varchar(30);not null"`
	Email             string        `gorm:"type:varchar(254)" json:"-"`
	ProviderReference *string       `gorm:"type:varchar(100);uniqueIndex"`

	CreatedAt time.Time
	UpdatedAt time.Time
}
