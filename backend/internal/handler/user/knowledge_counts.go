package user

import (
	"auralogic/internal/models"
	"gorm.io/gorm"
)

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

