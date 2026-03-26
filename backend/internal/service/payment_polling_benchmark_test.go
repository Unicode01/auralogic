package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	oplogger "auralogic/internal/pkg/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type paymentPollingBenchmarkFixture struct {
	db          *gorm.DB
	service     *PaymentPollingService
	orderIDs    []uint
	taskRecords []models.PaymentPollingTask
}

func openPaymentPollingBenchmarkDB(b *testing.B, name string) *gorm.DB {
	b.Helper()

	dir := b.TempDir()
	dbPath := filepath.ToSlash(filepath.Join(dir, name+".db"))
	dsn := fmt.Sprintf("file:%s?_busy_timeout=10000&_journal_mode=WAL", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		b.Fatalf("open sqlite failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		b.Fatalf("get sql db failed: %v", err)
	}
	sqlDB.SetMaxOpenConns(32)
	sqlDB.SetMaxIdleConns(32)
	b.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(
		&models.User{},
		&models.OperationLog{},
		&models.Order{},
		&models.PaymentMethod{},
		&models.OrderPaymentMethod{},
		&models.PaymentPollingTask{},
	); err != nil {
		b.Fatalf("auto migrate failed: %v", err)
	}

	return db
}

func buildPaymentPollingBenchmarkFixture(b *testing.B, total int) paymentPollingBenchmarkFixture {
	b.Helper()

	db := openPaymentPollingBenchmarkDB(b, fmt.Sprintf("payment-polling-bench-%d", total))
	pm := models.PaymentMethod{
		Name:         fmt.Sprintf("bench-pm-%d", total),
		Type:         models.PaymentMethodTypeBuiltin,
		Enabled:      true,
		PollInterval: 45,
	}
	if err := db.Create(&pm).Error; err != nil {
		b.Fatalf("create payment method failed: %v", err)
	}

	orders := make([]models.Order, 0, total)
	for i := 0; i < total; i++ {
		userID := uint(i + 1)
		orders = append(orders, models.Order{
			OrderNo:     fmt.Sprintf("BENCH-POLL-%d-%06d", total, i+1),
			UserID:      &userID,
			Status:      models.OrderStatusPendingPayment,
			TotalAmount: 100,
			Currency:    "CNY",
			Items: []models.OrderItem{{
				SKU:         fmt.Sprintf("SKU-BENCH-%d", i+1),
				Name:        "Bench Item",
				Quantity:    1,
				ProductType: models.ProductTypePhysical,
			}},
		})
	}
	if err := db.CreateInBatches(&orders, 500).Error; err != nil {
		b.Fatalf("create orders failed: %v", err)
	}

	orderPaymentMethods := make([]models.OrderPaymentMethod, 0, total)
	taskRecords := make([]models.PaymentPollingTask, 0, total)
	orderIDs := make([]uint, 0, total)
	now := time.Now().UTC()
	for i := range orders {
		orderPaymentMethods = append(orderPaymentMethods, models.OrderPaymentMethod{
			OrderID:         orders[i].ID,
			PaymentMethodID: pm.ID,
		})
		orderIDs = append(orderIDs, orders[i].ID)

		task := PollingTask{
			OrderID:         orders[i].ID,
			UserID:          *orders[i].UserID,
			PaymentMethodID: pm.ID,
			AddedAt:         now.Add(-10 * time.Minute),
			NextCheckAt:     now.Add(30 * time.Second),
			CheckInterval:   pm.PollInterval,
			RetryCount:      i % 5,
		}
		data, err := json.Marshal(task)
		if err != nil {
			b.Fatalf("marshal task failed: %v", err)
		}
		taskRecords = append(taskRecords, models.PaymentPollingTask{
			OrderID: task.OrderID,
			Data:    string(data),
		})
	}
	if err := db.CreateInBatches(&orderPaymentMethods, 500).Error; err != nil {
		b.Fatalf("create order payment methods failed: %v", err)
	}

	svc := NewPaymentPollingService(db, nil, nil, &config.Config{
		Order: config.OrderConfig{
			MaxPaymentPollingTasksGlobal: total + 10,
		},
	})

	return paymentPollingBenchmarkFixture{
		db:          db,
		service:     svc,
		orderIDs:    orderIDs,
		taskRecords: taskRecords,
	}
}

func resetPaymentPollingBenchmarkPersistedTasks(b *testing.B, fixture paymentPollingBenchmarkFixture, withTasks bool) {
	b.Helper()

	fixture.service.resetQueueState()
	if err := fixture.db.Exec("DELETE FROM payment_polling_tasks").Error; err != nil {
		b.Fatalf("clear payment polling tasks failed: %v", err)
	}
	if !withTasks {
		return
	}
	if len(fixture.taskRecords) == 0 {
		return
	}
	if err := fixture.db.CreateInBatches(&fixture.taskRecords, 500).Error; err != nil {
		b.Fatalf("seed payment polling task records failed: %v", err)
	}
}

func benchmarkScanPendingPaymentOrdersLegacy(s *PaymentPollingService) int {
	addedCount := 0
	now := time.Now()
	lastCursor := uint(0)
	for {
		remaining := s.remainingQueueCapacity()
		if remaining == 0 {
			break
		}

		batchSize := paymentPollingStartupBatchSize
		if remaining > 0 && remaining < batchSize {
			batchSize = remaining
		}

		var rows []pendingPaymentPollingOrderRow
		err := s.db.Table("order_payment_methods AS opm").
			Select("opm.id AS cursor_id, opm.order_id, orders.user_id, opm.payment_method_id, COALESCE(payment_methods.poll_interval, 0) AS poll_interval").
			Joins("JOIN orders ON orders.id = opm.order_id").
			Joins("LEFT JOIN payment_methods ON payment_methods.id = opm.payment_method_id").
			Where("opm.id > ? AND orders.status = ?", lastCursor, models.OrderStatusPendingPayment).
			Order("opm.id ASC").
			Limit(batchSize).
			Scan(&rows).Error
		if err != nil {
			break
		}
		if len(rows) == 0 {
			break
		}

		for _, row := range rows {
			lastCursor = row.CursorID

			queueUserID := uint(0)
			if row.UserID != nil {
				queueUserID = *row.UserID
			}
			interval := s.defaultInterval
			if row.PollInterval > 0 {
				interval = row.PollInterval
			}

			task := &PollingTask{
				OrderID:         row.OrderID,
				UserID:          queueUserID,
				PaymentMethodID: row.PaymentMethodID,
				AddedAt:         now,
				NextCheckAt:     now,
				CheckInterval:   interval,
				RetryCount:      0,
			}
			added, err := s.addRecoveredTask(task)
			if err != nil || !added {
				continue
			}
			s.saveTaskToDB(s.cloneTask(task))
			addedCount++
		}

		if len(rows) < batchSize {
			break
		}
	}

	return addedCount
}

func benchmarkRecoverTasksLegacy(s *PaymentPollingService) {
	var records []models.PaymentPollingTask
	if err := s.db.Find(&records).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}

	recoveredCount := 0
	removedCount := 0

	for _, record := range records {
		var task PollingTask
		if err := json.Unmarshal([]byte(record.Data), &task); err != nil {
			s.removeTaskFromDB(record.OrderID)
			removedCount++
			continue
		}

		var order models.Order
		if err := s.db.Select("id", "user_id", "status").First(&order, task.OrderID).Error; err != nil {
			s.removeTaskFromDB(task.OrderID)
			removedCount++
			continue
		}
		if order.Status != models.OrderStatusPendingPayment {
			s.removeTaskFromDB(task.OrderID)
			removedCount++
			continue
		}
		if time.Since(task.AddedAt) > s.maxDuration {
			s.removeTaskFromDB(task.OrderID)
			removedCount++
			continue
		}
		if order.UserID != nil {
			task.UserID = *order.UserID
		} else {
			task.UserID = 0
		}

		var pm models.PaymentMethod
		if err := s.db.Select("id", "poll_interval").First(&pm, task.PaymentMethodID).Error; err == nil && pm.PollInterval > 0 {
			task.CheckInterval = pm.PollInterval
		}

		task.NextCheckAt = time.Now()
		if added, err := s.addRecoveredTask(s.cloneTask(&task)); err != nil {
			s.removeTaskFromDB(task.OrderID)
			removedCount++
			continue
		} else if !added {
			continue
		}
		recoveredCount++
	}

	addedCount := benchmarkScanPendingPaymentOrdersLegacy(s)
	if recoveredCount > 0 || removedCount > 0 || addedCount > 0 {
		oplogger.LogSystemOperation(s.db, "payment_polling_recover", "system", nil, map[string]interface{}{
			"recovered": recoveredCount,
			"removed":   removedCount,
			"added":     addedCount,
		})
	}
}

func BenchmarkPaymentPollingRecoverTasks(b *testing.B) {
	for _, total := range []int{200, 1000} {
		b.Run(fmt.Sprintf("current/tasks_%d", total), func(b *testing.B) {
			fixture := buildPaymentPollingBenchmarkFixture(b, total)
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				resetPaymentPollingBenchmarkPersistedTasks(b, fixture, true)
				b.StartTimer()

				fixture.service.recoverTasks()

				b.StopTimer()
				if got := len(fixture.service.GetQueueStatus()); got != total {
					b.Fatalf("expected %d recovered tasks, got %d", total, got)
				}
			}
		})

		b.Run(fmt.Sprintf("legacy/tasks_%d", total), func(b *testing.B) {
			fixture := buildPaymentPollingBenchmarkFixture(b, total)
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				resetPaymentPollingBenchmarkPersistedTasks(b, fixture, true)
				b.StartTimer()

				benchmarkRecoverTasksLegacy(fixture.service)

				b.StopTimer()
				if got := len(fixture.service.GetQueueStatus()); got != total {
					b.Fatalf("expected %d recovered tasks, got %d", total, got)
				}
			}
		})
	}
}

func BenchmarkPaymentPollingScanPendingPaymentOrders(b *testing.B) {
	for _, total := range []int{200, 1000} {
		b.Run(fmt.Sprintf("current/orders_%d", total), func(b *testing.B) {
			fixture := buildPaymentPollingBenchmarkFixture(b, total)
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				resetPaymentPollingBenchmarkPersistedTasks(b, fixture, false)
				b.StartTimer()

				addedCount := fixture.service.scanPendingPaymentOrders()

				b.StopTimer()
				if addedCount != total {
					b.Fatalf("expected %d scanned tasks, got %d", total, addedCount)
				}
				if got := len(fixture.service.GetQueueStatus()); got != total {
					b.Fatalf("expected queue size %d, got %d", total, got)
				}
			}
		})

		b.Run(fmt.Sprintf("legacy/orders_%d", total), func(b *testing.B) {
			fixture := buildPaymentPollingBenchmarkFixture(b, total)
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				resetPaymentPollingBenchmarkPersistedTasks(b, fixture, false)
				b.StartTimer()

				addedCount := benchmarkScanPendingPaymentOrdersLegacy(fixture.service)

				b.StopTimer()
				if addedCount != total {
					b.Fatalf("expected %d scanned tasks, got %d", total, addedCount)
				}
				if got := len(fixture.service.GetQueueStatus()); got != total {
					b.Fatalf("expected queue size %d, got %d", total, got)
				}
			}
		})
	}
}
