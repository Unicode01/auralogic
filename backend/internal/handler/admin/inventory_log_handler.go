package admin

import (
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"gorm.io/gorm"
)

type InventoryLogHandler struct {
	db *gorm.DB
}

func NewInventoryLogHandler(db *gorm.DB) *InventoryLogHandler {
	return &InventoryLogHandler{db: db}
}

// ListInventoryLogs getInventory日志列表
func (h *InventoryLogHandler) ListInventoryLogs(c *gin.Context) {
	page, limit := response.GetPagination(c)
	source := c.Query("source")
	inventoryID := c.Query("inventory_id")
	logType := c.Query("type")
	orderNo := c.Query("order_no")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	var logs []models.InventoryLog
	var total int64

	query := h.db.Model(&models.InventoryLog{})

	// 来源筛选
	if source != "" {
		query = query.Where("source = ?", source)
	}

	// 过滤条件
	if inventoryID != "" {
		query = query.Where("inventory_id = ?", inventoryID)
	}
	if logType != "" {
		query = query.Where("type = ?", logType)
	}
	if orderNo != "" {
		query = query.Where("order_no = ?", orderNo)
	}
	if startDate != "" {
		if t, err := time.Parse("2006-01-02", startDate); err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if endDate != "" {
		if t, err := time.Parse("2006-01-02", endDate); err == nil {
			// 包含当天结束时间
			t = t.Add(24*time.Hour - time.Second)
			query = query.Where("created_at <= ?", t)
		}
	}

	// get总数
	if err := query.Count(&total).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	// 分页Query，按Create时间倒序
	offset := (page - 1) * limit
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&logs).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, logs, page, limit, total)
}

// GetInventoryLogStatistics getInventory日志统计
func (h *InventoryLogHandler) GetInventoryLogStatistics(c *gin.Context) {
	source := c.Query("source")

	var stats struct {
		TotalLogs   int64 `json:"total_logs"`
		InLogs      int64 `json:"in_logs"`
		OutLogs     int64 `json:"out_logs"`
		ReserveLogs int64 `json:"reserve_logs"`
		ReleaseLogs int64 `json:"release_logs"`
		AdjustLogs  int64 `json:"adjust_logs"`
		ImportLogs  int64 `json:"import_logs"`
		DeliverLogs int64 `json:"deliver_logs"`
		DeleteLogs  int64 `json:"delete_logs"`
	}

	baseQuery := h.db.Model(&models.InventoryLog{})
	if source != "" {
		baseQuery = baseQuery.Where("source = ?", source)
	}

	baseQuery.Count(&stats.TotalLogs)
	h.db.Model(&models.InventoryLog{}).Where("type = ?", models.InventoryLogTypeIn).Count(&stats.InLogs)
	h.db.Model(&models.InventoryLog{}).Where("type = ?", models.InventoryLogTypeOut).Count(&stats.OutLogs)
	h.db.Model(&models.InventoryLog{}).Where("type = ?", models.InventoryLogTypeReserve).Count(&stats.ReserveLogs)
	h.db.Model(&models.InventoryLog{}).Where("type = ?", models.InventoryLogTypeRelease).Count(&stats.ReleaseLogs)
	h.db.Model(&models.InventoryLog{}).Where("type = ?", models.InventoryLogTypeAdjust).Count(&stats.AdjustLogs)
	h.db.Model(&models.InventoryLog{}).Where("type = ?", models.InventoryLogTypeImport).Count(&stats.ImportLogs)
	h.db.Model(&models.InventoryLog{}).Where("type = ?", models.InventoryLogTypeDeliver).Count(&stats.DeliverLogs)
	h.db.Model(&models.InventoryLog{}).Where("type = ?", models.InventoryLogTypeDelete).Count(&stats.DeleteLogs)

	response.Success(c, stats)
}
