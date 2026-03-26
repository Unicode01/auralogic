package admin

import (
	"fmt"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type UploadHandler struct {
	uploadDir     string
	baseURL       string
	pluginManager *service.PluginManagerService
}

func NewUploadHandler(uploadDir, baseURL string, pluginManager *service.PluginManagerService) *UploadHandler {
	return &UploadHandler{
		uploadDir:     uploadDir,
		baseURL:       baseURL,
		pluginManager: pluginManager,
	}
}

func defaultUploadImageAllowedExts() map[string]bool {
	return map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}
}

func normalizeUploadAllowedExts(values []string) map[string]bool {
	allowed := make(map[string]bool)
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" || strings.Contains(normalized, "/") {
			continue
		}
		if !strings.HasPrefix(normalized, ".") {
			normalized = "." + strings.TrimPrefix(normalized, ".")
		}
		allowed[normalized] = true
	}
	return allowed
}

func (h *UploadHandler) currentUploadRuntime() (string, string, int64, map[string]bool) {
	uploadDir := strings.TrimSpace(h.uploadDir)
	baseURL := strings.TrimRight(strings.TrimSpace(h.baseURL), "/")
	maxSize := int64(5 * 1024 * 1024)
	allowedExts := defaultUploadImageAllowedExts()

	if runtimeCfg := config.GetConfig(); runtimeCfg != nil {
		if strings.TrimSpace(runtimeCfg.Upload.Dir) != "" {
			uploadDir = strings.TrimSpace(runtimeCfg.Upload.Dir)
		}
		baseURL = strings.TrimRight(strings.TrimSpace(runtimeCfg.App.URL), "/")
		if runtimeCfg.Upload.MaxSize > 0 {
			maxSize = runtimeCfg.Upload.MaxSize
		}
		if normalized := normalizeUploadAllowedExts(runtimeCfg.Upload.AllowedTypes); len(normalized) > 0 {
			allowedExts = normalized
		}
	}

	if uploadDir == "" {
		uploadDir = "uploads"
	}
	return uploadDir, baseURL, maxSize, allowedExts
}

func extractProductUploadRelativePath(imageURL string) (string, bool) {
	const marker = "/uploads/products/"
	idx := strings.Index(strings.TrimSpace(imageURL), marker)
	if idx < 0 {
		return "", false
	}

	relativePath := strings.TrimPrefix(imageURL[idx+len(marker):], "/")
	cleanRel := filepath.Clean(filepath.FromSlash(relativePath))
	if cleanRel == "." || strings.HasPrefix(cleanRel, "..") {
		return "", false
	}
	return filepath.ToSlash(cleanRel), true
}

func resolveProductUploadFilePath(uploadDirs []string, relativePath string) (string, error) {
	cleanRel := filepath.Clean(relativePath)
	if cleanRel == "." || strings.HasPrefix(cleanRel, "..") {
		return "", fmt.Errorf("invalid image path")
	}

	seen := make(map[string]struct{})
	for _, uploadDir := range uploadDirs {
		trimmedDir := strings.TrimSpace(uploadDir)
		if trimmedDir == "" {
			continue
		}
		if _, exists := seen[trimmedDir]; exists {
			continue
		}
		seen[trimmedDir] = struct{}{}

		baseDir, err := filepath.Abs(filepath.Join(trimmedDir, "products"))
		if err != nil {
			continue
		}
		targetPath, err := filepath.Abs(filepath.Join(baseDir, cleanRel))
		if err != nil {
			continue
		}
		if targetPath != baseDir && !strings.HasPrefix(targetPath, baseDir+string(os.PathSeparator)) {
			continue
		}
		if _, err := os.Stat(targetPath); err == nil {
			return targetPath, nil
		}
	}

	return "", fmt.Errorf("image does not exist")
}

func buildUploadImageFileHookPayload(file *multipart.FileHeader) map[string]interface{} {
	if file == nil {
		return map[string]interface{}{}
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	return map[string]interface{}{
		"original_filename": file.Filename,
		"size":              file.Size,
		"extension":         ext,
		"content_type":      file.Header.Get("Content-Type"),
		"upload_area":       "products",
	}
}

func buildUploadDeleteHookPayload(imageURL string, relativePath string) map[string]interface{} {
	filename := filepath.Base(relativePath)
	ext := strings.ToLower(filepath.Ext(filename))
	return map[string]interface{}{
		"url":           imageURL,
		"relative_path": relativePath,
		"filename":      filename,
		"extension":     ext,
		"upload_area":   "products",
	}
}

// UploadImage 上传图片
func (h *UploadHandler) UploadImage(c *gin.Context) {
	// get上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "Please select a file to upload")
		return
	}

	uploadDir, baseURL, maxSize, allowedExts := h.currentUploadRuntime()

	// 检查文件大小
	if file.Size > maxSize {
		response.BadRequest(c, fmt.Sprintf("File size cannot exceed %dMB", maxSize/1024/1024))
		return
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedExts[ext] {
		response.BadRequest(c, "This file type is not allowed")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		hookPayload := buildUploadImageFileHookPayload(file)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "upload.image.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "upload_image",
			"hook_source":   "admin_api",
			"hook_action":   "upload",
		}))
		if hookErr != nil {
			log.Printf("upload.image.before hook execution failed: admin=%d filename=%s err=%v", adminIDValue, file.Filename, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Image upload rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	// generate唯一文件名
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)

	// 按日期组织目录
	dateDir := time.Now().Format("2006/01/02")
	targetDir := filepath.Join(uploadDir, "products", dateDir)

	// Create目录
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		response.InternalError(c, "Failed to create directory")
		return
	}

	// 保存文件
	targetPath := filepath.Join(targetDir, filename)
	if err := c.SaveUploadedFile(file, targetPath); err != nil {
		response.InternalError(c, "Failed to save file")
		return
	}

	// 构建访问URL
	imageURL := fmt.Sprintf("%s/uploads/products/%s/%s", baseURL, dateDir, filename)
	relativePath := filepath.ToSlash(filepath.Join(dateDir, filename))

	if h.pluginManager != nil {
		afterPayload := buildUploadImageFileHookPayload(file)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		afterPayload["filename"] = filename
		afterPayload["url"] = imageURL
		afterPayload["relative_path"] = relativePath
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, storedFilename string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "upload.image.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("upload.image.after hook execution failed: admin=%d filename=%s err=%v", adminIDValue, storedFilename, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "upload_image",
			"hook_source":   "admin_api",
			"hook_action":   "upload",
		})), afterPayload, filename)
	}

	response.Success(c, gin.H{
		"url":      imageURL,
		"filename": filename,
		"size":     file.Size,
	})
}

// DeleteImage Delete图片
func (h *UploadHandler) DeleteImage(c *gin.Context) {
	imageURL := c.PostForm("url")
	if imageURL == "" {
		response.BadRequest(c, "Please provide image URL")
		return
	}

	uploadDir, _, _, _ := h.currentUploadRuntime()
	relativePath, ok := extractProductUploadRelativePath(imageURL)
	if !ok {
		response.BadRequest(c, "Invalid image URL")
		return
	}

	cleanRel := filepath.Clean(relativePath)
	if cleanRel == "." || strings.HasPrefix(cleanRel, "..") {
		response.BadRequest(c, "Invalid image path")
		return
	}
	filePath, err := resolveProductUploadFilePath([]string{uploadDir, h.uploadDir}, cleanRel)
	if err != nil {
		response.NotFound(c, "Image does not exist")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	hookPayload := buildUploadDeleteHookPayload(imageURL, filepath.ToSlash(cleanRel))
	if h.pluginManager != nil {
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "upload.image.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "upload_image",
			"hook_source":   "admin_api",
			"hook_action":   "delete",
		}))
		if hookErr != nil {
			log.Printf("upload.image.delete.before hook execution failed: admin=%d url=%s err=%v", adminIDValue, imageURL, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Image deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	// Delete文件
	if err := os.Remove(filePath); err != nil {
		response.InternalError(c, "Failed to delete image")
		return
	}

	if h.pluginManager != nil {
		afterPayload := buildUploadDeleteHookPayload(imageURL, filepath.ToSlash(cleanRel))
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, deletedURL string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "upload.image.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("upload.image.delete.after hook execution failed: admin=%d url=%s err=%v", adminIDValue, deletedURL, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "upload_image",
			"hook_source":   "admin_api",
			"hook_action":   "delete",
		})), afterPayload, imageURL)
	}

	response.Success(c, gin.H{
		"message": "Image deleted",
	})
}
