package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/dbutil"
	"auralogic/internal/repository"
	"gorm.io/gorm"
)

type paidOrderFinalizeOptions struct {
	AdminRemark             string
	SkipAutoDelivery        bool
	StrictAutoDeliveryCheck bool
}

type paidOrderFinalizeResult struct {
	Updated              bool
	IsVirtualOnly        bool
	FinalStatus          models.OrderStatus
	ShippedAt            *time.Time
	AutoDeliveryCheckErr error
	VirtualDeliveryErr   error
}

func (s *OrderService) ensurePendingPaymentLimitTx(tx *gorm.DB, userID uint) error {
	limit := s.cfg.Order.MaxPendingPaymentOrdersPerUser
	if limit <= 0 {
		return nil
	}

	if err := dbutil.LockForUpdate(tx, &models.User{}, "id = ?", userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newOrderUserNotFoundError()
		}
		return fmt.Errorf("failed to lock user row: %w", err)
	}

	count, err := s.OrderRepo.CountByUserAndStatusTx(tx, userID, models.OrderStatusPendingPayment)
	if err != nil {
		return fmt.Errorf("failed to count pending payment orders: %w", err)
	}
	if count < int64(limit) {
		return nil
	}

	return bizerr.Newf(
		"order.pendingPaymentLimitExceeded",
		"You already have %d unpaid orders. The maximum is %d. Please complete or cancel existing unpaid orders first.",
		count, limit,
	).WithParams(map[string]interface{}{
		"current": count,
		"max":     limit,
	})
}

func (s *OrderService) ensurePurchaseLimitsTx(tx *gorm.DB, userID uint, requestedQtyBySKU map[string]int) error {
	if len(requestedQtyBySKU) == 0 {
		return nil
	}

	if err := dbutil.LockForUpdate(tx, &models.User{}, "id = ?", userID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newOrderUserNotFoundError()
		}
		return fmt.Errorf("failed to lock user row: %w", err)
	}

	skus := make([]string, 0, len(requestedQtyBySKU))
	for sku := range requestedQtyBySKU {
		skus = append(skus, sku)
	}

	purchasedQtyBySKU, err := repository.NewOrderRepository(tx).GetUserPurchaseQuantityBySKUs(userID, skus)
	if err != nil {
		return fmt.Errorf("failed to query purchase stats: %w", err)
	}

	var products []models.Product
	if err := tx.Select("sku", "name", "max_purchase_limit").Where("sku IN ?", skus).Find(&products).Error; err != nil {
		return fmt.Errorf("failed to query products for purchase limit validation: %w", err)
	}

	productBySKU := make(map[string]models.Product, len(products))
	for _, product := range products {
		productBySKU[product.SKU] = product
	}

	for sku, requestedQty := range requestedQtyBySKU {
		product, exists := productBySKU[sku]
		if !exists || product.MaxPurchaseLimit <= 0 {
			continue
		}

		purchasedQty := purchasedQtyBySKU[sku]
		if purchasedQty+requestedQty <= product.MaxPurchaseLimit {
			continue
		}

		remaining := product.MaxPurchaseLimit - purchasedQty
		if remaining <= 0 {
			return bizerr.Newf("order.purchaseLimitReached",
				"Product %s has reached purchase limit (maximum %d per account)", product.Name, product.MaxPurchaseLimit).
				WithParams(map[string]interface{}{"product": product.Name, "limit": product.MaxPurchaseLimit})
		}
		return bizerr.Newf("order.purchaseLimitExceeded",
			"Product %s purchase quantity exceeds limit, you can still purchase %d (maximum %d per account)",
			product.Name, remaining, product.MaxPurchaseLimit).
			WithParams(map[string]interface{}{"product": product.Name, "remaining": remaining, "limit": product.MaxPurchaseLimit})
	}

	return nil
}

func finalizePendingPaymentOrderTx(tx *gorm.DB, order *models.Order, virtualSvc *VirtualInventoryService, options paidOrderFinalizeOptions) (*paidOrderFinalizeResult, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is required")
	}
	if order == nil {
		return nil, fmt.Errorf("order is required")
	}

	result := &paidOrderFinalizeResult{
		FinalStatus: order.Status,
	}
	if order.Status != models.OrderStatusPendingPayment {
		return result, nil
	}

	isVirtualOnly := true
	hasVirtualItems := false
	for _, item := range order.Items {
		if item.ProductType == models.ProductTypeVirtual {
			hasVirtualItems = true
			continue
		}
		if item.ProductType != models.ProductTypeVirtual {
			isVirtualOnly = false
		}
	}
	result.IsVirtualOnly = isVirtualOnly

	txUpdates := map[string]interface{}{}
	if isVirtualOnly {
		txUpdates["status"] = models.OrderStatusPending
	} else {
		hasShippingInfo := strings.TrimSpace(order.ReceiverName) != "" && strings.TrimSpace(order.ReceiverAddress) != ""
		if hasShippingInfo {
			txUpdates["status"] = models.OrderStatusPending
		} else {
			txUpdates["status"] = models.OrderStatusDraft
		}
	}

	shouldAttemptAutoDelivery := false
	if virtualSvc != nil && hasVirtualItems {
		canAuto, err := virtualSvc.CanAutoDeliver(order.OrderNo)
		if err != nil {
			result.AutoDeliveryCheckErr = err
			if options.StrictAutoDeliveryCheck {
				return nil, fmt.Errorf("failed to check auto delivery: %w", err)
			}
			canAuto = false
		}
		if options.SkipAutoDelivery {
			canAuto = false
		}
		if canAuto {
			shouldAttemptAutoDelivery = true
		}
	}

	if shouldAttemptAutoDelivery && virtualSvc != nil {
		if err := tx.Transaction(func(deliveryTx *gorm.DB) error {
			return virtualSvc.DeliverAutoDeliveryStockWithTx(deliveryTx, order.ID, order.OrderNo, nil)
		}); err != nil {
			result.VirtualDeliveryErr = err
			if isVirtualOnly {
				txUpdates["status"] = models.OrderStatusPending
				delete(txUpdates, "shipped_at")
			}
		} else if isVirtualOnly {
			now := models.NowFunc()
			txUpdates["status"] = models.OrderStatusShipped
			txUpdates["shipped_at"] = now
		}
	}

	if trimmed := strings.TrimSpace(options.AdminRemark); trimmed != "" {
		if strings.TrimSpace(order.AdminRemark) != "" {
			order.AdminRemark += "\n"
		}
		order.AdminRemark += trimmed
		txUpdates["admin_remark"] = order.AdminRemark
	}

	if err := tx.Model(order).Updates(txUpdates).Error; err != nil {
		return nil, err
	}

	result.Updated = true
	if status, ok := txUpdates["status"].(models.OrderStatus); ok {
		result.FinalStatus = status
		order.Status = status
	}
	if shippedAt, ok := txUpdates["shipped_at"].(time.Time); ok {
		tmp := shippedAt
		result.ShippedAt = &tmp
		order.ShippedAt = &tmp
	} else {
		result.ShippedAt = nil
		order.ShippedAt = nil
	}

	return result, nil
}
