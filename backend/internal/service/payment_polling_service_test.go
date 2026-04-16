package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPaymentPollingServiceTestDB(t *testing.T) (*PaymentPollingService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(
		&models.OperationLog{},
		&models.Order{},
		&models.PaymentMethod{},
		&models.PaymentPollingTask{},
	); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	svc := NewPaymentPollingService(db, nil, nil, &config.Config{})
	return svc, db
}

func TestPaymentPollingAddToQueueRebindsExistingTaskToLatestPaymentMethod(t *testing.T) {
	svc, db := newPaymentPollingServiceTestDB(t)

	userID := uint(42)
	order := &models.Order{
		OrderNo:     "ORDER-POLL-REBIND",
		UserID:      &userID,
		Status:      models.OrderStatusPendingPayment,
		TotalAmount: 100,
		Currency:    "CNY",
		Items: []models.OrderItem{{
			SKU:         "SKU-1",
			Name:        "Item 1",
			Quantity:    1,
			ProductType: models.ProductTypePhysical,
		}},
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	pm1 := &models.PaymentMethod{Name: "USDT", Enabled: true, PollInterval: 30}
	pm2 := &models.PaymentMethod{Name: "Custom", Enabled: true, PollInterval: 7}
	if err := db.Create(pm1).Error; err != nil {
		t.Fatalf("create pm1: %v", err)
	}
	if err := db.Create(pm2).Error; err != nil {
		t.Fatalf("create pm2: %v", err)
	}

	if err := svc.AddToQueue(order.ID, pm1.ID); err != nil {
		t.Fatalf("add first task: %v", err)
	}

	task := svc.taskMap[order.ID]
	if task == nil {
		t.Fatalf("expected task in queue after first add")
	}
	task.RetryCount = 9
	task.AddedAt = time.Now().Add(-10 * time.Minute)
	task.NextCheckAt = time.Now().Add(10 * time.Minute)
	if task.index >= 0 && task.index < len(svc.taskHeap) {
		svc.taskHeap[task.index] = task
	}

	if err := svc.AddToQueue(order.ID, pm2.ID); err != nil {
		t.Fatalf("rebind task: %v", err)
	}

	updatedTask := svc.taskMap[order.ID]
	if updatedTask == nil {
		t.Fatalf("expected task to stay in queue after rebind")
	}
	if updatedTask.PaymentMethodID != pm2.ID {
		t.Fatalf("expected payment method id %d, got %d", pm2.ID, updatedTask.PaymentMethodID)
	}
	if updatedTask.CheckInterval != pm2.PollInterval {
		t.Fatalf("expected check interval %d, got %d", pm2.PollInterval, updatedTask.CheckInterval)
	}
	if updatedTask.RetryCount != 0 {
		t.Fatalf("expected retry count reset to 0, got %d", updatedTask.RetryCount)
	}
	if time.Since(updatedTask.AddedAt) > time.Second {
		t.Fatalf("expected added_at to be refreshed, got %s", updatedTask.AddedAt)
	}
	if time.Since(updatedTask.NextCheckAt) > time.Second {
		t.Fatalf("expected next_check_at to be refreshed, got %s", updatedTask.NextCheckAt)
	}
	if len(svc.taskMap) != 1 || len(svc.taskHeap) != 1 {
		t.Fatalf("expected single task after rebind, got map=%d heap=%d", len(svc.taskMap), len(svc.taskHeap))
	}

	var persisted models.PaymentPollingTask
	if err := db.Where("order_id = ?", order.ID).First(&persisted).Error; err != nil {
		t.Fatalf("load persisted task: %v", err)
	}
	var persistedPayload PollingTask
	if err := json.Unmarshal([]byte(persisted.Data), &persistedPayload); err != nil {
		t.Fatalf("unmarshal persisted task: %v", err)
	}
	if persistedPayload.PaymentMethodID != pm2.ID {
		t.Fatalf("expected persisted payment method id %d, got %d", pm2.ID, persistedPayload.PaymentMethodID)
	}
}
