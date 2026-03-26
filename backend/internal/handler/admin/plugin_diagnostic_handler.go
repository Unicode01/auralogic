package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	pluginDiagnosticExecutionObservabilityWindow = 24 * time.Hour
	pluginDiagnosticRecentFailuresLimit          = 8
	pluginDiagnosticFailureGroupsLimit           = 8
	pluginDiagnosticStorageAccessMetaKey         = "storage_access_mode"
	pluginDiagnosticStorageExecutionScanLimit    = 24

	pluginDiagnosticStorageObservationSourceRecentTasks = "execution_tasks.recent"
	pluginDiagnosticStorageObservationSourcePersisted   = "plugin_executions.latest"
)

type pluginDiagnosticCheck struct {
	Key     string `json:"key"`
	State   string `json:"state"`
	Summary string `json:"summary"`
	Detail  string `json:"detail,omitempty"`
}

type pluginDiagnosticIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Summary  string `json:"summary"`
	Detail   string `json:"detail,omitempty"`
	Hint     string `json:"hint,omitempty"`
}

type pluginFrontendRouteScopeDiagnostic struct {
	Scope           string                                   `json:"scope"`
	Eligible        bool                                     `json:"eligible"`
	FrontendVisible bool                                     `json:"frontend_visible"`
	ReasonCode      string                                   `json:"reason_code,omitempty"`
	Reason          string                                   `json:"reason,omitempty"`
	Diagnostic      service.PluginHookParticipationDiagnosis `json:"diagnostic"`
}

type pluginFrontendRouteDiagnostic struct {
	Area                string                               `json:"area"`
	Path                string                               `json:"path"`
	ExecuteAPIAvailable bool                                 `json:"execute_api_available"`
	ScopeChecks         []pluginFrontendRouteScopeDiagnostic `json:"scope_checks"`
}

type pluginPublicCacheEntryDiagnostic struct {
	Key            string    `json:"key"`
	CreatedAt      time.Time `json:"created_at,omitempty"`
	ExpiresAt      time.Time `json:"expires_at,omitempty"`
	Area           string    `json:"area,omitempty"`
	Path           string    `json:"path,omitempty"`
	Slot           string    `json:"slot,omitempty"`
	ExtensionCount int       `json:"extension_count,omitempty"`
	MenuCount      int       `json:"menu_count,omitempty"`
	RouteCount     int       `json:"route_count,omitempty"`
}

type pluginPublicCacheBucketDiagnostic struct {
	TotalEntries    int                                `json:"total_entries"`
	MatchingEntries int                                `json:"matching_entries"`
	Entries         []pluginPublicCacheEntryDiagnostic `json:"entries"`
}

type pluginPublicCacheDiagnostics struct {
	TTLSeconds int                               `json:"ttl_seconds"`
	MaxEntries int                               `json:"max_entries"`
	Extensions pluginPublicCacheBucketDiagnostic `json:"extensions"`
	Bootstrap  pluginPublicCacheBucketDiagnostic `json:"bootstrap"`
}

type pluginExecutionFailureSampleDiagnostic struct {
	ID        uint      `json:"id"`
	Action    string    `json:"action"`
	Hook      string    `json:"hook,omitempty"`
	Error     string    `json:"error,omitempty"`
	Duration  int       `json:"duration"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type pluginExecutionFailureGroupDiagnostic struct {
	Action        string     `json:"action"`
	Hook          string     `json:"hook,omitempty"`
	FailureCount  int        `json:"failure_count"`
	LastFailureAt *time.Time `json:"last_failure_at,omitempty"`
}

type pluginExecutionObservabilityDiagnostics struct {
	WindowHours            int                                      `json:"window_hours"`
	TotalExecutions        int                                      `json:"total_executions"`
	FailedExecutions       int                                      `json:"failed_executions"`
	HookFailedExecutions   int                                      `json:"hook_failed_executions"`
	ActionFailedExecutions int                                      `json:"action_failed_executions"`
	LastFailureAt          *time.Time                               `json:"last_failure_at,omitempty"`
	LastSuccessAt          *time.Time                               `json:"last_success_at,omitempty"`
	FailureGroups          []pluginExecutionFailureGroupDiagnostic  `json:"failure_groups"`
	RecentFailures         []pluginExecutionFailureSampleDiagnostic `json:"recent_failures"`
}

type pluginStorageActionProfileDiagnostic struct {
	Action string `json:"action"`
	Mode   string `json:"mode"`
}

type pluginStorageObservationDiagnostic struct {
	Source             string     `json:"source"`
	TaskID             string     `json:"task_id,omitempty"`
	Action             string     `json:"action,omitempty"`
	Hook               string     `json:"hook,omitempty"`
	Status             string     `json:"status,omitempty"`
	Stream             bool       `json:"stream"`
	DeclaredAccessMode string     `json:"declared_access_mode,omitempty"`
	ObservedAccessMode string     `json:"observed_access_mode,omitempty"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
}

type pluginStorageDiagnostics struct {
	ProfileCount      int                                    `json:"profile_count"`
	DeclaredProfiles  []pluginStorageActionProfileDiagnostic `json:"declared_profiles"`
	LastObserved      *pluginStorageObservationDiagnostic    `json:"last_observed,omitempty"`
	HasObservedAccess bool                                   `json:"has_observed_access"`
}

type pluginDiagnosticsResponse struct {
	Plugin                 pluginResponse                                `json:"plugin"`
	Runtime                service.PluginRuntimeInspection               `json:"runtime"`
	Compatibility          service.PluginProtocolCompatibilityInspection `json:"compatibility"`
	Registration           service.PluginRegistrationInspection          `json:"registration"`
	RecentDeployments      []pluginDeploymentResponse                    `json:"recent_deployments"`
	ExecutionTasks         service.PluginExecutionTaskOverview           `json:"execution_tasks"`
	ExecutionObservability pluginExecutionObservabilityDiagnostics       `json:"execution_observability"`
	StorageDiagnostics     pluginStorageDiagnostics                      `json:"storage_diagnostics"`
	PublicCache            pluginPublicCacheDiagnostics                  `json:"public_cache"`
	MissingPermissions     []string                                      `json:"missing_permissions"`
	Checks                 []pluginDiagnosticCheck                       `json:"checks"`
	FrontendRoutes         []pluginFrontendRouteDiagnostic               `json:"frontend_routes"`
	RequestedHook          *service.PluginHookParticipationDiagnosis     `json:"requested_hook,omitempty"`
	Issues                 []pluginDiagnosticIssue                       `json:"issues"`
}

func (h *PluginHandler) GetPluginDiagnostics(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, id).Error; err != nil {
		h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
		return
	}

	effectivePolicy := service.ResolveEffectivePluginCapabilityPolicy(&plugin)
	runtimeInspection := buildPluginRuntimeInspectionUnavailable(&plugin)
	registrationInspection := service.PluginRegistrationInspection{
		State:   "unavailable",
		Runtime: strings.TrimSpace(plugin.Runtime),
		Detail:  "plugin manager is unavailable",
	}
	executionTasks := service.PluginExecutionTaskOverview{
		Active: []service.PluginExecutionTaskSnapshot{},
		Recent: []service.PluginExecutionTaskSnapshot{},
	}
	if h.pluginManager != nil {
		runtimeInspection = h.pluginManager.InspectPluginRuntime(&plugin)
		registrationInspection = h.pluginManager.InspectPluginRegistration(&plugin)
		executionTasks = h.pluginManager.InspectPluginExecutionTasks(plugin.ID, 10, 10)
	}
	missingPermissions := diffNormalizedStringLists(
		effectivePolicy.RequestedPermissions,
		effectivePolicy.GrantedPermissions,
	)
	compatibility := service.InspectPluginProtocolCompatibility(&plugin)
	frontendRoutes := h.buildPluginFrontendRouteDiagnostics(&plugin)
	requestedHook := h.buildRequestedPluginHookDiagnostic(c, &plugin)
	publicCache := h.buildPluginPublicCacheDiagnostics(&plugin)
	executionObservability := h.buildPluginExecutionObservabilityDiagnostics(&plugin)
	storageExecutions := h.listRecentPluginExecutionsForStorageDiagnostics(plugin.ID, pluginDiagnosticStorageExecutionScanLimit)
	storageDiagnostics := buildPluginStorageDiagnostics(effectivePolicy, storageExecutions, executionTasks)
	recentDeployments := h.listRecentPluginDeployments(plugin.ID, 10)
	latestDeploymentMap := h.getLatestPluginDeployments([]uint{plugin.ID})
	pluginResp := buildPluginResponse(plugin, h.uploadDir)
	if latestDeployment, exists := latestDeploymentMap[plugin.ID]; exists {
		pluginResp = buildPluginResponseWithDeployment(plugin, &latestDeployment, h.uploadDir)
	}

	resp := pluginDiagnosticsResponse{
		Plugin:                 pluginResp,
		Runtime:                runtimeInspection,
		Compatibility:          compatibility,
		Registration:           registrationInspection,
		RecentDeployments:      buildPluginDeploymentResponses(recentDeployments),
		ExecutionTasks:         executionTasks,
		ExecutionObservability: executionObservability,
		StorageDiagnostics:     storageDiagnostics,
		PublicCache:            publicCache,
		MissingPermissions:     missingPermissions,
		Checks: buildPluginDiagnosticChecks(
			plugin,
			runtimeInspection,
			compatibility,
			registrationInspection,
			effectivePolicy,
			missingPermissions,
			storageDiagnostics,
		),
		FrontendRoutes: frontendRoutes,
		RequestedHook:  requestedHook,
		Issues: buildPluginDiagnosticIssues(
			plugin,
			runtimeInspection,
			compatibility,
			registrationInspection,
			effectivePolicy,
			missingPermissions,
			storageDiagnostics,
			frontendRoutes,
		),
	}
	c.JSON(http.StatusOK, resp)
}

func (h *PluginHandler) buildPluginFrontendRouteDiagnostics(plugin *models.Plugin) []pluginFrontendRouteDiagnostic {
	if plugin == nil {
		return []pluginFrontendRouteDiagnostic{}
	}

	adminPath, userPath := extractPluginManifestPagePaths(plugin.Manifest)
	executeAPIAvailable := parseFrontendRouteExecuteAPIAvailability(plugin.Capabilities)
	routes := make([]pluginFrontendRouteDiagnostic, 0, 2)

	if adminPath != "" {
		payload := map[string]interface{}{
			"area": frontendBootstrapAreaAdmin,
			"path": adminPath,
			"slot": "bootstrap",
		}
		diag := service.DiagnosePluginHookParticipation(
			plugin,
			"frontend.bootstrap",
			payload,
			buildPluginDiagnosticExecutionContext(pluginAccessScope{
				authenticated: true,
				superAdmin:    true,
				permissions:   map[string]struct{}{},
			}),
		)
		diag = h.applyPluginRuntimeExecutionGate(plugin, diag)
		routes = append(routes, pluginFrontendRouteDiagnostic{
			Area:                frontendBootstrapAreaAdmin,
			Path:                adminPath,
			ExecuteAPIAvailable: executeAPIAvailable,
			ScopeChecks: []pluginFrontendRouteScopeDiagnostic{
				buildPluginFrontendRouteScopeDiagnostic("super_admin", diag),
			},
		})
	}

	if userPath != "" {
		guestDiag := service.DiagnosePluginHookParticipation(
			plugin,
			"frontend.bootstrap",
			map[string]interface{}{
				"area": frontendBootstrapAreaUser,
				"path": userPath,
				"slot": "bootstrap",
			},
			buildPluginDiagnosticExecutionContext(pluginAccessScope{
				permissions: map[string]struct{}{},
			}),
		)
		guestDiag = h.applyPluginRuntimeExecutionGate(plugin, guestDiag)
		authDiag := service.DiagnosePluginHookParticipation(
			plugin,
			"frontend.bootstrap",
			map[string]interface{}{
				"area": frontendBootstrapAreaUser,
				"path": userPath,
				"slot": "bootstrap",
			},
			buildPluginDiagnosticExecutionContext(pluginAccessScope{
				authenticated: true,
				permissions:   map[string]struct{}{},
			}),
		)
		authDiag = h.applyPluginRuntimeExecutionGate(plugin, authDiag)
		routes = append(routes, pluginFrontendRouteDiagnostic{
			Area:                frontendBootstrapAreaUser,
			Path:                userPath,
			ExecuteAPIAvailable: executeAPIAvailable,
			ScopeChecks: []pluginFrontendRouteScopeDiagnostic{
				buildPluginFrontendRouteScopeDiagnostic("guest", guestDiag),
				buildPluginFrontendRouteScopeDiagnostic("authenticated", authDiag),
			},
		})
	}

	return routes
}

func buildPluginFrontendRouteScopeDiagnostic(
	scope string,
	diag service.PluginHookParticipationDiagnosis,
) pluginFrontendRouteScopeDiagnostic {
	return pluginFrontendRouteScopeDiagnostic{
		Scope:           scope,
		Eligible:        diag.Participates,
		FrontendVisible: diag.Participates && diag.AllowFrontendExtensions,
		ReasonCode:      strings.TrimSpace(diag.ReasonCode),
		Reason:          strings.TrimSpace(diag.Reason),
		Diagnostic:      diag,
	}
}

func buildPluginDiagnosticExecutionContext(scope pluginAccessScope) *service.ExecutionContext {
	metadata := map[string]string{}
	applyPluginAccessScopeMetadata(metadata, scope)
	return &service.ExecutionContext{
		SessionID: "plugin-diagnostics",
		Metadata:  metadata,
	}
}

func (h *PluginHandler) buildRequestedPluginHookDiagnostic(
	c *gin.Context,
	plugin *models.Plugin,
) *service.PluginHookParticipationDiagnosis {
	if c == nil || plugin == nil {
		return nil
	}
	hook := strings.TrimSpace(c.Query("hook"))
	if hook == "" {
		return nil
	}

	payload := map[string]interface{}{}
	scopeMode := strings.ToLower(strings.TrimSpace(c.Query("scope")))
	scope := pluginAccessScope{permissions: map[string]struct{}{}}
	switch scopeMode {
	case "authenticated", "auth", "user":
		scope.authenticated = true
	case "super_admin", "superadmin", "admin":
		scope.authenticated = true
		scope.superAdmin = true
	}
	if permissionsRaw := strings.TrimSpace(c.Query("permissions")); permissionsRaw != "" {
		scope.permissions = buildPermissionSet(strings.Split(permissionsRaw, ","))
		if len(scope.permissions) > 0 {
			scope.authenticated = true
		}
	}

	normalizedHook := strings.ToLower(strings.TrimSpace(hook))
	switch normalizedHook {
	case "frontend.bootstrap":
		area := normalizeFrontendBootstrapArea(c.Query("area"))
		path := normalizeFrontendBootstrapPath(c.Query("path"))
		if path == "" || path == "/" {
			if area == frontendBootstrapAreaAdmin {
				path = "/admin"
			} else {
				path = "/"
			}
		}
		payload["area"] = area
		payload["path"] = path
		payload["slot"] = "bootstrap"
	case "frontend.slot.render":
		slot := normalizedSlotValue(c.Query("slot"))
		if slot == "" {
			slot = "user.plugin_page.top"
		}
		path := normalizeFrontendBootstrapPath(c.Query("path"))
		if path == "" || path == "/" {
			if strings.HasPrefix(slot, "admin.") {
				path = "/admin"
			} else {
				path = "/"
			}
		}
		payload["slot"] = slot
		payload["path"] = path
	default:
		if path := strings.TrimSpace(c.Query("path")); path != "" {
			payload["path"] = normalizeFrontendBootstrapPath(path)
		}
		if slot := normalizedSlotValue(c.Query("slot")); slot != "" {
			payload["slot"] = slot
		}
		if area := strings.TrimSpace(c.Query("area")); area != "" {
			payload["area"] = normalizeFrontendBootstrapArea(area)
		}
	}

	diag := service.DiagnosePluginHookParticipation(
		plugin,
		hook,
		payload,
		buildPluginDiagnosticExecutionContext(scope),
	)
	diag = h.applyPluginRuntimeExecutionGate(plugin, diag)
	return &diag
}

func (h *PluginHandler) applyPluginRuntimeExecutionGate(
	plugin *models.Plugin,
	diag service.PluginHookParticipationDiagnosis,
) service.PluginHookParticipationDiagnosis {
	if plugin == nil || h == nil || h.pluginManager == nil {
		return diag
	}

	runtime := h.pluginManager.InspectPluginRuntime(plugin)
	if !diag.Participates {
		return diag
	}

	switch runtime.BreakerState {
	case "open":
		diag.Participates = false
		diag.ReasonCode = "plugin_cooldown_active"
		diag.Reason = firstNonEmpty(
			strings.TrimSpace(runtime.CooldownReason),
			"plugin execution is temporarily cooled down after repeated failures",
		)
	case "half_open":
		if runtime.ProbeInFlight {
			diag.Participates = false
			diag.ReasonCode = "plugin_half_open_probe_in_flight"
			diag.Reason = firstNonEmpty(
				strings.TrimSpace(runtime.CooldownReason),
				"plugin is already running a single half-open recovery probe",
			)
		}
	}
	return diag
}

func buildPluginRuntimeInspectionUnavailable(plugin *models.Plugin) service.PluginRuntimeInspection {
	inspection := service.PluginRuntimeInspection{
		ConnectionState: "unavailable",
	}
	if plugin == nil {
		inspection.LastError = "plugin record is unavailable"
		return inspection
	}
	inspection.ConfiguredRuntime = strings.TrimSpace(plugin.Runtime)
	inspection.Enabled = plugin.Enabled
	inspection.LifecycleStatus = strings.TrimSpace(plugin.LifecycleStatus)
	inspection.HealthStatus = strings.TrimSpace(plugin.Status)
	inspection.AddressPresent = strings.TrimSpace(plugin.Address) != ""
	inspection.PackagePathPresent = strings.TrimSpace(plugin.PackagePath) != ""
	inspection.FailureCount = plugin.FailCount
	if inspection.FailureCount > 0 {
		inspection.BreakerState = "unknown"
	} else {
		inspection.BreakerState = "closed"
	}
	inspection.LastError = strings.TrimSpace(plugin.LastError)
	return inspection
}

func (h *PluginHandler) buildPluginExecutionObservabilityDiagnostics(
	plugin *models.Plugin,
) pluginExecutionObservabilityDiagnostics {
	diag := pluginExecutionObservabilityDiagnostics{
		WindowHours:    int(pluginDiagnosticExecutionObservabilityWindow / time.Hour),
		FailureGroups:  []pluginExecutionFailureGroupDiagnostic{},
		RecentFailures: []pluginExecutionFailureSampleDiagnostic{},
	}
	if h == nil || h.db == nil || plugin == nil || plugin.ID == 0 {
		return diag
	}

	windowStart := time.Now().UTC().Add(-pluginDiagnosticExecutionObservabilityWindow)

	type aggregateRow struct {
		TotalExecutions        int64 `gorm:"column:total_executions"`
		FailedExecutions       int64 `gorm:"column:failed_executions"`
		HookFailedExecutions   int64 `gorm:"column:hook_failed_executions"`
		ActionFailedExecutions int64 `gorm:"column:action_failed_executions"`
	}
	var aggregate aggregateRow
	if err := h.db.Model(&models.PluginExecution{}).
		Where("plugin_id = ? AND created_at >= ?", plugin.ID, windowStart).
		Select(`
			COUNT(*) AS total_executions,
			COALESCE(SUM(CASE WHEN success THEN 0 ELSE 1 END), 0) AS failed_executions,
			COALESCE(SUM(CASE WHEN success THEN 0 ELSE CASE WHEN action = 'hook.execute' THEN 1 ELSE 0 END END), 0) AS hook_failed_executions,
			COALESCE(SUM(CASE WHEN success THEN 0 ELSE CASE WHEN action <> 'hook.execute' THEN 1 ELSE 0 END END), 0) AS action_failed_executions
		`).
		Scan(&aggregate).Error; err != nil {
		return diag
	}

	diag.TotalExecutions = int(aggregate.TotalExecutions)
	diag.FailedExecutions = int(aggregate.FailedExecutions)
	diag.HookFailedExecutions = int(aggregate.HookFailedExecutions)
	diag.ActionFailedExecutions = int(aggregate.ActionFailedExecutions)
	var lastFailed models.PluginExecution
	if err := h.db.Where("plugin_id = ? AND created_at >= ? AND success = ?", plugin.ID, windowStart, false).
		Order("created_at DESC").
		First(&lastFailed).Error; err == nil && !lastFailed.CreatedAt.IsZero() {
		lastFailureAt := lastFailed.CreatedAt.UTC()
		diag.LastFailureAt = &lastFailureAt
	}
	var lastSucceeded models.PluginExecution
	if err := h.db.Where("plugin_id = ? AND created_at >= ? AND success = ?", plugin.ID, windowStart, true).
		Order("created_at DESC").
		First(&lastSucceeded).Error; err == nil && !lastSucceeded.CreatedAt.IsZero() {
		lastSuccessAt := lastSucceeded.CreatedAt.UTC()
		diag.LastSuccessAt = &lastSuccessAt
	}

	var recentFailures []models.PluginExecution
	if err := h.db.Where("plugin_id = ? AND created_at >= ? AND success = ?", plugin.ID, windowStart, false).
		Select("id, action, hook, params, metadata, error, duration, created_at").
		Order("created_at DESC").
		Limit(pluginDiagnosticRecentFailuresLimit).
		Find(&recentFailures).Error; err != nil {
		return diag
	}
	for _, execution := range recentFailures {
		diag.RecentFailures = append(diag.RecentFailures, pluginExecutionFailureSampleDiagnostic{
			ID:        execution.ID,
			Action:    strings.TrimSpace(execution.Action),
			Hook:      resolvePluginExecutionObservedHook(execution.Action, execution.Hook, execution.Params, execution.Metadata),
			Error:     strings.TrimSpace(execution.Error),
			Duration:  execution.Duration,
			CreatedAt: execution.CreatedAt.UTC(),
		})
	}

	type failureGroupRow struct {
		Action           string `gorm:"column:action"`
		Hook             string `gorm:"column:hook"`
		FailureCount     int64  `gorm:"column:failure_count"`
		LastFailureAtRaw string `gorm:"column:last_failure_at"`
	}
	var failureGroups []failureGroupRow
	if err := h.db.Model(&models.PluginExecution{}).
		Where("plugin_id = ? AND created_at >= ? AND success = ?", plugin.ID, windowStart, false).
		Select("action, hook, COUNT(*) AS failure_count, MAX(created_at) AS last_failure_at").
		Group("action, hook").
		Order("failure_count DESC").
		Order("last_failure_at DESC").
		Order("hook ASC").
		Order("action ASC").
		Limit(pluginDiagnosticFailureGroupsLimit).
		Scan(&failureGroups).Error; err != nil {
		return diag
	}
	diag.FailureGroups = make([]pluginExecutionFailureGroupDiagnostic, 0, len(failureGroups))
	for _, group := range failureGroups {
		lastFailureAt, ok := parsePluginAggregateTimestamp(group.LastFailureAtRaw)
		if !ok {
			lastFailureAt = time.Time{}
		}
		diag.FailureGroups = append(diag.FailureGroups, pluginExecutionFailureGroupDiagnostic{
			Action:        strings.TrimSpace(group.Action),
			Hook:          strings.TrimSpace(group.Hook),
			FailureCount:  int(group.FailureCount),
			LastFailureAt: optionalPluginAggregateTimestamp(lastFailureAt),
		})
	}

	return diag
}

func (h *PluginHandler) listRecentPluginExecutionsForStorageDiagnostics(
	pluginID uint,
	limit int,
) []models.PluginExecution {
	if h == nil || h.db == nil || pluginID == 0 {
		return []models.PluginExecution{}
	}
	if limit <= 0 {
		limit = pluginDiagnosticStorageExecutionScanLimit
	}

	executions := make([]models.PluginExecution, 0, limit)
	if err := h.db.Model(&models.PluginExecution{}).
		Where("plugin_id = ?", pluginID).
		Select("id, action, hook, params, metadata, success, created_at").
		Order("created_at DESC").
		Limit(limit).
		Find(&executions).Error; err != nil {
		return []models.PluginExecution{}
	}
	return executions
}

func parsePluginExecutionObservedHook(action string, paramsJSON string) string {
	if strings.TrimSpace(action) != "hook.execute" {
		return ""
	}
	trimmed := strings.TrimSpace(paramsJSON)
	if trimmed == "" {
		return ""
	}
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &params); err != nil {
		return ""
	}
	hook, _ := params["hook"].(string)
	return strings.TrimSpace(hook)
}

func resolvePluginExecutionObservedHook(
	action string,
	hook string,
	paramsJSON string,
	metadata models.JSONMap,
) string {
	if strings.TrimSpace(action) != "hook.execute" {
		return ""
	}
	if normalized := strings.TrimSpace(hook); normalized != "" {
		return normalized
	}
	if hook := strings.TrimSpace(metadata[service.PluginExecutionMetadataHook]); hook != "" {
		return hook
	}
	return parsePluginExecutionObservedHook(action, paramsJSON)
}

func buildPluginStorageDiagnostics(
	policy service.EffectivePluginCapabilityPolicy,
	executions []models.PluginExecution,
	executionTasks service.PluginExecutionTaskOverview,
) pluginStorageDiagnostics {
	diag := pluginStorageDiagnostics{
		DeclaredProfiles: []pluginStorageActionProfileDiagnostic{},
	}

	if len(policy.ExecuteActionStorage) > 0 {
		diag.DeclaredProfiles = make([]pluginStorageActionProfileDiagnostic, 0, len(policy.ExecuteActionStorage))
		for action, mode := range policy.ExecuteActionStorage {
			diag.DeclaredProfiles = append(diag.DeclaredProfiles, pluginStorageActionProfileDiagnostic{
				Action: strings.TrimSpace(action),
				Mode:   normalizePluginDiagnosticStorageAccessMode(mode),
			})
		}
		sort.SliceStable(diag.DeclaredProfiles, func(i, j int) bool {
			return diag.DeclaredProfiles[i].Action < diag.DeclaredProfiles[j].Action
		})
	}
	diag.ProfileCount = len(diag.DeclaredProfiles)

	persistedObservation := buildPersistedPluginStorageObservation(policy, executions)
	recentObservation := buildRecentTaskPluginStorageObservation(policy, executionTasks)
	diag.LastObserved = chooseLatestPluginStorageObservation(persistedObservation, recentObservation)
	diag.HasObservedAccess = diag.LastObserved != nil
	return diag
}

func buildPersistedPluginStorageObservation(
	policy service.EffectivePluginCapabilityPolicy,
	executions []models.PluginExecution,
) *pluginStorageObservationDiagnostic {
	for _, execution := range executions {
		observedMode := normalizePluginDiagnosticStorageAccessMode(execution.Metadata[pluginDiagnosticStorageAccessMetaKey])
		if observedMode == "" || observedMode == "unknown" {
			continue
		}

		completedAt := execution.CreatedAt.UTC()
		hook := resolvePluginExecutionObservedHook(execution.Action, execution.Hook, execution.Params, execution.Metadata)
		stream, _ := strconv.ParseBool(strings.TrimSpace(execution.Metadata[service.PluginExecutionMetadataStream]))
		status := strings.TrimSpace(execution.Metadata[service.PluginExecutionMetadataStatus])
		if status == "" {
			if execution.Success {
				status = service.PluginExecutionStatusCompleted
			} else {
				status = service.PluginExecutionStatusFailed
			}
		}
		return &pluginStorageObservationDiagnostic{
			Source:             pluginDiagnosticStorageObservationSourcePersisted,
			TaskID:             strings.TrimSpace(execution.Metadata[service.PluginExecutionMetadataID]),
			Action:             strings.TrimSpace(execution.Action),
			Hook:               hook,
			Status:             status,
			Stream:             stream,
			DeclaredAccessMode: resolvePluginDiagnosticDeclaredStorageAccessMode(policy.ExecuteActionStorage, execution.Action),
			ObservedAccessMode: observedMode,
			UpdatedAt:          &completedAt,
			CompletedAt:        &completedAt,
		}
	}

	return nil
}

func buildRecentTaskPluginStorageObservation(
	policy service.EffectivePluginCapabilityPolicy,
	executionTasks service.PluginExecutionTaskOverview,
) *pluginStorageObservationDiagnostic {
	for _, snapshot := range executionTasks.Recent {
		observedMode := normalizePluginDiagnosticStorageAccessMode(snapshot.Metadata[pluginDiagnosticStorageAccessMetaKey])
		if observedMode == "" || observedMode == "unknown" {
			continue
		}

		updatedAt := snapshot.UpdatedAt.UTC()
		return &pluginStorageObservationDiagnostic{
			Source:             pluginDiagnosticStorageObservationSourceRecentTasks,
			TaskID:             strings.TrimSpace(snapshot.ID),
			Action:             strings.TrimSpace(snapshot.Action),
			Hook:               strings.TrimSpace(snapshot.Hook),
			Status:             strings.TrimSpace(snapshot.Status),
			Stream:             snapshot.Stream,
			DeclaredAccessMode: resolvePluginDiagnosticDeclaredStorageAccessMode(policy.ExecuteActionStorage, snapshot.Action),
			ObservedAccessMode: observedMode,
			UpdatedAt:          &updatedAt,
			CompletedAt:        cloneOptionalDiagnosticTime(snapshot.CompletedAt),
		}
	}

	return nil
}

func chooseLatestPluginStorageObservation(
	left *pluginStorageObservationDiagnostic,
	right *pluginStorageObservationDiagnostic,
) *pluginStorageObservationDiagnostic {
	switch {
	case left == nil:
		return right
	case right == nil:
		return left
	}

	leftTime := pluginStorageObservationUpdatedAt(left)
	rightTime := pluginStorageObservationUpdatedAt(right)
	switch {
	case leftTime.After(rightTime):
		return left
	case rightTime.After(leftTime):
		return right
	case left.Source == pluginDiagnosticStorageObservationSourcePersisted:
		return left
	default:
		return right
	}
}

func pluginStorageObservationUpdatedAt(observation *pluginStorageObservationDiagnostic) time.Time {
	if observation == nil {
		return time.Time{}
	}
	if observation.UpdatedAt != nil {
		return observation.UpdatedAt.UTC()
	}
	if observation.CompletedAt != nil {
		return observation.CompletedAt.UTC()
	}
	return time.Time{}
}

func normalizePluginDiagnosticStorageAccessMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "none":
		return "none"
	case "read":
		return "read"
	case "write":
		return "write"
	case "unknown":
		return "unknown"
	default:
		return ""
	}
}

func resolvePluginDiagnosticDeclaredStorageAccessMode(
	profiles map[string]string,
	action string,
) string {
	if len(profiles) == 0 {
		return "unknown"
	}
	mode, exists := profiles[strings.ToLower(strings.TrimSpace(action))]
	if !exists {
		return "unknown"
	}
	normalized := normalizePluginDiagnosticStorageAccessMode(mode)
	if normalized == "" {
		return "unknown"
	}
	return normalized
}

func cloneOptionalDiagnosticTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := value.UTC()
	return &copied
}

func parsePluginAggregateTimestamp(raw string) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999999Z07:00",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, trimmed)
		if err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func optionalPluginAggregateTimestamp(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value.UTC()
	return &copied
}

func (h *PluginHandler) buildPluginPublicCacheDiagnostics(plugin *models.Plugin) pluginPublicCacheDiagnostics {
	diag := pluginPublicCacheDiagnostics{
		TTLSeconds: int(pluginPublicEndpointCacheTTL / time.Second),
		MaxEntries: pluginPublicEndpointCacheMaxEntries,
		Extensions: pluginPublicCacheBucketDiagnostic{Entries: []pluginPublicCacheEntryDiagnostic{}},
		Bootstrap:  pluginPublicCacheBucketDiagnostic{Entries: []pluginPublicCacheEntryDiagnostic{}},
	}
	if h == nil || plugin == nil {
		return diag
	}

	if h.publicExtensionsCache != nil {
		if h.publicExtensionsCache.ttl > 0 {
			diag.TTLSeconds = int(h.publicExtensionsCache.ttl / time.Second)
		}
		if h.publicExtensionsCache.maxEntries > 0 {
			diag.MaxEntries = h.publicExtensionsCache.maxEntries
		}
		snapshots := h.publicExtensionsCache.Snapshot()
		diag.Extensions.TotalEntries = len(snapshots)
		for _, entry := range snapshots {
			payload, ok := entry.Value.(frontendExtensionsResponseData)
			if !ok {
				continue
			}
			extensionCount := countPluginFrontendExtensions(payload.Extensions, plugin.ID)
			if extensionCount == 0 {
				continue
			}
			diag.Extensions.MatchingEntries++
			diag.Extensions.Entries = append(diag.Extensions.Entries, pluginPublicCacheEntryDiagnostic{
				Key:            entry.Key,
				CreatedAt:      entry.CreatedAt,
				ExpiresAt:      entry.ExpiresAt,
				Path:           payload.Path,
				Slot:           payload.Slot,
				ExtensionCount: extensionCount,
			})
		}
	}

	if h.publicBootstrapCache != nil {
		snapshots := h.publicBootstrapCache.Snapshot()
		diag.Bootstrap.TotalEntries = len(snapshots)
		for _, entry := range snapshots {
			payload, ok := entry.Value.(frontendBootstrapResponseData)
			if !ok {
				continue
			}
			menuCount, routeCount := countPluginFrontendBootstrapEntries(payload, plugin.ID)
			if menuCount == 0 && routeCount == 0 {
				continue
			}
			diag.Bootstrap.MatchingEntries++
			diag.Bootstrap.Entries = append(diag.Bootstrap.Entries, pluginPublicCacheEntryDiagnostic{
				Key:        entry.Key,
				CreatedAt:  entry.CreatedAt,
				ExpiresAt:  entry.ExpiresAt,
				Area:       payload.Area,
				Path:       payload.Path,
				MenuCount:  menuCount,
				RouteCount: routeCount,
			})
		}
	}

	return diag
}

func countPluginFrontendExtensions(items []service.FrontendExtension, pluginID uint) int {
	if pluginID == 0 || len(items) == 0 {
		return 0
	}
	count := 0
	for _, item := range items {
		if item.PluginID == pluginID {
			count++
		}
	}
	return count
}

func countPluginFrontendBootstrapEntries(payload frontendBootstrapResponseData, pluginID uint) (int, int) {
	if pluginID == 0 {
		return 0, 0
	}
	menuCount := 0
	for _, item := range payload.Menus {
		if item.PluginID == pluginID {
			menuCount++
		}
	}
	routeCount := 0
	for _, item := range payload.Routes {
		if item.PluginID == pluginID {
			routeCount++
		}
	}
	return menuCount, routeCount
}

func buildPluginDiagnosticChecks(
	plugin models.Plugin,
	runtime service.PluginRuntimeInspection,
	compatibility service.PluginProtocolCompatibilityInspection,
	registration service.PluginRegistrationInspection,
	policy service.EffectivePluginCapabilityPolicy,
	missingPermissions []string,
	storageDiagnostics pluginStorageDiagnostics,
) []pluginDiagnosticCheck {
	checks := []pluginDiagnosticCheck{
		{
			Key:     "runtime_connection",
			State:   resolvePluginRuntimeCheckState(plugin, runtime),
			Summary: fmt.Sprintf("runtime=%s connection=%s", firstNonEmpty(runtime.ResolvedRuntime, runtime.ConfiguredRuntime, "unknown"), firstNonEmpty(runtime.ConnectionState, "unknown")),
			Detail:  firstNonEmpty(runtime.LastError, plugin.LastError),
		},
		{
			Key:     "protocol_compatibility",
			State:   resolvePluginCompatibilityCheckState(compatibility),
			Summary: buildPluginCompatibilityCheckSummary(compatibility),
			Detail:  strings.TrimSpace(compatibility.Reason),
		},
		{
			Key:     "registration_outcome",
			State:   resolvePluginRegistrationCheckState(registration),
			Summary: buildPluginRegistrationCheckSummary(registration),
			Detail:  strings.TrimSpace(registration.Detail),
		},
		{
			Key:     "hook_execute",
			State:   boolCheckState(policy.AllowHookExecute, "disabled"),
			Summary: boolCheckSummary(policy.AllowHookExecute, "Hook execution is enabled", "Hook execution is disabled"),
		},
		{
			Key:     "frontend_extensions",
			State:   boolCheckState(policy.AllowFrontendExtensions, "disabled"),
			Summary: boolCheckSummary(policy.AllowFrontendExtensions, "Frontend extensions are enabled", "Frontend extensions are disabled"),
		},
		{
			Key:     "manual_execute",
			State:   boolCheckState(policy.AllowExecuteAPI, "disabled"),
			Summary: boolCheckSummary(policy.AllowExecuteAPI, "Admin execute API is enabled", "Admin execute API is disabled"),
		},
		{
			Key:     "runtime_network",
			State:   boolCheckState(policy.AllowNetwork, "restricted"),
			Summary: boolCheckSummary(policy.AllowNetwork, "Runtime network access is granted", "Runtime network access is restricted"),
		},
		{
			Key:     "runtime_file_system",
			State:   boolCheckState(policy.AllowFileSystem, "restricted"),
			Summary: boolCheckSummary(policy.AllowFileSystem, "Runtime file-system access is granted", "Runtime file-system access is restricted"),
		},
	}
	if strings.EqualFold(strings.TrimSpace(firstNonEmpty(runtime.ResolvedRuntime, runtime.ConfiguredRuntime, plugin.Runtime)), service.PluginRuntimeJSWorker) {
		storageCheck := pluginDiagnosticCheck{
			Key:     "execute_action_storage",
			State:   "warn",
			Summary: "No execute_action_storage profiles declared. js_worker actions fall back to conservative serialized Plugin.storage access.",
		}
		if storageDiagnostics.ProfileCount > 0 {
			storageCheck.State = "ok"
			storageCheck.Summary = fmt.Sprintf(
				"execute_action_storage declares %d action profile(s) for js_worker Plugin.storage access.",
				storageDiagnostics.ProfileCount,
			)
			if storageDiagnostics.LastObserved != nil {
				storageCheck.Detail = fmt.Sprintf(
					"latest observed action=%s declared=%s observed=%s",
					firstNonEmpty(storageDiagnostics.LastObserved.Action, "-"),
					firstNonEmpty(storageDiagnostics.LastObserved.DeclaredAccessMode, "unknown"),
					firstNonEmpty(storageDiagnostics.LastObserved.ObservedAccessMode, "unknown"),
				)
			}
		}
		checks = append(checks, storageCheck)
	}
	if len(missingPermissions) > 0 {
		checks = append(checks, pluginDiagnosticCheck{
			Key:     "permission_grants",
			State:   "warn",
			Summary: "Some requested permissions are not granted",
			Detail:  strings.Join(missingPermissions, ", "),
		})
	} else {
		checks = append(checks, pluginDiagnosticCheck{
			Key:     "permission_grants",
			State:   "ok",
			Summary: "Requested permissions are fully granted",
		})
	}
	return checks
}

func buildPluginDiagnosticIssues(
	plugin models.Plugin,
	runtime service.PluginRuntimeInspection,
	compatibility service.PluginProtocolCompatibilityInspection,
	registration service.PluginRegistrationInspection,
	policy service.EffectivePluginCapabilityPolicy,
	missingPermissions []string,
	storageDiagnostics pluginStorageDiagnostics,
	routes []pluginFrontendRouteDiagnostic,
) []pluginDiagnosticIssue {
	issues := make([]pluginDiagnosticIssue, 0, 8)
	if !runtime.Valid && runtime.ConnectionState != "unavailable" {
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "runtime_invalid",
			Severity: "error",
			Summary:  "Configured runtime is unsupported",
			Detail:   firstNonEmpty(plugin.Runtime, "unknown"),
			Hint:     "Update the plugin runtime to grpc or js_worker.",
		})
	}
	if plugin.Enabled && runtime.ResolvedRuntime == service.PluginRuntimeGRPC && runtime.ConnectionState != "connected" {
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "grpc_not_connected",
			Severity: "error",
			Summary:  "Plugin is enabled but gRPC connection is not established",
			Detail:   firstNonEmpty(plugin.LastError, runtime.LastError, plugin.Address),
			Hint:     "Check the plugin address, network reachability, and plugin server health.",
		})
	}
	if !plugin.Enabled {
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "plugin_disabled",
			Severity: "warn",
			Summary:  "Plugin is currently disabled",
			Hint:     "Start or resume the plugin before expecting hooks or frontend pages to appear.",
		})
	}
	if strings.TrimSpace(plugin.LastError) != "" {
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "last_error_present",
			Severity: "warn",
			Summary:  "Plugin has a recorded runtime error",
			Detail:   strings.TrimSpace(plugin.LastError),
			Hint:     "Review plugin logs or execution history to confirm whether the error is still current.",
		})
	}
	if runtime.CooldownActive {
		detail := strings.TrimSpace(runtime.CooldownReason)
		if runtime.CooldownUntil != nil && !runtime.CooldownUntil.IsZero() {
			detail = firstNonEmpty(detail, "plugin cooldown is active")
			detail = fmt.Sprintf("%s (until %s)", detail, runtime.CooldownUntil.UTC().Format(time.RFC3339))
		}
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "plugin_cooldown_active",
			Severity: "warn",
			Summary:  "Plugin execution is temporarily cooled down after recent failures",
			Detail:   detail,
			Hint:     "Wait for the cooldown window to expire or run a health test/reload after fixing the plugin.",
		})
	}
	if runtime.BreakerState == "half_open" && runtime.ProbeInFlight {
		detail := strings.TrimSpace(runtime.CooldownReason)
		if runtime.ProbeStartedAt != nil && !runtime.ProbeStartedAt.IsZero() {
			detail = firstNonEmpty(detail, "plugin recovery probe is in progress")
			detail = fmt.Sprintf("%s (started at %s)", detail, runtime.ProbeStartedAt.UTC().Format(time.RFC3339))
		}
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "plugin_half_open_probe_in_flight",
			Severity: "warn",
			Summary:  "Plugin circuit breaker is half-open and running a single recovery probe",
			Detail:   detail,
			Hint:     "Wait for the current probe to finish before retrying hook or manual execution.",
		})
	}
	if !compatibility.Compatible {
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "plugin_protocol_incompatible",
			Severity: "error",
			Summary:  "Plugin manifest compatibility check failed",
			Detail:   strings.TrimSpace(compatibility.Reason),
			Hint:     "Update manifest_version, protocol_version, and host protocol range fields to match the current host.",
		})
	} else if compatibility.LegacyDefaultsApplied {
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "plugin_protocol_implicit",
			Severity: "warn",
			Summary:  "Plugin manifest does not declare explicit compatibility metadata",
			Detail:   strings.TrimSpace(compatibility.Reason),
			Hint:     "Add manifest_version, protocol_version, and optional host protocol range fields to make compatibility explicit.",
		})
	}
	if registration.State == "error" {
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "registration_failed",
			Severity: "warn",
			Summary:  "Last plugin registration attempt failed",
			Detail:   strings.TrimSpace(registration.Detail),
			Hint:     "Check plugin runtime settings, address or package path, and retry start/reload.",
		})
	}
	if plugin.Enabled && registration.State == "never_attempted" {
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "registration_not_attempted",
			Severity: "warn",
			Summary:  "No registration attempt was recorded in the current process",
			Hint:     "Try reloading or starting the plugin to confirm the current runtime state.",
		})
	}
	if len(missingPermissions) > 0 {
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "missing_permission_grants",
			Severity: "warn",
			Summary:  "Some requested permissions are not granted",
			Detail:   strings.Join(missingPermissions, ", "),
			Hint:     "Grant the missing permissions if the plugin should use those capabilities.",
		})
	}
	if !policy.AllowHookExecute {
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "hook_execution_disabled",
			Severity: "error",
			Summary:  "Hook execution is disabled by effective capability policy",
			Hint:     "Grant hook.execute and ensure allow_hook_execute remains enabled.",
		})
	}
	if storageDiagnostics.LastObserved != nil &&
		storageDiagnostics.LastObserved.DeclaredAccessMode == "unknown" &&
		storageDiagnostics.LastObserved.ObservedAccessMode != "" &&
		storageDiagnostics.LastObserved.ObservedAccessMode != "none" {
		issues = append(issues, pluginDiagnosticIssue{
			Code:     "execute_action_storage_missing_profile",
			Severity: "warn",
			Summary:  "Recent js_worker action used Plugin.storage without an explicit execute_action_storage declaration",
			Detail: fmt.Sprintf(
				"action=%s observed=%s source=%s",
				firstNonEmpty(storageDiagnostics.LastObserved.Action, "-"),
				storageDiagnostics.LastObserved.ObservedAccessMode,
				firstNonEmpty(storageDiagnostics.LastObserved.Source, "unknown"),
			),
			Hint: "Declare capabilities.execute_action_storage for frequently used actions so host locking and diagnostics stay explicit.",
		})
	}
	for _, route := range routes {
		for _, scopeCheck := range route.ScopeChecks {
			if scopeCheck.FrontendVisible {
				continue
			}
			summary := fmt.Sprintf("Declared %s plugin page is unavailable for %s scope", route.Area, scopeCheck.Scope)
			hint := "Check frontend.bootstrap hook access, frontend allowed areas, minimum scope, and frontend extension permission."
			if scopeCheck.Diagnostic.Participates && !scopeCheck.Diagnostic.AllowFrontendExtensions {
				hint = "Grant frontend.extension and keep allow_frontend_extensions enabled so route/menu output is not dropped."
			}
			issues = append(issues, pluginDiagnosticIssue{
				Code:     "frontend_route_unavailable",
				Severity: "warn",
				Summary:  summary,
				Detail:   firstNonEmpty(scopeCheck.Reason, scopeCheck.ReasonCode, route.Path),
				Hint:     hint,
			})
		}
	}
	return dedupePluginDiagnosticIssues(issues)
}

func dedupePluginDiagnosticIssues(issues []pluginDiagnosticIssue) []pluginDiagnosticIssue {
	if len(issues) == 0 {
		return []pluginDiagnosticIssue{}
	}
	seen := map[string]struct{}{}
	out := make([]pluginDiagnosticIssue, 0, len(issues))
	for _, issue := range issues {
		key := strings.Join([]string{
			strings.TrimSpace(issue.Code),
			strings.TrimSpace(issue.Severity),
			strings.TrimSpace(issue.Summary),
			strings.TrimSpace(issue.Detail),
		}, "|")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, issue)
	}
	return out
}

func diffNormalizedStringLists(expected []string, actual []string) []string {
	actualSet := make(map[string]struct{}, len(actual))
	for _, item := range actual {
		key := strings.ToLower(strings.TrimSpace(item))
		if key == "" {
			continue
		}
		actualSet[key] = struct{}{}
	}

	missing := make([]string, 0)
	for _, item := range expected {
		key := strings.ToLower(strings.TrimSpace(item))
		if key == "" {
			continue
		}
		if _, exists := actualSet[key]; exists {
			continue
		}
		missing = append(missing, key)
	}
	sort.Strings(missing)
	return missing
}

func resolvePluginRuntimeCheckState(plugin models.Plugin, runtime service.PluginRuntimeInspection) string {
	switch {
	case runtime.ConnectionState == "unavailable":
		return "warn"
	case !runtime.Valid:
		return "error"
	case runtime.ResolvedRuntime == service.PluginRuntimeGRPC && plugin.Enabled && runtime.ConnectionState != "connected":
		return "error"
	case runtime.ConnectionState == "connected" || runtime.ConnectionState == "stateless":
		return "ok"
	default:
		return "warn"
	}
}

func resolvePluginRegistrationCheckState(registration service.PluginRegistrationInspection) string {
	switch strings.ToLower(strings.TrimSpace(registration.State)) {
	case "success":
		return "ok"
	case "error":
		return "error"
	case "unavailable":
		return "warn"
	default:
		return "warn"
	}
}

func resolvePluginCompatibilityCheckState(compatibility service.PluginProtocolCompatibilityInspection) string {
	switch {
	case !compatibility.Compatible:
		return "error"
	case compatibility.LegacyDefaultsApplied:
		return "warn"
	default:
		return "ok"
	}
}

func buildPluginCompatibilityCheckSummary(compatibility service.PluginProtocolCompatibilityInspection) string {
	switch {
	case !compatibility.Compatible:
		return "Plugin manifest compatibility check failed"
	case compatibility.LegacyDefaultsApplied:
		return "Plugin compatibility uses legacy/default host version assumptions"
	default:
		return "Plugin manifest compatibility is explicitly declared"
	}
}

func buildPluginRegistrationCheckSummary(registration service.PluginRegistrationInspection) string {
	switch strings.ToLower(strings.TrimSpace(registration.State)) {
	case "success":
		return fmt.Sprintf(
			"Last registration succeeded via %s",
			firstNonEmpty(registration.Trigger, "unknown"),
		)
	case "error":
		return fmt.Sprintf(
			"Last registration failed via %s",
			firstNonEmpty(registration.Trigger, "unknown"),
		)
	case "unavailable":
		return "Registration inspection is unavailable"
	default:
		return "No registration attempt recorded in the current process"
	}
}

func boolCheckState(value bool, falseState string) string {
	if value {
		return "ok"
	}
	return falseState
}

func boolCheckSummary(value bool, trueSummary string, falseSummary string) string {
	if value {
		return trueSummary
	}
	return falseSummary
}
