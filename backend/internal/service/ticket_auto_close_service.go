package service

import (
	"fmt"
	"log"
	"strconv"
	"strings"
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
	pluginManager *PluginManagerService
	lifecycleMu   sync.Mutex
	running       bool
	stopChan      chan struct{}
	doneChan      chan struct{}
	checkInterval time.Duration
}

// NewTicketAutoCloseService 创建工单自动关闭服务
func NewTicketAutoCloseService(db *gorm.DB, cfg *config.Config) *TicketAutoCloseService {
	return &TicketAutoCloseService{
		db:            db,
		cfg:           cfg,
		checkInterval: 30 * time.Minute, // 每30分钟检查一次
	}
}

func (s *TicketAutoCloseService) SetPluginManager(pluginManager *PluginManagerService) {
	s.pluginManager = pluginManager
}

func cloneTicketAutoCloseExecutionContext(execCtx *ExecutionContext) *ExecutionContext {
	if execCtx == nil {
		return nil
	}
	cloned := &ExecutionContext{
		SessionID:      execCtx.SessionID,
		RequestContext: execCtx.RequestContext,
	}
	if execCtx.UserID != nil {
		userID := *execCtx.UserID
		cloned.UserID = &userID
	}
	if execCtx.OrderID != nil {
		orderID := *execCtx.OrderID
		cloned.OrderID = &orderID
	}
	if len(execCtx.Metadata) > 0 {
		cloned.Metadata = make(map[string]string, len(execCtx.Metadata))
		for key, value := range execCtx.Metadata {
			cloned.Metadata[key] = value
		}
	}
	return cloned
}

func (s *TicketAutoCloseService) buildTicketAutoCloseExecutionContext(ticket *models.Ticket) *ExecutionContext {
	if ticket == nil {
		return nil
	}
	userID := ticket.UserID
	metadata := map[string]string{
		"source":    "ticket_auto_close",
		"ticket_id": strconv.FormatUint(uint64(ticket.ID), 10),
		"ticket_no": ticket.TicketNo,
	}
	return &ExecutionContext{
		UserID:   &userID,
		Metadata: metadata,
	}
}

func ticketAutoCloseHookValueToOptionalString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	str, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("value must be string")
	}
	return str, nil
}

// Start 启动自动关闭服务
func (s *TicketAutoCloseService) Start() {
	s.lifecycleMu.Lock()
	if s.running {
		s.lifecycleMu.Unlock()
		return
	}
	stopChan := make(chan struct{})
	doneChan := make(chan struct{})
	s.stopChan = stopChan
	s.doneChan = doneChan
	s.running = true
	s.lifecycleMu.Unlock()

	logger.LogSystemOperation(s.db, "ticket_auto_close_start", "system", nil, map[string]interface{}{
		"auto_close_hours": s.cfg.Ticket.AutoCloseHours,
		"check_interval":   s.checkInterval.String(),
	})

	go func() {
		defer close(doneChan)
		runBackgroundServiceWithStopChan("ticket_auto_close.closeLoop", stopChan, s.closeLoop)
	}()
}

// Stop 停止自动关闭服务
func (s *TicketAutoCloseService) Stop() {
	s.lifecycleMu.Lock()
	if !s.running {
		s.lifecycleMu.Unlock()
		return
	}
	stopChan := s.stopChan
	doneChan := s.doneChan
	s.stopChan = nil
	s.doneChan = nil
	s.running = false

	logger.LogSystemOperation(s.db, "ticket_auto_close_stop", "system", nil, nil)
	close(stopChan)
	<-doneChan
	s.lifecycleMu.Unlock()
}

// closeLoop 自动关闭循环
func (s *TicketAutoCloseService) closeLoop(stopChan <-chan struct{}) {
	// 启动时执行一次
	s.closeInactiveTickets()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
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
		closed, err := s.closeTicket(&ticket, autoCloseHours, now, activeStatuses)
		if err != nil {
			log.Printf("[TicketAutoClose] Error closing ticket %s: %v", ticket.TicketNo, err)
			continue
		}
		if closed {
			closedCount++
		}
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
func (s *TicketAutoCloseService) closeTicket(ticket *models.Ticket, autoCloseHours int, now time.Time, activeStatuses []string) (bool, error) {
	if ticket == nil {
		return false, fmt.Errorf("ticket is nil")
	}

	beforeStatus := ticket.Status
	msgContent := ticketAutoCloseMessage(ticket.User, autoCloseHours)
	hookExecCtx := cloneTicketAutoCloseExecutionContext(s.buildTicketAutoCloseExecutionContext(ticket))
	if s.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"ticket_id":         ticket.ID,
			"ticket_no":         ticket.TicketNo,
			"user_id":           ticket.UserID,
			"status_before":     beforeStatus,
			"auto_close_hours":  autoCloseHours,
			"last_message_at":   ticket.LastMessageAt,
			"last_message_by":   ticket.LastMessageBy,
			"unread_count_user": ticket.UnreadCountUser,
			"source":            "ticket_auto_close",
		}
		hookResult, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
			Hook:    "ticket.auto_close.before",
			Payload: hookPayload,
		}, hookExecCtx)
		if hookErr != nil {
			log.Printf("ticket.auto_close.before hook execution failed: ticket=%s err=%v", ticket.TicketNo, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "ticket auto-close blocked by plugin"
				}
				log.Printf("[TicketAutoClose] Skip auto-close ticket %s: %s", ticket.TicketNo, reason)
				return false, nil
			}
			if hookResult.Payload != nil {
				if rawContent, exists := hookResult.Payload["content"]; exists {
					content, convErr := ticketAutoCloseHookValueToOptionalString(rawContent)
					if convErr != nil {
						log.Printf("ticket.auto_close.before payload content decode failed, fallback to default: ticket=%s err=%v", ticket.TicketNo, convErr)
					} else if strings.TrimSpace(content) != "" {
						msgContent = content
					}
				} else if rawMessage, exists := hookResult.Payload["message"]; exists {
					message, convErr := ticketAutoCloseHookValueToOptionalString(rawMessage)
					if convErr != nil {
						log.Printf("ticket.auto_close.before payload message decode failed, fallback to default: ticket=%s err=%v", ticket.TicketNo, convErr)
					} else if strings.TrimSpace(message) != "" {
						msgContent = message
					}
				}
			}
		}
	}

	closed := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
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
		closed = true

		// 生成系统消息（支持被插件 before hook 覆写）
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
	if err != nil {
		return false, err
	}
	if !closed {
		return false, nil
	}

	if s.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"ticket_id":        ticket.ID,
			"ticket_no":        ticket.TicketNo,
			"user_id":          ticket.UserID,
			"status_before":    beforeStatus,
			"status_after":     models.TicketStatusClosed,
			"auto_close_hours": autoCloseHours,
			"content":          msgContent,
			"source":           "ticket_auto_close",
		}
		go func(execCtx *ExecutionContext, payload map[string]interface{}, ticketNo string) {
			_, hookErr := s.pluginManager.ExecuteHook(HookExecutionRequest{
				Hook:    "ticket.auto_close.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("ticket.auto_close.after hook execution failed: ticket=%s err=%v", ticketNo, hookErr)
			}
		}(hookExecCtx, afterPayload, ticket.TicketNo)
	}

	return true, nil
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
