package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	pathpkg "path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"auralogic/internal/config"
	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/pkg/response"
	"auralogic/internal/pkg/utils"
	"auralogic/internal/pluginobs"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

type frontendBootstrapMenuItem struct {
	ID                  string   `json:"id,omitempty"`
	Area                string   `json:"area,omitempty"`
	Title               string   `json:"title,omitempty"`
	Path                string   `json:"path,omitempty"`
	Icon                string   `json:"icon,omitempty"`
	Priority            int      `json:"priority,omitempty"`
	RequiredPermissions []string `json:"required_permissions,omitempty"`
	SuperAdminOnly      bool     `json:"super_admin_only,omitempty"`
	GuestVisible        bool     `json:"guest_visible,omitempty"`
	MobileVisible       bool     `json:"mobile_visible,omitempty"`
	PluginID            uint     `json:"plugin_id,omitempty"`
	PluginName          string   `json:"plugin_name,omitempty"`
}

type frontendBootstrapRouteItem struct {
	ID                   string                   `json:"id,omitempty"`
	Area                 string                   `json:"area,omitempty"`
	Title                string                   `json:"title,omitempty"`
	Path                 string                   `json:"path,omitempty"`
	RouteParams          map[string]string        `json:"route_params,omitempty"`
	Priority             int                      `json:"priority,omitempty"`
	RequiredPermissions  []string                 `json:"required_permissions,omitempty"`
	SuperAdminOnly       bool                     `json:"super_admin_only,omitempty"`
	GuestVisible         bool                     `json:"guest_visible,omitempty"`
	HTMLMode             string                   `json:"html_mode,omitempty"`
	Page                 map[string]interface{}   `json:"page,omitempty"`
	ExecuteAPI           *frontendRouteExecuteAPI `json:"execute_api,omitempty"`
	AllowedActions       []string                 `json:"-"`
	AllowedStreamActions []string                 `json:"-"`
	PluginID             uint                     `json:"plugin_id,omitempty"`
	PluginName           string                   `json:"plugin_name,omitempty"`
}

type frontendRouteExecuteAPI struct {
	URL            string   `json:"url,omitempty"`
	Method         string   `json:"method,omitempty"`
	Scope          string   `json:"scope,omitempty"`
	RequiresAuth   bool     `json:"requires_auth,omitempty"`
	PathParam      string   `json:"path_param,omitempty"`
	ActionParam    string   `json:"action_param,omitempty"`
	ParamsFormat   string   `json:"params_format,omitempty"`
	AllowedActions []string `json:"allowed_actions,omitempty"`
	StreamURL      string   `json:"stream_url,omitempty"`
	StreamFormat   string   `json:"stream_format,omitempty"`
	StreamActions  []string `json:"stream_actions,omitempty"`
}

type frontendExecutePluginRequest struct {
	Action      string            `json:"action" binding:"required"`
	Params      map[string]string `json:"params"`
	Path        string            `json:"path" binding:"required"`
	QueryParams map[string]string `json:"query_params"`
	RouteParams map[string]string `json:"route_params"`
}

type frontendExtensionsResponseData struct {
	Path       string                      `json:"path"`
	Slot       string                      `json:"slot"`
	Extensions []service.FrontendExtension `json:"extensions"`
}

type frontendExtensionsBatchItemRequest struct {
	Key         string                 `json:"key"`
	Slot        string                 `json:"slot" binding:"required"`
	Path        string                 `json:"path"`
	QueryParams map[string]string      `json:"query_params"`
	HostContext map[string]interface{} `json:"host_context"`
}

type frontendExtensionsBatchRequest struct {
	Path  string                               `json:"path"`
	Items []frontendExtensionsBatchItemRequest `json:"items" binding:"required"`
}

type frontendExtensionsBatchItemResponse struct {
	Key        string                      `json:"key,omitempty"`
	Path       string                      `json:"path"`
	Slot       string                      `json:"slot"`
	Extensions []service.FrontendExtension `json:"extensions"`
}

type frontendExtensionsBatchResponseData struct {
	Items []frontendExtensionsBatchItemResponse `json:"items"`
}

type frontendExtensionsRequest struct {
	Slot        string
	Path        string
	QueryParams map[string]string
	HostContext map[string]interface{}
}

type frontendExtensionsRuntimeContext struct {
	Scope           pluginAccessScope
	SessionID       string
	ExecUserID      *uint
	OperatorUserID  *uint
	CacheVaryKey    string
	RequestPath     string
	Route           string
	ClientIP        string
	UserAgent       string
	Locale          string
	AcceptLanguage  string
	RequestContext  context.Context
	CapabilityCache *frontendPluginCapabilityResolveCache
	RequestCache    *service.ExecutionRequestCache
}

type frontendPluginCapabilityResolveCache struct {
	mu                 sync.Mutex
	htmlModes          map[uint]string
	executeAPIByID     map[uint]bool
	htmlModeInflight   map[uint]*frontendStringResolveCall
	executeAPIInflight map[uint]*frontendBoolResolveCall
}

type frontendStringResolveCall struct {
	done chan struct{}
}

type frontendBoolResolveCall struct {
	done chan struct{}
}

type frontendBootstrapResponseData struct {
	Area   string                       `json:"area"`
	Path   string                       `json:"path"`
	Menus  []frontendBootstrapMenuItem  `json:"menus"`
	Routes []frontendBootstrapRouteItem `json:"routes"`
}

func resolvePluginPublicCacheSubjectKey(scope pluginAccessScope, userID *uint) string {
	if userID != nil && *userID > 0 {
		return fmt.Sprintf("user:%d", *userID)
	}
	if scope.superAdmin {
		return "super_admin"
	}
	if scope.authenticated {
		return "authenticated"
	}
	return "guest"
}

func resolvePluginPublicCacheVaryKey(c *gin.Context, scope pluginAccessScope, userID *uint, sessionID string) string {
	if c == nil {
		return buildPublicRequestCacheVaryKey(resolvePluginPublicCacheSubjectKey(scope, userID), sessionID, "", "", "")
	}
	return buildPublicRequestCacheVaryKey(
		resolvePluginPublicCacheSubjectKey(scope, userID),
		sessionID,
		resolvePluginPublicCacheLocaleVaryKey(c),
		utils.GetRealIP(c),
		c.GetHeader("User-Agent"),
	)
}

func normalizeFrontendParamMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	normalized := make(map[string]string, len(values))
	for key, value := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		normalized[trimmedKey] = value
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func parseFrontendQueryParams(raw string) (map[string]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	var values map[string]string
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, fmt.Errorf("query_params must be a JSON object: %w", err)
	}
	return normalizeFrontendParamMap(values), nil
}

func marshalFrontendParamMapJSON(values map[string]string) string {
	normalized := normalizeFrontendParamMap(values)
	if len(normalized) == 0 {
		return "{}"
	}
	body, err := json.Marshal(normalized)
	if err != nil {
		return "{}"
	}
	return string(body)
}

const (
	maxFrontendHostContextBytes       = 8 * 1024
	maxFrontendExtensionsBatchItems   = 64
	maxFrontendExtensionsBatchWorkers = 8
)

func newFrontendPluginCapabilityResolveCache() *frontendPluginCapabilityResolveCache {
	return &frontendPluginCapabilityResolveCache{
		htmlModes:          make(map[uint]string),
		executeAPIByID:     make(map[uint]bool),
		htmlModeInflight:   make(map[uint]*frontendStringResolveCall),
		executeAPIInflight: make(map[uint]*frontendBoolResolveCall),
	}
}

func normalizeFrontendPluginIDs(pluginIDs []uint) []uint {
	if len(pluginIDs) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(pluginIDs))
	out := make([]uint, 0, len(pluginIDs))
	for _, id := range pluginIDs {
		if id == 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (c *frontendPluginCapabilityResolveCache) resolveHTMLModes(
	pluginIDs []uint,
	resolver func([]uint) map[uint]string,
) map[uint]string {
	normalizedIDs := normalizeFrontendPluginIDs(pluginIDs)
	modes := make(map[uint]string, len(normalizedIDs))
	for _, id := range normalizedIDs {
		modes[id] = pluginFrontendHTMLModeSanitize
	}
	if len(normalizedIDs) == 0 {
		return modes
	}
	if c == nil {
		for id, mode := range resolver(normalizedIDs) {
			modes[id] = normalizePluginFrontendHTMLMode(mode)
		}
		return modes
	}

	c.mu.Lock()
	waitCalls := make([]*frontendStringResolveCall, 0, len(normalizedIDs))
	missing := make([]uint, 0, len(normalizedIDs))
	for _, id := range normalizedIDs {
		if mode, exists := c.htmlModes[id]; exists {
			pluginobs.RecordFrontendResolverCacheHit("html_mode", 1)
			modes[id] = normalizePluginFrontendHTMLMode(mode)
			continue
		}
		if call, exists := c.htmlModeInflight[id]; exists && call != nil {
			pluginobs.RecordFrontendResolverSingleflightWait("html_mode", 1)
			waitCalls = append(waitCalls, call)
			continue
		}
		pluginobs.RecordFrontendResolverCacheMiss("html_mode", 1)
		call := &frontendStringResolveCall{done: make(chan struct{})}
		c.htmlModeInflight[id] = call
		missing = append(missing, id)
	}
	c.mu.Unlock()

	if len(missing) > 0 {
		resolved := resolver(missing)
		c.mu.Lock()
		for _, id := range missing {
			mode := pluginFrontendHTMLModeSanitize
			if resolvedMode, exists := resolved[id]; exists {
				mode = normalizePluginFrontendHTMLMode(resolvedMode)
			}
			c.htmlModes[id] = mode
			modes[id] = mode
			if call, exists := c.htmlModeInflight[id]; exists && call != nil {
				close(call.done)
				delete(c.htmlModeInflight, id)
			}
		}
		c.mu.Unlock()
	}

	for _, call := range uniqueFrontendStringResolveCalls(waitCalls) {
		<-call.done
	}

	c.mu.Lock()
	for _, id := range normalizedIDs {
		if mode, exists := c.htmlModes[id]; exists {
			modes[id] = normalizePluginFrontendHTMLMode(mode)
		}
	}
	c.mu.Unlock()
	return modes
}

func (c *frontendPluginCapabilityResolveCache) resolveExecuteAPIAvailability(
	pluginIDs []uint,
	resolver func([]uint) map[uint]bool,
) map[uint]bool {
	normalizedIDs := normalizeFrontendPluginIDs(pluginIDs)
	availability := make(map[uint]bool, len(normalizedIDs))
	if len(normalizedIDs) == 0 {
		return availability
	}
	if c == nil {
		for id, allowed := range resolver(normalizedIDs) {
			availability[id] = allowed
		}
		for _, id := range normalizedIDs {
			if _, exists := availability[id]; !exists {
				availability[id] = false
			}
		}
		return availability
	}

	c.mu.Lock()
	waitCalls := make([]*frontendBoolResolveCall, 0, len(normalizedIDs))
	missing := make([]uint, 0, len(normalizedIDs))
	for _, id := range normalizedIDs {
		if allowed, exists := c.executeAPIByID[id]; exists {
			pluginobs.RecordFrontendResolverCacheHit("execute_api", 1)
			availability[id] = allowed
			continue
		}
		if call, exists := c.executeAPIInflight[id]; exists && call != nil {
			pluginobs.RecordFrontendResolverSingleflightWait("execute_api", 1)
			waitCalls = append(waitCalls, call)
			continue
		}
		pluginobs.RecordFrontendResolverCacheMiss("execute_api", 1)
		call := &frontendBoolResolveCall{done: make(chan struct{})}
		c.executeAPIInflight[id] = call
		missing = append(missing, id)
	}
	c.mu.Unlock()

	if len(missing) > 0 {
		resolved := resolver(missing)
		c.mu.Lock()
		for _, id := range missing {
			allowed := resolved[id]
			c.executeAPIByID[id] = allowed
			availability[id] = allowed
			if call, exists := c.executeAPIInflight[id]; exists && call != nil {
				close(call.done)
				delete(c.executeAPIInflight, id)
			}
		}
		c.mu.Unlock()
	}

	for _, call := range uniqueFrontendBoolResolveCalls(waitCalls) {
		<-call.done
	}

	c.mu.Lock()
	for _, id := range normalizedIDs {
		if allowed, exists := c.executeAPIByID[id]; exists {
			availability[id] = allowed
		}
	}
	c.mu.Unlock()
	return availability
}

func uniqueFrontendStringResolveCalls(calls []*frontendStringResolveCall) []*frontendStringResolveCall {
	if len(calls) == 0 {
		return nil
	}
	seen := make(map[*frontendStringResolveCall]struct{}, len(calls))
	out := make([]*frontendStringResolveCall, 0, len(calls))
	for _, call := range calls {
		if call == nil {
			continue
		}
		if _, exists := seen[call]; exists {
			continue
		}
		seen[call] = struct{}{}
		out = append(out, call)
	}
	return out
}

func uniqueFrontendBoolResolveCalls(calls []*frontendBoolResolveCall) []*frontendBoolResolveCall {
	if len(calls) == 0 {
		return nil
	}
	seen := make(map[*frontendBoolResolveCall]struct{}, len(calls))
	out := make([]*frontendBoolResolveCall, 0, len(calls))
	for _, call := range calls {
		if call == nil {
			continue
		}
		if _, exists := seen[call]; exists {
			continue
		}
		seen[call] = struct{}{}
		out = append(out, call)
	}
	return out
}

func normalizeFrontendHostContextValue(value interface{}) (interface{}, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, true
	case string, bool,
		float64, float32,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return typed, true
	case []interface{}:
		normalized := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			value, ok := normalizeFrontendHostContextValue(item)
			if !ok {
				continue
			}
			normalized = append(normalized, value)
		}
		return normalized, true
	case map[string]interface{}:
		if len(typed) == 0 {
			return map[string]interface{}{}, true
		}
		normalized := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey == "" {
				continue
			}
			value, ok := normalizeFrontendHostContextValue(item)
			if !ok {
				continue
			}
			normalized[trimmedKey] = value
		}
		if len(normalized) == 0 {
			return map[string]interface{}{}, true
		}
		return normalized, true
	default:
		return nil, false
	}
}

func normalizeFrontendHostContext(values map[string]interface{}) map[string]interface{} {
	if len(values) == 0 {
		return nil
	}

	normalized := make(map[string]interface{}, len(values))
	for key, value := range values {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		normalizedValue, ok := normalizeFrontendHostContextValue(value)
		if !ok {
			continue
		}
		normalized[trimmedKey] = normalizedValue
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func parseFrontendHostContext(raw string) (map[string]interface{}, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	if len(trimmed) > maxFrontendHostContextBytes {
		return nil, fmt.Errorf("host_context is too long")
	}

	var values map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil, fmt.Errorf("host_context must be a JSON object: %w", err)
	}
	return normalizeFrontendHostContext(values), nil
}

func marshalFrontendHostContextJSON(values map[string]interface{}) string {
	normalized := normalizeFrontendHostContext(values)
	if len(normalized) == 0 {
		return "{}"
	}
	body, err := json.Marshal(normalized)
	if err != nil {
		return "{}"
	}
	return string(body)
}

func buildFrontendQueryString(values map[string]string) string {
	normalized := normalizeFrontendParamMap(values)
	if len(normalized) == 0 {
		return ""
	}

	query := url.Values{}
	for key, value := range normalized {
		query.Set(key, value)
	}
	return query.Encode()
}

func buildFrontendFullPath(path string, queryParams map[string]string) string {
	normalizedPath := normalizeFrontendBootstrapPath(path)
	queryString := buildFrontendQueryString(queryParams)
	if queryString == "" {
		return normalizedPath
	}
	return normalizedPath + "?" + queryString
}

func normalizeFrontendExtensionsRequest(
	slot string,
	path string,
	queryParams map[string]string,
	hostContext map[string]interface{},
	publicEndpoint bool,
) (frontendExtensionsRequest, int, error) {
	normalized := frontendExtensionsRequest{
		Slot:        strings.TrimSpace(slot),
		Path:        strings.TrimSpace(path),
		QueryParams: normalizeFrontendParamMap(queryParams),
		HostContext: normalizeFrontendHostContext(hostContext),
	}
	if normalized.HostContext == nil {
		normalized.HostContext = map[string]interface{}{}
	}
	if len(marshalFrontendHostContextJSON(normalized.HostContext)) > maxFrontendHostContextBytes {
		return frontendExtensionsRequest{}, http.StatusBadRequest, fmt.Errorf("host_context is too long")
	}
	if normalized.Slot == "" {
		normalized.Slot = "default"
	}
	if normalized.Path == "" {
		normalized.Path = "/"
	}
	if len(normalized.Slot) > 100 {
		return frontendExtensionsRequest{}, http.StatusBadRequest, fmt.Errorf("slot is too long")
	}
	if len(normalized.Path) > 500 {
		return frontendExtensionsRequest{}, http.StatusBadRequest, fmt.Errorf("path is too long")
	}
	normalized.Path = normalizeFrontendBootstrapPath(normalized.Path)
	if publicEndpoint && strings.HasPrefix(normalizedSlotValue(normalized.Slot), "admin.") {
		return frontendExtensionsRequest{}, http.StatusForbidden, fmt.Errorf("admin slots are not available in public endpoint")
	}
	return normalized, http.StatusOK, nil
}

func (h *PluginHandler) buildFrontendExtensionsRuntimeContext(c *gin.Context) frontendExtensionsRuntimeContext {
	runtimeCtx := frontendExtensionsRuntimeContext{
		RequestPath:     "/",
		Route:           "/",
		CapabilityCache: newFrontendPluginCapabilityResolveCache(),
		RequestCache:    service.NewExecutionRequestCache(),
	}
	if c == nil {
		return runtimeCtx
	}

	runtimeCtx.Scope = h.resolvePluginAccessScope(c)
	runtimeCtx.SessionID = strings.TrimSpace(c.GetHeader("X-Session-ID"))
	if userID, ok := middleware.GetUserID(c); ok {
		runtimeCtx.ExecUserID = &userID
	}
	runtimeCtx.OperatorUserID = getOptionalOperatorUserID(c)
	runtimeCtx.CacheVaryKey = resolvePluginPublicCacheVaryKey(c, runtimeCtx.Scope, runtimeCtx.ExecUserID, runtimeCtx.SessionID)
	if c.Request != nil {
		runtimeCtx.RequestPath = c.Request.URL.Path
		runtimeCtx.RequestContext = c.Request.Context()
	}
	if runtimeCtx.RequestPath == "" {
		runtimeCtx.RequestPath = "/"
	}
	runtimeCtx.Route = c.FullPath()
	if runtimeCtx.Route == "" {
		runtimeCtx.Route = runtimeCtx.RequestPath
	}
	runtimeCtx.ClientIP = utils.GetRealIP(c)
	runtimeCtx.UserAgent = c.GetHeader("User-Agent")
	runtimeCtx.Locale, runtimeCtx.AcceptLanguage = resolvePluginRequestLocaleMetadata(c)
	return runtimeCtx
}

func buildFrontendExtensionsRequestKey(req frontendExtensionsRequest) string {
	return fmt.Sprintf(
		"slot=%s|path=%s|query=%s|host_context=%s",
		req.Slot,
		req.Path,
		marshalFrontendParamMapJSON(req.QueryParams),
		marshalFrontendHostContextJSON(req.HostContext),
	)
}

func resolveFrontendExtensionsBatchWorkers(total int) int {
	if total <= 0 {
		return 0
	}
	if total > maxFrontendExtensionsBatchWorkers {
		return maxFrontendExtensionsBatchWorkers
	}
	return total
}

func (h *PluginHandler) resolveFrontendExtensions(
	runtimeCtx frontendExtensionsRuntimeContext,
	req frontendExtensionsRequest,
	publicEndpoint bool,
) (frontendExtensionsResponseData, int, error) {
	if publicEndpoint {
		if cached, ok := h.getCachedPublicExtensions(req.Slot, req.Path, req.QueryParams, req.HostContext, runtimeCtx.Scope, runtimeCtx.CacheVaryKey); ok {
			return cached, http.StatusOK, nil
		}
	}

	payload := map[string]interface{}{
		"slot":         req.Slot,
		"path":         req.Path,
		"query_params": req.QueryParams,
		"query_string": buildFrontendQueryString(req.QueryParams),
		"full_path":    buildFrontendFullPath(req.Path, req.QueryParams),
		"host_context": req.HostContext,
	}

	metadata := map[string]string{
		"request_path":    runtimeCtx.RequestPath,
		"route":           runtimeCtx.Route,
		"client_ip":       runtimeCtx.ClientIP,
		"user_agent":      runtimeCtx.UserAgent,
		"accept_language": runtimeCtx.AcceptLanguage,
	}
	if strings.TrimSpace(runtimeCtx.Locale) != "" {
		metadata["locale"] = strings.TrimSpace(runtimeCtx.Locale)
	}
	applyPluginAccessScopeMetadata(metadata, runtimeCtx.Scope)
	execCtx := &service.ExecutionContext{
		SessionID:      runtimeCtx.SessionID,
		Metadata:       metadata,
		RequestContext: runtimeCtx.RequestContext,
		RequestCache:   runtimeCtx.RequestCache,
	}
	if runtimeCtx.ExecUserID != nil {
		execCtx.UserID = runtimeCtx.ExecUserID
	}
	if runtimeCtx.OperatorUserID != nil {
		execCtx.OperatorUserID = cloneOptionalUint(runtimeCtx.OperatorUserID)
	}

	var extensions []service.FrontendExtension
	if h.pluginManager != nil {
		hookResult, err := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
			Hook:    "frontend.slot.render",
			Payload: payload,
		}, execCtx)
		if err != nil {
			return frontendExtensionsResponseData{}, http.StatusInternalServerError, fmt.Errorf("Failed to load plugin extensions")
		}
		if hookResult != nil && len(hookResult.FrontendExtensions) > 0 {
			extensions = hookResult.FrontendExtensions
		}
	}
	extensions = filterFrontendExtensionsByScope(extensions, req.Slot, runtimeCtx.Scope, publicEndpoint)
	htmlModes := h.resolvePluginFrontendHTMLModes(
		collectFrontendExtensionPluginIDs(extensions),
		runtimeCtx.CapabilityCache,
	)
	extensions = annotateFrontendExtensionsHTMLMode(extensions, htmlModes)

	respPayload := frontendExtensionsResponseData{
		Path:       req.Path,
		Slot:       req.Slot,
		Extensions: extensions,
	}
	if publicEndpoint {
		h.cachePublicExtensions(respPayload, req.QueryParams, req.HostContext, runtimeCtx.Scope, runtimeCtx.CacheVaryKey)
	}
	return respPayload, http.StatusOK, nil
}

func (h *PluginHandler) getFrontendExtensions(c *gin.Context, publicEndpoint bool) {
	slot := strings.TrimSpace(c.Query("slot"))
	path := strings.TrimSpace(c.Query("path"))
	queryParams, err := parseFrontendQueryParams(c.Query("query_params"))
	if err != nil {
		h.respondPluginError(c, http.StatusBadRequest, err.Error())
		return
	}
	hostContext, err := parseFrontendHostContext(c.Query("host_context"))
	if err != nil {
		h.respondPluginError(c, http.StatusBadRequest, err.Error())
		return
	}
	req, status, err := normalizeFrontendExtensionsRequest(slot, path, queryParams, hostContext, publicEndpoint)
	if err != nil {
		h.respondPluginError(c, status, err.Error())
		return
	}
	pluginobs.RecordFrontendSlotRequest()
	runtimeCtx := h.buildFrontendExtensionsRuntimeContext(c)
	respPayload, status, err := h.resolveFrontendExtensions(runtimeCtx, req, publicEndpoint)
	if err != nil {
		h.respondPluginError(c, status, err.Error())
		return
	}
	response.Success(c, respPayload)
}

// GetPublicExtensions 获取前端页面可渲染的插件扩展（公开接口）
func (h *PluginHandler) GetPublicExtensions(c *gin.Context) {
	h.getFrontendExtensions(c, true)
}

// GetAdminExtensions 获取管理端前端页面可渲染的插件扩展（管理员接口）
func (h *PluginHandler) GetAdminExtensions(c *gin.Context) {
	h.getFrontendExtensions(c, false)
}

// GetPublicExtensionsBatch 批量获取前端页面可渲染的插件扩展（公开接口）
func (h *PluginHandler) GetPublicExtensionsBatch(c *gin.Context) {
	h.getFrontendExtensionsBatch(c, true)
}

// GetAdminExtensionsBatch 批量获取管理端前端页面可渲染的插件扩展（管理员接口）
func (h *PluginHandler) GetAdminExtensionsBatch(c *gin.Context) {
	h.getFrontendExtensionsBatch(c, false)
}

func (h *PluginHandler) getFrontendExtensionsBatch(c *gin.Context, publicEndpoint bool) {
	var req frontendExtensionsBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	if len(req.Items) == 0 {
		h.respondPluginError(c, http.StatusBadRequest, "items is required")
		return
	}
	if len(req.Items) > maxFrontendExtensionsBatchItems {
		h.respondPluginError(c, http.StatusBadRequest, "too many batch items")
		return
	}

	runtimeCtx := h.buildFrontendExtensionsRuntimeContext(c)
	defaultPath := strings.TrimSpace(req.Path)
	items := make([]frontendExtensionsBatchItemResponse, len(req.Items))
	normalizedRequests := make([]frontendExtensionsRequest, len(req.Items))
	requestIndexesByKey := make(map[string][]int, len(req.Items))
	uniqueRequests := make([]frontendExtensionsRequest, 0, len(req.Items))
	uniqueRequestIndexes := make([]int, 0, len(req.Items))
	for idx, item := range req.Items {
		normalizedReq, status, err := normalizeFrontendExtensionsRequest(
			item.Slot,
			firstNonEmpty(strings.TrimSpace(item.Path), defaultPath),
			item.QueryParams,
			item.HostContext,
			publicEndpoint,
		)
		if err != nil {
			h.respondPluginError(c, status, err.Error())
			return
		}
		normalizedRequests[idx] = normalizedReq
		requestKey := buildFrontendExtensionsRequestKey(normalizedReq)
		if _, exists := requestIndexesByKey[requestKey]; !exists {
			uniqueRequests = append(uniqueRequests, normalizedReq)
			uniqueRequestIndexes = append(uniqueRequestIndexes, idx)
		}
		requestIndexesByKey[requestKey] = append(requestIndexesByKey[requestKey], idx)
		items[idx] = frontendExtensionsBatchItemResponse{
			Key:        strings.TrimSpace(item.Key),
			Path:       normalizedReq.Path,
			Slot:       normalizedReq.Slot,
			Extensions: []service.FrontendExtension{},
		}
	}
	pluginobs.RecordFrontendBatchRequest(len(req.Items), len(uniqueRequests))

	type frontendBatchResolveResult struct {
		payload frontendExtensionsResponseData
		status  int
		err     error
	}

	resultsByKey := make(map[string]frontendBatchResolveResult, len(uniqueRequests))
	if len(uniqueRequests) == 1 {
		requestKey := buildFrontendExtensionsRequestKey(uniqueRequests[0])
		respPayload, status, err := h.resolveFrontendExtensions(runtimeCtx, uniqueRequests[0], publicEndpoint)
		resultsByKey[requestKey] = frontendBatchResolveResult{
			payload: respPayload,
			status:  status,
			err:     err,
		}
	} else {
		var (
			mu sync.Mutex
			wg sync.WaitGroup
		)
		jobs := make(chan int)
		workerCount := resolveFrontendExtensionsBatchWorkers(len(uniqueRequests))
		for worker := 0; worker < workerCount; worker++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for uniqueIdx := range jobs {
					request := uniqueRequests[uniqueIdx]
					requestKey := buildFrontendExtensionsRequestKey(request)
					respPayload, status, err := h.resolveFrontendExtensions(runtimeCtx, request, publicEndpoint)
					mu.Lock()
					resultsByKey[requestKey] = frontendBatchResolveResult{
						payload: respPayload,
						status:  status,
						err:     err,
					}
					mu.Unlock()
				}
			}()
		}
		for uniqueIdx := range uniqueRequests {
			jobs <- uniqueIdx
		}
		close(jobs)
		wg.Wait()
	}

	for _, originalIdx := range uniqueRequestIndexes {
		requestKey := buildFrontendExtensionsRequestKey(normalizedRequests[originalIdx])
		result, ok := resultsByKey[requestKey]
		if !ok {
			h.respondPluginError(c, http.StatusInternalServerError, "Failed to load plugin extensions")
			return
		}
		if result.err != nil {
			h.respondPluginError(c, result.status, result.err.Error())
			return
		}
		for _, itemIdx := range requestIndexesByKey[requestKey] {
			items[itemIdx].Path = result.payload.Path
			items[itemIdx].Slot = result.payload.Slot
			items[itemIdx].Extensions = cloneFrontendExtensions(result.payload.Extensions)
		}
	}

	response.Success(c, frontendExtensionsBatchResponseData{
		Items: items,
	})
}

// GetPublicFrontendBootstrap 获取用户端前端插件 bootstrap（公开接口）
func (h *PluginHandler) GetPublicFrontendBootstrap(c *gin.Context) {
	h.handleFrontendBootstrap(c, frontendBootstrapAreaUser, true)
}

// GetAdminFrontendBootstrap 获取管理端前端插件 bootstrap（需管理员鉴权）
func (h *PluginHandler) GetAdminFrontendBootstrap(c *gin.Context) {
	h.handleFrontendBootstrap(c, frontendBootstrapAreaAdmin, false)
}

// ExecutePublicPlugin 执行用户侧/公开插件页动作
func (h *PluginHandler) ExecutePublicPlugin(c *gin.Context) {
	h.executePublicPlugin(c, false)
}

// ExecutePublicPluginStream 流式执行用户侧/公开插件页动作
func (h *PluginHandler) ExecutePublicPluginStream(c *gin.Context) {
	h.executePublicPlugin(c, true)
}

func (h *PluginHandler) executePublicPlugin(c *gin.Context, stream bool) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}

	var req frontendExecutePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	req.Action = strings.TrimSpace(req.Action)
	if req.Action == "" {
		h.respondPluginError(c, http.StatusBadRequest, "action is required")
		return
	}

	path := normalizeFrontendBootstrapPath(req.Path)
	if len(path) > 500 {
		h.respondPluginError(c, http.StatusBadRequest, "path is too long")
		return
	}
	if !isAllowedPluginPagePath(frontendBootstrapAreaUser, path) {
		h.respondPluginError(c, http.StatusForbidden, "plugin page execute path is invalid")
		return
	}

	scope := h.resolvePluginAccessScope(c)
	sessionID := strings.TrimSpace(c.GetHeader("X-Session-ID"))
	queryParams := normalizeFrontendParamMap(req.QueryParams)
	var execUserID *uint
	if userID, exists := middleware.GetUserID(c); exists {
		execUserID = &userID
	}

	route, err := h.resolveAccessibleFrontendPluginRoute(
		c,
		frontendBootstrapAreaUser,
		path,
		id,
		queryParams,
		scope,
		sessionID,
		execUserID,
	)
	if err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to resolve plugin page route")
		return
	}
	if route == nil {
		h.respondPluginError(c, http.StatusForbidden, "plugin page route is unavailable for current request scope")
		return
	}
	if route.ExecuteAPI == nil {
		h.respondPluginError(c, http.StatusForbidden, "plugin page execute api is unavailable for current route or current capabilities")
		return
	}
	if !frontendRouteAllowsExecuteAction(route, req.Action) {
		h.respondPluginError(c, http.StatusForbidden, "plugin execute action is not declared for this page route")
		return
	}
	if stream && !frontendRouteAllowsStreamAction(route, req.Action) {
		h.respondPluginError(c, http.StatusForbidden, "plugin stream action is not declared for this page route")
		return
	}

	routeParams := cloneStringMap(route.RouteParams)
	if len(routeParams) == 0 {
		if _, matchedParams := frontendBootstrapRouteMatch(route.Path, path); len(matchedParams) > 0 {
			routeParams = matchedParams
		}
	}

	execCtx := h.buildFrontendPluginExecutionContext(
		c,
		scope,
		frontendBootstrapAreaUser,
		path,
		queryParams,
		routeParams,
		sessionID,
		execUserID,
	)
	execCtx.Metadata["plugin_page_route"] = route.Path
	execCtx.Metadata["plugin_page_plugin_id"] = strconv.FormatUint(uint64(route.PluginID), 10)
	execCtx.Metadata["plugin_page_action"] = strings.ToLower(req.Action)
	taskID := service.EnsurePluginExecutionMetadata(execCtx, stream)
	h.applyPluginExecutionHeaders(c, taskID)

	if stream {
		if err := h.startPluginNDJSONStream(c, taskID); err != nil {
			h.respondPluginErrorErr(c, http.StatusBadRequest, err)
			return
		}
		if err := h.writePluginStreamTaskStarted(c, taskID, pluginExecutionTaskLinks{}, execCtx.Metadata); err != nil {
			return
		}
		streamIndex := 0
		result, execErr := h.pluginManager.ExecutePluginStream(id, req.Action, req.Params, execCtx, func(chunk *service.ExecutionStreamChunk) error {
			if chunk != nil {
				streamIndex = chunk.Index + 1
			}
			return h.writePluginStreamChunk(c, chunk)
		})
		if execErr != nil {
			_ = h.writePluginStreamErrorWithLinks(
				c,
				streamIndex,
				execErr.Error(),
				taskID,
				pluginExecutionTaskLinks{},
				clonePluginExecutionMetadataWithStatus(execCtx.Metadata, resolvePluginExecutionFailureStatus(execErr)),
			)
			return
		}
		_ = result
		return
	}

	result, execErr := h.pluginManager.ExecutePlugin(id, req.Action, req.Params, execCtx)
	if execErr != nil {
		c.JSON(http.StatusOK, buildPluginExecuteFailurePayload(
			http.StatusBadRequest,
			"plugin execute failed",
			strings.TrimSpace(execErr.Error()),
			nil,
			clonePluginExecutionMetadataWithStatus(execCtx.Metadata, resolvePluginExecutionFailureStatus(execErr)),
		))
		return
	}

	resultErr := strings.TrimSpace(result.Error)
	resp := gin.H{
		"success":  result.Success,
		"task_id":  strings.TrimSpace(result.TaskID),
		"data":     result.Data,
		"metadata": result.Metadata,
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

func (h *PluginHandler) handleFrontendBootstrap(c *gin.Context, area string, publicEndpoint bool) {
	resolvedArea := normalizeFrontendBootstrapArea(area)
	defaultPath := "/"
	if resolvedArea == frontendBootstrapAreaAdmin {
		defaultPath = "/admin"
	}

	path := strings.TrimSpace(c.Query("path"))
	queryParams, err := parseFrontendQueryParams(c.Query("query_params"))
	if err != nil {
		h.respondPluginError(c, http.StatusBadRequest, err.Error())
		return
	}
	if path == "" {
		path = defaultPath
	}
	if len(path) > 500 {
		h.respondPluginError(c, http.StatusBadRequest, "path is too long")
		return
	}
	path = normalizeFrontendBootstrapPath(path)
	pluginobs.RecordFrontendBootstrapRequest()
	scope := h.resolvePluginAccessScope(c)
	sessionID := strings.TrimSpace(c.GetHeader("X-Session-ID"))
	var execUserID *uint
	if userID, ok := middleware.GetUserID(c); ok {
		execUserID = &userID
	}
	cacheVaryKey := resolvePluginPublicCacheVaryKey(c, scope, execUserID, sessionID)
	if publicEndpoint {
		if cached, ok := h.getCachedPublicBootstrap(resolvedArea, path, queryParams, scope, cacheVaryKey); ok {
			response.Success(c, cached)
			return
		}
	}

	respPayload, _, err := h.getOrLoadFrontendBootstrapPayload(
		c,
		resolvedArea,
		path,
		queryParams,
		scope,
		sessionID,
		execUserID,
	)
	if err != nil {
		h.respondPluginError(c, http.StatusInternalServerError, "Failed to load plugin frontend bootstrap")
		return
	}
	if publicEndpoint {
		h.cachePublicBootstrap(respPayload, queryParams, scope, cacheVaryKey)
	}
	response.Success(c, respPayload)
}

func (h *PluginHandler) getCachedPublicExtensions(slot string, path string, queryParams map[string]string, hostContext map[string]interface{}, scope pluginAccessScope, varyKey string) (frontendExtensionsResponseData, bool) {
	if h == nil || h.publicExtensionsCache == nil {
		pluginobs.RecordPublicCache("/api/config/plugin-extensions", false)
		return frontendExtensionsResponseData{}, false
	}
	cacheKey := buildPublicExtensionsCacheKey(slot, path, queryParams, hostContext, scope, varyKey)
	cached, exists := h.publicExtensionsCache.Get(cacheKey)
	if !exists {
		pluginobs.RecordPublicCache("/api/config/plugin-extensions", false)
		return frontendExtensionsResponseData{}, false
	}
	payload, ok := cached.(frontendExtensionsResponseData)
	if !ok {
		pluginobs.RecordPublicCache("/api/config/plugin-extensions", false)
		return frontendExtensionsResponseData{}, false
	}
	pluginobs.RecordPublicCache("/api/config/plugin-extensions", true)
	payload.Extensions = cloneFrontendExtensions(payload.Extensions)
	return payload, true
}

func (h *PluginHandler) cachePublicExtensions(payload frontendExtensionsResponseData, queryParams map[string]string, hostContext map[string]interface{}, scope pluginAccessScope, varyKey string) {
	if h == nil || h.publicExtensionsCache == nil {
		return
	}
	cached := frontendExtensionsResponseData{
		Path:       payload.Path,
		Slot:       payload.Slot,
		Extensions: cloneFrontendExtensions(payload.Extensions),
	}
	cacheKey := buildPublicExtensionsCacheKey(cached.Slot, cached.Path, queryParams, hostContext, scope, varyKey)
	h.publicExtensionsCache.Set(cacheKey, cached)
}

func (h *PluginHandler) getCachedPublicBootstrap(area string, path string, queryParams map[string]string, scope pluginAccessScope, varyKey string) (frontendBootstrapResponseData, bool) {
	if h == nil || h.publicBootstrapCache == nil {
		pluginobs.RecordPublicCache("/api/config/plugin-bootstrap", false)
		return frontendBootstrapResponseData{}, false
	}
	cacheKey := buildPublicBootstrapCacheKey(area, path, queryParams, scope, varyKey)
	cached, exists := h.publicBootstrapCache.Get(cacheKey)
	if !exists {
		pluginobs.RecordPublicCache("/api/config/plugin-bootstrap", false)
		return frontendBootstrapResponseData{}, false
	}
	payload, ok := cached.(frontendBootstrapResponseData)
	if !ok {
		pluginobs.RecordPublicCache("/api/config/plugin-bootstrap", false)
		return frontendBootstrapResponseData{}, false
	}
	pluginobs.RecordPublicCache("/api/config/plugin-bootstrap", true)
	return cloneFrontendBootstrapPayload(payload), true
}

func (h *PluginHandler) cachePublicBootstrap(payload frontendBootstrapResponseData, queryParams map[string]string, scope pluginAccessScope, varyKey string) {
	if h == nil || h.publicBootstrapCache == nil {
		return
	}
	cached := cloneFrontendBootstrapPayload(payload)
	cacheKey := buildPublicBootstrapCacheKey(cached.Area, cached.Path, queryParams, scope, varyKey)
	h.publicBootstrapCache.Set(cacheKey, cached)
}

func (h *PluginHandler) getCachedFrontendBootstrap(area string, path string, queryParams map[string]string, scope pluginAccessScope, varyKey string) (frontendBootstrapResponseData, bool) {
	if h == nil || h.frontendBootstrapCache == nil {
		return frontendBootstrapResponseData{}, false
	}
	cacheKey := buildFrontendBootstrapInternalCacheKey(area, path, queryParams, scope, varyKey)
	cached, exists := h.frontendBootstrapCache.Get(cacheKey)
	if !exists {
		return frontendBootstrapResponseData{}, false
	}
	payload, ok := cached.(frontendBootstrapResponseData)
	if !ok {
		return frontendBootstrapResponseData{}, false
	}
	return cloneFrontendBootstrapPayload(payload), true
}

func (h *PluginHandler) cacheFrontendBootstrap(payload frontendBootstrapResponseData, queryParams map[string]string, scope pluginAccessScope, varyKey string) {
	if h == nil || h.frontendBootstrapCache == nil {
		return
	}
	cached := cloneFrontendBootstrapPayload(payload)
	cacheKey := buildFrontendBootstrapInternalCacheKey(cached.Area, cached.Path, queryParams, scope, varyKey)
	h.frontendBootstrapCache.Set(cacheKey, cached)
}

func cloneFrontendExtensions(items []service.FrontendExtension) []service.FrontendExtension {
	if len(items) == 0 {
		return []service.FrontendExtension{}
	}
	out := make([]service.FrontendExtension, 0, len(items))
	for _, item := range items {
		copied := item
		copied.Data = cloneStringAnyMap(item.Data)
		copied.Metadata = cloneStringMap(item.Metadata)
		out = append(out, copied)
	}
	return out
}

func cloneFrontendBootstrapMenus(items []frontendBootstrapMenuItem) []frontendBootstrapMenuItem {
	if len(items) == 0 {
		return []frontendBootstrapMenuItem{}
	}
	out := make([]frontendBootstrapMenuItem, 0, len(items))
	for _, item := range items {
		copied := item
		copied.RequiredPermissions = append([]string(nil), item.RequiredPermissions...)
		out = append(out, copied)
	}
	return out
}

func cloneFrontendBootstrapRoutes(items []frontendBootstrapRouteItem) []frontendBootstrapRouteItem {
	if len(items) == 0 {
		return []frontendBootstrapRouteItem{}
	}
	out := make([]frontendBootstrapRouteItem, 0, len(items))
	for _, item := range items {
		copied := item
		copied.RouteParams = cloneStringMap(item.RouteParams)
		copied.RequiredPermissions = append([]string(nil), item.RequiredPermissions...)
		copied.AllowedActions = append([]string(nil), item.AllowedActions...)
		copied.AllowedStreamActions = append([]string(nil), item.AllowedStreamActions...)
		copied.Page = cloneStringAnyMap(item.Page)
		if item.ExecuteAPI != nil {
			executeAPI := *item.ExecuteAPI
			executeAPI.AllowedActions = append([]string(nil), item.ExecuteAPI.AllowedActions...)
			executeAPI.StreamActions = append([]string(nil), item.ExecuteAPI.StreamActions...)
			copied.ExecuteAPI = &executeAPI
		}
		out = append(out, copied)
	}
	return out
}

func cloneFrontendBootstrapPayload(payload frontendBootstrapResponseData) frontendBootstrapResponseData {
	return frontendBootstrapResponseData{
		Area:   payload.Area,
		Path:   payload.Path,
		Menus:  cloneFrontendBootstrapMenus(payload.Menus),
		Routes: cloneFrontendBootstrapRoutes(payload.Routes),
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		if source == nil {
			return nil
		}
		return map[string]string{}
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneStringAnyMap(source map[string]interface{}) map[string]interface{} {
	if len(source) == 0 {
		if source == nil {
			return nil
		}
		return map[string]interface{}{}
	}
	cloned := make(map[string]interface{}, len(source))
	for key, value := range source {
		cloned[key] = cloneInterfaceValue(value)
	}
	return cloned
}

func cloneInterfaceValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return cloneStringAnyMap(typed)
	case []interface{}:
		out := make([]interface{}, len(typed))
		for idx := range typed {
			out[idx] = cloneInterfaceValue(typed[idx])
		}
		return out
	default:
		return value
	}
}

func applyPluginAccessScopeMetadata(metadata map[string]string, scope pluginAccessScope) {
	if metadata == nil {
		return
	}
	metadata[service.PluginScopeMetadataAuthenticated] = strconv.FormatBool(scope.authenticated)
	metadata[service.PluginScopeMetadataSuperAdmin] = strconv.FormatBool(scope.superAdmin)
	metadata[service.PluginScopeMetadataPermissions] = strings.Join(sortedPluginAccessScopePermissionKeys(scope.permissions), ",")
}

func sortedPluginAccessScopePermissionKeys(permissions map[string]struct{}) []string {
	if len(permissions) == 0 {
		return nil
	}
	keys := make([]string, 0, len(permissions))
	for key := range permissions {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "" {
			continue
		}
		keys = append(keys, normalized)
	}
	sort.Strings(keys)
	return keys
}

func normalizeFrontendBootstrapArea(area string) string {
	switch strings.ToLower(strings.TrimSpace(area)) {
	case frontendBootstrapAreaAdmin:
		return frontendBootstrapAreaAdmin
	default:
		return frontendBootstrapAreaUser
	}
}

func normalizeFrontendBootstrapPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	cleaned := pathpkg.Clean(trimmed)
	if cleaned == "." {
		return "/"
	}
	return cleaned
}

func splitFrontendBootstrapPathSegments(value string) []string {
	normalized := strings.Trim(normalizeFrontendBootstrapPath(value), "/")
	if normalized == "" {
		return nil
	}
	return strings.Split(normalized, "/")
}

func frontendBootstrapRouteMatch(routePath string, currentPath string) (bool, map[string]string) {
	normalizedRoute := normalizeFrontendBootstrapPath(routePath)
	normalizedCurrent := normalizeFrontendBootstrapPath(currentPath)
	if normalizedRoute == normalizedCurrent {
		return true, nil
	}

	routeSegments := splitFrontendBootstrapPathSegments(normalizedRoute)
	currentSegments := splitFrontendBootstrapPathSegments(normalizedCurrent)
	routeParams := make(map[string]string)

	for routeIndex, currentIndex := 0, 0; routeIndex < len(routeSegments); routeIndex, currentIndex = routeIndex+1, currentIndex+1 {
		routeSegment := routeSegments[routeIndex]
		if strings.HasPrefix(routeSegment, "*") {
			if routeIndex != len(routeSegments)-1 || currentIndex >= len(currentSegments) {
				return false, nil
			}
			wildcardName := strings.TrimPrefix(routeSegment, "*")
			if wildcardName != "" {
				routeParams[wildcardName] = strings.Join(currentSegments[currentIndex:], "/")
			}
			return true, normalizeFrontendParamMap(routeParams)
		}
		if currentIndex >= len(currentSegments) {
			return false, nil
		}
		currentSegment := currentSegments[currentIndex]
		if routeSegment == currentSegment {
			continue
		}
		if strings.HasPrefix(routeSegment, ":") && len(routeSegment) > 1 {
			routeParams[strings.TrimPrefix(routeSegment, ":")] = currentSegment
			continue
		}
		return false, nil
	}

	if len(routeSegments) != len(currentSegments) {
		return false, nil
	}
	return true, normalizeFrontendParamMap(routeParams)
}

func frontendBootstrapRouteMatchesPath(routePath string, currentPath string) bool {
	matched, _ := frontendBootstrapRouteMatch(routePath, currentPath)
	return matched
}

func isAllowedPluginPagePath(area string, path string) bool {
	normalizedArea := normalizeFrontendBootstrapArea(area)
	normalizedPath := normalizeFrontendBootstrapPath(path)
	if normalizedArea == frontendBootstrapAreaAdmin {
		return strings.HasPrefix(normalizedPath, "/admin/plugin-pages/")
	}
	return strings.HasPrefix(normalizedPath, "/plugin-pages/")
}

func (h *PluginHandler) buildFrontendPluginExecutionContext(
	c *gin.Context,
	scope pluginAccessScope,
	area string,
	path string,
	queryParams map[string]string,
	routeParams map[string]string,
	sessionID string,
	execUserID *uint,
) *service.ExecutionContext {
	queryString := buildFrontendQueryString(queryParams)
	locale, acceptLanguage := resolvePluginRequestLocaleMetadata(c)
	metadata := map[string]string{
		"request_path":             c.Request.URL.Path,
		"route":                    c.FullPath(),
		"client_ip":                utils.GetRealIP(c),
		"user_agent":               c.GetHeader("User-Agent"),
		"accept_language":          acceptLanguage,
		"bootstrap_area":           normalizeFrontendBootstrapArea(area),
		"plugin_page_path":         normalizeFrontendBootstrapPath(path),
		"plugin_page_full_path":    buildFrontendFullPath(path, queryParams),
		"plugin_page_query_string": queryString,
		"plugin_page_query_params": marshalFrontendParamMapJSON(queryParams),
		"plugin_page_route_params": marshalFrontendParamMapJSON(routeParams),
	}
	if locale != "" {
		metadata["locale"] = locale
	}
	applyPluginAccessScopeMetadata(metadata, scope)

	execCtx := &service.ExecutionContext{
		OperatorUserID: getOptionalOperatorUserID(c),
		SessionID:      sessionID,
		Metadata:       metadata,
		RequestContext: c.Request.Context(),
	}
	if execUserID != nil {
		execCtx.UserID = execUserID
	}
	return execCtx
}

func buildEmptyFrontendBootstrapPayload(area string, path string) frontendBootstrapResponseData {
	return frontendBootstrapResponseData{
		Area:   normalizeFrontendBootstrapArea(area),
		Path:   normalizeFrontendBootstrapPath(path),
		Menus:  []frontendBootstrapMenuItem{},
		Routes: []frontendBootstrapRouteItem{},
	}
}

func (h *PluginHandler) buildFrontendBootstrapPayload(
	c *gin.Context,
	area string,
	path string,
	queryParams map[string]string,
	scope pluginAccessScope,
	sessionID string,
	execUserID *uint,
) (frontendBootstrapResponseData, error) {
	normalizedArea := normalizeFrontendBootstrapArea(area)
	normalizedPath := normalizeFrontendBootstrapPath(path)
	respPayload := buildEmptyFrontendBootstrapPayload(normalizedArea, normalizedPath)
	if h == nil || h.pluginManager == nil {
		return respPayload, nil
	}

	hookResult, err := h.pluginManager.ExecuteHook(service.HookExecutionRequest{
		Hook: "frontend.bootstrap",
		Payload: map[string]interface{}{
			"area":         normalizedArea,
			"path":         normalizedPath,
			"slot":         "bootstrap",
			"query_params": queryParams,
			"query_string": buildFrontendQueryString(queryParams),
			"full_path":    buildFrontendFullPath(normalizedPath, queryParams),
		},
	}, h.buildFrontendPluginExecutionContext(c, scope, normalizedArea, normalizedPath, queryParams, nil, sessionID, execUserID))
	if err != nil {
		return respPayload, err
	}

	extensions := make([]service.FrontendExtension, 0)
	if hookResult != nil {
		extensions = hookResult.FrontendExtensions
	}
	menus, routes := collectFrontendBootstrapEntries(normalizedArea, extensions)
	menus, routes = filterFrontendBootstrapByScope(normalizedArea, menus, routes, scope)
	capabilityCache := newFrontendPluginCapabilityResolveCache()
	htmlModes := h.resolvePluginFrontendHTMLModes(
		collectFrontendBootstrapRoutePluginIDs(routes),
		capabilityCache,
	)
	routes = annotateFrontendBootstrapRoutesHTMLMode(routes, htmlModes)
	routes = h.annotateFrontendBootstrapRoutesExecuteAPI(normalizedArea, routes, capabilityCache)
	for idx := range routes {
		matched, routeParams := frontendBootstrapRouteMatch(routes[idx].Path, normalizedPath)
		if !matched {
			routes[idx].RouteParams = nil
			continue
		}
		routes[idx].RouteParams = routeParams
	}

	respPayload.Menus = menus
	respPayload.Routes = routes
	return respPayload, nil
}

func (h *PluginHandler) getOrLoadFrontendBootstrapPayload(
	c *gin.Context,
	area string,
	path string,
	queryParams map[string]string,
	scope pluginAccessScope,
	sessionID string,
	execUserID *uint,
) (frontendBootstrapResponseData, bool, error) {
	normalizedArea := normalizeFrontendBootstrapArea(area)
	normalizedPath := normalizeFrontendBootstrapPath(path)
	cacheVaryKey := resolvePluginPublicCacheVaryKey(c, scope, execUserID, sessionID)
	if cached, ok := h.getCachedFrontendBootstrap(normalizedArea, normalizedPath, queryParams, scope, cacheVaryKey); ok {
		return cached, true, nil
	}
	if h == nil || h.pluginManager == nil {
		return buildEmptyFrontendBootstrapPayload(normalizedArea, normalizedPath), false, nil
	}

	payload, err := h.buildFrontendBootstrapPayload(c, normalizedArea, normalizedPath, queryParams, scope, sessionID, execUserID)
	if err != nil {
		return buildEmptyFrontendBootstrapPayload(normalizedArea, normalizedPath), false, err
	}
	h.cacheFrontendBootstrap(payload, queryParams, scope, cacheVaryKey)
	return payload, true, nil
}

func (h *PluginHandler) resolveAccessibleFrontendPluginRoute(
	c *gin.Context,
	area string,
	path string,
	pluginID uint,
	queryParams map[string]string,
	scope pluginAccessScope,
	sessionID string,
	execUserID *uint,
) (*frontendBootstrapRouteItem, error) {
	normalizedArea := normalizeFrontendBootstrapArea(area)
	normalizedPath := normalizeFrontendBootstrapPath(path)
	payload, available, err := h.getOrLoadFrontendBootstrapPayload(
		c,
		normalizedArea,
		normalizedPath,
		queryParams,
		scope,
		sessionID,
		execUserID,
	)
	if err != nil {
		return nil, err
	}
	if !available {
		return nil, fmt.Errorf("plugin manager is unavailable")
	}

	for idx := range payload.Routes {
		route := payload.Routes[idx]
		if route.PluginID != pluginID {
			continue
		}
		if matched, routeParams := frontendBootstrapRouteMatch(route.Path, normalizedPath); matched {
			copied := route
			if len(copied.RouteParams) == 0 {
				copied.RouteParams = routeParams
			} else {
				copied.RouteParams = cloneStringMap(copied.RouteParams)
			}
			return &copied, nil
		}
	}

	return nil, nil
}

func parseStringFromAny(values ...interface{}) string {
	for _, value := range values {
		if value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed != "" {
				return trimmed
			}
		default:
			text := strings.TrimSpace(fmt.Sprintf("%v", typed))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func normalizedSlotValue(value interface{}) string {
	return strings.ToLower(strings.TrimSpace(parseStringFromAny(value)))
}

func parseIntFromAny(value interface{}, defaultValue int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case uint:
		return int(typed)
	case uint8:
		return int(typed)
	case uint16:
		return int(typed)
	case uint32:
		return int(typed)
	case uint64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return defaultValue
		}
		if parsed, err := strconv.Atoi(trimmed); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func parseBoolFromAny(value interface{}, defaultValue bool) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	}
	return defaultValue
}

func parseStringListFromAny(value interface{}) []string {
	switch typed := value.(type) {
	case []string:
		return normalizeLowerStringListValues(typed)
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := parseStringFromAny(item); text != "" {
				out = append(out, text)
			}
		}
		return normalizeLowerStringListValues(out)
	default:
		text := parseStringFromAny(value)
		if text == "" {
			return nil
		}
		return normalizeLowerStringListValues([]string{text})
	}
}

func frontendRouteAllowsExecuteAction(route *frontendBootstrapRouteItem, action string) bool {
	if route == nil {
		return false
	}
	normalizedAction := strings.ToLower(strings.TrimSpace(action))
	if normalizedAction == "" {
		return false
	}
	for _, candidate := range normalizeLowerStringListValues(route.AllowedActions) {
		if candidate == normalizedAction {
			return true
		}
	}
	return false
}

func frontendRouteAllowsStreamAction(route *frontendBootstrapRouteItem, action string) bool {
	if route == nil {
		return false
	}
	normalizedAction := strings.ToLower(strings.TrimSpace(action))
	if normalizedAction == "" {
		return false
	}
	for _, candidate := range normalizeLowerStringListValues(route.AllowedStreamActions) {
		if candidate == normalizedAction {
			return true
		}
	}
	return false
}

func collectFrontendRouteExecuteActions(routeData map[string]interface{}, pageSchema map[string]interface{}) []string {
	actions := make([]string, 0)
	if len(routeData) > 0 {
		actions = append(actions, parseStringListFromAny(routeData["allowed_actions"])...)
		actions = append(actions, parseStringListFromAny(routeData["execute_actions"])...)
	}
	if len(pageSchema) == 0 {
		return normalizeLowerStringListValues(actions)
	}

	actions = append(actions, parseStringListFromAny(pageSchema["allowed_actions"])...)
	actions = append(actions, parseStringListFromAny(pageSchema["execute_actions"])...)

	blocks, ok := pageSchema["blocks"].([]interface{})
	if !ok {
		return normalizeLowerStringListValues(actions)
	}
	for _, rawBlock := range blocks {
		block, ok := rawBlock.(map[string]interface{})
		if !ok {
			continue
		}
		actions = append(actions, collectFrontendPageBlockExecuteActions(block)...)
	}
	return normalizeLowerStringListValues(actions)
}

func collectFrontendRouteStreamActions(routeData map[string]interface{}, pageSchema map[string]interface{}) []string {
	actions := make([]string, 0)
	if len(routeData) > 0 {
		actions = append(actions, parseStringListFromAny(routeData["stream_actions"])...)
		actions = append(actions, parseStringListFromAny(routeData["execute_stream_actions"])...)
	}
	if len(pageSchema) == 0 {
		return normalizeLowerStringListValues(actions)
	}

	actions = append(actions, parseStringListFromAny(pageSchema["stream_actions"])...)
	actions = append(actions, parseStringListFromAny(pageSchema["execute_stream_actions"])...)

	blocks, ok := pageSchema["blocks"].([]interface{})
	if !ok {
		return normalizeLowerStringListValues(actions)
	}
	for _, rawBlock := range blocks {
		block, ok := rawBlock.(map[string]interface{})
		if !ok {
			continue
		}
		actions = append(actions, collectFrontendPageBlockStreamActions(block)...)
	}
	return normalizeLowerStringListValues(actions)
}

func collectFrontendPageBlockExecuteActions(block map[string]interface{}) []string {
	if len(block) == 0 {
		return nil
	}

	actions := make([]string, 0)
	actions = append(actions, parseStringListFromAny(block["allowed_actions"])...)
	actions = append(actions, parseStringListFromAny(block["execute_actions"])...)

	data := asStringAnyMap(block["data"])
	if len(data) > 0 {
		actions = append(actions, parseStringListFromAny(data["allowed_actions"])...)
		actions = append(actions, parseStringListFromAny(data["execute_actions"])...)
	}

	blockType := strings.ToLower(strings.TrimSpace(parseStringFromAny(block["type"])))
	if blockType != "action_form" || len(data) == 0 {
		return normalizeLowerStringListValues(actions)
	}

	actionData := asStringAnyMap(data["actions"])
	if len(actionData) == 0 {
		return normalizeLowerStringListValues(actions)
	}

	actions = append(actions,
		parseStringFromAny(actionData["load"]),
		parseStringFromAny(actionData["save"]),
		parseStringFromAny(actionData["reset"]),
	)

	for _, key := range []string{"extra", "buttons"} {
		items, ok := actionData[key].([]interface{})
		if !ok {
			continue
		}
		for _, rawItem := range items {
			item, ok := rawItem.(map[string]interface{})
			if !ok {
				continue
			}
			actions = append(actions, parseStringFromAny(item["action"]))
		}
	}

	return normalizeLowerStringListValues(actions)
}

func collectFrontendPageBlockStreamActions(block map[string]interface{}) []string {
	if len(block) == 0 {
		return nil
	}

	actions := make([]string, 0)
	actions = append(actions, parseStringListFromAny(block["stream_actions"])...)
	actions = append(actions, parseStringListFromAny(block["execute_stream_actions"])...)

	data := asStringAnyMap(block["data"])
	if len(data) > 0 {
		actions = append(actions, parseStringListFromAny(data["stream_actions"])...)
		actions = append(actions, parseStringListFromAny(data["execute_stream_actions"])...)
	}

	blockType := strings.ToLower(strings.TrimSpace(parseStringFromAny(block["type"])))
	if blockType != "action_form" || len(data) == 0 {
		return normalizeLowerStringListValues(actions)
	}

	actionData := asStringAnyMap(data["actions"])
	if len(actionData) == 0 {
		return normalizeLowerStringListValues(actions)
	}

	loadAction := parseStringFromAny(actionData["load"])
	saveAction := parseStringFromAny(actionData["save"])
	resetAction := parseStringFromAny(actionData["reset"])
	if parseExecuteStreamActionMode(actionData["load_mode"], actionData["load_stream"]) {
		actions = append(actions, loadAction)
	}
	if parseExecuteStreamActionMode(actionData["save_mode"], actionData["save_stream"]) {
		actions = append(actions, saveAction)
	}
	if parseExecuteStreamActionMode(actionData["reset_mode"], actionData["reset_stream"]) {
		actions = append(actions, resetAction)
	}

	for _, key := range []string{"extra", "buttons"} {
		items, ok := actionData[key].([]interface{})
		if !ok {
			continue
		}
		for _, rawItem := range items {
			item, ok := rawItem.(map[string]interface{})
			if !ok {
				continue
			}
			if !parseExecuteStreamActionMode(item["mode"], item["stream"], item["execute_mode"]) {
				continue
			}
			actions = append(actions, parseStringFromAny(item["action"]))
		}
	}

	return normalizeLowerStringListValues(actions)
}

func parseExecuteStreamActionMode(values ...interface{}) bool {
	for _, value := range values {
		if parseBoolFromAny(value, false) {
			return true
		}
		if strings.EqualFold(strings.TrimSpace(parseStringFromAny(value)), "stream") {
			return true
		}
	}
	return false
}

func normalizePluginFrontendHTMLMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == pluginFrontendHTMLModeTrusted {
		return pluginFrontendHTMLModeTrusted
	}
	return pluginFrontendHTMLModeSanitize
}

func parsePluginFrontendHTMLModeFromCapabilitiesRaw(capabilitiesRaw string) string {
	trimmed := strings.TrimSpace(capabilitiesRaw)
	if trimmed == "" {
		return pluginFrontendHTMLModeSanitize
	}

	var capabilities map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &capabilities); err != nil {
		return pluginFrontendHTMLModeSanitize
	}
	return parsePluginFrontendHTMLModeFromCapabilitiesMap(capabilities)
}

func parsePluginFrontendHTMLModeFromCapabilitiesMap(capabilities map[string]interface{}) string {
	if len(capabilities) == 0 {
		return pluginFrontendHTMLModeSanitize
	}
	mode := parseStringFromAny(capabilities["frontend_html_mode"], capabilities["html_mode"])
	if normalizePluginFrontendHTMLMode(mode) != pluginFrontendHTMLModeTrusted {
		return pluginFrontendHTMLModeSanitize
	}
	requestedPermissions := parseStringListFromAny(capabilities["requested_permissions"])
	grantedPermissions := parseStringListFromAny(capabilities["granted_permissions"])
	if !service.IsPluginPermissionGranted(requestedPermissions, grantedPermissions, service.PluginPermissionFrontendHTMLTrust) {
		return pluginFrontendHTMLModeSanitize
	}
	return pluginFrontendHTMLModeTrusted
}

func resolveEffectivePluginFrontendHTMLMode(mode string, forceSanitize bool) string {
	if forceSanitize {
		return pluginFrontendHTMLModeSanitize
	}
	return normalizePluginFrontendHTMLMode(mode)
}

func (h *PluginHandler) isPluginFrontendTrustedHTMLForceSanitizeEnabled() bool {
	cfg := config.GetConfig()
	if cfg == nil {
		return false
	}
	return cfg.Plugin.Frontend.ForceSanitizeHTML
}

func collectFrontendExtensionPluginIDs(extensions []service.FrontendExtension) []uint {
	if len(extensions) == 0 {
		return nil
	}
	set := make(map[uint]struct{}, len(extensions))
	for _, item := range extensions {
		if item.PluginID == 0 {
			continue
		}
		set[item.PluginID] = struct{}{}
	}
	out := make([]uint, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

func collectFrontendBootstrapRoutePluginIDs(routes []frontendBootstrapRouteItem) []uint {
	if len(routes) == 0 {
		return nil
	}
	set := make(map[uint]struct{}, len(routes))
	for _, item := range routes {
		if item.PluginID == 0 {
			continue
		}
		set[item.PluginID] = struct{}{}
	}
	out := make([]uint, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

func (h *PluginHandler) resolvePluginFrontendHTMLModes(
	pluginIDs []uint,
	cache *frontendPluginCapabilityResolveCache,
) map[uint]string {
	if cache == nil {
		return h.resolvePluginFrontendHTMLModesUncached(pluginIDs)
	}
	return cache.resolveHTMLModes(pluginIDs, h.resolvePluginFrontendHTMLModesUncached)
}

func (h *PluginHandler) resolvePluginFrontendHTMLModesUncached(pluginIDs []uint) map[uint]string {
	normalizedIDs := normalizeFrontendPluginIDs(pluginIDs)
	modes := make(map[uint]string, len(normalizedIDs))
	forceSanitize := h.isPluginFrontendTrustedHTMLForceSanitizeEnabled()
	for _, id := range normalizedIDs {
		modes[id] = resolveEffectivePluginFrontendHTMLMode(pluginFrontendHTMLModeSanitize, forceSanitize)
	}
	if len(modes) == 0 || forceSanitize {
		return modes
	}

	resolved := make(map[uint]struct{}, len(normalizedIDs))
	if h != nil && h.pluginManager != nil {
		catalogModes := h.pluginManager.ResolvePluginFrontendHTMLModesFromCatalog(normalizedIDs)
		if len(catalogModes) > 0 {
			pluginobs.RecordFrontendResolverCatalogHit("html_mode", len(catalogModes))
		}
		for id, mode := range catalogModes {
			modes[id] = resolveEffectivePluginFrontendHTMLMode(mode, forceSanitize)
			resolved[id] = struct{}{}
		}
	}
	if h == nil || h.db == nil {
		return modes
	}

	ids := make([]uint, 0, len(modes)-len(resolved))
	for _, id := range normalizedIDs {
		if _, exists := resolved[id]; exists {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return modes
	}
	pluginobs.RecordFrontendResolverDBFallback("html_mode", len(ids))

	var plugins []models.Plugin
	if err := h.db.Select("id", "capabilities").Where("id IN ?", ids).Find(&plugins).Error; err != nil {
		return modes
	}
	for _, plugin := range plugins {
		modes[plugin.ID] = resolveEffectivePluginFrontendHTMLMode(
			parsePluginFrontendHTMLModeFromCapabilitiesRaw(plugin.Capabilities),
			forceSanitize,
		)
	}
	return modes
}

func annotateFrontendExtensionsHTMLMode(
	extensions []service.FrontendExtension,
	modes map[uint]string,
) []service.FrontendExtension {
	if len(extensions) == 0 {
		return []service.FrontendExtension{}
	}
	out := make([]service.FrontendExtension, 0, len(extensions))
	for _, item := range extensions {
		mode := pluginFrontendHTMLModeSanitize
		if resolved, exists := modes[item.PluginID]; exists {
			mode = normalizePluginFrontendHTMLMode(resolved)
		}
		metadata := make(map[string]string, len(item.Metadata)+1)
		for key, value := range item.Metadata {
			metadata[key] = value
		}
		metadata["html_mode"] = mode
		item.Metadata = metadata
		out = append(out, item)
	}
	return out
}

func annotateFrontendBootstrapRoutesHTMLMode(
	routes []frontendBootstrapRouteItem,
	modes map[uint]string,
) []frontendBootstrapRouteItem {
	if len(routes) == 0 {
		return []frontendBootstrapRouteItem{}
	}
	out := make([]frontendBootstrapRouteItem, 0, len(routes))
	for _, item := range routes {
		mode := pluginFrontendHTMLModeSanitize
		if resolved, exists := modes[item.PluginID]; exists {
			mode = normalizePluginFrontendHTMLMode(resolved)
		}
		item.HTMLMode = mode
		out = append(out, item)
	}
	return out
}

func (h *PluginHandler) annotateFrontendBootstrapRoutesExecuteAPI(
	area string,
	routes []frontendBootstrapRouteItem,
	cache *frontendPluginCapabilityResolveCache,
) []frontendBootstrapRouteItem {
	if len(routes) == 0 {
		return []frontendBootstrapRouteItem{}
	}
	availability := h.resolvePluginExecuteAPIAvailability(
		collectFrontendBootstrapRoutePluginIDs(routes),
		cache,
	)
	out := make([]frontendBootstrapRouteItem, 0, len(routes))
	for _, item := range routes {
		if item.PluginID > 0 && availability[item.PluginID] {
			item.ExecuteAPI = buildFrontendRouteExecuteAPI(area, item)
		}
		out = append(out, item)
	}
	return out
}

func (h *PluginHandler) resolvePluginExecuteAPIAvailability(
	pluginIDs []uint,
	cache *frontendPluginCapabilityResolveCache,
) map[uint]bool {
	if cache == nil {
		return h.resolvePluginExecuteAPIAvailabilityUncached(pluginIDs)
	}
	return cache.resolveExecuteAPIAvailability(pluginIDs, h.resolvePluginExecuteAPIAvailabilityUncached)
}

func (h *PluginHandler) resolvePluginExecuteAPIAvailabilityUncached(pluginIDs []uint) map[uint]bool {
	normalizedIDs := normalizeFrontendPluginIDs(pluginIDs)
	availability := make(map[uint]bool, len(normalizedIDs))
	if len(normalizedIDs) == 0 {
		return availability
	}
	for _, id := range normalizedIDs {
		availability[id] = false
	}

	resolved := make(map[uint]struct{}, len(normalizedIDs))
	if h != nil && h.pluginManager != nil {
		catalogAvailability := h.pluginManager.ResolvePluginExecuteAPIAvailabilityFromCatalog(normalizedIDs)
		if len(catalogAvailability) > 0 {
			pluginobs.RecordFrontendResolverCatalogHit("execute_api", len(catalogAvailability))
		}
		for id, allowed := range catalogAvailability {
			availability[id] = allowed
			resolved[id] = struct{}{}
		}
	}

	if h == nil || h.db == nil {
		return availability
	}

	ids := make([]uint, 0, len(availability)-len(resolved))
	for _, id := range normalizedIDs {
		if _, exists := resolved[id]; exists {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return availability
	}
	pluginobs.RecordFrontendResolverDBFallback("execute_api", len(ids))

	var plugins []models.Plugin
	if err := h.db.Select("id", "name", "capabilities").Where("id IN ?", ids).Find(&plugins).Error; err != nil {
		return availability
	}
	for _, plugin := range plugins {
		availability[plugin.ID] = parseFrontendRouteExecuteAPIAvailability(plugin.Capabilities)
	}
	return availability
}

func parseFrontendRouteExecuteAPIAvailability(capabilitiesRaw string) bool {
	trimmed := strings.TrimSpace(capabilitiesRaw)
	if trimmed == "" {
		return false
	}

	var capabilities map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &capabilities); err != nil {
		return false
	}

	allowExecuteAPI := parseBoolFromAny(capabilities["allow_execute_api"], true)
	requestedPermissions := service.NormalizePluginPermissionList(parseStringListFromAny(capabilities["requested_permissions"]))
	grantedPermissions := service.NormalizePluginPermissionList(parseStringListFromAny(capabilities["granted_permissions"]))
	return allowExecuteAPI &&
		service.IsPluginPermissionGranted(requestedPermissions, grantedPermissions, service.PluginPermissionExecuteAPI)
}

func buildFrontendRouteExecuteAPI(area string, route frontendBootstrapRouteItem) *frontendRouteExecuteAPI {
	allowedActions := normalizeLowerStringListValues(route.AllowedActions)
	if route.PluginID == 0 || len(allowedActions) == 0 {
		return nil
	}
	allowedActionSet := make(map[string]struct{}, len(allowedActions))
	for _, action := range allowedActions {
		allowedActionSet[action] = struct{}{}
	}
	streamActions := make([]string, 0, len(route.AllowedStreamActions))
	for _, action := range normalizeLowerStringListValues(route.AllowedStreamActions) {
		if _, exists := allowedActionSet[action]; exists {
			streamActions = append(streamActions, action)
		}
	}

	normalizedArea := normalizeFrontendBootstrapArea(area)
	url := fmt.Sprintf("/api/config/plugins/%d/execute", route.PluginID)
	streamURL := ""
	streamFormat := ""
	scope := "public"
	requiresAuth := !route.GuestVisible
	if normalizedArea == frontendBootstrapAreaAdmin {
		url = fmt.Sprintf("/api/admin/plugins/%d/execute", route.PluginID)
		if len(streamActions) > 0 {
			streamURL = fmt.Sprintf("/api/admin/plugins/%d/execute/stream", route.PluginID)
			streamFormat = "ndjson"
		}
		scope = "admin"
		requiresAuth = true
	} else if len(streamActions) > 0 {
		streamURL = fmt.Sprintf("/api/config/plugins/%d/execute/stream", route.PluginID)
		streamFormat = "ndjson"
	}

	return &frontendRouteExecuteAPI{
		URL:            url,
		Method:         http.MethodPost,
		Scope:          scope,
		RequiresAuth:   requiresAuth,
		PathParam:      "path",
		ActionParam:    "action",
		ParamsFormat:   "json",
		AllowedActions: allowedActions,
		StreamURL:      streamURL,
		StreamFormat:   streamFormat,
		StreamActions:  streamActions,
	}
}

func collectFrontendBootstrapEntries(area string, extensions []service.FrontendExtension) ([]frontendBootstrapMenuItem, []frontendBootstrapRouteItem) {
	normalizedArea := normalizeFrontendBootstrapArea(area)
	menus := make([]frontendBootstrapMenuItem, 0)
	routes := make([]frontendBootstrapRouteItem, 0)
	for idx, extension := range extensions {
		typeName := strings.ToLower(strings.TrimSpace(extension.Type))
		if typeName == "" {
			continue
		}

		data := extension.Data
		if data == nil {
			data = map[string]interface{}{}
		}
		entryArea := normalizeFrontendBootstrapArea(parseStringFromAny(data["area"], data["target_area"], extension.Metadata["area"], normalizedArea))
		if entryArea != normalizedArea {
			continue
		}

		switch typeName {
		case "menu_item", "menu", "nav_item":
			path := parseStringFromAny(extension.Link, data["path"], data["href"])
			if path == "" {
				continue
			}
			path = normalizeFrontendBootstrapPath(path)
			if !isAllowedPluginPagePath(entryArea, path) {
				continue
			}

			title := parseStringFromAny(extension.Title, data["title"], data["label"], extension.Content)
			if title == "" {
				continue
			}

			menuID := strings.TrimSpace(extension.ID)
			if menuID == "" {
				menuID = fmt.Sprintf("plugin-%d-menu-%d", extension.PluginID, idx)
			}

			menus = append(menus, frontendBootstrapMenuItem{
				ID:                  menuID,
				Area:                entryArea,
				Title:               title,
				Path:                path,
				Icon:                strings.TrimSpace(parseStringFromAny(data["icon"])),
				Priority:            parseIntFromAny(data["priority"], extension.Priority),
				RequiredPermissions: parseStringListFromAny(data["required_permissions"]),
				SuperAdminOnly:      parseBoolFromAny(data["super_admin_only"], false),
				GuestVisible:        parseBoolFromAny(data["guest_visible"], false),
				MobileVisible:       parseBoolFromAny(data["mobile_visible"], true),
				PluginID:            extension.PluginID,
				PluginName:          strings.TrimSpace(extension.PluginName),
			})
		case "route_page", "page_route", "route", "plugin_page":
			path := parseStringFromAny(extension.Link, data["path"], data["href"])
			if path == "" {
				continue
			}
			path = normalizeFrontendBootstrapPath(path)
			if !isAllowedPluginPagePath(entryArea, path) {
				continue
			}

			title := parseStringFromAny(extension.Title, data["title"], data["label"])
			if title == "" {
				title = path
			}

			routeID := strings.TrimSpace(extension.ID)
			if routeID == "" {
				routeID = fmt.Sprintf("plugin-%d-route-%d", extension.PluginID, idx)
			}

			pageSchema := map[string]interface{}{}
			if value, exists := data["page"]; exists {
				if pageMap, ok := value.(map[string]interface{}); ok {
					pageSchema = pageMap
				}
			}
			if len(pageSchema) == 0 {
				if value, exists := data["schema"]; exists {
					if pageMap, ok := value.(map[string]interface{}); ok {
						pageSchema = pageMap
					}
				}
			}
			if len(pageSchema) == 0 {
				pageSchema = map[string]interface{}{
					"title":       title,
					"description": parseStringFromAny(data["description"], extension.Content),
				}
			}
			allowedActions := collectFrontendRouteExecuteActions(data, pageSchema)
			allowedStreamActions := collectFrontendRouteStreamActions(data, pageSchema)

			routes = append(routes, frontendBootstrapRouteItem{
				ID:                   routeID,
				Area:                 entryArea,
				Title:                title,
				Path:                 path,
				Priority:             parseIntFromAny(data["priority"], extension.Priority),
				RequiredPermissions:  parseStringListFromAny(data["required_permissions"]),
				SuperAdminOnly:       parseBoolFromAny(data["super_admin_only"], false),
				GuestVisible:         parseBoolFromAny(data["guest_visible"], false),
				Page:                 pageSchema,
				AllowedActions:       allowedActions,
				AllowedStreamActions: allowedStreamActions,
				PluginID:             extension.PluginID,
				PluginName:           strings.TrimSpace(extension.PluginName),
			})
		}
	}

	sort.SliceStable(menus, func(i, j int) bool {
		if menus[i].Priority == menus[j].Priority {
			return menus[i].Path < menus[j].Path
		}
		return menus[i].Priority < menus[j].Priority
	})
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].Priority == routes[j].Priority {
			return routes[i].Path < routes[j].Path
		}
		return routes[i].Priority < routes[j].Priority
	})
	return menus, routes
}
