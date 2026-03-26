package service

import (
	"strings"

	"auralogic/internal/models"
)

func collectOrderItemSKUs(items []models.OrderItem) []string {
	skus := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" {
			continue
		}
		if _, exists := seen[sku]; exists {
			continue
		}
		seen[sku] = struct{}{}
		skus = append(skus, sku)
	}
	return skus
}

func collectRequestedSKUs(requestedQtyBySKU map[string]int) []string {
	skus := make([]string, 0, len(requestedQtyBySKU))
	for sku := range requestedQtyBySKU {
		skus = append(skus, sku)
	}
	return skus
}

func (s *OrderService) loadProductsForOrderItems(items []models.OrderItem) (map[string]*models.Product, error) {
	if s == nil || s.productRepo == nil {
		return map[string]*models.Product{}, nil
	}
	return s.productRepo.FindBySKUs(collectOrderItemSKUs(items))
}
