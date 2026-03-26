package admin

import (
	"strings"
	"time"

	"auralogic/internal/jsworker"
	"auralogic/internal/models"
	"auralogic/internal/service"
)

func (h *PluginHandler) createPluginDeployment(
	pluginID uint,
	operation string,
	trigger string,
	targetVersionID *uint,
	requestedGeneration uint,
	runtimeSpecHash string,
	autoStart bool,
	requestedBy *uint,
	detail string,
) (*models.PluginDeployment, error) {
	if h == nil || h.db == nil {
		return nil, nil
	}

	record := &models.PluginDeployment{
		PluginID:            pluginID,
		Operation:           strings.TrimSpace(operation),
		Trigger:             strings.TrimSpace(trigger),
		Status:              models.PluginDeploymentStatusPending,
		TargetVersionID:     cloneOptionalUint(targetVersionID),
		RequestedGeneration: requestedGeneration,
		AppliedGeneration:   0,
		RuntimeSpecHash:     strings.TrimSpace(runtimeSpecHash),
		AutoStart:           autoStart,
		RequestedBy:         cloneOptionalUint(requestedBy),
		Detail:              strings.TrimSpace(detail),
	}
	if err := h.db.Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func (h *PluginHandler) markPluginDeploymentRunning(record *models.PluginDeployment, detail string) error {
	if h == nil || h.db == nil || record == nil || record.ID == 0 {
		return nil
	}
	now := time.Now().UTC()
	update := map[string]interface{}{
		"status":     models.PluginDeploymentStatusRunning,
		"started_at": now,
	}
	if trimmed := strings.TrimSpace(detail); trimmed != "" {
		update["detail"] = trimmed
		record.Detail = trimmed
	}
	record.Status = models.PluginDeploymentStatusRunning
	record.StartedAt = &now
	return h.db.Model(&models.PluginDeployment{}).Where("id = ?", record.ID).Updates(update).Error
}

func (h *PluginHandler) markPluginDeploymentSucceeded(
	record *models.PluginDeployment,
	appliedGeneration uint,
	detail string,
) error {
	if h == nil || h.db == nil || record == nil || record.ID == 0 {
		return nil
	}
	now := time.Now().UTC()
	update := map[string]interface{}{
		"status":             models.PluginDeploymentStatusSucceeded,
		"applied_generation": appliedGeneration,
		"finished_at":        now,
		"error":              "",
	}
	if trimmed := strings.TrimSpace(detail); trimmed != "" {
		update["detail"] = trimmed
		record.Detail = trimmed
	}
	record.Status = models.PluginDeploymentStatusSucceeded
	record.AppliedGeneration = appliedGeneration
	record.FinishedAt = &now
	record.Error = ""
	return h.db.Model(&models.PluginDeployment{}).Where("id = ?", record.ID).Updates(update).Error
}

func (h *PluginHandler) markPluginDeploymentFailed(record *models.PluginDeployment, err error) error {
	if h == nil || h.db == nil || record == nil || record.ID == 0 {
		return nil
	}
	now := time.Now().UTC()
	errText := ""
	if err != nil {
		errText = strings.TrimSpace(err.Error())
	}
	record.Status = models.PluginDeploymentStatusFailed
	record.Error = errText
	record.FinishedAt = &now
	return h.db.Model(&models.PluginDeployment{}).Where("id = ?", record.ID).Updates(map[string]interface{}{
		"status":      models.PluginDeploymentStatusFailed,
		"error":       errText,
		"finished_at": now,
	}).Error
}

func (h *PluginHandler) listRecentPluginDeployments(pluginID uint, limit int) []models.PluginDeployment {
	if h == nil || h.db == nil || pluginID == 0 {
		return []models.PluginDeployment{}
	}
	if limit <= 0 {
		limit = 10
	}

	var deployments []models.PluginDeployment
	if err := h.db.Where("plugin_id = ?", pluginID).Order("created_at DESC, id DESC").Limit(limit).Find(&deployments).Error; err != nil {
		return []models.PluginDeployment{}
	}
	return deployments
}

func (h *PluginHandler) getLatestPluginDeployments(pluginIDs []uint) map[uint]models.PluginDeployment {
	out := make(map[uint]models.PluginDeployment, len(pluginIDs))
	if h == nil || h.db == nil || len(pluginIDs) == 0 {
		return out
	}

	var records []models.PluginDeployment
	if err := h.db.Where("plugin_id IN ?", pluginIDs).Order("created_at DESC, id DESC").Find(&records).Error; err != nil {
		return out
	}
	for _, record := range records {
		if _, exists := out[record.PluginID]; exists {
			continue
		}
		out[record.PluginID] = record
	}
	return out
}

func (h *PluginHandler) invalidateJSWorkerProgramCaches(plugins ...*models.Plugin) {
	for _, plugin := range plugins {
		h.invalidateJSWorkerProgramCache(plugin)
	}
}

func (h *PluginHandler) invalidateJSWorkerProgramCache(plugin *models.Plugin) {
	if plugin == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(plugin.Runtime), service.PluginRuntimeJSWorker) {
		return
	}

	if scriptPath, err := service.ResolveJSWorkerScriptPath(plugin.Address, plugin.PackagePath); err == nil {
		jsworker.InvalidateWorkerProgramCachePath(scriptPath)
	}
	if root, err := service.ResolveJSWorkerPackageRoot(plugin.PackagePath); err == nil {
		jsworker.InvalidateWorkerProgramCachePathPrefix(root)
	}
}

func (h *PluginHandler) hotReloadPluginInternal(
	pluginID uint,
	trigger string,
	requestedBy *uint,
	detail string,
) (*models.Plugin, *models.PluginDeployment, error) {
	if h == nil || h.db == nil {
		return nil, nil, nil
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, pluginID).Error; err != nil {
		return nil, nil, err
	}

	targetRuntimeSpecHash := service.ComputePluginRuntimeSpecHash(&plugin)
	if targetRuntimeSpecHash == "" {
		targetRuntimeSpecHash = service.ResolvePluginRuntimeSpecHash(&plugin)
	}
	requestedGeneration := service.ResolveNextPluginGeneration(&plugin)

	deployment, err := h.createPluginDeployment(
		plugin.ID,
		models.PluginDeploymentOperationHotReload,
		trigger,
		nil,
		requestedGeneration,
		targetRuntimeSpecHash,
		false,
		requestedBy,
		detail,
	)
	if err != nil {
		return nil, nil, err
	}
	if err := h.markPluginDeploymentRunning(deployment, detail); err != nil {
		return nil, deployment, err
	}

	if err := h.db.Model(&models.Plugin{}).Where("id = ?", plugin.ID).Updates(map[string]interface{}{
		"desired_generation": requestedGeneration,
		"runtime_spec_hash":  targetRuntimeSpecHash,
	}).Error; err != nil {
		_ = h.markPluginDeploymentFailed(deployment, err)
		return nil, deployment, err
	}

	h.invalidateJSWorkerProgramCache(&plugin)
	if plugin.Enabled {
		if h.pluginManager == nil {
			err = errPluginManagerUnavailable()
			_ = h.markPluginDeploymentFailed(deployment, err)
			return nil, deployment, err
		}
		if err = h.pluginManager.ReloadPlugin(plugin.ID); err != nil {
			_ = h.markPluginDeploymentFailed(deployment, err)
			return nil, deployment, err
		}
	}

	if err = h.db.Model(&models.Plugin{}).Where("id = ?", plugin.ID).Updates(map[string]interface{}{
		"applied_generation": requestedGeneration,
		"runtime_spec_hash":  targetRuntimeSpecHash,
	}).Error; err != nil {
		_ = h.markPluginDeploymentFailed(deployment, err)
		return nil, deployment, err
	}

	if err = h.db.First(&plugin, plugin.ID).Error; err != nil {
		_ = h.markPluginDeploymentFailed(deployment, err)
		return nil, deployment, err
	}
	h.invalidatePublicPluginCaches()
	successDetail := strings.TrimSpace(detail)
	if successDetail == "" {
		successDetail = "hot reload applied"
	}
	if !plugin.Enabled {
		successDetail = successDetail + "; plugin is disabled so runtime cutover was skipped"
	}
	_ = h.markPluginDeploymentSucceeded(deployment, requestedGeneration, successDetail)
	return &plugin, deployment, nil
}

func errPluginManagerUnavailable() error {
	return &pluginDeploymentError{message: "plugin manager is unavailable"}
}

type pluginDeploymentError struct {
	message string
}

func (e *pluginDeploymentError) Error() string {
	if e == nil {
		return "plugin deployment failed"
	}
	return strings.TrimSpace(e.message)
}

func buildActivatedPluginSnapshot(
	base models.Plugin,
	version models.PluginVersion,
	requestedGeneration uint,
	runtimeSpecHash string,
) models.Plugin {
	snapshot := base
	snapshot.Version = normalizeVersion(version.Version)
	snapshot.PackagePath = version.PackagePath
	snapshot.PackageChecksum = version.PackageChecksum
	snapshot.RuntimeSpecHash = strings.TrimSpace(runtimeSpecHash)
	if requestedGeneration > 0 {
		snapshot.DesiredGeneration = requestedGeneration
		snapshot.AppliedGeneration = requestedGeneration
	}
	if strings.TrimSpace(version.Type) != "" {
		snapshot.Type = version.Type
	}
	if strings.TrimSpace(version.Runtime) != "" {
		snapshot.Runtime = version.Runtime
	}
	if strings.TrimSpace(version.Address) != "" {
		snapshot.Address = version.Address
	}
	if strings.TrimSpace(version.ConfigSnapshot) != "" {
		snapshot.Config = version.ConfigSnapshot
	}
	if strings.TrimSpace(version.RuntimeParams) != "" {
		snapshot.RuntimeParams = version.RuntimeParams
	}
	if strings.TrimSpace(version.CapabilitiesSnapshot) != "" {
		snapshot.Capabilities = version.CapabilitiesSnapshot
	}
	if strings.TrimSpace(version.Manifest) != "" {
		snapshot.Manifest = version.Manifest
	}
	return snapshot
}
