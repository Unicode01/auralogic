package service

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"auralogic/internal/models"
	"gorm.io/gorm"
)

const pluginHostMarketTaskIDPrefix = "market-install-"

type pluginHostMarketCoordinates struct {
	SourceID string
	Kind     string
	Name     string
	Version  string
}

func newPluginHostMarketCoordinates(sourceID string, kind string, name string, version string) pluginHostMarketCoordinates {
	return pluginHostMarketCoordinates{
		SourceID: strings.ToLower(strings.TrimSpace(sourceID)),
		Kind:     normalizePluginMarketArtifactKind(kind),
		Name:     strings.TrimSpace(name),
		Version:  strings.TrimSpace(version),
	}
}

func (c pluginHostMarketCoordinates) Empty() bool {
	return strings.TrimSpace(c.SourceID) == "" &&
		strings.TrimSpace(c.Kind) == "" &&
		strings.TrimSpace(c.Name) == "" &&
		strings.TrimSpace(c.Version) == ""
}

func (c pluginHostMarketCoordinates) Map() map[string]interface{} {
	return map[string]interface{}{
		"source_id": c.SourceID,
		"kind":      c.Kind,
		"name":      c.Name,
		"version":   c.Version,
	}
}

func pluginHostMarketCoordinatesFromVersion(version *models.PluginVersion) pluginHostMarketCoordinates {
	if version == nil {
		return pluginHostMarketCoordinates{}
	}
	return newPluginHostMarketCoordinates(
		version.MarketSourceID,
		version.MarketArtifactKind,
		version.MarketArtifactName,
		pluginHostMarketFirstNonEmpty(version.MarketArtifactVersion, version.Version),
	)
}

func pluginHostMarketCoordinatesFromDeployment(record *models.PluginDeployment) pluginHostMarketCoordinates {
	if record == nil {
		return pluginHostMarketCoordinates{}
	}
	coords := newPluginHostMarketCoordinates(
		record.MarketSourceID,
		record.MarketArtifactKind,
		record.MarketArtifactName,
		record.MarketArtifactVersion,
	)
	if !coords.Empty() {
		return coords
	}
	return pluginHostMarketCoordinatesFromVersion(record.TargetVersion)
}

func pluginHostMarketTaskIDForDeploymentID(id uint) string {
	if id == 0 {
		return ""
	}
	return fmt.Sprintf("%s%d", pluginHostMarketTaskIDPrefix, id)
}

func parsePluginHostMarketTaskID(params map[string]interface{}) (uint, error) {
	if id, ok, err := parsePluginHostOptionalUint(params, "id", "deployment_id", "deploymentId"); err != nil {
		return 0, err
	} else if ok {
		return id, nil
	}

	taskID := strings.TrimSpace(parsePluginHostOptionalString(params, "task_id", "taskId"))
	if taskID == "" {
		return 0, fmt.Errorf("task_id/taskId is required")
	}

	lowerTaskID := strings.ToLower(taskID)
	if strings.HasPrefix(lowerTaskID, pluginHostMarketTaskIDPrefix) {
		taskID = strings.TrimSpace(taskID[len(pluginHostMarketTaskIDPrefix):])
	}
	parsed, err := strconv.ParseUint(taskID, 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("task_id/taskId is invalid")
	}
	return uint(parsed), nil
}

func parsePluginHostMarketOptionalKind(params map[string]interface{}) (string, error) {
	rawKind := strings.TrimSpace(parsePluginHostOptionalString(params, "kind"))
	if rawKind == "" {
		return "", nil
	}
	kind := normalizePluginMarketArtifactKind(rawKind)
	if kind == "" {
		return "", fmt.Errorf("kind is invalid")
	}
	return kind, nil
}

func parsePluginHostMarketTemplateTargetKey(kind string, params map[string]interface{}) string {
	switch kind {
	case "email_template":
		targetKey := strings.TrimSpace(parsePluginHostOptionalString(params, "target_key", "targetKey", "email_key", "emailKey"))
		return strings.TrimSuffix(targetKey, ".html")
	case "landing_page_template":
		return strings.TrimSpace(parsePluginHostOptionalString(params, "target_key", "targetKey", "landing_slug", "landingSlug", "slug", "page_key", "pageKey"))
	case "invoice_template":
		return pluginHostInvoiceTemplateTargetKey
	case "auth_branding_template":
		return pluginHostAuthBrandingTemplateTargetKey
	case "page_rule_pack":
		return pluginHostPageRulePackTargetKey
	default:
		return strings.TrimSpace(parsePluginHostOptionalString(params, "target_key", "targetKey"))
	}
}

func parsePluginHostMarketNonNegativeInt(params map[string]interface{}, defaultValue int, max int, keys ...string) (int, error) {
	parsed, ok, err := parsePluginHostOptionalInt64(params, keys...)
	if err != nil {
		return 0, err
	}
	if !ok {
		return defaultValue, nil
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s must be non-negative integer", keys[0])
	}
	if parsed > int64(max) {
		parsed = int64(max)
	}
	return int(parsed), nil
}

func parsePluginHostMarketPositiveInt(params map[string]interface{}, defaultValue int, max int, keys ...string) (int, error) {
	parsed, ok, err := parsePluginHostOptionalInt64(params, keys...)
	if err != nil {
		return 0, err
	}
	if !ok {
		return defaultValue, nil
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be positive integer", keys[0])
	}
	if parsed > int64(max) {
		parsed = int64(max)
	}
	return int(parsed), nil
}

func pluginHostMarketLoadScopedSources(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
) (*models.Plugin, []PluginMarketSource, error) {
	plugin, err := pluginHostLoadPluginForScopedConfig(db, claims)
	if err != nil {
		return nil, nil, err
	}
	sources, err := loadPluginMarketSourcesFromPlugin(plugin)
	if err != nil {
		return nil, nil, err
	}
	return plugin, sources, nil
}

func pluginHostMarketSourceIDs(sources []PluginMarketSource) []string {
	if len(sources) == 0 {
		return []string{}
	}
	ids := make([]string, 0, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(source.SourceID) == "" {
			continue
		}
		ids = append(ids, source.SourceID)
	}
	return ids
}

func pluginHostMarketScopeAllowsCoordinates(sources []PluginMarketSource, coords pluginHostMarketCoordinates) bool {
	if len(sources) == 0 || strings.TrimSpace(coords.SourceID) == "" {
		return false
	}
	for _, source := range sources {
		if !strings.EqualFold(source.SourceID, coords.SourceID) {
			continue
		}
		if strings.TrimSpace(coords.Kind) == "" {
			return true
		}
		return source.AllowsKind(coords.Kind)
	}
	return false
}

func pluginHostMarketResolveScopedSourceID(params map[string]interface{}, sources []PluginMarketSource) (string, error) {
	requestedID := strings.ToLower(strings.TrimSpace(parsePluginHostOptionalString(params, "source_id", "sourceId")))
	if requestedID == "" {
		return "", nil
	}

	for _, source := range sources {
		if !strings.EqualFold(source.SourceID, requestedID) {
			continue
		}
		kind, err := parsePluginHostMarketOptionalKind(params)
		if err != nil {
			return "", err
		}
		if kind != "" && !source.AllowsKind(kind) {
			return "", &PluginHostActionError{Status: http.StatusForbidden, Message: "market artifact kind is not allowed for this source"}
		}
		return source.SourceID, nil
	}

	return "", &PluginHostActionError{Status: http.StatusNotFound, Message: "market source not found"}
}

func pluginHostMarketBuildPagination(offset int, limit int, total int64) map[string]interface{} {
	return map[string]interface{}{
		"offset":   offset,
		"limit":    limit,
		"total":    total,
		"has_more": int64(offset+limit) < total,
	}
}

func pluginHostMarketTaskStatus(record *models.PluginDeployment) string {
	if record == nil {
		return "unknown"
	}
	switch strings.ToLower(strings.TrimSpace(record.Status)) {
	case models.PluginDeploymentStatusPending:
		return "pending"
	case models.PluginDeploymentStatusRunning:
		if strings.EqualFold(record.Operation, models.PluginDeploymentOperationRollback) {
			return "rolling_back"
		}
		return "installing"
	case models.PluginDeploymentStatusSucceeded:
		if strings.EqualFold(record.Operation, models.PluginDeploymentOperationRollback) {
			return "rolled_back"
		}
		return "succeeded"
	case models.PluginDeploymentStatusRolledBack:
		return "rolled_back"
	case models.PluginDeploymentStatusFailed:
		return "failed"
	default:
		return strings.TrimSpace(record.Status)
	}
}

func pluginHostMarketTaskPhase(record *models.PluginDeployment) string {
	if record == nil {
		return "unknown"
	}
	switch pluginHostMarketTaskStatus(record) {
	case "pending":
		return "pending"
	case "installing":
		return "installing"
	case "rolling_back":
		return "rolling_back"
	case "rolled_back":
		return "rolled_back"
	case "failed":
		return "failed"
	case "succeeded":
		return "completed"
	default:
		return "unknown"
	}
}

func pluginHostMarketTaskProgress(record *models.PluginDeployment) int {
	if record == nil {
		return 0
	}
	switch pluginHostMarketTaskStatus(record) {
	case "pending":
		return 0
	case "installing", "rolling_back":
		return 60
	case "succeeded", "failed", "rolled_back":
		return 100
	default:
		return 0
	}
}

func pluginHostMarketBuildTaskSummary(record *models.PluginDeployment, includeResult bool) map[string]interface{} {
	if record == nil {
		return map[string]interface{}{}
	}

	coords := pluginHostMarketCoordinatesFromDeployment(record)
	summary := map[string]interface{}{
		"task_id":       pluginHostMarketTaskIDForDeploymentID(record.ID),
		"deployment_id": record.ID,
		"status":        pluginHostMarketTaskStatus(record),
		"phase":         pluginHostMarketTaskPhase(record),
		"progress":      pluginHostMarketTaskProgress(record),
		"coordinates":   coords.Map(),
		"error":         strings.TrimSpace(record.Error),
		"created_at":    record.CreatedAt,
		"started_at":    record.StartedAt,
		"finished_at":   record.FinishedAt,
	}
	if !includeResult {
		return summary
	}

	if strings.EqualFold(record.Status, models.PluginDeploymentStatusFailed) {
		summary["result"] = nil
		return summary
	}

	result := map[string]interface{}{
		"deployment": pluginHostMarketBuildDeploymentSummary(record),
	}
	if record.TargetVersion != nil {
		result["version"] = pluginHostMarketBuildVersionSummary(record.TargetVersion)
	}
	summary["result"] = result
	return summary
}

func pluginHostMarketApplyTaskStatusFilter(query *gorm.DB, status string) (*gorm.DB, error) {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" {
		return query, nil
	}
	switch normalized {
	case "pending":
		return query.Where("status = ?", models.PluginDeploymentStatusPending), nil
	case "running":
		return query.Where("status = ?", models.PluginDeploymentStatusRunning), nil
	case "installing":
		return query.Where("status = ? AND operation <> ?", models.PluginDeploymentStatusRunning, models.PluginDeploymentOperationRollback), nil
	case "rolling_back":
		return query.Where("status = ? AND operation = ?", models.PluginDeploymentStatusRunning, models.PluginDeploymentOperationRollback), nil
	case "failed":
		return query.Where("status = ?", models.PluginDeploymentStatusFailed), nil
	case "succeeded", "completed":
		return query.Where("status = ? AND operation <> ?", models.PluginDeploymentStatusSucceeded, models.PluginDeploymentOperationRollback), nil
	case "rolled_back":
		return query.Where(
			"(status = ?) OR (status = ? AND operation = ?)",
			models.PluginDeploymentStatusRolledBack,
			models.PluginDeploymentStatusSucceeded,
			models.PluginDeploymentOperationRollback,
		), nil
	default:
		return nil, fmt.Errorf("status is invalid")
	}
}

func executePluginHostMarketInstallTaskGet(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	_, sources, err := pluginHostMarketLoadScopedSources(db, claims)
	if err != nil {
		return nil, err
	}

	taskID, err := parsePluginHostMarketTaskID(params)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	var record models.PluginDeployment
	if err := db.Preload("TargetVersion").First(&record, taskID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "market install task not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query market install task failed"}
	}

	coords := pluginHostMarketCoordinatesFromDeployment(&record)
	if coords.Empty() || !pluginHostMarketScopeAllowsCoordinates(sources, coords) {
		return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "market install task not found"}
	}

	return pluginHostMarketBuildTaskSummary(&record, true), nil
}

func executePluginHostMarketInstallTaskList(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	_, sources, err := pluginHostMarketLoadScopedSources(db, claims)
	if err != nil {
		return nil, err
	}

	offset, err := parsePluginHostMarketNonNegativeInt(params, 0, 100000, "offset")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	limit, err := parsePluginHostMarketPositiveInt(params, 20, 100, "limit", "page_size", "pageSize")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	kind, err := parsePluginHostMarketOptionalKind(params)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	sourceID, err := pluginHostMarketResolveScopedSourceID(params, sources)
	if err != nil {
		return nil, err
	}

	sourceIDs := pluginHostMarketSourceIDs(sources)
	if len(sourceIDs) == 0 {
		return map[string]interface{}{
			"items":      []map[string]interface{}{},
			"pagination": pluginHostMarketBuildPagination(offset, limit, 0),
		}, nil
	}

	query := db.Model(&models.PluginDeployment{}).Where("market_source_id IN ?", sourceIDs)
	if sourceID != "" {
		query = query.Where("market_source_id = ?", sourceID)
	}
	if kind != "" {
		query = query.Where("market_artifact_kind = ?", kind)
	}
	if name := strings.TrimSpace(parsePluginHostOptionalString(params, "name")); name != "" {
		query = query.Where("market_artifact_name = ?", name)
	}
	if version := strings.TrimSpace(parsePluginHostOptionalString(params, "version")); version != "" {
		query = query.Where("market_artifact_version = ?", pluginHostMarketNormalizeVersion(version))
	}
	query, err = pluginHostMarketApplyTaskStatusFilter(query, parsePluginHostOptionalString(params, "status"))
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "count market install tasks failed"}
	}

	var records []models.PluginDeployment
	if err := query.Session(&gorm.Session{}).
		Preload("TargetVersion").
		Order("created_at DESC, id DESC").
		Offset(offset).
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "list market install tasks failed"}
	}

	items := make([]map[string]interface{}, 0, len(records))
	for idx := range records {
		items = append(items, pluginHostMarketBuildTaskSummary(&records[idx], false))
	}

	return map[string]interface{}{
		"items":      items,
		"pagination": pluginHostMarketBuildPagination(offset, limit, total),
	}, nil
}

func executePluginHostMarketInstallHistoryList(
	db *gorm.DB,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	_, sources, err := pluginHostMarketLoadScopedSources(db, claims)
	if err != nil {
		return nil, err
	}

	offset, err := parsePluginHostMarketNonNegativeInt(params, 0, 100000, "offset")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	limit, err := parsePluginHostMarketPositiveInt(params, 20, 100, "limit", "page_size", "pageSize")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	kind, err := parsePluginHostMarketOptionalKind(params)
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	sourceID, err := pluginHostMarketResolveScopedSourceID(params, sources)
	if err != nil {
		return nil, err
	}

	sourceIDs := pluginHostMarketSourceIDs(sources)
	if len(sourceIDs) == 0 {
		return map[string]interface{}{
			"items":      []map[string]interface{}{},
			"pagination": pluginHostMarketBuildPagination(offset, limit, 0),
		}, nil
	}
	if kind == "payment_package" {
		query := db.Model(&models.PaymentMethodVersion{}).Where("market_source_id IN ?", sourceIDs)
		if sourceID != "" {
			query = query.Where("market_source_id = ?", sourceID)
		}
		query = query.Where("market_artifact_kind = ?", kind)
		if name := strings.TrimSpace(parsePluginHostOptionalString(params, "name")); name != "" {
			query = query.Where("market_artifact_name = ?", name)
		}
		if version := strings.TrimSpace(parsePluginHostOptionalString(params, "version")); version != "" {
			query = query.Where("market_artifact_version = ?", pluginHostMarketNormalizeVersion(version))
		}

		var total int64
		if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "count market install history failed"}
		}

		var versions []models.PaymentMethodVersion
		if err := query.Session(&gorm.Session{}).
			Order("created_at DESC, id DESC").
			Offset(offset).
			Limit(limit).
			Find(&versions).Error; err != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "list market install history failed"}
		}

		items := make([]map[string]interface{}, 0, len(versions))
		for idx := range versions {
			version := versions[idx]
			installedAt := version.CreatedAt
			if version.ActivatedAt != nil {
				installedAt = *version.ActivatedAt
			}
			items = append(items, map[string]interface{}{
				"source_id":                 strings.TrimSpace(version.MarketSourceID),
				"kind":                      strings.TrimSpace(version.MarketArtifactKind),
				"name":                      strings.TrimSpace(version.MarketArtifactName),
				"version":                   pluginHostMarketNormalizeVersion(strings.TrimSpace(version.MarketArtifactVersion)),
				"installed_at":              installedAt,
				"installed_target_type":     "payment_method",
				"installed_target_id":       version.PaymentMethodID,
				"payment_method_version_id": version.ID,
				"is_active":                 version.IsActive,
				"update_available":          false,
			})
		}

		return map[string]interface{}{
			"items":      items,
			"pagination": pluginHostMarketBuildPagination(offset, limit, total),
		}, nil
	}
	if kind == "email_template" || kind == "landing_page_template" || kind == "invoice_template" || kind == "auth_branding_template" || kind == "page_rule_pack" {
		query := db.Model(&models.TemplateVersion{}).Where("market_source_id IN ?", sourceIDs)
		if sourceID != "" {
			query = query.Where("market_source_id = ?", sourceID)
		}
		query = query.Where("resource_kind = ? AND market_artifact_kind = ?", kind, kind)
		if name := strings.TrimSpace(parsePluginHostOptionalString(params, "name")); name != "" {
			query = query.Where("market_artifact_name = ?", name)
		}
		if version := strings.TrimSpace(parsePluginHostOptionalString(params, "version")); version != "" {
			query = query.Where("market_artifact_version = ?", pluginHostMarketNormalizeVersion(version))
		}
		if targetKey := parsePluginHostMarketTemplateTargetKey(kind, params); targetKey != "" {
			query = query.Where("target_key = ?", targetKey)
		}

		var total int64
		if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "count market install history failed"}
		}

		var versions []models.TemplateVersion
		if err := query.Session(&gorm.Session{}).
			Order("created_at DESC, id DESC").
			Offset(offset).
			Limit(limit).
			Find(&versions).Error; err != nil {
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "list market install history failed"}
		}

		items := make([]map[string]interface{}, 0, len(versions))
		for idx := range versions {
			version := versions[idx]
			installedAt := version.CreatedAt
			if version.ActivatedAt != nil {
				installedAt = *version.ActivatedAt
			}
			items = append(items, map[string]interface{}{
				"source_id":             strings.TrimSpace(version.MarketSourceID),
				"kind":                  strings.TrimSpace(version.MarketArtifactKind),
				"name":                  strings.TrimSpace(version.MarketArtifactName),
				"version":               pluginHostMarketNormalizeVersion(strings.TrimSpace(version.MarketArtifactVersion)),
				"installed_at":          installedAt,
				"installed_target_type": kind,
				"installed_target_id":   version.ID,
				"installed_target_key":  version.TargetKey,
				"template_version_id":   version.ID,
				"is_active":             version.IsActive,
				"update_available":      false,
			})
		}

		return map[string]interface{}{
			"items":      items,
			"pagination": pluginHostMarketBuildPagination(offset, limit, total),
		}, nil
	}

	query := db.Model(&models.PluginVersion{}).Where("market_source_id IN ?", sourceIDs)
	if sourceID != "" {
		query = query.Where("market_source_id = ?", sourceID)
	}
	if kind != "" {
		query = query.Where("market_artifact_kind = ?", kind)
	}
	if name := strings.TrimSpace(parsePluginHostOptionalString(params, "name")); name != "" {
		query = query.Where("market_artifact_name = ?", name)
	}
	if version := strings.TrimSpace(parsePluginHostOptionalString(params, "version")); version != "" {
		query = query.Where("market_artifact_version = ?", pluginHostMarketNormalizeVersion(version))
	}

	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "count market install history failed"}
	}

	var versions []models.PluginVersion
	if err := query.Session(&gorm.Session{}).
		Order("created_at DESC, id DESC").
		Offset(offset).
		Limit(limit).
		Find(&versions).Error; err != nil {
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "list market install history failed"}
	}

	items := make([]map[string]interface{}, 0, len(versions))
	for idx := range versions {
		version := versions[idx]
		coords := pluginHostMarketCoordinatesFromVersion(&version)
		installedAt := version.CreatedAt
		if version.ActivatedAt != nil {
			installedAt = *version.ActivatedAt
		}
		items = append(items, map[string]interface{}{
			"source_id":             coords.SourceID,
			"kind":                  coords.Kind,
			"name":                  coords.Name,
			"version":               coords.Version,
			"installed_at":          installedAt,
			"installed_target_type": "plugin",
			"installed_target_id":   version.PluginID,
			"plugin_version_id":     version.ID,
			"is_active":             version.IsActive,
			"update_available":      false,
		})
	}

	return map[string]interface{}{
		"items":      items,
		"pagination": pluginHostMarketBuildPagination(offset, limit, total),
	}, nil
}

func executePluginHostMarketInstallRollback(
	runtime *PluginHostRuntime,
	claims *PluginHostAccessClaims,
	params map[string]interface{},
) (map[string]interface{}, error) {
	db := runtime.database()
	if db == nil {
		return nil, &PluginHostActionError{Status: http.StatusServiceUnavailable, Message: "plugin host database is unavailable"}
	}

	plugin, err := pluginHostLoadPluginForScopedConfig(db, claims)
	if err != nil {
		return nil, err
	}
	source, kind, name, version, err := resolvePluginMarketCoordinates(plugin, params)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(version) == "" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "version is required"}
	}
	if kind == "email_template" || kind == "landing_page_template" || kind == "invoice_template" || kind == "auth_branding_template" || kind == "page_rule_pack" {
		targetKey := parsePluginHostMarketTemplateTargetKey(kind, params)
		if targetKey == "" {
			return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: "target_key is required for template rollback"}
		}

		coordinates := newPluginHostMarketCoordinates(source.SourceID, kind, name, pluginHostMarketNormalizeVersion(version))
		var targetVersion models.TemplateVersion
		if err := db.
			Where(
				"resource_kind = ? AND target_key = ? AND market_source_id = ? AND market_artifact_kind = ? AND market_artifact_name = ? AND market_artifact_version = ?",
				kind,
				targetKey,
				coordinates.SourceID,
				coordinates.Kind,
				coordinates.Name,
				coordinates.Version,
			).
			Order("created_at DESC, id DESC").
			First(&targetVersion).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "market rollback target version not found"}
			}
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query market rollback target failed"}
		}

		response := map[string]interface{}{
			"coordinates": coordinates.Map(),
			"source":      source.Summary(),
			"template": map[string]interface{}{
				"resource_kind": kind,
				"target_key":    targetKey,
			},
			"version": buildTemplateVersionSummary(&targetVersion),
		}
		if targetVersion.IsActive {
			response["status"] = "already_active"
			logPluginHostMarketOperation(db, claims, plugin, "plugin_market_rollback", coordinates, params, response)
			return response, nil
		}

		saved, activatedVersion, activateErr := activateTemplateVersion(db, &targetVersion)
		if activateErr != nil {
			response["status"] = "rollback_failed"
			response["error"] = activateErr.Error()
			logPluginHostMarketOperation(db, claims, plugin, "plugin_market_rollback", coordinates, params, response)
			return response, nil
		}

		response["status"] = "rolled_back"
		response["saved"] = saved
		response["version"] = buildTemplateVersionSummary(activatedVersion)
		logPluginHostMarketOperation(db, claims, plugin, "plugin_market_rollback", coordinates, params, response)
		return response, nil
	}
	if kind == "payment_package" {
		coordinates := newPluginHostMarketCoordinates(source.SourceID, kind, name, pluginHostMarketNormalizeVersion(version))

		var targetVersion models.PaymentMethodVersion
		if err := db.
			Where(
				"market_source_id = ? AND market_artifact_kind = ? AND market_artifact_name = ? AND market_artifact_version = ?",
				coordinates.SourceID,
				coordinates.Kind,
				coordinates.Name,
				coordinates.Version,
			).
			Order("created_at DESC, id DESC").
			First(&targetVersion).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "market rollback target version not found"}
			}
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query market rollback target failed"}
		}

		var installedMethod models.PaymentMethod
		if err := db.First(&installedMethod, targetVersion.PaymentMethodID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "local payment method not found for rollback target"}
			}
			return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query local payment method failed"}
		}

		response := map[string]interface{}{
			"coordinates":    coordinates.Map(),
			"source":         source.Summary(),
			"payment_method": buildPaymentMethodMarketSummary(&installedMethod),
			"version":        buildPaymentMethodVersionSummary(&targetVersion),
		}
		if targetVersion.IsActive {
			response["status"] = "already_active"
			logPluginHostMarketOperation(db, claims, plugin, "plugin_market_rollback", coordinates, params, response)
			return response, nil
		}

		activatedMethod, activatedVersion, activateErr := activatePaymentMethodVersion(db, targetVersion.PaymentMethodID, &targetVersion)
		if activateErr != nil {
			response["status"] = "rollback_failed"
			response["error"] = activateErr.Error()
			logPluginHostMarketOperation(db, claims, plugin, "plugin_market_rollback", coordinates, params, response)
			return response, nil
		}

		response["status"] = "rolled_back"
		response["payment_method"] = buildPaymentMethodMarketSummary(activatedMethod)
		response["version"] = buildPaymentMethodVersionSummary(activatedVersion)
		logPluginHostMarketOperation(db, claims, plugin, "plugin_market_rollback", coordinates, params, response)
		return response, nil
	}
	if kind != "plugin_package" {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: fmt.Sprintf("market rollback kind %q is not supported yet", kind)}
	}

	coordinates := newPluginHostMarketCoordinates(source.SourceID, kind, name, pluginHostMarketNormalizeVersion(version))

	var targetVersion models.PluginVersion
	if err := db.
		Where(
			"market_source_id = ? AND market_artifact_kind = ? AND market_artifact_name = ? AND market_artifact_version = ?",
			coordinates.SourceID,
			coordinates.Kind,
			coordinates.Name,
			coordinates.Version,
		).
		Order("created_at DESC, id DESC").
		First(&targetVersion).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "market rollback target version not found"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query market rollback target failed"}
	}

	var installedPlugin models.Plugin
	if err := db.First(&installedPlugin, targetVersion.PluginID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, &PluginHostActionError{Status: http.StatusNotFound, Message: "local plugin not found for rollback target"}
		}
		return nil, &PluginHostActionError{Status: http.StatusInternalServerError, Message: "query local plugin failed"}
	}

	response := map[string]interface{}{
		"coordinates": coordinates.Map(),
		"source":      source.Summary(),
		"plugin":      pluginHostMarketBuildPluginSummary(&installedPlugin),
		"version":     pluginHostMarketBuildVersionSummary(&targetVersion),
	}
	if targetVersion.IsActive {
		response["status"] = "already_active"
		logPluginHostMarketOperation(db, claims, plugin, "plugin_market_rollback", coordinates, params, response)
		return response, nil
	}

	autoStart, err := pluginHostMarketInstallBoolOption(params, false, "auto_start", "autoStart")
	if err != nil {
		return nil, &PluginHostActionError{Status: http.StatusBadRequest, Message: err.Error()}
	}
	operatorUserID := pluginHostMarketOptionalUint(claims.OperatorUserID)
	detail := pluginHostMarketInstallStringOption(params, "note", "detail")
	if detail == "" {
		detail = fmt.Sprintf("market rollback %s/%s@%s", coordinates.SourceID, coordinates.Name, coordinates.Version)
	}

	activatedPlugin, activatedVersion, deployment, activateErr := pluginHostMarketActivatePluginVersion(
		runtime,
		targetVersion.PluginID,
		targetVersion.ID,
		autoStart,
		operatorUserID,
		detail,
		models.PluginDeploymentOperationRollback,
		"market_rollback",
		coordinates,
	)
	if activateErr != nil {
		response["status"] = "rollback_failed"
		response["error"] = activateErr.Error()
		if deployment != nil {
			response["deployment"] = pluginHostMarketBuildDeploymentSummary(deployment)
			response["task_id"] = pluginHostMarketTaskIDForDeploymentID(deployment.ID)
		}
		logPluginHostMarketOperation(db, claims, plugin, "plugin_market_rollback", coordinates, params, response)
		return response, nil
	}

	response["status"] = "rolled_back"
	response["plugin"] = pluginHostMarketBuildPluginSummary(activatedPlugin)
	response["version"] = pluginHostMarketBuildVersionSummary(activatedVersion)
	if deployment != nil {
		response["deployment"] = pluginHostMarketBuildDeploymentSummary(deployment)
		response["task_id"] = pluginHostMarketTaskIDForDeploymentID(deployment.ID)
	}
	logPluginHostMarketOperation(db, claims, plugin, "plugin_market_rollback", coordinates, params, response)
	return response, nil
}
