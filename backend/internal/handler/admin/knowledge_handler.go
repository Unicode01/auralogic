package admin

import (
	"log"
	"strconv"
	"strings"

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
	return &KnowledgeHandler{db: db, pluginManager: pluginManager}
}

func buildKnowledgeCategoryHookPayload(category *models.KnowledgeCategory) map[string]interface{} {
	if category == nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"category_id": category.ID,
		"parent_id":   category.ParentID,
		"name":        category.Name,
		"sort_order":  category.SortOrder,
		"created_at":  category.CreatedAt,
		"updated_at":  category.UpdatedAt,
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
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("knowledge.category.create.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "knowledge.category.create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "knowledge_category",
				"hook_source":   "admin_api",
			}))
			if hookErr != nil {
				log.Printf("knowledge.category.create.before hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Knowledge category creation rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("knowledge.category.create.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		response.BadRequest(c, "Name is required")
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
	if h.pluginManager != nil {
		afterPayload := buildKnowledgeCategoryHookPayload(&category)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, categoryID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "knowledge.category.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("knowledge.category.create.after hook execution failed: admin=%d category=%d err=%v", adminIDValue, categoryID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "knowledge_category",
			"hook_source":   "admin_api",
			"category_id":   strconv.FormatUint(uint64(category.ID), 10),
		})), afterPayload, category.ID)
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
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("knowledge.category.update.before payload build failed: admin=%d category=%d err=%v", adminIDValue, uint(id), payloadErr)
		} else {
			hookPayload["category_id"] = uint(id)
			hookPayload["current"] = buildKnowledgeCategoryHookPayload(&category)
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "knowledge.category.update.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "knowledge_category",
				"hook_source":   "admin_api",
				"category_id":   strconv.FormatUint(id, 10),
			}))
			if hookErr != nil {
				log.Printf("knowledge.category.update.before hook execution failed: admin=%d category=%d err=%v", adminIDValue, uint(id), hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Knowledge category update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("knowledge.category.update.before payload apply failed, fallback to original request: admin=%d category=%d err=%v", adminIDValue, uint(id), mergeErr)
						req = originalReq
					}
				}
			}
		}
	}
	beforeCategory := category
	req.Name = strings.TrimSpace(req.Name)

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
	if h.pluginManager != nil {
		afterPayload := buildKnowledgeCategoryHookPayload(&category)
		afterPayload["before_parent_id"] = beforeCategory.ParentID
		afterPayload["before_name"] = beforeCategory.Name
		afterPayload["before_sort_order"] = beforeCategory.SortOrder
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "knowledge.category.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("knowledge.category.update.after hook execution failed: admin=%d category=%d err=%v", adminIDValue, uint(id), hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "knowledge_category",
			"hook_source":   "admin_api",
			"category_id":   strconv.FormatUint(id, 10),
		})), afterPayload)
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
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	var category models.KnowledgeCategory
	if err := h.db.First(&category, uint(id)).Error; err != nil {
		response.NotFound(c, "Category not found")
		return
	}

	// 检查是否有文章引用
	var articleCount int64
	h.db.Model(&models.KnowledgeArticle{}).Where("category_id = ?", uint(id)).Count(&articleCount)
	// 检查是否有子分类
	var childCount int64
	h.db.Model(&models.KnowledgeCategory{}).Where("parent_id = ?", uint(id)).Count(&childCount)
	if h.pluginManager != nil {
		hookPayload := buildKnowledgeCategoryHookPayload(&category)
		hookPayload["article_count"] = articleCount
		hookPayload["child_count"] = childCount
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "knowledge.category.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "knowledge_category",
			"hook_source":   "admin_api",
			"category_id":   strconv.FormatUint(id, 10),
		}))
		if hookErr != nil {
			log.Printf("knowledge.category.delete.before hook execution failed: admin=%d category=%d err=%v", adminIDValue, uint(id), hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Knowledge category deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}
	if articleCount > 0 {
		response.BizError(
			c,
			"Cannot delete category with articles",
			"knowledge.categoryHasArticles",
			map[string]interface{}{"article_count": articleCount},
		)
		return
	}
	if childCount > 0 {
		response.BizError(
			c,
			"Cannot delete category with subcategories",
			"knowledge.categoryHasSubcategories",
			map[string]interface{}{"child_count": childCount},
		)
		return
	}

	if err := h.db.Delete(&models.KnowledgeCategory{}, uint(id)).Error; err != nil {
		response.InternalError(c, "DeleteFailed")
		return
	}
	if h.pluginManager != nil {
		afterPayload := buildKnowledgeCategoryHookPayload(&category)
		afterPayload["article_count"] = articleCount
		afterPayload["child_count"] = childCount
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "knowledge.category.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("knowledge.category.delete.after hook execution failed: admin=%d category=%d err=%v", adminIDValue, uint(id), hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "knowledge_category",
			"hook_source":   "admin_api",
			"category_id":   strconv.FormatUint(id, 10),
		})), afterPayload)
	}
	response.Success(c, gin.H{"message": "Category deleted"})
}

// ==================== Articles ====================

// ListArticles 文章列表
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
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("knowledge.article.create.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "knowledge.article.create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "knowledge_article",
				"hook_source":   "admin_api",
			}))
			if hookErr != nil {
				log.Printf("knowledge.article.create.before hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Knowledge article creation rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("knowledge.article.create.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		response.BadRequest(c, "Title is required")
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
	if h.pluginManager != nil {
		afterPayload := buildKnowledgeArticleHookPayload(&article)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, articleID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "knowledge.article.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("knowledge.article.create.after hook execution failed: admin=%d article=%d err=%v", adminIDValue, articleID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "knowledge_article",
			"hook_source":   "admin_api",
			"article_id":    strconv.FormatUint(uint64(article.ID), 10),
		})), afterPayload, article.ID)
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
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("knowledge.article.update.before payload build failed: admin=%d article=%d err=%v", adminIDValue, uint(id), payloadErr)
		} else {
			hookPayload["article_id"] = uint(id)
			hookPayload["current"] = buildKnowledgeArticleHookPayload(&article)
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "admin_api"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "knowledge.article.update.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "knowledge_article",
				"hook_source":   "admin_api",
				"article_id":    strconv.FormatUint(id, 10),
			}))
			if hookErr != nil {
				log.Printf("knowledge.article.update.before hook execution failed: admin=%d article=%d err=%v", adminIDValue, uint(id), hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Knowledge article update rejected by plugin"
					}
					response.BadRequest(c, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("knowledge.article.update.before payload apply failed, fallback to original request: admin=%d article=%d err=%v", adminIDValue, uint(id), mergeErr)
						req = originalReq
					}
				}
			}
		}
	}
	beforeArticle := article
	req.Title = strings.TrimSpace(req.Title)

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
	if h.pluginManager != nil {
		afterPayload := buildKnowledgeArticleHookPayload(&article)
		afterPayload["before_category_id"] = beforeArticle.CategoryID
		afterPayload["before_title"] = beforeArticle.Title
		afterPayload["before_content"] = beforeArticle.Content
		afterPayload["before_sort_order"] = beforeArticle.SortOrder
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "knowledge.article.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("knowledge.article.update.after hook execution failed: admin=%d article=%d err=%v", adminIDValue, uint(id), hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "knowledge_article",
			"hook_source":   "admin_api",
			"article_id":    strconv.FormatUint(id, 10),
		})), afterPayload)
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
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	var article models.KnowledgeArticle
	if err := h.db.Preload("Category").First(&article, uint(id)).Error; err != nil {
		response.NotFound(c, "Article not found")
		return
	}
	if h.pluginManager != nil {
		hookPayload := buildKnowledgeArticleHookPayload(&article)
		hookPayload["admin_id"] = adminIDValue
		hookPayload["source"] = "admin_api"
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "knowledge.article.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "knowledge_article",
			"hook_source":   "admin_api",
			"article_id":    strconv.FormatUint(id, 10),
		}))
		if hookErr != nil {
			log.Printf("knowledge.article.delete.before hook execution failed: admin=%d article=%d err=%v", adminIDValue, uint(id), hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Knowledge article deletion rejected by plugin"
			}
			response.BadRequest(c, reason)
			return
		}
	}

	if err := h.db.Delete(&models.KnowledgeArticle{}, uint(id)).Error; err != nil {
		response.InternalError(c, "DeleteFailed")
		return
	}
	if h.pluginManager != nil {
		afterPayload := buildKnowledgeArticleHookPayload(&article)
		afterPayload["admin_id"] = adminIDValue
		afterPayload["source"] = "admin_api"
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "knowledge.article.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("knowledge.article.delete.after hook execution failed: admin=%d article=%d err=%v", adminIDValue, uint(id), hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "knowledge_article",
			"hook_source":   "admin_api",
			"article_id":    strconv.FormatUint(id, 10),
		})), afterPayload)
	}
	response.Success(c, gin.H{"message": "Article deleted"})
}
