package service

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"auralogic/internal/models"
	"gorm.io/gorm"
)

func createPaymentMethodVersionSnapshot(
	db *gorm.DB,
	method *models.PaymentMethod,
	coordinates pluginHostMarketCoordinates,
	importedBy *uint,
) (*models.PaymentMethodVersion, error) {
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "payment method database is unavailable"}
	}
	if method == nil || method.ID == 0 {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "payment method is required"}
	}

	now := time.Now().UTC()
	record := &models.PaymentMethodVersion{
		PaymentMethodID:       method.ID,
		Version:               pluginHostMarketNormalizeVersion(strings.TrimSpace(method.Version)),
		Type:                  method.Type,
		NameSnapshot:          strings.TrimSpace(method.Name),
		DescriptionSnapshot:   strings.TrimSpace(method.Description),
		Icon:                  strings.TrimSpace(method.Icon),
		ScriptSnapshot:        method.Script,
		ConfigSnapshot:        method.Config,
		Manifest:              method.Manifest,
		PackageName:           strings.TrimSpace(method.PackageName),
		PackageEntry:          strings.TrimSpace(method.PackageEntry),
		PackageChecksum:       strings.TrimSpace(method.PackageChecksum),
		PollInterval:          method.PollInterval,
		Enabled:               method.Enabled,
		MarketSourceID:        strings.TrimSpace(coordinates.SourceID),
		MarketArtifactKind:    strings.TrimSpace(coordinates.Kind),
		MarketArtifactName:    strings.TrimSpace(coordinates.Name),
		MarketArtifactVersion: normalizeOptionalPluginHostMarketVersion(coordinates.Version),
		ImportedBy:            importedBy,
		IsActive:              true,
		ActivatedAt:           &now,
	}

	if err := db.Model(&models.PaymentMethodVersion{}).
		Where("payment_method_id = ?", method.ID).
		Update("is_active", false).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "deactivate payment method versions failed"}
	}
	if err := db.Create(record).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "create payment method version failed"}
	}
	return record, nil
}

func activatePaymentMethodVersion(
	db *gorm.DB,
	methodID uint,
	targetVersion *models.PaymentMethodVersion,
) (*models.PaymentMethod, *models.PaymentMethodVersion, error) {
	if db == nil {
		return nil, nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "payment method database is unavailable"}
	}
	if methodID == 0 {
		return nil, nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "payment method id is required"}
	}
	if targetVersion == nil || targetVersion.ID == 0 {
		return nil, nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "payment method version is required"}
	}

	if err := ensureMarketPaymentMethodNameAvailable(db, strings.TrimSpace(targetVersion.NameSnapshot), methodID); err != nil {
		return nil, nil, err
	}

	now := time.Now().UTC()
	updates := map[string]interface{}{
		"name":             strings.TrimSpace(targetVersion.NameSnapshot),
		"description":      strings.TrimSpace(targetVersion.DescriptionSnapshot),
		"type":             targetVersion.Type,
		"enabled":          targetVersion.Enabled,
		"script":           targetVersion.ScriptSnapshot,
		"config":           targetVersion.ConfigSnapshot,
		"icon":             strings.TrimSpace(targetVersion.Icon),
		"version":          pluginHostMarketNormalizeVersion(strings.TrimSpace(targetVersion.Version)),
		"package_name":     strings.TrimSpace(targetVersion.PackageName),
		"package_entry":    strings.TrimSpace(targetVersion.PackageEntry),
		"package_checksum": strings.TrimSpace(targetVersion.PackageChecksum),
		"manifest":         targetVersion.Manifest,
		"poll_interval":    targetVersion.PollInterval,
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.PaymentMethod{}).Where("id = ?", methodID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.PaymentMethodVersion{}).
			Where("payment_method_id = ?", methodID).
			Update("is_active", false).Error; err != nil {
			return err
		}
		return tx.Model(&models.PaymentMethodVersion{}).
			Where("id = ?", targetVersion.ID).
			Updates(map[string]interface{}{
				"is_active":    true,
				"activated_at": now,
			}).Error
	}); err != nil {
		return nil, nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "activate payment method version failed"}
	}

	var method models.PaymentMethod
	if err := db.First(&method, methodID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "payment method not found"}
		}
		return nil, nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "load payment method failed"}
	}

	var refreshed models.PaymentMethodVersion
	if err := db.First(&refreshed, targetVersion.ID).Error; err != nil {
		return &method, nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "load payment method version failed"}
	}

	return &method, &refreshed, nil
}

func buildPaymentMethodMarketSummary(method *models.PaymentMethod) map[string]interface{} {
	if method == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":               method.ID,
		"name":             strings.TrimSpace(method.Name),
		"description":      strings.TrimSpace(method.Description),
		"type":             method.Type,
		"enabled":          method.Enabled,
		"version":          pluginHostMarketNormalizeVersion(strings.TrimSpace(method.Version)),
		"icon":             strings.TrimSpace(method.Icon),
		"package_name":     strings.TrimSpace(method.PackageName),
		"package_entry":    strings.TrimSpace(method.PackageEntry),
		"package_checksum": strings.TrimSpace(method.PackageChecksum),
		"poll_interval":    method.PollInterval,
		"updated_at":       method.UpdatedAt,
	}
}

func buildPaymentMethodVersionSummary(version *models.PaymentMethodVersion) map[string]interface{} {
	if version == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":                      version.ID,
		"payment_method_id":       version.PaymentMethodID,
		"version":                 pluginHostMarketNormalizeVersion(strings.TrimSpace(version.Version)),
		"type":                    version.Type,
		"name_snapshot":           strings.TrimSpace(version.NameSnapshot),
		"description_snapshot":    strings.TrimSpace(version.DescriptionSnapshot),
		"icon":                    strings.TrimSpace(version.Icon),
		"package_name":            strings.TrimSpace(version.PackageName),
		"package_entry":           strings.TrimSpace(version.PackageEntry),
		"package_checksum":        strings.TrimSpace(version.PackageChecksum),
		"poll_interval":           version.PollInterval,
		"enabled":                 version.Enabled,
		"market_source_id":        strings.TrimSpace(version.MarketSourceID),
		"market_artifact_kind":    strings.TrimSpace(version.MarketArtifactKind),
		"market_artifact_name":    strings.TrimSpace(version.MarketArtifactName),
		"market_artifact_version": normalizeOptionalPluginHostMarketVersion(version.MarketArtifactVersion),
		"is_active":               version.IsActive,
		"activated_at":            version.ActivatedAt,
		"created_at":              version.CreatedAt,
		"updated_at":              version.UpdatedAt,
	}
}

func normalizeOptionalPluginHostMarketVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return ""
	}
	return pluginHostMarketNormalizeVersion(trimmed)
}
