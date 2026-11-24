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

type lifecycleDB struct {
	db *gorm.DB
}

func (d lifecycleDB) GetDB() *gorm.DB {
	return d.db
}

func TestTransitionStatusWithPostgres(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_DSN to a disposable PostgreSQL database to test payment status transitions")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{TablePrefix: fmt.Sprintf("test_%d_", time.Now().UnixNano())},
	})
	if err != nil {
		t.Fatalf("open PostgreSQL test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL connection: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Migrator().DropTable(&models.Payment{}); err != nil {
			t.Errorf("drop test payments table: %v", err)
		}
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close PostgreSQL connection: %v", err)
		}
	})
	if err := db.AutoMigrate(&models.Payment{}); err != nil {
		t.Fatalf("migrate PostgreSQL test schema: %v", err)
	}

	repository := NewGormPaymentRepository(lifecycleDB{db: db})
	payment := &models.Payment{
		Reference: "lifecycle-test-ref",
		Amount:    1000,
		Currency:  "USD",
		Status:    models.PaymentStatusPending,
		Provider:  "paystack",
	}
	if err := repository.Create(payment); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	updated, err := repository.TransitionStatus(payment.ID, models.PaymentStatusPending, models.PaymentStatusSucceeded)
	if err != nil {
		t.Fatalf("TransitionStatus() error = %v", err)
	}
	if !updated {
		t.Fatal("TransitionStatus() updated = false, want true")
	}

	stored, err := repository.FindByID(payment.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if stored.Status != models.PaymentStatusSucceeded {
		t.Fatalf("stored status = %q, want %q", stored.Status, models.PaymentStatusSucceeded)
	}

	updated, err = repository.TransitionStatus(payment.ID, models.PaymentStatusPending, models.PaymentStatusFailed)
	if err != nil {
		t.Fatalf("TransitionStatus() with stale expected status error = %v", err)
	}
	if updated {
		t.Fatal("TransitionStatus() with stale expected status updated = true, want false")
	}
	stored, err = repository.FindByID(payment.ID)
	if err != nil {
		t.Fatalf("FindByID() after stale transition error = %v", err)
	}
	if stored.Status != models.PaymentStatusSucceeded {
		t.Fatalf("stored status after stale transition = %q, want %q", stored.Status, models.PaymentStatusSucceeded)
	}
}
