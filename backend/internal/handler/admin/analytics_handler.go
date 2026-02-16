package admin

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"gorm.io/gorm"
)

type AnalyticsHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewAnalyticsHandler(db *gorm.DB, cfg *config.Config) *AnalyticsHandler {
	return &AnalyticsHandler{db: db, cfg: cfg}
}

// dateGroupExpr returns the SQL expression for grouping by date, compatible across SQLite/MySQL/PostgreSQL
func (h *AnalyticsHandler) dateGroupExpr(column string) string {
	switch h.cfg.Database.Driver {
	case "mysql":
		return fmt.Sprintf("DATE(%s)", column)
	case "postgres":
		return fmt.Sprintf("DATE(%s)", column)
	default: // sqlite
		return fmt.Sprintf("DATE(%s)", column)
	}
}

// monthGroupExpr returns the SQL expression for grouping by month
func (h *AnalyticsHandler) monthGroupExpr(column string) string {
	switch h.cfg.Database.Driver {
	case "mysql":
		return fmt.Sprintf("DATE_FORMAT(%s, '%%Y-%%m')", column)
	case "postgres":
		return fmt.Sprintf("TO_CHAR(%s, 'YYYY-MM')", column)
	default: // sqlite
		return fmt.Sprintf("strftime('%%Y-%%m', %s)", column)
	}
}

// checkEnabled returns true if analytics is disabled (and sends the response).
// Callers should return immediately when this returns true.
func (h *AnalyticsHandler) checkDisabled(c *gin.Context) bool {
	if !h.cfg.Analytics.Enabled {
		response.Success(c, gin.H{"enabled": false})
		return true
	}
	return false
}

// GetUserAnalytics returns user analytics data
func (h *AnalyticsHandler) GetUserAnalytics(c *gin.Context) {
	if h.checkDisabled(c) {
		return
	}
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonthStart := monthStart.AddDate(0, -1, 0)

	paidStatuses := []models.OrderStatus{
		models.OrderStatusPending,
		models.OrderStatusShipped,
		models.OrderStatusCompleted,
	}

	var result struct {
		// Overview
		Overview struct {
			Total         int64   `json:"total"`
			Active        int64   `json:"active"`
			Inactive      int64   `json:"inactive"`
			ThisMonth     int64   `json:"this_month"`
			LastMonth     int64   `json:"last_month"`
			MonthlyGrowth float64 `json:"monthly_growth"`
		} `json:"overview"`

		// Registration trend (last 30 days)
		RegistrationTrend []struct {
			Date  string `json:"date"`
			Count int64  `json:"count"`
		} `json:"registration_trend"`

		// Country distribution
		CountryDistribution []struct {
			Country string `json:"country"`
			Count   int64  `json:"count"`
		} `json:"country_distribution"`

		// Locale distribution
		LocaleDistribution []struct {
			Locale string `json:"locale"`
			Count  int64  `json:"count"`
		} `json:"locale_distribution"`

		// Users with orders vs without
		OrderEngagement struct {
			WithOrders    int64 `json:"with_orders"`
			WithoutOrders int64 `json:"without_orders"`
		} `json:"order_engagement"`

		// Top users by order count
		TopUsers []struct {
			ID         uint    `json:"id"`
			Name       string  `json:"name"`
			Email      string  `json:"email"`
			OrderCount int64   `json:"order_count"`
			TotalSpent float64 `json:"total_spent"`
		} `json:"top_users"`
	}

	// Overview
	h.db.Model(&models.User{}).Where("role = ?", "user").Count(&result.Overview.Total)
	h.db.Model(&models.User{}).Where("role = ? AND is_active = ?", "user", true).Count(&result.Overview.Active)
	h.db.Model(&models.User{}).Where("role = ? AND is_active = ?", "user", false).Count(&result.Overview.Inactive)
	h.db.Model(&models.User{}).Where("role = ? AND created_at >= ?", "user", monthStart).Count(&result.Overview.ThisMonth)
	h.db.Model(&models.User{}).Where("role = ? AND created_at >= ? AND created_at < ?", "user", lastMonthStart, monthStart).Count(&result.Overview.LastMonth)
	if result.Overview.LastMonth > 0 {
		result.Overview.MonthlyGrowth = float64(result.Overview.ThisMonth-result.Overview.LastMonth) / float64(result.Overview.LastMonth) * 100
	}

	// Registration trend (last 30 days)
	dateExpr := h.dateGroupExpr("created_at")
	h.db.Model(&models.User{}).
		Select(fmt.Sprintf("%s as date, COUNT(*) as count", dateExpr)).
		Where("role = ? AND created_at >= ?", "user", thirtyDaysAgo).
		Group(dateExpr).
		Order("date").
		Scan(&result.RegistrationTrend)

	// Country distribution
	h.db.Model(&models.User{}).
		Select("COALESCE(NULLIF(country, ''), 'Unknown') as country, COUNT(*) as count").
		Where("role = ?", "user").
		Group("country").
		Order("count DESC").
		Limit(20).
		Scan(&result.CountryDistribution)

	// Locale distribution
	h.db.Model(&models.User{}).
		Select("COALESCE(NULLIF(locale, ''), 'Unknown') as locale, COUNT(*) as count").
		Where("role = ?", "user").
		Group("locale").
		Order("count DESC").
		Scan(&result.LocaleDistribution)

	// Order engagement
	h.db.Model(&models.User{}).
		Where("role = ? AND id IN (?)",
			"user",
			// "with orders" is defined as having at least one paid order.
			h.db.Model(&models.Order{}).
				Select("DISTINCT user_id").
				Where("user_id IS NOT NULL AND deleted_at IS NULL AND status IN ?", paidStatuses),
		).Count(&result.OrderEngagement.WithOrders)
	result.OrderEngagement.WithoutOrders = result.Overview.Total - result.OrderEngagement.WithOrders

	// Top users by order count (top 10)
	h.db.Model(&models.Order{}).
		Select("users.id, users.name, users.email, COUNT(orders.id) as order_count, COALESCE(SUM(orders.total_amount), 0) as total_spent").
		Joins("JOIN users ON users.id = orders.user_id").
		Where("orders.user_id IS NOT NULL AND orders.deleted_at IS NULL AND users.deleted_at IS NULL AND orders.status IN ?", paidStatuses).
		Group("users.id, users.name, users.email").
		Order("order_count DESC").
		Limit(10).
		Scan(&result.TopUsers)

	response.Success(c, result)
}

// GetOrderAnalytics returns order analytics data
func (h *AnalyticsHandler) GetOrderAnalytics(c *gin.Context) {
	if h.checkDisabled(c) {
		return
	}
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonthStart := monthStart.AddDate(0, -1, 0)

	paidStatuses := []models.OrderStatus{
		models.OrderStatusPending,
		models.OrderStatusShipped,
		models.OrderStatusCompleted,
	}

	var result struct {
		// Overview
		Overview struct {
			Total         int64   `json:"total"`
			ThisMonth     int64   `json:"this_month"`
			LastMonth     int64   `json:"last_month"`
			MonthlyGrowth float64 `json:"monthly_growth"`
			AvgOrderValue float64 `json:"avg_order_value"`
			Currency      string  `json:"currency"`
		} `json:"overview"`

		// Order trend (daily, last 30 days)
		OrderTrend []struct {
			Date  string `json:"date"`
			Count int64  `json:"count"`
		} `json:"order_trend"`

		// Status distribution
		StatusDistribution []struct {
			Status string `json:"status"`
			Count  int64  `json:"count"`
		} `json:"status_distribution"`

		// Source distribution
		SourceDistribution []struct {
			Source string `json:"source"`
			Count  int64  `json:"count"`
		} `json:"source_distribution"`

		// Source platform distribution
		PlatformDistribution []struct {
			Platform string `json:"platform"`
			Count    int64  `json:"count"`
		} `json:"platform_distribution"`

		// Country distribution (by receiver_country)
		CountryDistribution []struct {
			Country string `json:"country"`
			Count   int64  `json:"count"`
		} `json:"country_distribution"`

		// Amount distribution (ranges)
		AmountDistribution []struct {
			Range string `json:"range"`
			Count int64  `json:"count"`
		} `json:"amount_distribution"`

		// Top products by sale_count
		TopProducts []struct {
			ID        uint    `json:"id"`
			SKU       string  `json:"sku"`
			Name      string  `json:"name"`
			SaleCount int     `json:"sale_count"`
			Price     float64 `json:"price"`
			Category  string  `json:"category"`
		} `json:"top_products"`
	}

	// Overview
	h.db.Model(&models.Order{}).Count(&result.Overview.Total)
	h.db.Model(&models.Order{}).Where("created_at >= ?", monthStart).Count(&result.Overview.ThisMonth)
	h.db.Model(&models.Order{}).Where("created_at >= ? AND created_at < ?", lastMonthStart, monthStart).Count(&result.Overview.LastMonth)
	if result.Overview.LastMonth > 0 {
		result.Overview.MonthlyGrowth = float64(result.Overview.ThisMonth-result.Overview.LastMonth) / float64(result.Overview.LastMonth) * 100
	}

	// Average order value (paid orders only)
	var avgResult struct{ Avg float64 }
	h.db.Model(&models.Order{}).
		Select("COALESCE(AVG(total_amount), 0) as avg").
		Where("status IN ? AND total_amount > 0", paidStatuses).
		Scan(&avgResult)
	result.Overview.AvgOrderValue = avgResult.Avg
	result.Overview.Currency = h.cfg.Order.Currency
	if result.Overview.Currency == "" {
		result.Overview.Currency = "CNY"
	}

	// Order trend (last 30 days)
	dateExpr := h.dateGroupExpr("created_at")
	h.db.Model(&models.Order{}).
		Select(fmt.Sprintf("%s as date, COUNT(*) as count", dateExpr)).
		Where("created_at >= ?", thirtyDaysAgo).
		Group(dateExpr).
		Order("date").
		Scan(&result.OrderTrend)

	// Status distribution
	h.db.Model(&models.Order{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Order("count DESC").
		Scan(&result.StatusDistribution)

	// Source distribution
	h.db.Model(&models.Order{}).
		Select("COALESCE(NULLIF(source, ''), 'direct') as source, COUNT(*) as count").
		Group("source").
		Order("count DESC").
		Scan(&result.SourceDistribution)

	// Platform distribution
	h.db.Model(&models.Order{}).
		Select("COALESCE(NULLIF(source_platform, ''), 'Unknown') as platform, COUNT(*) as count").
		Group("source_platform").
		Order("count DESC").
		Limit(15).
		Scan(&result.PlatformDistribution)

	// Country distribution
	h.db.Model(&models.Order{}).
		Select("COALESCE(NULLIF(receiver_country, ''), 'Unknown') as country, COUNT(*) as count").
		Group("receiver_country").
		Order("count DESC").
		Limit(20).
		Scan(&result.CountryDistribution)

	// Amount distribution
	type amountRange struct {
		Range string `json:"range"`
		Count int64  `json:"count"`
	}
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"0-100", 0, 100},
		{"100-500", 100, 500},
		{"500-1000", 500, 1000},
		{"1000-5000", 1000, 5000},
		{"5000+", 5000, -1},
	}
	for _, r := range ranges {
		var count int64
		query := h.db.Model(&models.Order{}).Where("total_amount >= ?", r.min)
		if r.max > 0 {
			query = query.Where("total_amount < ?", r.max)
		}
		query.Count(&count)
		result.AmountDistribution = append(result.AmountDistribution, struct {
			Range string `json:"range"`
			Count int64  `json:"count"`
		}{Range: r.label, Count: count})
	}

	// Top products by sale_count
	h.db.Model(&models.Product{}).
		Select("id, sku, name, sale_count, price, category").
		Where("sale_count > 0").
		Order("sale_count DESC").
		Limit(10).
		Scan(&result.TopProducts)

	response.Success(c, result)
}

// GetRevenueAnalytics returns revenue analytics data
func (h *AnalyticsHandler) GetRevenueAnalytics(c *gin.Context) {
	if h.checkDisabled(c) {
		return
	}
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)
	twelveMonthsAgo := now.AddDate(-1, 0, 0)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonthStart := monthStart.AddDate(0, -1, 0)

	paidStatuses := []models.OrderStatus{
		models.OrderStatusPending,
		models.OrderStatusShipped,
		models.OrderStatusCompleted,
	}

	currency := h.cfg.Order.Currency
	if currency == "" {
		currency = "CNY"
	}

	var result struct {
		// Overview
		Overview struct {
			TotalRevenue    float64 `json:"total_revenue"`
			ThisMonth       float64 `json:"this_month"`
			LastMonth       float64 `json:"last_month"`
			MonthlyGrowth   float64 `json:"monthly_growth"`
			TodayRevenue    float64 `json:"today_revenue"`
			AvgOrderValue   float64 `json:"avg_order_value"`
			TotalPaidOrders int64   `json:"total_paid_orders"`
			Currency        string  `json:"currency"`
		} `json:"overview"`

		// Daily revenue trend (last 30 days)
		DailyTrend []struct {
			Date    string  `json:"date"`
			Revenue float64 `json:"revenue"`
			Count   int64   `json:"count"`
		} `json:"daily_trend"`

		// Monthly revenue trend (last 12 months)
		MonthlyTrend []struct {
			Month   string  `json:"month"`
			Revenue float64 `json:"revenue"`
			Count   int64   `json:"count"`
		} `json:"monthly_trend"`

		// Revenue by source
		RevenueBySource []struct {
			Source  string  `json:"source"`
			Revenue float64 `json:"revenue"`
			Count   int64   `json:"count"`
		} `json:"revenue_by_source"`

		// Revenue by country
		RevenueByCountry []struct {
			Country string  `json:"country"`
			Revenue float64 `json:"revenue"`
			Count   int64   `json:"count"`
		} `json:"revenue_by_country"`
	}

	// Overview - total revenue
	var totalRev struct{ Total float64 }
	h.db.Model(&models.Order{}).
		Select("COALESCE(SUM(total_amount), 0) as total").
		Where("status IN ?", paidStatuses).
		Scan(&totalRev)
	result.Overview.TotalRevenue = totalRev.Total

	// This month
	var thisMonthRev struct{ Total float64 }
	h.db.Model(&models.Order{}).
		Select("COALESCE(SUM(total_amount), 0) as total").
		Where("status IN ? AND created_at >= ?", paidStatuses, monthStart).
		Scan(&thisMonthRev)
	result.Overview.ThisMonth = thisMonthRev.Total

	// Last month
	var lastMonthRev struct{ Total float64 }
	h.db.Model(&models.Order{}).
		Select("COALESCE(SUM(total_amount), 0) as total").
		Where("status IN ? AND created_at >= ? AND created_at < ?", paidStatuses, lastMonthStart, monthStart).
		Scan(&lastMonthRev)
	result.Overview.LastMonth = lastMonthRev.Total

	if result.Overview.LastMonth > 0 {
		result.Overview.MonthlyGrowth = (result.Overview.ThisMonth - result.Overview.LastMonth) / result.Overview.LastMonth * 100
	}

	// Today
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var todayRev struct{ Total float64 }
	h.db.Model(&models.Order{}).
		Select("COALESCE(SUM(total_amount), 0) as total").
		Where("status IN ? AND created_at >= ?", paidStatuses, todayStart).
		Scan(&todayRev)
	result.Overview.TodayRevenue = todayRev.Total

	// Average and count
	var paidStats struct {
		Avg   float64
		Count int64
	}
	h.db.Model(&models.Order{}).
		Select("COALESCE(AVG(total_amount), 0) as avg, COUNT(*) as count").
		Where("status IN ? AND total_amount > 0", paidStatuses).
		Scan(&paidStats)
	result.Overview.AvgOrderValue = paidStats.Avg
	result.Overview.TotalPaidOrders = paidStats.Count
	result.Overview.Currency = currency

	// Daily revenue trend (last 30 days)
	dateExpr := h.dateGroupExpr("created_at")
	h.db.Model(&models.Order{}).
		Select(fmt.Sprintf("%s as date, COALESCE(SUM(total_amount), 0) as revenue, COUNT(*) as count", dateExpr)).
		Where("status IN ? AND created_at >= ?", paidStatuses, thirtyDaysAgo).
		Group(dateExpr).
		Order("date").
		Scan(&result.DailyTrend)

	// Monthly revenue trend (last 12 months)
	monthExpr := h.monthGroupExpr("created_at")
	h.db.Model(&models.Order{}).
		Select(fmt.Sprintf("%s as month, COALESCE(SUM(total_amount), 0) as revenue, COUNT(*) as count", monthExpr)).
		Where("status IN ? AND created_at >= ?", paidStatuses, twelveMonthsAgo).
		Group(monthExpr).
		Order("month").
		Scan(&result.MonthlyTrend)

	// Revenue by source
	h.db.Model(&models.Order{}).
		Select("COALESCE(NULLIF(source, ''), 'direct') as source, COALESCE(SUM(total_amount), 0) as revenue, COUNT(*) as count").
		Where("status IN ?", paidStatuses).
		Group("source").
		Order("revenue DESC").
		Scan(&result.RevenueBySource)

	// Revenue by country
	h.db.Model(&models.Order{}).
		Select("COALESCE(NULLIF(receiver_country, ''), 'Unknown') as country, COALESCE(SUM(total_amount), 0) as revenue, COUNT(*) as count").
		Where("status IN ?", paidStatuses).
		Group("receiver_country").
		Order("revenue DESC").
		Limit(15).
		Scan(&result.RevenueByCountry)

	response.Success(c, result)
}

// parseDeviceType extracts device type from User-Agent string
func parseDeviceType(ua string) string {
	ua = strings.ToLower(ua)
	if strings.Contains(ua, "ipad") || (strings.Contains(ua, "android") && !strings.Contains(ua, "mobile")) || strings.Contains(ua, "tablet") {
		return "Tablet"
	}
	if strings.Contains(ua, "mobile") || strings.Contains(ua, "iphone") || strings.Contains(ua, "ipod") || strings.Contains(ua, "android") {
		return "Mobile"
	}
	return "Desktop"
}

// parseOS extracts OS from User-Agent string
func parseOS(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") || strings.Contains(ua, "ipod"):
		return "iOS"
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "windows"):
		return "Windows"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os"):
		return "macOS"
	case strings.Contains(ua, "linux"):
		return "Linux"
	default:
		return "Other"
	}
}

// GetPageViewAnalytics returns page view analytics data
func (h *AnalyticsHandler) GetPageViewAnalytics(c *gin.Context) {
	if h.checkDisabled(c) {
		return
	}
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastMonthStart := monthStart.AddDate(0, -1, 0)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var result struct {
		Overview struct {
			Total         int64   `json:"total"`
			Today         int64   `json:"today"`
			ThisMonth     int64   `json:"this_month"`
			LastMonth     int64   `json:"last_month"`
			MonthlyGrowth float64 `json:"monthly_growth"`
		} `json:"overview"`

		DailyTrend []struct {
			Date  string `json:"date"`
			Count int64  `json:"count"`
		} `json:"daily_trend"`

		RefererDistribution []struct {
			Referer string `json:"referer"`
			Count   int64  `json:"count"`
		} `json:"referer_distribution"`

		DeviceDistribution []struct {
			Name  string `json:"name"`
			Count int64  `json:"count"`
		} `json:"device_distribution"`

		OSDistribution []struct {
			Name  string `json:"name"`
			Count int64  `json:"count"`
		} `json:"os_distribution"`
	}

	// Overview
	h.db.Model(&models.PageView{}).Count(&result.Overview.Total)
	h.db.Model(&models.PageView{}).Where("created_at >= ?", todayStart).Count(&result.Overview.Today)
	h.db.Model(&models.PageView{}).Where("created_at >= ?", monthStart).Count(&result.Overview.ThisMonth)
	h.db.Model(&models.PageView{}).Where("created_at >= ? AND created_at < ?", lastMonthStart, monthStart).Count(&result.Overview.LastMonth)
	if result.Overview.LastMonth > 0 {
		result.Overview.MonthlyGrowth = float64(result.Overview.ThisMonth-result.Overview.LastMonth) / float64(result.Overview.LastMonth) * 100
	}

	// Daily trend (last 30 days)
	dateExpr := h.dateGroupExpr("created_at")
	h.db.Model(&models.PageView{}).
		Select(fmt.Sprintf("%s as date, COUNT(*) as count", dateExpr)).
		Where("created_at >= ?", thirtyDaysAgo).
		Group(dateExpr).
		Order("date").
		Scan(&result.DailyTrend)

	// Referer distribution
	h.db.Model(&models.PageView{}).
		Select("COALESCE(NULLIF(referer, ''), 'Direct') as referer, COUNT(*) as count").
		Group("referer").
		Order("count DESC").
		Limit(20).
		Scan(&result.RefererDistribution)

	// Device & OS distribution from user_agent
	var views []struct {
		UserAgent string
	}
	h.db.Model(&models.PageView{}).
		Select("user_agent").
		Where("user_agent != ''").
		Find(&views)

	deviceCounts := map[string]int64{}
	osCounts := map[string]int64{}
	for _, v := range views {
		device := parseDeviceType(v.UserAgent)
		os := parseOS(v.UserAgent)
		deviceCounts[device]++
		osCounts[os]++
	}

	for name, count := range deviceCounts {
		result.DeviceDistribution = append(result.DeviceDistribution, struct {
			Name  string `json:"name"`
			Count int64  `json:"count"`
		}{Name: name, Count: count})
	}
	for name, count := range osCounts {
		result.OSDistribution = append(result.OSDistribution, struct {
			Name  string `json:"name"`
			Count int64  `json:"count"`
		}{Name: name, Count: count})
	}

	sort.Slice(result.DeviceDistribution, func(i, j int) bool {
		return result.DeviceDistribution[i].Count > result.DeviceDistribution[j].Count
	})
	sort.Slice(result.OSDistribution, func(i, j int) bool {
		return result.OSDistribution[i].Count > result.OSDistribution[j].Count
	})

	response.Success(c, result)
}

// GetDeviceAnalytics returns device and OS distribution from login user-agents
func (h *AnalyticsHandler) GetDeviceAnalytics(c *gin.Context) {
	if h.checkDisabled(c) {
		return
	}
	var logs []struct {
		UserAgent string
	}
	h.db.Model(&models.OperationLog{}).
		Select("user_agent").
		Where("action = ? AND user_agent != ''", "login").
		Find(&logs)

	deviceCounts := map[string]int64{}
	osCounts := map[string]int64{}

	for _, log := range logs {
		device := parseDeviceType(log.UserAgent)
		os := parseOS(log.UserAgent)
		deviceCounts[device]++
		osCounts[os]++
	}

	type distItem struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}

	var result struct {
		DeviceDistribution []distItem `json:"device_distribution"`
		OSDistribution     []distItem `json:"os_distribution"`
		Total              int64      `json:"total"`
	}

	result.Total = int64(len(logs))

	for name, count := range deviceCounts {
		result.DeviceDistribution = append(result.DeviceDistribution, distItem{Name: name, Count: count})
	}
	for name, count := range osCounts {
		result.OSDistribution = append(result.OSDistribution, distItem{Name: name, Count: count})
	}

	sortDist := func(items []distItem) {
		for i := 0; i < len(items); i++ {
			for j := i + 1; j < len(items); j++ {
				if items[j].Count > items[i].Count {
					items[i], items[j] = items[j], items[i]
				}
			}
		}
	}
	sortDist(result.DeviceDistribution)
	sortDist(result.OSDistribution)

	response.Success(c, result)
}
