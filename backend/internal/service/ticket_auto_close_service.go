package service

import (
	"fmt"
	"log"
	"sync"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
	"gorm.io/gorm"
)

// TicketAutoCloseService 工单超时自动关闭服务
type TicketAutoCloseService struct {
	db            *gorm.DB
	cfg           *config.Config
	stopChan      chan struct{}
	wg            sync.WaitGroup
	checkInterval time.Duration
}

// NewTicketAutoCloseService 创建工单自动关闭服务
func NewTicketAutoCloseService(db *gorm.DB, cfg *config.Config) *TicketAutoCloseService {
	return &TicketAutoCloseService{
		db:            db,
		cfg:           cfg,
		stopChan:      make(chan struct{}),
		checkInterval: 30 * time.Minute, // 每30分钟检查一次
	}
}

// Start 启动自动关闭服务
func (s *TicketAutoCloseService) Start() {
	logger.LogSystemOperation(s.db, "ticket_auto_close_start", "system", nil, map[string]interface{}{
		"auto_close_hours": s.cfg.Ticket.AutoCloseHours,
		"check_interval":   s.checkInterval.String(),
	})

	s.wg.Add(1)
	go s.closeLoop()
}

// Stop 停止自动关闭服务
func (s *TicketAutoCloseService) Stop() {
	logger.LogSystemOperation(s.db, "ticket_auto_close_stop", "system", nil, nil)
	close(s.stopChan)
	s.wg.Wait()
}

// closeLoop 自动关闭循环
func (s *TicketAutoCloseService) closeLoop() {
	defer s.wg.Done()

	// 启动时执行一次
	s.closeInactiveTickets()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.closeInactiveTickets()
		}
	}
}

// closeInactiveTickets 关闭超时无回复的工单
func (s *TicketAutoCloseService) closeInactiveTickets() {
	// 每次执行时读取最新配置，支持热更新
	autoCloseHours := s.cfg.Ticket.AutoCloseHours
	if autoCloseHours <= 0 {
		return // 0 或负数表示不自动关闭
	}

	cutoff := time.Now().Add(-time.Duration(autoCloseHours) * time.Hour)
	now := time.Now()

	activeStatuses := []string{
		string(models.TicketStatusOpen),
		string(models.TicketStatusProcessing),
		string(models.TicketStatusResolved),
	}

	// 分批查询超时未回复的工单，每次最多处理100条，关联用户以获取语言偏好
	var tickets []models.Ticket
	err := s.db.Preload("User").Where(
		"status IN ? AND last_message_at IS NOT NULL AND last_message_at < ?",
		activeStatuses,
		cutoff,
	).Limit(100).Find(&tickets).Error

	if err != nil {
		log.Printf("[TicketAutoClose] Error querying inactive tickets: %v", err)
		return
	}

	if len(tickets) == 0 {
		return
	}

	closedCount := 0
	for _, ticket := range tickets {
		if err := s.closeTicket(&ticket, autoCloseHours, now, activeStatuses); err != nil {
			log.Printf("[TicketAutoClose] Error closing ticket %s: %v", ticket.TicketNo, err)
			continue
		}
		closedCount++
	}

	if closedCount > 0 {
		logger.LogSystemOperation(s.db, "ticket_auto_close", "system", nil, map[string]interface{}{
			"closed_count":     closedCount,
			"auto_close_hours": autoCloseHours,
			"cutoff_time":      cutoff.Format(time.RFC3339),
		})
	}
}

// closeTicket 关闭单个工单（事务内完成状态更新、系统消息、未读计数）
func (s *TicketAutoCloseService) closeTicket(ticket *models.Ticket, autoCloseHours int, now time.Time, activeStatuses []string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 原子更新状态，WHERE status 条件防止并发重复处理
		result := tx.Model(ticket).
			Where("status IN ?", activeStatuses).
			Updates(map[string]interface{}{
				"status":    models.TicketStatusClosed,
				"closed_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			// 工单状态已被其他流程修改，跳过
			return nil
		}

		// 根据用户语言偏好生成系统消息
		msgContent := ticketAutoCloseMessage(ticket.User, autoCloseHours)
		sysMsg := &models.TicketMessage{
			TicketID:      ticket.ID,
			SenderType:    "admin",
			SenderID:      0,
			SenderName:    "System",
			Content:       msgContent,
			ContentType:   "text",
			IsReadByUser:  false,
			IsReadByAdmin: true,
		}
		if err := tx.Create(sysMsg).Error; err != nil {
			return err
		}

		// 更新未读计数和最后消息信息
		preview := msgContent
		if len(preview) > 200 {
			preview = preview[:200]
		}
		if err := tx.Model(ticket).Updates(map[string]interface{}{
			"unread_count_user":    gorm.Expr("unread_count_user + 1"),
			"last_message_at":      now,
			"last_message_preview": preview,
			"last_message_by":      "admin",
		}).Error; err != nil {
			return err
		}

		return nil
	})
}

// ticketAutoCloseMessage 根据用户语言偏好生成自动关闭消息
func ticketAutoCloseMessage(user *models.User, hours int) string {
	locale := ""
	if user != nil {
		locale = user.Locale
	}
	switch locale {
	case "zh":
		return fmt.Sprintf("工单已超过 %d 小时无回复，系统自动关闭。", hours)
	default:
		return fmt.Sprintf("Ticket automatically closed after %d hours with no reply.", hours)
	}
}
