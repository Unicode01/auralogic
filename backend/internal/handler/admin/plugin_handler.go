package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/bizerr"
	"auralogic/internal/pkg/logger"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type PluginHandler struct {
	db                     *gorm.DB
	pluginManager          *service.PluginManagerService
	uploadDir              string
	publicExtensionsCache  *pluginPublicEndpointCache
	publicBootstrapCache   *pluginPublicEndpointCache
	frontendBootstrapCache *pluginPublicEndpointCache
}

func NewPluginHandler(db *gorm.DB, pluginManager *service.PluginManagerService, uploadDir string) *PluginHandler {
	if strings.TrimSpace(uploadDir) == "" {
		uploadDir = filepath.Join("uploads", "plugins")
	}
	return &PluginHandler{
		db:                     db,
		pluginManager:          pluginManager,
		uploadDir:              uploadDir,
		publicExtensionsCache:  newPluginPublicEndpointCache(pluginPublicEndpointCacheTTL, pluginPublicEndpointCacheMaxEntries),
		publicBootstrapCache:   newPluginPublicEndpointCache(pluginPublicEndpointCacheTTL, pluginPublicEndpointCacheMaxEntries),
		frontendBootstrapCache: newPluginPublicEndpointCache(pluginPublicEndpointCacheTTL, pluginPublicEndpointCacheMaxEntries),
	}
}

type executePluginContext struct {
	UserID    *uint             `json:"user_id"`
	OrderID   *uint             `json:"order_id"`
	SessionID string            `json:"session_id"`
	Metadata  map[string]string `json:"metadata"`
}

type executePluginRequest struct {
	Action      string                `json:"action" binding:"required"`
	Params      map[string]string     `json:"params"`
	Path        string                `json:"path,omitempty"`
	QueryParams map[string]string     `json:"query_params"`
	RouteParams map[string]string     `json:"route_params"`
	Context     *executePluginContext `json:"context"`
}

type testPluginRequest struct {
	Checks             []string                          `json:"checks"`
	StopOnFailure      bool                              `json:"stop_on_failure"`
	Slot               string                            `json:"slot"`
	Path               string                            `json:"path"`
	UserBootstrapPath  string                            `json:"user_bootstrap_path"`
	AdminBootstrapPath string                            `json:"admin_bootstrap_path"`
	HookPayloads       map[string]map[string]interface{} `json:"hook_payloads"`
	Context            *executePluginContext             `json:"context"`
	Metadata           map[string]string                 `json:"metadata"`
}

type pluginCheckResult struct {
	Check      string                 `json:"check"`
	Success    bool                   `json:"success"`
	Skipped    bool                   `json:"skipped,omitempty"`
	Error      string                 `json:"error,omitempty"`
	DurationMs int                    `json:"duration_ms"`
	Data       map[string]interface{} `json:"data,omitempty"`
}

type lifecycleActionRequest struct {
	Action    string `json:"action" binding:"required"`
	VersionID *uint  `json:"version_id"`
	AutoStart *bool  `json:"auto_start"`
}

type activateVersionRequest struct {
	AutoStart *bool `json:"auto_start"`
}

type updatePluginRequest struct {
	DisplayName   *string `json:"display_name"`
	Description   *string `json:"description"`
	Type          *string `json:"type"`
	Runtime       *string `json:"runtime"`
	Address       *string `json:"address"`
	PackagePath   *string `json:"package_path"`
	Config        *string `json:"config"`
	RuntimeParams *string `json:"runtime_params"`
	Capabilities  *string `json:"capabilities"`
	Version       *string `json:"version"`
	Enabled       *bool   `json:"enabled"`
}

type pluginPackagePermission struct {
	Key      string                        `json:"key"`
	Required bool                          `json:"required"`
	Reason   service.ManifestLocalizedText `json:"reason"`
}

type pluginPackageWebhookManifest struct {
	Key             string                        `json:"key"`
	Description     service.ManifestLocalizedText `json:"description"`
	Action          string                        `json:"action"`
	Method          string                        `json:"method"`
	AuthMode        string                        `json:"auth_mode"`
	SecretKey       string                        `json:"secret_key"`
	Header          string                        `json:"header"`
	QueryParam      string                        `json:"query_param"`
	SignatureHeader string                        `json:"signature_header"`
}

type pluginPackageFrontendPage struct {
	Path  string                        `json:"path"`
	Title service.ManifestLocalizedText `json:"title"`
}

type pluginPackageFrontendManifest struct {
	AdminPage *pluginPackageFrontendPage `json:"admin_page"`
	UserPage  *pluginPackageFrontendPage `json:"user_page"`
}

type pluginPackageWorkspaceCommand struct {
	Name        string                        `json:"name"`
	Title       service.ManifestLocalizedText `json:"title"`
	Description service.ManifestLocalizedText `json:"description"`
	Entry       string                        `json:"entry"`
	Interactive bool                          `json:"interactive"`
	Permissions []string                      `json:"permissions"`
}

type pluginPackageWorkspaceManifest struct {
	Enabled  *bool                           `json:"enabled"`
	Title    service.ManifestLocalizedText   `json:"title"`
	Commands []pluginPackageWorkspaceCommand `json:"commands"`
}

type pluginPackageManifest struct {
	Name                   string                          `json:"name"`
	DisplayName            service.ManifestLocalizedText   `json:"display_name"`
	Description            service.ManifestLocalizedText   `json:"description"`
	Icon                   string                          `json:"icon"`
	Type                   string                          `json:"type"`
	Runtime                string                          `json:"runtime"`
	Address                string                          `json:"address"`
	Entry                  string                          `json:"entry"`
	Version                string                          `json:"version"`
	PollInterval           *int                            `json:"poll_interval"`
	ManifestVersion        string                          `json:"manifest_version"`
	ProtocolVersion        string                          `json:"protocol_version"`
	MinHostProtocolVersion string                          `json:"min_host_protocol_version"`
	MaxHostProtocolVersion string                          `json:"max_host_protocol_version"`
	Changelog              service.ManifestLocalizedText   `json:"changelog"`
	Activate               *bool                           `json:"activate"`
	AutoStart              *bool                           `json:"auto_start"`
	Config                 map[string]interface{}          `json:"config"`
	ConfigSchema           map[string]interface{}          `json:"config_schema"`
	SecretSchema           map[string]interface{}          `json:"secret_schema"`
	RuntimeParams          map[string]interface{}          `json:"runtime_params"`
	RuntimeParamsSchema    map[string]interface{}          `json:"runtime_params_schema"`
	Frontend               *pluginPackageFrontendManifest  `json:"frontend"`
	Workspace              *pluginPackageWorkspaceManifest `json:"workspace"`
	Webhooks               []pluginPackageWebhookManifest  `json:"webhooks"`
	Capabilities           map[string]interface{}          `json:"capabilities"`
	RequestedPermissions   []string                        `json:"requested_permissions"`
	RequiredPermissions    []string                        `json:"required_permissions"`
	Permissions            []pluginPackagePermission       `json:"permissions"`
}

const (
	maxPluginPackageFiles            = 1024
	maxPluginPackageSingleFileBytes  = int64(16 * 1024 * 1024)  // 16MB
	maxPluginPackageTotalBytes       = int64(128 * 1024 * 1024) // 128MB
	maxPluginPackageCompressionRatio = 200.0

	frontendBootstrapAreaUser  = "user"
	frontendBootstrapAreaAdmin = "admin"

	pluginFrontendHTMLModeSanitize = "sanitize"
	pluginFrontendHTMLModeTrusted  = "trusted"
)

var pluginTestDefaultChecks = []string{
	"health",
	"hook.frontend.slot.render",
	"hook.frontend.bootstrap",
	"hook.auth.login.before",
	"hook.order.create.before",
	"hook.payment.confirm.before",
	"hook.ticket.create.after",
	"hook.product.update.before",
	"hook.promo.validate.before",
}

var pluginTestAllHookChecks = []string{
	"hook.frontend.slot.render",
	"hook.frontend.bootstrap",
	"hook.auth.register.before",
	"hook.auth.register.after",
	"hook.auth.login.before",
	"hook.auth.login.after",
	"hook.auth.password.reset.before",
	"hook.auth.password.reset.after",
	"hook.order.create.before",
	"hook.order.create.after",
	"hook.order.complete.before",
	"hook.order.complete.after",
	"hook.order.admin.complete.before",
	"hook.order.admin.complete.after",
	"hook.order.admin.cancel.before",
	"hook.order.admin.cancel.after",
	"hook.order.admin.refund.before",
	"hook.order.admin.refund.after",
	"hook.order.admin.refund_finalize.before",
	"hook.order.admin.refund_finalize.after",
	"hook.order.admin.mark_paid.before",
	"hook.order.admin.mark_paid.after",
	"hook.order.admin.deliver_virtual.before",
	"hook.order.admin.deliver_virtual.after",
	"hook.order.admin.update_shipping.before",
	"hook.order.admin.update_shipping.after",
	"hook.order.admin.update_price.before",
	"hook.order.admin.update_price.after",
	"hook.order.admin.delete.before",
	"hook.order.admin.delete.after",
	"hook.order.auto_cancel.before",
	"hook.order.auto_cancel.after",
	"hook.order.status.changed.after",
	"hook.payment.method.select.before",
	"hook.payment.method.select.after",
	"hook.payment.confirm.before",
	"hook.payment.confirm.after",
	"hook.payment.polling.succeeded",
	"hook.payment.polling.failed",
	"hook.ticket.create.before",
	"hook.ticket.create.after",
	"hook.ticket.message.user.before",
	"hook.ticket.message.user.after",
	"hook.ticket.message.admin.before",
	"hook.ticket.message.admin.after",
	"hook.ticket.status.user.before",
	"hook.ticket.status.user.after",
	"hook.ticket.update.admin.before",
	"hook.ticket.update.admin.after",
	"hook.ticket.assign.before",
	"hook.ticket.assign.after",
	"hook.ticket.attachment.upload.before",
	"hook.ticket.attachment.upload.after",
	"hook.ticket.message.read.user.after",
	"hook.ticket.message.read.admin.after",
	"hook.ticket.order.share.after",
	"hook.ticket.auto_close.before",
	"hook.ticket.auto_close.after",
	"hook.product.create.before",
	"hook.product.create.after",
	"hook.product.update.before",
	"hook.product.update.after",
	"hook.product.delete.before",
	"hook.product.delete.after",
	"hook.product.status.update.before",
	"hook.product.status.update.after",
	"hook.product.inventory_mode.update.before",
	"hook.product.inventory_mode.update.after",
	"hook.inventory.reserve.before",
	"hook.inventory.reserve.after",
	"hook.inventory.release.after",
	"hook.promo.validate.before",
	"hook.promo.validate.after",
}

func (h *PluginHandler) parsePluginID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		h.respondPluginError(c, http.StatusBadRequest, "Invalid plugin id")
		return 0, false
	}
	return uint(id), true
}

func (h *PluginHandler) parseVersionID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("version_id"), 10, 32)
	if err != nil || id == 0 {
		h.respondPluginError(c, http.StatusBadRequest, "Invalid plugin version id")
		return 0, false
	}
	return uint(id), true
}

func mergePluginOperationDetails(plugin *models.Plugin, extra map[string]interface{}) map[string]interface{} {
	details := make(map[string]interface{}, len(extra)+8)
	if plugin != nil {
		details["plugin_name"] = plugin.Name
		details["display_name"] = plugin.DisplayName
		details["type"] = plugin.Type
		details["runtime"] = plugin.Runtime
		details["version"] = plugin.Version
		details["enabled"] = plugin.Enabled
		details["lifecycle_status"] = plugin.LifecycleStatus
		details["frontend_html_mode"] = parsePluginFrontendHTMLModeFromCapabilitiesRaw(plugin.Capabilities)
	}
	for key, value := range extra {
		details[key] = value
	}
	return details
}

func (h *PluginHandler) invalidatePublicPluginCaches() {
	if h == nil {
		return
	}
	if h.publicExtensionsCache != nil {
		h.publicExtensionsCache.Clear()
	}
	if h.publicBootstrapCache != nil {
		h.publicBootstrapCache.Clear()
	}
	if h.frontendBootstrapCache != nil {
		h.frontendBootstrapCache.Clear()
	}
}

func sortedStringMapKeys(source map[string]string) []string {
	if len(source) == 0 {
		return []string{}
	}
	keys := make([]string, 0, len(source))
	for key := range source {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		keys = append(keys, trimmed)
	}
	sort.Strings(keys)
	return keys
}

func (h *PluginHandler) logPluginOperation(c *gin.Context, action string, plugin *models.Plugin, resourceID *uint, extra map[string]interface{}) {
	if h == nil || h.db == nil || c == nil || strings.TrimSpace(action) == "" {
		return
	}
	if resourceID == nil && plugin != nil {
		id := plugin.ID
		resourceID = &id
	}
	logger.LogOperation(h.db, c, strings.TrimSpace(action), "plugin", resourceID, mergePluginOperationDetails(plugin, extra))
}

func (h *PluginHandler) respondPluginError(c *gin.Context, status int, message string) {
	h.respondPluginBizError(c, status, newPluginBizError(status, message, nil))
}

func (h *PluginHandler) respondPluginErrorErr(c *gin.Context, status int, err error) {
	if err == nil {
		h.respondPluginError(c, status, http.StatusText(status))
		return
	}

	var bizErr *bizerr.Error
	if errors.As(err, &bizErr) {
		h.respondPluginBizError(c, status, bizErr)
		return
	}

	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = strings.TrimSpace(http.StatusText(status))
	}
	if message == "" {
		message = "plugin operation failed"
	}

	params := map[string]interface{}{
		"cause": message,
	}
	h.respondPluginBizError(c, status, bizerr.New(pluginErrorKey(status, "operation failed"), message).WithParams(params))
}

func (h *PluginHandler) respondPluginBizError(c *gin.Context, status int, err *bizerr.Error) {
	c.JSON(status, pluginBizErrorPayload(status, err))
}

func newPluginBizError(status int, message string, params map[string]interface{}) *bizerr.Error {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		trimmed = strings.TrimSpace(http.StatusText(status))
	}
	if trimmed == "" {
		trimmed = "plugin operation failed"
	}

	err := bizerr.New(pluginErrorKey(status, trimmed), trimmed)
	normalizedParams := normalizePluginBizErrorParams(params)
	if len(normalizedParams) > 0 {
		err = err.WithParams(normalizedParams)
	}
	return err
}

func pluginBizErrorPayload(status int, err *bizerr.Error) gin.H {
	if err == nil {
		err = newPluginBizError(status, http.StatusText(status), nil)
	}

	payload := gin.H{
		"success":      false,
		"status":       status,
		"message":      err.Message,
		"error":        err.Message,
		"error_key":    err.Key,
		"error_params": err.Params,
	}
	if cause := pluginBizErrorParamText(err.Params, "cause"); cause != "" {
		payload["cause"] = cause
	}
	if details := pluginBizErrorParamText(err.Params, "details"); details != "" {
		payload["details"] = details
	}
	return payload
}

func pluginBizErrorParamText(params map[string]interface{}, key string) string {
	if len(params) == 0 {
		return ""
	}
	value, exists := params[key]
	if !exists || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func mergePluginFailurePayload(target gin.H, payload gin.H) gin.H {
	if target == nil {
		target = gin.H{}
	}
	for key, value := range payload {
		target[key] = value
	}
	return target
}

func buildPluginFailurePayload(status int, message string, params map[string]interface{}) gin.H {
	return pluginBizErrorPayload(status, newPluginBizError(status, message, params))
}

func buildPluginExecuteFailurePayload(status int, message string, cause string, data interface{}, metadata interface{}) gin.H {
	params := map[string]interface{}{}
	if strings.TrimSpace(cause) != "" {
		params["cause"] = strings.TrimSpace(cause)
	}
	payload := buildPluginFailurePayload(status, message, params)
	payload["data"] = data
	payload["metadata"] = metadata
	switch typed := metadata.(type) {
	case map[string]string:
		if taskID := extractPluginExecutionTaskID(typed); taskID != "" {
			payload["task_id"] = taskID
		}
	case map[string]interface{}:
		if taskID := strings.TrimSpace(fmt.Sprint(typed[service.PluginExecutionMetadataID])); taskID != "" && taskID != "<nil>" {
			payload["task_id"] = taskID
		}
	}
	return payload
}

type pluginExecuteStreamEvent struct {
	Type          string                 `json:"type"`
	Index         int                    `json:"index"`
	TaskID        string                 `json:"task_id,omitempty"`
	TaskStatusURL string                 `json:"task_status_url,omitempty"`
	TaskCancelURL string                 `json:"task_cancel_url,omitempty"`
	Success       bool                   `json:"success"`
	Data          map[string]interface{} `json:"data,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Metadata      map[string]string      `json:"metadata,omitempty"`
	IsFinal       bool                   `json:"is_final"`
}

type pluginExecutionTaskLinks struct {
	StatusURL string
	CancelURL string
}

func buildAdminPluginExecutionTaskLinks(pluginID uint, taskID string) pluginExecutionTaskLinks {
	taskID = strings.TrimSpace(taskID)
	if pluginID == 0 || taskID == "" {
		return pluginExecutionTaskLinks{}
	}
	return pluginExecutionTaskLinks{
		StatusURL: fmt.Sprintf("/api/admin/plugins/%d/tasks/%s", pluginID, taskID),
		CancelURL: fmt.Sprintf("/api/admin/plugins/%d/tasks/%s/cancel", pluginID, taskID),
	}
}

func extractPluginExecutionTaskID(metadata map[string]string) string {
	if len(metadata) == 0 {
		return ""
	}
	return strings.TrimSpace(metadata[service.PluginExecutionMetadataID])
}

func clonePluginExecutionMetadataWithStatus(metadata map[string]string, status string) map[string]string {
	cloned := cloneStringMap(metadata)
	if cloned == nil {
		cloned = make(map[string]string)
	}
	if strings.TrimSpace(status) != "" {
		cloned[service.PluginExecutionMetadataStatus] = strings.TrimSpace(status)
	}
	return cloned
}

func sanitizeUserProvidedExecutionMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return cloneStringMap(metadata)
	}
	cloned := cloneStringMap(metadata)
	for _, key := range []string{
		service.PluginExecutionMetadataID,
		service.PluginExecutionMetadataStatus,
		service.PluginExecutionMetadataStream,
		service.PluginExecutionMetadataRuntime,
		service.PluginExecutionMetadataStartedAt,
		service.PluginExecutionMetadataHook,
		service.PluginScopeMetadataAuthenticated,
		service.PluginScopeMetadataSuperAdmin,
		service.PluginScopeMetadataPermissions,
	} {
		delete(cloned, key)
	}
	return cloned
}

func resolvePluginExecutionFailureStatus(err error) string {
	switch {
	case err == nil:
		return service.PluginExecutionStatusFailed
	case errors.Is(err, context.Canceled) || strings.Contains(strings.ToLower(strings.TrimSpace(err.Error())), "context canceled"):
		return service.PluginExecutionStatusCanceled
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(strings.ToLower(strings.TrimSpace(err.Error())), "deadline exceeded"):
		return service.PluginExecutionStatusTimedOut
	default:
		return service.PluginExecutionStatusFailed
	}
}

func (h *PluginHandler) applyPluginExecutionHeaders(c *gin.Context, taskID string) {
	if c == nil {
		return
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}
	c.Header("X-Plugin-Execution-ID", taskID)
}

func (h *PluginHandler) startPluginNDJSONStream(c *gin.Context, taskID string) error {
	if c == nil {
		return fmt.Errorf("request context is nil")
	}
	if _, ok := c.Writer.(http.Flusher); !ok {
		return fmt.Errorf("streaming is not supported by the current response writer")
	}
	h.applyPluginExecutionHeaders(c, taskID)
	c.Status(http.StatusOK)
	c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Header("Connection", "keep-alive")
	return nil
}

func (h *PluginHandler) writePluginStreamEvent(c *gin.Context, event pluginExecuteStreamEvent) error {
	if c == nil {
		return fmt.Errorf("request context is nil")
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := c.Writer.Write(append(body, '\n')); err != nil {
		return err
	}
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

func (h *PluginHandler) writePluginStreamTaskStarted(
	c *gin.Context,
	taskID string,
	links pluginExecutionTaskLinks,
	metadata map[string]string,
) error {
	return h.writePluginStreamEvent(c, pluginExecuteStreamEvent{
		Type:          "task",
		Index:         -1,
		TaskID:        strings.TrimSpace(taskID),
		TaskStatusURL: strings.TrimSpace(links.StatusURL),
		TaskCancelURL: strings.TrimSpace(links.CancelURL),
		Success:       true,
		Metadata:      clonePluginExecutionMetadataWithStatus(metadata, service.PluginExecutionStatusRunning),
		IsFinal:       false,
	})
}

func (h *PluginHandler) writePluginStreamChunk(c *gin.Context, chunk *service.ExecutionStreamChunk) error {
	return h.writePluginStreamChunkWithLinks(c, chunk, pluginExecutionTaskLinks{})
}

func (h *PluginHandler) writePluginStreamChunkWithLinks(c *gin.Context, chunk *service.ExecutionStreamChunk, links pluginExecutionTaskLinks) error {
	if chunk == nil {
		return fmt.Errorf("stream chunk is nil")
	}
	taskID := strings.TrimSpace(chunk.TaskID)
	if taskID == "" {
		taskID = extractPluginExecutionTaskID(chunk.Metadata)
	}
	h.applyPluginExecutionHeaders(c, taskID)
	return h.writePluginStreamEvent(c, pluginExecuteStreamEvent{
		Type:          "chunk",
		Index:         chunk.Index,
		TaskID:        taskID,
		TaskStatusURL: strings.TrimSpace(links.StatusURL),
		TaskCancelURL: strings.TrimSpace(links.CancelURL),
		Success:       chunk.Success,
		Data:          chunk.Data,
		Error:         strings.TrimSpace(chunk.Error),
		Metadata:      chunk.Metadata,
		IsFinal:       chunk.IsFinal,
	})
}

func (h *PluginHandler) writePluginStreamError(c *gin.Context, index int, cause string) error {
	return h.writePluginStreamErrorWithLinks(c, index, cause, "", pluginExecutionTaskLinks{}, nil)
}

func (h *PluginHandler) writePluginStreamErrorWithLinks(
	c *gin.Context,
	index int,
	cause string,
	taskID string,
	links pluginExecutionTaskLinks,
	metadata map[string]string,
) error {
	taskID = strings.TrimSpace(taskID)
	h.applyPluginExecutionHeaders(c, taskID)
	return h.writePluginStreamEvent(c, pluginExecuteStreamEvent{
		Type:          "error",
		Index:         index,
		TaskID:        taskID,
		TaskStatusURL: strings.TrimSpace(links.StatusURL),
		TaskCancelURL: strings.TrimSpace(links.CancelURL),
		Success:       false,
		Error:         strings.TrimSpace(cause),
		Metadata:      metadata,
		IsFinal:       true,
	})
}

func summarizePluginCheckFailures(results []pluginCheckResult) string {
	failures := make([]string, 0, 3)
	for _, result := range results {
		if result.Success {
			continue
		}
		check := strings.TrimSpace(result.Check)
		errText := strings.TrimSpace(result.Error)
		switch {
		case check != "" && errText != "":
			failures = append(failures, fmt.Sprintf("%s: %s", check, errText))
		case errText != "":
			failures = append(failures, errText)
		case check != "":
			failures = append(failures, fmt.Sprintf("%s failed", check))
		default:
			failures = append(failures, "plugin check failed")
		}
		if len(failures) >= 3 {
			break
		}
	}
	return strings.Join(failures, "; ")
}

func normalizePluginBizErrorParams(params map[string]interface{}) map[string]interface{} {
	if len(params) == 0 {
		return nil
	}

	normalized := make(map[string]interface{}, len(params)+3)
	for key, value := range params {
		normalized[key] = value
	}

	resolveText := func(keys ...string) string {
		for _, key := range keys {
			value, ok := normalized[key]
			if !ok {
				continue
			}
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
		return ""
	}

	if cause := resolveText("cause", "case", "details", "reason", "error", "message"); cause != "" {
		if _, exists := normalized["cause"]; !exists {
			normalized["cause"] = cause
		}
		if _, exists := normalized["case"]; !exists {
			normalized["case"] = cause
		}
		if _, exists := normalized["details"]; !exists {
			normalized["details"] = cause
		}
	}

	return normalized
}

func pluginErrorKey(status int, message string) string {
	normalized := normalizePluginErrorKeyPart(message)
	return fmt.Sprintf("plugin.admin.http_%d.%s", status, normalized)
}

func normalizePluginErrorKeyPart(message string) string {
	lowered := strings.ToLower(strings.TrimSpace(message))
	if lowered == "" {
		return "unknown"
	}

	var builder strings.Builder
	builder.Grow(len(lowered))
	lastUnderscore := false
	for _, r := range lowered {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			builder.WriteByte('_')
			lastUnderscore = true
		}
	}

	normalized := strings.Trim(builder.String(), "_")
	if normalized == "" {
		return "unknown"
	}
	return normalized
}

// TestPlugin 测试插件连接
func (h *PluginHandler) TestPlugin(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, id).Error; err != nil {
		h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
		return
	}

	var req testPluginRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	checks := normalizePluginTestChecks(req.Checks)
	if len(checks) == 0 {
		// 兼容旧行为：不带请求体时仅执行健康检查。
		checks = []string{"health"}
	}

	execCtx := buildTestExecutionContext(c, req)
	startTime := time.Now()
	results := make([]pluginCheckResult, 0, len(checks))

	for _, check := range checks {
		checkStart := time.Now()
		result := pluginCheckResult{
			Check:   check,
			Success: false,
			Data:    map[string]interface{}{},
		}

		switch {
		case check == "health":
			health, healthErr := h.pluginManager.TestPlugin(id)
			if healthErr != nil {
				result.Error = healthErr.Error()
				break
			}
			result.Success = true
			result.Data["healthy"] = health.Healthy
			result.Data["version"] = health.Version
			result.Data["metadata"] = health.Metadata
		case strings.HasPrefix(check, "hook."):
			hookName := strings.TrimSpace(strings.TrimPrefix(check, "hook."))
			if hookName == "" {
				result.Error = "invalid hook check name"
				break
			}

			payload := buildTestHookPayload(req, check, hookName)
			hookResult, hookErr := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
				Hook:    hookName,
				Payload: payload,
			}, execCtx)
			if hookErr != nil {
				result.Error = hookErr.Error()
				result.Data["hook"] = hookName
				result.Data["payload"] = payload
				break
			}

			result.Data["hook"] = hookName
			result.Data["payload"] = payload
			if hookResult != nil {
				result.Data["blocked"] = hookResult.Blocked
				if strings.TrimSpace(hookResult.BlockReason) != "" {
					result.Data["block_reason"] = strings.TrimSpace(hookResult.BlockReason)
				}
				result.Data["frontend_extensions_count"] = len(hookResult.FrontendExtensions)
				result.Data["plugin_results_total"] = len(hookResult.PluginResults)
			}

			pluginResult := findPluginHookResultByID(hookResult, id)
			if pluginResult == nil {
				result.Skipped = true
				result.Error = fmt.Sprintf("plugin %d did not participate in hook %s (disabled / not subscribed / denied by capabilities)", id, hookName)
				if strings.EqualFold(hookName, "frontend.bootstrap") {
					area := normalizeFrontendBootstrapArea(parseStringFromAny(payload["area"]))
					menus, routes := collectFrontendBootstrapEntries(area, nil)
					result.Data["bootstrap_area"] = area
					result.Data["bootstrap_menu_count"] = len(menus)
					result.Data["bootstrap_route_count"] = len(routes)
				}
				break
			}

			result.Data["plugin_result"] = pluginResult
			if !pluginResult.Success {
				if strings.TrimSpace(pluginResult.Error) != "" {
					result.Error = strings.TrimSpace(pluginResult.Error)
				} else {
					result.Error = fmt.Sprintf("hook %s failed on plugin %d", hookName, id)
				}
				break
			}

			if strings.EqualFold(hookName, "frontend.bootstrap") {
				area := normalizeFrontendBootstrapArea(parseStringFromAny(payload["area"]))
				extensions := make([]service.FrontendExtension, 0)
				if hookResult != nil {
					extensions = hookResult.FrontendExtensions
				}
				menus, routes := collectFrontendBootstrapEntries(area, extensions)
				result.Data["bootstrap_area"] = area
				result.Data["bootstrap_menu_count"] = len(menus)
				result.Data["bootstrap_route_count"] = len(routes)
			}
			result.Success = true
		default:
			result.Error = fmt.Sprintf("unsupported check %q", check)
		}

		result.DurationMs = int(time.Since(checkStart).Milliseconds())
		results = append(results, result)
		if req.StopOnFailure && !result.Success {
			break
		}
	}

	passed := 0
	failed := 0
	for _, result := range results {
		if result.Success {
			passed++
		} else {
			failed++
		}
	}
	totalDuration := int(time.Since(startTime).Milliseconds())
	resp := gin.H{
		"success":      failed == 0,
		"plugin_id":    plugin.ID,
		"plugin_name":  plugin.Name,
		"total":        len(results),
		"passed":       passed,
		"failed":       failed,
		"duration_ms":  totalDuration,
		"stop_on_fail": req.StopOnFailure,
		"checks":       results,
	}
	if failed > 0 {
		summary := summarizePluginCheckFailures(results)
		resp = mergePluginFailurePayload(resp, buildPluginFailurePayload(http.StatusBadRequest, "plugin test failed", map[string]interface{}{
			"cause": summary,
		}))
		resp["success"] = false
	}

	c.JSON(http.StatusOK, resp)
}

// ExecutePlugin 执行插件动作
func (h *PluginHandler) ExecutePlugin(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	resourceID := id

	var req executePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	req.Action = strings.TrimSpace(req.Action)
	if req.Action == "" {
		h.respondPluginError(c, http.StatusBadRequest, "action is required")
		return
	}

	execCtx := &service.ExecutionContext{
		RequestContext: c.Request.Context(),
	}
	if req.Context != nil {
		execCtx.UserID = req.Context.UserID
		execCtx.OrderID = req.Context.OrderID
		execCtx.SessionID = req.Context.SessionID
		execCtx.Metadata = sanitizeUserProvidedExecutionMetadata(req.Context.Metadata)
	}
	execCtx = ensureOperatorUserID(c, execCtx)
	execCtx = enrichPluginExecutionContextWithRequestMetadata(execCtx, c)
	resolvedExecCtx, resolveErr := h.resolveAdminPluginRouteExecutionContext(
		c,
		id,
		req.Path,
		req.QueryParams,
		req.RouteParams,
		req.Action,
		false,
		execCtx,
	)
	if resolveErr != nil {
		h.logPluginOperation(c, "plugin_execute_failed", nil, &resourceID, map[string]interface{}{
			"success":        false,
			"execute_action": req.Action,
			"params_keys":    sortedStringMapKeys(req.Params),
			"path":           strings.TrimSpace(req.Path),
			"error":          strings.TrimSpace(resolveErr.Error()),
		})
		c.JSON(http.StatusOK, buildPluginExecuteFailurePayload(
			http.StatusBadRequest,
			"plugin execute failed",
			strings.TrimSpace(resolveErr.Error()),
			nil,
			nil,
		))
		return
	}
	taskID := service.EnsurePluginExecutionMetadata(resolvedExecCtx, false)
	taskLinks := buildAdminPluginExecutionTaskLinks(id, taskID)
	h.applyPluginExecutionHeaders(c, taskID)

	result, err := h.pluginManager.ExecutePlugin(id, req.Action, req.Params, resolvedExecCtx)
	if err != nil {
		failureMetadata := clonePluginExecutionMetadataWithStatus(resolvedExecCtx.Metadata, resolvePluginExecutionFailureStatus(err))
		h.logPluginOperation(c, "plugin_execute_failed", nil, &resourceID, map[string]interface{}{
			"success":        false,
			"execute_action": req.Action,
			"params_keys":    sortedStringMapKeys(req.Params),
			"path":           strings.TrimSpace(req.Path),
			"error":          strings.TrimSpace(err.Error()),
		})
		resp := buildPluginExecuteFailurePayload(
			http.StatusBadRequest,
			"plugin execute failed",
			strings.TrimSpace(err.Error()),
			nil,
			failureMetadata,
		)
		resp["task_status_url"] = taskLinks.StatusURL
		resp["task_cancel_url"] = taskLinks.CancelURL
		c.JSON(http.StatusOK, resp)
		return
	}

	resultErr := strings.TrimSpace(result.Error)
	if !result.Success {
		h.logPluginOperation(c, "plugin_execute_failed", nil, &resourceID, map[string]interface{}{
			"success":        false,
			"execute_action": req.Action,
			"params_keys":    sortedStringMapKeys(req.Params),
			"path":           strings.TrimSpace(req.Path),
			"result_error":   resultErr,
		})
	}
	resp := gin.H{
		"success":         result.Success,
		"task_id":         strings.TrimSpace(result.TaskID),
		"task_status_url": taskLinks.StatusURL,
		"task_cancel_url": taskLinks.CancelURL,
		"data":            result.Data,
		"metadata":        result.Metadata,
	}
	if !result.Success {
		resp = mergePluginFailurePayload(resp, buildPluginExecuteFailurePayload(
			http.StatusBadRequest,
			"plugin execute failed",
			resultErr,
			result.Data,
			result.Metadata,
		))
		resp["success"] = false
	}
	c.JSON(http.StatusOK, resp)
}

// ExecutePluginStream 流式执行插件动作
func (h *PluginHandler) ExecutePluginStream(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	resourceID := id

	var req executePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	req.Action = strings.TrimSpace(req.Action)
	if req.Action == "" {
		h.respondPluginError(c, http.StatusBadRequest, "action is required")
		return
	}

	execCtx := &service.ExecutionContext{
		RequestContext: c.Request.Context(),
	}
	if req.Context != nil {
		execCtx.UserID = req.Context.UserID
		execCtx.OrderID = req.Context.OrderID
		execCtx.SessionID = req.Context.SessionID
		execCtx.Metadata = sanitizeUserProvidedExecutionMetadata(req.Context.Metadata)
	}
	execCtx = ensureOperatorUserID(c, execCtx)
	execCtx = enrichPluginExecutionContextWithRequestMetadata(execCtx, c)
	resolvedExecCtx, resolveErr := h.resolveAdminPluginRouteExecutionContext(
		c,
		id,
		req.Path,
		req.QueryParams,
		req.RouteParams,
		req.Action,
		true,
		execCtx,
	)
	if resolveErr != nil {
		h.logPluginOperation(c, "plugin_execute_stream_failed", nil, &resourceID, map[string]interface{}{
			"success":        false,
			"execute_action": req.Action,
			"params_keys":    sortedStringMapKeys(req.Params),
			"path":           strings.TrimSpace(req.Path),
			"error":          strings.TrimSpace(resolveErr.Error()),
		})
		c.JSON(http.StatusOK, buildPluginExecuteFailurePayload(
			http.StatusBadRequest,
			"plugin execute failed",
			strings.TrimSpace(resolveErr.Error()),
			nil,
			nil,
		))
		return
	}
	taskID := service.EnsurePluginExecutionMetadata(resolvedExecCtx, true)
	taskLinks := buildAdminPluginExecutionTaskLinks(id, taskID)
	if err := h.startPluginNDJSONStream(c, taskID); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	if err := h.writePluginStreamTaskStarted(c, taskID, taskLinks, resolvedExecCtx.Metadata); err != nil {
		return
	}

	streamIndex := 0
	result, execErr := h.pluginManager.ExecutePluginStream(id, req.Action, req.Params, resolvedExecCtx, func(chunk *service.ExecutionStreamChunk) error {
		if chunk != nil {
			streamIndex = chunk.Index + 1
		}
		return h.writePluginStreamChunkWithLinks(c, chunk, taskLinks)
	})
	if execErr != nil {
		h.logPluginOperation(c, "plugin_execute_stream_failed", nil, &resourceID, map[string]interface{}{
			"success":        false,
			"execute_action": req.Action,
			"params_keys":    sortedStringMapKeys(req.Params),
			"path":           strings.TrimSpace(req.Path),
			"error":          strings.TrimSpace(execErr.Error()),
		})
		_ = h.writePluginStreamErrorWithLinks(
			c,
			streamIndex,
			execErr.Error(),
			taskID,
			taskLinks,
			clonePluginExecutionMetadataWithStatus(resolvedExecCtx.Metadata, resolvePluginExecutionFailureStatus(execErr)),
		)
		return
	}
	if result != nil && !result.Success {
		h.logPluginOperation(c, "plugin_execute_stream_failed", nil, &resourceID, map[string]interface{}{
			"success":        false,
			"execute_action": req.Action,
			"params_keys":    sortedStringMapKeys(req.Params),
			"path":           strings.TrimSpace(req.Path),
			"result_error":   strings.TrimSpace(result.Error),
		})
	}
}

// GetPluginExecutions 获取插件执行历史
func (h *PluginHandler) GetPluginExecutions(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}

	var executions []models.PluginExecution
	query := h.db.Where("plugin_id = ?", id).Order("created_at DESC").Limit(100)

	if err := query.Find(&executions).Error; err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to fetch executions")
		return
	}

	c.JSON(http.StatusOK, executions)
}

func (h *PluginHandler) activatePluginVersionInternal(pluginID, versionID uint, autoStart bool) (*models.Plugin, *models.PluginVersion, error) {
	return h.activatePluginVersionInternalWithDeploymentContext(
		pluginID,
		versionID,
		autoStart,
		"manual_hot_update",
		nil,
		"activate plugin version",
	)
}

func (h *PluginHandler) activatePluginVersionInternalWithDeploymentContext(
	pluginID,
	versionID uint,
	autoStart bool,
	trigger string,
	requestedBy *uint,
	detail string,
) (*models.Plugin, *models.PluginVersion, error) {
	var plugin models.Plugin
	if err := h.db.First(&plugin, pluginID).Error; err != nil {
		return nil, nil, err
	}
	originalPlugin := plugin

	var version models.PluginVersion
	if err := h.db.Where("id = ? AND plugin_id = ?", versionID, pluginID).First(&version).Error; err != nil {
		return nil, nil, err
	}
	if err := service.ValidatePluginManifestCompatibility(version.Manifest, firstNonEmpty(version.Runtime, plugin.Runtime)); err != nil {
		return nil, nil, fmt.Errorf("plugin manifest compatibility check failed: %w", err)
	}

	var previousActive models.PluginVersion
	hadPreviousActive := false
	if err := h.db.Where("plugin_id = ? AND is_active = ?", pluginID, true).First(&previousActive).Error; err == nil {
		hadPreviousActive = true
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, err
	}

	if _, migrationErr := h.migrateJSWorkerWritableFilesForActivation(&originalPlugin, &version); migrationErr != nil {
		return nil, nil, fmt.Errorf("migrate js_worker writable files failed: %w", migrationErr)
	}

	requestedGeneration := service.ResolveNextPluginGeneration(&originalPlugin)
	targetPlugin := buildActivatedPluginSnapshot(originalPlugin, version, requestedGeneration, "")
	targetRuntimeSpecHash := service.ComputePluginRuntimeSpecHash(&targetPlugin)
	if targetRuntimeSpecHash == "" {
		targetRuntimeSpecHash = service.ResolvePluginRuntimeSpecHash(&originalPlugin)
	}
	targetPlugin.RuntimeSpecHash = targetRuntimeSpecHash
	shouldStart := autoStart || originalPlugin.Enabled

	deployment, err := h.createPluginDeployment(
		pluginID,
		models.PluginDeploymentOperationHotUpdate,
		trigger,
		&versionID,
		requestedGeneration,
		targetRuntimeSpecHash,
		shouldStart,
		requestedBy,
		detail,
	)
	if err != nil {
		return nil, nil, err
	}
	if err := h.markPluginDeploymentRunning(deployment, detail); err != nil {
		return nil, nil, err
	}

	now := time.Now()
	update := map[string]interface{}{
		"version":            normalizeVersion(version.Version),
		"package_path":       version.PackagePath,
		"package_checksum":   version.PackageChecksum,
		"runtime_spec_hash":  targetRuntimeSpecHash,
		"desired_generation": requestedGeneration,
		"applied_generation": requestedGeneration,
		"lifecycle_status":   models.PluginLifecycleInstalled,
		"installed_at":       now,
		"last_error":         "",
		"retired_at":         nil,
	}
	if strings.TrimSpace(version.Type) != "" {
		update["type"] = version.Type
	}
	if strings.TrimSpace(version.Runtime) != "" {
		update["runtime"] = version.Runtime
	}
	if strings.TrimSpace(version.Address) != "" {
		update["address"] = version.Address
	}
	if strings.TrimSpace(version.ConfigSnapshot) != "" {
		update["config"] = version.ConfigSnapshot
	}
	if strings.TrimSpace(version.RuntimeParams) != "" {
		update["runtime_params"] = version.RuntimeParams
	}
	if strings.TrimSpace(version.CapabilitiesSnapshot) != "" {
		update["capabilities"] = version.CapabilitiesSnapshot
	}
	if strings.TrimSpace(version.Manifest) != "" {
		update["manifest"] = version.Manifest
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Plugin{}).Where("id = ?", pluginID).Updates(update).Error; err != nil {
			return err
		}

		if err := tx.Model(&models.PluginVersion{}).Where("plugin_id = ?", pluginID).Updates(map[string]interface{}{
			"is_active":        false,
			"lifecycle_status": models.PluginLifecycleUploaded,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.PluginVersion{}).Where("id = ?", versionID).Updates(map[string]interface{}{
			"is_active":        true,
			"activated_at":     now,
			"lifecycle_status": models.PluginLifecycleInstalled,
		}).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		_ = h.markPluginDeploymentFailed(deployment, err)
		return nil, nil, err
	}

	h.invalidateJSWorkerProgramCaches(&originalPlugin, &targetPlugin)
	if shouldStart {
		if err := h.pluginManager.StartPlugin(pluginID); err != nil {
			rollbackErr := h.rollbackActivatedVersion(pluginID, originalPlugin, previousActive, hadPreviousActive)
			if rollbackErr != nil {
				_ = h.markPluginDeploymentFailed(deployment, fmt.Errorf("start plugin failed: %v; rollback failed: %v", err, rollbackErr))
				return nil, nil, fmt.Errorf("start plugin failed: %v; rollback failed: %v", err, rollbackErr)
			}
			_ = h.markPluginDeploymentFailed(deployment, err)
			return nil, nil, fmt.Errorf("start plugin failed: %w", err)
		}
	}

	if err := h.db.First(&plugin, pluginID).Error; err != nil {
		return nil, nil, err
	}
	if err := h.db.First(&version, versionID).Error; err != nil {
		_ = h.markPluginDeploymentFailed(deployment, err)
		return nil, nil, err
	}
	h.invalidatePublicPluginCaches()
	successDetail := strings.TrimSpace(detail)
	if successDetail == "" {
		successDetail = "hot update applied"
	}
	_ = h.markPluginDeploymentSucceeded(deployment, requestedGeneration, successDetail)
	return &plugin, &version, nil
}

func (h *PluginHandler) rollbackActivatedVersion(pluginID uint, originalPlugin models.Plugin, previousActive models.PluginVersion, hadPreviousActive bool) error {
	restorePlugin := map[string]interface{}{
		"version":            normalizeVersion(originalPlugin.Version),
		"package_path":       originalPlugin.PackagePath,
		"package_checksum":   originalPlugin.PackageChecksum,
		"lifecycle_status":   originalPlugin.LifecycleStatus,
		"installed_at":       originalPlugin.InstalledAt,
		"last_error":         originalPlugin.LastError,
		"retired_at":         originalPlugin.RetiredAt,
		"type":               originalPlugin.Type,
		"runtime":            originalPlugin.Runtime,
		"address":            originalPlugin.Address,
		"config":             originalPlugin.Config,
		"runtime_params":     originalPlugin.RuntimeParams,
		"capabilities":       originalPlugin.Capabilities,
		"manifest":           originalPlugin.Manifest,
		"runtime_spec_hash":  originalPlugin.RuntimeSpecHash,
		"desired_generation": originalPlugin.DesiredGeneration,
		"applied_generation": originalPlugin.AppliedGeneration,
	}

	return h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Plugin{}).Where("id = ?", pluginID).Updates(restorePlugin).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.PluginVersion{}).Where("plugin_id = ?", pluginID).Update("is_active", false).Error; err != nil {
			return err
		}
		if !hadPreviousActive {
			return nil
		}

		restoreVersion := map[string]interface{}{
			"is_active":        true,
			"lifecycle_status": previousActive.LifecycleStatus,
			"activated_at":     previousActive.ActivatedAt,
		}
		if err := tx.Model(&models.PluginVersion{}).Where("id = ?", previousActive.ID).Updates(restoreVersion).Error; err != nil {
			return err
		}
		return nil
	})
}

func (h *PluginHandler) createVersionSnapshot(plugin *models.Plugin, reason string, active bool) (*models.PluginVersion, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	version := normalizeVersion(plugin.Version)
	record := &models.PluginVersion{
		PluginID:             plugin.ID,
		Version:              version,
		PackagePath:          plugin.PackagePath,
		PackageChecksum:      plugin.PackageChecksum,
		Manifest:             plugin.Manifest,
		Type:                 plugin.Type,
		Runtime:              plugin.Runtime,
		Address:              plugin.Address,
		ConfigSnapshot:       plugin.Config,
		RuntimeParams:        plugin.RuntimeParams,
		CapabilitiesSnapshot: plugin.Capabilities,
		Changelog:            reason,
		LifecycleStatus:      plugin.LifecycleStatus,
		IsActive:             active,
		UploadedBy:           nil,
	}
	if active {
		now := time.Now()
		record.ActivatedAt = &now
		if err := h.db.Model(&models.PluginVersion{}).Where("plugin_id = ?", plugin.ID).Update("is_active", false).Error; err != nil {
			return nil, err
		}
	}
	if err := h.db.Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func normalizePluginTestChecks(checks []string) []string {
	if len(checks) == 0 {
		return nil
	}

	out := make([]string, 0, len(checks))
	for _, item := range checks {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized == "" {
			continue
		}

		switch normalized {
		case "default":
			out = append(out, pluginTestDefaultChecks...)
			continue
		case "all":
			out = append(out, "health")
			out = append(out, pluginTestAllHookChecks...)
			continue
		case "health":
			out = append(out, "health")
			continue
		}

		normalized = strings.ReplaceAll(normalized, ":", ".")
		if !strings.HasPrefix(normalized, "hook.") {
			normalized = "hook." + strings.TrimPrefix(normalized, ".")
		}
		if normalized == "hook." {
			continue
		}
		out = append(out, normalized)
	}

	return normalizeLowerStringListValues(out)
}

func buildTestExecutionContext(c *gin.Context, req testPluginRequest) *service.ExecutionContext {
	locale, acceptLanguage := resolvePluginRequestLocaleMetadata(c)
	metadata := map[string]string{
		"request_path":    c.Request.URL.Path,
		"route":           c.FullPath(),
		"client_ip":       utils.GetRealIP(c),
		"user_agent":      c.GetHeader("User-Agent"),
		"accept_language": acceptLanguage,
	}
	if locale != "" {
		metadata["locale"] = locale
	}
	if req.Metadata != nil {
		for key, value := range sanitizeUserProvidedExecutionMetadata(req.Metadata) {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			metadata[key] = strings.TrimSpace(value)
		}
	}

	sessionID := strings.TrimSpace(c.GetHeader("X-Session-ID"))
	execCtx := &service.ExecutionContext{
		SessionID:      sessionID,
		Metadata:       metadata,
		RequestContext: c.Request.Context(),
	}

	if req.Context != nil {
		execCtx.SessionID = firstNonEmpty(req.Context.SessionID, sessionID)
		if req.Context.UserID != nil {
			userID := *req.Context.UserID
			execCtx.UserID = &userID
		}
		if req.Context.OrderID != nil {
			orderID := *req.Context.OrderID
			execCtx.OrderID = &orderID
		}
		if req.Context.Metadata != nil {
			for key, value := range sanitizeUserProvidedExecutionMetadata(req.Context.Metadata) {
				key = strings.TrimSpace(key)
				if key == "" {
					continue
				}
				execCtx.Metadata[key] = strings.TrimSpace(value)
			}
		}
	}

	execCtx = ensureOperatorUserID(c, execCtx)
	if execCtx.UserID == nil {
		if userID, ok := middleware.GetUserID(c); ok {
			value := userID
			execCtx.UserID = &value
		}
	}

	if execCtx.UserID == nil && execCtx.OrderID == nil && strings.TrimSpace(execCtx.SessionID) == "" {
		execCtx.SessionID = fmt.Sprintf("plugin-test-%d", time.Now().UnixNano())
	}
	return enrichPluginExecutionContextWithRequestMetadata(execCtx, c)
}

func buildTestHookPayload(req testPluginRequest, check string, hookName string) map[string]interface{} {
	payload := make(map[string]interface{})

	normalizedHook := strings.ToLower(strings.TrimSpace(hookName))
	switch normalizedHook {
	case "frontend.slot.render":
		path := normalizeFrontendBootstrapPath(firstNonEmpty(req.Path, "/admin/plugins"))
		slot := strings.TrimSpace(req.Slot)
		if slot == "" {
			if strings.HasPrefix(path, "/admin/") {
				slot = "admin.dashboard.top"
			} else {
				slot = "user.orders.top"
			}
		}
		payload["path"] = path
		payload["slot"] = strings.ToLower(strings.TrimSpace(slot))
	case "frontend.bootstrap":
		path := normalizeFrontendBootstrapPath(firstNonEmpty(req.Path, req.AdminBootstrapPath, req.UserBootstrapPath))
		area := frontendBootstrapAreaUser
		if strings.HasPrefix(path, "/admin/") || strings.TrimSpace(req.AdminBootstrapPath) != "" || strings.Contains(strings.ToLower(check), ".admin") {
			area = frontendBootstrapAreaAdmin
		}
		if area == frontendBootstrapAreaAdmin {
			path = normalizeFrontendBootstrapPath(firstNonEmpty(req.AdminBootstrapPath, path, "/admin"))
		} else {
			path = normalizeFrontendBootstrapPath(firstNonEmpty(req.UserBootstrapPath, path, "/"))
		}
		payload["area"] = area
		payload["path"] = path
		payload["slot"] = "bootstrap"
	default:
		payload["source"] = "plugin_test"
		payload["hook"] = normalizedHook
	}

	mergePayloadFromMap(payload, req.HookPayloads, check)
	mergePayloadFromMap(payload, req.HookPayloads, hookName)
	mergePayloadFromMap(payload, req.HookPayloads, "hook."+hookName)

	return payload
}

func mergePayloadFromMap(target map[string]interface{}, payloads map[string]map[string]interface{}, key string) {
	if target == nil || payloads == nil {
		return
	}
	payload, exists := payloads[strings.TrimSpace(key)]
	if !exists || payload == nil {
		return
	}
	for field, value := range payload {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		target[field] = value
	}
}

func findPluginHookResultByID(result *service.HookExecutionResult, pluginID uint) *service.HookPluginResult {
	if result == nil {
		return nil
	}
	for idx := range result.PluginResults {
		item := result.PluginResults[idx]
		if item.PluginID == pluginID {
			copyItem := item
			return &copyItem
		}
	}
	return nil
}

func (h *PluginHandler) resolveRuntime(runtime string) (string, error) {
	if h != nil && h.pluginManager != nil {
		return h.pluginManager.ResolveRuntime(runtime)
	}
	trimmed := strings.ToLower(strings.TrimSpace(runtime))
	if trimmed == "" {
		return "grpc", nil
	}
	switch trimmed {
	case "grpc", "js_worker":
		return trimmed, nil
	default:
		return "", fmt.Errorf("unsupported plugin runtime %q", trimmed)
	}
}

func (h *PluginHandler) validatePluginTypeAndRuntime(pluginType, runtime string) error {
	if strings.TrimSpace(pluginType) == "" {
		return fmt.Errorf("plugin type is required")
	}
	if h != nil && h.pluginManager != nil {
		return h.pluginManager.ValidatePluginProfile(runtime, pluginType)
	}
	return nil
}

func getOptionalUserID(c *gin.Context) *uint {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		return nil
	}
	value := userID
	return &value
}

func getOptionalOperatorUserID(c *gin.Context) *uint {
	if c == nil {
		return nil
	}
	if userID := getOptionalUserID(c); userID != nil {
		return userID
	}
	if middleware.IsAPIKeyAuth(c) {
		value := uint(0)
		return &value
	}
	return nil
}

func ensureOperatorUserID(c *gin.Context, execCtx *service.ExecutionContext) *service.ExecutionContext {
	if execCtx == nil {
		execCtx = &service.ExecutionContext{}
	}
	if execCtx.OperatorUserID == nil {
		execCtx.OperatorUserID = getOptionalOperatorUserID(c)
	}
	return execCtx
}

func cloneOptionalUint(value *uint) *uint {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func mergePluginExecutionContextDefaults(target *service.ExecutionContext, defaults *service.ExecutionContext) *service.ExecutionContext {
	if defaults == nil {
		if target == nil {
			return nil
		}
		return &service.ExecutionContext{
			OperatorUserID: cloneOptionalUint(target.OperatorUserID),
			UserID:         cloneOptionalUint(target.UserID),
			OrderID:        cloneOptionalUint(target.OrderID),
			SessionID:      strings.TrimSpace(target.SessionID),
			Metadata:       cloneStringMap(target.Metadata),
			RequestContext: target.RequestContext,
		}
	}
	if target == nil {
		return &service.ExecutionContext{
			OperatorUserID: cloneOptionalUint(defaults.OperatorUserID),
			UserID:         cloneOptionalUint(defaults.UserID),
			OrderID:        cloneOptionalUint(defaults.OrderID),
			SessionID:      strings.TrimSpace(defaults.SessionID),
			Metadata:       cloneStringMap(defaults.Metadata),
			RequestContext: defaults.RequestContext,
		}
	}

	merged := &service.ExecutionContext{
		OperatorUserID: cloneOptionalUint(target.OperatorUserID),
		UserID:         cloneOptionalUint(target.UserID),
		OrderID:        cloneOptionalUint(target.OrderID),
		SessionID:      strings.TrimSpace(target.SessionID),
		Metadata:       cloneStringMap(target.Metadata),
		RequestContext: target.RequestContext,
	}
	if merged.UserID == nil {
		merged.UserID = cloneOptionalUint(defaults.UserID)
	}
	if merged.OperatorUserID == nil {
		merged.OperatorUserID = cloneOptionalUint(defaults.OperatorUserID)
	}
	if merged.OrderID == nil {
		merged.OrderID = cloneOptionalUint(defaults.OrderID)
	}
	if merged.SessionID == "" {
		merged.SessionID = strings.TrimSpace(defaults.SessionID)
	}
	if merged.RequestContext == nil {
		merged.RequestContext = defaults.RequestContext
	}
	if merged.Metadata == nil {
		merged.Metadata = make(map[string]string)
	}
	for key, value := range defaults.Metadata {
		if _, exists := merged.Metadata[key]; exists {
			continue
		}
		merged.Metadata[key] = value
	}
	return merged
}

func (h *PluginHandler) resolveAdminPluginRouteExecutionContext(
	c *gin.Context,
	pluginID uint,
	path string,
	queryParams map[string]string,
	routeParams map[string]string,
	action string,
	requireStream bool,
	execCtx *service.ExecutionContext,
) (*service.ExecutionContext, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return execCtx, nil
	}
	_ = routeParams
	normalizedQueryParams := normalizeFrontendParamMap(queryParams)

	normalizedPath := normalizeFrontendBootstrapPath(trimmedPath)
	if len(normalizedPath) > 500 {
		return nil, fmt.Errorf("path is too long")
	}
	if !isAllowedPluginPagePath(frontendBootstrapAreaAdmin, normalizedPath) {
		return nil, fmt.Errorf("plugin page execute path is invalid")
	}

	scope := h.resolvePluginAccessScope(c)
	sessionID := strings.TrimSpace(c.GetHeader("X-Session-ID"))
	execUserID := getOptionalUserID(c)
	route, err := h.resolveAccessibleFrontendPluginRoute(
		c,
		frontendBootstrapAreaAdmin,
		normalizedPath,
		pluginID,
		normalizedQueryParams,
		scope,
		sessionID,
		execUserID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve plugin page route: %w", err)
	}
	if route == nil {
		return nil, fmt.Errorf("plugin page route is unavailable for current request scope")
	}
	if route.ExecuteAPI == nil {
		return nil, fmt.Errorf("plugin page execute api is unavailable for current route or current capabilities")
	}
	if !frontendRouteAllowsExecuteAction(route, action) {
		return nil, fmt.Errorf("plugin execute action is not declared for this page route")
	}
	if requireStream && !frontendRouteAllowsStreamAction(route, action) {
		return nil, fmt.Errorf("plugin stream action is not declared for this page route")
	}

	resolvedRouteParams := cloneStringMap(route.RouteParams)
	if len(resolvedRouteParams) == 0 {
		if _, matchedRouteParams := frontendBootstrapRouteMatch(route.Path, normalizedPath); len(matchedRouteParams) > 0 {
			resolvedRouteParams = matchedRouteParams
		}
	}

	routeExecCtx := h.buildFrontendPluginExecutionContext(
		c,
		scope,
		frontendBootstrapAreaAdmin,
		normalizedPath,
		normalizedQueryParams,
		resolvedRouteParams,
		sessionID,
		execUserID,
	)
	routeExecCtx.Metadata["plugin_page_route"] = route.Path
	routeExecCtx.Metadata["plugin_page_plugin_id"] = strconv.FormatUint(uint64(route.PluginID), 10)
	routeExecCtx.Metadata["plugin_page_action"] = strings.ToLower(strings.TrimSpace(action))

	return mergePluginExecutionContextDefaults(execCtx, routeExecCtx), nil
}
