package service

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openBackgroundServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := "file:background-service-stability-" + time.Now().UTC().Format("20060102150405.000000000") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	if err := db.AutoMigrate(
		&models.OperationLog{},
		&models.Order{},
		&models.UserPurchaseStat{},
		&models.Ticket{},
		&models.PaymentMethod{},
		&models.OrderPaymentMethod{},
		&models.PaymentPollingTask{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	return db
}

func TestBackgroundServicesStopIsIdempotent(t *testing.T) {
	db := openBackgroundServiceTestDB(t)
	cfg := &config.Config{
		Order: config.OrderConfig{
			AutoCancelHours: 72,
		},
		Ticket: config.TicketConfig{
			AutoCloseHours: 48,
			Attachment: &config.TicketAttachmentConfig{
				RetentionDays: 7,
			},
		},
		Upload: config.UploadConfig{
			Dir: filepath.Join(t.TempDir(), "uploads"),
		},
	}

	orderCancel := NewOrderCancelService(
		db,
		cfg,
		repository.NewInventoryRepository(db),
		repository.NewPromoCodeRepository(db),
		nil,
		nil,
	)
	ticketAutoClose := NewTicketAutoCloseService(db, cfg)
	ticketAttachmentCleanup := NewTicketAttachmentCleanupService(db, cfg)
	paymentPolling := NewPaymentPollingService(db, nil, nil, cfg)

	services := []struct {
		name    string
		service interface {
			Start()
			Stop()
		}
	}{
		{name: "order_cancel", service: orderCancel},
		{name: "ticket_auto_close", service: ticketAutoClose},
		{name: "ticket_attachment_cleanup", service: ticketAttachmentCleanup},
		{name: "payment_polling", service: paymentPolling},
	}

	for _, tc := range services {
		t.Run(tc.name, func(t *testing.T) {
			tc.service.Start()
			time.Sleep(20 * time.Millisecond)
			tc.service.Stop()
			tc.service.Stop()

			tc.service.Start()
			time.Sleep(20 * time.Millisecond)
			tc.service.Stop()
		})
	}
}

func TestPaymentPollingScanPendingPaymentOrdersRespectsCapacityAndSkipsDuplicates(t *testing.T) {
	db := openBackgroundServiceTestDB(t)

	pm := models.PaymentMethod{
		Name:         "scan-test",
		Type:         models.PaymentMethodTypeBuiltin,
		Enabled:      true,
		PollInterval: 45,
	}
	if err := db.Create(&pm).Error; err != nil {
		t.Fatalf("create payment method failed: %v", err)
	}

	createOrder := func(orderNo string, status models.OrderStatus, userID *uint) models.Order {
		order := models.Order{
			OrderNo:   orderNo,
			UserID:    userID,
			Items:     []models.OrderItem{{SKU: "sku", Name: "item", Quantity: 1, ProductType: models.ProductTypePhysical}},
			Status:    status,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := db.Create(&order).Error; err != nil {
			t.Fatalf("create order %s failed: %v", orderNo, err)
		}
		opm := models.OrderPaymentMethod{
			OrderID:         order.ID,
			PaymentMethodID: pm.ID,
		}
		if err := db.Create(&opm).Error; err != nil {
			t.Fatalf("create order payment method for %s failed: %v", orderNo, err)
		}
		return order
	}

	user1 := uint(101)
	user2 := uint(102)
	user3 := uint(103)
	pending1 := createOrder("ORD-PENDING-1", models.OrderStatusPendingPayment, &user1)
	pending2 := createOrder("ORD-PENDING-2", models.OrderStatusPendingPayment, &user2)
	pending3 := createOrder("ORD-PENDING-3", models.OrderStatusPendingPayment, &user3)
	_ = pending3
	shipped := createOrder("ORD-SHIPPED-1", models.OrderStatusShipped, &user1)
	_ = shipped

	svc := NewPaymentPollingService(db, nil, nil, &config.Config{
		Order: config.OrderConfig{
			MaxPaymentPollingTasksGlobal: 2,
		},
	})

	added, err := svc.addRecoveredTask(&PollingTask{
		OrderID:         pending1.ID,
		UserID:          user1,
		PaymentMethodID: pm.ID,
		AddedAt:         time.Now(),
		NextCheckAt:     time.Now(),
		CheckInterval:   pm.PollInterval,
	})
	if err != nil {
		t.Fatalf("preload pending task failed: %v", err)
	}
	if !added {
		t.Fatalf("expected initial recovered task to be added")
	}

	addedCount := svc.scanPendingPaymentOrders()
	if addedCount != 1 {
		t.Fatalf("expected 1 scanned task to be added, got %d", addedCount)
	}

	tasks := svc.GetQueueStatus()
	if len(tasks) != 2 {
		t.Fatalf("expected queue size 2, got %d", len(tasks))
	}

	seen := make(map[uint]bool, len(tasks))
	for _, task := range tasks {
		seen[task.OrderID] = true
		if task.OrderID == shipped.ID {
			t.Fatalf("non-pending order should not be queued")
		}
	}
	if !seen[pending1.ID] {
		t.Fatalf("expected existing pending order to remain queued")
	}
	if !seen[pending2.ID] {
		t.Fatalf("expected next pending order to be queued")
	}
	if seen[pending3.ID] {
		t.Fatalf("expected queue capacity to prevent third pending order from being queued")
	}
}

func TestPaymentPollingRecoverTasksRestoresValidAndCleansInvalidRecords(t *testing.T) {
	db := openBackgroundServiceTestDB(t)

	pm := models.PaymentMethod{
		Name:         "recover-test",
		Type:         models.PaymentMethodTypeBuiltin,
		Enabled:      true,
		PollInterval: 45,
	}
	if err := db.Create(&pm).Error; err != nil {
		t.Fatalf("create payment method failed: %v", err)
	}

	createOrder := func(orderNo string, status models.OrderStatus, userID *uint) models.Order {
		order := models.Order{
			OrderNo:     orderNo,
			UserID:      userID,
			Items:       []models.OrderItem{{SKU: "sku", Name: "item", Quantity: 1, ProductType: models.ProductTypePhysical}},
			Status:      status,
			TotalAmount: 100,
			Currency:    "CNY",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := db.Create(&order).Error; err != nil {
			t.Fatalf("create order %s failed: %v", orderNo, err)
		}
		opm := models.OrderPaymentMethod{
			OrderID:         order.ID,
			PaymentMethodID: pm.ID,
		}
		if err := db.Create(&opm).Error; err != nil {
			t.Fatalf("create order payment method for %s failed: %v", orderNo, err)
		}
		return order
	}

	validUserID := uint(201)
	extraUserID := uint(202)
	shippedUserID := uint(203)
	validOrder := createOrder("ORD-RECOVER-VALID", models.OrderStatusPendingPayment, &validUserID)
	extraPendingOrder := createOrder("ORD-RECOVER-EXTRA", models.OrderStatusPendingPayment, &extraUserID)
	shippedOrder := createOrder("ORD-RECOVER-SHIPPED", models.OrderStatusShipped, &shippedUserID)

	mustCreateTaskRecord := func(orderID uint, task PollingTask) {
		data, err := json.Marshal(task)
		if err != nil {
			t.Fatalf("marshal polling task failed: %v", err)
		}
		if err := db.Create(&models.PaymentPollingTask{
			OrderID: orderID,
			Data:    string(data),
		}).Error; err != nil {
			t.Fatalf("create polling task record failed: %v", err)
		}
	}

	mustCreateTaskRecord(validOrder.ID, PollingTask{
		OrderID:         validOrder.ID,
		UserID:          validUserID,
		PaymentMethodID: pm.ID,
		AddedAt:         time.Now().Add(-10 * time.Minute),
		NextCheckAt:     time.Now().Add(5 * time.Minute),
		CheckInterval:   15,
		RetryCount:      3,
	})
	mustCreateTaskRecord(shippedOrder.ID, PollingTask{
		OrderID:         shippedOrder.ID,
		UserID:          shippedUserID,
		PaymentMethodID: pm.ID,
		AddedAt:         time.Now().Add(-10 * time.Minute),
		NextCheckAt:     time.Now().Add(5 * time.Minute),
		CheckInterval:   30,
		RetryCount:      1,
	})
	mustCreateTaskRecord(999901, PollingTask{
		OrderID:         999901,
		UserID:          999,
		PaymentMethodID: pm.ID,
		AddedAt:         time.Now().Add(-10 * time.Minute),
		NextCheckAt:     time.Now().Add(5 * time.Minute),
		CheckInterval:   30,
		RetryCount:      1,
	})
	if err := db.Create(&models.PaymentPollingTask{
		OrderID: 999902,
		Data:    "{invalid-json",
	}).Error; err != nil {
		t.Fatalf("create malformed polling task record failed: %v", err)
	}

	svc := NewPaymentPollingService(db, nil, nil, &config.Config{
		Order: config.OrderConfig{
			MaxPaymentPollingTasksGlobal: 10,
		},
	})

	svc.recoverTasks()

	tasks := svc.GetQueueStatus()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 recovered/scanned tasks, got %d", len(tasks))
	}

	taskByOrderID := make(map[uint]PollingTask, len(tasks))
	for _, task := range tasks {
		taskByOrderID[task.OrderID] = task
	}

	validTask, exists := taskByOrderID[validOrder.ID]
	if !exists {
		t.Fatalf("expected valid order %d to be recovered", validOrder.ID)
	}
	if validTask.UserID != validUserID {
		t.Fatalf("expected valid task user id %d, got %d", validUserID, validTask.UserID)
	}
	if validTask.CheckInterval != pm.PollInterval {
		t.Fatalf("expected valid task interval %d, got %d", pm.PollInterval, validTask.CheckInterval)
	}
	if time.Since(validTask.NextCheckAt) > time.Second {
		t.Fatalf("expected valid task next check to be reset near now, got %s", validTask.NextCheckAt)
	}

	extraTask, exists := taskByOrderID[extraPendingOrder.ID]
	if !exists {
		t.Fatalf("expected extra pending order %d to be scanned into queue", extraPendingOrder.ID)
	}
	if extraTask.CheckInterval != pm.PollInterval {
		t.Fatalf("expected extra task interval %d, got %d", pm.PollInterval, extraTask.CheckInterval)
	}

	var persisted []models.PaymentPollingTask
	if err := db.Order("order_id ASC").Find(&persisted).Error; err != nil {
		t.Fatalf("query persisted polling tasks failed: %v", err)
	}
	if len(persisted) != 2 {
		t.Fatalf("expected 2 persisted polling task records after cleanup, got %d", len(persisted))
	}
	if persisted[0].OrderID != validOrder.ID || persisted[1].OrderID != extraPendingOrder.ID {
		t.Fatalf("unexpected persisted task order ids after cleanup: %+v", persisted)
	}
}

func TestPaymentPollingBackfillsQueueAfterSlotIsFreed(t *testing.T) {
	db := openBackgroundServiceTestDB(t)

	pm := models.PaymentMethod{
		Name:         "backfill-test",
		Type:         models.PaymentMethodTypeBuiltin,
		Enabled:      true,
		PollInterval: 45,
	}
	if err := db.Create(&pm).Error; err != nil {
		t.Fatalf("create payment method failed: %v", err)
	}

	createOrder := func(orderNo string, status models.OrderStatus, userID *uint) models.Order {
		order := models.Order{
			OrderNo:     orderNo,
			UserID:      userID,
			Items:       []models.OrderItem{{SKU: "sku", Name: "item", Quantity: 1, ProductType: models.ProductTypePhysical}},
			Status:      status,
			TotalAmount: 100,
			Currency:    "CNY",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := db.Create(&order).Error; err != nil {
			t.Fatalf("create order %s failed: %v", orderNo, err)
		}
		if err := db.Create(&models.OrderPaymentMethod{
			OrderID:         order.ID,
			PaymentMethodID: pm.ID,
		}).Error; err != nil {
			t.Fatalf("create order payment method for %s failed: %v", orderNo, err)
		}
		return order
	}

	user1 := uint(301)
	user2 := uint(302)
	firstOrder := createOrder("ORD-BACKFILL-1", models.OrderStatusPendingPayment, &user1)
	secondOrder := createOrder("ORD-BACKFILL-2", models.OrderStatusPendingPayment, &user2)

	svc := NewPaymentPollingService(db, nil, nil, &config.Config{
		Order: config.OrderConfig{
			MaxPaymentPollingTasksGlobal: 1,
		},
	})
	svc.Start()
	defer svc.Stop()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tasks := svc.GetQueueStatus()
		if len(tasks) == 1 && tasks[0].OrderID == firstOrder.ID {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	tasks := svc.GetQueueStatus()
	if len(tasks) != 1 || tasks[0].OrderID != firstOrder.ID {
		t.Fatalf("expected first order to occupy the only queue slot, got %+v", tasks)
	}

	if err := db.Model(&models.Order{}).
		Where("id = ?", firstOrder.ID).
		Update("status", models.OrderStatusDraft).Error; err != nil {
		t.Fatalf("update first order status failed: %v", err)
	}

	svc.RemoveFromQueue(firstOrder.ID)

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tasks = svc.GetQueueStatus()
		if len(tasks) == 1 && tasks[0].OrderID == secondOrder.ID {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if len(tasks) != 1 || tasks[0].OrderID != secondOrder.ID {
		t.Fatalf("expected second order to be backfilled after first slot was freed, got %+v", tasks)
	}

	var persisted []models.PaymentPollingTask
	if err := db.Order("order_id ASC").Find(&persisted).Error; err != nil {
		t.Fatalf("query persisted polling tasks failed: %v", err)
	}
	if len(persisted) != 1 || persisted[0].OrderID != secondOrder.ID {
		t.Fatalf("expected only second order task to remain persisted, got %+v", persisted)
	}
}

func TestEmailServiceQueueEmailSkipsWhenDisabled(t *testing.T) {
	svc := NewEmailService(nil, &config.SMTPConfig{}, "https://example.com")
	if err := svc.QueueEmail("user@example.com", "subject", "<p>content</p>", "order.created", nil, nil); err != nil {
		t.Fatalf("expected disabled email service to skip queueing, got err=%v", err)
	}
}
