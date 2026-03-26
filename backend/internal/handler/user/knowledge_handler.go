package user

import (
	"log"
	"strconv"

	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type KnowledgeHandler struct {
	db            *gorm.DB
	pluginManager *service.PluginManagerService
}

func NewKnowledgeHandler(db *gorm.DB, pluginManager *service.PluginManagerService) *KnowledgeHandler {
	return &KnowledgeHandler{
		db:            db,
		pluginManager: pluginManager,
	}
}

func buildKnowledgeArticleHookPayload(article *models.KnowledgeArticle) map[string]interface{} {
	if article == nil {
		return map[string]interface{}{}
	}

	payload := map[string]interface{}{
		"article_id":  article.ID,
		"category_id": article.CategoryID,
		"title":       article.Title,
		"content":     article.Content,
		"sort_order":  article.SortOrder,
		"created_at":  article.CreatedAt,
		"updated_at":  article.UpdatedAt,
	}
	if article.Category != nil {
		payload["category_name"] = article.Category.Name
	}
	return payload
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
	page, limit := response.GetPagination(c)
	categoryID := c.Query("category_id")
	search := c.Query("search")

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
	userID := middleware.MustGetUserID(c)
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
	if h.pluginManager != nil {
		payload := buildKnowledgeArticleHookPayload(&article)
		payload["user_id"] = userID
		payload["source"] = "user_api"
		go func(execCtx *service.ExecutionContext, hookPayload map[string]interface{}, articleID uint, uid uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "knowledge.article.view.after",
				Payload: hookPayload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("knowledge.article.view.after hook execution failed: user=%d article=%d err=%v", uid, articleID, hookErr)
			}
		}(cloneUserHookExecutionContext(buildUserHookExecutionContext(c, userID, map[string]string{
			"hook_resource": "knowledge_article",
			"hook_source":   "user_api",
			"hook_action":   "view",
			"article_id":    strconv.FormatUint(id, 10),
		})), payload, article.ID, userID)
	}
	response.Success(c, article)
}
