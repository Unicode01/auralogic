package service

import (
	"testing"

	"auralogic/internal/models"
)

func TestApplyMarketingAudienceQueryBuildsNestedConditions(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{})

	users := []models.User{
		{
			UUID:            "marketing-audience-user-1",
			Email:           "one@example.com",
			Name:            "User One",
			Role:            "user",
			IsActive:        true,
			PasswordHash:    "hash",
			Country:         "CN",
			TotalSpentMinor: 1500,
			TotalOrderCount: 0,
		},
		{
			UUID:            "marketing-audience-user-2",
			Email:           "two@example.com",
			Name:            "User Two",
			Role:            "user",
			IsActive:        true,
			PasswordHash:    "hash",
			Country:         "CN",
			TotalSpentMinor: 500,
			TotalOrderCount: 2,
		},
		{
			UUID:            "marketing-audience-user-3",
			Email:           "three@example.com",
			Name:            "User Three",
			Role:            "user",
			IsActive:        true,
			PasswordHash:    "hash",
			Country:         "US",
			TotalSpentMinor: 2500,
			TotalOrderCount: 3,
		},
		{
			UUID:            "marketing-audience-user-4",
			Email:           "four@example.com",
			Name:            "User Four",
			Role:            "user",
			IsActive:        true,
			PasswordHash:    "hash",
			Country:         "CN",
			TotalSpentMinor: 200,
			TotalOrderCount: 0,
		},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("create user %d failed: %v", i+1, err)
		}
	}

	query := &MarketingAudienceNode{
		Type:       MarketingAudienceNodeTypeGroup,
		Combinator: MarketingAudienceCombinatorAnd,
		Rules: []MarketingAudienceNode{
			{
				Type:     MarketingAudienceNodeTypeCondition,
				Field:    "country",
				Operator: MarketingAudienceOperatorEq,
				Value:    "CN",
			},
			{
				Type:       MarketingAudienceNodeTypeGroup,
				Combinator: MarketingAudienceCombinatorOr,
				Rules: []MarketingAudienceNode{
					{
						Type:     MarketingAudienceNodeTypeCondition,
						Field:    "total_spent_minor",
						Operator: MarketingAudienceOperatorGte,
						Value:    1000,
					},
					{
						Type:     MarketingAudienceNodeTypeCondition,
						Field:    "total_order_count",
						Operator: MarketingAudienceOperatorGte,
						Value:    1,
					},
				},
			},
		},
	}

	scoped, err := ApplyMarketingAudienceQuery(
		db.Model(&models.User{}).Where("role = ?", "user"),
		query,
	)
	if err != nil {
		t.Fatalf("apply audience query failed: %v", err)
	}

	var matchedIDs []uint
	if err := scoped.Order("id ASC").Pluck("id", &matchedIDs).Error; err != nil {
		t.Fatalf("query matched ids failed: %v", err)
	}

	if len(matchedIDs) != 2 {
		t.Fatalf("expected 2 matched users, got %d: %#v", len(matchedIDs), matchedIDs)
	}
	if matchedIDs[0] != users[0].ID || matchedIDs[1] != users[1].ID {
		t.Fatalf("unexpected matched ids: %#v", matchedIDs)
	}
}

func TestApplyMarketingAudienceQuerySupportsIsEmpty(t *testing.T) {
	db := openConcurrentServiceTestDB(t, &models.User{})

	emptyPhone := "   "
	phone := "13800138000"
	users := []models.User{
		{
			UUID:         "marketing-audience-empty-1",
			Email:        "empty1@example.com",
			Name:         "Empty One",
			Role:         "user",
			IsActive:     true,
			PasswordHash: "hash",
		},
		{
			UUID:         "marketing-audience-empty-2",
			Email:        "empty2@example.com",
			Name:         "Empty Two",
			Role:         "user",
			IsActive:     true,
			PasswordHash: "hash",
			Phone:        &emptyPhone,
		},
		{
			UUID:         "marketing-audience-empty-3",
			Email:        "empty3@example.com",
			Name:         "Empty Three",
			Role:         "user",
			IsActive:     true,
			PasswordHash: "hash",
			Phone:        &phone,
		},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("create user %d failed: %v", i+1, err)
		}
	}

	query := &MarketingAudienceNode{
		Type:     MarketingAudienceNodeTypeCondition,
		Field:    "phone",
		Operator: MarketingAudienceOperatorIsEmpty,
	}

	scoped, err := ApplyMarketingAudienceQuery(
		db.Model(&models.User{}).Where("role = ?", "user"),
		query,
	)
	if err != nil {
		t.Fatalf("apply audience query failed: %v", err)
	}

	var matchedIDs []uint
	if err := scoped.Order("id ASC").Pluck("id", &matchedIDs).Error; err != nil {
		t.Fatalf("query matched ids failed: %v", err)
	}

	if len(matchedIDs) != 2 {
		t.Fatalf("expected 2 matched users, got %d", len(matchedIDs))
	}
	if matchedIDs[0] != users[0].ID || matchedIDs[1] != users[1].ID {
		t.Fatalf("unexpected matched ids: %#v", matchedIDs)
	}
}
