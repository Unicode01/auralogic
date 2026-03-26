package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type concurrencyProfileResult struct {
	Workload    string
	Concurrency int
	Total       int
	Success     int
	Failed      int
	Elapsed     time.Duration
	Throughput  float64
	P50         time.Duration
	P95         time.Duration
	P99         time.Duration
	Max         time.Duration
}

func applyProfileHighConcurrencyProtection(cfg *config.Config) {
	if cfg == nil || os.Getenv("AURALOGIC_PROFILE_ENABLE_HOT_PATH_PROTECTION") != "1" {
		return
	}

	protection := config.OrderHighConcurrencyProtectionConfig{
		Enabled:       true,
		Mode:          "memory",
		MaxInFlight:   8,
		WaitTimeoutMs: 5000,
		RedisLeaseMs:  30000,
	}
	if raw := strings.TrimSpace(os.Getenv("AURALOGIC_PROFILE_HOT_PATH_PROTECTION_MODE")); raw != "" {
		protection.Mode = raw
	}
	if raw := strings.TrimSpace(os.Getenv("AURALOGIC_PROFILE_HOT_PATH_PROTECTION_MAX_INFLIGHT")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			protection.MaxInFlight = parsed
		}
	}
	if raw := strings.TrimSpace(os.Getenv("AURALOGIC_PROFILE_HOT_PATH_PROTECTION_WAIT_TIMEOUT_MS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			protection.WaitTimeoutMs = parsed
		}
	}
	if raw := strings.TrimSpace(os.Getenv("AURALOGIC_PROFILE_HOT_PATH_PROTECTION_REDIS_LEASE_MS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			protection.RedisLeaseMs = parsed
		}
	}

	cfg.Order.HighConcurrencyProtection = protection
}

func profileAsyncSerialGenerationEnabled() bool {
	return strings.TrimSpace(os.Getenv("AURALOGIC_PROFILE_ENABLE_ASYNC_SERIAL_GENERATION")) == "1"
}

func profileAsyncSerialGenerationWorkerEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("AURALOGIC_PROFILE_START_ASYNC_SERIAL_WORKER"))
	if raw == "" {
		return true
	}
	return raw == "1"
}

func profileExistingOrdersPerUser() int {
	raw := strings.TrimSpace(os.Getenv("AURALOGIC_PROFILE_EXISTING_ORDERS_PER_USER"))
	if raw == "" {
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}

func profileForcePurchaseStatsFallback() bool {
	return strings.TrimSpace(os.Getenv("AURALOGIC_PROFILE_FORCE_PURCHASE_STATS_FALLBACK")) == "1"
}

func openPerfServiceTestDB(t *testing.T, name string, migrations ...interface{}) *gorm.DB {
	t.Helper()

	dir := t.TempDir()
	dbPath := filepath.ToSlash(filepath.Join(dir, name+".db"))
	dsn := fmt.Sprintf("file:%s?_busy_timeout=10000&_journal_mode=WAL", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db failed: %v", err)
	}
	sqlDB.SetMaxOpenConns(32)
	sqlDB.SetMaxIdleConns(32)
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
	if profileForcePurchaseStatsFallback() {
		if err := db.Migrator().DropTable(&models.UserPurchaseStat{}); err != nil {
			t.Fatalf("drop user purchase stats table failed: %v", err)
		}
	}

	return db
}

func runConcurrencyProfile(totalOps int, concurrency int, fn func(opIndex int) error) concurrencyProfileResult {
	latencies := make([]time.Duration, totalOps)
	failed := 0
	jobs := make(chan int, totalOps)
	errs := make(chan error, totalOps)

	startedAt := time.Now()
	var wg sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for opIndex := range jobs {
				opStartedAt := time.Now()
				err := fn(opIndex)
				latencies[opIndex] = time.Since(opStartedAt)
				errs <- err
			}
		}()
	}

	for opIndex := 0; opIndex < totalOps; opIndex++ {
		jobs <- opIndex
	}
	close(jobs)
	wg.Wait()
	close(errs)

	successLatencies := make([]time.Duration, 0, totalOps)
	for err := range errs {
		if err != nil {
			failed++
			continue
		}
	}
	for opIndex := 0; opIndex < totalOps; opIndex++ {
		if latencies[opIndex] > 0 {
			successLatencies = append(successLatencies, latencies[opIndex])
		}
	}
	sort.Slice(successLatencies, func(i, j int) bool {
		return successLatencies[i] < successLatencies[j]
	})

	elapsed := time.Since(startedAt)
	success := totalOps - failed
	result := concurrencyProfileResult{
		Concurrency: concurrency,
		Total:       totalOps,
		Success:     success,
		Failed:      failed,
		Elapsed:     elapsed,
		Throughput:  float64(success) / elapsed.Seconds(),
	}
	if len(successLatencies) > 0 {
		result.P50 = percentileDuration(successLatencies, 0.50)
		result.P95 = percentileDuration(successLatencies, 0.95)
		result.P99 = percentileDuration(successLatencies, 0.99)
		result.Max = successLatencies[len(successLatencies)-1]
	}
	return result
}

func percentileDuration(values []time.Duration, ratio float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * ratio)
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func seedUsers(t *testing.T, db *gorm.DB, prefix string, total int) []models.User {
	t.Helper()

	users := make([]models.User, 0, total)
	for i := 0; i < total; i++ {
		users = append(users, models.User{
			UUID:         fmt.Sprintf("%s-user-%06d", prefix, i+1),
			Email:        fmt.Sprintf("%s-user-%06d@example.com", prefix, i+1),
			Name:         fmt.Sprintf("%s-user-%06d", prefix, i+1),
			Role:         "user",
			IsActive:     true,
			PasswordHash: "hash",
		})
	}
	if err := db.CreateInBatches(&users, 200).Error; err != nil {
		t.Fatalf("seed users failed: %v", err)
	}
	return users
}

func profileCreateUserOrder(t *testing.T, concurrency int, totalOps int) concurrencyProfileResult {
	db := openPerfServiceTestDB(t, fmt.Sprintf("create-user-order-c%d", concurrency), &models.User{}, &models.Product{}, &models.Order{})

	cfg := &config.Config{}
	cfg.Order.MaxOrderItems = 20
	cfg.Order.MaxItemQuantity = 10
	cfg.Order.MaxPendingPaymentOrdersPerUser = 0
	cfg.Order.Currency = "CNY"
	cfg.Form.ExpireHours = 24
	applyProfileHighConcurrencyProtection(cfg)

	users := seedUsers(t, db, fmt.Sprintf("create-order-c%d", concurrency), totalOps)
	product := models.Product{
		SKU:         fmt.Sprintf("SKU-PERF-VIRTUAL-%d", concurrency),
		Name:        "Perf Virtual Product",
		ProductType: models.ProductTypeVirtual,
		Status:      models.ProductStatusActive,
		Price:       100,
	}
	existingOrdersPerUser := profileExistingOrdersPerUser()
	if existingOrdersPerUser > 0 {
		product.MaxPurchaseLimit = existingOrdersPerUser + totalOps + 64
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	if existingOrdersPerUser > 0 {
		seedHistoricalPurchaseStatsForUsers(t, db, users, product, existingOrdersPerUser)
	}

	svc := newConcurrentOrderService(db, cfg, nil)
	items := []models.OrderItem{{
		SKU:      product.SKU,
		Name:     product.Name,
		Quantity: 1,
	}}

	result := runConcurrencyProfile(totalOps, concurrency, func(opIndex int) error {
		_, err := svc.CreateUserOrder(users[opIndex].ID, items, "", "")
		return err
	})
	result.Workload = "create_user_order"
	if existingOrdersPerUser > 0 {
		result.Workload = "create_user_order_with_history"
		if profileForcePurchaseStatsFallback() {
			result.Workload = "create_user_order_with_history_fallback_scan"
		}
	}
	return result
}

func seedHistoricalPurchaseStatsForUsers(t *testing.T, db *gorm.DB, users []models.User, product models.Product, existingOrdersPerUser int) {
	t.Helper()

	if existingOrdersPerUser <= 0 {
		return
	}

	orders := make([]models.Order, 0, len(users)*existingOrdersPerUser)
	for _, user := range users {
		userID := user.ID
		for i := 0; i < existingOrdersPerUser; i++ {
			orders = append(orders, models.Order{
				OrderNo: fmt.Sprintf("PERF-HISTORY-%d-%06d", user.ID, i+1),
				UserID:  &userID,
				Items: []models.OrderItem{{
					SKU:         product.SKU,
					Name:        product.Name,
					Quantity:    1,
					ProductType: models.ProductTypeVirtual,
				}},
				Status:      models.OrderStatusPendingPayment,
				TotalAmount: product.Price,
				Currency:    "CNY",
			})
		}
	}
	if len(orders) > 0 {
		if err := db.CreateInBatches(&orders, 500).Error; err != nil {
			t.Fatalf("seed historical orders failed: %v", err)
		}
	}

	orderRepo := repository.NewOrderRepository(db)
	for _, user := range users {
		if err := orderRepo.ReplaceUserPurchaseStats(user.ID, map[string]int64{
			product.SKU: int64(existingOrdersPerUser),
		}); err != nil {
			t.Fatalf("seed purchase stats failed for user %d: %v", user.ID, err)
		}
	}
}

func profileSubmitShippingForm(t *testing.T, concurrency int, totalOps int) concurrencyProfileResult {
	migrations := []interface{}{
		&models.User{},
		&models.Product{},
		&models.Order{},
		&models.ProductSerial{},
	}
	if profileAsyncSerialGenerationEnabled() {
		migrations = append(migrations, &models.SerialGenerationTask{})
	}
	db := openPerfServiceTestDB(t, fmt.Sprintf("submit-shipping-c%d", concurrency), migrations...)

	cfg := &config.Config{}
	cfg.Order.MaxOrderItems = 20
	cfg.Order.MaxItemQuantity = 10
	cfg.Form.ExpireHours = 24
	cfg.Security.PasswordPolicy.MinLength = 8
	cfg.Security.PasswordPolicy.RequireUppercase = true
	cfg.Security.PasswordPolicy.RequireLowercase = true
	cfg.Security.PasswordPolicy.RequireNumber = true
	cfg.Security.PasswordPolicy.RequireSpecial = true
	applyProfileHighConcurrencyProtection(cfg)

	product := models.Product{
		SKU:         fmt.Sprintf("SKU-PERF-PHYSICAL-%d", concurrency),
		Name:        "Perf Physical Product",
		ProductCode: "PFX",
		ProductType: models.ProductTypePhysical,
		Status:      models.ProductStatusActive,
		Price:       100,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	now := time.Now()
	orders := make([]models.Order, 0, totalOps)
	for i := 0; i < totalOps; i++ {
		token := fmt.Sprintf("perf-form-token-%d-%06d", concurrency, i+1)
		expiresAt := now.Add(24 * time.Hour)
		orders = append(orders, models.Order{
			OrderNo: fmt.Sprintf("PERF-FORM-%d-%06d", concurrency, i+1),
			Items: []models.OrderItem{{
				SKU:         product.SKU,
				Name:        product.Name,
				Quantity:    1,
				ProductType: models.ProductTypePhysical,
			}},
			Status:        models.OrderStatusDraft,
			TotalAmount:   100,
			Currency:      "CNY",
			FormToken:     &token,
			FormExpiresAt: &expiresAt,
		})
	}
	if err := db.CreateInBatches(&orders, 200).Error; err != nil {
		t.Fatalf("seed orders failed: %v", err)
	}

	serialSvc := NewSerialService(
		repository.NewSerialRepository(db),
		repository.NewProductRepository(db),
		repository.NewOrderRepository(db),
	)
	svc := newConcurrentOrderService(db, cfg, serialSvc)
	if profileAsyncSerialGenerationEnabled() {
		serialTaskSvc := NewSerialGenerationService(db, serialSvc)
		serialTaskSvc.pollInterval = 20 * time.Millisecond
		if profileAsyncSerialGenerationWorkerEnabled() {
			serialTaskSvc.Start()
			defer serialTaskSvc.Stop()
		}
		svc.SetSerialGenerationService(serialTaskSvc)
	}

	result := runConcurrencyProfile(totalOps, concurrency, func(opIndex int) error {
		receiverInfo := map[string]interface{}{
			"receiver_name":     fmt.Sprintf("Receiver-%06d", opIndex+1),
			"phone_code":        "+86",
			"receiver_phone":    fmt.Sprintf("138%08d", (opIndex+1)%100000000),
			"receiver_email":    fmt.Sprintf("receiver-%d-%06d@example.com", concurrency, opIndex+1),
			"receiver_country":  "CN",
			"receiver_province": "Shanghai",
			"receiver_city":     "Shanghai",
			"receiver_district": "Pudong",
			"receiver_address":  fmt.Sprintf("No.%d Perf Road", opIndex+1),
			"receiver_postcode": "200000",
		}
		_, _, _, err := svc.SubmitShippingForm(*orders[opIndex].FormToken, receiverInfo, false, "Valid123!", "", nil)
		return err
	})
	result.Workload = "submit_shipping_form"
	if profileAsyncSerialGenerationEnabled() {
		result.Workload = "submit_shipping_form_async_serial"
		if !profileAsyncSerialGenerationWorkerEnabled() {
			result.Workload = "submit_shipping_form_async_serial_enqueue_only"
		}
	}
	return result
}

func profileConfirmPaymentResult(t *testing.T, concurrency int, totalOps int) concurrencyProfileResult {
	db := openPerfServiceTestDB(t, fmt.Sprintf("confirm-payment-c%d", concurrency),
		&models.User{},
		&models.OperationLog{},
		&models.Order{},
		&models.PaymentMethod{},
		&models.OrderPaymentMethod{},
		&models.PaymentPollingTask{},
	)

	users := seedUsers(t, db, fmt.Sprintf("confirm-payment-c%d", concurrency), totalOps)
	orders := make([]models.Order, 0, totalOps)
	for i := 0; i < totalOps; i++ {
		userID := users[i].ID
		orders = append(orders, models.Order{
			OrderNo:         fmt.Sprintf("PERF-PAY-%d-%06d", concurrency, i+1),
			UserID:          &userID,
			Status:          models.OrderStatusPendingPayment,
			TotalAmount:     100,
			Currency:        "CNY",
			ReceiverName:    fmt.Sprintf("Paid User-%06d", i+1),
			ReceiverAddress: fmt.Sprintf("Paid Address-%06d", i+1),
			Items: []models.OrderItem{{
				SKU:         "SKU-PAY",
				Name:        "Pay Product",
				Quantity:    1,
				ProductType: models.ProductTypePhysical,
			}},
		})
	}
	if err := db.CreateInBatches(&orders, 200).Error; err != nil {
		t.Fatalf("seed payment orders failed: %v", err)
	}

	pm := models.PaymentMethod{
		Name:         fmt.Sprintf("Perf Pay %d", concurrency),
		Type:         models.PaymentMethodTypeBuiltin,
		Enabled:      true,
		PollInterval: 30,
	}
	if err := db.Create(&pm).Error; err != nil {
		t.Fatalf("create payment method failed: %v", err)
	}

	orderPaymentMethods := make([]models.OrderPaymentMethod, 0, totalOps)
	for i := 0; i < totalOps; i++ {
		orderPaymentMethods = append(orderPaymentMethods, models.OrderPaymentMethod{
			OrderID:         orders[i].ID,
			PaymentMethodID: pm.ID,
		})
	}
	if err := db.CreateInBatches(&orderPaymentMethods, 200).Error; err != nil {
		t.Fatalf("seed order payment methods failed: %v", err)
	}

	cfg := &config.Config{}
	applyProfileHighConcurrencyProtection(cfg)
	svc := NewPaymentPollingService(db, nil, nil, cfg)

	result := runConcurrencyProfile(totalOps, concurrency, func(opIndex int) error {
		_, err := svc.ConfirmPaymentResult(orders[opIndex].ID, pm.ID, &PaymentCheckResult{
			Paid:          true,
			TransactionID: fmt.Sprintf("PERF-TX-%d-%06d", concurrency, opIndex+1),
		}, "perf")
		return err
	})
	result.Workload = "confirm_payment_result"
	return result
}

func TestServiceConcurrencyProfile(t *testing.T) {
	if os.Getenv("AURALOGIC_RUN_CONCURRENCY_PROFILE") != "1" {
		t.Skip("set AURALOGIC_RUN_CONCURRENCY_PROFILE=1 to run service concurrency profile")
	}

	totalOps := 200
	if raw := strings.TrimSpace(os.Getenv("AURALOGIC_PROFILE_TOTAL_OPS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			t.Fatalf("invalid AURALOGIC_PROFILE_TOTAL_OPS=%q", raw)
		}
		totalOps = parsed
	}
	levels := []int{1, 4, 8, 12, 16, 24, 32, 48, 64}
	if raw := strings.TrimSpace(os.Getenv("AURALOGIC_PROFILE_LEVELS")); raw != "" {
		parts := strings.Split(raw, ",")
		levels = make([]int, 0, len(parts))
		for _, part := range parts {
			parsed, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil || parsed <= 0 {
				t.Fatalf("invalid concurrency level %q", part)
			}
			levels = append(levels, parsed)
		}
	}

	workloads := []struct {
		name string
		run  func(t *testing.T, concurrency int, totalOps int) concurrencyProfileResult
	}{
		{name: "create_user_order", run: profileCreateUserOrder},
		{name: "submit_shipping_form", run: profileSubmitShippingForm},
		{name: "confirm_payment_result", run: profileConfirmPaymentResult},
	}
	enabledWorkloads := make(map[string]bool, len(workloads))
	for _, workload := range workloads {
		enabledWorkloads[workload.name] = true
	}
	if raw := strings.TrimSpace(os.Getenv("AURALOGIC_PROFILE_WORKLOADS")); raw != "" {
		for key := range enabledWorkloads {
			enabledWorkloads[key] = false
		}
		for _, part := range strings.Split(raw, ",") {
			name := strings.TrimSpace(part)
			if _, exists := enabledWorkloads[name]; !exists {
				t.Fatalf("invalid workload %q", name)
			}
			enabledWorkloads[name] = true
		}
	}

	t.Logf(
		"concurrency profile settings: total_ops_per_level=%d levels=%v workloads=%v async_serial_generation=%v async_serial_worker=%v existing_orders_per_user=%d purchase_stats_fallback=%v",
		totalOps,
		levels,
		enabledWorkloads,
		profileAsyncSerialGenerationEnabled(),
		profileAsyncSerialGenerationWorkerEnabled(),
		profileExistingOrdersPerUser(),
		profileForcePurchaseStatsFallback(),
	)

	for _, workload := range workloads {
		if !enabledWorkloads[workload.name] {
			continue
		}
		t.Logf("=== workload=%s ===", workload.name)
		for _, concurrency := range levels {
			result := workload.run(t, concurrency, totalOps)
			t.Logf(
				"PROFILE workload=%s concurrency=%d total=%d success=%d failed=%d throughput=%.2f/s p50=%s p95=%s p99=%s max=%s elapsed=%s",
				result.Workload,
				result.Concurrency,
				result.Total,
				result.Success,
				result.Failed,
				result.Throughput,
				result.P50,
				result.P95,
				result.P99,
				result.Max,
				result.Elapsed,
			)
		}
	}
}
