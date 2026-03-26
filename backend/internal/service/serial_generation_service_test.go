package service

import (
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/repository"
	"gorm.io/gorm"
)

func waitForSerialGenerationCondition(t *testing.T, timeout time.Duration, fn func() (bool, error)) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		ok, err := fn()
		if ok && err == nil {
			return
		}
		lastErr = err
		time.Sleep(20 * time.Millisecond)
	}

	if lastErr != nil {
		t.Fatalf("condition not satisfied before timeout: %v", lastErr)
	}
	t.Fatal("condition not satisfied before timeout")
}

func TestSubmitShippingFormQueuesSerialTaskWhenAsyncServiceEnabled(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{}, &models.Product{}, &models.Order{}, &models.ProductSerial{}, &models.SerialGenerationTask{})

	cfg := &config.Config{}
	cfg.Order.MaxOrderItems = 20
	cfg.Order.MaxItemQuantity = 10
	cfg.Form.ExpireHours = 24
	cfg.Security.PasswordPolicy.MinLength = 8
	cfg.Security.PasswordPolicy.RequireUppercase = true
	cfg.Security.PasswordPolicy.RequireLowercase = true
	cfg.Security.PasswordPolicy.RequireNumber = true
	cfg.Security.PasswordPolicy.RequireSpecial = true

	product := models.Product{
		SKU:         "SKU-FORM-ASYNC",
		Name:        "Async Serial Product",
		ProductCode: "ASY",
		ProductType: models.ProductTypePhysical,
		Status:      models.ProductStatusActive,
		Price:       100,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	formToken := "form-async-token"
	formExpiresAt := time.Now().Add(24 * time.Hour)
	order := models.Order{
		OrderNo: "ORD-FORM-ASYNC",
		Items: []models.OrderItem{{
			SKU:         product.SKU,
			Name:        product.Name,
			Quantity:    2,
			ProductType: models.ProductTypePhysical,
		}},
		Status:        models.OrderStatusDraft,
		TotalAmount:   200,
		Currency:      "CNY",
		FormToken:     &formToken,
		FormExpiresAt: &formExpiresAt,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	serialSvc := NewSerialService(
		repository.NewSerialRepository(db),
		repository.NewProductRepository(db),
		repository.NewOrderRepository(db),
	)
	serialTaskSvc := NewSerialGenerationService(db, serialSvc)
	svc := newConcurrentOrderService(db, cfg, serialSvc)
	svc.SetSerialGenerationService(serialTaskSvc)

	receiverInfo := map[string]interface{}{
		"receiver_name":     "Receiver",
		"phone_code":        "+86",
		"receiver_phone":    "13800000000",
		"receiver_email":    "receiver-async@example.com",
		"receiver_country":  "CN",
		"receiver_province": "Shanghai",
		"receiver_city":     "Shanghai",
		"receiver_district": "Pudong",
		"receiver_address":  "No.1 Async Road",
		"receiver_postcode": "200000",
	}

	if _, _, _, err := svc.SubmitShippingForm(formToken, receiverInfo, false, "Valid123!", "", nil); err != nil {
		t.Fatalf("submit shipping form failed: %v", err)
	}

	var serialCount int64
	if err := db.Model(&models.ProductSerial{}).Where("order_id = ?", order.ID).Count(&serialCount).Error; err != nil {
		t.Fatalf("count serials failed: %v", err)
	}
	if serialCount != 0 {
		t.Fatalf("expected no serials to be created synchronously, got %d", serialCount)
	}

	var task models.SerialGenerationTask
	if err := db.Where("order_id = ?", order.ID).First(&task).Error; err != nil {
		t.Fatalf("load serial generation task failed: %v", err)
	}
	if task.Status != models.SerialGenerationStatusQueued {
		t.Fatalf("expected queued task, got %s", task.Status)
	}

	var refreshed models.Order
	if err := db.First(&refreshed, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if refreshed.SerialGenerationStatus != models.SerialGenerationStatusQueued {
		t.Fatalf("expected queued order serial status, got %s", refreshed.SerialGenerationStatus)
	}
}

func TestSerialGenerationServiceProcessesQueuedTasks(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{}, &models.Product{}, &models.Order{}, &models.ProductSerial{}, &models.SerialGenerationTask{})

	product := models.Product{
		SKU:         "SKU-SERIAL-WORKER",
		Name:        "Worker Product",
		ProductCode: "WRK",
		ProductType: models.ProductTypePhysical,
		Status:      models.ProductStatusActive,
		Price:       100,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	order := models.Order{
		OrderNo: "ORD-SERIAL-WORKER",
		Items: []models.OrderItem{{
			SKU:         product.SKU,
			Name:        product.Name,
			Quantity:    2,
			ProductType: models.ProductTypePhysical,
		}},
		Status:                 models.OrderStatusPending,
		TotalAmount:            200,
		Currency:               "CNY",
		SerialGenerationStatus: models.SerialGenerationStatusQueued,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	serialSvc := NewSerialService(
		repository.NewSerialRepository(db),
		repository.NewProductRepository(db),
		repository.NewOrderRepository(db),
	)
	serialTaskSvc := NewSerialGenerationService(db, serialSvc)
	serialTaskSvc.pollInterval = 20 * time.Millisecond
	if err := db.Transaction(func(tx *gorm.DB) error {
		return serialTaskSvc.EnqueueOrderTx(tx, order.ID)
	}); err != nil {
		t.Fatalf("enqueue serial task failed: %v", err)
	}
	serialTaskSvc.Start()
	defer serialTaskSvc.Stop()

	waitForSerialGenerationCondition(t, 3*time.Second, func() (bool, error) {
		var serialCount int64
		if err := db.Model(&models.ProductSerial{}).Where("order_id = ?", order.ID).Count(&serialCount).Error; err != nil {
			return false, err
		}
		if serialCount != 2 {
			return false, nil
		}

		var task models.SerialGenerationTask
		if err := db.Where("order_id = ?", order.ID).First(&task).Error; err != nil {
			return false, err
		}
		if task.Status != models.SerialGenerationStatusCompleted {
			return false, nil
		}

		var refreshed models.Order
		if err := db.First(&refreshed, order.ID).Error; err != nil {
			return false, err
		}
		if refreshed.SerialGenerationStatus != models.SerialGenerationStatusCompleted {
			return false, nil
		}
		return refreshed.SerialGeneratedAt != nil, nil
	})
}

func TestSerialGenerationServiceCancelsTaskForCancelledOrders(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{}, &models.Product{}, &models.Order{}, &models.ProductSerial{}, &models.SerialGenerationTask{})

	product := models.Product{
		SKU:         "SKU-SERIAL-CANCELLED",
		Name:        "Cancelled Product",
		ProductCode: "CNL",
		ProductType: models.ProductTypePhysical,
		Status:      models.ProductStatusActive,
		Price:       100,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	order := models.Order{
		OrderNo: "ORD-SERIAL-CANCELLED",
		Items: []models.OrderItem{{
			SKU:         product.SKU,
			Name:        product.Name,
			Quantity:    1,
			ProductType: models.ProductTypePhysical,
		}},
		Status:                 models.OrderStatusCancelled,
		TotalAmount:            100,
		Currency:               "CNY",
		SerialGenerationStatus: models.SerialGenerationStatusQueued,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	serialSvc := NewSerialService(
		repository.NewSerialRepository(db),
		repository.NewProductRepository(db),
		repository.NewOrderRepository(db),
	)
	serialTaskSvc := NewSerialGenerationService(db, serialSvc)
	serialTaskSvc.pollInterval = 20 * time.Millisecond
	if err := db.Transaction(func(tx *gorm.DB) error {
		return serialTaskSvc.EnqueueOrderTx(tx, order.ID)
	}); err != nil {
		t.Fatalf("enqueue serial task failed: %v", err)
	}
	serialTaskSvc.Start()
	defer serialTaskSvc.Stop()

	waitForSerialGenerationCondition(t, 3*time.Second, func() (bool, error) {
		var serialCount int64
		if err := db.Model(&models.ProductSerial{}).Where("order_id = ?", order.ID).Count(&serialCount).Error; err != nil {
			return false, err
		}
		if serialCount != 0 {
			return false, nil
		}

		var task models.SerialGenerationTask
		if err := db.Where("order_id = ?", order.ID).First(&task).Error; err != nil {
			return false, err
		}
		if task.Status != models.SerialGenerationStatusCancelled {
			return false, nil
		}

		var refreshed models.Order
		if err := db.First(&refreshed, order.ID).Error; err != nil {
			return false, err
		}
		return refreshed.SerialGenerationStatus == models.SerialGenerationStatusCancelled, nil
	})
}
