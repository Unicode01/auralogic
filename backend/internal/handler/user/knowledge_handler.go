package user

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

// GetCategoryTree 获取分类树
func (h *KnowledgeHandler) GetCategoryTree(c *gin.Context) {
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

// ListArticles 文章列表（分页+搜索+分类筛选）
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

// GetArticle 文章详情
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
