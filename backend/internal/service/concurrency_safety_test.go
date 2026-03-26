package service

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/cache"
	"auralogic/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openConcurrentServiceTestDB(t *testing.T, migrations ...interface{}) *gorm.DB {
	t.Helper()

	dbPath := filepath.ToSlash(filepath.Join(t.TempDir(), "concurrency-test.db"))
	dsn := fmt.Sprintf("file:%s?_busy_timeout=10000&_journal_mode=WAL", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db failed: %v", err)
	}
	sqlDB.SetMaxOpenConns(16)
	sqlDB.SetMaxIdleConns(16)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	allMigrations := []interface{}{
		&models.Inventory{},
		&models.ProductInventoryBinding{},
		&models.UserPurchaseStat{},
	}
	allMigrations = append(allMigrations, migrations...)

	if err := db.AutoMigrate(allMigrations...); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return db
}

func newConcurrentOrderService(db *gorm.DB, cfg *config.Config, serialSvc *SerialService) *OrderService {
	return NewOrderService(
		repository.NewOrderRepository(db),
		repository.NewUserRepository(db),
		repository.NewProductRepository(db),
		repository.NewInventoryRepository(db),
		nil,
		serialSvc,
		nil,
		nil,
		cfg,
		nil,
	)
}

func TestReserveMessageRateLimitSlotIsAtomic(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis failed: %v", err)
	}
	defer mr.Close()

	previousClient := cache.RedisClient
	cache.RedisClient = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() {
		if cache.RedisClient != nil {
			_ = cache.RedisClient.Close()
		}
		cache.RedisClient = previousClient
	}()

	rl := config.MessageRateLimit{Hourly: 3}
	var allowed atomic.Int64
	var denied atomic.Int64

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ok, availableAt, err := reserveMessageRateLimitSlot("email", "atomic@example.com", rl)
			if err != nil {
				t.Errorf("reserve rate limit slot failed: %v", err)
				return
			}
			if ok {
				allowed.Add(1)
				return
			}
			if availableAt.IsZero() {
				t.Errorf("expected denied request to include availability time")
			}
			denied.Add(1)
		}()
	}

	close(start)
	wg.Wait()

	if got := allowed.Load(); got != 3 {
		t.Fatalf("expected 3 allowed requests, got %d", got)
	}
	if got := denied.Load(); got != 9 {
		t.Fatalf("expected 9 denied requests, got %d", got)
	}
}

func TestCreateUserOrderPendingLimitEnforcedAcrossServiceInstances(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{}, &models.Product{}, &models.Order{})

	cfg := &config.Config{}
	cfg.Order.MaxOrderItems = 20
	cfg.Order.MaxItemQuantity = 10
	cfg.Order.MaxPendingPaymentOrdersPerUser = 1
	cfg.Order.Currency = "CNY"
	cfg.Form.ExpireHours = 24

	user := models.User{
		UUID:         "user-concurrency-1",
		Email:        "limit@example.com",
		Name:         "limit",
		Role:         "user",
		IsActive:     true,
		PasswordHash: "hash",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	product := models.Product{
		SKU:         "SKU-VIRTUAL-LIMIT",
		Name:        "Virtual Limit",
		ProductType: models.ProductTypeVirtual,
		Status:      models.ProductStatusActive,
		Price:       100,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	svc1 := newConcurrentOrderService(db, cfg, nil)
	svc2 := newConcurrentOrderService(db, cfg, nil)

	items := []models.OrderItem{{
		SKU:         product.SKU,
		Name:        product.Name,
		Quantity:    1,
		ProductType: models.ProductTypeVirtual,
	}}

	start := make(chan struct{})
	var wg sync.WaitGroup
	type result struct {
		err error
	}
	results := make(chan result, 2)

	for _, svc := range []*OrderService{svc1, svc2} {
		wg.Add(1)
		go func(current *OrderService) {
			defer wg.Done()
			<-start
			_, err := current.CreateUserOrder(user.ID, items, "", "")
			results <- result{err: err}
		}(svc)
	}

	close(start)
	wg.Wait()
	close(results)

	successes := 0
	failures := 0
	for res := range results {
		if res.err == nil {
			successes++
			continue
		}
		failures++
		var bizErr *bizerr.Error
		if !errors.As(res.err, &bizErr) || bizErr.Key != "order.pendingPaymentLimitExceeded" {
			t.Fatalf("expected pending payment limit biz error, got %v", res.err)
		}
	}

	if successes != 1 || failures != 1 {
		t.Fatalf("expected 1 success and 1 failure, got success=%d failure=%d", successes, failures)
	}

	var pendingCount int64
	if err := db.Model(&models.Order{}).
		Where("user_id = ? AND status = ?", user.ID, models.OrderStatusPendingPayment).
		Count(&pendingCount).Error; err != nil {
		t.Fatalf("count pending orders failed: %v", err)
	}
	if pendingCount != 1 {
		t.Fatalf("expected exactly 1 pending payment order, got %d", pendingCount)
	}
}

func TestCreateUserOrderPurchaseLimitEnforcedAcrossServiceInstances(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{}, &models.Product{}, &models.Order{})

	cfg := &config.Config{}
	cfg.Order.MaxOrderItems = 20
	cfg.Order.MaxItemQuantity = 10
	cfg.Order.MaxPendingPaymentOrdersPerUser = 0
	cfg.Order.Currency = "CNY"
	cfg.Form.ExpireHours = 24

	user := models.User{
		UUID:         "user-concurrency-purchase-limit",
		Email:        "purchase-limit@example.com",
		Name:         "purchase-limit",
		Role:         "user",
		IsActive:     true,
		PasswordHash: "hash",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	product := models.Product{
		SKU:              "SKU-VIRTUAL-PURCHASE-LIMIT",
		Name:             "Virtual Purchase Limit",
		ProductType:      models.ProductTypeVirtual,
		Status:           models.ProductStatusActive,
		Price:            100,
		MaxPurchaseLimit: 1,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	svc1 := newConcurrentOrderService(db, cfg, nil)
	svc2 := newConcurrentOrderService(db, cfg, nil)

	items := []models.OrderItem{{
		SKU:         product.SKU,
		Name:        product.Name,
		Quantity:    1,
		ProductType: models.ProductTypeVirtual,
	}}

	start := make(chan struct{})
	var wg sync.WaitGroup
	type result struct {
		err error
	}
	results := make(chan result, 2)

	for _, svc := range []*OrderService{svc1, svc2} {
		wg.Add(1)
		go func(current *OrderService) {
			defer wg.Done()
			<-start
			_, err := current.CreateUserOrder(user.ID, items, "", "")
			results <- result{err: err}
		}(svc)
	}

	close(start)
	wg.Wait()
	close(results)

	successes := 0
	failures := 0
	for res := range results {
		if res.err == nil {
			successes++
			continue
		}
		failures++
		var bizErr *bizerr.Error
		if !errors.As(res.err, &bizErr) {
			t.Fatalf("expected purchase limit biz error, got %v", res.err)
		}
		if bizErr.Key != "order.purchaseLimitReached" && bizErr.Key != "order.purchaseLimitExceeded" {
			t.Fatalf("expected purchase limit biz error, got %v", res.err)
		}
	}

	if successes != 1 || failures != 1 {
		t.Fatalf("expected 1 success and 1 failure, got success=%d failure=%d", successes, failures)
	}

	var totalPurchased int
	orders, _, err := repository.NewOrderRepository(db).FindByUserID(user.ID, 1, 10, "")
	if err != nil {
		t.Fatalf("query user orders failed: %v", err)
	}
	for _, order := range orders {
		if order.Status == models.OrderStatusCancelled {
			continue
		}
		for _, item := range order.Items {
			if item.SKU == product.SKU {
				totalPurchased += item.Quantity
			}
		}
	}
	if totalPurchased != 1 {
		t.Fatalf("expected non-cancelled purchased quantity to remain 1, got %d", totalPurchased)
	}
}

func TestInventoryReserveLimitHoldsUnderConcurrency(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.InventoryLog{})

	inventory := models.Inventory{
		Name:              "Concurrent Inventory",
		SKU:               "INV-CONCURRENCY-LIMIT",
		Stock:             3,
		AvailableQuantity: 3,
		IsActive:          true,
	}
	if err := db.Create(&inventory).Error; err != nil {
		t.Fatalf("create inventory failed: %v", err)
	}

	repo := repository.NewInventoryRepository(db)
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, 8)

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			results <- repo.Reserve(inventory.ID, 1, fmt.Sprintf("ORD-INV-%d", index+1))
		}(i)
	}

	close(start)
	wg.Wait()
	close(results)

	successes := 0
	failures := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		failures++
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "database is locked") || strings.Contains(message, "database table is locked") {
			t.Fatalf("unexpected sqlite lock error during reserve: %v", err)
		}
	}

	if successes != 3 || failures != 5 {
		t.Fatalf("expected 3 successes and 5 failures, got success=%d failure=%d", successes, failures)
	}

	var refreshed models.Inventory
	if err := db.First(&refreshed, inventory.ID).Error; err != nil {
		t.Fatalf("reload inventory failed: %v", err)
	}
	if refreshed.ReservedQuantity != 3 {
		t.Fatalf("expected reserved quantity 3, got %d", refreshed.ReservedQuantity)
	}
}

func TestPromoCodeReserveLimitHoldsUnderConcurrency(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.PromoCode{})

	promo := models.PromoCode{
		Code:             "PROMO-CONCURRENCY",
		Name:             "Concurrency Promo",
		DiscountType:     models.DiscountTypeFixed,
		DiscountValue:    100,
		Status:           models.PromoCodeStatusActive,
		TotalQuantity:    2,
		UsedQuantity:     0,
		ReservedQuantity: 0,
	}
	if err := db.Create(&promo).Error; err != nil {
		t.Fatalf("create promo code failed: %v", err)
	}

	repo := repository.NewPromoCodeRepository(db)
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, 6)

	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			results <- repo.Reserve(promo.ID, fmt.Sprintf("ORD-PROMO-%d", index+1))
		}(i)
	}

	close(start)
	wg.Wait()
	close(results)

	successes := 0
	failures := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		failures++
		message := strings.ToLower(err.Error())
		if strings.Contains(message, "database is locked") || strings.Contains(message, "database table is locked") {
			t.Fatalf("unexpected sqlite lock error during promo reserve: %v", err)
		}
	}

	if successes != 2 || failures != 4 {
		t.Fatalf("expected 2 successes and 4 failures, got success=%d failure=%d", successes, failures)
	}

	var refreshed models.PromoCode
	if err := db.First(&refreshed, promo.ID).Error; err != nil {
		t.Fatalf("reload promo code failed: %v", err)
	}
	if refreshed.ReservedQuantity != 2 {
		t.Fatalf("expected reserved quantity 2, got %d", refreshed.ReservedQuantity)
	}
}

func TestPaymentPollingPerUserQueueLimitHoldsUnderConcurrency(t *testing.T) {
	db := openConcurrentServiceTestDB(t,
		&models.User{},
		&models.Order{},
		&models.PaymentMethod{},
		&models.OrderPaymentMethod{},
		&models.PaymentPollingTask{},
	)

	user := models.User{
		UUID:         "polling-user-limit",
		Email:        "polling-user-limit@example.com",
		Name:         "polling-user-limit",
		Role:         "user",
		IsActive:     true,
		PasswordHash: "hash",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	pm := models.PaymentMethod{
		Name:         "Polling Limit Pay",
		Type:         models.PaymentMethodTypeBuiltin,
		Enabled:      true,
		PollInterval: 30,
	}
	if err := db.Create(&pm).Error; err != nil {
		t.Fatalf("create payment method failed: %v", err)
	}

	orderIDs := make([]uint, 0, 3)
	for i := 0; i < 3; i++ {
		userID := user.ID
		order := models.Order{
			OrderNo:     fmt.Sprintf("ORD-POLL-USER-LIMIT-%d", i+1),
			UserID:      &userID,
			Status:      models.OrderStatusPendingPayment,
			TotalAmount: 100,
			Currency:    "CNY",
			Items: []models.OrderItem{{
				SKU:         fmt.Sprintf("SKU-POLL-USER-%d", i+1),
				Name:        "Polling Item",
				Quantity:    1,
				ProductType: models.ProductTypePhysical,
			}},
		}
		if err := db.Create(&order).Error; err != nil {
			t.Fatalf("create order failed: %v", err)
		}
		if err := db.Create(&models.OrderPaymentMethod{
			OrderID:         order.ID,
			PaymentMethodID: pm.ID,
		}).Error; err != nil {
			t.Fatalf("create order payment method failed: %v", err)
		}
		orderIDs = append(orderIDs, order.ID)
	}

	svc := NewPaymentPollingService(db, nil, nil, &config.Config{
		Order: config.OrderConfig{
			MaxPaymentPollingTasksPerUser: 2,
			MaxPaymentPollingTasksGlobal:  10,
		},
	})

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, len(orderIDs))
	for _, orderID := range orderIDs {
		wg.Add(1)
		go func(currentOrderID uint) {
			defer wg.Done()
			<-start
			results <- svc.AddToQueue(currentOrderID, pm.ID)
		}(orderID)
	}

	close(start)
	wg.Wait()
	close(results)

	successes := 0
	failures := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		failures++
		var bizErr *bizerr.Error
		if !errors.As(err, &bizErr) || bizErr.Key != "payment.pollingUserQueueLimitExceeded" {
			t.Fatalf("expected per-user polling queue limit error, got %v", err)
		}
	}

	if successes != 2 || failures != 1 {
		t.Fatalf("expected 2 successes and 1 failure, got success=%d failure=%d", successes, failures)
	}

	if queueSize := len(svc.GetQueueStatus()); queueSize != 2 {
		t.Fatalf("expected queue size 2, got %d", queueSize)
	}

	var persisted int64
	if err := db.Model(&models.PaymentPollingTask{}).Count(&persisted).Error; err != nil {
		t.Fatalf("count persisted polling tasks failed: %v", err)
	}
	if persisted != 2 {
		t.Fatalf("expected 2 persisted polling tasks, got %d", persisted)
	}
}

func TestPaymentPollingGlobalQueueLimitHoldsUnderConcurrency(t *testing.T) {
	db := openConcurrentServiceTestDB(t,
		&models.User{},
		&models.Order{},
		&models.PaymentMethod{},
		&models.OrderPaymentMethod{},
		&models.PaymentPollingTask{},
	)

	pm := models.PaymentMethod{
		Name:         "Polling Global Limit Pay",
		Type:         models.PaymentMethodTypeBuiltin,
		Enabled:      true,
		PollInterval: 30,
	}
	if err := db.Create(&pm).Error; err != nil {
		t.Fatalf("create payment method failed: %v", err)
	}

	orderIDs := make([]uint, 0, 3)
	for i := 0; i < 3; i++ {
		user := models.User{
			UUID:         fmt.Sprintf("polling-global-limit-%d", i+1),
			Email:        fmt.Sprintf("polling-global-limit-%d@example.com", i+1),
			Name:         fmt.Sprintf("polling-global-limit-%d", i+1),
			Role:         "user",
			IsActive:     true,
			PasswordHash: "hash",
		}
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("create user failed: %v", err)
		}
		userID := user.ID
		order := models.Order{
			OrderNo:     fmt.Sprintf("ORD-POLL-GLOBAL-LIMIT-%d", i+1),
			UserID:      &userID,
			Status:      models.OrderStatusPendingPayment,
			TotalAmount: 100,
			Currency:    "CNY",
			Items: []models.OrderItem{{
				SKU:         fmt.Sprintf("SKU-POLL-GLOBAL-%d", i+1),
				Name:        "Polling Item",
				Quantity:    1,
				ProductType: models.ProductTypePhysical,
			}},
		}
		if err := db.Create(&order).Error; err != nil {
			t.Fatalf("create order failed: %v", err)
		}
		if err := db.Create(&models.OrderPaymentMethod{
			OrderID:         order.ID,
			PaymentMethodID: pm.ID,
		}).Error; err != nil {
			t.Fatalf("create order payment method failed: %v", err)
		}
		orderIDs = append(orderIDs, order.ID)
	}

	svc := NewPaymentPollingService(db, nil, nil, &config.Config{
		Order: config.OrderConfig{
			MaxPaymentPollingTasksPerUser: 10,
			MaxPaymentPollingTasksGlobal:  2,
		},
	})

	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, len(orderIDs))
	for _, orderID := range orderIDs {
		wg.Add(1)
		go func(currentOrderID uint) {
			defer wg.Done()
			<-start
			results <- svc.AddToQueue(currentOrderID, pm.ID)
		}(orderID)
	}

	close(start)
	wg.Wait()
	close(results)

	successes := 0
	failures := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		failures++
		var bizErr *bizerr.Error
		if !errors.As(err, &bizErr) || bizErr.Key != "payment.pollingGlobalQueueLimitExceeded" {
			t.Fatalf("expected global polling queue limit error, got %v", err)
		}
	}

	if successes != 2 || failures != 1 {
		t.Fatalf("expected 2 successes and 1 failure, got success=%d failure=%d", successes, failures)
	}

	if queueSize := len(svc.GetQueueStatus()); queueSize != 2 {
		t.Fatalf("expected queue size 2, got %d", queueSize)
	}

	var persisted int64
	if err := db.Model(&models.PaymentPollingTask{}).Count(&persisted).Error; err != nil {
		t.Fatalf("count persisted polling tasks failed: %v", err)
	}
	if persisted != 2 {
		t.Fatalf("expected 2 persisted polling tasks, got %d", persisted)
	}
}

func TestSubmitShippingFormConcurrentSubmissionCreatesSerialsOnce(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{}, &models.Product{}, &models.Order{}, &models.ProductSerial{})

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
		SKU:         "SKU-FORM-PHYSICAL",
		Name:        "Physical Form Product",
		ProductCode: "PFP",
		ProductType: models.ProductTypePhysical,
		Status:      models.ProductStatusActive,
		Price:       100,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	formToken := "form-concurrency-token"
	formExpiresAt := time.Now().Add(24 * time.Hour)
	order := models.Order{
		OrderNo: "ORD-FORM-CONCURRENCY",
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
	svc := newConcurrentOrderService(db, cfg, serialSvc)

	receiverInfo := map[string]interface{}{
		"receiver_name":     "Receiver",
		"phone_code":        "+86",
		"receiver_phone":    "13800000000",
		"receiver_email":    "receiver@example.com",
		"receiver_country":  "CN",
		"receiver_province": "Shanghai",
		"receiver_city":     "Shanghai",
		"receiver_district": "Pudong",
		"receiver_address":  "No.1 Test Road",
		"receiver_postcode": "200000",
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	var successes atomic.Int64
	errs := make(chan error, 8)

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _, _, err := svc.SubmitShippingForm(formToken, receiverInfo, false, "Valid123!", "", nil)
			if err == nil {
				successes.Add(1)
				return
			}
			errs <- err
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	if got := successes.Load(); got != 1 {
		t.Fatalf("expected exactly 1 successful submission, got %d", got)
	}
	for err := range errs {
		message := strings.ToLower(err.Error())
		if !strings.Contains(message, "already been submitted") && !strings.Contains(message, "not ready") {
			t.Fatalf("unexpected submission error: %v", err)
		}
	}

	var serialCount int64
	if err := db.Model(&models.ProductSerial{}).Where("order_id = ?", order.ID).Count(&serialCount).Error; err != nil {
		t.Fatalf("count serials failed: %v", err)
	}
	if serialCount != 2 {
		t.Fatalf("expected 2 serials for the order, got %d", serialCount)
	}

	var userCount int64
	if err := db.Model(&models.User{}).Where("email = ?", "receiver@example.com").Count(&userCount).Error; err != nil {
		t.Fatalf("count users failed: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("expected 1 created user, got %d", userCount)
	}
}

func TestSubmitShippingFormRollsBackWhenSerialCreationFails(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{}, &models.Product{}, &models.Order{}, &models.ProductSerial{})

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
		SKU:         "SKU-FORM-ROLLBACK",
		Name:        "Rollback Physical Product",
		ProductCode: "RBP",
		ProductType: models.ProductTypePhysical,
		Status:      models.ProductStatusActive,
		Price:       100,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	formToken := "form-rollback-token"
	formExpiresAt := time.Now().Add(24 * time.Hour)
	order := models.Order{
		OrderNo: "ORD-FORM-ROLLBACK",
		Items: []models.OrderItem{{
			SKU:         product.SKU,
			Name:        product.Name,
			Quantity:    1,
			ProductType: models.ProductTypePhysical,
		}},
		Status:        models.OrderStatusDraft,
		TotalAmount:   100,
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
	svc := newConcurrentOrderService(db, cfg, serialSvc)

	if err := db.Migrator().DropTable(&models.ProductSerial{}); err != nil {
		t.Fatalf("drop product_serials failed: %v", err)
	}

	receiverInfo := map[string]interface{}{
		"receiver_name":     "Rollback Receiver",
		"phone_code":        "+86",
		"receiver_phone":    "13800000001",
		"receiver_email":    "rollback@example.com",
		"receiver_country":  "CN",
		"receiver_province": "Shanghai",
		"receiver_city":     "Shanghai",
		"receiver_district": "Pudong",
		"receiver_address":  "Rollback Road 1",
		"receiver_postcode": "200000",
	}

	_, _, _, err := svc.SubmitShippingForm(formToken, receiverInfo, false, "Valid123!", "", nil)
	if err == nil {
		t.Fatal("expected serial creation failure, got nil")
	}
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "failed to create serials") && !strings.Contains(message, "product_serials") {
		t.Fatalf("unexpected submit error: %v", err)
	}

	var refreshed models.Order
	if err := db.First(&refreshed, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if refreshed.Status != models.OrderStatusDraft {
		t.Fatalf("expected order status to remain draft, got %s", refreshed.Status)
	}
	if refreshed.FormSubmittedAt != nil {
		t.Fatal("expected form submission timestamp to roll back")
	}
	if refreshed.UserID != nil {
		t.Fatalf("expected order user binding to roll back, got user_id=%d", *refreshed.UserID)
	}

	var userCount int64
	if err := db.Model(&models.User{}).Where("email = ?", "rollback@example.com").Count(&userCount).Error; err != nil {
		t.Fatalf("count rollback user failed: %v", err)
	}
	if userCount != 0 {
		t.Fatalf("expected rollback to remove created user, got %d users", userCount)
	}
}

func TestSerialServiceCreateSerialForOrderAllocatesUniqueSequencesConcurrently(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{}, &models.Product{}, &models.Order{}, &models.ProductSerial{})

	product := models.Product{
		SKU:         "SKU-SERIAL-CONCURRENCY",
		Name:        "Serial Product",
		ProductCode: "SER",
		ProductType: models.ProductTypePhysical,
		Status:      models.ProductStatusActive,
		Price:       100,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	var orderIDs []uint
	for i := 0; i < 10; i++ {
		order := models.Order{
			OrderNo: fmt.Sprintf("ORD-SERIAL-%02d", i+1),
			Items: []models.OrderItem{{
				SKU:         product.SKU,
				Name:        product.Name,
				Quantity:    1,
				ProductType: models.ProductTypePhysical,
			}},
			Status:      models.OrderStatusPending,
			TotalAmount: 100,
			Currency:    "CNY",
		}
		if err := db.Create(&order).Error; err != nil {
			t.Fatalf("create order failed: %v", err)
		}
		orderIDs = append(orderIDs, order.ID)
	}

	serialSvc := NewSerialService(
		repository.NewSerialRepository(db),
		repository.NewProductRepository(db),
		repository.NewOrderRepository(db),
	)

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, len(orderIDs))

	for _, orderID := range orderIDs {
		wg.Add(1)
		go func(currentOrderID uint) {
			defer wg.Done()
			<-start
			_, err := serialSvc.CreateSerialForOrder(currentOrderID, product.ID, 1)
			errs <- err
		}(orderID)
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("create serial concurrently failed: %v", err)
		}
	}

	var serials []models.ProductSerial
	if err := db.Where("product_id = ?", product.ID).Order("sequence_number ASC").Find(&serials).Error; err != nil {
		t.Fatalf("query serials failed: %v", err)
	}
	if len(serials) != len(orderIDs) {
		t.Fatalf("expected %d serials, got %d", len(orderIDs), len(serials))
	}

	seen := make(map[int]bool, len(serials))
	for _, serial := range serials {
		if seen[serial.SequenceNumber] {
			t.Fatalf("duplicate sequence number detected: %d", serial.SequenceNumber)
		}
		seen[serial.SequenceNumber] = true
	}

	var updatedProduct models.Product
	if err := db.Select("id", "last_serial_sequence").First(&updatedProduct, product.ID).Error; err != nil {
		t.Fatalf("query updated product failed: %v", err)
	}
	if updatedProduct.LastSerialSequence != len(orderIDs) {
		t.Fatalf("expected last serial sequence %d, got %d", len(orderIDs), updatedProduct.LastSerialSequence)
	}
}

func TestPaymentPollingConfirmPaymentResultIsIdempotentUnderConcurrency(t *testing.T) {
	db := openConcurrentServiceTestDB(t,
		&models.User{},
		&models.OperationLog{},
		&models.Order{},
		&models.PaymentMethod{},
		&models.OrderPaymentMethod{},
		&models.PaymentPollingTask{},
	)

	order := models.Order{
		OrderNo: "ORD-PAYMENT-CONCURRENCY",
		Items: []models.OrderItem{{
			SKU:         "SKU-PAYMENT",
			Name:        "Payment Product",
			Quantity:    1,
			ProductType: models.ProductTypePhysical,
		}},
		Status:      models.OrderStatusPendingPayment,
		TotalAmount: 100,
		Currency:    "CNY",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	pm := models.PaymentMethod{
		Name:         "Concurrent Pay",
		Type:         models.PaymentMethodTypeBuiltin,
		Enabled:      true,
		PollInterval: 30,
	}
	if err := db.Create(&pm).Error; err != nil {
		t.Fatalf("create payment method failed: %v", err)
	}

	opm := models.OrderPaymentMethod{
		OrderID:         order.ID,
		PaymentMethodID: pm.ID,
	}
	if err := db.Create(&opm).Error; err != nil {
		t.Fatalf("create order payment method failed: %v", err)
	}

	svc := NewPaymentPollingService(db, nil, nil, &config.Config{})

	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, 8)

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.ConfirmPaymentResult(order.ID, pm.ID, &PaymentCheckResult{
				Paid:          true,
				TransactionID: "TX-CONCURRENT-1",
			}, "test")
			errs <- err
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("confirm payment result failed: %v", err)
		}
	}

	var refreshed models.Order
	if err := db.First(&refreshed, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if refreshed.Status != models.OrderStatusDraft {
		t.Fatalf("expected order to transition to draft once, got %s", refreshed.Status)
	}

	var successLogs int64
	if err := db.Model(&models.OperationLog{}).
		Where("action = ? AND resource_id = ?", "payment_success", order.ID).
		Count(&successLogs).Error; err != nil {
		t.Fatalf("count payment success logs failed: %v", err)
	}
	if successLogs != 1 {
		t.Fatalf("expected exactly 1 payment_success log, got %d", successLogs)
	}
}
