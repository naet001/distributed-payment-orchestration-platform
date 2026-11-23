package repositories

import (
	"fmt"
	"os"
	"testing"
	"time"

	"example.com/payment-platform/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type paymentTestDB struct {
	conn *gorm.DB
}

func (db paymentTestDB) GetDB() *gorm.DB {
	return db.conn
}

func TestPaymentRepositoryListPagination(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to a disposable PostgreSQL database to run repository integration tests")
	}

	tablePrefix := fmt.Sprintf("test_%d_", time.Now().UnixNano())
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: tablePrefix},
	})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL connection: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close PostgreSQL connection: %v", err)
		}
	})
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(&models.Payment{}); err != nil {
			t.Errorf("drop test payments table: %v", err)
		}
	})

	if err := db.AutoMigrate(&models.Payment{}); err != nil {
		t.Fatalf("migrate PostgreSQL test schema: %v", err)
	}

	payments := make([]models.Payment, 101)
	for i := range payments {
		id := uint(len(payments) - i)
		payments[i] = models.Payment{
			ID:        id,
			Reference: fmt.Sprintf("test-payment-%03d", id),
			Amount:    100,
			Currency:  "USD",
			Status:    models.PaymentStatusPending,
			Provider:  "test",
		}
	}
	if err := db.Create(&payments).Error; err != nil {
		t.Fatalf("insert test payments: %v", err)
	}

	repository := NewGormPaymentRepository(paymentTestDB{conn: db})

	page, err := repository.List(2, 3)
	if err != nil {
		t.Fatalf("list payment page: %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("page length = %d, want 2", len(page))
	}
	if page[0].ID != 4 || page[1].ID != 5 {
		t.Fatalf("page IDs = [%d, %d], want [4, 5]", page[0].ID, page[1].ID)
	}

	maxPage, err := repository.List(MaxPaymentPageSize, 0)
	if err != nil {
		t.Fatalf("list maximum-size page: %v", err)
	}
	if len(maxPage) != MaxPaymentPageSize {
		t.Fatalf("maximum-size page length = %d, want %d", len(maxPage), MaxPaymentPageSize)
	}
	if maxPage[0].ID != 1 || maxPage[len(maxPage)-1].ID != MaxPaymentPageSize {
		t.Fatalf("maximum-size page IDs range from %d to %d, want 1 to %d", maxPage[0].ID, maxPage[len(maxPage)-1].ID, MaxPaymentPageSize)
	}
}

func TestPaymentRepositoryCreate(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to a disposable PostgreSQL database to run repository integration tests")
	}

	tablePrefix := fmt.Sprintf("test_%d_", time.Now().UnixNano())
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: tablePrefix},
	})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL connection: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close PostgreSQL connection: %v", err)
		}
	})
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(&models.Payment{}); err != nil {
			t.Errorf("drop test payments table: %v", err)
		}
	})

	if err := db.AutoMigrate(&models.Payment{}); err != nil {
		t.Fatalf("migrate PostgreSQL test schema: %v", err)
	}

	repository := NewGormPaymentRepository(paymentTestDB{conn: db})

	payment := &models.Payment{
		Reference: "create-test-ref",
		Amount:    5000,
		Currency:  "USD",
		Status:    models.PaymentStatusPending,
		Provider:  "stripe",
	}

	err = repository.Create(payment)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if payment.ID == 0 {
		t.Fatal("Create() did not set payment ID")
	}

	// Verify the payment was actually created
	found, err := repository.FindByID(payment.ID)
	if err != nil {
		t.Fatalf("FindByID() after Create() error = %v", err)
	}

	if found.Reference != payment.Reference {
		t.Fatalf("found.Reference = %s, want %s", found.Reference, payment.Reference)
	}

	if found.Amount != payment.Amount {
		t.Fatalf("found.Amount = %d, want %d", found.Amount, payment.Amount)
	}
}

func TestPaymentRepositoryCreateDuplicateReference(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to a disposable PostgreSQL database to run repository integration tests")
	}

	tablePrefix := fmt.Sprintf("test_%d_", time.Now().UnixNano())
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: tablePrefix},
	})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL connection: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close PostgreSQL connection: %v", err)
		}
	})
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(&models.Payment{}); err != nil {
			t.Errorf("drop test payments table: %v", err)
		}
	})

	if err := db.AutoMigrate(&models.Payment{}); err != nil {
		t.Fatalf("migrate PostgreSQL test schema: %v", err)
	}

	repository := NewGormPaymentRepository(paymentTestDB{conn: db})

	// Create first payment
	firstPayment := &models.Payment{
		Reference: "duplicate-test-ref",
		Amount:    1000,
		Currency:  "USD",
		Status:    models.PaymentStatusPending,
		Provider:  "stripe",
	}

	err = repository.Create(firstPayment)
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	// Try to create second payment with same reference
	secondPayment := &models.Payment{
		Reference: "duplicate-test-ref",
		Amount:    2000,
		Currency:  "USD",
		Status:    models.PaymentStatusPending,
		Provider:  "paypal",
	}

	err = repository.Create(secondPayment)
	if err == nil {
		t.Fatal("Create() with duplicate reference error = nil, want error")
	}
}

func TestPaymentRepositoryFindByID(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to a disposable PostgreSQL database to run repository integration tests")
	}

	tablePrefix := fmt.Sprintf("test_%d_", time.Now().UnixNano())
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: tablePrefix},
	})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL connection: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close PostgreSQL connection: %v", err)
		}
	})
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(&models.Payment{}); err != nil {
			t.Errorf("drop test payments table: %v", err)
		}
	})

	if err := db.AutoMigrate(&models.Payment{}); err != nil {
		t.Fatalf("migrate PostgreSQL test schema: %v", err)
	}

	repository := NewGormPaymentRepository(paymentTestDB{conn: db})

	// Create a payment
	payment := &models.Payment{
		Reference: "find-by-id-test-ref",
		Amount:    7500,
		Currency:  "EUR",
		Status:    models.PaymentStatusSucceeded,
		Provider:  "stripe",
	}

	err = repository.Create(payment)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Find by ID
	found, err := repository.FindByID(payment.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}

	if found == nil {
		t.Fatal("FindByID() returned nil")
	}

	if found.ID != payment.ID {
		t.Fatalf("found.ID = %d, want %d", found.ID, payment.ID)
	}

	if found.Reference != payment.Reference {
		t.Fatalf("found.Reference = %s, want %s", found.Reference, payment.Reference)
	}

	// Find non-existent ID
	_, err = repository.FindByID(99999)
	if err == nil {
		t.Fatal("FindByID() for non-existent ID error = nil, want error")
	}
}

func TestPaymentRepositoryFindByReference(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to a disposable PostgreSQL database to run repository integration tests")
	}

	tablePrefix := fmt.Sprintf("test_%d_", time.Now().UnixNano())
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: tablePrefix},
	})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL connection: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close PostgreSQL connection: %v", err)
		}
	})
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(&models.Payment{}); err != nil {
			t.Errorf("drop test payments table: %v", err)
		}
	})

	if err := db.AutoMigrate(&models.Payment{}); err != nil {
		t.Fatalf("migrate PostgreSQL test schema: %v", err)
	}

	repository := NewGormPaymentRepository(paymentTestDB{conn: db})

	// Create a payment
	payment := &models.Payment{
		Reference: "find-by-ref-test-ref",
		Amount:    3000,
		Currency:  "GBP",
		Status:    models.PaymentStatusPending,
		Provider:  "paypal",
	}

	err = repository.Create(payment)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Find by reference
	found, err := repository.FindByReference(payment.Reference)
	if err != nil {
		t.Fatalf("FindByReference() error = %v", err)
	}

	if found == nil {
		t.Fatal("FindByReference() returned nil")
	}

	if found.Reference != payment.Reference {
		t.Fatalf("found.Reference = %s, want %s", found.Reference, payment.Reference)
	}

	if found.Amount != payment.Amount {
		t.Fatalf("found.Amount = %d, want %d", found.Amount, payment.Amount)
	}

	// Find non-existent reference
	_, err = repository.FindByReference("nonexistent-reference")
	if err == nil {
		t.Fatal("FindByReference() for non-existent reference error = nil, want error")
	}
}

func TestPaymentRepositoryUpdate(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to a disposable PostgreSQL database to run repository integration tests")
	}

	tablePrefix := fmt.Sprintf("test_%d_", time.Now().UnixNano())
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: tablePrefix},
	})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL connection: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close PostgreSQL connection: %v", err)
		}
	})
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(&models.Payment{}); err != nil {
			t.Errorf("drop test payments table: %v", err)
		}
	})

	if err := db.AutoMigrate(&models.Payment{}); err != nil {
		t.Fatalf("migrate PostgreSQL test schema: %v", err)
	}

	repository := NewGormPaymentRepository(paymentTestDB{conn: db})

	// Create a payment
	payment := &models.Payment{
		Reference: "update-test-ref",
		Amount:    1000,
		Currency:  "USD",
		Status:    models.PaymentStatusPending,
		Provider:  "stripe",
	}

	err = repository.Create(payment)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update the payment
	payment.Amount = 5000
	payment.Status = models.PaymentStatusSucceeded

	err = repository.Update(payment)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify the update
	found, err := repository.FindByID(payment.ID)
	if err != nil {
		t.Fatalf("FindByID() after Update() error = %v", err)
	}

	if found.Amount != 5000 {
		t.Fatalf("found.Amount = %d, want 5000", found.Amount)
	}

	if found.Status != models.PaymentStatusSucceeded {
		t.Fatalf("found.Status = %s, want %s", found.Status, models.PaymentStatusSucceeded)
	}

	// Update non-existent payment
	nonExistentPayment := &models.Payment{
		ID:       99999,
		Reference: "nonexistent",
		Amount:    1000,
		Currency:  "USD",
		Status:    models.PaymentStatusPending,
		Provider:  "stripe",
	}

	err = repository.Update(nonExistentPayment)
	if err == nil {
		t.Fatal("Update() for non-existent payment error = nil, want error")
	}
}
