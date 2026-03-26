package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"auralogic/internal/models"
	"auralogic/internal/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newKnowledgeHandlerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(&models.KnowledgeCategory{}, &models.KnowledgeArticle{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	return db
}

func performKnowledgeDeleteCategory(t *testing.T, handler *KnowledgeHandler, id uint) response.Response {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/knowledge/categories/%d", id), nil)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", id)}}

	handler.DeleteCategory(ctx)

	var resp response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func readErrorKey(t *testing.T, data interface{}) string {
	t.Helper()

	payload, ok := data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected response data object, got %#v", data)
	}
	key, _ := payload["error_key"].(string)
	return key
}

func TestDeleteKnowledgeCategoryWithArticlesReturnsBizError(t *testing.T) {
	db := newKnowledgeHandlerTestDB(t)
	handler := NewKnowledgeHandler(db, nil)

	category := models.KnowledgeCategory{Name: "Category with articles"}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	article := models.KnowledgeArticle{CategoryID: &category.ID, Title: "Article"}
	if err := db.Create(&article).Error; err != nil {
		t.Fatalf("create article: %v", err)
	}

	resp := performKnowledgeDeleteCategory(t, handler, category.ID)
	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := readErrorKey(t, resp.Data); key != "knowledge.categoryHasArticles" {
		t.Fatalf("expected error key knowledge.categoryHasArticles, got %q", key)
	}
}

func TestDeleteKnowledgeCategoryWithChildrenReturnsBizError(t *testing.T) {
	db := newKnowledgeHandlerTestDB(t)
	handler := NewKnowledgeHandler(db, nil)

	parent := models.KnowledgeCategory{Name: "Parent"}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent category: %v", err)
	}
	child := models.KnowledgeCategory{Name: "Child", ParentID: &parent.ID}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child category: %v", err)
	}

	resp := performKnowledgeDeleteCategory(t, handler, parent.ID)
	if resp.Code != response.CodeBusinessError {
		t.Fatalf("expected business error code, got %d", resp.Code)
	}
	if key := readErrorKey(t, resp.Data); key != "knowledge.categoryHasSubcategories" {
		t.Fatalf("expected error key knowledge.categoryHasSubcategories, got %q", key)
	}
}
