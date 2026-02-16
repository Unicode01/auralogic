package service

import (
	"fmt"
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

	// 查找超时未回复的工单：
	// - 状态为 open 或 processing 或 resolved
	// - 最后消息时间早于截止时间
	var tickets []models.Ticket
	err := s.db.Where(
		"status IN ? AND last_message_at IS NOT NULL AND last_message_at < ?",
		[]string{
			string(models.TicketStatusOpen),
			string(models.TicketStatusProcessing),
			string(models.TicketStatusResolved),
		},
		cutoff,
	).Find(&tickets).Error

	if err != nil {
		fmt.Printf("Error querying inactive tickets: %v\n", err)
		return
	}

	if len(tickets) == 0 {
		return
	}

	closedCount := 0
	for _, ticket := range tickets {
		err := s.db.Model(&ticket).Updates(map[string]interface{}{
			"status":    models.TicketStatusClosed,
			"closed_at": now,
		}).Error
		if err != nil {
			fmt.Printf("Error auto-closing ticket %s: %v\n", ticket.TicketNo, err)
			continue
		}

		// 添加系统消息
		sysMsg := &models.TicketMessage{
			TicketID:      ticket.ID,
			SenderType:    "admin",
			SenderID:      0,
			SenderName:    "System",
			Content:       fmt.Sprintf("工单已超过 %d 小时无回复，系统自动关闭。", autoCloseHours),
			ContentType:   "text",
			IsReadByUser:  false,
			IsReadByAdmin: true,
		}
		s.db.Create(sysMsg)

		// 更新未读计数
		s.db.Model(&ticket).Update("unread_count_user", gorm.Expr("unread_count_user + 1"))

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
