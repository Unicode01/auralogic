package admin

import (
	"errors"
	"testing"

	"auralogic/internal/pkg/bizerr"
)

func TestBuildBindingBatchErrorFromBizError(t *testing.T) {
	err := bizerr.New("binding.noMatchingInventory", "No matching inventory configuration found").
		WithParams(map[string]interface{}{"sku": "SKU-1"})

	result := buildBindingBatchError(2, CreateBindingRequest{InventoryID: 12}, err)

	if result.Index != 2 {
		t.Fatalf("expected index 2, got %d", result.Index)
	}
	if result.InventoryID != 12 {
		t.Fatalf("expected inventory id 12, got %d", result.InventoryID)
	}
	if result.ErrorKey != "binding.noMatchingInventory" {
		t.Fatalf("expected error key to be preserved, got %q", result.ErrorKey)
	}
	if result.Message != "No matching inventory configuration found" {
		t.Fatalf("expected bizerr message to be preserved, got %q", result.Message)
	}
	if got := result.Params["sku"]; got != "SKU-1" {
		t.Fatalf("expected params to be preserved, got %#v", result.Params)
	}
}

func TestBuildBindingBatchErrorFromGenericError(t *testing.T) {
	result := buildBindingBatchError(1, CreateBindingRequest{InventoryID: 3}, errors.New("create failed"))

	if result.ErrorKey != "" {
		t.Fatalf("expected empty error key for generic errors, got %q", result.ErrorKey)
	}
	if result.Message != "create failed" {
		t.Fatalf("expected original error message, got %q", result.Message)
	}
	if result.Params != nil {
		t.Fatalf("expected nil params for generic errors, got %#v", result.Params)
	}
}
