package admin

import (
	"errors"
	"testing"

	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/service"
)

func TestBuildVirtualBindingBatchErrorFromBizError(t *testing.T) {
	result := buildVirtualBindingBatchError(service.VirtualVariantBindingBatchError{
		Index:              3,
		VirtualInventoryID: 9,
		Err: bizerr.New("virtual_binding.inventoryNotFound", "Virtual inventory does not exist").
			WithParams(map[string]interface{}{"inventory_id": 9}),
	})

	if result.Index != 3 {
		t.Fatalf("expected index 3, got %d", result.Index)
	}
	if result.VirtualInventoryID != 9 {
		t.Fatalf("expected virtual inventory id 9, got %d", result.VirtualInventoryID)
	}
	if result.ErrorKey != "virtual_binding.inventoryNotFound" {
		t.Fatalf("expected virtual binding error key, got %q", result.ErrorKey)
	}
	if result.Message != "Virtual inventory does not exist" {
		t.Fatalf("expected bizerr message, got %q", result.Message)
	}
	if got := result.Params["inventory_id"]; got != 9 {
		t.Fatalf("expected params to be preserved, got %#v", result.Params)
	}
}

func TestBuildVirtualBindingBatchErrorFromGenericError(t *testing.T) {
	result := buildVirtualBindingBatchError(service.VirtualVariantBindingBatchError{
		Index:              1,
		VirtualInventoryID: 2,
		Err:                errors.New("create failed"),
	})

	if result.ErrorKey != "" {
		t.Fatalf("expected empty error key for generic error, got %q", result.ErrorKey)
	}
	if result.Message != "create failed" {
		t.Fatalf("expected generic message to be preserved, got %q", result.Message)
	}
	if result.Params != nil {
		t.Fatalf("expected nil params for generic error, got %#v", result.Params)
	}
}
