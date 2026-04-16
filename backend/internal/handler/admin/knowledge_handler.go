package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/logger"
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

type knowledgePackage struct {
	Version    string                     `json:"version"`
	ExportedAt time.Time                  `json:"exported_at"`
	Summary    knowledgePackageSummary    `json:"summary"`
	Categories []knowledgePackageCategory `json:"categories"`
	Articles   []knowledgePackageArticle  `json:"articles"`
}

type knowledgePackageSummary struct {
	CategoryCount int `json:"category_count"`
	ArticleCount  int `json:"article_count"`
}

type knowledgePackageCategory struct {
	ID           uint      `json:"id"`
	ParentID     *uint     `json:"parent_id,omitempty"`
	Name         string    `json:"name"`
	SortOrder    int       `json:"sort_order"`
	PathSegments []string  `json:"path_segments,omitempty"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
}

type knowledgePackageArticle struct {
	ID                   uint      `json:"id"`
	CategoryID           *uint     `json:"category_id,omitempty"`
	CategoryPathSegments []string  `json:"category_path_segments,omitempty"`
	Title                string    `json:"title"`
	Content              string    `json:"content"`
	SortOrder            int       `json:"sort_order"`
	CreatedAt            time.Time `json:"created_at,omitempty"`
	UpdatedAt            time.Time `json:"updated_at,omitempty"`
}

type knowledgeImportResult struct {
	Message              string `json:"message"`
	ConflictMode         string `json:"conflict_mode"`
	CategoryCreatedCount int    `json:"category_created_count"`
	CategoryUpdatedCount int    `json:"category_updated_count"`
	CategorySkippedCount int    `json:"category_skipped_count"`
	ArticleCreatedCount  int    `json:"article_created_count"`
	ArticleUpdatedCount  int    `json:"article_updated_count"`
	ArticleSkippedCount  int    `json:"article_skipped_count"`
}

const knowledgePackageImportMaxBytes = 10 << 20

func parseKnowledgeImportConflictMode(value string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(value))
	if mode == "" {
		return "upsert", nil
	}
	switch mode {
	case "skip", "upsert":
		return mode, nil
	default:
		return "", fmt.Errorf("invalid conflict_mode: %s", value)
	}
}

func knowledgePathKey(segments []string) string {
	encoded, err := json.Marshal(segments)
	if err != nil {
		return strings.Join(segments, "\x1f")
	}
	return string(encoded)
}

func normalizeKnowledgeArticleLookupKey(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}

func buildKnowledgeModelCategorySegments(categories []models.KnowledgeCategory) (map[uint][]string, error) {
	byID := make(map[uint]models.KnowledgeCategory, len(categories))
	for _, category := range categories {
		byID[category.ID] = category
	}

	memo := make(map[uint][]string, len(categories))
	visiting := make(map[uint]bool, len(categories))
	var resolve func(id uint) ([]string, error)
	resolve = func(id uint) ([]string, error) {
		if segments, exists := memo[id]; exists {
			return append([]string(nil), segments...), nil
		}
		category, exists := byID[id]
		if !exists {
			return nil, fmt.Errorf("category %d not found", id)
		}
		if visiting[id] {
			return nil, fmt.Errorf("knowledge category cycle detected at %d", id)
		}
		visiting[id] = true

		segments := make([]string, 0, 4)
		if category.ParentID != nil {
			parentSegments, err := resolve(*category.ParentID)
			if err != nil {
				return nil, err
			}
			segments = append(segments, parentSegments...)
		}
		segments = append(segments, strings.TrimSpace(category.Name))

		visiting[id] = false
		memo[id] = append([]string(nil), segments...)
		return append([]string(nil), segments...), nil
	}

	for _, category := range categories {
		if _, err := resolve(category.ID); err != nil {
			return nil, err
		}
	}

	return memo, nil
}

func buildKnowledgePackageCategorySegments(categories []knowledgePackageCategory) (map[uint][]string, error) {
	byID := make(map[uint]knowledgePackageCategory, len(categories))
	for _, category := range categories {
		if category.ID == 0 {
			return nil, fmt.Errorf("category id is required")
		}
		if _, exists := byID[category.ID]; exists {
			return nil, fmt.Errorf("duplicate category id: %d", category.ID)
		}
		byID[category.ID] = category
	}

	memo := make(map[uint][]string, len(categories))
	visiting := make(map[uint]bool, len(categories))
	var resolve func(id uint) ([]string, error)
	resolve = func(id uint) ([]string, error) {
		if segments, exists := memo[id]; exists {
			return append([]string(nil), segments...), nil
		}
		category, exists := byID[id]
		if !exists {
			return nil, fmt.Errorf("category %d not found in package", id)
		}
		if visiting[id] {
			return nil, fmt.Errorf("knowledge package contains category cycle at %d", id)
		}
		name := strings.TrimSpace(category.Name)
		if name == "" {
			return nil, fmt.Errorf("category %d name is required", id)
		}

		visiting[id] = true
		segments := make([]string, 0, 4)
		if category.ParentID != nil {
			parentSegments, err := resolve(*category.ParentID)
			if err != nil {
				return nil, err
			}
			segments = append(segments, parentSegments...)
		}
		segments = append(segments, name)
		visiting[id] = false

		memo[id] = append([]string(nil), segments...)
		return append([]string(nil), segments...), nil
	}

	for _, category := range categories {
		if _, err := resolve(category.ID); err != nil {
			return nil, err
		}
	}

	return memo, nil
}

func buildKnowledgeCategoryPathIndex(categories []models.KnowledgeCategory, segments map[uint][]string) map[string]models.KnowledgeCategory {
	index := make(map[string]models.KnowledgeCategory, len(categories))
	for _, category := range categories {
		if pathSegments, exists := segments[category.ID]; exists {
			index[knowledgePathKey(pathSegments)] = category
		}
	}
	return index
}

func knowledgeArticleCategoryLookupID(categoryID *uint) uint {
	if categoryID == nil {
		return 0
	}
	return *categoryID
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

// ExportKnowledge 导出知识库迁移包
func (h *KnowledgeHandler) ExportKnowledge(c *gin.Context) {
	var categories []models.KnowledgeCategory
	if err := h.db.Order("sort_order ASC, id ASC").Find(&categories).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	categorySegments, err := buildKnowledgeModelCategorySegments(categories)
	if err != nil {
		response.InternalError(c, "Export failed")
		return
	}

	exportCategories := make([]knowledgePackageCategory, 0, len(categories))
	for _, category := range categories {
		exportCategories = append(exportCategories, knowledgePackageCategory{
			ID:           category.ID,
			ParentID:     category.ParentID,
			Name:         category.Name,
			SortOrder:    category.SortOrder,
			PathSegments: append([]string(nil), categorySegments[category.ID]...),
			CreatedAt:    category.CreatedAt,
			UpdatedAt:    category.UpdatedAt,
		})
	}
	sort.Slice(exportCategories, func(i, j int) bool {
		left := exportCategories[i].PathSegments
		right := exportCategories[j].PathSegments
		if len(left) != len(right) {
			return len(left) < len(right)
		}
		return knowledgePathKey(left) < knowledgePathKey(right)
	})

	var articles []models.KnowledgeArticle
	if err := h.db.Order("sort_order ASC, id ASC").Find(&articles).Error; err != nil {
		response.InternalError(c, "Query failed")
		return
	}

	exportArticles := make([]knowledgePackageArticle, 0, len(articles))
	for _, article := range articles {
		item := knowledgePackageArticle{
			ID:         article.ID,
			CategoryID: article.CategoryID,
			Title:      article.Title,
			Content:    article.Content,
			SortOrder:  article.SortOrder,
			CreatedAt:  article.CreatedAt,
			UpdatedAt:  article.UpdatedAt,
		}
		if article.CategoryID != nil {
			item.CategoryPathSegments = append([]string(nil), categorySegments[*article.CategoryID]...)
		}
		exportArticles = append(exportArticles, item)
	}

	payload := knowledgePackage{
		Version:    "knowledge.v1",
		ExportedAt: time.Now(),
		Summary: knowledgePackageSummary{
			CategoryCount: len(exportCategories),
			ArticleCount:  len(exportArticles),
		},
		Categories: exportCategories,
		Articles:   exportArticles,
	}

	logger.LogOperation(h.db, c, "export", "knowledge", nil, map[string]interface{}{
		"category_count": len(exportCategories),
		"article_count":  len(exportArticles),
		"format":         "json",
		"version":        payload.Version,
	})

	writeJSONAttachment(c, buildAdminJSONFileName("knowledge_package"), payload)
}

// ImportKnowledge 导入知识库迁移包
func (h *KnowledgeHandler) ImportKnowledge(c *gin.Context) {
	conflictMode, err := parseKnowledgeImportConflictMode(c.DefaultPostForm("conflict_mode", c.Query("conflict_mode")))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "Please select a JSON file to upload")
		return
	}
	if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(fileHeader.Filename)), ".json") {
		response.BadRequest(c, "Only .json format is supported")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.InternalError(c, "Failed to open file")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, knowledgePackageImportMaxBytes+1))
	if err != nil {
		response.InternalError(c, "Failed to read file")
		return
	}
	if int64(len(data)) > knowledgePackageImportMaxBytes {
		response.BadRequest(c, fmt.Sprintf("Knowledge package exceeds %d bytes", knowledgePackageImportMaxBytes))
		return
	}

	var pkg knowledgePackage
	if err := json.Unmarshal(data, &pkg); err != nil {
		response.BadRequest(c, "Failed to parse knowledge package")
		return
	}
	if pkg.Version != "" && pkg.Version != "knowledge.v1" {
		response.BadRequest(c, fmt.Sprintf("Unsupported knowledge package version: %s", pkg.Version))
		return
	}
	if len(pkg.Categories) == 0 && len(pkg.Articles) == 0 {
		response.BadRequest(c, "Knowledge package is empty")
		return
	}

	result := knowledgeImportResult{
		ConflictMode: conflictMode,
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		packageSegments, err := buildKnowledgePackageCategorySegments(pkg.Categories)
		if err != nil {
			return err
		}

		orderedCategories := append([]knowledgePackageCategory(nil), pkg.Categories...)
		sort.Slice(orderedCategories, func(i, j int) bool {
			left := packageSegments[orderedCategories[i].ID]
			right := packageSegments[orderedCategories[j].ID]
			if len(left) != len(right) {
				return len(left) < len(right)
			}
			return knowledgePathKey(left) < knowledgePathKey(right)
		})

		seenPackageCategoryPaths := make(map[string]uint, len(orderedCategories))
		for _, category := range orderedCategories {
			pathKey := knowledgePathKey(packageSegments[category.ID])
			if duplicateID, exists := seenPackageCategoryPaths[pathKey]; exists && duplicateID != category.ID {
				return fmt.Errorf("duplicate category path in package: %s", strings.Join(packageSegments[category.ID], " / "))
			}
			seenPackageCategoryPaths[pathKey] = category.ID
		}

		var existingCategories []models.KnowledgeCategory
		if err := tx.Order("sort_order ASC, id ASC").Find(&existingCategories).Error; err != nil {
			return err
		}

		existingSegments, err := buildKnowledgeModelCategorySegments(existingCategories)
		if err != nil {
			return err
		}
		existingCategoryByPath := buildKnowledgeCategoryPathIndex(existingCategories, existingSegments)
		sourceCategoryToActual := make(map[uint]uint, len(orderedCategories))

		for _, item := range orderedCategories {
			pathSegments := packageSegments[item.ID]
			pathKey := knowledgePathKey(pathSegments)
			name := strings.TrimSpace(item.Name)

			var actualParentID *uint
			if item.ParentID != nil {
				mappedParentID, exists := sourceCategoryToActual[*item.ParentID]
				if !exists {
					return fmt.Errorf("category %d parent %d could not be resolved", item.ID, *item.ParentID)
				}
				actualParentID = &mappedParentID
			}

			if existing, exists := existingCategoryByPath[pathKey]; exists {
				sourceCategoryToActual[item.ID] = existing.ID
				if conflictMode == "skip" {
					result.CategorySkippedCount++
					continue
				}

				existing.ParentID = actualParentID
				existing.Name = name
				existing.SortOrder = item.SortOrder
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
				existingCategoryByPath[pathKey] = existing
				sourceCategoryToActual[item.ID] = existing.ID
				result.CategoryUpdatedCount++
				continue
			}

			created := models.KnowledgeCategory{
				ParentID:  actualParentID,
				Name:      name,
				SortOrder: item.SortOrder,
			}
			if err := tx.Create(&created).Error; err != nil {
				return err
			}
			existingCategoryByPath[pathKey] = created
			sourceCategoryToActual[item.ID] = created.ID
			result.CategoryCreatedCount++
		}

		var existingArticles []models.KnowledgeArticle
		if err := tx.Order("sort_order ASC, id ASC").Find(&existingArticles).Error; err != nil {
			return err
		}

		existingArticleByKey := make(map[string]models.KnowledgeArticle, len(existingArticles))
		for _, article := range existingArticles {
			key := fmt.Sprintf(
				"%d|%s",
				knowledgeArticleCategoryLookupID(article.CategoryID),
				normalizeKnowledgeArticleLookupKey(article.Title),
			)
			if _, exists := existingArticleByKey[key]; !exists {
				existingArticleByKey[key] = article
			}
		}

		seenPackageArticleKeys := make(map[string]struct{}, len(pkg.Articles))
		for index, item := range pkg.Articles {
			title := strings.TrimSpace(item.Title)
			if title == "" {
				return fmt.Errorf("article at index %d title is required", index)
			}

			var actualCategoryID *uint
			if item.CategoryID != nil {
				mappedCategoryID, exists := sourceCategoryToActual[*item.CategoryID]
				if !exists {
					return fmt.Errorf("article %q references missing category %d", title, *item.CategoryID)
				}
				actualCategoryID = &mappedCategoryID
			}

			lookupKey := fmt.Sprintf(
				"%d|%s",
				knowledgeArticleCategoryLookupID(actualCategoryID),
				normalizeKnowledgeArticleLookupKey(title),
			)
			if _, exists := seenPackageArticleKeys[lookupKey]; exists {
				return fmt.Errorf("duplicate article in package: %s", title)
			}
			seenPackageArticleKeys[lookupKey] = struct{}{}

			if existing, exists := existingArticleByKey[lookupKey]; exists {
				if conflictMode == "skip" {
					result.ArticleSkippedCount++
					continue
				}

				existing.CategoryID = actualCategoryID
				existing.Title = title
				existing.Content = item.Content
				existing.SortOrder = item.SortOrder
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
				existingArticleByKey[lookupKey] = existing
				result.ArticleUpdatedCount++
				continue
			}

			created := models.KnowledgeArticle{
				CategoryID: actualCategoryID,
				Title:      title,
				Content:    item.Content,
				SortOrder:  item.SortOrder,
			}
			if err := tx.Create(&created).Error; err != nil {
				return err
			}
			existingArticleByKey[lookupKey] = created
			result.ArticleCreatedCount++
		}

		return nil
	}); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result.Message = fmt.Sprintf(
		"Knowledge import completed: categories +%d/~%d, articles +%d/~%d, skipped %d/%d",
		result.CategoryCreatedCount,
		result.CategoryUpdatedCount,
		result.ArticleCreatedCount,
		result.ArticleUpdatedCount,
		result.CategorySkippedCount,
		result.ArticleSkippedCount,
	)

	logger.LogOperation(h.db, c, "import", "knowledge", nil, map[string]interface{}{
		"filename":               strings.TrimSpace(fileHeader.Filename),
		"conflict_mode":          conflictMode,
		"category_created_count": result.CategoryCreatedCount,
		"category_updated_count": result.CategoryUpdatedCount,
		"category_skipped_count": result.CategorySkippedCount,
		"article_created_count":  result.ArticleCreatedCount,
		"article_updated_count":  result.ArticleUpdatedCount,
		"article_skipped_count":  result.ArticleSkippedCount,
		"format":                 "json",
		"version":                "knowledge.v1",
	})

	response.Success(c, result)
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
