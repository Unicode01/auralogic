package admin

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

type SerialHandler struct {
	serialService *service.SerialService
	pluginManager *service.PluginManagerService
}

func NewSerialHandler(serialService *service.SerialService, pluginManager *service.PluginManagerService) *SerialHandler {
	return &SerialHandler{
		serialService: serialService,
		pluginManager: pluginManager,
	}
}

func buildSerialAdminHookPayload(serial *models.ProductSerial) map[string]interface{} {
	if serial == nil {
		return map[string]interface{}{}
	}

	payload := map[string]interface{}{
		"serial_id":             serial.ID,
		"serial_number":         serial.SerialNumber,
		"product_id":            serial.ProductID,
		"order_id":              serial.OrderID,
		"product_code":          serial.ProductCode,
		"sequence_number":       serial.SequenceNumber,
		"anti_counterfeit_code": serial.AntiCounterfeitCode,
		"view_count":            serial.ViewCount,
		"first_viewed_at":       serial.FirstViewedAt,
		"last_viewed_at":        serial.LastViewedAt,
		"created_at":            serial.CreatedAt,
		"updated_at":            serial.UpdatedAt,
	}
	if serial.Product != nil {
		payload["product_name"] = serial.Product.Name
		payload["product_sku"] = serial.Product.SKU
	}
	if serial.Order != nil {
		payload["order_no"] = serial.Order.OrderNo
		payload["order_status"] = serial.Order.Status
		payload["user_id"] = serial.Order.UserID
	}
	return payload
}

func buildSerialAdminHookPayloadList(serials []models.ProductSerial) []map[string]interface{} {
	if len(serials) == 0 {
		return []map[string]interface{}{}
	}

	items := make([]map[string]interface{}, 0, len(serials))
	for i := range serials {
		items = append(items, buildSerialAdminHookPayload(&serials[i]))
	}
	return items
}

func serialHookDecodeIDs(value interface{}) ([]uint, error) {
	if value == nil {
		return nil, nil
	}

	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var ids []uint
	if err := json.Unmarshal(body, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// ListSerials 列出所有序列号（管理员）
func (h *SerialHandler) ListSerials(c *gin.Context) {
	page, limit := response.GetPagination(c)

	filters := make(map[string]interface{})

	if productID := c.Query("product_id"); productID != "" {
		if id, err := strconv.Atoi(productID); err == nil {
			filters["product_id"] = uint(id)
		}
	}

	if orderID := c.Query("order_id"); orderID != "" {
		if id, err := strconv.Atoi(orderID); err == nil {
			filters["order_id"] = uint(id)
		}
	}

	if productCode := c.Query("product_code"); productCode != "" {
		filters["product_code"] = productCode
	}

	if serialNumber := c.Query("serial_number"); serialNumber != "" {
		filters["serial_number"] = serialNumber
	}

	serials, total, err := h.serialService.ListSerials(page, limit, filters)
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, serials, page, limit, total)
}

// GetSerialByNumber 根据序列号查询
func (h *SerialHandler) GetSerialByNumber(c *gin.Context) {
	serialNumber := c.Param("serial_number")
	adminID := getOptionalUserID(c)

	serial, err := h.serialService.VerifySerialWithContext(serialNumber, buildAdminHookExecutionContext(c, adminID, map[string]string{
		"hook_resource": "serial",
		"hook_source":   "admin_api",
	}), "admin_api")
	if err != nil {
		if service.IsHookBlockedError(err) {
			response.BadRequest(c, err.Error())
			return
		}
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, serial)
}

// GetStatistics 获取统计信息
func (h *SerialHandler) GetStatistics(c *gin.Context) {
	stats, err := h.serialService.GetStatistics()
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, stats)
}

// GetSerialsByOrder 获取订单的所有序列号
func (h *SerialHandler) GetSerialsByOrder(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("order_id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid order ID")
		return
	}

	serials, err := h.serialService.GetSerialsByOrderID(uint(orderID))
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, serials)
}

// GetSerialsByProduct 获取商品的所有序列号
func (h *SerialHandler) GetSerialsByProduct(c *gin.Context) {
	productID, err := strconv.ParseUint(c.Param("product_id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid product ID")
		return
	}

	serials, err := h.serialService.GetSerialsByProductID(uint(productID))
	if err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Success(c, serials)
}

// DeleteSerial Delete a serial number
func (h *SerialHandler) DeleteSerial(c *gin.Context) {
	serialID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid serial ID")
		return
	}
	serial, err := h.serialService.GetSerialByID(uint(serialID))
	if err != nil {
		response.NotFound(c, "Serial number not found")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		hookPayload := buildSerialAdminHookPayload(serial)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "serial.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "serial",
			"hook_source":   "admin_api",
			"serial_id":     strconv.FormatUint(serialID, 10),
		}))
		if hookErr != nil {
			log.Printf("serial.delete.before hook execution failed: admin=%d serial=%d err=%v", adminIDValue, uint(serialID), hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Serial deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.serialService.DeleteSerial(uint(serialID)); err != nil {
		response.InternalError(c, "Failed to delete serial number")
		return
	}

	if h.pluginManager != nil {
		afterPayload := buildSerialAdminHookPayload(serial)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, deletedSerialID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "serial.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("serial.delete.after hook execution failed: admin=%d serial=%d err=%v", adminIDValue, deletedSerialID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "serial",
			"hook_source":   "admin_api",
			"serial_id":     strconv.FormatUint(serialID, 10),
		})), afterPayload, serial.ID)
	}

	response.Success(c, gin.H{"message": "Serial number deleted successfully"})
}

// BatchDeleteSerials Delete multiple serial numbers
func (h *SerialHandler) BatchDeleteSerials(c *gin.Context) {
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	if len(req.IDs) == 0 {
		response.BadRequest(c, "No serial IDs provided")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	existingSerials, err := h.serialService.GetSerialsByIDs(req.IDs)
	if err != nil {
		response.InternalError(c, "Failed to query serial numbers")
		return
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload := map[string]interface{}{
			"ids":      req.IDs,
			"serials":  buildSerialAdminHookPayloadList(existingSerials),
			"count":    len(req.IDs),
			"admin_id": adminIDValue,
			"source":   "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "serial.batch_delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "serial",
			"hook_source":   "admin_api",
		}))
		if hookErr != nil {
			log.Printf("serial.batch_delete.before hook execution failed: admin=%d count=%d err=%v", adminIDValue, len(req.IDs), hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Serial batch deletion rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawIDs, exists := hookResult.Payload["ids"]; exists {
					updatedIDs, decodeErr := serialHookDecodeIDs(rawIDs)
					if decodeErr != nil {
						log.Printf("serial.batch_delete.before ids patch ignored: admin=%d err=%v", adminIDValue, decodeErr)
						req = originalReq
					} else {
						req.IDs = updatedIDs
					}
				}
			}
		}
	}
	if len(req.IDs) == 0 {
		response.BadRequest(c, "No serial IDs provided")
		return
	}
	existingSerials, err = h.serialService.GetSerialsByIDs(req.IDs)
	if err != nil {
		response.InternalError(c, "Failed to query serial numbers")
		return
	}

	if err := h.serialService.BatchDeleteSerials(req.IDs); err != nil {
		response.InternalError(c, "Failed to delete serial numbers")
		return
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"ids":      req.IDs,
			"serials":  buildSerialAdminHookPayloadList(existingSerials),
			"count":    len(req.IDs),
			"admin_id": adminIDValue,
			"source":   "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, count int) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "serial.batch_delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("serial.batch_delete.after hook execution failed: admin=%d count=%d err=%v", adminIDValue, count, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "serial",
			"hook_source":   "admin_api",
		})), afterPayload, len(req.IDs))
	}

	response.Success(c, gin.H{
		"message": "Serial numbers deleted successfully",
		"count":   len(req.IDs),
	})
}
