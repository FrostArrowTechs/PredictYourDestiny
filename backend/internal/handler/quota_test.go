package handler

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"predictdestiny/internal/model"
)

func TestIncrementUsageConcurrentLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:quota_concurrency?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// Goroutines still race at the service boundary; one DB connection makes
	// SQLite deterministic while PostgreSQL enforces the same unique-row rule.
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.UsageQuota{}); err != nil {
		t.Fatal(err)
	}

	const requests, limit = 50, 5
	var wg sync.WaitGroup
	results := make(chan error, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- IncrementUsage(db, 42, limit)
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, ErrQuotaExceeded) {
			t.Fatalf("unexpected increment error: %v", err)
		}
	}
	if successes != limit {
		t.Fatalf("successful reservations = %d, want %d", successes, limit)
	}

	var quota model.UsageQuota
	if err := db.Where("user_id = ?", 42).First(&quota).Error; err != nil {
		t.Fatal(err)
	}
	if quota.Count != limit {
		t.Fatalf("stored count = %d, want %d", quota.Count, limit)
	}
}

func TestIncrementUsageUnlimitedDoesNotCreateCounter(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.UsageQuota{}); err != nil {
		t.Fatal(err)
	}
	if err := IncrementUsage(db, 7, -1); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.UsageQuota{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("unlimited tier created %d counter rows", count)
	}
}

func TestReserveAIRequestRejectsDuplicateWithoutDoubleCharge(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:idempotent_quota?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.UsageQuota{}, &model.AIRequestReservation{}); err != nil {
		t.Fatal(err)
	}

	if err := ReserveAIRequest(db, 99, 5, "request-123"); err != nil {
		t.Fatalf("first reservation: %v", err)
	}
	if err := ReserveAIRequest(db, 99, 5, "request-123"); !errors.Is(err, ErrDuplicateAIRequest) {
		t.Fatalf("duplicate error = %v", err)
	}

	var quota model.UsageQuota
	if err := db.Where("user_id = ?", 99).First(&quota).Error; err != nil {
		t.Fatal(err)
	}
	if quota.Count != 1 {
		t.Fatalf("quota count = %d, want 1", quota.Count)
	}
	var reservations int64
	if err := db.Model(&model.AIRequestReservation{}).Count(&reservations).Error; err != nil {
		t.Fatal(err)
	}
	if reservations != 1 {
		t.Fatalf("reservation count = %d, want 1", reservations)
	}
}

func TestReserveAIRequestRollsBackKeyWhenQuotaExceeded(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:idempotent_rollback?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.UsageQuota{}, &model.AIRequestReservation{}); err != nil {
		t.Fatal(err)
	}
	if err := ReserveAIRequest(db, 100, 0, "retry-after-upgrade"); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("quota error = %v", err)
	}
	if err := ReserveAIRequest(db, 100, 1, "retry-after-upgrade"); err != nil {
		t.Fatalf("rolled-back key could not be retried: %v", err)
	}
}

func TestReserveAIRequestWithCostIsAtomicAndBounded(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:cost_reservation?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.UsageQuota{}, &model.AIRequestReservation{}, &model.AIDailyCostUsage{}, &model.AICostReservation{}); err != nil {
		t.Fatal(err)
	}
	if err := ReserveAIRequestWithCost(db, 8, 10, "one", "request-one", 100, 60); err != nil {
		t.Fatal(err)
	}
	if err := ReserveAIRequestWithCost(db, 8, 10, "two", "request-two", 100, 60); !errors.Is(err, ErrCostBudgetExceeded) {
		t.Fatalf("second reservation error = %v", err)
	}
	var quota model.UsageQuota
	db.Where("user_id = ?", 8).First(&quota)
	if quota.Count != 1 {
		t.Fatalf("rolled-back request count = %d, want 1", quota.Count)
	}
	var idempotencyRows int64
	db.Model(&model.AIRequestReservation{}).Where("user_id = ?", 8).Count(&idempotencyRows)
	if idempotencyRows != 1 {
		t.Fatalf("idempotency rows = %d, want 1", idempotencyRows)
	}
	var daily model.AIDailyCostUsage
	db.Where("user_id = ?", 8).First(&daily)
	if daily.ReservedMicros != 60 || daily.SpentMicros != 0 {
		t.Fatalf("daily cost = %+v", daily)
	}
}

func TestReserveAIRequestWithCostConcurrentBudget(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:cost_reservation_concurrent?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.UsageQuota{}, &model.AIRequestReservation{}, &model.AIDailyCostUsage{}, &model.AICostReservation{}); err != nil {
		t.Fatal(err)
	}
	const requests = 20
	results := make(chan error, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			key := fmt.Sprintf("cost-%d", index)
			results <- ReserveAIRequestWithCost(db, 9, 100, key, "request-"+key, 100, 30)
		}(i)
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrCostBudgetExceeded) {
			t.Fatalf("unexpected reservation error: %v", err)
		}
	}
	if successes != 3 {
		t.Fatalf("successful cost reservations = %d, want 3", successes)
	}
}
