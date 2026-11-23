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
