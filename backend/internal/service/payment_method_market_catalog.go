package service

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"auralogic/internal/models"
	"gorm.io/gorm"
)

func ListPaymentMethodMarketSources(db *gorm.DB) (map[string]interface{}, error) {
	sources, err := loadPaymentMethodMarketSources(db)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]interface{}, 0, len(sources))
	for _, source := range sources {
		items = append(items, source.Summary())
	}
	return map[string]interface{}{
		"items": items,
	}, nil
}

func ListPaymentMethodMarketCatalog(db *gorm.DB, params map[string]interface{}) (map[string]interface{}, error) {
	source, err := resolvePaymentMethodMarketSource(db, params)
	if err != nil {
		return nil, err
	}

	catalogParams := clonePaymentMethodMarketParams(params)
	catalogParams["kind"] = "payment_package"

	client := newPluginMarketSourceClient()
	payload, err := client.FetchCatalog(context.Background(), source, catalogParams)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}
	filterPluginMarketCatalogItemsBySource(payload, source)
	payload["source"] = source.Summary()
	return payload, nil
}

func GetPaymentMethodMarketArtifact(db *gorm.DB, params map[string]interface{}) (map[string]interface{}, error) {
	source, err := resolvePaymentMethodMarketSource(db, params)
	if err != nil {
		return nil, err
	}

	name := strings.TrimSpace(parsePluginHostOptionalString(params, "name"))
	if name == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "name is required"}
	}

	client := newPluginMarketSourceClient()
	payload, err := client.FetchArtifact(context.Background(), source, "payment_package", name)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadGateway, Message: err.Error()}
	}
	payload["source"] = source.Summary()
	return payload, nil
}

func loadPaymentMethodMarketSources(db *gorm.DB) ([]PluginMarketSource, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "payment method database is unavailable"}
	}

	var plugins []models.Plugin
	if err := db.Where("enabled = ?", true).Order("id ASC").Find(&plugins).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query payment market sources failed"}
	}

	seen := make(map[string]struct{}, len(plugins))
	sources := make([]PluginMarketSource, 0, len(plugins))
	for idx := range plugins {
		pluginSources, err := loadPluginMarketSourcesFromPlugin(&plugins[idx])
		if err != nil {
			continue
		}
		for _, source := range pluginSources {
			if !source.Enabled || !source.AllowsKind("payment_package") {
				continue
			}
			dedupKey := strings.ToLower(strings.TrimSpace(source.SourceID))
			if dedupKey == "" {
				dedupKey = strings.ToLower(strings.TrimSpace(source.BaseURL))
			}
			if dedupKey == "" {
				continue
			}
			if _, exists := seen[dedupKey]; exists {
				continue
			}
			seen[dedupKey] = struct{}{}
			sources = append(sources, projectPaymentMethodMarketSource(source))
		}
	}

	sort.SliceStable(sources, func(i int, j int) bool {
		left := strings.ToLower(strings.TrimSpace(sources[i].SourceID))
		right := strings.ToLower(strings.TrimSpace(sources[j].SourceID))
		if left == right {
			return strings.ToLower(strings.TrimSpace(sources[i].BaseURL)) < strings.ToLower(strings.TrimSpace(sources[j].BaseURL))
		}
		return left < right
	})

	return sources, nil
}

func resolvePaymentMethodMarketSource(db *gorm.DB, params map[string]interface{}) (PluginMarketSource, error) {
	sources, err := loadPaymentMethodMarketSources(db)
	if err != nil {
		return PluginMarketSource{}, err
	}
	if len(sources) == 0 {
		return PluginMarketSource{}, &PluginHostActionError{Status: http.StatusNotFound, Message: "payment market source not found"}
	}

	if err := validatePaymentMethodMarketKind(params); err != nil {
		return PluginMarketSource{}, err
	}

	requestedID := strings.ToLower(strings.TrimSpace(parsePluginHostOptionalString(params, "source_id", "sourceId")))
	if requestedID == "" {
		if len(sources) == 1 {
			return sources[0], nil
		}
		return PluginMarketSource{}, &PluginHostActionError{Status: http.StatusBadRequest, Message: "source_id/sourceId is required"}
	}

	for _, source := range sources {
		if strings.EqualFold(source.SourceID, requestedID) {
			return source, nil
		}
	}
	return PluginMarketSource{}, &PluginHostActionError{Status: http.StatusNotFound, Message: "payment market source not found"}
}

func validatePaymentMethodMarketKind(params map[string]interface{}) error {
	if params == nil {
		return nil
	}
	kind := strings.TrimSpace(parsePluginHostOptionalString(params, "kind"))
	if kind == "" {
		return nil
	}
	if normalizePluginMarketArtifactKind(kind) != "payment_package" {
		return &PluginHostActionError{Status: http.StatusBadRequest, Message: "market kind must be payment_package"}
	}
	return nil
}

func projectPaymentMethodMarketSource(source PluginMarketSource) PluginMarketSource {
	projected := source
	projected.AllowedKinds = []string{"payment_package"}
	return projected
}

func clonePaymentMethodMarketParams(params map[string]interface{}) map[string]interface{} {
	if len(params) == 0 {
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(params))
	for key, value := range params {
		cloned[key] = value
	}
	return cloned
}
