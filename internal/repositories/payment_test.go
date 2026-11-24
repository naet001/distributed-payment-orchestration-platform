package repositories

import (
	"database/sql"
	"testing"

	"example.com/payment-platform/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// testDB implements database.DB for testing
type testDB struct {
	gormDB *gorm.DB
}

func (db testDB) GetDB() *gorm.DB {
	return db.gormDB
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err != nil {
			t.Errorf("failed to get test database connection: %v", err)
			return
		}
		if err := sqlDB.Close(); err != nil {
			t.Errorf("failed to close test database: %v", err)
		}
	})

	if err := db.AutoMigrate(&models.Payment{}); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return db
}

func TestSetupTestDBClosesConnection(t *testing.T) {
	var sqlDB *sql.DB
	t.Run("cleanup", func(t *testing.T) {
		db := setupTestDB(t)
		var err error
		sqlDB, err = db.DB()
		if err != nil {
			t.Fatalf("failed to get SQL database: %v", err)
		}
	})

	if err := sqlDB.Ping(); err == nil {
		t.Fatal("Ping() error = nil after test cleanup, want closed database error")
	}
}

func TestListRejectsInvalidBoundsBeforeDatabaseAccess(t *testing.T) {
	repository := &GormPaymentRepository{}

	tests := []struct {
		name   string
		limit  int
		offset int
	}{
		{name: "zero limit", limit: 0},
		{name: "negative limit", limit: -1},
		{name: "limit above maximum", limit: MaxPaymentPageSize + 1},
		{name: "negative offset", limit: 1, offset: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := repository.List(tt.limit, tt.offset); err == nil {
				t.Fatal("List() error = nil, want invalid bounds error")
			}
		})
	}
}

func TestCreate(t *testing.T) {
	gormDB := setupTestDB(t)
	repo := NewGormPaymentRepository(testDB{gormDB: gormDB})

	t.Run("successful create", func(t *testing.T) {
		payment := &models.Payment{
			Reference: "test-ref-123",
			Amount:    1000,
			Currency:  "USD",
			Status:    models.PaymentStatusPending,
			Provider:  "stripe",
		}

		err := repo.Create(payment)
		if err != nil {
			t.Fatalf("Create() error = %v, want nil", err)
		}

		if payment.ID != 1 {
			t.Fatalf("payment.ID = %d, want 1", payment.ID)
		}

		// Verify the payment was actually created
		found, err := repo.FindByID(payment.ID)
		if err != nil {
			t.Fatalf("FindByID() after Create() error = %v", err)
		}

		if found.Reference != payment.Reference {
			t.Fatalf("found.Reference = %s, want %s", found.Reference, payment.Reference)
		}

		if found.Amount != payment.Amount {
			t.Fatalf("found.Amount = %d, want %d", found.Amount, payment.Amount)
		}
	})

	t.Run("create with duplicate reference", func(t *testing.T) {
		// Create first payment
		firstPayment := &models.Payment{
			Reference: "duplicate-ref",
			Amount:    1000,
			Currency:  "USD",
			Status:    models.PaymentStatusPending,
			Provider:  "stripe",
		}

		err := repo.Create(firstPayment)
		if err != nil {
			t.Fatalf("first Create() error = %v", err)
		}

		// Try to create second payment with same reference
		secondPayment := &models.Payment{
			Reference: "duplicate-ref",
			Amount:    2000,
			Currency:  "USD",
			Status:    models.PaymentStatusPending,
			Provider:  "paypal",
		}

		err = repo.Create(secondPayment)
		if err == nil {
			t.Fatal("Create() with duplicate reference error = nil, want error")
		}
	})
}

func TestFindByID(t *testing.T) {
	gormDB := setupTestDB(t)
	repo := NewGormPaymentRepository(testDB{gormDB: gormDB})

	// Create a payment
	payment := &models.Payment{
		Reference: "test-ref-123",
		Amount:    1000,
		Currency:  "USD",
		Status:    models.PaymentStatusPending,
		Provider:  "stripe",
	}

	err := repo.Create(payment)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("found", func(t *testing.T) {
		found, err := repo.FindByID(payment.ID)
		if err != nil {
			t.Fatalf("FindByID() error = %v, want nil", err)
		}

		if found == nil {
			t.Fatal("FindByID() returned nil payment")
		}

		if found.Reference != "test-ref-123" {
			t.Fatalf("found.Reference = %s, want test-ref-123", found.Reference)
		}
	})

	t.Run("not found", func(t *testing.T) {
		found, err := repo.FindByID(999)
		if err == nil {
			t.Fatal("FindByID() error = nil, want error for not found")
		}

		if found != nil {
			t.Fatal("FindByID() returned non-nil payment for not found")
		}
	})
}

func TestFindByReference(t *testing.T) {
	gormDB := setupTestDB(t)
	repo := NewGormPaymentRepository(testDB{gormDB: gormDB})

	// Create a payment
	payment := &models.Payment{
		Reference: "test-ref-456",
		Amount:    2000,
		Currency:  "EUR",
		Status:    models.PaymentStatusSucceeded,
		Provider:  "paypal",
	}

	err := repo.Create(payment)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("found", func(t *testing.T) {
		found, err := repo.FindByReference(payment.Reference)
		if err != nil {
			t.Fatalf("FindByReference() error = %v, want nil", err)
		}

		if found == nil {
			t.Fatal("FindByReference() returned nil payment")
		}

		if found.Reference != "test-ref-456" {
			t.Fatalf("found.Reference = %s, want test-ref-456", found.Reference)
		}

		if found.Amount != payment.Amount {
			t.Fatalf("found.Amount = %d, want %d", found.Amount, payment.Amount)
		}
	})

	t.Run("not found", func(t *testing.T) {
		found, err := repo.FindByReference("nonexistent-ref")
		if err == nil {
			t.Fatal("FindByReference() error = nil, want error for not found")
		}

		if found != nil {
			t.Fatal("FindByReference() returned non-nil payment for not found")
		}
	})
}

func TestUpdate(t *testing.T) {
	gormDB := setupTestDB(t)
	repo := NewGormPaymentRepository(testDB{gormDB: gormDB})

	// Create a payment
	payment := &models.Payment{
		Reference: "test-ref-789",
		Amount:    2000,
		Currency:  "USD",
		Status:    models.PaymentStatusPending,
		Provider:  "stripe",
	}

	err := repo.Create(payment)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("successful update", func(t *testing.T) {
		providerReference := "provider-ref-123"
		payment.Amount = 3000
		payment.Currency = "GBP"
		payment.Status = models.PaymentStatusSucceeded
		payment.ProviderReference = &providerReference

		err := repo.Update(payment)
		if err != nil {
			t.Fatalf("Update() error = %v, want nil", err)
		}

		// Verify the update
		found, err := repo.FindByID(payment.ID)
		if err != nil {
			t.Fatalf("FindByID() after Update() error = %v", err)
		}

		if found.Amount != 3000 {
			t.Fatalf("found.Amount = %d, want 3000", found.Amount)
		}

		if found.Currency != "GBP" {
			t.Fatalf("found.Currency = %s, want GBP", found.Currency)
		}

		if found.Status != models.PaymentStatusSucceeded {
			t.Fatalf("found.Status = %s, want %s", found.Status, models.PaymentStatusSucceeded)
		}
		if found.ProviderReference == nil || *found.ProviderReference != providerReference {
			t.Fatalf("found.ProviderReference = %v, want %q", found.ProviderReference, providerReference)
		}
	})

	t.Run("update not found", func(t *testing.T) {
		nonExistentPayment := &models.Payment{
			ID:        999,
			Reference: "nonexistent",
			Amount:    3000,
			Currency:  "GBP",
			Status:    models.PaymentStatusSucceeded,
			Provider:  "stripe",
		}

		err := repo.Update(nonExistentPayment)
		if err == nil {
			t.Fatal("Update() error = nil, want error for not found")
		}
	})
}
