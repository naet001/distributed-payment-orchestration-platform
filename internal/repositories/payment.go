package repositories

import (
	"fmt"

	"example.com/payment-platform/internal/database"
	"example.com/payment-platform/internal/models"
)

// PaymentRepository defines the contract for payment data access.
type PaymentRepository interface {
	Create(payment *models.Payment) error
	FindByID(id uint) (*models.Payment, error)
	FindByReference(ref string) (*models.Payment, error)
	Update(payment *models.Payment) error
	List(limit, offset int) ([]models.Payment, error)
}

const MaxPaymentPageSize = 100

// GormPaymentRepository implements PaymentRepository using GORM.
type GormPaymentRepository struct {
	db database.DB
}

// NewGormPaymentRepository creates a new GormPaymentRepository.
func NewGormPaymentRepository(db database.DB) *GormPaymentRepository {
	return &GormPaymentRepository{db: db}
}

// Create saves a new payment to the database.
func (r *GormPaymentRepository) Create(payment *models.Payment) error {
	return r.db.GetDB().Create(payment).Error
}

// FindByID retrieves a payment by its ID.
func (r *GormPaymentRepository) FindByID(id uint) (*models.Payment, error) {
	var payment models.Payment
	err := r.db.GetDB().First(&payment, id).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// FindByReference retrieves a payment by its reference.
func (r *GormPaymentRepository) FindByReference(ref string) (*models.Payment, error) {
	var payment models.Payment
	err := r.db.GetDB().Where("reference = ?", ref).First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

// Update saves changes to an existing payment.
func (r *GormPaymentRepository) Update(payment *models.Payment) error {
	return r.db.GetDB().Save(payment).Error
}

// List retrieves one page of payments ordered by ID.
func (r *GormPaymentRepository) List(limit, offset int) ([]models.Payment, error) {
	if limit < 1 || limit > MaxPaymentPageSize {
		return nil, fmt.Errorf("limit must be between 1 and %d", MaxPaymentPageSize)
	}
	if offset < 0 {
		return nil, fmt.Errorf("offset must not be negative")
	}

	var payments []models.Payment
	err := r.db.GetDB().Order("id ASC").Limit(limit).Offset(offset).Find(&payments).Error
	if err != nil {
		return nil, err
	}
	return payments, nil
}
