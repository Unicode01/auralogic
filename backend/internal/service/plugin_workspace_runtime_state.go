package service

import (
	"fmt"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pluginipc"
)

type PluginWorkspaceRuntimeState struct {
	Available       bool       `json:"available"`
	Exists          bool       `json:"exists"`
	InstanceID      string     `json:"instance_id,omitempty"`
	PluginID        uint       `json:"plugin_id,omitempty"`
	Generation      uint       `json:"generation,omitempty"`
	ScriptPath      string     `json:"script_path,omitempty"`
	Loaded          bool       `json:"loaded"`
	Busy            bool       `json:"busy"`
	CurrentAction   string     `json:"current_action,omitempty"`
	LastAction      string     `json:"last_action,omitempty"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	LastUsedAt      *time.Time `json:"last_used_at,omitempty"`
	LastBootAt      *time.Time `json:"last_boot_at,omitempty"`
	BootCount       int64      `json:"boot_count"`
	TotalRequests   int64      `json:"total_requests"`
	ExecuteCount    int64      `json:"execute_count"`
	EvalCount       int64      `json:"eval_count"`
	InspectCount    int64      `json:"inspect_count"`
	LastError       string     `json:"last_error,omitempty"`
	RefCount        int        `json:"ref_count"`
	Disposed        bool       `json:"disposed"`
	CompletionPaths []string   `json:"completion_paths,omitempty"`
	CallablePaths   []string   `json:"callable_paths,omitempty"`
}

func parsePluginWorkspaceRuntimeState(raw map[string]interface{}) PluginWorkspaceRuntimeState {
	state := PluginWorkspaceRuntimeState{
		Available: parsePluginWorkspaceRuntimeBool(raw["available"]),
		Exists:    parsePluginWorkspaceRuntimeBool(raw["exists"]),
		Loaded:    parsePluginWorkspaceRuntimeBool(raw["loaded"]),
		Busy:      parsePluginWorkspaceRuntimeBool(raw["busy"]),
		Disposed:  parsePluginWorkspaceRuntimeBool(raw["disposed"]),
		RefCount:  parsePluginWorkspaceRuntimeInt(raw["ref_count"]),
	}
	if pluginID := parsePluginWorkspaceRuntimeUint(raw["plugin_id"]); pluginID > 0 {
		state.PluginID = pluginID
	}
	if generation := parsePluginWorkspaceRuntimeUint(raw["generation"]); generation > 0 {
		state.Generation = generation
	}
	state.InstanceID = strings.TrimSpace(parsePluginWorkspaceRuntimeString(raw["instance_id"]))
	state.ScriptPath = strings.TrimSpace(parsePluginWorkspaceRuntimeString(raw["script_path"]))
	state.CurrentAction = strings.TrimSpace(parsePluginWorkspaceRuntimeString(raw["current_action"]))
	state.LastAction = strings.TrimSpace(parsePluginWorkspaceRuntimeString(raw["last_action"]))
	state.LastError = strings.TrimSpace(parsePluginWorkspaceRuntimeString(raw["last_error"]))
	state.BootCount = parsePluginWorkspaceRuntimeInt64(raw["boot_count"])
	state.TotalRequests = parsePluginWorkspaceRuntimeInt64(raw["total_requests"])
	state.ExecuteCount = parsePluginWorkspaceRuntimeInt64(raw["execute_count"])
	state.EvalCount = parsePluginWorkspaceRuntimeInt64(raw["eval_count"])
	state.InspectCount = parsePluginWorkspaceRuntimeInt64(raw["inspect_count"])
	state.CreatedAt = parsePluginWorkspaceRuntimeTimePointer(raw["created_at"])
	state.LastUsedAt = parsePluginWorkspaceRuntimeTimePointer(raw["last_used_at"])
	state.LastBootAt = parsePluginWorkspaceRuntimeTimePointer(raw["last_boot_at"])
	state.CompletionPaths = parsePluginWorkspaceRuntimeStringList(raw["completion_paths"])
	state.CallablePaths = parsePluginWorkspaceRuntimeStringList(raw["callable_paths"])
	return state
}

func parsePluginWorkspaceRuntimeString(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

func parsePluginWorkspaceRuntimeBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "y", "on":
			return true
		}
	case float64:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	}
	return false
}

func parsePluginWorkspaceRuntimeInt(value interface{}) int {
	return int(parsePluginWorkspaceRuntimeInt64(value))
}

func parsePluginWorkspaceRuntimeInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case uint:
		return int64(typed)
	case uint8:
		return int64(typed)
	case uint16:
		return int64(typed)
	case uint32:
		return int64(typed)
	case uint64:
		return int64(typed)
	case float64:
		return int64(typed)
	case string:
		parsed := strings.TrimSpace(typed)
		if parsed == "" {
			return 0
		}
		if value, err := time.ParseDuration(parsed); err == nil {
			return int64(value)
		}
		var out int64
		_, _ = fmt.Sscan(parsed, &out)
		return out
	default:
		return 0
	}
}

func parsePluginWorkspaceRuntimeUint(value interface{}) uint {
	parsed := parsePluginWorkspaceRuntimeInt64(value)
	if parsed <= 0 {
		return 0
	}
	return uint(parsed)
}

func parsePluginWorkspaceRuntimeTimePointer(value interface{}) *time.Time {
	raw := strings.TrimSpace(parsePluginWorkspaceRuntimeString(value))
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, raw)
	}
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}

func parsePluginWorkspaceRuntimeStringList(value interface{}) []string {
	items, ok := value.([]interface{})
	if !ok {
		if typed, ok := value.([]string); ok {
			out := make([]string, 0, len(typed))
			for _, item := range typed {
				normalized := strings.TrimSpace(item)
				if normalized == "" {
					continue
				}
				out = append(out, normalized)
			}
			return out
		}
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		normalized := strings.TrimSpace(parsePluginWorkspaceRuntimeString(item))
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}

func (s *PluginManagerService) GetPluginWorkspaceRuntimeState(
	plugin *models.Plugin,
) (PluginWorkspaceRuntimeState, error) {
	if s == nil || plugin == nil {
		return PluginWorkspaceRuntimeState{}, fmt.Errorf("plugin runtime is unavailable")
	}
	runtime := strings.ToLower(strings.TrimSpace(plugin.Runtime))
	if runtime != PluginRuntimeJSWorker {
		return PluginWorkspaceRuntimeState{}, fmt.Errorf("workspace runtime is only available for js_worker plugins")
	}
	state := PluginWorkspaceRuntimeState{
		Available:  s.jsWorker != nil,
		PluginID:   plugin.ID,
		Generation: resolvePluginAppliedGeneration(plugin),
	}
	if s.jsWorker == nil {
		return state, nil
	}
	raw, err := s.jsWorker.GetRuntimeState(plugin)
	if err != nil {
		return state, err
	}
	parsed := parsePluginWorkspaceRuntimeState(raw)
	if parsed.PluginID == 0 {
		parsed.PluginID = plugin.ID
	}
	if parsed.Generation == 0 {
		parsed.Generation = resolvePluginAppliedGeneration(plugin)
	}
	return parsed, nil
}

func (s *PluginManagerService) ResetPluginWorkspaceRuntime(
	plugin *models.Plugin,
	adminID uint,
) (PluginWorkspaceSnapshot, PluginWorkspaceRuntimeState, error) {
	if s == nil || plugin == nil {
		return PluginWorkspaceSnapshot{}, PluginWorkspaceRuntimeState{}, fmt.Errorf("plugin runtime is unavailable")
	}
	runtime := strings.ToLower(strings.TrimSpace(plugin.Runtime))
	if runtime != PluginRuntimeJSWorker {
		return PluginWorkspaceSnapshot{}, PluginWorkspaceRuntimeState{}, fmt.Errorf("workspace runtime is only available for js_worker plugins")
	}
	if s.jsWorker == nil {
		return PluginWorkspaceSnapshot{}, PluginWorkspaceRuntimeState{}, fmt.Errorf("js worker supervisor is not initialized")
	}

	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, runtime)
	if err := buffer.ensureControl(adminID); err != nil {
		snapshot := buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return snapshot, PluginWorkspaceRuntimeState{}, err
	}
	if strings.TrimSpace(buffer.activeTaskID) != "" && (buffer.status == "running" || buffer.status == "waiting_input") {
		commandLabel := strings.TrimSpace(buffer.activeCommand)
		if commandLabel == "" {
			commandLabel = strings.TrimSpace(buffer.activeTaskID)
		}
		snapshot := buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return snapshot, PluginWorkspaceRuntimeState{}, fmt.Errorf("workspace command %s is still running", commandLabel)
	}
	buffer.touchOwnerActivity(adminID)
	s.workspaceMu.Unlock()

	if err := s.jsWorker.DisposePluginRuntime(plugin.ID, resolvePluginAppliedGeneration(plugin)); err != nil {
		snapshot := s.GetPluginWorkspaceSnapshot(plugin, defaultPluginWorkspaceSnapshotLimit)
		return snapshot, PluginWorkspaceRuntimeState{}, err
	}

	s.ApplyPluginWorkspaceDelta(plugin.ID, plugin.Name, runtime, map[string]string{
		"action": "workspace.runtime.reset",
	}, []pluginipc.WorkspaceBufferEntry{{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Channel:   "system",
		Level:     "info",
		Message:   fmt.Sprintf("JS runtime disposed by admin #%d. The next command will boot a fresh VM instance.", adminID),
		Source:    "host.workspace.runtime.reset",
	}}, false)

	_, _ = s.NotePluginWorkspaceControlActivity(
		plugin,
		adminID,
		"workspace_runtime_reset",
		fmt.Sprintf("Workspace JS runtime reset by admin #%d.", adminID),
	)

	snapshot := s.GetPluginWorkspaceSnapshot(plugin, defaultPluginWorkspaceSnapshotLimit)
	state, stateErr := s.GetPluginWorkspaceRuntimeState(plugin)
	if stateErr != nil {
		return snapshot, PluginWorkspaceRuntimeState{}, stateErr
	}
	return snapshot, state, nil
}
