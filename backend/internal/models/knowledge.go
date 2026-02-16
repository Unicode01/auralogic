package models

import (
	"time"

	"gorm.io/gorm"
)

// KnowledgeCategory 知识库分类（树形结构）
type KnowledgeCategory struct {
	ID        uint                `gorm:"primaryKey" json:"id"`
	ParentID  *uint               `gorm:"index" json:"parent_id,omitempty"`
	Name      string              `gorm:"type:varchar(255);not null" json:"name"`
	SortOrder int                 `gorm:"default:0;index" json:"sort_order"`
	Children  []KnowledgeCategory `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	// Derived fields for UI; populated by handlers when listing category trees.
	ArticleCount      int64 `gorm:"-" json:"article_count"`
	TotalArticleCount int64 `gorm:"-" json:"total_article_count"`
	CreatedAt time.Time           `json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
	DeletedAt gorm.DeletedAt      `gorm:"index" json:"-"`
}

func (KnowledgeCategory) TableName() string {
	return "knowledge_categories"
}

// KnowledgeArticle 知识库文章
type KnowledgeArticle struct {
	ID         uint               `gorm:"primaryKey" json:"id"`
	CategoryID *uint              `gorm:"index" json:"category_id,omitempty"`
	Category   *KnowledgeCategory `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Title      string             `gorm:"type:varchar(255);not null" json:"title"`
	Content    string             `gorm:"type:text" json:"content"`
	SortOrder  int                `gorm:"default:0;index" json:"sort_order"`
	CreatedAt  time.Time          `json:"created_at"`
	UpdatedAt  time.Time          `json:"updated_at"`
	DeletedAt  gorm.DeletedAt     `gorm:"index" json:"-"`
}

func (KnowledgeArticle) TableName() string {
	return "knowledge_articles"
}
