package service

import (
	"testing"

	"auralogic/internal/models"
	"auralogic/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBuildUserConsumptionStatsDelta(t *testing.T) {
	tests := []struct {
		name           string
		before         models.OrderStatus
		after          models.OrderStatus
		totalAmount    int64
		expectedSpent  int64
		expectedOrders int64
	}{
		{
			name:           "pending payment to draft enters tracked states",
			before:         models.OrderStatusPendingPayment,
			after:          models.OrderStatusDraft,
			totalAmount:    1200,
			expectedSpent:  1200,
			expectedOrders: 1,
		},
		{
			name:           "draft to pending stays tracked",
			before:         models.OrderStatusDraft,
			after:          models.OrderStatusPending,
			totalAmount:    1200,
			expectedSpent:  0,
			expectedOrders: 0,
		},
		{
			name:           "pending to cancelled leaves tracked states",
			before:         models.OrderStatusPending,
			after:          models.OrderStatusCancelled,
			totalAmount:    1200,
			expectedSpent:  -1200,
			expectedOrders: -1,
		},
		{
			name:           "shipped to completed stays tracked",
			before:         models.OrderStatusShipped,
			after:          models.OrderStatusCompleted,
			totalAmount:    1200,
			expectedSpent:  0,
			expectedOrders: 0,
		},
		{
			name:           "delete tracked order removes stats",
			before:         models.OrderStatusDraft,
			after:          "",
			totalAmount:    1200,
			expectedSpent:  -1200,
			expectedOrders: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spentDelta, orderDelta := buildUserConsumptionStatsDelta(tt.before, tt.after, tt.totalAmount)
			if spentDelta != tt.expectedSpent || orderDelta != tt.expectedOrders {
				t.Fatalf(
					"expected spent_delta=%d order_delta=%d, got spent_delta=%d order_delta=%d",
					tt.expectedSpent,
					tt.expectedOrders,
					spentDelta,
					orderDelta,
				)
			}
		})
	}
}

func TestApplyUserConsumptionStatsDeltaUpdatesUserStats(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:user-consumption-delta?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	user := models.User{
		UUID:            "user-consumption-delta",
		Email:           "user-consumption-delta@example.com",
		Name:            "user-consumption-delta",
		Role:            "user",
		IsActive:        true,
		PasswordHash:    "hash",
		TotalSpentMinor: 500,
		TotalOrderCount: 1,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	repo := repository.NewUserRepository(db)
	if err := repo.ApplyConsumptionStatsDelta(user.ID, 300, 1); err != nil {
		t.Fatalf("apply positive delta failed: %v", err)
	}
	if err := repo.ApplyConsumptionStatsDelta(user.ID, -200, -1); err != nil {
		t.Fatalf("apply negative delta failed: %v", err)
	}

	var refreshed models.User
	if err := db.First(&refreshed, user.ID).Error; err != nil {
		t.Fatalf("reload user failed: %v", err)
	}
	if refreshed.TotalSpentMinor != 600 {
		t.Fatalf("expected total_spent_minor=600, got %d", refreshed.TotalSpentMinor)
	}
	if refreshed.TotalOrderCount != 1 {
		t.Fatalf("expected total_order_count=1, got %d", refreshed.TotalOrderCount)
	}
}
