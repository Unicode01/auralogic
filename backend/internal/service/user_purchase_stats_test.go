package service

import (
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/repository"
)

func TestCreateUserOrderAndCancelKeepPurchaseStatsInSync(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{}, &models.Product{}, &models.Order{})

	cfg := &config.Config{}
	cfg.Order.MaxOrderItems = 20
	cfg.Order.MaxItemQuantity = 10
	cfg.Order.MaxPendingPaymentOrdersPerUser = 0
	cfg.Order.Currency = "CNY"
	cfg.Form.ExpireHours = 24

	user := models.User{
		UUID:         "purchase-stats-user-create-cancel",
		Email:        "purchase-stats-create-cancel@example.com",
		Name:         "purchase-stats-create-cancel",
		Role:         "user",
		IsActive:     true,
		PasswordHash: "hash",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	product := models.Product{
		SKU:              "SKU-PURCHASE-STATS-CREATE-CANCEL",
		Name:             "Purchase Stats Product",
		ProductType:      models.ProductTypeVirtual,
		Status:           models.ProductStatusActive,
		Price:            100,
		MaxPurchaseLimit: 10,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	svc := newConcurrentOrderService(db, cfg, nil)
	order, err := svc.CreateUserOrder(user.ID, []models.OrderItem{{
		SKU:         product.SKU,
		Name:        product.Name,
		Quantity:    2,
		ProductType: models.ProductTypeVirtual,
	}}, "", "")
	if err != nil {
		t.Fatalf("create user order failed: %v", err)
	}

	orderRepo := repository.NewOrderRepository(db)
	quantity, err := orderRepo.GetUserPurchaseQuantityBySKU(user.ID, product.SKU)
	if err != nil {
		t.Fatalf("query purchase quantity failed: %v", err)
	}
	if quantity != 2 {
		t.Fatalf("expected purchase quantity 2 after create, got %d", quantity)
	}

	if err := svc.CancelOrder(order.ID, "test cancel"); err != nil {
		t.Fatalf("cancel order failed: %v", err)
	}

	quantity, err = orderRepo.GetUserPurchaseQuantityBySKU(user.ID, product.SKU)
	if err != nil {
		t.Fatalf("query purchase quantity after cancel failed: %v", err)
	}
	if quantity != 0 {
		t.Fatalf("expected purchase quantity 0 after cancel, got %d", quantity)
	}
}

func TestSubmitShippingFormTracksPurchaseStatsOnInitialUserBinding(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{}, &models.Product{}, &models.Order{})

	cfg := &config.Config{}
	cfg.Order.MaxOrderItems = 20
	cfg.Order.MaxItemQuantity = 10
	cfg.Form.ExpireHours = 24

	user := models.User{
		UUID:         "purchase-stats-user-submit-form",
		Email:        "purchase-stats-submit-form@example.com",
		Name:         "purchase-stats-submit-form",
		Role:         "user",
		IsActive:     true,
		PasswordHash: "hash",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	formToken := "purchase-stats-form-token"
	expiresAt := time.Now().Add(24 * time.Hour)
	order := models.Order{
		OrderNo: "ORD-PURCHASE-STATS-FORM",
		Items: []models.OrderItem{{
			SKU:         "SKU-PURCHASE-STATS-FORM",
			Name:        "Purchase Stats Form Product",
			Quantity:    3,
			ProductType: models.ProductTypePhysical,
		}},
		Status:        models.OrderStatusDraft,
		TotalAmount:   300,
		Currency:      "CNY",
		FormToken:     &formToken,
		FormExpiresAt: &expiresAt,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	svc := newConcurrentOrderService(db, cfg, nil)
	receiverInfo := map[string]interface{}{
		"receiver_name":     "Receiver",
		"phone_code":        "+86",
		"receiver_phone":    "13800138000",
		"receiver_email":    "receiver@example.com",
		"receiver_country":  "CN",
		"receiver_province": "Shanghai",
		"receiver_city":     "Shanghai",
		"receiver_district": "Pudong",
		"receiver_address":  "No.1 Test Road",
		"receiver_postcode": "200000",
	}
	_, _, _, err := svc.SubmitShippingForm(formToken, receiverInfo, false, "", "", &user.ID)
	if err != nil {
		t.Fatalf("submit shipping form failed: %v", err)
	}

	quantity, err := repository.NewOrderRepository(db).GetUserPurchaseQuantityBySKU(user.ID, "SKU-PURCHASE-STATS-FORM")
	if err != nil {
		t.Fatalf("query purchase quantity failed: %v", err)
	}
	if quantity != 3 {
		t.Fatalf("expected purchase quantity 3 after initial form submit, got %d", quantity)
	}
}

func TestSyncUserPurchaseStatsRebuildsFromOrderHistory(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{}, &models.Order{})

	user := models.User{
		UUID:         "purchase-stats-user-sync",
		Email:        "purchase-stats-sync@example.com",
		Name:         "purchase-stats-sync",
		Role:         "user",
		IsActive:     true,
		PasswordHash: "hash",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	createOrder := func(orderNo string, status models.OrderStatus, quantity int) {
		userID := user.ID
		order := models.Order{
			OrderNo: orderNo,
			UserID:  &userID,
			Items: []models.OrderItem{{
				SKU:         "SKU-PURCHASE-STATS-SYNC",
				Name:        "Purchase Stats Sync Product",
				Quantity:    quantity,
				ProductType: models.ProductTypeVirtual,
			}},
			Status:      status,
			TotalAmount: int64(quantity) * 100,
			Currency:    "CNY",
		}
		if err := db.Create(&order).Error; err != nil {
			t.Fatalf("create order %s failed: %v", orderNo, err)
		}
	}

	createOrder("ORD-PURCHASE-STATS-SYNC-1", models.OrderStatusPendingPayment, 1)
	createOrder("ORD-PURCHASE-STATS-SYNC-2", models.OrderStatusCancelled, 5)
	createOrder("ORD-PURCHASE-STATS-SYNC-3", models.OrderStatusRefunded, 2)

	orderRepo := repository.NewOrderRepository(db)
	if err := syncUserPurchaseStats(orderRepo, user.ID); err != nil {
		t.Fatalf("sync user purchase stats failed: %v", err)
	}

	quantity, err := orderRepo.GetUserPurchaseQuantityBySKU(user.ID, "SKU-PURCHASE-STATS-SYNC")
	if err != nil {
		t.Fatalf("query purchase quantity failed: %v", err)
	}
	if quantity != 3 {
		t.Fatalf("expected purchase quantity 3 after full rebuild, got %d", quantity)
	}
}
