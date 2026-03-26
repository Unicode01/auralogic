package admin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxPluginPackageUploadBytes = int64(128 * 1024 * 1024)

var errPluginPackageUploadTooLarge = errors.New("plugin package raw upload exceeds limit")

func pluginUploadStatusFromErr(err error) int {
	if errors.Is(err, errPluginPackageUploadTooLarge) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusInternalServerError
}

func buildPluginNameConflictError(plugin *models.Plugin, requestedName string) error {
	params := map[string]interface{}{
		"name": strings.TrimSpace(requestedName),
	}
	if plugin != nil {
		params["plugin_id"] = plugin.ID
		params["plugin_name"] = strings.TrimSpace(plugin.Name)
		params["plugin_display_name"] = strings.TrimSpace(plugin.DisplayName)
	}
	return newPluginBizError(http.StatusConflict, "plugin name conflicts with existing plugin", params)
}

func buildPluginPagePathConflictError(area string, path string, plugin *models.Plugin) error {
	params := map[string]interface{}{
		"area": strings.TrimSpace(area),
		"path": strings.TrimSpace(path),
	}
	if plugin != nil {
		params["plugin_id"] = plugin.ID
		params["plugin_name"] = strings.TrimSpace(plugin.Name)
		params["plugin_display_name"] = strings.TrimSpace(plugin.DisplayName)
	}
	return newPluginBizError(http.StatusConflict, "plugin page path conflicts with existing plugin", params)
}

func (h *PluginHandler) validatePluginUploadPageConflicts(targetPluginID uint, manifestRaw string) error {
	if h == nil || h.db == nil {
		return nil
	}

	adminPath, userPath := extractPluginManifestPagePaths(manifestRaw)
	if adminPath == "" && userPath == "" {
		return nil
	}

	var plugins []models.Plugin
	query := h.db.Select("id", "name", "display_name", "manifest")
	if targetPluginID > 0 {
		query = query.Where("id <> ?", targetPluginID)
	}
	if err := query.Find(&plugins).Error; err != nil {
		return fmt.Errorf("failed to check plugin page conflicts: %w", err)
	}

	for idx := range plugins {
		plugin := plugins[idx]
		existingAdminPath, existingUserPath := extractPluginManifestPagePaths(plugin.Manifest)
		if adminPath != "" && existingAdminPath != "" && adminPath == existingAdminPath {
			return buildPluginPagePathConflictError(frontendBootstrapAreaAdmin, adminPath, &plugin)
		}
		if userPath != "" && existingUserPath != "" && userPath == existingUserPath {
			return buildPluginPagePathConflictError(frontendBootstrapAreaUser, userPath, &plugin)
		}
	}
	return nil
}

// PreviewPluginPackage 预览插件包 manifest 与权限请求
func (h *PluginHandler) PreviewPluginPackage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		h.respondPluginError(c, http.StatusBadRequest, "file is required")
		return
	}

	savedPath, _, err := h.saveUploadedPackage(fileHeader)
	if err != nil {
		h.respondPluginErrorErr(c, pluginUploadStatusFromErr(err), err)
		return
	}
	defer func() {
		_ = os.Remove(savedPath)
	}()

	manifest, _, err := readManifestFromPackage(savedPath)
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	permissionRequests := collectManifestPermissionRequests(manifest)
	defaultGranted := collectManifestDefaultGrantedPermissions(manifest, permissionRequests)

	c.JSON(http.StatusOK, gin.H{
		"manifest":                    manifest,
		"requested_permissions":       permissionRequests,
		"default_granted_permissions": defaultGranted,
	})
}

// UploadPluginPackage 上传插件包并写入版本
func (h *PluginHandler) UploadPluginPackage(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
		h.respondPluginError(c, http.StatusBadRequest, "file is required")
		return
	}
	adminID := getOptionalUserID(c)
	adminIDValue := uint(0)
	if adminID != nil {
		adminIDValue = *adminID
	}

	savedPath, checksum, err := h.saveUploadedPackage(fileHeader)
	if err != nil {
		h.respondPluginErrorErr(c, pluginUploadStatusFromErr(err), err)
		return
	}

	cleanupFiles := []string{filepath.Clean(filepath.FromSlash(savedPath))}
	cleanupDirs := make([]string, 0, 1)
	cleanupPending := true
	defer func() {
		if !cleanupPending {
			return
		}
		_ = cleanupPluginArtifactTargets(cleanupFiles, cleanupDirs)
	}()

	manifest, manifestRaw, err := readManifestFromPackage(savedPath)
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	formName := strings.TrimSpace(c.PostForm("name"))
	formDisplayName := strings.TrimSpace(c.PostForm("display_name"))
	formDescription := strings.TrimSpace(c.PostForm("description"))
	formType := strings.TrimSpace(c.PostForm("type"))
	formRuntime := strings.TrimSpace(c.PostForm("runtime"))
	formAddress := strings.TrimSpace(c.PostForm("address"))
	formEntry := strings.TrimSpace(c.PostForm("entry"))
	formVersion := strings.TrimSpace(c.PostForm("version"))
	formConfig := strings.TrimSpace(c.PostForm("config"))
	formRuntimeParams := strings.TrimSpace(c.PostForm("runtime_params"))
	formCapabilities := strings.TrimSpace(c.PostForm("capabilities"))
	formGrantedPermissions := strings.TrimSpace(c.PostForm("granted_permissions"))
	pluginIDRaw := strings.TrimSpace(c.PostForm("plugin_id"))
	changelog := firstNonEmpty(strings.TrimSpace(c.PostForm("changelog")), manifestField(manifest, "changelog"))
	activate := parseBoolForm(c.PostForm("activate"), manifestBoolField(manifest, "activate", true))
	autoStart := parseBoolForm(c.PostForm("auto_start"), manifestBoolField(manifest, "auto_start", false))
	if h.pluginManager != nil {
		hookPayload := map[string]interface{}{
			"file_name":           fileHeader.Filename,
			"package_size_bytes":  fileHeader.Size,
			"checksum":            checksum,
			"name":                formName,
			"display_name":        formDisplayName,
			"description":         formDescription,
			"type":                formType,
			"runtime":             formRuntime,
			"address":             formAddress,
			"entry":               formEntry,
			"version":             formVersion,
			"config":              formConfig,
			"runtime_params":      formRuntimeParams,
			"capabilities":        formCapabilities,
			"granted_permissions": formGrantedPermissions,
			"plugin_id":           pluginIDRaw,
			"changelog":           changelog,
			"activate":            activate,
			"auto_start":          autoStart,
			"manifest_name":       manifestField(manifest, "name"),
			"manifest_type":       manifestField(manifest, "type"),
			"manifest_runtime":    manifestField(manifest, "runtime"),
			"admin_id":            adminIDValue,
			"source":              "package_upload",
		}
		hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "plugin.package.upload.before",
			Payload: hookPayload,
		}, buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin_package",
			"hook_source":   "admin_api",
		}))
		if hookErr != nil {
			log.Printf("plugin.package.upload.before hook execution failed: admin=%d file=%s err=%v", adminIDValue, fileHeader.Filename, hookErr)
		} else if hookResult != nil {
			if hookResult.Blocked {
				reason := strings.TrimSpace(hookResult.BlockReason)
				if reason == "" {
					reason = "Plugin package upload rejected by plugin"
				}
				h.respondPluginError(c, http.StatusBadRequest, reason)
				return
			}
			if hookResult.Payload != nil {
				if value, exists := hookResult.Payload["name"]; exists {
					formName = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["display_name"]; exists {
					formDisplayName = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["description"]; exists {
					formDescription = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["type"]; exists {
					formType = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["runtime"]; exists {
					formRuntime = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["address"]; exists {
					formAddress = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["entry"]; exists {
					formEntry = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["version"]; exists {
					formVersion = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["config"]; exists {
					formConfig = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["runtime_params"]; exists {
					formRuntimeParams = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["capabilities"]; exists {
					formCapabilities = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["granted_permissions"]; exists {
					if text := parseStringFromAny(value); text != "" {
						formGrantedPermissions = text
					} else if list := parseStringListFromAny(value); len(list) > 0 {
						body, marshalErr := json.Marshal(list)
						if marshalErr == nil {
							formGrantedPermissions = string(body)
						}
					}
				}
				if value, exists := hookResult.Payload["plugin_id"]; exists {
					formPluginID := parseStringFromAny(value)
					if formPluginID != "" {
						pluginIDRaw = formPluginID
					}
				}
				if value, exists := hookResult.Payload["changelog"]; exists {
					changelog = parseStringFromAny(value)
				}
				if value, exists := hookResult.Payload["activate"]; exists {
					activate = parseBoolFromAny(value, activate)
				}
				if value, exists := hookResult.Payload["auto_start"]; exists {
					autoStart = parseBoolFromAny(value, autoStart)
				}
			}
		}
	}

	emitUploadAfterHook := func(plugin models.Plugin, version models.PluginVersion, activateRequested bool, activateFailed bool, errorMessage string) {
		if h.pluginManager == nil {
			return
		}
		afterPayload := map[string]interface{}{
			"plugin_id":        plugin.ID,
			"plugin_name":      plugin.Name,
			"plugin_exists":    plugin.ID > 0 && pluginIDRaw != "",
			"version_id":       version.ID,
			"version":          version.Version,
			"file_name":        fileHeader.Filename,
			"manifest_name":    manifestField(manifest, "name"),
			"manifest_type":    manifestField(manifest, "type"),
			"manifest_runtime": manifestField(manifest, "runtime"),
			"activate":         activateRequested,
			"activate_failed":  activateFailed,
			"auto_start":       autoStart,
			"checksum":         checksum,
			"runtime":          version.Runtime,
			"type":             version.Type,
			"admin_id":         adminIDValue,
			"source":           "package_upload",
		}
		if strings.TrimSpace(errorMessage) != "" {
			afterPayload["error"] = errorMessage
		}
		go func(execCtx *service.ExecutionContext, payload map[string]interface{}, pluginID uint, versionID uint) {
			_, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    "plugin.package.upload.after",
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				log.Printf("plugin.package.upload.after hook execution failed: admin=%d plugin=%d version=%d err=%v", adminIDValue, pluginID, versionID, hookErr)
			}
		}(cloneAdminHookExecutionContext(buildAdminHookExecutionContext(c, adminID, map[string]string{
			"hook_resource": "plugin_package",
			"hook_source":   "admin_api",
		})), afterPayload, plugin.ID, version.ID)
	}

	configJSON, err := normalizeConfigJSON(formConfig, manifest)
	if err != nil {
		h.respondPluginError(c, http.StatusBadRequest, "invalid config json")
		return
	}
	runtimeParamsJSON, err := normalizeRuntimeParamsJSON(formRuntimeParams, manifest)
	if err != nil {
		h.respondPluginError(c, http.StatusBadRequest, "invalid runtime_params json")
		return
	}
	defaultCapabilities := manifestCapabilitiesJSON(manifest)

	name := firstNonEmpty(formName, manifestField(manifest, "name"))
	if name == "" {
		h.respondPluginError(c, http.StatusBadRequest, "plugin name is required (form field `name` or manifest.name)")
		return
	}

	var plugin models.Plugin
	pluginExists := false
	if pluginIDRaw != "" {
		pluginID, parseErr := strconv.ParseUint(pluginIDRaw, 10, 32)
		if parseErr != nil || pluginID == 0 {
			h.respondPluginError(c, http.StatusBadRequest, "invalid plugin_id")
			return
		}
		if err := h.db.First(&plugin, uint(pluginID)).Error; err != nil {
			h.respondPluginError(c, http.StatusNotFound, "plugin not found")
			return
		}
		pluginExists = true
	} else {
		findErr := h.db.Where("name = ?", name).First(&plugin).Error
		if findErr == nil {
			h.respondPluginErrorErr(c, http.StatusConflict, buildPluginNameConflictError(&plugin, name))
			return
		} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusInternalServerError, "failed to query plugin")
			return
		}
	}
	if pluginExists {
		defaultCapabilities = firstNonEmpty(defaultCapabilities, strings.TrimSpace(plugin.Capabilities), "{}")
	}
	capabilitiesJSON, err := normalizeCapabilitiesJSON(formCapabilities, defaultCapabilities)
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	permissionRequests := collectManifestPermissionRequests(manifest)
	if len(permissionRequests) > 0 {
		defaultGranted := collectManifestDefaultGrantedPermissions(manifest, permissionRequests)
		grantedPermissions, grantErr := resolveGrantedPermissions(formGrantedPermissions, permissionRequests, defaultGranted)
		if grantErr != nil {
			h.respondPluginErrorErr(c, http.StatusBadRequest, grantErr)
			return
		}
		capabilitiesJSON, err = applyPermissionGrantToCapabilities(capabilitiesJSON, permissionRequests, grantedPermissions)
		if err != nil {
			h.respondPluginError(c, http.StatusBadRequest, "invalid capabilities after applying permission grants")
			return
		}
	}

	selectedType := firstNonEmpty(formType, manifestField(manifest, "type"), "custom")
	selectedRuntimeRaw := firstNonEmpty(formRuntime, manifestField(manifest, "runtime"))
	if pluginExists {
		selectedRuntimeRaw = firstNonEmpty(selectedRuntimeRaw, plugin.Runtime)
	}
	selectedRuntime, err := h.resolveRuntime(selectedRuntimeRaw)
	if err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	if err := validatePluginPackageCompatibility(manifest, selectedRuntime); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	if err := h.validatePluginTypeAndRuntime(selectedType, selectedRuntime); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	selectedAddress := firstNonEmpty(formAddress, formEntry, manifestField(manifest, "address"), manifestField(manifest, "entry"))
	if pluginExists {
		selectedAddress = firstNonEmpty(selectedAddress, plugin.Address)
	}
	selectedVersion := normalizeVersion(firstNonEmpty(formVersion, manifestField(manifest, "version"), "0.0.0"))
	selectedDisplayName := firstNonEmpty(formDisplayName, manifestField(manifest, "display_name"), name)
	selectedDescription := firstNonEmpty(formDescription, manifestField(manifest, "description"))
	if selectedRuntime == service.PluginRuntimeJSWorker {
		resolvedAddress, normalizedPackagePath, extractedRoot, resolveErr := h.prepareJSWorkerAddressWithExtractRoot(savedPath, selectedAddress)
		if strings.TrimSpace(extractedRoot) != "" {
			cleanupDirs = append(cleanupDirs, filepath.Clean(filepath.FromSlash(extractedRoot)))
		}
		if resolveErr != nil {
			h.respondPluginErrorErr(c, http.StatusBadRequest, resolveErr)
			return
		}
		savedPath = normalizedPackagePath
		selectedAddress = resolvedAddress
	}
	if selectedRuntime == service.PluginRuntimeGRPC && strings.TrimSpace(selectedAddress) == "" {
		h.respondPluginError(c, http.StatusBadRequest, "service address is required for grpc runtime")
		return
	}
	if selectedRuntime == service.PluginRuntimeJSWorker && strings.TrimSpace(selectedAddress) == "" {
		h.respondPluginError(c, http.StatusBadRequest, "entry script path is required for js_worker runtime")
		return
	}
	if err := h.validatePluginUploadPageConflicts(plugin.ID, manifestRaw); err != nil {
		status := http.StatusInternalServerError
		var conflictErr *bizerr.Error
		if errors.As(err, &conflictErr) {
			status = http.StatusConflict
		}
		h.respondPluginErrorErr(c, status, err)
		return
	}

	if !pluginExists {
		plugin = models.Plugin{
			Name:            name,
			DisplayName:     selectedDisplayName,
			Description:     selectedDescription,
			Type:            selectedType,
			Runtime:         selectedRuntime,
			Address:         selectedAddress,
			Version:         selectedVersion,
			Config:          configJSON,
			RuntimeParams:   runtimeParamsJSON,
			Capabilities:    capabilitiesJSON,
			Manifest:        manifestRaw,
			PackagePath:     savedPath,
			PackageChecksum: checksum,
			Enabled:         false,
			Status:          "unknown",
			LifecycleStatus: models.PluginLifecycleUploaded,
		}
		plugin.RuntimeSpecHash = service.ComputePluginRuntimeSpecHash(&plugin)
		plugin.DesiredGeneration = 1
		plugin.AppliedGeneration = 1
		if selectedAddress != "" {
			plugin.LifecycleStatus = models.PluginLifecycleInstalled
		}
		if err := h.db.Select("*").Create(&plugin).Error; err != nil {
			h.respondPluginError(c, http.StatusInternalServerError, "failed to create plugin")
			return
		}
		if err := h.db.Model(&models.Plugin{}).Where("id = ?", plugin.ID).Update("enabled", false).Error; err != nil {
			h.respondPluginError(c, http.StatusInternalServerError, "failed to initialize plugin enabled state")
			return
		}
		plugin.Enabled = false
	}

	versionRecord := &models.PluginVersion{
		PluginID:             plugin.ID,
		Version:              selectedVersion,
		PackageName:          fileHeader.Filename,
		PackagePath:          savedPath,
		PackageChecksum:      checksum,
		Manifest:             manifestRaw,
		Type:                 selectedType,
		Runtime:              selectedRuntime,
		Address:              selectedAddress,
		ConfigSnapshot:       configJSON,
		RuntimeParams:        runtimeParamsJSON,
		CapabilitiesSnapshot: capabilitiesJSON,
		Changelog:            changelog,
		LifecycleStatus:      models.PluginLifecycleUploaded,
		UploadedBy:           getOptionalUserID(c),
	}
	if err := h.db.Create(versionRecord).Error; err != nil {
		if !pluginExists && plugin.ID > 0 {
			_ = h.db.Delete(&plugin).Error
		}
		h.respondPluginError(c, http.StatusInternalServerError, "failed to create plugin version")
		return
	}
	cleanupPending = false

	if activate {
		pluginRefreshed, activatedVersion, activateErr := h.activatePluginVersionInternalWithDeploymentContext(
			plugin.ID,
			versionRecord.ID,
			autoStart,
			"package_upload_activate",
			getOptionalUserID(c),
			"activate uploaded plugin package",
		)
		if activateErr != nil {
			h.logPluginOperation(c, "plugin_package_upload_activate_failed", &plugin, &plugin.ID, map[string]interface{}{
				"file_name":        fileHeader.Filename,
				"plugin_exists":    pluginExists,
				"activate":         true,
				"auto_start":       autoStart,
				"version_id":       versionRecord.ID,
				"version":          versionRecord.Version,
				"manifest_name":    manifestField(manifest, "name"),
				"manifest_type":    manifestField(manifest, "type"),
				"manifest_runtime": manifestField(manifest, "runtime"),
				"error":            activateErr.Error(),
			})
			biz := newPluginBizError(http.StatusBadRequest, "package uploaded but activate failed", map[string]interface{}{
				"details": activateErr.Error(),
			})
			// 上传成功但激活失败：返回 200，避免前端把整个上传判定为失败。
			payload := pluginBizErrorPayload(http.StatusBadRequest, biz)
			payload["activate_failed"] = true
			payload["plugin"] = buildPluginResponse(plugin, h.uploadDir)
			payload["version"] = buildPluginVersionResponse(*versionRecord, h.uploadDir)
			payload["manifest"] = manifest
			payload["requested_action"] = "activate"
			emitUploadAfterHook(plugin, *versionRecord, true, true, activateErr.Error())
			c.JSON(http.StatusOK, payload)
			return
		}
		h.logPluginOperation(c, "plugin_package_upload_activate", pluginRefreshed, &pluginRefreshed.ID, map[string]interface{}{
			"file_name":        fileHeader.Filename,
			"plugin_exists":    pluginExists,
			"activate":         true,
			"auto_start":       autoStart,
			"version_id":       activatedVersion.ID,
			"version":          activatedVersion.Version,
			"manifest_name":    manifestField(manifest, "name"),
			"manifest_type":    manifestField(manifest, "type"),
			"manifest_runtime": manifestField(manifest, "runtime"),
		})
		emitUploadAfterHook(*pluginRefreshed, *activatedVersion, true, false, "")
		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"plugin":   buildPluginResponse(*pluginRefreshed, h.uploadDir),
			"version":  buildPluginVersionResponse(*activatedVersion, h.uploadDir),
			"manifest": manifest,
		})
		return
	}

	h.logPluginOperation(c, "plugin_package_upload", &plugin, &plugin.ID, map[string]interface{}{
		"file_name":        fileHeader.Filename,
		"plugin_exists":    pluginExists,
		"activate":         false,
		"auto_start":       false,
		"version_id":       versionRecord.ID,
		"version":          versionRecord.Version,
		"manifest_name":    manifestField(manifest, "name"),
		"manifest_type":    manifestField(manifest, "type"),
		"manifest_runtime": manifestField(manifest, "runtime"),
	})
	emitUploadAfterHook(plugin, *versionRecord, false, false, "")

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"plugin":   buildPluginResponse(plugin, h.uploadDir),
		"version":  buildPluginVersionResponse(*versionRecord, h.uploadDir),
		"manifest": manifest,
	})
}

func (h *PluginHandler) saveUploadedPackage(fileHeader *multipart.FileHeader) (string, string, error) {
	if fileHeader == nil {
		return "", "", fmt.Errorf("empty file")
	}
	if fileHeader.Size > maxPluginPackageUploadBytes {
		return "", "", fmt.Errorf("%w: max=%d bytes", errPluginPackageUploadTooLarge, maxPluginPackageUploadBytes)
	}

	if err := os.MkdirAll(h.uploadDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create upload dir: %w", err)
	}

	safeName := sanitizeFileName(fileHeader.Filename)
	finalName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), safeName)
	savePath := filepath.Join(h.uploadDir, finalName)
	cleanupSavedFile := func() {
		_ = os.Remove(savePath)
	}

	src, err := fileHeader.Open()
	if err != nil {
		return "", "", fmt.Errorf("failed to open upload: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(savePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create file: %w", err)
	}
	dstClosed := false
	defer func() {
		if !dstClosed {
			_ = dst.Close()
		}
	}()

	hasher := sha256.New()
	limitedReader := io.LimitReader(src, maxPluginPackageUploadBytes+1)
	written, copyErr := io.Copy(io.MultiWriter(dst, hasher), limitedReader)
	if closeErr := dst.Close(); closeErr != nil {
		dstClosed = true
		cleanupSavedFile()
		return "", "", fmt.Errorf("failed to close saved upload: %w", closeErr)
	}
	dstClosed = true
	if copyErr != nil {
		cleanupSavedFile()
		return "", "", fmt.Errorf("failed to save file: %w", copyErr)
	}
	if written > maxPluginPackageUploadBytes {
		cleanupSavedFile()
		return "", "", fmt.Errorf("%w: max=%d bytes", errPluginPackageUploadTooLarge, maxPluginPackageUploadBytes)
	}

	return filepath.ToSlash(savePath), hex.EncodeToString(hasher.Sum(nil)), nil
}

func (h *PluginHandler) prepareJSWorkerAddress(packagePath, requestedAddress string) (string, string, error) {
	resolved, normalizedPackagePath, _, err := h.prepareJSWorkerAddressWithExtractRoot(packagePath, requestedAddress)
	return resolved, normalizedPackagePath, err
}

func (h *PluginHandler) prepareJSWorkerAddressWithExtractRoot(packagePath, requestedAddress string) (string, string, string, error) {
	normalizedPackagePath, err := service.NormalizeJSWorkerPackagePath(packagePath)
	if err != nil {
		return "", "", "", err
	}

	extractRoot, err := service.ResolveJSWorkerPackageRoot(normalizedPackagePath)
	if err != nil {
		return "", "", "", err
	}
	extractRootPublic := filepath.ToSlash(extractRoot)

	packageExt := strings.ToLower(filepath.Ext(normalizedPackagePath))
	switch packageExt {
	case ".zip":
		if removeErr := os.RemoveAll(extractRoot); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", normalizedPackagePath, extractRootPublic, fmt.Errorf("failed to reset js_worker extract root: %w", removeErr)
		}
		if err := unzipPackageSafe(normalizedPackagePath, extractRoot); err != nil {
			return "", normalizedPackagePath, extractRootPublic, err
		}
	case ".js", ".mjs", ".cjs", ".jsx", ".ts", ".tsx":
		if removeErr := os.RemoveAll(extractRoot); removeErr != nil && !os.IsNotExist(removeErr) {
			return "", normalizedPackagePath, extractRootPublic, fmt.Errorf("failed to reset js_worker extract root: %w", removeErr)
		}
		if err := os.MkdirAll(extractRoot, 0755); err != nil {
			return "", normalizedPackagePath, extractRootPublic, fmt.Errorf("failed to create js_worker extract root: %w", err)
		}
		targetPath := filepath.Join(extractRoot, filepath.Base(normalizedPackagePath))
		if _, err := copyFileWithMode(normalizedPackagePath, targetPath, 0644); err != nil {
			return "", normalizedPackagePath, extractRootPublic, fmt.Errorf("failed to stage js_worker script: %w", err)
		}
	default:
		return "", normalizedPackagePath, "", fmt.Errorf("js_worker package must be a zip archive or js module file")
	}

	entry := strings.TrimSpace(requestedAddress)
	if entry == "" {
		entry = findDefaultJSWorkerEntryFile(extractRoot)
	}
	if entry == "" && packageExt != ".zip" {
		entry = filepath.Join(extractRoot, filepath.Base(normalizedPackagePath))
	}
	if entry == "" {
		firstJS, err := findFirstJSFile(extractRoot)
		if err != nil {
			return "", normalizedPackagePath, extractRootPublic, fmt.Errorf("failed to detect js entry from package: %w", err)
		}
		entry = firstJS
	}

	normalizedAddress, err := service.NormalizeJSWorkerRelativeEntryPath(normalizedPackagePath, entry)
	if err != nil {
		return "", normalizedPackagePath, extractRootPublic, err
	}

	return normalizedAddress, normalizedPackagePath, extractRootPublic, nil
}
