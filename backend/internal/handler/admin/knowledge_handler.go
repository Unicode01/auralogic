package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"gorm.io/gorm"
)

type KnowledgeHandler struct {
	db *gorm.DB
}

func NewKnowledgeHandler(db *gorm.DB) *KnowledgeHandler {
	return &KnowledgeHandler{db: db}
}

// ==================== Categories ====================

// ListCategories 获取分类树
func (h *KnowledgeHandler) ListCategories(c *gin.Context) {
	var categories []models.KnowledgeCategory
	if err := h.db.Where("parent_id IS NULL").
		Order("sort_order ASC, id ASC").
		Preload("Children", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC, id ASC")
		}).
		Find(&categories).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}
	populateKnowledgeCategoryArticleCounts(h.db, categories)
	response.Success(c, categories)
}

type knowledgeCategoryCountRow struct {
	CategoryID uint  `gorm:"column:category_id"`
	Cnt        int64 `gorm:"column:cnt"`
}

func populateKnowledgeCategoryArticleCounts(db *gorm.DB, categories []models.KnowledgeCategory) {
	var ids []uint
	var collect func(list []models.KnowledgeCategory)
	collect = func(list []models.KnowledgeCategory) {
		for _, cat := range list {
			ids = append(ids, cat.ID)
			if len(cat.Children) > 0 {
				collect(cat.Children)
			}
		}
	}
	collect(categories)
	if len(ids) == 0 {
		return
	}

	var rows []knowledgeCategoryCountRow
	db.Model(&models.KnowledgeArticle{}).
		Select("category_id, COUNT(*) as cnt").
		Where("category_id IN ?", ids).
		Group("category_id").
		Scan(&rows)

	counts := make(map[uint]int64, len(rows))
	for _, r := range rows {
		counts[r.CategoryID] = r.Cnt
	}

	var apply func(cat *models.KnowledgeCategory) int64
	apply = func(cat *models.KnowledgeCategory) int64 {
		direct := counts[cat.ID]
		cat.ArticleCount = direct

		total := direct
		for i := range cat.Children {
			total += apply(&cat.Children[i])
		}
		cat.TotalArticleCount = total
		return total
	}

	for i := range categories {
		apply(&categories[i])
	}
}

// CreateCategory 创建分类
func (h *KnowledgeHandler) CreateCategory(c *gin.Context) {
	var req struct {
		ParentID  *uint  `json:"parent_id"`
		Name      string `json:"name" binding:"required"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	// 验证父分类存在
	if req.ParentID != nil {
		var parent models.KnowledgeCategory
		if err := h.db.First(&parent, *req.ParentID).Error; err != nil {
			response.NotFound(c, "Parent category not found")
			return
		}
	}

	category := models.KnowledgeCategory{
		ParentID:  req.ParentID,
		Name:      req.Name,
		SortOrder: req.SortOrder,
	}
	if err := h.db.Create(&category).Error; err != nil {
		response.InternalError(c, "CreateFailed")
		return
	}
	response.Success(c, category)
}

// UpdateCategory 更新分类
func (h *KnowledgeHandler) UpdateCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	var category models.KnowledgeCategory
	if err := h.db.First(&category, uint(id)).Error; err != nil {
		response.NotFound(c, "Category not found")
		return
	}

	var req struct {
		ParentID  *uint  `json:"parent_id"`
		Name      string `json:"name"`
		SortOrder *int   `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	if req.Name != "" {
		category.Name = req.Name
	}
	if req.SortOrder != nil {
		category.SortOrder = *req.SortOrder
	}
	// 允许设置 parent_id 为 null（移到根级）
	category.ParentID = req.ParentID

	if err := h.db.Save(&category).Error; err != nil {
		response.InternalError(c, "UpdateFailed")
		return
	}
	response.Success(c, category)
}

// DeleteCategory 删除分类
func (h *KnowledgeHandler) DeleteCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	// 检查是否有文章引用
	var articleCount int64
	h.db.Model(&models.KnowledgeArticle{}).Where("category_id = ?", uint(id)).Count(&articleCount)
	if articleCount > 0 {
		response.BadRequest(c, "Cannot delete category with articles")
		return
	}

	// 检查是否有子分类
	var childCount int64
	h.db.Model(&models.KnowledgeCategory{}).Where("parent_id = ?", uint(id)).Count(&childCount)
	if childCount > 0 {
		response.BadRequest(c, "Cannot delete category with subcategories")
		return
	}

	if err := h.db.Delete(&models.KnowledgeCategory{}, uint(id)).Error; err != nil {
		response.InternalError(c, "DeleteFailed")
		return
	}
	response.Success(c, gin.H{"message": "Category deleted"})
}

// ==================== Articles ====================

// ListArticles 文章列表
func (h *KnowledgeHandler) ListArticles(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	categoryID := c.Query("category_id")
	search := c.Query("search")

	if limit > 100 {
		limit = 100
	}

	query := h.db.Model(&models.KnowledgeArticle{})

	if categoryID != "" {
		cid, err := strconv.ParseUint(categoryID, 10, 32)
		if err != nil {
			response.BadRequest(c, "Invalid category_id")
			return
		}

		// Include direct children categories (one level) for a more intuitive "category contains" filter.
		var childIDs []uint
		h.db.Model(&models.KnowledgeCategory{}).
			Where("parent_id = ?", uint(cid)).
			Pluck("id", &childIDs)

		ids := append([]uint{uint(cid)}, childIDs...)
		query = query.Where("category_id IN ?", ids)
	}
	if search != "" {
		query = query.Where("title LIKE ?", "%"+search+"%")
	}

	var total int64
	query.Count(&total)

	var articles []models.KnowledgeArticle
	if err := query.Preload("Category").
		Order("sort_order ASC, id DESC").
		Offset((page - 1) * limit).
		Limit(limit).
		Find(&articles).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	response.Paginated(c, articles, page, limit, total)
}

// CreateArticle 创建文章
func (h *KnowledgeHandler) CreateArticle(c *gin.Context) {
	var req struct {
		CategoryID *uint  `json:"category_id"`
		Title      string `json:"title" binding:"required"`
		Content    string `json:"content"`
		SortOrder  int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	article := models.KnowledgeArticle{
		CategoryID: req.CategoryID,
		Title:      req.Title,
		Content:    req.Content,
		SortOrder:  req.SortOrder,
	}
	if err := h.db.Create(&article).Error; err != nil {
		response.InternalError(c, "CreateFailed")
		return
	}
	response.Success(c, article)
}

// GetArticle 获取文章详情
func (h *KnowledgeHandler) GetArticle(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	var article models.KnowledgeArticle
	if err := h.db.Preload("Category").First(&article, uint(id)).Error; err != nil {
		response.NotFound(c, "Article not found")
		return
	}
	response.Success(c, article)
}

// UpdateArticle 更新文章
func (h *KnowledgeHandler) UpdateArticle(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	var article models.KnowledgeArticle
	if err := h.db.First(&article, uint(id)).Error; err != nil {
		response.NotFound(c, "Article not found")
		return
	}

	var req struct {
		CategoryID *uint  `json:"category_id"`
		Title      string `json:"title"`
		Content    string `json:"content"`
		SortOrder  *int   `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request parameters")
		return
	}

	if req.Title != "" {
		article.Title = req.Title
	}
	if req.Content != "" {
		article.Content = req.Content
	}
	if req.SortOrder != nil {
		article.SortOrder = *req.SortOrder
	}
	article.CategoryID = req.CategoryID

	if err := h.db.Save(&article).Error; err != nil {
		response.InternalError(c, "UpdateFailed")
		return
	}
	response.Success(c, article)
}

// DeleteArticle 删除文章
func (h *KnowledgeHandler) DeleteArticle(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.BadRequest(c, "Invalid ID")
		return
	}

	if err := h.db.Delete(&models.KnowledgeArticle{}, uint(id)).Error; err != nil {
		response.InternalError(c, "DeleteFailed")
		return
	}
	response.Success(c, gin.H{"message": "Article deleted"})
}
