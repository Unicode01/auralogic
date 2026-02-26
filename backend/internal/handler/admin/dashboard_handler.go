package admin

import (
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	db        *gorm.DB
	cfg       *config.Config
	gitCommit string
}

func NewDashboardHandler(db *gorm.DB, cfg *config.Config, gitCommit string) *DashboardHandler {
	return &DashboardHandler{db: db, cfg: cfg, gitCommit: gitCommit}
}

// GetStatistics get仪表盘统计数据
func (h *DashboardHandler) GetStatistics(c *gin.Context) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonthStart := monthStart.AddDate(0, -1, 0)

	var stats struct {
		// 版本号（Git Commit）
		GitCommit string `json:"git_commit"`

		// Order统计
		Orders struct {
			Total          int64   `json:"total"`
			Today          int64   `json:"today"`
			ThisMonth      int64   `json:"this_month"`
			LastMonth      int64   `json:"last_month"`
			Pending        int64   `json:"pending"`
			Shipped        int64   `json:"shipped"`
			Completed      int64   `json:"completed"`
			MonthlyGrowth  float64 `json:"monthly_growth"`
		} `json:"orders"`

		// 销售额统计
		Sales struct {
			ThisMonth     float64 `json:"this_month"`
			LastMonth     float64 `json:"last_month"`
			Today         float64 `json:"today"`
			MonthlyGrowth float64 `json:"monthly_growth"`
			Currency      string  `json:"currency"`
		} `json:"sales"`

		// User统计
		Users struct {
			Total       int64   `json:"total"`
			Active      int64   `json:"active"`
			Today       int64   `json:"today"`
			ThisMonth   int64   `json:"this_month"`
			LastMonth   int64   `json:"last_month"`
			MonthlyGrowth float64 `json:"monthly_growth"`
		} `json:"users"`

		// Admin统计
		Admins struct {
			Total       int64 `json:"total"`
			Admins      int64 `json:"admins"`
			SuperAdmins int64 `json:"super_admins"`
		} `json:"admins"`

		// API密钥统计
		APIKeys struct {
			Total  int64 `json:"total"`
			Active int64 `json:"active"`
		} `json:"api_keys"`

		// 最近活动
		RecentOrders []models.Order `json:"recent_orders"`

		// Order状态分布
		OrderStatusDistribution []struct {
			Status string `json:"status"`
			Count  int64  `json:"count"`
		} `json:"order_status_distribution"`
	}

	stats.GitCommit = h.gitCommit

	// Order统计
	h.db.Model(&models.Order{}).Count(&stats.Orders.Total)
	h.db.Model(&models.Order{}).Where("created_at >= ?", todayStart).Count(&stats.Orders.Today)
	h.db.Model(&models.Order{}).Where("created_at >= ?", monthStart).Count(&stats.Orders.ThisMonth)
	h.db.Model(&models.Order{}).Where("created_at >= ? AND created_at < ?", lastMonthStart, monthStart).Count(&stats.Orders.LastMonth)
	h.db.Model(&models.Order{}).Where("status = ?", models.OrderStatusPending).Count(&stats.Orders.Pending)
	h.db.Model(&models.Order{}).Where("status = ?", models.OrderStatusShipped).Count(&stats.Orders.Shipped)
	h.db.Model(&models.Order{}).Where("status = ?", models.OrderStatusCompleted).Count(&stats.Orders.Completed)

	// 计算Order月增长率
	if stats.Orders.LastMonth > 0 {
		stats.Orders.MonthlyGrowth = float64(stats.Orders.ThisMonth-stats.Orders.LastMonth) / float64(stats.Orders.LastMonth) * 100
	}

	// 销售额统计（只统计已付款的订单：pending, shipped, completed）
	paidStatuses := []models.OrderStatus{
		models.OrderStatusPending,
		models.OrderStatusShipped,
		models.OrderStatusCompleted,
	}

	// 本月销售额
	var thisMonthSales struct {
		Total float64
	}
	h.db.Model(&models.Order{}).
		Select("COALESCE(SUM(total_amount), 0) as total").
		Where("created_at >= ? AND status IN ?", monthStart, paidStatuses).
		Scan(&thisMonthSales)
	stats.Sales.ThisMonth = thisMonthSales.Total

	// 上月销售额
	var lastMonthSales struct {
		Total float64
	}
	h.db.Model(&models.Order{}).
		Select("COALESCE(SUM(total_amount), 0) as total").
		Where("created_at >= ? AND created_at < ? AND status IN ?", lastMonthStart, monthStart, paidStatuses).
		Scan(&lastMonthSales)
	stats.Sales.LastMonth = lastMonthSales.Total

	// 今日销售额
	var todaySales struct {
		Total float64
	}
	h.db.Model(&models.Order{}).
		Select("COALESCE(SUM(total_amount), 0) as total").
		Where("created_at >= ? AND status IN ?", todayStart, paidStatuses).
		Scan(&todaySales)
	stats.Sales.Today = todaySales.Total

	// 计算销售额月增长率
	if stats.Sales.LastMonth > 0 {
		stats.Sales.MonthlyGrowth = (stats.Sales.ThisMonth - stats.Sales.LastMonth) / stats.Sales.LastMonth * 100
	}

	// 货币单位
	stats.Sales.Currency = h.cfg.Order.Currency
	if stats.Sales.Currency == "" {
		stats.Sales.Currency = "CNY"
	}

	// User统计
	h.db.Model(&models.User{}).Where("role = ?", "user").Count(&stats.Users.Total)
	h.db.Model(&models.User{}).Where("role = ? AND is_active = ?", "user", true).Count(&stats.Users.Active)
	h.db.Model(&models.User{}).Where("role = ? AND created_at >= ?", "user", todayStart).Count(&stats.Users.Today)
	h.db.Model(&models.User{}).Where("role = ? AND created_at >= ?", "user", monthStart).Count(&stats.Users.ThisMonth)
	h.db.Model(&models.User{}).Where("role = ? AND created_at >= ? AND created_at < ?", "user", lastMonthStart, monthStart).Count(&stats.Users.LastMonth)

	// 计算User月增长率
	if stats.Users.LastMonth > 0 {
		stats.Users.MonthlyGrowth = float64(stats.Users.ThisMonth-stats.Users.LastMonth) / float64(stats.Users.LastMonth) * 100
	}

	// Admin统计
	h.db.Model(&models.User{}).Where("role IN ?", []string{"admin", "super_admin"}).Count(&stats.Admins.Total)
	h.db.Model(&models.User{}).Where("role = ?", "admin").Count(&stats.Admins.Admins)
	h.db.Model(&models.User{}).Where("role = ?", "super_admin").Count(&stats.Admins.SuperAdmins)

	// API密钥统计
	h.db.Model(&models.APIKey{}).Count(&stats.APIKeys.Total)
	h.db.Model(&models.APIKey{}).Where("is_active = ?", true).Count(&stats.APIKeys.Active)

	// 最近10个Order
	h.db.Model(&models.Order{}).
		Order("created_at DESC").
		Limit(10).
		Find(&stats.RecentOrders)

	// Order状态分布
	h.db.Model(&models.Order{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&stats.OrderStatusDistribution)

	response.Success(c, stats)
}

// GetRecentActivities get最近活动
func (h *DashboardHandler) GetRecentActivities(c *gin.Context) {
	var activities []models.OperationLog
	
	h.db.Model(&models.OperationLog{}).
		Preload("User").
		Order("created_at DESC").
		Limit(20).
		Find(&activities)

	response.Success(c, activities)
}

