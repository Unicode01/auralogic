package jsworker

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"auralogic/internal/pluginipc"
	"github.com/dop251/goja"
)

const (
	defaultWorkerWorkspaceMaxEntries     = 256
	defaultWorkerWorkspaceForwardBatch   = 8
	workerWorkspaceCommandExecuteAction  = "workspace.command.execute"
	workerWorkspaceRuntimeEvalAction     = "workspace.runtime.eval"
	workerWorkspaceRuntimeInspectAction  = "workspace.runtime.inspect"
	workerWorkspaceExecutionIDMetadata   = "plugin_execution_id"
	workerWorkspaceTerminalLineMetadata  = "workspace_terminal_line"
	workerWorkspaceCommandNameParam      = "workspace_command_name"
	workerWorkspaceCommandEntryParam     = "workspace_command_entry"
	workerWorkspaceCommandIDParam        = "workspace_command_id"
	workerWorkspaceCommandRawParam       = "workspace_command_raw"
	workerWorkspaceCommandArgvJSONParam  = "workspace_command_argv_json"
	workerWorkspaceCommandInputJSONParam = "workspace_command_input_lines_json"
	workerWorkspaceChannelStdout         = "stdout"
	workerWorkspaceSourceWrite           = "plugin.workspace.write"
	workerWorkspaceSourceWriteln         = "plugin.workspace.writeln"
)

type pluginWorkspaceState struct {
	enabled      bool
	maxEntries   int
	history      []pluginipc.WorkspaceBufferEntry
	pending      []pluginipc.WorkspaceBufferEntry
	cleared      bool
	forwarder    func([]pluginipc.WorkspaceBufferEntry, bool) error
	commandName  string
	commandEntry string
	commandID    string
	commandRaw   string
	commandArgv  []string
	inputQueue   []string
}

func newPluginWorkspaceState(cfg *pluginipc.WorkspaceConfig) *pluginWorkspaceState {
	state := &pluginWorkspaceState{
		maxEntries: defaultWorkerWorkspaceMaxEntries,
	}
	if cfg == nil || !cfg.Enabled {
		return state
	}
	state.enabled = true
	if cfg.MaxEntries > 0 {
		state.maxEntries = cfg.MaxEntries
	}
	for _, entry := range cfg.History {
		state.appendEntry(entry, false)
	}
	state.pending = nil
	state.cleared = false
	return state
}

func normalizeWorkerWorkspaceLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "error":
		return "error"
	case "warn", "warning":
		return "warn"
	case "debug":
		return "debug"
	default:
		return "info"
	}
}

func normalizeWorkerWorkspaceChannel(channel string) string {
	trimmed := strings.ToLower(strings.TrimSpace(channel))
	if trimmed == "" {
		return "workspace"
	}
	return trimmed
}

func normalizeWorkerWorkspaceMessage(message string) string {
	return message
}

func isWorkerWorkspaceTerminalStreamEntry(entry pluginipc.WorkspaceBufferEntry) bool {
	source := strings.ToLower(strings.TrimSpace(entry.Source))
	channel := normalizeWorkerWorkspaceChannel(entry.Channel)
	return channel == workerWorkspaceChannelStdout &&
		(source == workerWorkspaceSourceWrite || source == workerWorkspaceSourceWriteln)
}

func equalWorkerWorkspaceMetadata(left map[string]string, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		if right[key] != leftValue {
			return false
		}
	}
	return true
}

func canMergeWorkerWorkspaceEntry(left pluginipc.WorkspaceBufferEntry, right pluginipc.WorkspaceBufferEntry) bool {
	if !isWorkerWorkspaceTerminalStreamEntry(left) || !isWorkerWorkspaceTerminalStreamEntry(right) {
		return false
	}
	if normalizeWorkerWorkspaceLevel(left.Level) != normalizeWorkerWorkspaceLevel(right.Level) {
		return false
	}
	return equalWorkerWorkspaceMetadata(left.Metadata, right.Metadata)
}

func isWorkerWorkspaceCommandAction(action string) bool {
	return strings.EqualFold(strings.TrimSpace(action), workerWorkspaceCommandExecuteAction)
}

func isWorkerWorkspaceForwardAction(action string) bool {
	normalized := strings.ToLower(strings.TrimSpace(action))
	return normalized == workerWorkspaceCommandExecuteAction ||
		normalized == workerWorkspaceRuntimeEvalAction ||
		normalized == workerWorkspaceRuntimeInspectAction
}

func parseWorkerWorkspaceStringSliceJSON(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var decoded []interface{}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
		return nil
	}
	out := make([]string, 0, len(decoded))
	for _, item := range decoded {
		value := strings.TrimSpace(fmt.Sprintf("%v", item))
		if value == "" || value == "<nil>" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func cloneWorkspaceBufferEntries(entries []pluginipc.WorkspaceBufferEntry) []pluginipc.WorkspaceBufferEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]pluginipc.WorkspaceBufferEntry, 0, len(entries))
	for _, entry := range entries {
		cloned = append(cloned, pluginipc.WorkspaceBufferEntry{
			Timestamp: strings.TrimSpace(entry.Timestamp),
			Channel:   normalizeWorkerWorkspaceChannel(entry.Channel),
			Level:     normalizeWorkerWorkspaceLevel(entry.Level),
			Message:   entry.Message,
			Source:    strings.TrimSpace(entry.Source),
			Metadata:  mergeStringMaps(entry.Metadata, nil),
		})
	}
	return cloned
}

func normalizeWorkerWorkspaceLimit(limit int, size int) int {
	if size <= 0 {
		return 0
	}
	if limit <= 0 || limit >= size {
		return size
	}
	return limit
}

func (s *pluginWorkspaceState) trimHistory() {
	if s == nil || s.maxEntries <= 0 || len(s.history) <= s.maxEntries {
		return
	}
	s.history = append([]pluginipc.WorkspaceBufferEntry(nil), s.history[len(s.history)-s.maxEntries:]...)
}

func (s *pluginWorkspaceState) setForwarder(forwarder func([]pluginipc.WorkspaceBufferEntry, bool) error) {
	if s == nil {
		return
	}
	s.forwarder = forwarder
}

func (s *pluginWorkspaceState) appendEntry(entry pluginipc.WorkspaceBufferEntry, recordPending bool) {
	if s == nil || !s.enabled {
		return
	}
	if strings.TrimSpace(entry.Timestamp) == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	entry.Channel = normalizeWorkerWorkspaceChannel(entry.Channel)
	entry.Level = normalizeWorkerWorkspaceLevel(entry.Level)
	entry.Source = strings.TrimSpace(entry.Source)
	entry.Metadata = mergeStringMaps(entry.Metadata, nil)
	if len(s.history) > 0 && canMergeWorkerWorkspaceEntry(s.history[len(s.history)-1], entry) {
		last := &s.history[len(s.history)-1]
		last.Message += entry.Message
		last.Timestamp = entry.Timestamp
	} else {
		s.history = append(s.history, entry)
		s.trimHistory()
	}
	if recordPending {
		s.appendPendingEntry(entry)
		if s.forwarder != nil && shouldFlushWorkerWorkspaceForwarder(entry, len(s.pending)) {
			s.flushForwarder(false)
		}
	}
}

func (s *pluginWorkspaceState) write(channel string, level string, message string, source string, metadata map[string]string) {
	if s == nil || !s.enabled {
		return
	}
	entry := pluginipc.WorkspaceBufferEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Channel:   normalizeWorkerWorkspaceChannel(channel),
		Level:     normalizeWorkerWorkspaceLevel(level),
		Message:   normalizeWorkerWorkspaceMessage(message),
		Source:    strings.TrimSpace(source),
		Metadata:  mergeStringMaps(metadata, nil),
	}
	s.appendEntry(entry, true)
}

func (s *pluginWorkspaceState) clear() bool {
	if s == nil || !s.enabled {
		return false
	}
	s.history = nil
	s.pending = nil
	if s.forwarder != nil {
		if err := s.forwarder(nil, true); err == nil {
			s.cleared = false
			return true
		}
	}
	s.cleared = true
	return true
}

func (s *pluginWorkspaceState) configureCommand(action string, params map[string]string) {
	if s == nil {
		return
	}
	s.commandName = ""
	s.commandEntry = ""
	s.commandID = ""
	s.commandRaw = ""
	s.commandArgv = nil
	s.inputQueue = nil
	if !isWorkerWorkspaceCommandAction(action) {
		return
	}
	if len(params) == 0 {
		return
	}
	s.commandName = strings.TrimSpace(params[workerWorkspaceCommandNameParam])
	s.commandEntry = strings.TrimSpace(params[workerWorkspaceCommandEntryParam])
	s.commandID = strings.TrimSpace(params[workerWorkspaceCommandIDParam])
	s.commandRaw = strings.TrimSpace(params[workerWorkspaceCommandRawParam])
	s.commandArgv = parseWorkerWorkspaceStringSliceJSON(params[workerWorkspaceCommandArgvJSONParam])
	s.inputQueue = parseWorkerWorkspaceStringSliceJSON(params[workerWorkspaceCommandInputJSONParam])
}

func (s *pluginWorkspaceState) configureRuntimeConsole(
	action string,
	executionCtx *pluginipc.ExecutionContext,
) {
	if s == nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(action), workerWorkspaceRuntimeEvalAction) &&
		!strings.EqualFold(strings.TrimSpace(action), workerWorkspaceRuntimeInspectAction) {
		return
	}
	s.commandName = strings.TrimSpace(action)
	s.commandEntry = strings.TrimSpace(action)
	s.commandRaw = ""
	s.commandArgv = nil
	s.inputQueue = nil
	if executionCtx == nil || executionCtx.Metadata == nil {
		return
	}
	if commandID := strings.TrimSpace(executionCtx.Metadata[workerWorkspaceExecutionIDMetadata]); commandID != "" {
		s.commandID = commandID
	}
	if rawLine := strings.TrimSpace(executionCtx.Metadata[workerWorkspaceTerminalLineMetadata]); rawLine != "" {
		s.commandRaw = rawLine
		s.commandName = rawLine
	}
}

func (s *pluginWorkspaceState) readInput(prompt string, masked bool, echo bool, source string) (string, bool) {
	if s == nil || !s.enabled {
		return "", false
	}
	s.recordPrompt(prompt, source)
	return s.consumeQueuedInput(masked, echo, source)
}

func (s *pluginWorkspaceState) consumeQueuedInput(masked bool, echo bool, source string) (string, bool) {
	if s == nil || !s.enabled {
		return "", false
	}
	if len(s.inputQueue) == 0 {
		return "", false
	}
	value := s.inputQueue[0]
	s.inputQueue = append([]string(nil), s.inputQueue[1:]...)
	s.recordInputValue(value, masked, echo, source)
	return value, true
}

func (s *pluginWorkspaceState) recordPrompt(prompt string, source string) {
	if s == nil || !s.enabled {
		return
	}
	if trimmedPrompt := strings.TrimSpace(prompt); trimmedPrompt != "" {
		s.write("prompt", "info", trimmedPrompt, source+".prompt", map[string]string{
			"command": s.commandName,
		})
	}
}

func (s *pluginWorkspaceState) recordInputValue(value string, masked bool, echo bool, source string) {
	if s == nil || !s.enabled || !echo {
		return
	}
	message := value
	if masked {
		message = "<masked>"
	}
	s.write("input", "info", message, source, map[string]string{
		"command": s.commandName,
		"masked":  strconv.FormatBool(masked),
	})
}

func (s *pluginWorkspaceState) tail(limit int) []pluginipc.WorkspaceBufferEntry {
	if s == nil || !s.enabled || len(s.history) == 0 {
		return nil
	}
	size := normalizeWorkerWorkspaceLimit(limit, len(s.history))
	return cloneWorkspaceBufferEntries(s.history[len(s.history)-size:])
}

func (s *pluginWorkspaceState) snapshot(limit int) map[string]interface{} {
	if s == nil {
		return map[string]interface{}{
			"enabled":     false,
			"max_entries": 0,
			"entry_count": 0,
			"entries":     []map[string]interface{}{},
		}
	}
	entries := s.tail(limit)
	return map[string]interface{}{
		"enabled":     s.enabled,
		"max_entries": s.maxEntries,
		"entry_count": len(s.history),
		"entries":     workspaceEntriesToMaps(entries),
	}
}

func (s *pluginWorkspaceState) flushDelta() ([]pluginipc.WorkspaceBufferEntry, bool) {
	if s == nil || !s.enabled {
		return nil, false
	}
	entries := cloneWorkspaceBufferEntries(s.pending)
	cleared := s.cleared
	s.pending = nil
	s.cleared = false
	return entries, cleared
}

func workspaceMetadataFromGojaValue(value goja.Value) map[string]string {
	if value == nil || goja.IsNull(value) || goja.IsUndefined(value) {
		return nil
	}
	exported := value.Export()
	switch typed := exported.(type) {
	case map[string]string:
		return mergeStringMaps(typed, nil)
	case map[string]interface{}:
		metadata := make(map[string]string, len(typed))
		for key, item := range typed {
			normalizedKey := strings.TrimSpace(key)
			if normalizedKey == "" || item == nil {
				continue
			}
			metadata[normalizedKey] = fmt.Sprintf("%v", item)
		}
		if len(metadata) == 0 {
			return nil
		}
		return metadata
	default:
		return nil
	}
}

func workspaceEntriesToMaps(entries []pluginipc.WorkspaceBufferEntry) []map[string]interface{} {
	if len(entries) == 0 {
		return nil
	}
	mapped := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		item := map[string]interface{}{
			"timestamp": strings.TrimSpace(entry.Timestamp),
			"channel":   normalizeWorkerWorkspaceChannel(entry.Channel),
			"level":     normalizeWorkerWorkspaceLevel(entry.Level),
			"message":   entry.Message,
		}
		if source := strings.TrimSpace(entry.Source); source != "" {
			item["source"] = source
		}
		if len(entry.Metadata) > 0 {
			item["metadata"] = mergeStringMaps(entry.Metadata, nil)
		}
		mapped = append(mapped, item)
	}
	return mapped
}

func workspaceLimitFromGojaArguments(call goja.FunctionCall, index int) int {
	if len(call.Arguments) <= index {
		return 0
	}
	value := call.Arguments[index]
	if value == nil || goja.IsNull(value) || goja.IsUndefined(value) {
		return 0
	}
	exported := value.Export()
	switch typed := exported.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return 0
}

func argumentAt(call goja.FunctionCall, index int) goja.Value {
	if len(call.Arguments) <= index {
		return nil
	}
	return call.Arguments[index]
}

func (s *pluginWorkspaceState) appendPendingEntry(entry pluginipc.WorkspaceBufferEntry) {
	if s == nil {
		return
	}
	if len(s.pending) > 0 && canMergeWorkerWorkspaceEntry(s.pending[len(s.pending)-1], entry) {
		s.pending[len(s.pending)-1].Message += entry.Message
		s.pending[len(s.pending)-1].Timestamp = entry.Timestamp
		return
	}
	s.pending = append(s.pending, entry)
}

func (s *pluginWorkspaceState) flushForwarder(cleared bool) bool {
	if s == nil || s.forwarder == nil {
		return false
	}
	if !cleared && len(s.pending) == 0 {
		return true
	}
	entries := cloneWorkspaceBufferEntries(s.pending)
	if err := s.forwarder(entries, cleared); err != nil {
		return false
	}
	s.pending = nil
	if cleared {
		s.cleared = false
	}
	return true
}

func shouldFlushWorkerWorkspaceForwarder(entry pluginipc.WorkspaceBufferEntry, pendingCount int) bool {
	if !isWorkerWorkspaceTerminalStreamEntry(entry) {
		return true
	}
	if strings.Contains(entry.Message, "\n") {
		return true
	}
	return pendingCount >= defaultWorkerWorkspaceForwardBatch
}

func parseWorkspaceReadOptions(value goja.Value) (bool, bool) {
	echo := true
	masked := false
	if value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return echo, masked
	}
	record, ok := value.Export().(map[string]interface{})
	if !ok {
		return echo, masked
	}
	if rawEcho, exists := record["echo"]; exists {
		switch typed := rawEcho.(type) {
		case bool:
			echo = typed
		case string:
			echo = strings.TrimSpace(strings.ToLower(typed)) != "false"
		}
	}
	if rawMasked, exists := record["masked"]; exists {
		switch typed := rawMasked.(type) {
		case bool:
			masked = typed
		case string:
			masked = strings.TrimSpace(strings.ToLower(typed)) == "true"
		}
	}
	return echo, masked
}

func configureWorkerWorkspaceHostBridge(
	workspaceState *pluginWorkspaceState,
	hostCfg *pluginipc.HostAPIConfig,
	enabled bool,
) {
	if workspaceState == nil {
		return
	}
	if !enabled || hostCfg == nil || strings.TrimSpace(workspaceState.commandID) == "" {
		workspaceState.setForwarder(nil)
		return
	}
	workspaceState.setForwarder(func(entries []pluginipc.WorkspaceBufferEntry, cleared bool) error {
		_, err := performPluginHostRequest(hostCfg, "host.workspace.append", map[string]interface{}{
			"command_id": workspaceState.commandID,
			"entries":    entries,
			"clear":      cleared,
		})
		return err
	})
}

func requestWorkerWorkspaceInput(
	workspaceState *pluginWorkspaceState,
	hostCfg *pluginipc.HostAPIConfig,
	workspaceHostEnabled bool,
	prompt string,
	masked bool,
	echo bool,
	source string,
) (string, bool, error) {
	if workspaceState == nil {
		return "", false, nil
	}
	if prompt != "" {
		workspaceState.recordPrompt(prompt, source)
	}
	if value, ok := workspaceState.consumeQueuedInput(masked, echo, source); ok {
		return value, true, nil
	}
	if !workspaceHostEnabled || hostCfg == nil {
		return "", false, nil
	}
	result, err := performPluginHostRequest(hostCfg, "host.workspace.read_input", map[string]interface{}{
		"command_id": workspaceState.commandID,
		"prompt":     strings.TrimSpace(prompt),
		"timeout_ms": workerWorkspaceHostInputTimeoutMs,
	})
	if err != nil {
		return "", false, err
	}
	value := ""
	if rawValue, exists := result["value"]; exists && rawValue != nil {
		value = fmt.Sprintf("%v", rawValue)
	}
	workspaceState.recordInputValue(value, masked, echo, source)
	return value, true, nil
}
