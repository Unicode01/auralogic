package admin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"auralogic/internal/pkg/response"
)

type UploadHandler struct {
	uploadDir string
	baseURL   string
}

func NewUploadHandler(uploadDir, baseURL string) *UploadHandler {
	return &UploadHandler{
		uploadDir: uploadDir,
		baseURL:   baseURL,
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

	// 检查文件大小（限制5MB）
	maxSize := int64(5 * 1024 * 1024)
	if file.Size > maxSize {
		response.BadRequest(c, "File size cannot exceed 5MB")
		return
	}

	// 检查文件类型
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}

	if !allowedExts[ext] {
		response.BadRequest(c, "Only JPG, PNG, GIF, WEBP image formats are supported")
		return
	}

	// generate唯一文件名
	filename := fmt.Sprintf("%s%s", uuid.New().String(), ext)
	
	// 按日期组织目录
	dateDir := time.Now().Format("2006/01/02")
	targetDir := filepath.Join(h.uploadDir, "products", dateDir)
	
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
	imageURL := fmt.Sprintf("%s/uploads/products/%s/%s", h.baseURL, dateDir, filename)

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

	// 从URL中提取文件路径
	// 例如：http://localhost:8080/uploads/products/2026/01/07/uuid.jpg
	// 提取：products/2026/01/07/uuid.jpg
	prefix := h.baseURL + "/uploads/"
	if !strings.HasPrefix(imageURL, prefix) {
		response.BadRequest(c, "Invalid image URL")
		return
	}

	relativePath := strings.TrimPrefix(imageURL, prefix)
	cleanRel := filepath.Clean(relativePath)
	baseDir, err := filepath.Abs(h.uploadDir)
	if err != nil {
		response.InternalError(c, "Failed to resolve upload directory")
		return
	}
	targetPath, err := filepath.Abs(filepath.Join(baseDir, cleanRel))
	if err != nil {
		response.InternalError(c, "Failed to resolve image path")
		return
	}
	if targetPath != baseDir && !strings.HasPrefix(targetPath, baseDir+string(os.PathSeparator)) {
		response.BadRequest(c, "Invalid image path")
		return
	}
	filePath := targetPath

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		response.NotFound(c, "Image does not exist")
		return
	}

	// Delete文件
	if err := os.Remove(filePath); err != nil {
		response.InternalError(c, "Failed to delete image")
		return
	}

	response.Success(c, gin.H{
		"message": "Image deleted",
	})
}

