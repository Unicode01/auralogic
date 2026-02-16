package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
)

type SerialHandler struct {
	serialService *service.SerialService
}

func NewSerialHandler(serialService *service.SerialService) *SerialHandler {
	return &SerialHandler{
		serialService: serialService,
	}
}

// ListSerials 列出所有序列号（管理员）
func (h *SerialHandler) ListSerials(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

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
		response.InternalError(c, err.Error())
		return
	}

	response.Paginated(c, serials, page, limit, total)
}

// GetSerialByNumber 根据序列号查询
func (h *SerialHandler) GetSerialByNumber(c *gin.Context) {
	serialNumber := c.Param("serial_number")

	serial, err := h.serialService.VerifySerial(serialNumber)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, serial)
}

// GetStatistics 获取统计信息
func (h *SerialHandler) GetStatistics(c *gin.Context) {
	stats, err := h.serialService.GetStatistics()
	if err != nil {
		response.InternalError(c, err.Error())
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
		response.InternalError(c, err.Error())
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
		response.InternalError(c, err.Error())
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

	if err := h.serialService.DeleteSerial(uint(serialID)); err != nil {
		response.InternalError(c, "Failed to delete serial number")
		return
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

	if err := h.serialService.BatchDeleteSerials(req.IDs); err != nil {
		response.InternalError(c, "Failed to delete serial numbers")
		return
	}

	response.Success(c, gin.H{
		"message": "Serial numbers deleted successfully",
		"count":   len(req.IDs),
	})
}
