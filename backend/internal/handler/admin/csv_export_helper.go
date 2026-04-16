package admin

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"auralogic/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const (
	adminCSVTimeFormat    = "2006-01-02 15:04:05"
	adminCSVExportMaxRows = 20000
)

func writeCSVAttachment(c *gin.Context, fileName string, headers []string, rows [][]string) {
	var buffer bytes.Buffer
	buffer.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(&buffer)
	if err := writer.Write(headers); err != nil {
		response.InternalError(c, "Export failed")
		return
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			response.InternalError(c, "Export failed")
			return
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		response.InternalError(c, "Export failed")
		return
	}

	safeName := sanitizeAdminAttachmentFileName(fileName, buildAdminCSVFileName("export"))

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", safeName))
	c.Header("Cache-Control", "no-store")
	c.Data(200, "text/csv; charset=utf-8", buffer.Bytes())
}

func writeJSONAttachment(c *gin.Context, fileName string, payload interface{}) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		response.InternalError(c, "Export failed")
		return
	}

	safeName := sanitizeAdminAttachmentFileName(fileName, buildAdminJSONFileName("export"))

	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", safeName))
	c.Header("Cache-Control", "no-store")
	c.Data(200, "application/json; charset=utf-8", buffer.Bytes())
}

func buildAdminCSVFileName(prefix string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		trimmed = "export"
	}
	return fmt.Sprintf("%s_%s.csv", trimmed, time.Now().Format("20060102_150405"))
}

func buildAdminJSONFileName(prefix string) string {
	trimmed := strings.TrimSpace(prefix)
	if trimmed == "" {
		trimmed = "export"
	}
	return fmt.Sprintf("%s_%s.json", trimmed, time.Now().Format("20060102_150405"))
}

func sanitizeAdminAttachmentFileName(fileName string, fallback string) string {
	safeName := strings.NewReplacer("\r", "_", "\n", "_", "\"", "_").Replace(strings.TrimSpace(fileName))
	if safeName == "" {
		return strings.TrimSpace(fallback)
	}
	return safeName
}

func csvTimeValue(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(adminCSVTimeFormat)
}

func csvTimePtrValue(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.Format(adminCSVTimeFormat)
}

func csvJSONValue(value interface{}) string {
	if value == nil {
		return ""
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}
