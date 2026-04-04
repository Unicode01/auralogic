package admin

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"auralogic/internal/middleware"
	"auralogic/internal/models"
	"auralogic/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type pluginWorkspaceStreamEvent struct {
	Type       string                               `json:"type"`
	Workspace  interface{}                          `json:"workspace,omitempty"`
	Entries    []service.PluginWorkspaceBufferEntry `json:"entries,omitempty"`
	Cleared    bool                                 `json:"cleared,omitempty"`
	LastSeq    int64                                `json:"last_seq,omitempty"`
	EntryCount int                                  `json:"entry_count,omitempty"`
	UpdatedAt  string                               `json:"updated_at,omitempty"`
}

type pluginWorkspaceCommandRequest struct {
	Command     string                `json:"command"`
	CommandLine string                `json:"command_line"`
	Argv        []string              `json:"argv"`
	Input       string                `json:"input"`
	InputLines  []string              `json:"input_lines"`
	Async       bool                  `json:"async"`
	Context     *executePluginContext `json:"context"`
}

type pluginWorkspaceInputRequest struct {
	TaskID string `json:"task_id"`
	Input  string `json:"input"`
}

type pluginWorkspaceTerminalRequest struct {
	Line    string                `json:"line"`
	Context *executePluginContext `json:"context"`
}

type pluginWorkspaceRuntimeRequest struct {
	TaskID  string                `json:"task_id"`
	Line    string                `json:"line"`
	Depth   int                   `json:"depth"`
	Silent  bool                  `json:"silent"`
	Context *executePluginContext `json:"context"`
}

type pluginWorkspaceSignalRequest struct {
	TaskID string `json:"task_id"`
	Signal string `json:"signal"`
}

func resolvePluginWorkspaceSignalTaskID(
	requestTaskID string,
	snapshot service.PluginWorkspaceSnapshot,
) string {
	resolvedTaskID := strings.TrimSpace(requestTaskID)
	if resolvedTaskID != "" {
		return resolvedTaskID
	}
	return strings.TrimSpace(snapshot.ActiveTaskID)
}

type pluginWorkspaceWebSocketClientFrame struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	TaskID    string `json:"task_id,omitempty"`
	Input     string `json:"input,omitempty"`
	Line      string `json:"line,omitempty"`
	Depth     int    `json:"depth,omitempty"`
	Signal    string `json:"signal,omitempty"`
}

type pluginWorkspaceWebSocketAck struct {
	RequestID    string      `json:"request_id,omitempty"`
	Action       string      `json:"action,omitempty"`
	Success      bool        `json:"success"`
	Error        string      `json:"error,omitempty"`
	RuntimeState interface{} `json:"runtime_state,omitempty"`
	Workspace    interface{} `json:"workspace,omitempty"`
}

type pluginWorkspaceWebSocketServerFrame struct {
	Type    string                       `json:"type"`
	Event   *pluginWorkspaceStreamEvent  `json:"event,omitempty"`
	Ack     *pluginWorkspaceWebSocketAck `json:"ack,omitempty"`
	Message string                       `json:"message,omitempty"`
}

const pluginWorkspaceWebSocketProtocol = "auralogic.workspace.v1"

var pluginWorkspaceWebSocketUpgrader = websocket.Upgrader{
	Subprotocols: []string{pluginWorkspaceWebSocketProtocol},
	CheckOrigin: func(r *http.Request) bool {
		if r == nil {
			return false
		}
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		parsedOrigin, err := url.Parse(origin)
		if err != nil {
			return false
		}
		return strings.EqualFold(strings.TrimSpace(parsedOrigin.Host), strings.TrimSpace(r.Host))
	},
}

func buildPluginWorkspaceInputLines(req pluginWorkspaceCommandRequest) []string {
	if len(req.InputLines) > 0 {
		return req.InputLines
	}
	trimmed := strings.TrimRight(req.Input, "\r\n")
	if strings.TrimSpace(trimmed) == "" {
		return nil
	}
	return []string{trimmed}
}

func (h *PluginHandler) buildPluginWorkspaceExecutionContext(
	c *gin.Context,
	execCtx *service.ExecutionContext,
) *service.ExecutionContext {
	execCtx = enrichPluginExecutionContextWithRequestMetadata(execCtx, c)
	if execCtx == nil {
		execCtx = &service.ExecutionContext{}
	}
	if execCtx.Metadata == nil {
		execCtx.Metadata = make(map[string]string)
	}
	applyPluginAccessScopeMetadata(execCtx.Metadata, h.resolvePluginAccessScope(c))
	execCtx = ensureOperatorUserID(c, execCtx)
	if execCtx.UserID == nil {
		execCtx.UserID = getOptionalUserID(c)
	}
	return execCtx
}

func applyPluginWorkspaceRuntimeTaskID(
	execCtx *service.ExecutionContext,
	taskID string,
) *service.ExecutionContext {
	normalizedTaskID := strings.TrimSpace(taskID)
	if normalizedTaskID == "" {
		return execCtx
	}
	if execCtx == nil {
		execCtx = &service.ExecutionContext{}
	}
	if execCtx.Metadata == nil {
		execCtx.Metadata = make(map[string]string, 1)
	}
	execCtx.Metadata[service.PluginExecutionMetadataID] = normalizedTaskID
	return execCtx
}

func sanitizePluginWorkspaceRuntimeStateMapForAdmin(value map[string]interface{}) map[string]interface{} {
	if value == nil {
		return nil
	}
	sanitized := make(map[string]interface{}, 14)
	for _, key := range []string{
		"available",
		"exists",
		"instance_id",
		"script_path",
		"loaded",
		"busy",
		"current_action",
		"last_action",
		"created_at",
		"last_used_at",
		"boot_count",
		"total_requests",
		"execute_count",
		"eval_count",
		"inspect_count",
		"last_error",
		"completion_paths",
	} {
		if item, exists := value[key]; exists {
			sanitized[key] = item
		}
	}
	return sanitized
}

func sanitizePluginWorkspaceRuntimeStateForAdmin(value interface{}) interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case service.PluginWorkspaceRuntimeState:
		sanitized := gin.H{
			"available":      typed.Available,
			"exists":         typed.Exists,
			"loaded":         typed.Loaded,
			"busy":           typed.Busy,
			"boot_count":     typed.BootCount,
			"total_requests": typed.TotalRequests,
			"execute_count":  typed.ExecuteCount,
			"eval_count":     typed.EvalCount,
			"inspect_count":  typed.InspectCount,
		}
		if instanceID := strings.TrimSpace(typed.InstanceID); instanceID != "" {
			sanitized["instance_id"] = instanceID
		}
		if scriptPath := strings.TrimSpace(typed.ScriptPath); scriptPath != "" {
			sanitized["script_path"] = scriptPath
		}
		if currentAction := strings.TrimSpace(typed.CurrentAction); currentAction != "" {
			sanitized["current_action"] = currentAction
		}
		if lastAction := strings.TrimSpace(typed.LastAction); lastAction != "" {
			sanitized["last_action"] = lastAction
		}
		if typed.CreatedAt != nil {
			sanitized["created_at"] = typed.CreatedAt
		}
		if typed.LastUsedAt != nil {
			sanitized["last_used_at"] = typed.LastUsedAt
		}
		if lastError := strings.TrimSpace(typed.LastError); lastError != "" {
			sanitized["last_error"] = lastError
		}
		if len(typed.CompletionPaths) > 0 {
			sanitized["completion_paths"] = append([]string(nil), typed.CompletionPaths...)
		}
		return sanitized
	case *service.PluginWorkspaceRuntimeState:
		if typed == nil {
			return nil
		}
		return sanitizePluginWorkspaceRuntimeStateForAdmin(*typed)
	case map[string]interface{}:
		return sanitizePluginWorkspaceRuntimeStateMapForAdmin(typed)
	case gin.H:
		return sanitizePluginWorkspaceRuntimeStateMapForAdmin(map[string]interface{}(typed))
	default:
		return value
	}
}

func extractPluginWorkspaceRuntimeStateForAdmin(data interface{}) interface{} {
	switch typed := data.(type) {
	case map[string]interface{}:
		return sanitizePluginWorkspaceRuntimeStateForAdmin(typed["runtime_state"])
	case gin.H:
		return sanitizePluginWorkspaceRuntimeStateForAdmin(typed["runtime_state"])
	default:
		return nil
	}
}

func sanitizePluginWorkspaceRuntimeResponseMapDataForAdmin(
	value map[string]interface{},
) map[string]interface{} {
	if value == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(value))
	for key, item := range value {
		if strings.EqualFold(strings.TrimSpace(key), "runtime_state") {
			continue
		}
		cloned[key] = item
	}
	return cloned
}

func sanitizePluginWorkspaceRuntimeResponseDataForAdmin(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return sanitizePluginWorkspaceRuntimeResponseMapDataForAdmin(typed)
	case gin.H:
		return sanitizePluginWorkspaceRuntimeResponseMapDataForAdmin(map[string]interface{}(typed))
	default:
		return value
	}
}

func sanitizePluginWorkspaceSnapshotForAdmin(
	snapshot service.PluginWorkspaceSnapshot,
) map[string]interface{} {
	sanitized := map[string]interface{}{
		"control_granted": snapshot.ControlGranted,
		"viewer_count":    snapshot.ViewerCount,
		"buffer_capacity": snapshot.BufferCapacity,
		"entry_count":     snapshot.EntryCount,
		"last_seq":        snapshot.LastSeq,
	}
	entries := snapshot.Entries
	if entries == nil {
		entries = make([]service.PluginWorkspaceBufferEntry, 0)
	}
	sanitized["entries"] = entries
	if snapshot.OwnerAdminID > 0 {
		sanitized["owner_admin_id"] = snapshot.OwnerAdminID
	}
	if status := strings.TrimSpace(snapshot.Status); status != "" {
		sanitized["status"] = status
	}
	if activeTaskID := strings.TrimSpace(snapshot.ActiveTaskID); activeTaskID != "" {
		sanitized["active_task_id"] = activeTaskID
	}
	if activeCommand := strings.TrimSpace(snapshot.ActiveCommand); activeCommand != "" {
		sanitized["active_command"] = activeCommand
	}
	if prompt := strings.TrimSpace(snapshot.Prompt); prompt != "" {
		sanitized["prompt"] = prompt
	}
	if completionReason := strings.TrimSpace(snapshot.CompletionReason); completionReason != "" {
		sanitized["completion_reason"] = completionReason
	}
	if lastError := strings.TrimSpace(snapshot.LastError); lastError != "" {
		sanitized["last_error"] = lastError
	}
	if snapshot.HasMore {
		sanitized["has_more"] = true
	}
	if len(snapshot.RecentControlEvents) > 0 {
		sanitized["recent_control_events"] = snapshot.RecentControlEvents
	}
	return sanitized
}

func (h *PluginHandler) resolveJSWorkerWorkspacePlugin(id uint) (*models.Plugin, error) {
	if h == nil || h.pluginManager == nil {
		return nil, errors.New("plugin manager is unavailable")
	}

	var plugin models.Plugin
	if err := h.db.First(&plugin, id).Error; err != nil {
		return nil, err
	}
	if !plugin.Enabled {
		return nil, fmt.Errorf("plugin %d is disabled", plugin.ID)
	}

	runtime, err := h.resolveRuntime(plugin.Runtime)
	if err != nil {
		return nil, err
	}
	if runtime != service.PluginRuntimeJSWorker {
		return nil, errors.New("Workspace is only available for js_worker plugins")
	}
	return &plugin, nil
}

func buildPluginWorkspaceStreamEvent(
	event service.PluginWorkspaceStreamEvent,
	adminID uint,
) pluginWorkspaceStreamEvent {
	out := pluginWorkspaceStreamEvent{
		Type:       strings.TrimSpace(event.Type),
		Entries:    event.Entries,
		Cleared:    event.Cleared,
		LastSeq:    event.LastSeq,
		EntryCount: event.EntryCount,
	}
	if event.Workspace != nil {
		snapshot := applyPluginWorkspaceSnapshotForAdmin(*event.Workspace, adminID)
		out.Workspace = sanitizePluginWorkspaceSnapshotForAdmin(snapshot)
	}
	if !event.UpdatedAt.IsZero() {
		out.UpdatedAt = event.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	return out
}

func applyPluginWorkspaceSnapshotForAdmin(
	snapshot service.PluginWorkspaceSnapshot,
	adminID uint,
) service.PluginWorkspaceSnapshot {
	snapshot.ControlGranted = adminID > 0 && (snapshot.OwnerAdminID == 0 || snapshot.OwnerAdminID == adminID)
	return snapshot
}

func buildPluginWorkspaceStreamEventForAdmin(
	event service.PluginWorkspaceStreamEvent,
	adminID uint,
) pluginWorkspaceStreamEvent {
	return buildPluginWorkspaceStreamEvent(event, adminID)
}

func ptrPluginWorkspaceStreamEvent(event pluginWorkspaceStreamEvent) *pluginWorkspaceStreamEvent {
	cloned := event
	return &cloned
}

func buildPluginWorkspaceAuditFields(
	snapshot service.PluginWorkspaceSnapshot,
	extra map[string]interface{},
) map[string]interface{} {
	fields := map[string]interface{}{
		"workspace_owner_admin_id": snapshot.OwnerAdminID,
		"workspace_viewer_count":   snapshot.ViewerCount,
		"workspace_active_task_id": strings.TrimSpace(snapshot.ActiveTaskID),
		"workspace_status":         strings.TrimSpace(snapshot.Status),
	}
	for key, value := range extra {
		if strings.TrimSpace(key) == "" {
			continue
		}
		fields[key] = value
	}
	return fields
}

func (h *PluginHandler) GetPluginWorkspace(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	snapshot := applyPluginWorkspaceSnapshotForAdmin(
		h.pluginManager.GetPluginWorkspaceSnapshot(plugin, parsePluginExecutionTaskLimit(c, 200)),
		adminID,
	)
	c.JSON(http.StatusOK, gin.H{
		"workspace": sanitizePluginWorkspaceSnapshotForAdmin(snapshot),
	})
}

func (h *PluginHandler) GetPluginWorkspaceRuntimeState(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	state, stateErr := h.pluginManager.GetPluginWorkspaceRuntimeState(plugin)
	if stateErr != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, stateErr)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"runtime_state": sanitizePluginWorkspaceRuntimeStateForAdmin(state),
	})
}

func (h *PluginHandler) ExecutePluginWorkspaceCommand(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	resourceID := id
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	var req pluginWorkspaceCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	req.Command = strings.TrimSpace(req.Command)
	req.CommandLine = strings.TrimSpace(req.CommandLine)
	execCtx := &service.ExecutionContext{
		RequestContext: c.Request.Context(),
	}
	if req.Context != nil {
		execCtx.UserID = req.Context.UserID
		execCtx.OrderID = req.Context.OrderID
		execCtx.SessionID = req.Context.SessionID
		execCtx.Metadata = sanitizeUserProvidedExecutionMetadata(req.Context.Metadata)
	}
	execCtx = h.buildPluginWorkspaceExecutionContext(c, execCtx)
	if req.CommandLine != "" {
		if execCtx.Metadata == nil {
			execCtx.Metadata = make(map[string]string, 1)
		}
		execCtx.Metadata["workspace_command_line"] = req.CommandLine
	}

	workspaceSnapshot := h.pluginManager.GetPluginWorkspaceSnapshot(plugin, 0)
	shellVariables := service.BuildPluginWorkspaceShellVariables(plugin, adminID, &workspaceSnapshot, execCtx)
	var shellProgram []service.PluginWorkspaceShellResolvedPipeline
	pipelineMode := false
	sequenceMode := false
	standaloneShellCommand := false
	if req.CommandLine != "" {
		var resolveErr error
		shellProgram, resolveErr = h.pluginManager.ResolvePluginWorkspaceShellProgramWithVariablesForPlugin(
			plugin,
			req.CommandLine,
			shellVariables,
		)
		if resolveErr != nil {
			h.respondPluginErrorErr(c, http.StatusBadRequest, resolveErr)
			return
		}
		if len(shellProgram) == 0 {
			h.respondPluginError(c, http.StatusBadRequest, "command line is empty")
			return
		}
		sequenceMode = len(shellProgram) > 1
		for _, statement := range shellProgram {
			if len(statement.Stages) > 1 {
				pipelineMode = true
				break
			}
		}
		standaloneShellCommand = len(shellProgram) == 1 && len(shellProgram[0].Stages) == 1
		lastStatement := shellProgram[len(shellProgram)-1]
		lastStage := lastStatement.Stages[len(lastStatement.Stages)-1]
		req.Command = strings.TrimSpace(lastStage.Command.Name)
		req.Argv = append([]string(nil), lastStage.Argv...)
	} else if req.Command == "" {
		h.respondPluginError(c, http.StatusBadRequest, "command or command_line is required")
		return
	}

	taskID := service.EnsurePluginExecutionMetadata(execCtx, false)
	h.applyPluginExecutionHeaders(c, taskID)

	commandSpec, commandErr := h.pluginManager.ResolvePluginWorkspaceCommandForPlugin(plugin, req.Command)
	if commandErr != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, commandErr)
		return
	}
	if (pipelineMode || sequenceMode) && req.Async {
		h.respondPluginError(
			c,
			http.StatusBadRequest,
			"workspace command lines with pipelines or chained sequences do not support async mode",
		)
		return
	}
	if pipelineMode || sequenceMode {
		for _, statement := range shellProgram {
			for _, stage := range statement.Stages {
				if stage.Command == nil || !stage.Command.Interactive {
					continue
				}
				if len(statement.Stages) > 1 {
					h.respondPluginError(
						c,
						http.StatusBadRequest,
						fmt.Sprintf("interactive workspace command %s cannot be used in a pipeline", stage.Command.Name),
					)
					return
				}
				if len(shellProgram) > 1 {
					h.respondPluginError(
						c,
						http.StatusBadRequest,
						fmt.Sprintf(
							"interactive workspace command %s cannot be used in a chained sequence",
							stage.Command.Name,
						),
					)
					return
				}
			}
		}
	}
	if (req.CommandLine == "" || standaloneShellCommand) && (req.Async || commandSpec.Interactive) {
		started, startErr := h.pluginManager.StartPluginWorkspaceCommand(
			id,
			adminID,
			req.Command,
			req.Argv,
			buildPluginWorkspaceInputLines(req),
			execCtx,
		)
		if startErr != nil {
			h.logPluginOperation(c, "plugin_workspace_command_failed", plugin, &resourceID, map[string]interface{}{
				"success":                false,
				"workspace_command":      req.Command,
				"workspace_command_line": req.CommandLine,
				"argv":                   req.Argv,
				"async":                  true,
				"error":                  strings.TrimSpace(startErr.Error()),
			})
			resp := buildPluginExecuteFailurePayload(
				http.StatusBadRequest,
				"plugin workspace command failed",
				strings.TrimSpace(startErr.Error()),
				nil,
				nil,
			)
			c.JSON(http.StatusOK, resp)
			return
		}
		actualTaskID := strings.TrimSpace(started.TaskID)
		if actualTaskID == "" {
			actualTaskID = strings.TrimSpace(taskID)
		}
		h.applyPluginExecutionHeaders(c, actualTaskID)
		workspaceSnapshot := applyPluginWorkspaceSnapshotForAdmin(started.Workspace, adminID)
		h.logPluginOperation(c, "plugin_workspace_command_start", plugin, &resourceID, buildPluginWorkspaceAuditFields(workspaceSnapshot, map[string]interface{}{
			"success":                true,
			"workspace_command":      req.Command,
			"workspace_command_line": req.CommandLine,
			"argv":                   req.Argv,
			"async":                  true,
			"task_id":                actualTaskID,
		}))
		c.JSON(http.StatusOK, gin.H{
			"success":   true,
			"async":     true,
			"task_id":   actualTaskID,
			"workspace": sanitizePluginWorkspaceSnapshotForAdmin(workspaceSnapshot),
		})
		return
	}

	var (
		result  *service.ExecutionResult
		execErr error
	)
	if req.CommandLine != "" {
		result, execErr = h.pluginManager.ExecutePluginWorkspaceShellCommand(
			id,
			adminID,
			req.CommandLine,
			buildPluginWorkspaceInputLines(req),
			execCtx,
		)
	} else {
		result, execErr = h.pluginManager.ExecutePluginWorkspaceCommand(
			id,
			adminID,
			req.Command,
			req.Argv,
			buildPluginWorkspaceInputLines(req),
			execCtx,
		)
	}
	if execErr != nil {
		failureMetadata := clonePluginExecutionMetadataWithStatus(execCtx.Metadata, resolvePluginExecutionFailureStatus(execErr))
		h.logPluginOperation(c, "plugin_workspace_command_failed", plugin, &resourceID, map[string]interface{}{
			"success":                false,
			"workspace_command":      req.Command,
			"workspace_command_line": req.CommandLine,
			"argv":                   req.Argv,
			"error":                  strings.TrimSpace(execErr.Error()),
		})
		resp := buildPluginExecuteFailurePayload(
			http.StatusBadRequest,
			"plugin workspace command failed",
			strings.TrimSpace(execErr.Error()),
			nil,
			failureMetadata,
		)
		c.JSON(http.StatusOK, resp)
		return
	}

	resultErr := strings.TrimSpace(result.Error)
	if !result.Success {
		h.logPluginOperation(c, "plugin_workspace_command_failed", plugin, &resourceID, map[string]interface{}{
			"success":                false,
			"workspace_command":      req.Command,
			"workspace_command_line": req.CommandLine,
			"argv":                   req.Argv,
			"result_error":           resultErr,
		})
	} else {
		snapshot, _ := h.pluginManager.NotePluginWorkspaceControlActivity(
			plugin,
			adminID,
			"command_executed",
			fmt.Sprintf("Workspace command %s executed by admin #%d.", req.Command, adminID),
		)
		h.logPluginOperation(c, "plugin_workspace_command_execute", plugin, &resourceID, map[string]interface{}{
			"success":                  true,
			"workspace_command":        req.Command,
			"workspace_command_line":   req.CommandLine,
			"argv":                     req.Argv,
			"task_id":                  strings.TrimSpace(result.TaskID),
			"workspace_owner_admin_id": snapshot.OwnerAdminID,
			"workspace_viewer_count":   snapshot.ViewerCount,
		})
	}
	resp := gin.H{
		"success": result.Success,
		"data":    result.Data,
	}
	if taskID := strings.TrimSpace(result.TaskID); taskID != "" {
		resp["task_id"] = taskID
	}
	if !result.Success {
		resp = mergePluginFailurePayload(resp, buildPluginExecuteFailurePayload(
			http.StatusBadRequest,
			"plugin workspace command failed",
			resultErr,
			result.Data,
			result.Metadata,
		))
		resp["success"] = false
	}
	c.JSON(http.StatusOK, resp)
}

func (h *PluginHandler) EnterPluginWorkspaceTerminalLine(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	resourceID := id
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	var req pluginWorkspaceTerminalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
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
	execCtx = h.buildPluginWorkspaceExecutionContext(c, execCtx)

	result, terminalErr := h.pluginManager.EnterPluginWorkspaceTerminalLine(id, adminID, req.Line, execCtx)
	if terminalErr != nil {
		h.logPluginOperation(c, "plugin_workspace_terminal_failed", plugin, &resourceID, map[string]interface{}{
			"success": false,
			"line":    strings.TrimRight(req.Line, "\r\n"),
			"error":   strings.TrimSpace(terminalErr.Error()),
		})
		h.respondPluginErrorErr(c, http.StatusBadRequest, terminalErr)
		return
	}

	workspaceSnapshot := applyPluginWorkspaceSnapshotForAdmin(result.Workspace, adminID)
	response := gin.H{
		"success":   result.Success,
		"workspace": sanitizePluginWorkspaceSnapshotForAdmin(workspaceSnapshot),
	}
	if strings.TrimSpace(result.TaskID) != "" {
		h.applyPluginExecutionHeaders(c, strings.TrimSpace(result.TaskID))
	}
	if !result.Success {
		h.logPluginOperation(c, "plugin_workspace_terminal_failed", plugin, &resourceID, buildPluginWorkspaceAuditFields(workspaceSnapshot, map[string]interface{}{
			"success": false,
			"mode":    strings.TrimSpace(result.Mode),
			"task_id": strings.TrimSpace(result.TaskID),
			"line":    strings.TrimRight(req.Line, "\r\n"),
			"error":   strings.TrimSpace(result.Error),
		}))
		response = mergePluginFailurePayload(response, buildPluginExecuteFailurePayload(
			http.StatusBadRequest,
			"plugin workspace terminal line failed",
			strings.TrimSpace(result.Error),
			result.Data,
			result.Metadata,
		))
		response["success"] = false
		c.JSON(http.StatusOK, response)
		return
	}

	h.logPluginOperation(c, "plugin_workspace_terminal", plugin, &resourceID, buildPluginWorkspaceAuditFields(workspaceSnapshot, map[string]interface{}{
		"success":     true,
		"mode":        strings.TrimSpace(result.Mode),
		"queued":      result.Queued,
		"task_id":     strings.TrimSpace(result.TaskID),
		"interactive": result.Interactive,
		"line":        strings.TrimRight(req.Line, "\r\n"),
	}))
	c.JSON(http.StatusOK, response)
}

func (h *PluginHandler) EvaluatePluginWorkspaceRuntime(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	resourceID := id
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	var req pluginWorkspaceRuntimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
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
	execCtx = h.buildPluginWorkspaceExecutionContext(c, execCtx)
	execCtx = applyPluginWorkspaceRuntimeTaskID(execCtx, req.TaskID)

	result, runtimeErr := h.pluginManager.EvaluatePluginWorkspaceRuntime(id, adminID, req.Line, execCtx, req.Silent)
	if runtimeErr != nil {
		h.logPluginOperation(c, "plugin_workspace_runtime_eval_failed", plugin, &resourceID, map[string]interface{}{
			"success": false,
			"line":    strings.TrimRight(req.Line, "\r\n"),
			"silent":  req.Silent,
			"error":   strings.TrimSpace(runtimeErr.Error()),
		})
		h.respondPluginErrorErr(c, http.StatusBadRequest, runtimeErr)
		return
	}

	workspaceSnapshot := applyPluginWorkspaceSnapshotForAdmin(result.Workspace, adminID)
	runtimeState := extractPluginWorkspaceRuntimeStateForAdmin(result.Data)
	response := gin.H{
		"success":   result.Success,
		"workspace": sanitizePluginWorkspaceSnapshotForAdmin(workspaceSnapshot),
	}
	if runtimeState != nil {
		response["runtime_state"] = runtimeState
	}
	if strings.TrimSpace(result.TaskID) != "" {
		h.applyPluginExecutionHeaders(c, strings.TrimSpace(result.TaskID))
	}
	if !result.Success {
		h.logPluginOperation(c, "plugin_workspace_runtime_eval_failed", plugin, &resourceID, buildPluginWorkspaceAuditFields(workspaceSnapshot, map[string]interface{}{
			"success": false,
			"mode":    strings.TrimSpace(result.Mode),
			"task_id": strings.TrimSpace(result.TaskID),
			"line":    strings.TrimRight(req.Line, "\r\n"),
			"silent":  req.Silent,
			"error":   strings.TrimSpace(result.Error),
		}))
		response = mergePluginFailurePayload(response, buildPluginExecuteFailurePayload(
			http.StatusBadRequest,
			"plugin workspace runtime eval failed",
			strings.TrimSpace(result.Error),
			nil,
			result.Metadata,
		))
		response["success"] = false
		c.JSON(http.StatusOK, response)
		return
	}

	h.logPluginOperation(c, "plugin_workspace_runtime_eval", plugin, &resourceID, buildPluginWorkspaceAuditFields(workspaceSnapshot, map[string]interface{}{
		"success": true,
		"mode":    strings.TrimSpace(result.Mode),
		"task_id": strings.TrimSpace(result.TaskID),
		"line":    strings.TrimRight(req.Line, "\r\n"),
		"silent":  req.Silent,
	}))
	c.JSON(http.StatusOK, response)
}

func (h *PluginHandler) InspectPluginWorkspaceRuntime(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	resourceID := id
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	var req pluginWorkspaceRuntimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
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
	execCtx = h.buildPluginWorkspaceExecutionContext(c, execCtx)
	execCtx = applyPluginWorkspaceRuntimeTaskID(execCtx, req.TaskID)

	result, runtimeErr := h.pluginManager.InspectPluginWorkspaceRuntime(id, adminID, req.Line, req.Depth, execCtx, req.Silent)
	if runtimeErr != nil {
		h.logPluginOperation(c, "plugin_workspace_runtime_inspect_failed", plugin, &resourceID, map[string]interface{}{
			"success": false,
			"line":    strings.TrimRight(req.Line, "\r\n"),
			"depth":   req.Depth,
			"silent":  req.Silent,
			"error":   strings.TrimSpace(runtimeErr.Error()),
		})
		h.respondPluginErrorErr(c, http.StatusBadRequest, runtimeErr)
		return
	}

	workspaceSnapshot := applyPluginWorkspaceSnapshotForAdmin(result.Workspace, adminID)
	runtimeState := extractPluginWorkspaceRuntimeStateForAdmin(result.Data)
	response := gin.H{
		"success":   result.Success,
		"workspace": sanitizePluginWorkspaceSnapshotForAdmin(workspaceSnapshot),
	}
	if runtimeState != nil {
		response["runtime_state"] = runtimeState
	}
	if strings.TrimSpace(result.TaskID) != "" {
		h.applyPluginExecutionHeaders(c, strings.TrimSpace(result.TaskID))
	}
	if !result.Success {
		h.logPluginOperation(c, "plugin_workspace_runtime_inspect_failed", plugin, &resourceID, buildPluginWorkspaceAuditFields(workspaceSnapshot, map[string]interface{}{
			"success": false,
			"mode":    strings.TrimSpace(result.Mode),
			"task_id": strings.TrimSpace(result.TaskID),
			"line":    strings.TrimRight(req.Line, "\r\n"),
			"depth":   req.Depth,
			"silent":  req.Silent,
			"error":   strings.TrimSpace(result.Error),
		}))
		response = mergePluginFailurePayload(response, buildPluginExecuteFailurePayload(
			http.StatusBadRequest,
			"plugin workspace runtime inspect failed",
			strings.TrimSpace(result.Error),
			nil,
			result.Metadata,
		))
		response["success"] = false
		c.JSON(http.StatusOK, response)
		return
	}

	h.logPluginOperation(c, "plugin_workspace_runtime_inspect", plugin, &resourceID, buildPluginWorkspaceAuditFields(workspaceSnapshot, map[string]interface{}{
		"success": true,
		"mode":    strings.TrimSpace(result.Mode),
		"task_id": strings.TrimSpace(result.TaskID),
		"line":    strings.TrimRight(req.Line, "\r\n"),
		"depth":   req.Depth,
		"silent":  req.Silent,
	}))
	c.JSON(http.StatusOK, response)
}

func (h *PluginHandler) StreamPluginWorkspace(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	snapshot, stream, cancel, subscribeErr := h.pluginManager.SubscribePluginWorkspace(
		plugin,
		adminID,
		parsePluginExecutionTaskLimit(c, 200),
	)
	if subscribeErr != nil {
		h.respondPluginErrorErr(c, http.StatusInternalServerError, subscribeErr)
		return
	}
	defer cancel()
	h.logPluginOperation(c, "plugin_workspace_attach", plugin, &id, buildPluginWorkspaceAuditFields(applyPluginWorkspaceSnapshotForAdmin(snapshot, adminID), map[string]interface{}{
		"success":           true,
		"workspace_channel": "ndjson",
	}))
	defer func() {
		current := applyPluginWorkspaceSnapshotForAdmin(h.pluginManager.GetPluginWorkspaceSnapshot(plugin, 64), adminID)
		h.logPluginOperation(c, "plugin_workspace_detach", plugin, &id, buildPluginWorkspaceAuditFields(current, map[string]interface{}{
			"success":           true,
			"workspace_channel": "ndjson",
		}))
	}()

	if err := h.startPluginNDJSONStream(c, ""); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	if err := h.writePluginStreamEvent(c, pluginExecuteStreamEvent{
		Type: "workspace_snapshot",
		Data: map[string]interface{}{
			"event": buildPluginWorkspaceStreamEvent(service.PluginWorkspaceStreamEvent{
				Type:       "snapshot",
				Workspace:  &snapshot,
				LastSeq:    snapshot.LastSeq,
				EntryCount: snapshot.EntryCount,
				UpdatedAt:  snapshot.UpdatedAt,
			}, adminID),
		},
		Success: true,
		IsFinal: false,
	}); err != nil {
		return
	}

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-heartbeat.C:
			h.pluginManager.TickPluginWorkspace(plugin)
			if err := h.writePluginStreamEvent(c, pluginExecuteStreamEvent{
				Type:    "workspace_keepalive",
				Data:    map[string]interface{}{"event": pluginWorkspaceStreamEvent{Type: "keepalive"}},
				Success: true,
				IsFinal: false,
			}); err != nil {
				return
			}
		case event, ok := <-stream:
			if !ok {
				return
			}
			if err := h.writePluginStreamEvent(c, pluginExecuteStreamEvent{
				Type:    "workspace_delta",
				Data:    map[string]interface{}{"event": buildPluginWorkspaceStreamEventForAdmin(event, adminID)},
				Success: true,
				IsFinal: false,
			}); err != nil {
				return
			}
		}
	}
}

func (h *PluginHandler) WebSocketPluginWorkspace(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	conn, err := pluginWorkspaceWebSocketUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	logCtx := c.Copy()

	snapshot, stream, cancel, subscribeErr := h.pluginManager.SubscribePluginWorkspace(
		plugin,
		adminID,
		parsePluginExecutionTaskLimit(c, 200),
	)
	if subscribeErr != nil {
		_ = conn.WriteJSON(pluginWorkspaceWebSocketServerFrame{
			Type:    "workspace_error",
			Message: strings.TrimSpace(subscribeErr.Error()),
		})
		return
	}
	defer cancel()
	h.logPluginOperation(logCtx, "plugin_workspace_attach", plugin, &id, buildPluginWorkspaceAuditFields(applyPluginWorkspaceSnapshotForAdmin(snapshot, adminID), map[string]interface{}{
		"success":           true,
		"workspace_channel": "websocket",
	}))
	defer func() {
		current := applyPluginWorkspaceSnapshotForAdmin(h.pluginManager.GetPluginWorkspaceSnapshot(plugin, 64), adminID)
		h.logPluginOperation(logCtx, "plugin_workspace_detach", plugin, &id, buildPluginWorkspaceAuditFields(current, map[string]interface{}{
			"success":           true,
			"workspace_channel": "websocket",
		}))
	}()

	conn.SetReadLimit(1 << 20)
	_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))
		return nil
	})

	var writeMu sync.Mutex
	writeFrame := func(frame pluginWorkspaceWebSocketServerFrame) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(15 * time.Second))
		return conn.WriteJSON(frame)
	}
	writePing := func() error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(15*time.Second))
	}

	if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
		Type: "workspace_snapshot",
		Event: ptrPluginWorkspaceStreamEvent(buildPluginWorkspaceStreamEvent(service.PluginWorkspaceStreamEvent{
			Type:       "snapshot",
			Workspace:  &snapshot,
			LastSeq:    snapshot.LastSeq,
			EntryCount: snapshot.EntryCount,
			UpdatedAt:  snapshot.UpdatedAt,
		}, adminID)),
	}); err != nil {
		return
	}

	workspaceExecDefaults := h.buildPluginWorkspaceExecutionContext(c, &service.ExecutionContext{
		RequestContext: c.Request.Context(),
	})
	cloneWorkspaceExecCtx := func(taskID string) *service.ExecutionContext {
		execCtx := mergePluginExecutionContextDefaults(nil, workspaceExecDefaults)
		return applyPluginWorkspaceRuntimeTaskID(execCtx, taskID)
	}

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			var frame pluginWorkspaceWebSocketClientFrame
			if err := conn.ReadJSON(&frame); err != nil {
				return
			}
			switch strings.ToLower(strings.TrimSpace(frame.Type)) {
			case "":
				continue
			case "terminal_line":
				ack := &pluginWorkspaceWebSocketAck{
					RequestID: strings.TrimSpace(frame.RequestID),
					Action:    "terminal_line",
				}
				result, terminalErr := h.pluginManager.EnterPluginWorkspaceTerminalLine(id, adminID, frame.Line, cloneWorkspaceExecCtx(""))
				if terminalErr != nil {
					ack.Success = false
					ack.Error = strings.TrimSpace(terminalErr.Error())
					h.logPluginOperation(logCtx, "plugin_workspace_terminal_failed", plugin, &id, map[string]interface{}{
						"success":    false,
						"request_id": strings.TrimSpace(frame.RequestID),
						"line":       strings.TrimRight(frame.Line, "\r\n"),
						"error":      ack.Error,
					})
					if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
						Type: "workspace_request_ack",
						Ack:  ack,
					}); err != nil {
						return
					}
					continue
				}
				ack.Success = result.Success
				ack.Error = strings.TrimSpace(result.Error)
				ack.Workspace = sanitizePluginWorkspaceSnapshotForAdmin(
					applyPluginWorkspaceSnapshotForAdmin(result.Workspace, adminID),
				)
				if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
					Type: "workspace_request_ack",
					Ack:  ack,
				}); err != nil {
					return
				}
				if !ack.Success {
					h.logPluginOperation(logCtx, "plugin_workspace_terminal_failed", plugin, &id, map[string]interface{}{
						"success":    false,
						"request_id": strings.TrimSpace(frame.RequestID),
						"mode":       strings.TrimSpace(result.Mode),
						"task_id":    strings.TrimSpace(result.TaskID),
						"line":       strings.TrimRight(frame.Line, "\r\n"),
						"error":      ack.Error,
					})
					continue
				}
				h.logPluginOperation(logCtx, "plugin_workspace_terminal", plugin, &id, map[string]interface{}{
					"success":     true,
					"request_id":  strings.TrimSpace(frame.RequestID),
					"mode":        strings.TrimSpace(result.Mode),
					"queued":      result.Queued,
					"task_id":     strings.TrimSpace(result.TaskID),
					"interactive": result.Interactive,
					"line":        strings.TrimRight(frame.Line, "\r\n"),
				})
			case "runtime_eval":
				ack := &pluginWorkspaceWebSocketAck{
					RequestID: strings.TrimSpace(frame.RequestID),
					Action:    "runtime_eval",
				}
				result, runtimeErr := h.pluginManager.EvaluatePluginWorkspaceRuntime(id, adminID, frame.Line, cloneWorkspaceExecCtx(frame.TaskID), false)
				if runtimeErr != nil {
					ack.Success = false
					ack.Error = strings.TrimSpace(runtimeErr.Error())
					h.logPluginOperation(logCtx, "plugin_workspace_runtime_eval_failed", plugin, &id, map[string]interface{}{
						"success":    false,
						"request_id": strings.TrimSpace(frame.RequestID),
						"line":       strings.TrimRight(frame.Line, "\r\n"),
						"error":      ack.Error,
					})
					if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
						Type: "workspace_request_ack",
						Ack:  ack,
					}); err != nil {
						return
					}
					continue
				}
				ack.Success = result.Success
				ack.Error = strings.TrimSpace(result.Error)
				ack.RuntimeState = extractPluginWorkspaceRuntimeStateForAdmin(result.Data)
				ack.Workspace = sanitizePluginWorkspaceSnapshotForAdmin(
					applyPluginWorkspaceSnapshotForAdmin(result.Workspace, adminID),
				)
				if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
					Type: "workspace_request_ack",
					Ack:  ack,
				}); err != nil {
					return
				}
				if !ack.Success {
					h.logPluginOperation(logCtx, "plugin_workspace_runtime_eval_failed", plugin, &id, map[string]interface{}{
						"success":    false,
						"request_id": strings.TrimSpace(frame.RequestID),
						"mode":       strings.TrimSpace(result.Mode),
						"task_id":    strings.TrimSpace(result.TaskID),
						"line":       strings.TrimRight(frame.Line, "\r\n"),
						"error":      ack.Error,
					})
					continue
				}
				h.logPluginOperation(logCtx, "plugin_workspace_runtime_eval", plugin, &id, map[string]interface{}{
					"success":    true,
					"request_id": strings.TrimSpace(frame.RequestID),
					"mode":       strings.TrimSpace(result.Mode),
					"task_id":    strings.TrimSpace(result.TaskID),
					"line":       strings.TrimRight(frame.Line, "\r\n"),
				})
			case "runtime_inspect":
				ack := &pluginWorkspaceWebSocketAck{
					RequestID: strings.TrimSpace(frame.RequestID),
					Action:    "runtime_inspect",
				}
				result, runtimeErr := h.pluginManager.InspectPluginWorkspaceRuntime(id, adminID, frame.Line, frame.Depth, cloneWorkspaceExecCtx(frame.TaskID), false)
				if runtimeErr != nil {
					ack.Success = false
					ack.Error = strings.TrimSpace(runtimeErr.Error())
					h.logPluginOperation(logCtx, "plugin_workspace_runtime_inspect_failed", plugin, &id, map[string]interface{}{
						"success":    false,
						"request_id": strings.TrimSpace(frame.RequestID),
						"line":       strings.TrimRight(frame.Line, "\r\n"),
						"depth":      frame.Depth,
						"error":      ack.Error,
					})
					if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
						Type: "workspace_request_ack",
						Ack:  ack,
					}); err != nil {
						return
					}
					continue
				}
				ack.Success = result.Success
				ack.Error = strings.TrimSpace(result.Error)
				ack.RuntimeState = extractPluginWorkspaceRuntimeStateForAdmin(result.Data)
				ack.Workspace = sanitizePluginWorkspaceSnapshotForAdmin(
					applyPluginWorkspaceSnapshotForAdmin(result.Workspace, adminID),
				)
				if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
					Type: "workspace_request_ack",
					Ack:  ack,
				}); err != nil {
					return
				}
				if !ack.Success {
					h.logPluginOperation(logCtx, "plugin_workspace_runtime_inspect_failed", plugin, &id, map[string]interface{}{
						"success":    false,
						"request_id": strings.TrimSpace(frame.RequestID),
						"mode":       strings.TrimSpace(result.Mode),
						"task_id":    strings.TrimSpace(result.TaskID),
						"line":       strings.TrimRight(frame.Line, "\r\n"),
						"depth":      frame.Depth,
						"error":      ack.Error,
					})
					continue
				}
				h.logPluginOperation(logCtx, "plugin_workspace_runtime_inspect", plugin, &id, map[string]interface{}{
					"success":    true,
					"request_id": strings.TrimSpace(frame.RequestID),
					"mode":       strings.TrimSpace(result.Mode),
					"task_id":    strings.TrimSpace(result.TaskID),
					"line":       strings.TrimRight(frame.Line, "\r\n"),
					"depth":      frame.Depth,
				})
			case "input":
				if _, submitErr := h.pluginManager.SubmitPluginWorkspaceInput(id, adminID, strings.TrimSpace(frame.TaskID), frame.Input); submitErr != nil {
					h.logPluginOperation(logCtx, "plugin_workspace_input_failed", plugin, &id, map[string]interface{}{
						"success":     false,
						"task_id":     strings.TrimSpace(frame.TaskID),
						"input_bytes": len(frame.Input),
						"error":       strings.TrimSpace(submitErr.Error()),
					})
					if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
						Type:    "workspace_error",
						Message: strings.TrimSpace(submitErr.Error()),
					}); err != nil {
						return
					}
				} else {
					h.logPluginOperation(logCtx, "plugin_workspace_input", plugin, &id, map[string]interface{}{
						"success":     true,
						"task_id":     strings.TrimSpace(frame.TaskID),
						"input_bytes": len(frame.Input),
					})
				}
			case "signal":
				ack := &pluginWorkspaceWebSocketAck{
					RequestID: strings.TrimSpace(frame.RequestID),
					Action:    "signal",
				}
				normalizedSignal := strings.ToLower(strings.TrimSpace(frame.Signal))
				if normalizedSignal == "reset" || normalizedSignal == "reset_workspace" {
					if _, resetErr := h.pluginManager.ResetPluginWorkspace(plugin, adminID); resetErr != nil {
						ack.Success = false
						ack.Error = strings.TrimSpace(resetErr.Error())
						h.logPluginOperation(logCtx, "plugin_workspace_reset_failed", plugin, &id, map[string]interface{}{
							"success":    false,
							"request_id": strings.TrimSpace(frame.RequestID),
							"task_id":    strings.TrimSpace(frame.TaskID),
							"signal":     strings.TrimSpace(frame.Signal),
							"error":      ack.Error,
						})
						if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
							Type: "workspace_request_ack",
							Ack:  ack,
						}); err != nil {
							return
						}
					} else {
						ack.Success = true
						h.logPluginOperation(logCtx, "plugin_workspace_reset", plugin, &id, map[string]interface{}{
							"success":    true,
							"request_id": strings.TrimSpace(frame.RequestID),
							"task_id":    strings.TrimSpace(frame.TaskID),
							"signal":     strings.TrimSpace(frame.Signal),
						})
						if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
							Type: "workspace_request_ack",
							Ack:  ack,
						}); err != nil {
							return
						}
					}
					continue
				}
				snapshot, signalErr := h.pluginManager.SignalPluginWorkspace(id, adminID, strings.TrimSpace(frame.TaskID), frame.Signal)
				resolvedTaskID := resolvePluginWorkspaceSignalTaskID(frame.TaskID, snapshot)
				if signalErr != nil {
					ack.Success = false
					ack.Error = strings.TrimSpace(signalErr.Error())
					h.logPluginOperation(logCtx, "plugin_workspace_signal_failed", plugin, &id, map[string]interface{}{
						"success":    false,
						"request_id": strings.TrimSpace(frame.RequestID),
						"task_id":    resolvedTaskID,
						"signal":     strings.TrimSpace(frame.Signal),
						"error":      ack.Error,
					})
					if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
						Type: "workspace_request_ack",
						Ack:  ack,
					}); err != nil {
						return
					}
				} else {
					ack.Success = true
					h.logPluginOperation(logCtx, "plugin_workspace_signal", plugin, &id, map[string]interface{}{
						"success":    true,
						"request_id": strings.TrimSpace(frame.RequestID),
						"task_id":    resolvedTaskID,
						"signal":     strings.TrimSpace(frame.Signal),
					})
					if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
						Type: "workspace_request_ack",
						Ack:  ack,
					}); err != nil {
						return
					}
				}
			default:
				if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
					Type:    "workspace_error",
					Message: "unsupported workspace websocket frame",
				}); err != nil {
					return
				}
			}
		}
	}()

	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-readDone:
			return
		case <-heartbeat.C:
			h.pluginManager.TickPluginWorkspace(plugin)
			if err := writePing(); err != nil {
				return
			}
		case event, ok := <-stream:
			if !ok {
				return
			}
			if err := writeFrame(pluginWorkspaceWebSocketServerFrame{
				Type:  "workspace_delta",
				Event: ptrPluginWorkspaceStreamEvent(buildPluginWorkspaceStreamEventForAdmin(event, adminID)),
			}); err != nil {
				return
			}
		}
	}
}

func (h *PluginHandler) SubmitPluginWorkspaceInput(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	var req pluginWorkspaceInputRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	snapshot, submitErr := h.pluginManager.SubmitPluginWorkspaceInput(id, adminID, strings.TrimSpace(req.TaskID), req.Input)
	if submitErr != nil {
		h.logPluginOperation(c, "plugin_workspace_input_failed", plugin, &id, map[string]interface{}{
			"success":     false,
			"task_id":     strings.TrimSpace(req.TaskID),
			"input_bytes": len(req.Input),
			"error":       strings.TrimSpace(submitErr.Error()),
		})
		h.respondPluginErrorErr(c, http.StatusBadRequest, submitErr)
		return
	}
	snapshot = applyPluginWorkspaceSnapshotForAdmin(snapshot, adminID)
	h.logPluginOperation(c, "plugin_workspace_input", plugin, &id, buildPluginWorkspaceAuditFields(snapshot, map[string]interface{}{
		"success":     true,
		"task_id":     strings.TrimSpace(req.TaskID),
		"input_bytes": len(req.Input),
	}))
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"task_id":   strings.TrimSpace(req.TaskID),
		"workspace": sanitizePluginWorkspaceSnapshotForAdmin(snapshot),
	})
}

func (h *PluginHandler) SignalPluginWorkspace(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	var req pluginWorkspaceSignalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}
	req.Signal = strings.TrimSpace(req.Signal)
	if req.Signal == "" {
		req.Signal = "interrupt"
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	snapshot, signalErr := h.pluginManager.SignalPluginWorkspace(id, adminID, strings.TrimSpace(req.TaskID), req.Signal)
	resolvedTaskID := resolvePluginWorkspaceSignalTaskID(req.TaskID, snapshot)
	if signalErr != nil {
		h.logPluginOperation(c, "plugin_workspace_signal_failed", plugin, &id, map[string]interface{}{
			"success": false,
			"task_id": resolvedTaskID,
			"signal":  req.Signal,
			"error":   strings.TrimSpace(signalErr.Error()),
		})
		h.respondPluginErrorErr(c, http.StatusBadRequest, signalErr)
		return
	}
	snapshot = applyPluginWorkspaceSnapshotForAdmin(snapshot, adminID)
	h.logPluginOperation(c, "plugin_workspace_signal", plugin, &id, buildPluginWorkspaceAuditFields(snapshot, map[string]interface{}{
		"success": true,
		"task_id": resolvedTaskID,
		"signal":  req.Signal,
	}))
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"task_id":   resolvedTaskID,
		"signal":    req.Signal,
		"workspace": sanitizePluginWorkspaceSnapshotForAdmin(snapshot),
	})
}

func (h *PluginHandler) ClearPluginWorkspace(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	snapshot, clearErr := h.pluginManager.ClearPluginWorkspace(plugin, adminID)
	if clearErr != nil {
		h.logPluginOperation(c, "plugin_workspace_clear_failed", plugin, &id, map[string]interface{}{
			"success": false,
			"error":   strings.TrimSpace(clearErr.Error()),
		})
		h.respondPluginErrorErr(c, http.StatusBadRequest, clearErr)
		return
	}
	snapshot = applyPluginWorkspaceSnapshotForAdmin(snapshot, adminID)
	h.logPluginOperation(c, "plugin_workspace_clear", plugin, &id, buildPluginWorkspaceAuditFields(snapshot, map[string]interface{}{
		"success": true,
	}))
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"workspace": sanitizePluginWorkspaceSnapshotForAdmin(snapshot),
	})
}

func (h *PluginHandler) ResetPluginWorkspace(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	snapshot, resetErr := h.pluginManager.ResetPluginWorkspace(plugin, adminID)
	if resetErr != nil {
		h.logPluginOperation(c, "plugin_workspace_reset_failed", plugin, &id, map[string]interface{}{
			"success": false,
			"error":   strings.TrimSpace(resetErr.Error()),
		})
		h.respondPluginErrorErr(c, http.StatusBadRequest, resetErr)
		return
	}
	snapshot = applyPluginWorkspaceSnapshotForAdmin(snapshot, adminID)
	h.logPluginOperation(c, "plugin_workspace_reset", plugin, &id, buildPluginWorkspaceAuditFields(snapshot, map[string]interface{}{
		"success": true,
	}))
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"workspace": sanitizePluginWorkspaceSnapshotForAdmin(snapshot),
	})
}

func (h *PluginHandler) ResetPluginWorkspaceRuntime(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	snapshot, state, resetErr := h.pluginManager.ResetPluginWorkspaceRuntime(plugin, adminID)
	if resetErr != nil {
		h.logPluginOperation(c, "plugin_workspace_runtime_reset_failed", plugin, &id, map[string]interface{}{
			"success": false,
			"error":   strings.TrimSpace(resetErr.Error()),
		})
		h.respondPluginErrorErr(c, http.StatusBadRequest, resetErr)
		return
	}
	snapshot = applyPluginWorkspaceSnapshotForAdmin(snapshot, adminID)
	h.logPluginOperation(c, "plugin_workspace_runtime_reset", plugin, &id, buildPluginWorkspaceAuditFields(snapshot, map[string]interface{}{
		"success": true,
	}))
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"workspace":     sanitizePluginWorkspaceSnapshotForAdmin(snapshot),
		"runtime_state": sanitizePluginWorkspaceRuntimeStateForAdmin(state),
	})
}

func (h *PluginHandler) ClaimPluginWorkspaceControl(c *gin.Context) {
	id, ok := h.parsePluginID(c)
	if !ok {
		return
	}
	if h == nil || h.pluginManager == nil {
		h.respondPluginError(c, http.StatusServiceUnavailable, "Plugin manager is unavailable")
		return
	}
	adminID, adminIDOK := middleware.RequireUserID(c)
	if !adminIDOK {
		return
	}
	if c.IsAborted() {
		return
	}

	plugin, err := h.resolveJSWorkerWorkspacePlugin(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.respondPluginError(c, http.StatusNotFound, "Plugin not found")
			return
		}
		h.respondPluginErrorErr(c, http.StatusBadRequest, err)
		return
	}

	snapshot, previousOwner, claimed, claimErr := h.pluginManager.ClaimPluginWorkspaceControl(plugin, adminID)
	if claimErr != nil {
		h.logPluginOperation(c, "plugin_workspace_control_claim_failed", plugin, &id, map[string]interface{}{
			"success":                  false,
			"workspace_previous_owner": previousOwner,
			"error":                    strings.TrimSpace(claimErr.Error()),
		})
		h.respondPluginErrorErr(c, http.StatusBadRequest, claimErr)
		return
	}
	snapshot = applyPluginWorkspaceSnapshotForAdmin(snapshot, adminID)
	h.logPluginOperation(c, "plugin_workspace_control_claim", plugin, &id, buildPluginWorkspaceAuditFields(snapshot, map[string]interface{}{
		"success":                   true,
		"workspace_previous_owner":  previousOwner,
		"workspace_control_changed": claimed,
	}))
	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"claimed":        claimed,
		"previous_owner": previousOwner,
		"workspace":      sanitizePluginWorkspaceSnapshotForAdmin(snapshot),
	})
}
