package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/pkg/logger"
	"gorm.io/gorm"
)

// TicketAttachmentCleanupService 工单附件自动清理服务
type TicketAttachmentCleanupService struct {
	db            *gorm.DB
	cfg           *config.Config
	stopChan      chan struct{}
	wg            sync.WaitGroup
	checkInterval time.Duration
}

// NewTicketAttachmentCleanupService 创建工单附件清理服务
func NewTicketAttachmentCleanupService(db *gorm.DB, cfg *config.Config) *TicketAttachmentCleanupService {
	return &TicketAttachmentCleanupService{
		db:            db,
		cfg:           cfg,
		stopChan:      make(chan struct{}),
		checkInterval: 1 * time.Hour, // 每小时检查一次
	}
}

// Start 启动清理服务
func (s *TicketAttachmentCleanupService) Start() {
	retentionDays := 0
	if s.cfg.Ticket.Attachment != nil {
		retentionDays = s.cfg.Ticket.Attachment.RetentionDays
	}

	logger.LogSystemOperation(s.db, "ticket_attachment_cleanup_start", "system", nil, map[string]interface{}{
		"retention_days": retentionDays,
		"check_interval": s.checkInterval.String(),
	})

	s.wg.Add(1)
	go s.cleanupLoop()
}

// Stop 停止清理服务
func (s *TicketAttachmentCleanupService) Stop() {
	logger.LogSystemOperation(s.db, "ticket_attachment_cleanup_stop", "system", nil, nil)
	close(s.stopChan)
	s.wg.Wait()
}

// cleanupLoop 清理循环
func (s *TicketAttachmentCleanupService) cleanupLoop() {
	defer s.wg.Done()

	// 启动时执行一次
	s.cleanExpiredAttachments()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.cleanExpiredAttachments()
		}
	}
}

// cleanExpiredAttachments 清理过期附件
func (s *TicketAttachmentCleanupService) cleanExpiredAttachments() {
	// 获取最新配置
	retentionDays := 0
	if s.cfg.Ticket.Attachment != nil {
		retentionDays = s.cfg.Ticket.Attachment.RetentionDays
	}
	if retentionDays <= 0 {
		return // 0 或负数表示永久保存
	}

	ticketsDir := filepath.Join(s.cfg.Upload.Dir, "tickets")
	if _, err := os.Stat(ticketsDir); os.IsNotExist(err) {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	deletedCount := 0
	deletedSize := int64(0)

	// 遍历日期目录结构: tickets/YYYY/MM/DD/
	err := filepath.Walk(ticketsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的路径
		}
		if info.IsDir() {
			return nil
		}

		if info.ModTime().Before(cutoff) {
			fileSize := info.Size()
			if removeErr := os.Remove(path); removeErr == nil {
				deletedCount++
				deletedSize += fileSize
			}
		}
		return nil
	})

	if err != nil {
		fmt.Printf("Error walking ticket attachments directory: %v\n", err)
	}

	// 清理空的日期目录
	s.cleanEmptyDirs(ticketsDir)

	if deletedCount > 0 {
		logger.LogSystemOperation(s.db, "ticket_attachment_cleanup", "system", nil, map[string]interface{}{
			"deleted_count":  deletedCount,
			"deleted_size":   deletedSize,
			"retention_days": retentionDays,
			"cutoff_time":    cutoff.Format(time.RFC3339),
		})
	}
}

// cleanEmptyDirs 递归清理空目录
func (s *TicketAttachmentCleanupService) cleanEmptyDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(dir, entry.Name())
			s.cleanEmptyDirs(subDir)

			// 检查子目录是否为空
			subEntries, err := os.ReadDir(subDir)
			if err == nil && len(subEntries) == 0 {
				os.Remove(subDir)
			}
		}
	}
}
