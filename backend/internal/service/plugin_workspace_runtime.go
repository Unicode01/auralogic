package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pluginipc"
	"auralogic/internal/pluginobs"
)

const (
	pluginWorkspaceRuntimeEvalAction     = "workspace.runtime.eval"
	pluginWorkspaceRuntimeInspectAction  = "workspace.runtime.inspect"
	pluginWorkspaceRuntimePreviewJSONKey = "workspace_runtime_preview_json"
	defaultPluginWorkspaceRuntimeTimeout = 3 * time.Minute
)

var pluginWorkspaceRuntimeSideEffectExpressionPattern = regexp.MustCompile(
	`^(?:(?:globalThis\.)?console\.(?:log|info|warn|error|debug)|Plugin\.workspace\.(?:write|writeln|info|warn|error|clear))\s*\(`,
)

func (s *PluginManagerService) EvaluatePluginWorkspaceRuntime(
	pluginID uint,
	adminID uint,
	line string,
	execCtx *ExecutionContext,
	silent bool,
) (*PluginWorkspaceTerminalLineResult, error) {
	return s.executePluginWorkspaceRuntimeConsole(pluginID, adminID, line, 0, false, execCtx, silent)
}

func (s *PluginManagerService) InspectPluginWorkspaceRuntime(
	pluginID uint,
	adminID uint,
	expression string,
	depth int,
	execCtx *ExecutionContext,
	silent bool,
) (*PluginWorkspaceTerminalLineResult, error) {
	return s.executePluginWorkspaceRuntimeConsole(pluginID, adminID, expression, depth, true, execCtx, silent)
}

func (s *PluginManagerService) executePluginWorkspaceRuntimeConsole(
	pluginID uint,
	adminID uint,
	line string,
	depth int,
	inspect bool,
	execCtx *ExecutionContext,
	silent bool,
) (*PluginWorkspaceTerminalLineResult, error) {
	plugin, runtime, capabilityPolicy, err, _ := s.getPluginByIDWithCatalog(pluginID)
	if err != nil {
		return nil, err
	}
	if runtime != PluginRuntimeJSWorker {
		return nil, fmt.Errorf("workspace runtime console is only available for js_worker plugins")
	}

	normalizedLine := strings.TrimRight(line, "\r\n")
	if inspect && strings.TrimSpace(normalizedLine) == "" {
		normalizedLine = "globalThis"
	}

	var idleWorkspace PluginWorkspaceSnapshot
	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, runtime)
	if controlErr := buffer.ensureControl(adminID); controlErr != nil {
		idleWorkspace = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return nil, controlErr
	}
	session, _, resolveErr := s.resolvePluginWorkspaceActiveSessionLocked(plugin.ID)
	if resolveErr != nil {
		idleWorkspace = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return nil, resolveErr
	}
	if session != nil {
		commandLabel := strings.TrimSpace(session.commandName)
		if commandLabel == "" {
			commandLabel = strings.TrimSpace(session.taskID)
		}
		idleWorkspace = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return nil, fmt.Errorf("workspace command %s is still running", commandLabel)
	}
	idleWorkspace = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
	s.workspaceMu.Unlock()

	if !inspect && strings.TrimSpace(normalizedLine) == "" {
		return &PluginWorkspaceTerminalLineResult{
			Mode:      "noop",
			Success:   true,
			Workspace: idleWorkspace,
		}, nil
	}

	preparedExecCtx := clonePluginExecutionContext(execCtx)
	if preparedExecCtx == nil {
		preparedExecCtx = &ExecutionContext{}
	}
	if preparedExecCtx.Metadata == nil {
		preparedExecCtx.Metadata = make(map[string]string, 2)
	}
	preparedExecCtx.Metadata["workspace_terminal_line"] = normalizedLine
	preparedExecCtx.Metadata["workspace_console_mode"] = "js"
	if silent {
		preparedExecCtx.Metadata["workspace_console_silent"] = "true"
	}

	action := pluginWorkspaceRuntimeEvalAction
	mode := "runtime_eval"
	commandLabel := "eval"
	requestParams := map[string]string{
		"workspace_console_mode": "js",
		"statement_kind":         "eval",
		"statement_bytes":        strconv.Itoa(len(normalizedLine)),
	}
	if silent {
		requestParams["silent"] = "true"
	}
	if inspect {
		action = pluginWorkspaceRuntimeInspectAction
		mode = "runtime_inspect"
		commandLabel = "inspect"
		requestParams["statement_kind"] = "inspect"
		requestParams["inspect_depth"] = strconv.Itoa(normalizeWorkspaceRuntimeInspectDepth(depth))
	}

	preparedExecCtx, task := s.startPluginExecutionTask(plugin, runtime, action, requestParams, preparedExecCtx, false)
	taskID := strings.TrimSpace(preparedExecCtx.Metadata[PluginExecutionMetadataID])
	entryMetadata := mergePluginWorkspaceMetadata(preparedExecCtx.Metadata, map[string]string{
		"action":                 action,
		"task_id":                taskID,
		"workspace_console_mode": "js",
		"workspace_console_kind": commandLabel,
	})
	if !silent {
		s.appendPluginWorkspaceEchoEntry(
			plugin.ID,
			plugin.Name,
			runtime,
			"host.workspace.command",
			"command",
			"js> "+normalizedLine,
			entryMetadata,
		)
	}

	startTime := time.Now()
	runtimeTimeout := defaultPluginWorkspaceRuntimeTimeout
	var result *ExecutionResult
	if inspect {
		result, err = s.executeWithJSPluginRuntimeInspect(
			plugin,
			capabilityPolicy,
			normalizedLine,
			depth,
			preparedExecCtx,
			runtimeTimeout,
		)
	} else {
		result, err = s.executeWithJSPluginRuntimeEval(
			plugin,
			capabilityPolicy,
			normalizedLine,
			preparedExecCtx,
			runtimeTimeout,
		)
	}
	if task != nil {
		snapshot := s.completePluginExecutionTask(task, result, err)
		preparedExecCtx.Metadata = mergePluginExecutionTaskMetadata(preparedExecCtx.Metadata, snapshot)
		result = applyPluginExecutionTaskToResult(result, snapshot)
	}

	duration := int(time.Since(startTime).Milliseconds())
	if duration < 0 {
		duration = 0
	}
	success := err == nil && result != nil && result.Success
	timedOut := isPluginExecutionTimeoutError(err)
	pluginobs.RecordExecution(plugin.ID, plugin.Name, runtime, action, int64(duration), success, timedOut)
	s.emitPluginExecutionAuditEvent(plugin, runtime, action, requestParams, preparedExecCtx, result, err, duration)
	s.recordExecution(plugin.ID, action, requestParams, preparedExecCtx, result, err, duration)

	metadata := cloneStringMap(preparedExecCtx.Metadata)
	if result != nil && len(result.Metadata) > 0 {
		for key, value := range result.Metadata {
			metadata[key] = value
		}
	}
	if taskID == "" && result != nil {
		taskID = strings.TrimSpace(result.TaskID)
	}
	if taskID != "" {
		metadata["task_id"] = taskID
	}

	if err != nil || result == nil || !result.Success {
		errorText := ""
		switch {
		case err != nil:
			errorText = strings.TrimSpace(err.Error())
		case result != nil:
			errorText = strings.TrimSpace(result.Error)
		default:
			errorText = "runtime console execution failed"
		}
		if errorText == "" {
			errorText = "runtime console execution failed"
		}
		resultData := map[string]interface{}{}
		if result != nil && result.Data != nil {
			resultData = clonePayloadMap(result.Data)
		}
		workspaceSnapshot := s.GetPluginWorkspaceSnapshot(plugin, defaultPluginWorkspaceSnapshotLimit)
		if !silent {
			s.ApplyPluginWorkspaceDelta(plugin.ID, plugin.Name, runtime, metadata, []pluginipc.WorkspaceBufferEntry{{
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				Channel:   "stderr",
				Level:     "error",
				Message:   errorText,
				Source:    "host.workspace.runtime." + commandLabel,
			}}, false)
			workspaceSnapshot = s.GetPluginWorkspaceSnapshot(plugin, defaultPluginWorkspaceSnapshotLimit)
		}
		return &PluginWorkspaceTerminalLineResult{
			Mode:      mode,
			TaskID:    taskID,
			Success:   false,
			Error:     errorText,
			Data:      resultData,
			Metadata:  metadata,
			Workspace: workspaceSnapshot,
		}, nil
	}

	resultText := formatWorkspaceRuntimeConsoleResult(result.Data)
	if strings.TrimSpace(resultText) == "" {
		resultText = "undefined"
	}
	outputMetadata := cloneStringMap(metadata)
	if previewJSON := encodePluginWorkspaceRuntimePreviewJSON(result.Data); previewJSON != "" {
		if outputMetadata == nil {
			outputMetadata = map[string]string{}
		}
		outputMetadata[pluginWorkspaceRuntimePreviewJSONKey] = previewJSON
	}
	workspaceSnapshot := s.GetPluginWorkspaceSnapshot(plugin, defaultPluginWorkspaceSnapshotLimit)
	if !silent && !shouldSuppressPluginWorkspaceRuntimeResult(normalizedLine, inspect, result.Data) {
		s.ApplyPluginWorkspaceDelta(plugin.ID, plugin.Name, runtime, outputMetadata, []pluginipc.WorkspaceBufferEntry{{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Channel:   "stdout",
			Level:     "info",
			Message:   "< " + resultText,
			Source:    "host.workspace.runtime." + commandLabel,
		}}, false)
		workspaceSnapshot = s.GetPluginWorkspaceSnapshot(plugin, defaultPluginWorkspaceSnapshotLimit)
		if notedSnapshot, noteErr := s.NotePluginWorkspaceControlActivity(
			plugin,
			adminID,
			"command_executed",
			fmt.Sprintf("Workspace JS %s executed by admin #%d.", commandLabel, adminID),
		); noteErr == nil {
			workspaceSnapshot = notedSnapshot
		}
	}
	return &PluginWorkspaceTerminalLineResult{
		Mode:      mode,
		TaskID:    taskID,
		Success:   true,
		Data:      clonePayloadMap(result.Data),
		Metadata:  outputMetadata,
		Workspace: workspaceSnapshot,
	}, nil
}

func (s *PluginManagerService) executeWithJSPluginRuntimeEval(
	plugin *models.Plugin,
	capabilityPolicy pluginCapabilityPolicy,
	code string,
	execCtx *ExecutionContext,
	timeoutOverride time.Duration,
) (*ExecutionResult, error) {
	return s.executeWithJSPluginRuntimeConsole(plugin, capabilityPolicy, pluginWorkspaceRuntimeEvalAction, code, 0, execCtx, timeoutOverride)
}

func (s *PluginManagerService) executeWithJSPluginRuntimeInspect(
	plugin *models.Plugin,
	capabilityPolicy pluginCapabilityPolicy,
	expression string,
	depth int,
	execCtx *ExecutionContext,
	timeoutOverride time.Duration,
) (*ExecutionResult, error) {
	return s.executeWithJSPluginRuntimeConsole(plugin, capabilityPolicy, pluginWorkspaceRuntimeInspectAction, expression, depth, execCtx, timeoutOverride)
}

func (s *PluginManagerService) executeWithJSPluginRuntimeConsole(
	plugin *models.Plugin,
	capabilityPolicy pluginCapabilityPolicy,
	action string,
	line string,
	depth int,
	execCtx *ExecutionContext,
	timeoutOverride time.Duration,
) (*ExecutionResult, error) {
	if s.jsWorker == nil {
		return nil, fmt.Errorf("js worker supervisor is not initialized")
	}

	declaredStorageMode := capabilityPolicy.ResolveExecuteActionStorageMode(action)
	releaseStorageLock := acquirePluginStorageExecutionLock(plugin.ID, declaredStorageMode)
	defer releaseStorageLock()

	storageSnapshot, err := s.loadPluginStorageSnapshot(plugin.ID)
	if err != nil {
		return nil, fmt.Errorf("load plugin storage failed: %w", err)
	}
	secretSnapshot, err := s.loadPluginSecretSnapshot(plugin.ID)
	if err != nil {
		return nil, fmt.Errorf("load plugin secrets failed: %w", err)
	}

	var result *ExecutionResult
	switch action {
	case pluginWorkspaceRuntimeInspectAction:
		result, err = s.jsWorker.InspectRuntimeWithTimeoutAndStorage(
			plugin,
			line,
			normalizeWorkspaceRuntimeInspectDepth(depth),
			storageSnapshot,
			secretSnapshot,
			execCtx,
			timeoutOverride,
		)
	default:
		result, err = s.jsWorker.EvaluateRuntimeWithTimeoutAndStorage(
			plugin,
			line,
			storageSnapshot,
			secretSnapshot,
			execCtx,
			timeoutOverride,
		)
	}
	if err != nil {
		return nil, err
	}
	if result != nil {
		if validateErr := validatePluginStorageAccessMode(
			declaredStorageMode,
			resolvePluginStorageAccessModeFromMetadata(result.Metadata),
			result.StorageChanged,
		); validateErr != nil {
			return nil, validateErr
		}
	}
	if result != nil && result.StorageChanged {
		if persistErr := s.replacePluginStorageSnapshot(plugin.ID, result.Storage); persistErr != nil {
			return nil, fmt.Errorf("persist plugin storage failed: %w", persistErr)
		}
	}
	return result, nil
}

func normalizeWorkspaceRuntimeInspectDepth(depth int) int {
	if depth <= 0 {
		return 2
	}
	if depth > 4 {
		return 4
	}
	return depth
}

func formatWorkspaceRuntimeConsoleResult(data map[string]interface{}) string {
	if len(data) == 0 {
		return "undefined"
	}
	if summary, ok := data["summary"].(string); ok && strings.TrimSpace(summary) != "" {
		return strings.TrimSpace(summary)
	}
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Sprintf("%v", data)
	}
	return string(body)
}

func encodePluginWorkspaceRuntimePreviewJSON(data map[string]interface{}) string {
	sanitized := sanitizePluginWorkspaceRuntimePreviewPayload(data)
	if len(sanitized) == 0 {
		return ""
	}
	body, err := json.Marshal(sanitized)
	if err != nil {
		return ""
	}
	return string(body)
}

func sanitizePluginWorkspaceRuntimePreviewPayload(data map[string]interface{}) map[string]interface{} {
	if len(data) == 0 {
		return nil
	}
	cloned := clonePayloadMap(data)
	delete(cloned, "runtime_state")
	return cloned
}

func shouldSuppressPluginWorkspaceRuntimeResult(
	line string,
	inspect bool,
	data map[string]interface{},
) bool {
	if inspect {
		return false
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !pluginWorkspaceRuntimeSideEffectExpressionPattern.MatchString(trimmed) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(interfaceToString(data["type"])), "undefined")
}
