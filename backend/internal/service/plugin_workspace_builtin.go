package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pluginipc"
	"auralogic/internal/pluginobs"
)

const (
	pluginWorkspaceBuiltinCommandPrefix        = "builtin/"
	pluginWorkspaceBuiltinCommandHelp          = "builtin/help"
	pluginWorkspaceBuiltinCommandClear         = "builtin/clear"
	pluginWorkspaceBuiltinCommandLogTail       = "builtin/log.tail"
	pluginWorkspaceBuiltinCommandPWD           = "builtin/pwd"
	pluginWorkspaceBuiltinCommandLS            = "builtin/ls"
	pluginWorkspaceBuiltinCommandStat          = "builtin/stat"
	pluginWorkspaceBuiltinCommandCat           = "builtin/cat"
	pluginWorkspaceBuiltinCommandMkdir         = "builtin/mkdir"
	pluginWorkspaceBuiltinCommandFind          = "builtin/find"
	pluginWorkspaceBuiltinCommandGrep          = "builtin/grep"
	pluginWorkspaceBuiltinCommandKVGet         = "builtin/kv.get"
	pluginWorkspaceBuiltinCommandKVSet         = "builtin/kv.set"
	pluginWorkspaceBuiltinCommandKVList        = "builtin/kv.list"
	pluginWorkspaceBuiltinCommandKVDel         = "builtin/kv.del"
	pluginWorkspaceBuiltinEntryPrefix          = "host.workspace.builtin."
	pluginWorkspaceBuiltinSource               = "host.workspace.builtin"
	pluginWorkspaceBuiltinReadLimitBytes int64 = 512 * 1024
	pluginWorkspaceBuiltinGrepLimitBytes int64 = 256 * 1024
	pluginWorkspaceBuiltinGrepMaxFiles         = 512
	pluginWorkspaceBuiltinGrepMaxMatches       = 200
	pluginWorkspaceBuiltinFindMaxEntries       = 512
)

type pluginWorkspaceBuiltinRoots struct {
	CodeRoot string
	DataRoot string
}

type pluginWorkspaceBuiltinResolvedPath struct {
	RelPath     string
	DisplayPath string
	DataRoot    string
	CodeRoot    string
	DataPath    string
	CodePath    string
}

type pluginWorkspaceBuiltinEntry struct {
	Path  string
	Info  os.FileInfo
	Layer string
}

type pluginWorkspaceBuiltinListItem struct {
	Path       string `json:"path"`
	Name       string `json:"name"`
	Layer      string `json:"layer"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Mode       string `json:"mode,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

type pluginWorkspaceBuiltinStatItem struct {
	Path       string `json:"path"`
	Layer      string `json:"layer"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Mode       string `json:"mode,omitempty"`
	ModifiedAt string `json:"modified_at,omitempty"`
}

type pluginWorkspaceBuiltinGrepMatch struct {
	Path  string `json:"path"`
	Line  int    `json:"line"`
	Text  string `json:"text"`
	Layer string `json:"layer"`
}

func pluginWorkspaceBuiltinCommands() []PluginWorkspaceCommand {
	return []PluginWorkspaceCommand{
		{
			Name:        pluginWorkspaceBuiltinCommandHelp,
			Title:       "help",
			Description: "List builtin workspace tools or inspect a live callable alias.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "help",
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandClear,
			Title:       "clear",
			Description: "Clear the current host-managed workspace buffer.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "clear",
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandLogTail,
			Title:       "log.tail",
			Description: "Tail recent workspace buffer entries from the current plugin workspace.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "log.tail",
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandPWD,
			Title:       "pwd",
			Description: "Show the current virtual workspace path.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "pwd",
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandLS,
			Title:       "ls",
			Description: "List plugin files from the merged data/code workspace view.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "ls",
			Permissions: []string{PluginPermissionRuntimeFileSystem},
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandStat,
			Title:       "stat",
			Description: "Show file metadata from the merged plugin workspace view.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "stat",
			Permissions: []string{PluginPermissionRuntimeFileSystem},
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandCat,
			Title:       "cat",
			Description: "Read a text file from plugin code/data storage.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "cat",
			Permissions: []string{PluginPermissionRuntimeFileSystem},
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandMkdir,
			Title:       "mkdir",
			Description: "Create a directory in the plugin data workspace layer.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "mkdir",
			Permissions: []string{PluginPermissionRuntimeFileSystem},
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandFind,
			Title:       "find",
			Description: "Search plugin files and directories by path substring.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "find",
			Permissions: []string{PluginPermissionRuntimeFileSystem},
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandGrep,
			Title:       "grep",
			Description: "Search text inside plugin files from the merged workspace view.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "grep",
			Permissions: []string{PluginPermissionRuntimeFileSystem},
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandKVGet,
			Title:       "kv.get",
			Description: "Read a key from plugin KV storage.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "kv.get",
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandKVSet,
			Title:       "kv.set",
			Description: "Write a key to plugin KV storage.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "kv.set",
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandKVList,
			Title:       "kv.list",
			Description: "List plugin KV keys, optionally filtered by prefix.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "kv.list",
			Builtin:     true,
		},
		{
			Name:        pluginWorkspaceBuiltinCommandKVDel,
			Title:       "kv.del",
			Description: "Delete a key from plugin KV storage.",
			Entry:       pluginWorkspaceBuiltinEntryPrefix + "kv.del",
			Builtin:     true,
		},
	}
}

func resolvePluginWorkspaceBuiltinCommands(plugin *models.Plugin) []PluginWorkspaceCommand {
	if plugin == nil {
		return nil
	}
	policy := resolvePluginCapabilityPolicy(plugin)
	grantedSet := make(map[string]struct{}, len(policy.GrantedPermissions))
	for _, item := range policy.GrantedPermissions {
		normalized := NormalizePluginPermissionKey(item)
		if normalized == "" {
			continue
		}
		grantedSet[normalized] = struct{}{}
	}

	commands := pluginWorkspaceBuiltinCommands()
	out := make([]PluginWorkspaceCommand, 0, len(commands))
	for _, item := range commands {
		item.Permissions = NormalizePluginPermissionList(item.Permissions)
		item.MissingPermissions = resolvePluginWorkspaceCommandMissingPermissions(item.Permissions, grantedSet)
		item.Granted = len(item.MissingPermissions) == 0
		out = append(out, item)
	}
	return out
}

func resolvePluginWorkspaceCommandMissingPermissions(
	permissions []string,
	grantedSet map[string]struct{},
) []string {
	missing := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		if _, exists := grantedSet[permission]; exists {
			continue
		}
		missing = append(missing, permission)
	}
	return missing
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinCommand(
	plugin *models.Plugin,
	runtime string,
	command *PluginWorkspaceCommand,
	argv []string,
	inputLines []string,
	execCtx *ExecutionContext,
) (result *ExecutionResult, execErr error) {
	if s == nil {
		return nil, fmt.Errorf("plugin manager is unavailable")
	}
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	if command == nil || !command.Builtin {
		return nil, fmt.Errorf("workspace builtin command is unavailable")
	}

	baseExecCtx := execCtx
	if baseExecCtx == nil {
		baseExecCtx = &ExecutionContext{}
	}
	commandID := strings.TrimSpace(baseExecCtx.Metadata[PluginExecutionMetadataID])
	params, err := buildPluginWorkspaceCommandParams(command, argv, inputLines, commandID)
	if err != nil {
		return nil, err
	}
	preparedExecCtx, task := s.startPluginExecutionTask(plugin, runtime, pluginWorkspaceCommandExecuteAction, params, baseExecCtx, false)
	ctx, cancel := s.executeTimeoutContext(preparedExecCtx, 0)
	defer cancel()
	preparedExecCtx.RequestContext = ctx

	startedAt := time.Now().UTC()
	result, execErr = s.runPluginWorkspaceBuiltinCommand(ctx, plugin, command, argv, inputLines, preparedExecCtx)
	if task != nil {
		snapshot := s.completePluginExecutionTask(task, result, execErr)
		preparedExecCtx.Metadata = mergePluginExecutionTaskMetadata(preparedExecCtx.Metadata, snapshot)
		result = applyPluginExecutionTaskToResult(result, snapshot)
	}

	s.writePluginWorkspaceBuiltinTranscript(plugin, runtime, command, argv, preparedExecCtx, result, execErr)

	duration := int(time.Since(startedAt).Milliseconds())
	if duration < 0 {
		duration = 0
	}
	success := execErr == nil && result != nil && result.Success
	timedOut := isPluginExecutionTimeoutError(execErr)
	pluginobs.RecordExecution(plugin.ID, plugin.Name, runtime, pluginWorkspaceCommandExecuteAction, int64(duration), success, timedOut)
	s.emitPluginExecutionAuditEvent(plugin, runtime, pluginWorkspaceCommandExecuteAction, params, preparedExecCtx, result, execErr, duration)
	if s.db != nil {
		s.recordExecution(plugin.ID, pluginWorkspaceCommandExecuteAction, params, preparedExecCtx, result, execErr, duration)
	}
	return result, execErr
}

func (s *PluginManagerService) runPluginWorkspaceBuiltinCommand(
	ctx context.Context,
	plugin *models.Plugin,
	command *PluginWorkspaceCommand,
	argv []string,
	inputLines []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if command == nil {
		return nil, fmt.Errorf("workspace command is nil")
	}
	switch strings.ToLower(strings.TrimSpace(command.Name)) {
	case pluginWorkspaceBuiltinCommandHelp:
		return s.executePluginWorkspaceBuiltinHelp(plugin, argv, execCtx)
	case pluginWorkspaceBuiltinCommandClear:
		return s.executePluginWorkspaceBuiltinClear(plugin, execCtx)
	case pluginWorkspaceBuiltinCommandLogTail:
		return s.executePluginWorkspaceBuiltinLogTail(plugin, argv, execCtx)
	case pluginWorkspaceBuiltinCommandPWD:
		return s.executePluginWorkspaceBuiltinPWD(plugin, execCtx)
	case pluginWorkspaceBuiltinCommandLS:
		return s.executePluginWorkspaceBuiltinLS(ctx, plugin, argv, execCtx)
	case pluginWorkspaceBuiltinCommandStat:
		return s.executePluginWorkspaceBuiltinStat(ctx, plugin, argv, execCtx)
	case pluginWorkspaceBuiltinCommandCat:
		return s.executePluginWorkspaceBuiltinCat(ctx, plugin, argv, execCtx)
	case pluginWorkspaceBuiltinCommandMkdir:
		return s.executePluginWorkspaceBuiltinMkdir(ctx, plugin, argv, execCtx)
	case pluginWorkspaceBuiltinCommandFind:
		return s.executePluginWorkspaceBuiltinFind(ctx, plugin, argv, execCtx)
	case pluginWorkspaceBuiltinCommandGrep:
		return s.executePluginWorkspaceBuiltinGrep(ctx, plugin, argv, inputLines, execCtx)
	case pluginWorkspaceBuiltinCommandKVGet:
		return s.executePluginWorkspaceBuiltinKVGet(plugin, argv, execCtx)
	case pluginWorkspaceBuiltinCommandKVSet:
		return s.executePluginWorkspaceBuiltinKVSet(plugin, argv, inputLines, execCtx)
	case pluginWorkspaceBuiltinCommandKVList:
		return s.executePluginWorkspaceBuiltinKVList(plugin, argv, execCtx)
	case pluginWorkspaceBuiltinCommandKVDel:
		return s.executePluginWorkspaceBuiltinKVDel(plugin, argv, execCtx)
	default:
		return nil, fmt.Errorf("unsupported workspace builtin command %q", command.Name)
	}
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinHelp(
	plugin *models.Plugin,
	argv []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	commands := s.ResolvePluginWorkspaceCommandsForPlugin(plugin)
	runtimeCallables := []string(nil)
	if state, err := s.GetPluginWorkspaceRuntimeState(plugin); err == nil && len(state.CallablePaths) > 0 {
		runtimeCallables = append(runtimeCallables, state.CallablePaths...)
	}
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		outputLines := make([]string, 0, len(commands))
		for _, item := range commands {
			status := "available"
			if !item.Granted {
				status = "missing_permissions"
			}
			label := strings.TrimSpace(item.Title)
			if label == "" {
				label = item.Name
			}
			outputLines = append(outputLines, fmt.Sprintf("%s | %s | %s", label, status, strings.TrimSpace(item.Description)))
		}
		if len(runtimeCallables) > 0 {
			outputLines = append(outputLines, "")
			outputLines = append(outputLines, "runtime exports:")
			for _, item := range runtimeCallables {
				outputLines = append(outputLines, "  - "+item)
			}
		}
		return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
			"commands":               commands,
			"count":                  len(commands),
			"runtime_callable_paths": runtimeCallables,
			"runtime_callable_count": len(runtimeCallables),
			"output":                 strings.Join(outputLines, "\n"),
		}, map[string]string{
			pluginStorageAccessMetaKey: pluginStorageAccessNone,
			"runtime":                  "host",
			"workspace_builtin":        pluginWorkspaceBuiltinCommandHelp,
		}), nil
	}

	commandName := NormalizePluginWorkspaceShellCommandName(argv[0])
	command, err := s.ResolvePluginWorkspaceCommandForPlugin(plugin, commandName)
	if err != nil {
		target := strings.TrimSpace(argv[0])
		for _, callable := range runtimeCallables {
			if !strings.EqualFold(strings.TrimSpace(callable), target) {
				continue
			}
			outputLines := []string{
				"name: " + callable,
				"summary: runtime callable exported by the current plugin VM",
			}
			return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
				"runtime_callable": callable,
				"output":           strings.Join(outputLines, "\n"),
			}, map[string]string{
				pluginStorageAccessMetaKey: pluginStorageAccessNone,
				"runtime":                  "host",
				"workspace_builtin":        pluginWorkspaceBuiltinCommandHelp,
			}), nil
		}
		return nil, err
	}
	outputLines := []string{
		"name: " + command.Name,
		"title: " + pluginWorkspaceBuiltinDisplayName(command),
		"description: " + strings.TrimSpace(command.Description),
		fmt.Sprintf("interactive: %t", command.Interactive),
		fmt.Sprintf("builtin: %t", command.Builtin),
	}
	if len(command.Permissions) > 0 {
		outputLines = append(outputLines, "permissions: "+strings.Join(command.Permissions, ", "))
	}
	if len(command.MissingPermissions) > 0 {
		outputLines = append(outputLines, "missing_permissions: "+strings.Join(command.MissingPermissions, ", "))
	}
	if entry := strings.TrimSpace(command.Entry); entry != "" {
		outputLines = append(outputLines, "entry: "+entry)
	}
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"command": command,
		"output":  strings.Join(outputLines, "\n"),
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessNone,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandHelp,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinClear(
	plugin *models.Plugin,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	runtime := strings.ToLower(strings.TrimSpace(plugin.Runtime))
	if runtime != PluginRuntimeJSWorker {
		return nil, fmt.Errorf("workspace is only available for js_worker plugins")
	}
	s.ApplyPluginWorkspaceDelta(plugin.ID, plugin.Name, runtime, nil, nil, true)
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"cleared": true,
		"output":  "workspace buffer cleared",
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessNone,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandClear,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinLogTail(
	plugin *models.Plugin,
	argv []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	runtime := strings.ToLower(strings.TrimSpace(plugin.Runtime))
	if runtime != PluginRuntimeJSWorker {
		return nil, fmt.Errorf("workspace is only available for js_worker plugins")
	}

	limit := defaultPluginWorkspaceSeedLimit
	channel := ""
	level := ""
	for idx := 0; idx < len(argv); idx++ {
		item := strings.TrimSpace(argv[idx])
		if item == "" {
			continue
		}
		switch item {
		case "--channel":
			if idx+1 >= len(argv) {
				return nil, fmt.Errorf("log.tail --channel requires a value")
			}
			idx++
			channel = strings.ToLower(strings.TrimSpace(argv[idx]))
		case "--level":
			if idx+1 >= len(argv) {
				return nil, fmt.Errorf("log.tail --level requires a value")
			}
			idx++
			switch strings.ToLower(strings.TrimSpace(argv[idx])) {
			case "info", "warn", "warning", "error", "debug":
				level = normalizePluginWorkspaceLevel(argv[idx])
			default:
				return nil, fmt.Errorf("unsupported log.tail level %q", strings.TrimSpace(argv[idx]))
			}
		case "--limit":
			if idx+1 >= len(argv) {
				return nil, fmt.Errorf("log.tail --limit requires a value")
			}
			idx++
			parsed, err := strconv.Atoi(strings.TrimSpace(argv[idx]))
			if err != nil {
				return nil, fmt.Errorf("log.tail limit must be an integer")
			}
			limit = parsed
		default:
			if parsed, err := strconv.Atoi(item); err == nil {
				limit = parsed
				continue
			}
			if channel == "" {
				channel = strings.ToLower(item)
				continue
			}
			return nil, fmt.Errorf("unsupported log.tail argument %q", item)
		}
	}
	limit = normalizePluginWorkspaceLimit(limit, defaultPluginWorkspaceSeedLimit)

	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, runtime)
	snapshot := buffer.snapshot(limit)
	s.workspaceMu.Unlock()
	entries := snapshot.Entries

	filtered := make([]PluginWorkspaceBufferEntry, 0, len(entries))
	for _, entry := range entries {
		if channel != "" && strings.ToLower(strings.TrimSpace(entry.Channel)) != channel {
			continue
		}
		if level != "" && normalizePluginWorkspaceLevel(entry.Level) != level {
			continue
		}
		filtered = append(filtered, entry)
	}
	output := formatPluginWorkspaceBuiltinLogTail(filtered)
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"entries":   filtered,
		"count":     len(filtered),
		"limit":     limit,
		"channel":   channel,
		"level":     level,
		"output":    output,
		"truncated": snapshot.HasMore,
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessNone,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandLogTail,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinPWD(
	plugin *models.Plugin,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if _, err := s.resolvePluginWorkspaceBuiltinRoots(plugin); err != nil {
		return nil, err
	}
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"path":   ".",
		"output": ".",
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessNone,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandPWD,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinLS(
	ctx context.Context,
	plugin *models.Plugin,
	argv []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if err := pluginWorkspaceBuiltinCheckContext(ctx); err != nil {
		return nil, err
	}
	roots, err := s.resolvePluginWorkspaceBuiltinRoots(plugin)
	if err != nil {
		return nil, err
	}
	pathArg := "."
	if len(argv) > 0 {
		pathArg = strings.TrimSpace(argv[0])
		if pathArg == "" {
			pathArg = "."
		}
	}
	resolved, err := resolvePluginWorkspaceBuiltinPath(roots, pathArg, true)
	if err != nil {
		return nil, err
	}
	entry, err := lookupPluginWorkspaceBuiltinEntry(resolved)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("path not found: %s", resolved.DisplayPath)
	}

	items := make([]pluginWorkspaceBuiltinListItem, 0)
	if !entry.Info.IsDir() {
		items = append(items, newPluginWorkspaceBuiltinListItem(resolved.DisplayPath, entry.Layer, entry.Info))
	} else {
		items, err = listPluginWorkspaceBuiltinDirectory(ctx, resolved)
		if err != nil {
			return nil, err
		}
	}
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"path":   resolved.DisplayPath,
		"items":  items,
		"count":  len(items),
		"output": formatPluginWorkspaceBuiltinList(items),
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessNone,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandLS,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinStat(
	ctx context.Context,
	plugin *models.Plugin,
	argv []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if err := pluginWorkspaceBuiltinCheckContext(ctx); err != nil {
		return nil, err
	}
	roots, err := s.resolvePluginWorkspaceBuiltinRoots(plugin)
	if err != nil {
		return nil, err
	}
	pathArg := "."
	if len(argv) > 0 {
		pathArg = strings.TrimSpace(argv[0])
		if pathArg == "" {
			pathArg = "."
		}
	}
	resolved, err := resolvePluginWorkspaceBuiltinPath(roots, pathArg, true)
	if err != nil {
		return nil, err
	}
	entry, err := lookupPluginWorkspaceBuiltinEntry(resolved)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("path not found: %s", resolved.DisplayPath)
	}
	stat := pluginWorkspaceBuiltinStatItem{
		Path:       resolved.DisplayPath,
		Layer:      entry.Layer,
		Type:       pluginWorkspaceBuiltinEntryType(entry.Info),
		Size:       entry.Info.Size(),
		Mode:       entry.Info.Mode().String(),
		ModifiedAt: entry.Info.ModTime().UTC().Format(time.RFC3339),
	}
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"path":   resolved.DisplayPath,
		"item":   stat,
		"output": formatPluginWorkspaceBuiltinStat(stat),
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessNone,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandStat,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinCat(
	ctx context.Context,
	plugin *models.Plugin,
	argv []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if err := pluginWorkspaceBuiltinCheckContext(ctx); err != nil {
		return nil, err
	}
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return nil, fmt.Errorf("cat requires a file path")
	}
	roots, err := s.resolvePluginWorkspaceBuiltinRoots(plugin)
	if err != nil {
		return nil, err
	}
	resolved, err := resolvePluginWorkspaceBuiltinPath(roots, argv[0], false)
	if err != nil {
		return nil, err
	}
	entry, err := lookupPluginWorkspaceBuiltinEntry(resolved)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("path not found: %s", resolved.DisplayPath)
	}
	if entry.Info.IsDir() {
		return nil, fmt.Errorf("path is a directory: %s", resolved.DisplayPath)
	}
	content, truncated, err := readPluginWorkspaceBuiltinTextFile(ctx, entry.Path, pluginWorkspaceBuiltinReadLimitBytes)
	if err != nil {
		return nil, err
	}
	output := content
	if truncated {
		output += "\n...[truncated]"
	}
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"path":      resolved.DisplayPath,
		"layer":     entry.Layer,
		"content":   content,
		"size":      entry.Info.Size(),
		"truncated": truncated,
		"output":    output,
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessNone,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandCat,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinMkdir(
	ctx context.Context,
	plugin *models.Plugin,
	argv []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if err := pluginWorkspaceBuiltinCheckContext(ctx); err != nil {
		return nil, err
	}
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return nil, fmt.Errorf("mkdir requires a directory path")
	}
	roots, err := s.resolvePluginWorkspaceBuiltinRoots(plugin)
	if err != nil {
		return nil, err
	}
	resolved, err := resolvePluginWorkspaceBuiltinPath(roots, argv[0], false)
	if err != nil {
		return nil, err
	}
	if info, statErr := os.Stat(resolved.CodePath); statErr == nil && !info.IsDir() {
		return nil, fmt.Errorf("path is a file in code layer: %s", resolved.DisplayPath)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return nil, statErr
	}
	created := false
	if info, statErr := os.Stat(resolved.DataPath); statErr == nil {
		if !info.IsDir() {
			return nil, fmt.Errorf("path is a file in data layer: %s", resolved.DisplayPath)
		}
	} else if os.IsNotExist(statErr) {
		created = true
	} else if statErr != nil {
		return nil, statErr
	}
	if err := os.MkdirAll(resolved.DataPath, 0o755); err != nil {
		return nil, err
	}
	info, err := os.Stat(resolved.DataPath)
	if err != nil {
		return nil, err
	}
	item := pluginWorkspaceBuiltinStatItem{
		Path:       resolved.DisplayPath,
		Layer:      "data",
		Type:       pluginWorkspaceBuiltinEntryType(info),
		Size:       info.Size(),
		Mode:       info.Mode().String(),
		ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
	}
	output := formatPluginWorkspaceBuiltinStat(item) + fmt.Sprintf("\ncreated: %t", created)
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"path":    resolved.DisplayPath,
		"created": created,
		"item":    item,
		"output":  output,
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessNone,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandMkdir,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinFind(
	ctx context.Context,
	plugin *models.Plugin,
	argv []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if err := pluginWorkspaceBuiltinCheckContext(ctx); err != nil {
		return nil, err
	}
	ignoreCase := false
	remaining := make([]string, 0, len(argv))
	for _, item := range argv {
		trimmed := strings.TrimSpace(item)
		switch trimmed {
		case "", " ":
			continue
		case "-i", "--ignore-case":
			ignoreCase = true
		default:
			remaining = append(remaining, trimmed)
		}
	}
	if len(remaining) == 0 {
		return nil, fmt.Errorf("find requires a pattern")
	}
	pattern := remaining[0]
	pathArg := "."
	if len(remaining) > 1 {
		pathArg = remaining[1]
	}
	roots, err := s.resolvePluginWorkspaceBuiltinRoots(plugin)
	if err != nil {
		return nil, err
	}
	resolved, err := resolvePluginWorkspaceBuiltinPath(roots, pathArg, true)
	if err != nil {
		return nil, err
	}
	entry, err := lookupPluginWorkspaceBuiltinEntry(resolved)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("path not found: %s", resolved.DisplayPath)
	}

	itemsByPath := make(map[string]pluginWorkspaceBuiltinListItem)
	matchPattern := pattern
	if ignoreCase {
		matchPattern = strings.ToLower(matchPattern)
	}
	matchItem := func(path string, name string) bool {
		candidatePath := path
		candidateName := name
		if ignoreCase {
			candidatePath = strings.ToLower(candidatePath)
			candidateName = strings.ToLower(candidateName)
		}
		return strings.Contains(candidatePath, matchPattern) || strings.Contains(candidateName, matchPattern)
	}
	if err := walkPluginWorkspaceBuiltinTreeMatches(
		ctx,
		itemsByPath,
		resolved.DataRoot,
		resolved.DataPath,
		resolved.RelPath,
		"data",
		pluginWorkspaceBuiltinFindMaxEntries,
		matchItem,
	); err != nil && err != io.EOF {
		return nil, err
	}
	truncated := len(itemsByPath) >= pluginWorkspaceBuiltinFindMaxEntries
	if !truncated {
		if err := walkPluginWorkspaceBuiltinTreeMatches(
			ctx,
			itemsByPath,
			resolved.CodeRoot,
			resolved.CodePath,
			resolved.RelPath,
			"code",
			pluginWorkspaceBuiltinFindMaxEntries,
			matchItem,
		); err != nil && err != io.EOF {
			return nil, err
		}
		truncated = len(itemsByPath) >= pluginWorkspaceBuiltinFindMaxEntries
	}

	items := make([]pluginWorkspaceBuiltinListItem, 0, len(itemsByPath))
	for _, item := range itemsByPath {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		left := strings.ToLower(items[i].Path)
		right := strings.ToLower(items[j].Path)
		if left == right {
			return items[i].Layer < items[j].Layer
		}
		return left < right
	})
	output := formatPluginWorkspaceBuiltinList(items)
	if truncated {
		output += "\ntruncated=true"
	}
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"pattern":     pattern,
		"path":        resolved.DisplayPath,
		"ignore_case": ignoreCase,
		"items":       items,
		"count":       len(items),
		"truncated":   truncated,
		"output":      output,
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessNone,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandFind,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinGrep(
	ctx context.Context,
	plugin *models.Plugin,
	argv []string,
	inputLines []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if err := pluginWorkspaceBuiltinCheckContext(ctx); err != nil {
		return nil, err
	}
	ignoreCase := false
	remaining := make([]string, 0, len(argv))
	for _, item := range argv {
		trimmed := strings.TrimSpace(item)
		switch trimmed {
		case "", " ":
			continue
		case "-i", "--ignore-case":
			ignoreCase = true
		default:
			remaining = append(remaining, trimmed)
		}
	}
	if len(remaining) == 0 {
		return nil, fmt.Errorf("grep requires a pattern")
	}
	pattern := remaining[0]
	pathArg := "."
	usePipeInput := false
	if len(remaining) > 1 {
		pathArg = remaining[1]
	} else if len(inputLines) > 0 {
		usePipeInput = true
	}
	matches := make([]pluginWorkspaceBuiltinGrepMatch, 0)
	scannedFiles := 0
	matchLimitHit := false
	truncatedFiles := false
	resolvedPath := pathArg

	matchLines := func(lines []string, targetPath string, layer string) {
		for lineNo, line := range lines {
			lineText := line
			candidateText := lineText
			candidatePattern := pattern
			if ignoreCase {
				candidateText = strings.ToLower(candidateText)
				candidatePattern = strings.ToLower(candidatePattern)
			}
			if !strings.Contains(candidateText, candidatePattern) {
				continue
			}
			matches = append(matches, pluginWorkspaceBuiltinGrepMatch{
				Path:  targetPath,
				Line:  lineNo + 1,
				Text:  lineText,
				Layer: layer,
			})
			if len(matches) >= pluginWorkspaceBuiltinGrepMaxMatches {
				matchLimitHit = true
				break
			}
		}
	}

	if usePipeInput {
		matchLines(inputLines, "(stdin)", "pipe")
		scannedFiles = 1
	} else {
		roots, err := s.resolvePluginWorkspaceBuiltinRoots(plugin)
		if err != nil {
			return nil, err
		}
		resolved, err := resolvePluginWorkspaceBuiltinPath(roots, pathArg, true)
		if err != nil {
			return nil, err
		}
		resolvedPath = resolved.DisplayPath
		entry, err := lookupPluginWorkspaceBuiltinEntry(resolved)
		if err != nil {
			return nil, err
		}
		if entry == nil {
			return nil, fmt.Errorf("path not found: %s", resolved.DisplayPath)
		}

		files, truncatedLookup, err := collectPluginWorkspaceBuiltinFiles(ctx, resolved)
		if err != nil {
			return nil, err
		}
		truncatedFiles = truncatedLookup
		if !entry.Info.IsDir() {
			files = []pluginWorkspaceBuiltinEntry{{Path: entry.Path, Info: entry.Info, Layer: entry.Layer}}
			truncatedFiles = false
		}

		for _, fileEntry := range files {
			if err := pluginWorkspaceBuiltinCheckContext(ctx); err != nil {
				return nil, err
			}
			content, _, readErr := readPluginWorkspaceBuiltinTextFile(
				ctx,
				fileEntry.Path,
				pluginWorkspaceBuiltinGrepLimitBytes,
			)
			if readErr != nil {
				continue
			}
			scannedFiles++
			filePath := pluginWorkspaceBuiltinRelFromResolved(resolved, fileEntry.Path, fileEntry.Layer)
			if filePath == "" {
				filePath = filepath.ToSlash(filepath.Base(fileEntry.Path))
			}
			matchLines(splitPluginWorkspaceBuiltinLines(content), filePath, fileEntry.Layer)
			if matchLimitHit {
				break
			}
		}
	}

	truncated := truncatedFiles || matchLimitHit
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"pattern":       pattern,
		"path":          resolvedPath,
		"ignore_case":   ignoreCase,
		"matches":       matches,
		"count":         len(matches),
		"scanned_files": scannedFiles,
		"used_pipe":     usePipeInput,
		"truncated":     truncated,
		"output":        formatPluginWorkspaceBuiltinGrep(matches, scannedFiles, truncated),
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessNone,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandGrep,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinKVGet(
	plugin *models.Plugin,
	argv []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return nil, fmt.Errorf("kv.get requires a key")
	}
	key := strings.TrimSpace(argv[0])
	snapshot, err := s.loadPluginStorageSnapshot(plugin.ID)
	if err != nil {
		return nil, err
	}
	value, exists := snapshot[key]
	output := fmt.Sprintf("exists: %t\nkey: %s", exists, key)
	if exists {
		output += "\nvalue: " + value
	}
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"key":    key,
		"exists": exists,
		"value":  value,
		"output": output,
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessRead,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandKVGet,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinKVSet(
	plugin *models.Plugin,
	argv []string,
	inputLines []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return nil, fmt.Errorf("kv.set requires a key")
	}
	key := strings.TrimSpace(argv[0])
	value := ""
	if len(argv) > 1 {
		value = strings.Join(argv[1:], " ")
	} else if len(inputLines) > 0 {
		value = strings.Join(inputLines, "\n")
	} else {
		return nil, fmt.Errorf("kv.set requires a value")
	}
	release := acquirePluginStorageExecutionLock(plugin.ID, pluginStorageAccessWrite)
	defer release()
	snapshot, err := s.loadPluginStorageSnapshot(plugin.ID)
	if err != nil {
		return nil, err
	}
	nextSnapshot := cloneStringMap(snapshot)
	if nextSnapshot == nil {
		nextSnapshot = make(map[string]string, 1)
	}
	previousValue, existed := nextSnapshot[key]
	nextSnapshot[key] = value
	if err := s.replacePluginStorageSnapshot(plugin.ID, nextSnapshot); err != nil {
		return nil, err
	}
	changed := !existed || previousValue != value
	output := strings.Join([]string{
		fmt.Sprintf("key: %s", key),
		fmt.Sprintf("created: %t", !existed),
		fmt.Sprintf("changed: %t", changed),
		"value: " + value,
	}, "\n")
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"key":            key,
		"value":          value,
		"created":        !existed,
		"changed":        changed,
		"previous_value": previousValue,
		"output":         output,
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessWrite,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandKVSet,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinKVList(
	plugin *models.Plugin,
	argv []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	prefix := ""
	if len(argv) > 0 {
		prefix = strings.TrimSpace(argv[0])
	}
	snapshot, err := s.loadPluginStorageSnapshot(plugin.ID)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(snapshot))
	for key := range snapshot {
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	output := "(empty)"
	if len(keys) > 0 {
		output = strings.Join(keys, "\n")
	}
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"prefix": prefix,
		"keys":   keys,
		"count":  len(keys),
		"output": output,
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessRead,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandKVList,
	}), nil
}

func (s *PluginManagerService) executePluginWorkspaceBuiltinKVDel(
	plugin *models.Plugin,
	argv []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return nil, fmt.Errorf("kv.del requires a key")
	}
	key := strings.TrimSpace(argv[0])
	release := acquirePluginStorageExecutionLock(plugin.ID, pluginStorageAccessWrite)
	defer release()
	snapshot, err := s.loadPluginStorageSnapshot(plugin.ID)
	if err != nil {
		return nil, err
	}
	nextSnapshot := cloneStringMap(snapshot)
	if nextSnapshot == nil {
		nextSnapshot = make(map[string]string)
	}
	previousValue, existed := nextSnapshot[key]
	if existed {
		delete(nextSnapshot, key)
		if err := s.replacePluginStorageSnapshot(plugin.ID, nextSnapshot); err != nil {
			return nil, err
		}
	}
	output := strings.Join([]string{
		fmt.Sprintf("key: %s", key),
		fmt.Sprintf("deleted: %t", existed),
	}, "\n")
	if existed {
		output += "\nprevious_value: " + previousValue
	}
	return buildPluginWorkspaceBuiltinResult(execCtx, map[string]interface{}{
		"key":            key,
		"deleted":        existed,
		"exists":         existed,
		"previous_value": previousValue,
		"output":         output,
	}, map[string]string{
		pluginStorageAccessMetaKey: pluginStorageAccessWrite,
		"runtime":                  "host",
		"workspace_builtin":        pluginWorkspaceBuiltinCommandKVDel,
	}), nil
}

func buildPluginWorkspaceBuiltinResult(
	execCtx *ExecutionContext,
	data map[string]interface{},
	extraMetadata map[string]string,
) *ExecutionResult {
	var metadata map[string]string
	if execCtx != nil {
		metadata = cloneStringMap(execCtx.Metadata)
	}
	if metadata == nil {
		metadata = make(map[string]string, len(extraMetadata))
	}
	for key, value := range extraMetadata {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		metadata[normalizedKey] = value
	}
	if data == nil {
		data = map[string]interface{}{}
	}
	return &ExecutionResult{
		Success:  true,
		Data:     data,
		Metadata: metadata,
	}
}

func (s *PluginManagerService) writePluginWorkspaceBuiltinTranscript(
	plugin *models.Plugin,
	runtime string,
	command *PluginWorkspaceCommand,
	argv []string,
	execCtx *ExecutionContext,
	result *ExecutionResult,
	execErr error,
) {
	if s == nil || plugin == nil || command == nil {
		return
	}
	var baseMetadata map[string]string
	if execCtx != nil {
		baseMetadata = cloneStringMap(execCtx.Metadata)
	}
	if baseMetadata == nil {
		baseMetadata = map[string]string{}
	}
	baseMetadata["action"] = pluginWorkspaceBuiltinAction(command.Name)
	baseMetadata["command"] = strings.TrimSpace(command.Name)
	baseMetadata["workspace_command_builtin"] = "true"
	entries := make([]pluginipc.WorkspaceBufferEntry, 0, 2)
	if strings.TrimSpace(baseMetadata["workspace_terminal_line"]) == "" {
		entries = append(entries, pluginipc.WorkspaceBufferEntry{
			Channel:  "command",
			Level:    "info",
			Message:  pluginWorkspaceBuiltinInvocationLine(command, argv),
			Source:   pluginWorkspaceBuiltinSource,
			Metadata: cloneStringMap(baseMetadata),
		})
	}
	if execErr != nil {
		entries = append(entries, pluginipc.WorkspaceBufferEntry{
			Channel:  "stderr",
			Level:    "error",
			Message:  strings.TrimSpace(execErr.Error()),
			Source:   pluginWorkspaceBuiltinSource,
			Metadata: cloneStringMap(baseMetadata),
		})
	} else {
		message := ""
		level := "info"
		if result != nil && len(result.Data) > 0 {
			if output, ok := result.Data["output"].(string); ok {
				message = strings.TrimSpace(output)
			}
		}
		if message == "" && result != nil && strings.TrimSpace(result.Error) != "" {
			message = strings.TrimSpace(result.Error)
			level = "error"
		}
		if message == "" {
			message = "(no output)"
		}
		entries = append(entries, pluginipc.WorkspaceBufferEntry{
			Channel:  "stdout",
			Level:    level,
			Message:  message,
			Source:   pluginWorkspaceBuiltinSource,
			Metadata: cloneStringMap(baseMetadata),
		})
	}
	s.ApplyPluginWorkspaceDelta(plugin.ID, plugin.Name, runtime, baseMetadata, entries, false)
}

func pluginWorkspaceBuiltinInvocationLine(command *PluginWorkspaceCommand, argv []string) string {
	parts := []string{pluginWorkspaceBuiltinDisplayName(command)}
	for _, item := range argv {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		parts = append(parts, trimmed)
	}
	return "$ " + strings.Join(parts, " ")
}

func pluginWorkspaceBuiltinDisplayName(command *PluginWorkspaceCommand) string {
	if command == nil {
		return "builtin"
	}
	if title := strings.TrimSpace(command.Title); title != "" {
		return title
	}
	name := strings.TrimSpace(command.Name)
	return strings.TrimPrefix(name, pluginWorkspaceBuiltinCommandPrefix)
}

func pluginWorkspaceBuiltinAction(commandName string) string {
	normalized := strings.TrimSpace(strings.ToLower(commandName))
	normalized = strings.TrimPrefix(normalized, pluginWorkspaceBuiltinCommandPrefix)
	normalized = strings.ReplaceAll(normalized, "/", ".")
	return "workspace.command.builtin." + normalized
}

func (s *PluginManagerService) resolvePluginWorkspaceBuiltinRoots(plugin *models.Plugin) (pluginWorkspaceBuiltinRoots, error) {
	if plugin == nil {
		return pluginWorkspaceBuiltinRoots{}, fmt.Errorf("plugin is nil")
	}
	if !strings.EqualFold(strings.TrimSpace(plugin.Runtime), PluginRuntimeJSWorker) {
		return pluginWorkspaceBuiltinRoots{}, fmt.Errorf("workspace builtin commands are only available for js_worker plugins")
	}

	codeRoot := ""
	packagePath := strings.TrimSpace(plugin.PackagePath)
	if packagePath != "" {
		if root, err := ResolveJSWorkerPackageRoot(packagePath); err == nil {
			if info, statErr := os.Stat(root); statErr == nil && info.IsDir() {
				codeRoot = filepath.Clean(root)
			}
		}
	}
	if codeRoot == "" {
		scriptPath, err := ResolveJSWorkerScriptPath(plugin.Address, plugin.PackagePath)
		if err != nil {
			return pluginWorkspaceBuiltinRoots{}, err
		}
		codeRoot = detectPluginWorkspaceBuiltinModuleRoot(scriptPath)
	}
	codeInfo, err := os.Stat(codeRoot)
	if err != nil {
		return pluginWorkspaceBuiltinRoots{}, fmt.Errorf("plugin code root is unavailable: %w", err)
	}
	if !codeInfo.IsDir() {
		return pluginWorkspaceBuiltinRoots{}, fmt.Errorf("plugin code root is not a directory: %s", filepath.ToSlash(codeRoot))
	}

	artifactRoot, err := resolvePluginWorkspaceBuiltinArtifactRoot(s.cfg)
	if err != nil {
		return pluginWorkspaceBuiltinRoots{}, err
	}
	dataLeaf := pluginWorkspaceBuiltinDataLeaf(plugin.ID, plugin.Name)
	dataRoot := filepath.Clean(filepath.Join(artifactRoot, "data", dataLeaf))
	return pluginWorkspaceBuiltinRoots{
		CodeRoot: filepath.Clean(codeRoot),
		DataRoot: dataRoot,
	}, nil
}

func resolvePluginWorkspaceBuiltinArtifactRoot(cfg *config.Config) (string, error) {
	artifactRoot := ""
	if cfg != nil {
		artifactRoot = filepath.Clean(filepath.FromSlash(strings.TrimSpace(cfg.Plugin.ArtifactDir)))
	}
	if artifactRoot == "" || artifactRoot == "." {
		artifactRoot = filepath.Join("data", "plugins")
	}
	if !filepath.IsAbs(artifactRoot) {
		absRoot, err := filepath.Abs(artifactRoot)
		if err != nil {
			return "", fmt.Errorf("resolve plugin artifact dir failed: %w", err)
		}
		artifactRoot = filepath.Clean(absRoot)
	}
	if err := os.MkdirAll(artifactRoot, 0o755); err != nil {
		return "", fmt.Errorf("prepare plugin artifact dir failed: %w", err)
	}
	return artifactRoot, nil
}

func pluginWorkspaceBuiltinDataLeaf(pluginID uint, pluginName string) string {
	if pluginID > 0 {
		return fmt.Sprintf("plugin_%d", pluginID)
	}
	safeName := sanitizePluginWorkspaceBuiltinIdentifier(pluginName)
	if safeName == "" {
		return "plugin_anonymous"
	}
	return "plugin_" + safeName
}

func sanitizePluginWorkspaceBuiltinIdentifier(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, ch := range trimmed {
		isAlphaNum := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
		if isAlphaNum || ch == '-' || ch == '_' {
			builder.WriteRune(ch)
			continue
		}
		builder.WriteByte('_')
	}
	return strings.Trim(builder.String(), "_")
}

func detectPluginWorkspaceBuiltinModuleRoot(entryScriptPath string) string {
	entryDir := filepath.Clean(filepath.Dir(entryScriptPath))
	current := entryDir
	for i := 0; i < 16; i++ {
		if hasPluginWorkspaceBuiltinManifest(current) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return entryDir
}

func hasPluginWorkspaceBuiltinManifest(dir string) bool {
	manifestNames := []string{"manifest.json", "plugin.json", "plugin-manifest.json"}
	for _, name := range manifestNames {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func resolvePluginWorkspaceBuiltinPath(
	roots pluginWorkspaceBuiltinRoots,
	path string,
	allowEmpty bool,
) (pluginWorkspaceBuiltinResolvedPath, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		if allowEmpty {
			trimmed = "."
		} else {
			return pluginWorkspaceBuiltinResolvedPath{}, fmt.Errorf("path is required")
		}
	}
	normalized := filepath.Clean(filepath.FromSlash(trimmed))
	if filepath.IsAbs(normalized) {
		return pluginWorkspaceBuiltinResolvedPath{}, fmt.Errorf("absolute path is not allowed: %s", filepath.ToSlash(path))
	}
	if normalized == ".." || strings.HasPrefix(normalized, ".."+string(os.PathSeparator)) {
		return pluginWorkspaceBuiltinResolvedPath{}, fmt.Errorf("path outside plugin root: %s", filepath.ToSlash(path))
	}
	dataCandidate := filepath.Clean(filepath.Join(roots.DataRoot, normalized))
	codeCandidate := filepath.Clean(filepath.Join(roots.CodeRoot, normalized))
	resolvedData, err := resolvePluginWorkspaceBuiltinPathWithinRoot(roots.DataRoot, dataCandidate)
	if err != nil {
		return pluginWorkspaceBuiltinResolvedPath{}, err
	}
	resolvedCode, err := resolvePluginWorkspaceBuiltinPathWithinRoot(roots.CodeRoot, codeCandidate)
	if err != nil {
		return pluginWorkspaceBuiltinResolvedPath{}, err
	}
	relPath := normalized
	if relPath == "" {
		relPath = "."
	}
	return pluginWorkspaceBuiltinResolvedPath{
		RelPath:     relPath,
		DisplayPath: pluginWorkspaceBuiltinDisplayPath(relPath),
		DataRoot:    roots.DataRoot,
		CodeRoot:    roots.CodeRoot,
		DataPath:    resolvedData,
		CodePath:    resolvedCode,
	}, nil
}

func resolvePluginWorkspaceBuiltinPathWithinRoot(root string, path string) (string, error) {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if !isPathWithinRoot(root, path) {
		return "", fmt.Errorf("path outside plugin root: %s", filepath.ToSlash(path))
	}
	rootInfo, err := os.Stat(root)
	switch {
	case err == nil && rootInfo.IsDir():
		resolved, resolveErr := resolvePluginWorkspaceBuiltinSymlinkSafePath(root, path)
		if resolveErr != nil {
			return "", resolveErr
		}
		if !isPathWithinRoot(root, resolved) {
			return "", fmt.Errorf("resolved path outside plugin root")
		}
		return resolved, nil
	case err == nil && !rootInfo.IsDir():
		return "", fmt.Errorf("plugin root is not a directory: %s", filepath.ToSlash(root))
	case os.IsNotExist(err):
		return path, nil
	case err != nil:
		return "", err
	default:
		return path, nil
	}
}

func resolvePluginWorkspaceBuiltinSymlinkSafePath(root string, path string) (string, error) {
	evaluated, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Clean(evaluated), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	ancestor, ancestorErr := evalPluginWorkspaceBuiltinExistingPath(path)
	if ancestorErr != nil {
		return "", ancestorErr
	}
	if !isPathWithinRoot(root, ancestor) {
		return "", fmt.Errorf("path escapes plugin root via symlink")
	}
	return filepath.Clean(path), nil
}

func evalPluginWorkspaceBuiltinExistingPath(path string) (string, error) {
	current := filepath.Clean(path)
	for {
		evaluated, err := filepath.EvalSymlinks(current)
		if err == nil {
			return filepath.Clean(evaluated), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(path), nil
		}
		current = parent
	}
}

func lookupPluginWorkspaceBuiltinEntry(resolved pluginWorkspaceBuiltinResolvedPath) (*pluginWorkspaceBuiltinEntry, error) {
	if info, err := os.Stat(resolved.DataPath); err == nil {
		return &pluginWorkspaceBuiltinEntry{Path: resolved.DataPath, Info: info, Layer: "data"}, nil
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if info, err := os.Stat(resolved.CodePath); err == nil {
		return &pluginWorkspaceBuiltinEntry{Path: resolved.CodePath, Info: info, Layer: "code"}, nil
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return nil, nil
}

func listPluginWorkspaceBuiltinDirectory(
	ctx context.Context,
	resolved pluginWorkspaceBuiltinResolvedPath,
) ([]pluginWorkspaceBuiltinListItem, error) {
	if err := pluginWorkspaceBuiltinCheckContext(ctx); err != nil {
		return nil, err
	}
	itemsByName := make(map[string]pluginWorkspaceBuiltinListItem)
	if err := mergePluginWorkspaceBuiltinDirectoryEntries(ctx, itemsByName, resolved, resolved.DataRoot, resolved.DataPath, "data"); err != nil {
		return nil, err
	}
	if err := mergePluginWorkspaceBuiltinDirectoryEntries(ctx, itemsByName, resolved, resolved.CodeRoot, resolved.CodePath, "code"); err != nil {
		return nil, err
	}
	items := make([]pluginWorkspaceBuiltinListItem, 0, len(itemsByName))
	for _, item := range itemsByName {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		left := strings.ToLower(items[i].Path)
		right := strings.ToLower(items[j].Path)
		if left == right {
			return items[i].Layer < items[j].Layer
		}
		return left < right
	})
	return items, nil
}

func mergePluginWorkspaceBuiltinDirectoryEntries(
	ctx context.Context,
	itemsByName map[string]pluginWorkspaceBuiltinListItem,
	resolved pluginWorkspaceBuiltinResolvedPath,
	rootPath string,
	dirPath string,
	layer string,
) error {
	info, err := os.Stat(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := pluginWorkspaceBuiltinCheckContext(ctx); err != nil {
			return err
		}
		name := entry.Name()
		if _, exists := itemsByName[strings.ToLower(name)]; exists {
			continue
		}
		safePath, resolveErr := resolvePluginWorkspaceBuiltinPathWithinRoot(rootPath, filepath.Join(dirPath, name))
		if resolveErr != nil {
			continue
		}
		childInfo, infoErr := os.Stat(safePath)
		if infoErr != nil {
			continue
		}
		childRel := name
		if resolved.RelPath != "." {
			childRel = filepath.Join(resolved.RelPath, name)
		}
		itemsByName[strings.ToLower(name)] = newPluginWorkspaceBuiltinListItem(
			pluginWorkspaceBuiltinDisplayPath(childRel),
			layer,
			childInfo,
		)
	}
	return nil
}

func collectPluginWorkspaceBuiltinFiles(
	ctx context.Context,
	resolved pluginWorkspaceBuiltinResolvedPath,
) ([]pluginWorkspaceBuiltinEntry, bool, error) {
	filesByPath := make(map[string]pluginWorkspaceBuiltinEntry)
	if err := walkPluginWorkspaceBuiltinFiles(ctx, filesByPath, resolved.DataRoot, resolved.DataPath, resolved.RelPath, "data", pluginWorkspaceBuiltinGrepMaxFiles); err != nil {
		if err != io.EOF {
			return nil, false, err
		}
	}
	truncated := len(filesByPath) >= pluginWorkspaceBuiltinGrepMaxFiles
	if !truncated {
		if err := walkPluginWorkspaceBuiltinFiles(ctx, filesByPath, resolved.CodeRoot, resolved.CodePath, resolved.RelPath, "code", pluginWorkspaceBuiltinGrepMaxFiles); err != nil {
			if err != io.EOF {
				return nil, false, err
			}
		}
		truncated = len(filesByPath) >= pluginWorkspaceBuiltinGrepMaxFiles
	}
	files := make([]pluginWorkspaceBuiltinEntry, 0, len(filesByPath))
	keys := make([]string, 0, len(filesByPath))
	for key := range filesByPath {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		files = append(files, filesByPath[key])
	}
	return files, truncated, nil
}

func walkPluginWorkspaceBuiltinTreeMatches(
	ctx context.Context,
	itemsByPath map[string]pluginWorkspaceBuiltinListItem,
	rootPath string,
	basePath string,
	baseRel string,
	layer string,
	maxEntries int,
	matchFn func(path string, name string) bool,
) error {
	info, err := os.Stat(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	addEntry := func(displayPath string, info os.FileInfo) error {
		if !matchFn(displayPath, filepath.Base(filepath.FromSlash(displayPath))) {
			return nil
		}
		if maxEntries > 0 && len(itemsByPath) >= maxEntries {
			return io.EOF
		}
		key := strings.ToLower(displayPath)
		if _, exists := itemsByPath[key]; exists {
			return nil
		}
		itemsByPath[key] = newPluginWorkspaceBuiltinListItem(displayPath, layer, info)
		return nil
	}
	if !info.IsDir() {
		return addEntry(pluginWorkspaceBuiltinDisplayPath(baseRel), info)
	}
	return filepath.Walk(basePath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if err := pluginWorkspaceBuiltinCheckContext(ctx); err != nil {
			return err
		}
		if info == nil {
			return nil
		}
		if path == basePath {
			return nil
		}
		safePath, resolveErr := resolvePluginWorkspaceBuiltinPathWithinRoot(rootPath, path)
		if resolveErr != nil {
			return nil
		}
		safeInfo, statErr := os.Stat(safePath)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				return nil
			}
			return statErr
		}
		rel, relErr := filepath.Rel(basePath, path)
		if relErr != nil {
			return nil
		}
		targetRel := rel
		if baseRel != "." {
			targetRel = filepath.Join(baseRel, rel)
		}
		return addEntry(pluginWorkspaceBuiltinDisplayPath(targetRel), safeInfo)
	})
}

func walkPluginWorkspaceBuiltinFiles(
	ctx context.Context,
	filesByPath map[string]pluginWorkspaceBuiltinEntry,
	rootPath string,
	basePath string,
	baseRel string,
	layer string,
	maxFiles int,
) error {
	info, err := os.Stat(basePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		key := strings.ToLower(pluginWorkspaceBuiltinDisplayPath(baseRel))
		if _, exists := filesByPath[key]; !exists {
			filesByPath[key] = pluginWorkspaceBuiltinEntry{Path: basePath, Info: info, Layer: layer}
		}
		return nil
	}
	return filepath.Walk(basePath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if err := pluginWorkspaceBuiltinCheckContext(ctx); err != nil {
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		if maxFiles > 0 && len(filesByPath) >= maxFiles {
			return io.EOF
		}
		safePath, resolveErr := resolvePluginWorkspaceBuiltinPathWithinRoot(rootPath, path)
		if resolveErr != nil {
			return nil
		}
		safeInfo, statErr := os.Stat(safePath)
		if statErr != nil || safeInfo.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(basePath, path)
		if relErr != nil {
			return nil
		}
		targetRel := rel
		if baseRel != "." {
			targetRel = filepath.Join(baseRel, rel)
		}
		key := strings.ToLower(pluginWorkspaceBuiltinDisplayPath(targetRel))
		if _, exists := filesByPath[key]; exists {
			return nil
		}
		filesByPath[key] = pluginWorkspaceBuiltinEntry{Path: safePath, Info: safeInfo, Layer: layer}
		return nil
	})
}

func newPluginWorkspaceBuiltinListItem(path string, layer string, info os.FileInfo) pluginWorkspaceBuiltinListItem {
	item := pluginWorkspaceBuiltinListItem{
		Path:       path,
		Name:       filepath.Base(filepath.FromSlash(path)),
		Layer:      layer,
		Type:       pluginWorkspaceBuiltinEntryType(info),
		Size:       info.Size(),
		Mode:       info.Mode().String(),
		ModifiedAt: info.ModTime().UTC().Format(time.RFC3339),
	}
	if path == "." {
		item.Name = "."
	}
	return item
}

func pluginWorkspaceBuiltinEntryType(info os.FileInfo) string {
	if info == nil {
		return "file"
	}
	if info.IsDir() {
		return "dir"
	}
	return "file"
}

func pluginWorkspaceBuiltinDisplayPath(relPath string) string {
	trimmed := strings.TrimSpace(relPath)
	if trimmed == "" || trimmed == "." {
		return "."
	}
	return filepath.ToSlash(trimmed)
}

func readPluginWorkspaceBuiltinTextFile(ctx context.Context, path string, limitBytes int64) (string, bool, error) {
	if err := pluginWorkspaceBuiltinCheckContext(ctx); err != nil {
		return "", false, err
	}
	file, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer file.Close()

	head := make([]byte, 4096)
	n, readErr := file.Read(head)
	if readErr != nil && readErr != io.EOF {
		return "", false, readErr
	}
	head = head[:n]
	if isPluginWorkspaceBuiltinBinary(head) {
		return "", false, fmt.Errorf("file is not valid UTF-8 text: %s", filepath.ToSlash(path))
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", false, err
	}
	if limitBytes <= 0 {
		limitBytes = pluginWorkspaceBuiltinReadLimitBytes
	}
	data, err := io.ReadAll(io.LimitReader(file, limitBytes+1))
	if err != nil {
		return "", false, err
	}
	truncated := int64(len(data)) > limitBytes
	if truncated {
		data = data[:limitBytes]
	}
	if !utf8.Valid(data) {
		return "", false, fmt.Errorf("file is not valid UTF-8 text: %s", filepath.ToSlash(path))
	}
	return string(data), truncated, nil
}

func isPluginWorkspaceBuiltinBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return true
	}
	return !utf8.Valid(data)
}

func splitPluginWorkspaceBuiltinLines(content string) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.Split(normalized, "\n")
}

func pluginWorkspaceBuiltinRelFromResolved(
	resolved pluginWorkspaceBuiltinResolvedPath,
	path string,
	layer string,
) string {
	basePath := resolved.CodePath
	if layer == "data" {
		basePath = resolved.DataPath
	}
	entryInfo, err := os.Stat(basePath)
	if err == nil && entryInfo.IsDir() {
		rel, relErr := filepath.Rel(basePath, path)
		if relErr == nil {
			combined := rel
			if resolved.RelPath != "." {
				combined = filepath.Join(resolved.RelPath, rel)
			}
			return pluginWorkspaceBuiltinDisplayPath(combined)
		}
	}
	return resolved.DisplayPath
}

func pluginWorkspaceBuiltinCheckContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func formatPluginWorkspaceBuiltinList(items []pluginWorkspaceBuiltinListItem) string {
	if len(items) == 0 {
		return "(empty)"
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("%s | %s | %d | %s | %s", item.Type, item.Layer, item.Size, item.ModifiedAt, item.Path))
	}
	return strings.Join(lines, "\n")
}

func formatPluginWorkspaceBuiltinStat(item pluginWorkspaceBuiltinStatItem) string {
	return strings.Join([]string{
		"path: " + item.Path,
		"layer: " + item.Layer,
		"type: " + item.Type,
		fmt.Sprintf("size: %d", item.Size),
		"mode: " + item.Mode,
		"modified_at: " + item.ModifiedAt,
	}, "\n")
}

func formatPluginWorkspaceBuiltinGrep(matches []pluginWorkspaceBuiltinGrepMatch, scannedFiles int, truncated bool) string {
	lines := make([]string, 0, len(matches)+2)
	for _, match := range matches {
		lines = append(lines, fmt.Sprintf("%s:%d [%s] %s", match.Path, match.Line, match.Layer, match.Text))
	}
	if len(lines) == 0 {
		lines = append(lines, "(no matches)")
	}
	lines = append(lines, fmt.Sprintf("scanned_files=%d", scannedFiles))
	if truncated {
		lines = append(lines, "truncated=true")
	}
	return strings.Join(lines, "\n")
}

func formatPluginWorkspaceBuiltinLogTail(entries []PluginWorkspaceBufferEntry) string {
	if len(entries) == 0 {
		return "(empty)"
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		parts := make([]string, 0, 5)
		if !entry.Timestamp.IsZero() {
			parts = append(parts, entry.Timestamp.UTC().Format(time.RFC3339))
		}
		if level := strings.TrimSpace(entry.Level); level != "" {
			parts = append(parts, "["+strings.ToUpper(level)+"]")
		}
		if channel := strings.TrimSpace(entry.Channel); channel != "" {
			parts = append(parts, "["+channel+"]")
		}
		if source := strings.TrimSpace(entry.Source); source != "" {
			parts = append(parts, "["+source+"]")
		}
		head := strings.Join(parts, " ")
		if head == "" {
			lines = append(lines, strings.TrimSpace(entry.Message))
			continue
		}
		lines = append(lines, strings.TrimSpace(head+" "+strings.TrimSpace(entry.Message)))
	}
	return strings.Join(lines, "\n")
}
