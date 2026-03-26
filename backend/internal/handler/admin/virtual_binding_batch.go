package admin

import (
	"errors"

	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/service"
)

type VirtualBindingBatchError struct {
	Index              int                    `json:"index"`
	VirtualInventoryID uint                   `json:"virtual_inventory_id,omitempty"`
	ErrorKey           string                 `json:"error_key,omitempty"`
	Message            string                 `json:"message"`
	Params             map[string]interface{} `json:"params,omitempty"`
}

func buildVirtualBindingBatchError(batchErr service.VirtualVariantBindingBatchError) VirtualBindingBatchError {
	result := VirtualBindingBatchError{
		Index:              batchErr.Index,
		VirtualInventoryID: batchErr.VirtualInventoryID,
		Message:            batchErr.Err.Error(),
	}

	var bizErr *bizerr.Error
	if errors.As(batchErr.Err, &bizErr) {
		result.ErrorKey = bizErr.Key
		result.Message = bizErr.Message
		result.Params = bizErr.Params
	}

	return result
}
