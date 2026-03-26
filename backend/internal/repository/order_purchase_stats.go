package repository

import (
	"strings"

	"auralogic/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func normalizePurchaseStatSKUs(skus []string) []string {
	if len(skus) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(skus))
	seen := make(map[string]struct{}, len(skus))
	for _, sku := range skus {
		trimmed := strings.TrimSpace(sku)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}

func normalizePurchaseStatDelta(deltaBySKU map[string]int64) map[string]int64 {
	if len(deltaBySKU) == 0 {
		return nil
	}

	normalized := make(map[string]int64, len(deltaBySKU))
	for sku, delta := range deltaBySKU {
		trimmed := strings.TrimSpace(sku)
		if trimmed == "" || delta == 0 {
			continue
		}
		normalized[trimmed] += delta
		if normalized[trimmed] == 0 {
			delete(normalized, trimmed)
		}
	}
	return normalized
}

func isUserPurchaseStatsTableMissingError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no such table: user_purchase_stats") ||
		strings.Contains(message, "doesn't exist") && strings.Contains(message, "user_purchase_stats") ||
		strings.Contains(message, "relation \"user_purchase_stats\" does not exist")
}

func buildPurchaseStatTargetSet(skus []string) map[string]struct{} {
	if len(skus) == 0 {
		return nil
	}

	targets := make(map[string]struct{}, len(skus))
	for _, sku := range skus {
		targets[sku] = struct{}{}
	}
	return targets
}

func scanUserPurchaseQuantitiesFromOrders(db *gorm.DB, userID uint, targetSKUs map[string]struct{}) (map[string]int64, error) {
	quantities := make(map[string]int64, len(targetSKUs))
	if userID == 0 {
		return quantities, nil
	}
	if len(targetSKUs) > 0 {
		for sku := range targetSKUs {
			quantities[sku] = 0
		}
	}

	var orders []models.Order
	if err := db.Select("items").
		Where("user_id = ? AND status != ?", userID, models.OrderStatusCancelled).
		Find(&orders).Error; err != nil {
		return nil, err
	}

	for _, order := range orders {
		for _, item := range order.Items {
			sku := strings.TrimSpace(item.SKU)
			if sku == "" || item.Quantity <= 0 {
				continue
			}
			if len(targetSKUs) > 0 {
				if _, tracked := targetSKUs[sku]; !tracked {
					continue
				}
			}
			quantities[sku] += int64(item.Quantity)
		}
	}

	return quantities, nil
}

func (r *OrderRepository) GetUserPurchaseQuantitySummaryFromOrders(userID uint) (map[string]int64, error) {
	return scanUserPurchaseQuantitiesFromOrders(r.db, userID, nil)
}

func (r *OrderRepository) GetUserPurchaseQuantityBySKUs(userID uint, skus []string) (map[string]int, error) {
	normalizedSKUs := normalizePurchaseStatSKUs(skus)
	quantities := make(map[string]int, len(normalizedSKUs))
	for _, sku := range normalizedSKUs {
		quantities[sku] = 0
	}
	if len(normalizedSKUs) == 0 {
		return quantities, nil
	}

	var stats []models.UserPurchaseStat
	err := r.db.Select("sku", "quantity").
		Where("user_id = ? AND sku IN ?", userID, normalizedSKUs).
		Find(&stats).Error
	if err != nil {
		if !isUserPurchaseStatsTableMissingError(err) {
			return nil, err
		}
		fallback, fallbackErr := scanUserPurchaseQuantitiesFromOrders(r.db, userID, buildPurchaseStatTargetSet(normalizedSKUs))
		if fallbackErr != nil {
			return nil, fallbackErr
		}
		for sku, quantity := range fallback {
			quantities[sku] = int(quantity)
		}
		return quantities, nil
	}

	for _, stat := range stats {
		quantities[stat.SKU] = int(stat.Quantity)
	}
	return quantities, nil
}

func (r *OrderRepository) ApplyUserPurchaseStatsDelta(userID uint, deltaBySKU map[string]int64) error {
	normalized := normalizePurchaseStatDelta(deltaBySKU)
	if userID == 0 || len(normalized) == 0 {
		return nil
	}

	for sku, delta := range normalized {
		initialQuantity := delta
		if initialQuantity < 0 {
			initialQuantity = 0
		}

		stat := models.UserPurchaseStat{
			UserID:   userID,
			SKU:      sku,
			Quantity: initialQuantity,
		}
		if err := r.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "sku"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"quantity": gorm.Expr(
					"CASE WHEN quantity + ? < 0 THEN 0 ELSE quantity + ? END",
					delta,
					delta,
				),
				"updated_at": models.NowFunc(),
			}),
		}).Create(&stat).Error; err != nil {
			if isUserPurchaseStatsTableMissingError(err) {
				return nil
			}
			return err
		}

		if delta < 0 {
			if err := r.db.Where("user_id = ? AND sku = ? AND quantity <= 0", userID, sku).
				Delete(&models.UserPurchaseStat{}).Error; err != nil && !isUserPurchaseStatsTableMissingError(err) {
				return err
			}
		}
	}

	return nil
}

func (r *OrderRepository) ReplaceUserPurchaseStats(userID uint, quantities map[string]int64) error {
	if userID == 0 {
		return nil
	}

	normalized := normalizePurchaseStatDelta(quantities)
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserPurchaseStat{}).Error; err != nil {
			if isUserPurchaseStatsTableMissingError(err) {
				return nil
			}
			return err
		}

		if len(normalized) == 0 {
			return nil
		}

		rows := make([]models.UserPurchaseStat, 0, len(normalized))
		for sku, quantity := range normalized {
			if quantity <= 0 {
				continue
			}
			rows = append(rows, models.UserPurchaseStat{
				UserID:   userID,
				SKU:      sku,
				Quantity: quantity,
			})
		}
		if len(rows) == 0 {
			return nil
		}

		if err := tx.CreateInBatches(rows, 200).Error; err != nil {
			if isUserPurchaseStatsTableMissingError(err) {
				return nil
			}
			return err
		}
		return nil
	})
}
