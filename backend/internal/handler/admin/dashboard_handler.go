package admin

import (
	"strings"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"github.com/gin-gonic/gin"
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
	paidStatuses := []models.OrderStatus{
		models.OrderStatusPending,
		models.OrderStatusShipped,
		models.OrderStatusCompleted,
	}

	var stats struct {
		// 版本号（Git Commit）
		GitCommit string `json:"git_commit"`

		// Order统计
		Orders struct {
			Total         int64   `json:"total"`
			Today         int64   `json:"today"`
			ThisMonth     int64   `json:"this_month"`
			LastMonth     int64   `json:"last_month"`
			Pending       int64   `json:"pending"`
			Shipped       int64   `json:"shipped"`
			Completed     int64   `json:"completed"`
			MonthlyGrowth float64 `json:"monthly_growth"`
		} `json:"orders"`

		// 销售额统计
		Sales struct {
			ThisMonth     int64   `json:"this_month"`
			LastMonth     int64   `json:"last_month"`
			Today         int64   `json:"today"`
			MonthlyGrowth float64 `json:"monthly_growth"`
			Currency      string  `json:"currency"`
		} `json:"sales"`

		// User统计
		Users struct {
			Total         int64   `json:"total"`
			Active        int64   `json:"active"`
			Today         int64   `json:"today"`
			ThisMonth     int64   `json:"this_month"`
			LastMonth     int64   `json:"last_month"`
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

	var orderOverview struct {
		Total          int64 `gorm:"column:total"`
		Today          int64 `gorm:"column:today"`
		ThisMonth      int64 `gorm:"column:this_month"`
		LastMonth      int64 `gorm:"column:last_month"`
		Pending        int64 `gorm:"column:pending"`
		Shipped        int64 `gorm:"column:shipped"`
		Completed      int64 `gorm:"column:completed"`
		SalesThisMonth int64 `gorm:"column:sales_this_month"`
		SalesLastMonth int64 `gorm:"column:sales_last_month"`
		SalesToday     int64 `gorm:"column:sales_today"`
	}
	paidStatusCondition := "(status = ? OR status = ? OR status = ?)"
	if err := h.db.Model(&models.Order{}).
		Select(strings.Join([]string{
			"COUNT(*) AS total",
			aggregateCountExpr("created_at >= ?", "today"),
			aggregateCountExpr("created_at >= ?", "this_month"),
			aggregateCountExpr("created_at >= ? AND created_at < ?", "last_month"),
			aggregateCountExpr("status = ?", "pending"),
			aggregateCountExpr("status = ?", "shipped"),
			aggregateCountExpr("status = ?", "completed"),
			aggregateSumExpr("total_amount", paidStatusCondition+" AND created_at >= ?", "sales_this_month"),
			aggregateSumExpr("total_amount", paidStatusCondition+" AND created_at >= ? AND created_at < ?", "sales_last_month"),
			aggregateSumExpr("total_amount", paidStatusCondition+" AND created_at >= ?", "sales_today"),
		}, ", "),
			todayStart,
			monthStart,
			lastMonthStart, monthStart,
			models.OrderStatusPending,
			models.OrderStatusShipped,
			models.OrderStatusCompleted,
			paidStatuses[0], paidStatuses[1], paidStatuses[2], monthStart,
			paidStatuses[0], paidStatuses[1], paidStatuses[2], lastMonthStart, monthStart,
			paidStatuses[0], paidStatuses[1], paidStatuses[2], todayStart,
		).
		Scan(&orderOverview).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}
	stats.Orders.Total = orderOverview.Total
	stats.Orders.Today = orderOverview.Today
	stats.Orders.ThisMonth = orderOverview.ThisMonth
	stats.Orders.LastMonth = orderOverview.LastMonth
	stats.Orders.Pending = orderOverview.Pending
	stats.Orders.Shipped = orderOverview.Shipped
	stats.Orders.Completed = orderOverview.Completed
	stats.Sales.ThisMonth = orderOverview.SalesThisMonth
	stats.Sales.LastMonth = orderOverview.SalesLastMonth
	stats.Sales.Today = orderOverview.SalesToday

	// 计算Order月增长率
	if stats.Orders.LastMonth > 0 {
		stats.Orders.MonthlyGrowth = float64(stats.Orders.ThisMonth-stats.Orders.LastMonth) / float64(stats.Orders.LastMonth) * 100
	}

	// 计算销售额月增长率
	if stats.Sales.LastMonth > 0 {
		stats.Sales.MonthlyGrowth = float64(stats.Sales.ThisMonth-stats.Sales.LastMonth) / float64(stats.Sales.LastMonth) * 100
	}

	// 货币单位
	stats.Sales.Currency = h.cfg.Order.Currency
	if stats.Sales.Currency == "" {
		stats.Sales.Currency = "CNY"
	}

	var userOverview struct {
		UserTotal        int64 `gorm:"column:user_total"`
		UserActive       int64 `gorm:"column:user_active"`
		UserToday        int64 `gorm:"column:user_today"`
		UserThisMonth    int64 `gorm:"column:user_this_month"`
		UserLastMonth    int64 `gorm:"column:user_last_month"`
		AdminTotal       int64 `gorm:"column:admin_total"`
		AdminAdmins      int64 `gorm:"column:admin_admins"`
		AdminSuperAdmins int64 `gorm:"column:admin_super_admins"`
	}
	if err := h.db.Model(&models.User{}).
		Select(strings.Join([]string{
			aggregateCountExpr("role = ?", "user_total"),
			aggregateCountExpr("role = ? AND is_active = ?", "user_active"),
			aggregateCountExpr("role = ? AND created_at >= ?", "user_today"),
			aggregateCountExpr("role = ? AND created_at >= ?", "user_this_month"),
			aggregateCountExpr("role = ? AND created_at >= ? AND created_at < ?", "user_last_month"),
			aggregateCountExpr("(role = ? OR role = ?)", "admin_total"),
			aggregateCountExpr("role = ?", "admin_admins"),
			aggregateCountExpr("role = ?", "admin_super_admins"),
		}, ", "),
			"user",
			"user", true,
			"user", todayStart,
			"user", monthStart,
			"user", lastMonthStart, monthStart,
			"admin", "super_admin",
			"admin",
			"super_admin",
		).
		Scan(&userOverview).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}
	stats.Users.Total = userOverview.UserTotal
	stats.Users.Active = userOverview.UserActive
	stats.Users.Today = userOverview.UserToday
	stats.Users.ThisMonth = userOverview.UserThisMonth
	stats.Users.LastMonth = userOverview.UserLastMonth

	// 计算User月增长率
	if stats.Users.LastMonth > 0 {
		stats.Users.MonthlyGrowth = float64(stats.Users.ThisMonth-stats.Users.LastMonth) / float64(stats.Users.LastMonth) * 100
	}

	stats.Admins.Total = userOverview.AdminTotal
	stats.Admins.Admins = userOverview.AdminAdmins
	stats.Admins.SuperAdmins = userOverview.AdminSuperAdmins

	if err := h.db.Model(&models.APIKey{}).
		Select(strings.Join([]string{
			"COUNT(*) AS total",
			aggregateCountExpr("is_active = ?", "active"),
		}, ", "), true).
		Scan(&stats.APIKeys).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

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
