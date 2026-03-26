package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/constants"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

// ExportOrdersRequest 导出Order请求
type ExportOrdersRequest struct {
	Status        string `form:"status" json:"status"`                 // Order状态过滤
	Search        string `form:"search" json:"search"`                 // 搜索关键词
	Country       string `form:"country" json:"country"`               // 国家过滤
	ProductSearch string `form:"product_search" json:"product_search"` // ProductSKU/名称搜索
	PromoCode     string `form:"promo_code" json:"promo_code"`         // Promo code
	PromoCodeID   string `form:"promo_code_id" json:"promo_code_id"`   // Promo code id
}

type orderImportEntry struct {
	RowIndex   int      `json:"row_index"`
	OrderNo    string   `json:"order_no"`
	TrackingNo string   `json:"tracking_no"`
	Columns    []string `json:"columns,omitempty"`
}

func decodeOrderImportEntries(value interface{}) ([]orderImportEntry, error) {
	if value == nil {
		return nil, nil
	}

	body, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}

	var entries []orderImportEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func buildOrderImportEntries(rows [][]string) []orderImportEntry {
	entries := make([]orderImportEntry, 0, len(rows))
	for i, row := range rows {
		entry := orderImportEntry{
			RowIndex: i + 2,
		}
		if len(row) > 0 {
			entry.OrderNo = row[0]
		}
		if len(row) > 12 {
			entry.TrackingNo = row[12]
		}
		if len(row) > 0 {
			entry.Columns = append([]string(nil), row...)
		}
		entries = append(entries, entry)
	}
	return entries
}

// ExportOrders 导出Order到Excel
func (h *OrderHandler) ExportOrders(c *gin.Context) {
	var req ExportOrdersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("order.export.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.export.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "order",
				"hook_source":   "admin_api",
				"hook_action":   "export",
			}))
			if hookErr != nil {
				log.Printf("order.export.before hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Order export rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("order.export.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}

	// get所有匹配条件的Order（不分页）
	promoCode := strings.ToUpper(strings.TrimSpace(req.PromoCode))
	var promoCodeID *uint
	if req.PromoCodeID != "" {
		if pid, err := strconv.ParseUint(req.PromoCodeID, 10, 32); err == nil {
			pidUint := uint(pid)
			promoCodeID = &pidUint
		}
	}
	orders, _, err := h.orderService.ListOrders(1, 10000, req.Status, req.Search, req.Country, req.ProductSearch, promoCodeID, promoCode, nil)
	if err != nil {
		response.InternalError(c, "QueryOrderFailed")
		return
	}

	// Check if admin has privacy view permission
	hasPrivacyPerm := h.hasPrivacyPermission(c)
	for i := range orders {
		h.orderService.MaskOrderIfNeeded(&orders[i], hasPrivacyPerm)
	}

	// CreateExcel文件
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	// Create工作表
	sheetName := "Order List"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		response.InternalError(c, "CreateExcelFailed")
		return
	}

	// Delete默认的Sheet1
	f.DeleteSheet("Sheet1")

	// 设置表头样式
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 12,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#4472C4"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		response.InternalError(c, "Failed to set style")
		return
	}

	// 设置表头
	headers := []string{
		"Order No.", "User Email", "Order Status", "Privacy Protected",
		"Recipient", "Phone", "Country", "Province", "City", "District", "Address", "Postcode",
		"Tracking No.",
		"Created At", "Completed At",
	}

	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// 设置列宽
	colWidths := map[string]float64{
		"A": 18, // Order号
		"B": 25, // UserEmail
		"C": 10, // Order状态
		"D": 10, // 隐私保护
		"E": 12, // 收货人
		"F": 15, // Phone
		"G": 12, // 国家
		"H": 10, // 省
		"I": 10, // 市
		"J": 10, // 区
		"K": 30, // 详细Address
		"L": 10, // 邮编
		"M": 20, // 物流单号
		"N": 20, // Create时间
		"O": 20, // 完成时间
	}

	for col, width := range colWidths {
		f.SetColWidth(sheetName, col, col, width)
	}

	// 填充数据
	for i, order := range orders {
		row := i + 2

		// Order status translation
		statusMap := map[string]string{
			"draft":         "Draft",
			"pending":       "Pending Shipment",
			"need_resubmit": "Needs Resubmit",
			"shipped":       "Shipped",
			"completed":     "Completed",
			"cancelled":     "Cancelled",
			"refunded":      "Refunded",
		}
		statusText := statusMap[string(order.Status)]
		if statusText == "" {
			statusText = string(order.Status)
		}

		// Privacy protected
		privacyText := "No"
		if order.PrivacyProtected {
			privacyText = "Yes"
		}

		// 完成时间
		completedAt := ""
		if order.CompletedAt != nil {
			completedAt = order.CompletedAt.Format("2006-01-02 15:04:05")
		}

		// 国家代码转换为中文名称
		country := order.ReceiverCountry
		if country == "" {
			country = "CN"
		}
		// 转换国家代码为中文名称
		countryName := constants.GetCountryNameZH(country)

		// Phone（包含区号）
		fullPhone := order.ReceiverPhone
		if order.PhoneCode != "" {
			fullPhone = order.PhoneCode + " " + order.ReceiverPhone
		}

		// 填充每一行
		values := []interface{}{
			order.OrderNo,
			order.UserEmail,
			statusText,
			privacyText,
			order.ReceiverName,
			fullPhone,
			countryName,
			order.ReceiverProvince,
			order.ReceiverCity,
			order.ReceiverDistrict,
			order.ReceiverAddress,
			order.ReceiverPostcode,
			order.TrackingNo,
			order.CreatedAt.Format("2006-01-02 15:04:05"),
			completedAt,
		}

		for j, value := range values {
			cell := string(rune('A'+j)) + strconv.Itoa(row)
			f.SetCellValue(sheetName, cell, value)
		}
	}

	// 设置默认工作表
	f.SetActiveSheet(index)

	// generate文件名
	fileName := fmt.Sprintf("Order List_%s.xlsx", time.Now().Format("20060102_150405"))

	// 设置响应头
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Header("Content-Transfer-Encoding", "binary")

	// 写入响应
	if err := f.Write(c.Writer); err != nil {
		response.InternalError(c, "Export failed")
		return
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"status":                 req.Status,
			"search":                 req.Search,
			"country":                req.Country,
			"product_search":         req.ProductSearch,
			"promo_code":             req.PromoCode,
			"promo_code_id":          req.PromoCodeID,
			"exported_count":         len(orders),
			"matched_total":          len(orders),
			"file_name":              fileName,
			"has_privacy_permission": hasPrivacyPerm,
			"admin_id":               adminIDValue,
			"source":                 "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, exportedCount int) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.export.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("order.export.after hook execution failed: admin=%d exported=%d err=%v", adminIDValue, exportedCount, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "order",
			"hook_source":   "admin_api",
			"hook_action":   "export",
		})), afterPayload, len(orders))
	}
}

// ImportOrders 导入Order（批量分配物流Info）
func (h *OrderHandler) ImportOrders(c *gin.Context) {
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	// get上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "Please select a file to upload")
		return
	}

	// 检查文件扩展名（更可靠）
	if !strings.HasSuffix(strings.ToLower(file.Filename), ".xlsx") {
		response.BadRequest(c, "Only .xlsx Excel format is supported")
		return
	}

	// 打开文件
	src, err := file.Open()
	if err != nil {
		response.InternalError(c, "Failed to open file")
		return
	}
	defer src.Close()

	// 读取Excel
	f, err := excelize.OpenReader(src)
	if err != nil {
		response.BadRequest(c, "Failed to parse Excel file")
		return
	}
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	// get所有工作表名称
	sheetList := f.GetSheetList()

	if len(sheetList) == 0 {
		response.BadRequest(c, "No worksheet found in Excel file")
		return
	}

	// get第一个工作表的所有行
	sheetName := sheetList[0]

	rows, err := f.GetRows(sheetName)
	if err != nil {
		response.InternalError(c, fmt.Sprintf("Failed to read Excel data: %v", err))
		return
	}

	if len(rows) < 2 {
		response.BadRequest(c, fmt.Sprintf("No data rows in Excel file (total rows: %d）", len(rows)))
		return
	}

	entries := buildOrderImportEntries(rows[1:])
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"filename":    file.Filename,
			"sheet_name":  sheetName,
			"entry_count": len(entries),
			"entries":     entries,
			"admin_id":    adminIDValue,
			"source":      "admin_api",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "order.import.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "order",
			"hook_source":   "admin_api",
			"hook_action":   "import",
		}))
		if hookErr != nil {
			log.Printf("order.import.before hook execution failed: admin=%d filename=%s err=%v", adminIDValue, file.Filename, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Order import rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if rawEntries, exists := hookResult.Payload["entries"]; exists {
					updatedEntries, decodeErr := decodeOrderImportEntries(rawEntries)
					if decodeErr != nil {
						log.Printf("order.import.before entries patch ignored: admin=%d filename=%s err=%v", adminIDValue, file.Filename, decodeErr)
					} else {
						entries = updatedEntries
					}
				}
			}
		}
	}

	// 解析数据并UpdateOrder
	var successCount, skipCount, errorCount int
	var errors []string

	for _, entry := range entries {
		if len(entry.Columns) > 0 && len(entry.Columns) < 13 && strings.TrimSpace(entry.TrackingNo) == "" {
			errorCount++
			errors = append(errors, fmt.Sprintf("Row %d: Insufficient columns (actual columns:%d, need at least 13 columns)", entry.RowIndex, len(entry.Columns)))
			continue
		}

		orderNo := strings.TrimSpace(entry.OrderNo)
		trackingNo := strings.TrimSpace(entry.TrackingNo)

		// 检查Order号是否为空
		if orderNo == "" {
			skipCount++
			continue
		}

		// 检查物流Info是否为空
		if trackingNo == "" {
			skipCount++
			continue
		}

		// QueryOrder
		order, err := h.orderService.GetOrderByNo(orderNo)
		if err != nil {
			errorCount++
			errors = append(errors, fmt.Sprintf("Row %d: Order %s does not exist", entry.RowIndex, orderNo))
			continue
		}

		// 检查Order状态：只有待发货和need重填状态的Order才能分配物流单号
		if order.Status != models.OrderStatusPending && order.Status != models.OrderStatusNeedResubmit {
			skipCount++
			statusText := map[string]string{
				"draft":         "Draft",
				"pending":       "Pending Shipment",
				"need_resubmit": "Needs Resubmit",
				"shipped":       "Shipped",
				"completed":     "Completed",
				"cancelled":     "Cancelled",
				"refunded":      "Refunded",
			}[string(order.Status)]
			if statusText == "" {
				statusText = string(order.Status)
			}
			errors = append(errors, fmt.Sprintf("Row %d: Order %s status is [%s], tracking number assignment not allowed", entry.RowIndex, orderNo, statusText))
			continue
		}

		// 检查Order是否已有物流Info
		if order.TrackingNo != "" {
			skipCount++
			errors = append(errors, fmt.Sprintf("Row %d: Order %s already has tracking number [%s]", entry.RowIndex, orderNo, order.TrackingNo))
			continue
		}

		// Update物流Info
		if err := h.orderService.AssignTracking(order.ID, trackingNo); err != nil {
			errorCount++
			errors = append(errors, fmt.Sprintf("Row %d: Failed to update order %s: %v", entry.RowIndex, orderNo, err))
			continue
		}

		successCount++
	}

	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"filename":      file.Filename,
			"sheet_name":    sheetName,
			"entry_count":   len(entries),
			"success_count": successCount,
			"skip_count":    skipCount,
			"error_count":   errorCount,
			"errors":        errors,
			"admin_id":      adminIDValue,
			"source":        "admin_api",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, successCount int, errorCount int) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "order.import.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("order.import.after hook execution failed: admin=%d success=%d error=%d err=%v", adminIDValue, successCount, errorCount, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "order",
			"hook_source":   "admin_api",
			"hook_action":   "import",
		})), afterPayload, successCount, errorCount)
	}

	// 返回导入结果
	response.Success(c, gin.H{
		"success_count": successCount,
		"skip_count":    skipCount,
		"error_count":   errorCount,
		"errors":        errors,
		"message":       fmt.Sprintf("Successfully imported %d, skipped %d, failed %d", successCount, skipCount, errorCount),
	})
}

// DownloadTemplate 下载导入模板
func (h *OrderHandler) DownloadTemplate(c *gin.Context) {
	// CreateExcel文件
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	// Create工作表
	sheetName := "Shipping Info Import Template"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		response.InternalError(c, "CreateExcelFailed")
		return
	}

	// Delete默认的Sheet1
	f.DeleteSheet("Sheet1")

	// 设置表头样式
	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 12,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#4472C4"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		response.InternalError(c, "Failed to set style")
		return
	}

	// 设置表头
	headers := []string{
		"Order No.*", "User Email", "Order Status", "Privacy Protected",
		"Recipient", "Phone", "Country", "Province", "City", "District", "Address", "Postcode",
		"Tracking No.*",
		"Created At", "Completed At",
	}

	for i, header := range headers {
		cell := string(rune('A'+i)) + "1"
		f.SetCellValue(sheetName, cell, header)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	// 设置列宽
	colWidths := map[string]float64{
		"A": 18, "B": 25, "C": 10, "D": 10, "E": 12, "F": 15,
		"G": 12, "H": 10, "I": 10, "J": 10, "K": 30, "L": 10,
		"M": 20, "N": 20, "O": 20,
	}

	for col, width := range colWidths {
		f.SetColWidth(sheetName, col, col, width)
	}

	// 添加示例数据
	exampleData := []interface{}{
		"ORD202601050001",
		"user@example.com",
		"Pending Shipment",
		"No",
		"John Doe",
		"13800138000",
		"China",
		"Guangdong",
		"Shenzhen",
		"Nanshan",
		"Science Park South",
		"518000",
		"SF1234567890",
		"2026-01-05 10:00:00",
		"",
	}

	for j, value := range exampleData {
		cell := string(rune('A'+j)) + "2"
		f.SetCellValue(sheetName, cell, value)
	}

	// 添加说明
	f.SetCellValue(sheetName, "A4", "Notes:")
	f.SetCellValue(sheetName, "A5", "1. Columns marked with * are required")
	f.SetCellValue(sheetName, "A6", "2. Only orders with empty tracking info will be updated")
	f.SetCellValue(sheetName, "A7", "3. You can export orders first, fill in tracking info, then import")

	// 设置默认工作表
	f.SetActiveSheet(index)

	// generate文件名
	fileName := "Order_Shipping_Import_Template.xlsx"

	// 设置响应头
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Header("Content-Transfer-Encoding", "binary")

	// 写入响应
	if err := f.Write(c.Writer); err != nil {
		response.InternalError(c, "Export failed")
		return
	}
}
