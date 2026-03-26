package admin

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"auralogic/internal/models"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func normalizePluginLifecycleHookAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "pause", "stop":
		return "pause"
	case "reload", "hot_reload":
		return "hot_reload"
	default:
		return strings.ToLower(strings.TrimSpace(action))
	}
}

// ListPlugins 列出所有插件
func (h *PluginHandler) ListPlugins(c *gin.Context) {
	var plugins []models.Plugin
	if err := h.db.Order("created_at DESC").Find(&plugins).Error; err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to fetch plugins")
		return
	}

	pluginIDs := make([]uint, 0, len(plugins))
	for _, plugin := range plugins {
		if plugin.ID == 0 {
			continue
		}
		pluginIDs = append(pluginIDs, plugin.ID)
	}
	c.JSON(http.StatusOK, buildPluginResponsesWithLatestDeployments(plugins, h.getLatestPluginDeployments(pluginIDs), h.uploadDir))
}

// CreatePlugin 创建插件
func (h *PluginHandler) CreatePlugin(c *gin.Context) {
	var req models.Plugin
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.TrimSpace(req.Type)
	req.Runtime = strings.TrimSpace(req.Runtime)
	req.Address = strings.TrimSpace(req.Address)
	req.PackagePath = strings.TrimSpace(req.PackagePath)
	if h.pluginManager != nil {
		originalReq := req
		hookPayload, payloadErr := adminHookStructToPayload(req)
		if payloadErr != nil {
			log.Printf("plugin.create.before payload build failed: admin=%d err=%v", adminIDValue, payloadErr)
		} else {
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "manual"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "plugin.create.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "plugin",
				"hook_source":   "admin_api",
			}))
			if hookErr != nil {
				log.Printf("plugin.create.before hook execution failed: admin=%d err=%v", adminIDValue, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Plugin creation rejected by plugin"
					}
					h.respondPluginError(c, http.StatusBadRequest, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("plugin.create.before payload apply failed, fallback to original request: admin=%d err=%v", adminIDValue, mergeErr)
						req = originalReq
					}
				}
			}
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Type = strings.TrimSpace(req.Type)
		req.Runtime = strings.TrimSpace(req.Runtime)
		req.Address = strings.TrimSpace(req.Address)
		req.PackagePath = strings.TrimSpace(req.PackagePath)
	}
	if req.Name == "" || req.Type == "" {
		h.respondPluginError(c, http.StatusBadRequest, "name/type are required")
		return
	}
	runtime, err := h.resolveRuntime(req.Runtime)
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	if err := h.validatePluginTypeAndRuntime(req.Type, runtime); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	if runtime == service.PluginRuntimeGRPC && req.Address == "" {
		h.respondPluginError(c, http.StatusBadRequest, "service address is required for grpc runtime")
		return
	}
	if runtime == service.PluginRuntimeJSWorker && req.Address == "" {
		h.respondPluginError(c, http.StatusBadRequest, "entry script path is required for js_worker runtime")
		return
	}
	req.Runtime = runtime
	if runtime == service.PluginRuntimeJSWorker {
		if req.PackagePath == "" {
			h.respondPluginError(c, http.StatusBadRequest, "plugin package path is required for js_worker runtime")
			return
		}
		normalizedPackagePath, normalizedAddress, normalizeErr := normalizeStoredJSWorkerPathConfig(req.PackagePath, req.Address)
		if normalizeErr != nil {
			h.respondPluginErrorErr(c, http.StatusBadRequest, normalizeErr)
			return
		}
		req.PackagePath = normalizedPackagePath
		req.Address = normalizedAddress
	}
	if req.Version == "" {
		req.Version = "0.0.0"
	}
	req.Config, err = normalizeJSONObjectString(req.Config, "{}")
	if err != nil {
		h.respondPluginError(c, http.StatusBadRequest, "config must be a valid JSON object")
		return
	}
	req.RuntimeParams, err = normalizeJSONObjectString(req.RuntimeParams, "{}")
	if err != nil {
		h.respondPluginError(c, http.StatusBadRequest, "runtime_params must be a valid JSON object")
		return
	}
	req.Capabilities, err = normalizeCapabilitiesJSON(req.Capabilities, "{}")
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	if req.LifecycleStatus == "" {
		req.LifecycleStatus = models.PluginLifecycleInstalled
	}
	if req.Enabled {
		req.LifecycleStatus = models.PluginLifecycleRunning
	}
	req.RuntimeSpecHash = service.ComputePluginRuntimeSpecHash(&req)
	if req.DesiredGeneration < 1 {
		req.DesiredGeneration = 1
	}
	if req.AppliedGeneration < 1 {
		req.AppliedGeneration = 1
	}

	if err := h.db.Create(&req).Error; err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to create plugin")
		return
	}

	if _, err := h.createVersionSnapshot(&req, "manual-create", true); err != nil {
		h.invalidatePublicPluginCaches()
		h.respondPluginError(c, http.StatusInternalServerError, "Plugin created, but failed to create version snapshot")
		return
	}
	h.invalidatePublicPluginCaches()

	// 如果启用，立即加载
	if req.Enabled {
		if err := h.pluginManager.StartPlugin(req.ID); err != nil {
			payload := buildPluginFailurePayload(http.StatusBadGateway, "Plugin created but failed to connect", map[string]interface{}{
				"details": err.Error(),
			})
			payload["plugin"] = buildPluginResponse(req, h.uploadDir)
			c.JSON(http.StatusBadGateway, payload)
			return
		}
	}

	_ = h.db.First(&req, req.ID).Error
	h.logPluginOperation(c, "plugin_create", &req, &req.ID, map[string]interface{}{
		"source": "manual",
	})
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"plugin_id":          req.ID,
			"name":               req.Name,
			"display_name":       req.DisplayName,
			"type":               req.Type,
			"runtime":            req.Runtime,
			"address":            req.Address,
			"package_path":       req.PackagePath,
			"version":            req.Version,
			"enabled":            req.Enabled,
			"lifecycle_status":   req.LifecycleStatus,
			"desired_generation": req.DesiredGeneration,
			"applied_generation": req.AppliedGeneration,
			"admin_id":           adminIDValue,
			"source":             "manual",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, pluginID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "plugin.create.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("plugin.create.after hook execution failed: admin=%d plugin=%d err=%v", adminIDValue, pluginID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin",
			"hook_source":   "admin_api",
		})), afterPayload, req.ID)
	}
	c.JSON(http.StatusCreated, buildPluginResponse(req, h.uploadDir))
}

// GetPlugin 获取插件详情
func (h *PluginHandler) GetPlugin(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, id).Error; err != nil {
		h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
		return
	}
	latest := h.getLatestPluginDeployments([]uint{plugin.ID})
	if deployment, exists := latest[plugin.ID]; exists {
		c.JSON(http.StatusOK, buildPluginResponseWithDeployment(plugin, &deployment, h.uploadDir))
		return
	}
	c.JSON(http.StatusOK, buildPluginResponse(plugin, h.uploadDir))
}

// UpdatePlugin 更新插件
func (h *PluginHandler) UpdatePlugin(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, id).Error; err != nil {
		h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
		return
	}
	originalPlugin := plugin
	originalRuntimeSpecHash := service.ResolvePluginRuntimeSpecHash(&plugin)

	var req updatePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
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
			log.Printf("plugin.update.before payload build failed: admin=%d plugin=%d err=%v", adminIDValue, plugin.ID, payloadErr)
		} else {
			hookPayload["plugin_id"] = plugin.ID
			hookPayload["plugin_name"] = plugin.Name
			hookPayload["plugin_runtime"] = plugin.Runtime
			hookPayload["plugin_type"] = plugin.Type
			hookPayload["admin_id"] = adminIDValue
			hookPayload["source"] = "manual"
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "plugin.update.before",
				Payload: hookPayload,
			}, buildAdminHookExecutionContext(c, adminID, map[string]string{
				"hook_resource": "plugin",
				"hook_source":   "admin_api",
				"plugin_id":     fmt.Sprintf("%d", plugin.ID),
			}))
			if hookErr != nil {
				log.Printf("plugin.update.before hook execution failed: admin=%d plugin=%d err=%v", adminIDValue, plugin.ID, hookErr)
			} else if hookResult != nil {
				if hookResult.Blocked {
					reason := strings.TrimSpace(hookResult.BlockReason)
					if reason == "" {
						reason = "Plugin update rejected by plugin"
					}
					h.respondPluginError(c, http.StatusBadRequest, reason)
					return
				}
				if hookResult.Payload != nil {
					if mergeErr := mergeAdminHookStructPatch(&req, hookResult.Payload); mergeErr != nil {
						log.Printf("plugin.update.before payload apply failed, fallback to original request: admin=%d plugin=%d err=%v", adminIDValue, plugin.ID, mergeErr)
						req = originalReq
					}
				}
			}
		}
	}
	originalRuntime := strings.TrimSpace(plugin.Runtime)
	originalAddress := strings.TrimSpace(plugin.Address)
	originalPackagePath := strings.TrimSpace(plugin.PackagePath)
	runtimeChanged := false
	jsWorkerPathUpdated := false

	if req.DisplayName != nil {
		plugin.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.Description != nil {
		plugin.Description = strings.TrimSpace(*req.Description)
	}
	if req.Type != nil && strings.TrimSpace(*req.Type) != "" {
		plugin.Type = strings.TrimSpace(*req.Type)
	}
	if req.Runtime != nil && strings.TrimSpace(*req.Runtime) != "" {
		runtime, runtimeErr := h.resolveRuntime(*req.Runtime)
		if runtimeErr != nil {
			h.respondPluginErrorErr(c, http.StatusBadRequest, runtimeErr)
			return
		}
		runtimeChanged = runtime != originalRuntime
		plugin.Runtime = runtime
	}
	if req.Address != nil && strings.TrimSpace(*req.Address) != "" {
		trimmedAddress := strings.TrimSpace(*req.Address)
		jsWorkerPathUpdated = jsWorkerPathUpdated || trimmedAddress != originalAddress
		plugin.Address = trimmedAddress
	}
	if req.PackagePath != nil && strings.TrimSpace(*req.PackagePath) != "" {
		trimmedPackagePath := strings.TrimSpace(*req.PackagePath)
		jsWorkerPathUpdated = jsWorkerPathUpdated || trimmedPackagePath != originalPackagePath
		plugin.PackagePath = trimmedPackagePath
	}
	if req.Config != nil && strings.TrimSpace(*req.Config) != "" {
		configJSON, configErr := normalizeJSONObjectString(*req.Config, "{}")
		if configErr != nil {
			h.respondPluginError(c, http.StatusBadRequest, "config must be a valid JSON object")
			return
		}
		plugin.Config = configJSON
	}
	if req.RuntimeParams != nil && strings.TrimSpace(*req.RuntimeParams) != "" {
		runtimeParamsJSON, paramsErr := normalizeJSONObjectString(*req.RuntimeParams, "{}")
		if paramsErr != nil {
			h.respondPluginError(c, http.StatusBadRequest, "runtime_params must be a valid JSON object")
			return
		}
		plugin.RuntimeParams = runtimeParamsJSON
	}
	if req.Capabilities != nil && strings.TrimSpace(*req.Capabilities) != "" {
		capabilitiesJSON, capabilitiesErr := normalizeCapabilitiesJSON(*req.Capabilities, "{}")
		if capabilitiesErr != nil {
			h.respondPluginErrorErr(c, http.StatusBadRequest, capabilitiesErr)
			return
		}
		plugin.Capabilities = capabilitiesJSON
	}
	if req.Version != nil && strings.TrimSpace(*req.Version) != "" {
		plugin.Version = strings.TrimSpace(*req.Version)
	}
	if strings.TrimSpace(plugin.Runtime) == "" {
		runtime, runtimeErr := h.resolveRuntime("")
		if runtimeErr != nil {
			h.respondPluginErrorErr(c, http.StatusBadRequest, runtimeErr)
			return
		}
		plugin.Runtime = runtime
	}
	if err := h.validatePluginTypeAndRuntime(plugin.Type, plugin.Runtime); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	if plugin.Runtime == service.PluginRuntimeGRPC && strings.TrimSpace(plugin.Address) == "" {
		h.respondPluginError(c, http.StatusBadRequest, "service address is required for grpc runtime")
		return
	}
	if plugin.Runtime == service.PluginRuntimeJSWorker && strings.TrimSpace(plugin.Address) == "" {
		h.respondPluginError(c, http.StatusBadRequest, "entry script path is required for js_worker runtime")
		return
	}
	if plugin.Runtime == service.PluginRuntimeJSWorker && (runtimeChanged || jsWorkerPathUpdated) {
		if strings.TrimSpace(plugin.PackagePath) == "" {
			h.respondPluginError(c, http.StatusBadRequest, "plugin package path is required for js_worker runtime")
			return
		}
		normalizedPackagePath, normalizedAddress, normalizeErr := normalizeStoredJSWorkerPathConfig(plugin.PackagePath, plugin.Address)
		if normalizeErr != nil {
			h.respondPluginErrorErr(c, http.StatusBadRequest, normalizeErr)
			return
		}
		plugin.PackagePath = normalizedPackagePath
		plugin.Address = normalizedAddress
	}
	targetRuntimeSpecHash := service.ComputePluginRuntimeSpecHash(&plugin)
	if targetRuntimeSpecHash == "" {
		targetRuntimeSpecHash = originalRuntimeSpecHash
	}
	runtimeSpecChanged := targetRuntimeSpecHash != originalRuntimeSpecHash
	plugin.RuntimeSpecHash = targetRuntimeSpecHash
	if plugin.DesiredGeneration < 1 {
		plugin.DesiredGeneration = 1
	}
	if plugin.AppliedGeneration < 1 {
		plugin.AppliedGeneration = plugin.DesiredGeneration
	}

	enabledUpdated := req.Enabled != nil
	if enabledUpdated {
		plugin.Enabled = *req.Enabled
		if plugin.Enabled {
			plugin.LifecycleStatus = models.PluginLifecycleInstalled
		} else if plugin.LifecycleStatus != models.PluginLifecycleRetired {
			plugin.LifecycleStatus = models.PluginLifecyclePaused
		}
	}

	if err := h.db.Save(&plugin).Error; err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to update plugin")
		return
	}

	if _, err := h.createVersionSnapshot(&plugin, "manual-update", true); err != nil {
		h.invalidatePublicPluginCaches()
		h.respondPluginError(c, http.StatusInternalServerError, "plugin updated, but failed to create version snapshot")
		return
	}
	h.invalidatePublicPluginCaches()
	if runtimeSpecChanged {
		h.invalidateJSWorkerProgramCaches(&originalPlugin, &plugin)
	}

	if enabledUpdated && plugin.Enabled {
		if err := h.pluginManager.StartPlugin(plugin.ID); err != nil {
			payload := buildPluginFailurePayload(http.StatusBadGateway, "Plugin updated but failed to connect", map[string]interface{}{
				"details": err.Error(),
			})
			payload["plugin"] = buildPluginResponse(plugin, h.uploadDir)
			c.JSON(http.StatusBadGateway, payload)
			return
		}
	} else if enabledUpdated && !plugin.Enabled {
		if err := h.pluginManager.PausePlugin(plugin.ID); err != nil {
			h.respondPluginError(c, http.StatusInternalServerError, "plugin updated but failed to pause plugin")
			return
		}
	} else if plugin.Enabled && runtimeSpecChanged {
		if _, _, err := h.hotReloadPluginInternal(plugin.ID, "manual_update", getOptionalUserID(c), "runtime spec changed after manual update"); err != nil {
			payload := buildPluginFailurePayload(http.StatusBadGateway, "Plugin updated but failed to reload", map[string]interface{}{
				"details": err.Error(),
			})
			payload["plugin"] = buildPluginResponse(plugin, h.uploadDir)
			c.JSON(http.StatusBadGateway, payload)
			return
		}
	}

	if err := h.db.First(&plugin, plugin.ID).Error; err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to reload plugin")
		return
	}
	h.logPluginOperation(c, "plugin_update", &plugin, &plugin.ID, map[string]interface{}{
		"enabled_updated": enabledUpdated,
		"source":          "manual",
	})
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"plugin_id":            plugin.ID,
			"name":                 plugin.Name,
			"display_name":         plugin.DisplayName,
			"type":                 plugin.Type,
			"runtime":              plugin.Runtime,
			"version":              plugin.Version,
			"enabled":              plugin.Enabled,
			"lifecycle_status":     plugin.LifecycleStatus,
			"runtime_spec_changed": runtimeSpecChanged,
			"enabled_updated":      enabledUpdated,
			"desired_generation":   plugin.DesiredGeneration,
			"applied_generation":   plugin.AppliedGeneration,
			"admin_id":             adminIDValue,
			"source":               "manual",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, pluginID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "plugin.update.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("plugin.update.after hook execution failed: admin=%d plugin=%d err=%v", adminIDValue, pluginID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin",
			"hook_source":   "admin_api",
			"plugin_id":     fmt.Sprintf("%d", plugin.ID),
		})), afterPayload, plugin.ID)
	}
	c.JSON(http.StatusOK, buildPluginResponse(plugin, h.uploadDir))
}

// DeletePlugin 删除插件
func (h *PluginHandler) DeletePlugin(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to query plugin")
		return
	}

	var versions []models.PluginVersion
	if err := h.db.Where("plugin_id = ?", id).Find(&versions).Error; err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to query plugin versions")
		return
	}
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"plugin_id":        plugin.ID,
			"name":             plugin.Name,
			"display_name":     plugin.DisplayName,
			"type":             plugin.Type,
			"runtime":          plugin.Runtime,
			"version":          plugin.Version,
			"enabled":          plugin.Enabled,
			"lifecycle_status": plugin.LifecycleStatus,
			"version_count":    len(versions),
			"admin_id":         adminIDValue,
			"source":           "manual",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "plugin.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin",
			"hook_source":   "admin_api",
			"plugin_id":     fmt.Sprintf("%d", plugin.ID),
		}))
		if hookErr != nil {
			log.Printf("plugin.delete.before hook execution failed: admin=%d plugin=%d err=%v", adminIDValue, plugin.ID, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Plugin deletion rejected by plugin"
			}
			h.respondPluginError(c, http.StatusBadRequest, reason)
			return
		}
	}

	artifactFiles, artifactDirs := h.collectPluginArtifactTargets(&plugin, versions)

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plugin_id = ?", id).Delete(&models.PluginExecution{}).Error; err != nil {
			return fmt.Errorf("delete plugin executions failed: %w", err)
		}
		if err := tx.Where("plugin_id = ?", id).Delete(&models.PluginDeployment{}).Error; err != nil {
			return fmt.Errorf("delete plugin deployments failed: %w", err)
		}
		if err := tx.Where("plugin_id = ?", id).Delete(&models.PluginVersion{}).Error; err != nil {
			return fmt.Errorf("delete plugin versions failed: %w", err)
		}
		if err := tx.Where("plugin_id = ?", id).Delete(&models.PluginStorageEntry{}).Error; err != nil {
			return fmt.Errorf("delete plugin storage failed: %w", err)
		}
		if err := tx.Where("plugin_id = ?", id).Delete(&models.PluginSecretEntry{}).Error; err != nil {
			return fmt.Errorf("delete plugin secrets failed: %w", err)
		}
		if err := tx.Delete(&models.Plugin{}, id).Error; err != nil {
			return fmt.Errorf("delete plugin failed: %w", err)
		}
		return nil
	}); err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to delete plugin")
		return
	}

	if h.pluginManager != nil {
		h.pluginManager.UnregisterPlugin(id)
		h.pluginManager.RemovePluginWorkspace(id)
		_ = h.pluginManager.RefreshPluginExecutionCatalog()
	}
	h.invalidatePublicPluginCaches()

	cleanupErrors := cleanupPluginArtifactTargets(artifactFiles, artifactDirs)
	h.logPluginOperation(c, "plugin_delete", &plugin, &plugin.ID, map[string]interface{}{
		"artifact_file_targets":   len(artifactFiles),
		"artifact_dir_targets":    len(artifactDirs),
		"artifact_cleanup_errors": cleanupErrors,
	})
	resp := gin.H{
		"message": "Plugin deleted",
	}
	if len(cleanupErrors) > 0 {
		resp["artifact_cleanup_errors"] = cleanupErrors
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"plugin_id":               plugin.ID,
			"name":                    plugin.Name,
			"display_name":            plugin.DisplayName,
			"type":                    plugin.Type,
			"runtime":                 plugin.Runtime,
			"version":                 plugin.Version,
			"artifact_file_targets":   len(artifactFiles),
			"artifact_dir_targets":    len(artifactDirs),
			"artifact_cleanup_errors": cleanupErrors,
			"admin_id":                adminIDValue,
			"source":                  "manual",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, pluginID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "plugin.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("plugin.delete.after hook execution failed: admin=%d plugin=%d err=%v", adminIDValue, pluginID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin",
			"hook_source":   "admin_api",
			"plugin_id":     fmt.Sprintf("%d", plugin.ID),
		})), afterPayload, plugin.ID)
	}
	c.JSON(http.StatusOK, resp)
}

// HandleLifecycleAction 处理插件生命周期动作
func (h *PluginHandler) HandleLifecycleAction(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}

	var req lifecycleActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}

	action := strings.ToLower(strings.TrimSpace(req.Action))
	autoStart := req.AutoStart != nil && *req.AutoStart
	lifecycleAction := action
	var deployment *models.PluginDeployment
	hookAction := normalizePluginLifecycleHookAction(action)
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"plugin_id":  id,
			"action":     hookAction,
			"version_id": req.VersionID,
			"auto_start": autoStart,
			"admin_id":   adminIDValue,
			"source":     "manual",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "plugin.lifecycle." + hookAction + ".before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin",
			"hook_source":   "admin_api",
			"plugin_id":     fmt.Sprintf("%d", id),
			"action":        hookAction,
		}))
		if hookErr != nil {
			log.Printf("plugin.lifecycle.%s.before hook execution failed: admin=%d plugin=%d err=%v", hookAction, adminIDValue, id, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Plugin lifecycle action rejected by plugin"
				}
				h.respondPluginError(c, http.StatusBadRequest, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["version_id"]; exists {
					if parsed := parseIntFromAny(value, 0); parsed > 0 {
						versionID := uint(parsed)
						req.VersionID = &versionID
					}
				}
				if value, exists := hookResult.Payload["auto_start"]; exists {
					autoStartValue := parseBoolFromAny(value, autoStart)
					req.AutoStart = &autoStartValue
					autoStart = autoStartValue
				}
			}
		}
	}

	switch action {
	case "install":
		if req.VersionID != nil && *req.VersionID > 0 {
			if _, _, err := h.activatePluginVersionInternalWithDeploymentContext(
				id,
				*req.VersionID,
				false,
				"lifecycle_install",
				getOptionalUserID(c),
				"install plugin version before lifecycle install",
			); err != nil {
				h.respondPluginErrorErr(c, http.StatusBadGateway, err)
				return
			}
		} else if err := h.pluginManager.InstallPlugin(id); err != nil {
			h.respondPluginErrorErr(c, http.StatusInternalServerError, err)
			return
		}
	case "start":
		if req.VersionID != nil && *req.VersionID > 0 {
			if _, _, err := h.activatePluginVersionInternalWithDeploymentContext(
				id,
				*req.VersionID,
				false,
				"lifecycle_start",
				getOptionalUserID(c),
				"activate plugin version before lifecycle start",
			); err != nil {
				h.respondPluginErrorErr(c, http.StatusBadGateway, err)
				return
			}
		}
		if err := h.pluginManager.StartPlugin(id); err != nil {
			h.respondPluginErrorErr(c, http.StatusBadGateway, err)
			return
		}
	case "pause", "stop":
		lifecycleAction = "pause"
		if err := h.pluginManager.PausePlugin(id); err != nil {
			h.respondPluginErrorErr(c, http.StatusInternalServerError, err)
			return
		}
	case "restart":
		if err := h.pluginManager.RestartPlugin(id); err != nil {
			h.respondPluginErrorErr(c, http.StatusBadGateway, err)
			return
		}
	case "reload", "hot_reload":
		lifecycleAction = "hot_reload"
		_, deploymentResult, err := h.hotReloadPluginInternal(
			id,
			"manual_hot_reload",
			getOptionalUserID(c),
			"requested from admin lifecycle action",
		)
		if err != nil {
			h.respondPluginErrorErr(c, http.StatusBadGateway, err)
			return
		}
		deployment = deploymentResult
	case "retire":
		if err := h.pluginManager.RetirePlugin(id); err != nil {
			h.respondPluginErrorErr(c, http.StatusInternalServerError, err)
			return
		}
	case "resume":
		if req.VersionID != nil && *req.VersionID > 0 {
			if _, _, err := h.activatePluginVersionInternalWithDeploymentContext(
				id,
				*req.VersionID,
				false,
				"lifecycle_resume",
				getOptionalUserID(c),
				"activate plugin version before lifecycle resume",
			); err != nil {
				h.respondPluginErrorErr(c, http.StatusBadGateway, err)
				return
			}
		}
		if autoStart {
			if err := h.pluginManager.StartPlugin(id); err != nil {
				h.respondPluginErrorErr(c, http.StatusBadGateway, err)
				return
			}
		} else {
			if err := h.pluginManager.InstallPlugin(id); err != nil {
				h.respondPluginErrorErr(c, http.StatusInternalServerError, err)
				return
			}
		}
	default:
		h.respondPluginError(c, http.StatusBadRequest, "unsupported lifecycle action")
		return
	}
	h.invalidatePublicPluginCaches()

	var plugin models.Plugin
	if err := h.db.First(&plugin, id).Error; err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to query plugin")
		return
	}
	logDetails := map[string]interface{}{
		"lifecycle_action": lifecycleAction,
		"auto_start":       autoStart,
	}
	if req.VersionID != nil {
		logDetails["version_id"] = *req.VersionID
	}
	h.logPluginOperation(c, "plugin_lifecycle_"+lifecycleAction, &plugin, &plugin.ID, logDetails)

	resp := gin.H{
		"success": true,
		"plugin":  buildPluginResponse(plugin, h.uploadDir),
	}
	if deployment != nil {
		resp["deployment"] = deployment
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"plugin_id":          plugin.ID,
			"name":               plugin.Name,
			"display_name":       plugin.DisplayName,
			"action":             lifecycleAction,
			"version_id":         req.VersionID,
			"auto_start":         autoStart,
			"enabled":            plugin.Enabled,
			"lifecycle_status":   plugin.LifecycleStatus,
			"desired_generation": plugin.DesiredGeneration,
			"applied_generation": plugin.AppliedGeneration,
			"admin_id":           adminIDValue,
			"source":             "manual",
		}
		if deployment != nil {
			afterPayload["deployment_id"] = deployment.ID
			afterPayload["deployment_status"] = deployment.Status
			afterPayload["requested_generation"] = deployment.RequestedGeneration
			afterPayload["applied_generation"] = deployment.AppliedGeneration
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, pluginID uint, actionName string) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "plugin.lifecycle." + actionName + ".after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("plugin.lifecycle.%s.after hook execution failed: admin=%d plugin=%d err=%v", actionName, adminIDValue, pluginID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin",
			"hook_source":   "admin_api",
			"plugin_id":     fmt.Sprintf("%d", plugin.ID),
			"action":        lifecycleAction,
		})), afterPayload, plugin.ID, lifecycleAction)
	}
	c.JSON(http.StatusOK, resp)
}

// GetPluginVersions 获取插件版本
func (h *PluginHandler) GetPluginVersions(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}

	activeLifecycle := ""
	var plugin models.Plugin
	if err := h.db.Select("id", "lifecycle_status").First(&plugin, id).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusInternalServerError, "Failed to fetch plugin")
			return
		}
	} else {
		activeLifecycle = plugin.LifecycleStatus
	}

	var versions []models.PluginVersion
	if err := h.db.Where("plugin_id = ?", id).Order("created_at DESC").Limit(200).Find(&versions).Error; err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to fetch plugin versions")
		return
	}
	c.JSON(http.StatusOK, buildPluginVersionResponsesWithActiveLifecycle(versions, activeLifecycle, h.uploadDir))
}

// DeletePluginVersion 删除插件版本
func (h *PluginHandler) DeletePluginVersion(c *gin.Context) {
	pluginID, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	versionID, ok := h.parseVersionID(c)
	if !ok {
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, pluginID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to query plugin")
		return
	}

	var version models.PluginVersion
	if err := h.db.Where("plugin_id = ? AND id = ?", pluginID, versionID).First(&version).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin version not found")
			return
		}
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to query plugin version")
		return
	}
	if version.IsActive {
		h.respondPluginBizError(c, http.StatusBadRequest, newPluginBizError(http.StatusBadRequest, "active plugin version cannot be deleted", nil))
		return
	}
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"plugin_id":                plugin.ID,
			"plugin_name":              plugin.Name,
			"version_id":               version.ID,
			"version":                  version.Version,
			"version_lifecycle_status": version.LifecycleStatus,
			"admin_id":                 adminIDValue,
			"source":                   "manual",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "plugin.version.delete.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin_version",
			"hook_source":   "admin_api",
			"plugin_id":     fmt.Sprintf("%d", plugin.ID),
			"version_id":    fmt.Sprintf("%d", version.ID),
		}))
		if hookErr != nil {
			log.Printf("plugin.version.delete.before hook execution failed: admin=%d plugin=%d version=%d err=%v", adminIDValue, plugin.ID, version.ID, hookErr)
		} else if hookResult != nil && hookResult.Blocked {
			reason := strings.TrimSpace(hookResult.BlockReason)
			if reason == "" {
				reason = "Plugin version deletion rejected by plugin"
			}
			h.respondPluginError(c, http.StatusBadRequest, reason)
			return
		}
	}

	var siblingVersions []models.PluginVersion
	if err := h.db.Where("plugin_id = ? AND id <> ?", pluginID, versionID).Find(&siblingVersions).Error; err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to query sibling plugin versions")
		return
	}
	artifactFiles, artifactDirs := h.collectPluginVersionArtifactTargets(&plugin, &version, siblingVersions)

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.PluginDeployment{}).Where("target_version_id = ?", versionID).Update("target_version_id", nil).Error; err != nil {
			return fmt.Errorf("clear deployment target version failed: %w", err)
		}
		if err := tx.Where("plugin_id = ? AND id = ?", pluginID, versionID).Delete(&models.PluginVersion{}).Error; err != nil {
			return fmt.Errorf("delete plugin version failed: %w", err)
		}
		return nil
	}); err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to delete plugin version")
		return
	}
	h.invalidatePublicPluginCaches()

	cleanupErrors := cleanupPluginArtifactTargets(artifactFiles, artifactDirs)
	h.logPluginOperation(c, "plugin_version_delete", &plugin, &plugin.ID, map[string]interface{}{
		"version_id":               version.ID,
		"version":                  version.Version,
		"artifact_file_targets":    len(artifactFiles),
		"artifact_dir_targets":     len(artifactDirs),
		"artifact_cleanup_errors":  cleanupErrors,
		"version_lifecycle_status": version.LifecycleStatus,
	})
	resp := gin.H{
		"success": true,
		"message": "Plugin version deleted",
		"version": buildPluginVersionResponse(version, h.uploadDir),
	}
	if len(cleanupErrors) > 0 {
		resp["artifact_cleanup_errors"] = cleanupErrors
	}
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"plugin_id":                plugin.ID,
			"plugin_name":              plugin.Name,
			"version_id":               version.ID,
			"version":                  version.Version,
			"version_lifecycle_status": version.LifecycleStatus,
			"artifact_file_targets":    len(artifactFiles),
			"artifact_dir_targets":     len(artifactDirs),
			"artifact_cleanup_errors":  cleanupErrors,
			"admin_id":                 adminIDValue,
			"source":                   "manual",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "plugin.version.delete.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("plugin.version.delete.after hook execution failed: admin=%d plugin=%d version=%d err=%v", adminIDValue, plugin.ID, version.ID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin_version",
			"hook_source":   "admin_api",
			"plugin_id":     fmt.Sprintf("%d", plugin.ID),
			"version_id":    fmt.Sprintf("%d", version.ID),
		})), afterPayload)
	}
	c.JSON(http.StatusOK, resp)
}

// ActivatePluginVersion 激活插件版本
func (h *PluginHandler) ActivatePluginVersion(c *gin.Context) {
	pluginID, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	versionID, ok := h.parseVersionID(c)
	if !ok {
		return
	}

	var req activateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}

	autoStart := req.AutoStart != nil && *req.AutoStart
	if h.pluginManager != nil {
		var versionBefore models.PluginVersion
		if err := h.db.Where("plugin_id = ? AND id = ?", pluginID, versionID).First(&versionBefore).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusInternalServerError, "Failed to query plugin version")
			return
		}
		hookPayload := map[string]interface{}{
			"plugin_id":  pluginID,
			"version_id": versionID,
			"version":    versionBefore.Version,
			"auto_start": autoStart,
			"admin_id":   adminIDValue,
			"source":     "manual",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "plugin.version.activate.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin_version",
			"hook_source":   "admin_api",
			"plugin_id":     fmt.Sprintf("%d", pluginID),
			"version_id":    fmt.Sprintf("%d", versionID),
		}))
		if hookErr != nil {
			log.Printf("plugin.version.activate.before hook execution failed: admin=%d plugin=%d version=%d err=%v", adminIDValue, pluginID, versionID, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Plugin version activation rejected by plugin"
				}
				h.respondPluginError(c, http.StatusBadRequest, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["auto_start"]; exists {
					autoStartValue := parseBoolFromAny(value, autoStart)
					req.AutoStart = &autoStartValue
					autoStart = autoStartValue
				}
			}
		}
	}
	plugin, version, err := h.activatePluginVersionInternalWithDeploymentContext(
		pluginID,
		versionID,
		autoStart,
		"manual_hot_update",
		getOptionalUserID(c),
		"activate plugin version from versions dialog",
	)
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusBadGateway, err)
		return
	}
	h.logPluginOperation(c, "plugin_version_activate", plugin, &plugin.ID, map[string]interface{}{
		"version_id":   version.ID,
		"version":      version.Version,
		"auto_start":   autoStart,
		"activated_at": version.ActivatedAt,
	})
	if h.pluginManager != nil {
		afterPayload := map[string]interface{}{
			"plugin_id":          plugin.ID,
			"plugin_name":        plugin.Name,
			"version_id":         version.ID,
			"version":            version.Version,
			"auto_start":         autoStart,
			"lifecycle_status":   plugin.LifecycleStatus,
			"desired_generation": plugin.DesiredGeneration,
			"applied_generation": plugin.AppliedGeneration,
			"activated_at":       version.ActivatedAt,
			"admin_id":           adminIDValue,
			"source":             "manual",
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "plugin.version.activate.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("plugin.version.activate.after hook execution failed: admin=%d plugin=%d version=%d err=%v", adminIDValue, plugin.ID, version.ID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin_version",
			"hook_source":   "admin_api",
			"plugin_id":     fmt.Sprintf("%d", plugin.ID),
			"version_id":    fmt.Sprintf("%d", version.ID),
		})), afterPayload)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"plugin":  buildPluginResponse(*plugin, h.uploadDir),
		"version": buildPluginVersionResponse(*version, h.uploadDir),
	})
}
