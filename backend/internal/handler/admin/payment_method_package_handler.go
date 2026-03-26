package admin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *PaymentMethodHandler) PreviewPackage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}

	tempPath, checksum, err := saveUploadedPaymentPackageTemp(fileHeader)
	if err != nil {
		if errorsStatus := paymentPackageUploadStatus(err); errorsStatus == http.StatusRequestEntityTooLarge {
			response.Error(c, http.StatusRequestEntityTooLarge, response.CodeParamError, err.Error())
			return
		}
		response.InternalServerError(c, "Failed to read payment package", err)
		return
	}
	defer func() {
		_ = os.Remove(tempPath)
	}()

	params := map[string]interface{}{}
	if value := strings.TrimSpace(c.PostForm("payment_method_id")); value != "" {
		params["payment_method_id"] = value
	}
	if value := strings.TrimSpace(c.PostForm("entry")); value != "" {
		params["entry"] = value
	}

	result, err := service.PreviewPaymentMethodPackageArchive(
		h.paymentMethodMarketRuntime(),
		tempPath,
		checksum,
		params,
	)
	if err != nil {
		var validationErr *bizerr.Error
		if errors.As(err, &validationErr) {
			respondPaymentPackageValidationError(c, err)
			return
		}
		h.respondPaymentMethodMarketError(c, err)
		return
	}

	response.Success(c, result)
}

func (h *PaymentMethodHandler) UploadPackage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}
	adminID, _ := middleware.GetUserID(c)

	tempPath, checksum, err := saveUploadedPaymentPackageTemp(fileHeader)
	if err != nil {
		if errorsStatus := paymentPackageUploadStatus(err); errorsStatus == http.StatusRequestEntityTooLarge {
			response.Error(c, http.StatusRequestEntityTooLarge, response.CodeParamError, err.Error())
			return
		}
		response.InternalServerError(c, "Failed to read payment package", err)
		return
	}
	defer func() {
		_ = os.Remove(tempPath)
	}()

	params := map[string]interface{}{}
	if value := strings.TrimSpace(c.PostForm("payment_method_id")); value != "" {
		params["payment_method_id"] = value
	}
	if value := strings.TrimSpace(c.PostForm("name")); value != "" {
		params["name"] = value
	}
	if value := strings.TrimSpace(c.PostForm("description")); value != "" {
		params["description"] = value
	}
	if value := strings.TrimSpace(c.PostForm("icon")); value != "" {
		params["icon"] = value
	}
	if value := strings.TrimSpace(c.PostForm("entry")); value != "" {
		params["entry"] = value
	}
	if value := strings.TrimSpace(c.PostForm("config")); value != "" {
		params["config"] = value
	}
	if value := strings.TrimSpace(c.PostForm("version")); value != "" {
		params["package_version"] = value
	}
	if value := strings.TrimSpace(c.PostForm("poll_interval")); value != "" {
		params["poll_interval"] = value
	}
	if h.pluginManager != nil {
		hookExecCtx := buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_package",
		})
		hookPayload := map[string]interface{}{"source": "admin_api"}
		for key, value := range params {
			hookPayload[key] = value
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "payment.package.import.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("payment.package.import.before hook execution failed: file=%s err=%v", fileHeader.Filename, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Payment package import rejected by plugin"
				}
				response.BadRequest(c, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["payment_method_id"]; exists {
					params["payment_method_id"] = value
				}
				if value, exists := hookResult.Payload["name"]; exists {
					params["name"] = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["description"]; exists {
					params["description"] = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["icon"]; exists {
					params["icon"] = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["entry"]; exists {
					params["entry"] = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["config"]; exists {
					params["config"] = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["package_version"]; exists {
					params["package_version"] = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["poll_interval"]; exists {
					params["poll_interval"] = parseIntFromAny(value, 0)
				}
			}
		}
	}

	preview, err := service.PreviewPaymentMethodPackageArchive(
		h.paymentMethodMarketRuntime(),
		tempPath,
		checksum,
		params,
	)
	if err != nil {
		var validationErr *bizerr.Error
		if errors.As(err, &validationErr) {
			respondPaymentPackageValidationError(c, err)
			return
		}
		h.respondPaymentMethodMarketError(c, err)
		return
	}

	result, err := service.ImportPaymentMethodPackageArchive(
		h.paymentMethodMarketRuntime(),
		tempPath,
		fileHeader.Filename,
		checksum,
		params,
	)
	if err != nil {
		h.respondPaymentMethodMarketError(c, err)
		return
	}

	method, _ := result["item"].(*models.PaymentMethod)
	if method == nil || method.ID == 0 {
		response.InternalServerError(c, "Failed to load payment method", errors.New("payment method import returned empty item"))
		return
	}
	created, _ := result["created"].(bool)
	action := "payment_method_package_import"
	if !created {
		action = "payment_method_package_update"
	}
	webhookCount := 0
	switch value := preview["webhook_count"].(type) {
	case int:
		webhookCount = value
	case int32:
		webhookCount = int(value)
	case int64:
		webhookCount = int(value)
	case float64:
		webhookCount = int(value)
	}
	logger.LogOperation(h.db, c, action, "payment_method", &method.ID, map[string]interface{}{
		"name":          method.Name,
		"version":       method.Version,
		"package_name":  method.PackageName,
		"package_entry": method.PackageEntry,
		"created":       created,
		"script_bytes":  fileHeader.Size,
		"webhook_count": webhookCount,
	})
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"source":            "admin_api",
			"payment_method_id": method.ID,
			"name":              method.Name,
			"version":           method.Version,
			"package_name":      method.PackageName,
			"created":           created,
			"uploaded_by":       adminID,
			"webhook_count":     webhookCount,
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, methodID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "payment.package.import.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("payment.package.import.after hook execution failed: payment_method_id=%d err=%v", methodID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, &adminID, map[string]string{
			"resource_type": "payment_package",
			"resource_id":   fmt.Sprintf("%d", method.ID),
		})), afterPayload, method.ID)
	}

	response.Success(c, result)
}

func respondPaymentPackageValidationError(c *gin.Context, err error) {
	if err == nil {
		response.BadRequest(c, "Invalid payment package")
		return
	}

	var bizErr *bizerr.Error
	if !errors.As(err, &bizErr) {
		response.BadRequest(c, strings.TrimSpace(err.Error()))
		return
	}

	message := strings.TrimSpace(bizErr.Message)
	switch strings.TrimSpace(bizErr.Key) {
	case "plugin.admin.http_400.invalid_package_manifest_json":
		cause := strings.TrimSpace(fmt.Sprint(bizErr.Params["cause"]))
		if cause != "" && cause != "<nil>" {
			message = fmt.Sprintf("Invalid package manifest JSON: %s", cause)
		} else {
			message = "Invalid package manifest JSON"
		}
	case "plugin.admin.http_400.invalid_package_manifest_schema":
		path := strings.TrimSpace(fmt.Sprint(bizErr.Params["path"]))
		reason := strings.TrimSpace(fmt.Sprint(bizErr.Params["reason"]))
		switch {
		case path != "" && path != "<nil>" && reason != "" && reason != "<nil>":
			message = fmt.Sprintf("Invalid package manifest at %s: %s", path, reason)
		case path != "" && path != "<nil>":
			message = fmt.Sprintf("Invalid package manifest at %s", path)
		case reason != "" && reason != "<nil>":
			message = fmt.Sprintf("Invalid package manifest: %s", reason)
		default:
			message = "Invalid package manifest schema"
		}
	}
	if message == "" {
		message = "Invalid payment package"
	}

	response.ErrorWithData(c, http.StatusBadRequest, response.CodeParamError, message, gin.H{
		"error_key": bizErr.Key,
		"params":    bizErr.Params,
	})
}

func saveUploadedPaymentPackageTemp(fileHeader *multipart.FileHeader) (string, string, error) {
	if fileHeader == nil {
		return "", "", fmt.Errorf("empty file")
	}
	if fileHeader.Size > maxPluginPackageUploadBytes {
		return "", "", fmt.Errorf("%w: max=%d bytes", errPluginPackageUploadTooLarge, maxPluginPackageUploadBytes)
	}

	ext := strings.ToLower(filepath.Ext(sanitizeFileName(fileHeader.Filename)))
	if ext == "" {
		ext = ".zip"
	}
	tempFile, err := os.CreateTemp("", "auralogic-payment-package-*"+ext)
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
	}()

	src, err := fileHeader.Open()
	if err != nil {
		_ = os.Remove(tempPath)
		return "", "", fmt.Errorf("failed to open upload: %w", err)
	}
	defer src.Close()

	hasher := sha256.New()
	limitedReader := io.LimitReader(src, maxPluginPackageUploadBytes+1)
	written, err := io.Copy(io.MultiWriter(tempFile, hasher), limitedReader)
	if err != nil {
		_ = os.Remove(tempPath)
		return "", "", fmt.Errorf("failed to save upload: %w", err)
	}
	if written > maxPluginPackageUploadBytes {
		_ = os.Remove(tempPath)
		return "", "", fmt.Errorf("%w: max=%d bytes", errPluginPackageUploadTooLarge, maxPluginPackageUploadBytes)
	}
	return filepath.ToSlash(tempPath), hex.EncodeToString(hasher.Sum(nil)), nil
}

func paymentPackageUploadStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if errors.Is(err, errPluginPackageUploadTooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusInternalServerError
}
