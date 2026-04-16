package admin

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"auralogic/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

const adminSpreadsheetContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

func writeXLSXAttachment(c *gin.Context, fileName string, sheetName string, headers []string, rows [][]string) {
	f := excelize.NewFile()
	defer func() {
		_ = f.Close()
	}()

	worksheetName := sanitizeAdminWorksheetName(sheetName)
	if worksheetName == "" {
		worksheetName = "Sheet1"
	}
	if worksheetName != "Sheet1" {
		f.SetSheetName("Sheet1", worksheetName)
	}

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
			Size: 11,
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#E2E8F0"},
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
			WrapText:   true,
		},
	})
	if err != nil {
		response.InternalError(c, "Export failed")
		return
	}

	textStyle, err := f.NewStyle(&excelize.Style{
		NumFmt: 49,
		Alignment: &excelize.Alignment{
			Vertical: "top",
			WrapText: true,
		},
	})
	if err != nil {
		response.InternalError(c, "Export failed")
		return
	}

	if len(headers) > 0 {
		headerRow := make([]interface{}, len(headers))
		for i, value := range headers {
			headerRow[i] = value
		}
		if err := f.SetSheetRow(worksheetName, "A1", &headerRow); err != nil {
			response.InternalError(c, "Export failed")
			return
		}

		endColumn, cellErr := excelize.ColumnNumberToName(len(headers))
		if cellErr != nil {
			response.InternalError(c, "Export failed")
			return
		}
		if err := f.SetColStyle(worksheetName, fmt.Sprintf("A:%s", endColumn), textStyle); err != nil {
			response.InternalError(c, "Export failed")
			return
		}
		if err := f.SetCellStyle(worksheetName, "A1", fmt.Sprintf("%s1", endColumn), headerStyle); err != nil {
			response.InternalError(c, "Export failed")
			return
		}
	}

	columnWidths := buildAdminSpreadsheetColumnWidths(headers, rows)
	for idx, width := range columnWidths {
		columnName, err := excelize.ColumnNumberToName(idx + 1)
		if err != nil {
			response.InternalError(c, "Export failed")
			return
		}
		if err := f.SetColWidth(worksheetName, columnName, columnName, width); err != nil {
			response.InternalError(c, "Export failed")
			return
		}
	}

	for rowIndex, row := range rows {
		rowValues := make([]interface{}, len(row))
		for idx, value := range row {
			rowValues[idx] = value
		}
		cell, err := excelize.CoordinatesToCellName(1, rowIndex+2)
		if err != nil {
			response.InternalError(c, "Export failed")
			return
		}
		if err := f.SetSheetRow(worksheetName, cell, &rowValues); err != nil {
			response.InternalError(c, "Export failed")
			return
		}
	}

	if sheetIndex, err := f.GetSheetIndex(worksheetName); err == nil && sheetIndex >= 0 {
		f.SetActiveSheet(sheetIndex)
	}

	buffer, err := f.WriteToBuffer()
	if err != nil {
		response.InternalError(c, "Export failed")
		return
	}

	safeName := sanitizeAdminAttachmentFileName(fileName, buildAdminXLSXFileName("export"))
	c.Header("Content-Type", adminSpreadsheetContentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", safeName))
	c.Header("Cache-Control", "no-store")
	c.Data(200, adminSpreadsheetContentType, buffer.Bytes())
}

func buildAdminXLSXFileName(prefix string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		trimmed = "export"
	}
	return fmt.Sprintf("%s_%s.xlsx", trimmed, time.Now().Format("20060102_150405"))
}

func sanitizeAdminWorksheetName(value string) string {
	name := strings.TrimSpace(value)
	if name == "" {
		return ""
	}
	name = strings.NewReplacer("\\", "_", "/", "_", "?", "_", "*", "_", "[", "_", "]", "_", ":", "_").Replace(name)
	name = strings.Trim(name, "'")
	if name == "" {
		return ""
	}

	runes := []rune(name)
	if len(runes) > 31 {
		name = string(runes[:31])
	}
	return name
}

func buildAdminSpreadsheetColumnWidths(headers []string, rows [][]string) []float64 {
	totalColumns := len(headers)
	for _, row := range rows {
		if len(row) > totalColumns {
			totalColumns = len(row)
		}
	}
	if totalColumns == 0 {
		return nil
	}

	widths := make([]float64, totalColumns)
	for idx := range widths {
		widths[idx] = 12
	}

	for idx, header := range headers {
		widths[idx] = maxAdminSpreadsheetWidth(widths[idx], adminSpreadsheetWidthForValue(header))
	}
	for _, row := range rows {
		for idx, value := range row {
			widths[idx] = maxAdminSpreadsheetWidth(widths[idx], adminSpreadsheetWidthForValue(value))
		}
	}

	return widths
}

func adminSpreadsheetWidthForValue(value string) float64 {
	cleaned := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " "))
	if cleaned == "" {
		return 12
	}
	runes := utf8.RuneCountInString(cleaned)
	switch {
	case runes <= 10:
		return 12
	case runes >= 56:
		return 60
	default:
		return float64(runes + 4)
	}
}

func maxAdminSpreadsheetWidth(current float64, candidate float64) float64 {
	if candidate > current {
		return candidate
	}
	return current
}
