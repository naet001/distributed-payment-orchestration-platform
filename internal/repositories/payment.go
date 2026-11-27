package repositories

import (
	"errors"

	"example.com/payment-platform/internal/database"
	"example.com/payment-platform/internal/models"
	"gorm.io/gorm"
)

// ErrPaymentNotFound is returned when a payment does not exist.
var ErrPaymentNotFound = errors.New("payment not found")

// PaymentRepository defines the contract for payment data access.
type PaymentRepository interface {
	Create(payment *models.Payment) error
	FindByID(id uint) (*models.Payment, error)
	FindByReference(ref string) (*models.Payment, error)
	Update(payment *models.Payment) error
	TransitionStatus(id uint, expected, next models.PaymentStatus) (bool, error)
	List() ([]models.Payment, error)
}

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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPaymentNotFound
		}
		return nil, err
	}
	return &payment, nil
}

// FindByReference retrieves a payment by its reference.
func (r *GormPaymentRepository) FindByReference(ref string) (*models.Payment, error) {
	var payment models.Payment
	err := r.db.GetDB().Where("reference = ?", ref).First(&payment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPaymentNotFound
		}
		return nil, err
	}
	return &payment, nil
}

// Update saves changes to an existing payment.
func (r *GormPaymentRepository) Update(payment *models.Payment) error {
	var existing models.Payment
	if err := r.db.GetDB().First(&existing, payment.ID).Error; err != nil {
		return err
	}
	return r.db.GetDB().Model(&existing).Updates(map[string]interface{}{
		"amount":             payment.Amount,
		"currency":           payment.Currency,
		"provider":           payment.Provider,
		"email":              payment.Email,
		"provider_reference": payment.ProviderReference,
	}).Error
}

// TransitionStatus changes a payment status only when the stored status matches expected.
func (r *GormPaymentRepository) TransitionStatus(id uint, expected, next models.PaymentStatus) (bool, error) {
	result := r.db.GetDB().Model(&models.Payment{}).
		Where("id = ? AND status = ?", id, expected).
		Update("status", next)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// List retrieves all payments.
func (r *GormPaymentRepository) List() ([]models.Payment, error) {
	var payments []models.Payment
	err := r.db.GetDB().Find(&payments).Error
	if err != nil {
		return nil, err
	}
	return payments, nil
}
