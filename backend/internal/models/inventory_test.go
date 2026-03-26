package models

import "testing"

func TestInventoryCanPurchaseIncludesAvailableQuantity(t *testing.T) {
	inventory := &Inventory{
		Stock:             5,
		AvailableQuantity: 2,
		IsActive:          true,
	}

	canPurchase, message := inventory.CanPurchase(3)
	if canPurchase {
		t.Fatal("expected purchase to be blocked when quantity exceeds available stock")
	}

	const expected = "Insufficient stock, available quantity: 2"
	if message != expected {
		t.Fatalf("expected message %q, got %q", expected, message)
	}
}
