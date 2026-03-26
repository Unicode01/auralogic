package jsworker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/dop251/goja"
)

const runtimeConsoleResultHistoryDepth = 5

const (
	runtimeConsoleHelperHelpName         = "__auralogicConsoleHelp"
	runtimeConsoleHelperKeysName         = "__auralogicConsoleKeys"
	runtimeConsoleHelperRuntimeStateName = "__auralogicConsoleRuntimeState"
	runtimeConsoleHelperCommandsName     = "__auralogicConsoleCommands"
	runtimeConsoleHelperPermissionsName  = "__auralogicConsolePermissions"
	runtimeConsoleHelperWorkspaceName    = "__auralogicConsoleWorkspaceState"
	runtimeConsoleHelperInspectName      = "__auralogicConsoleInspect"
	runtimeConsoleHelperClearOutputName  = "__auralogicConsoleClearOutput"
)

func (r *persistentPluginRuntime) installGlobals() error {
	if r == nil || r.vm == nil {
		return fmt.Errorf("persistent runtime is unavailable")
	}

	installBrowserCompatibilityPolyfills(r.vm)
	r.installAsyncRuntimeGlobals()
	r.installRuntimeConsoleResultGlobals()
	r.installRuntimeConsoleHelperAliases()

	console := r.vm.NewObject()
	registerConsoleMethod := func(name string, level string) {
		_ = console.Set(name, func(call goja.FunctionCall) goja.Value {
			if workspaceState := r.currentWorkspaceState(); workspaceState != nil && workspaceState.enabled {
				message, metadata := buildRuntimeConsoleCallOutput(r.vm, call.Arguments)
				workspaceState.write("console", level, message, "console."+name, metadata)
			}
			return goja.Undefined()
		})
	}
	registerConsoleMethod("log", "info")
	registerConsoleMethod("info", "info")
	registerConsoleMethod("warn", "warn")
	registerConsoleMethod("error", "error")
	registerConsoleMethod("debug", "debug")
	r.vm.Set("console", console)

	r.sandboxObj = r.vm.NewObject()
	r.pluginObj = r.vm.NewObject()
	r.workspaceObj = r.vm.NewObject()
	r.storageObj = r.vm.NewObject()
	r.secretObj = r.vm.NewObject()
	r.webhookObj = r.vm.NewObject()
	r.httpObj = r.vm.NewObject()
	r.hostObj = r.vm.NewObject()
	r.fsObj = r.vm.NewObject()

	r.installWorkspaceGlobals()
	r.installStorageGlobals()
	r.installSecretGlobals()
	r.installWebhookGlobals()
	r.installHTTPGlobals()
	r.installHostGlobals()
	r.installFSGlobals()
	r.installWorkerRuntimeGlobals()

	_ = r.pluginObj.Set("workspace", r.workspaceObj)
	_ = r.pluginObj.Set("storage", r.storageObj)
	_ = r.pluginObj.Set("secret", r.secretObj)
	_ = r.pluginObj.Set("webhook", r.webhookObj)
	_ = r.pluginObj.Set("http", r.httpObj)
	_ = r.pluginObj.Set("host", r.hostObj)
	_ = r.pluginObj.Set("fs", r.fsObj)

	r.vm.Set("sandbox", r.sandboxObj)
	r.vm.Set("Plugin", r.pluginObj)
	return nil
}

func (r *persistentPluginRuntime) installRuntimeConsoleResultGlobals() {
	if r == nil || r.vm == nil {
		return
	}
	global := r.vm.GlobalObject()
	if global == nil {
		return
	}
	for _, name := range runtimeConsoleResultGlobalNames() {
		r.defineRuntimeConsoleGlobalValue(global, name, goja.Undefined(), false)
	}
}

func (r *persistentPluginRuntime) defineRuntimeConsoleGlobalValue(
	global *goja.Object,
	name string,
	value goja.Value,
	enumerable bool,
) {
	if global == nil || strings.TrimSpace(name) == "" {
		return
	}
	enumerableFlag := goja.FLAG_FALSE
	if enumerable {
		enumerableFlag = goja.FLAG_TRUE
	}
	_ = global.DefineDataProperty(
		name,
		value,
		goja.FLAG_TRUE,
		goja.FLAG_TRUE,
		enumerableFlag,
	)
}

func (r *persistentPluginRuntime) rememberRuntimeConsoleResult(value goja.Value) {
	if r == nil || r.vm == nil {
		return
	}
	global := r.vm.GlobalObject()
	if global == nil {
		return
	}
	for index := runtimeConsoleResultHistoryDepth; index >= 2; index-- {
		previousName := fmt.Sprintf("$%d", index-1)
		currentName := fmt.Sprintf("$%d", index)
		r.defineRuntimeConsoleGlobalValue(global, currentName, global.Get(previousName), false)
	}
	r.defineRuntimeConsoleGlobalValue(global, "$1", value, false)
	r.defineRuntimeConsoleGlobalValue(global, "$_", value, false)
}

func (r *persistentPluginRuntime) installRuntimeConsoleHelperAliases() {
	if r == nil || r.vm == nil {
		return
	}
	global := r.vm.GlobalObject()
	if global == nil {
		return
	}
	r.defineRuntimeConsoleGlobalValue(
		global,
		runtimeConsoleHelperHelpName,
		r.runtimeBridgeValue(func(call goja.FunctionCall) goja.Value {
			topic := ""
			if argument := argumentAt(call, 0); argument != nil && !goja.IsUndefined(argument) && !goja.IsNull(argument) {
				topic = argument.String()
			}
			return r.runtimeBridgeValue(r.runtimeConsoleHelpPayload(topic))
		}),
		false,
	)
	r.defineRuntimeConsoleGlobalValue(
		global,
		runtimeConsoleHelperKeysName,
		r.runtimeBridgeValue(func(call goja.FunctionCall) goja.Value {
			target := argumentAt(call, 0)
			object := global
			if target != nil && !goja.IsUndefined(target) && !goja.IsNull(target) {
				object = target.ToObject(r.vm)
			}
			if object == nil {
				return r.runtimeBridgeValue([]string{})
			}
			keys := append([]string(nil), object.Keys()...)
			sort.Strings(keys)
			return r.runtimeBridgeValue(keys)
		}),
		false,
	)
	r.defineRuntimeConsoleGlobalValue(
		global,
		runtimeConsoleHelperRuntimeStateName,
		r.runtimeBridgeValue(func(call goja.FunctionCall) goja.Value {
			return r.runtimeBridgeValue(r.runtimeConsoleStatePayload())
		}),
		false,
	)
	r.defineRuntimeConsoleGlobalValue(
		global,
		runtimeConsoleHelperCommandsName,
		r.runtimeBridgeValue(func(call goja.FunctionCall) goja.Value {
			return r.runtimeBridgeValue(r.runtimeConsoleCommandsPayload())
		}),
		false,
	)
	r.defineRuntimeConsoleGlobalValue(
		global,
		runtimeConsoleHelperPermissionsName,
		r.runtimeBridgeValue(func(call goja.FunctionCall) goja.Value {
			return r.runtimeBridgeValue(r.runtimeConsolePermissionsPayload())
		}),
		false,
	)
	r.defineRuntimeConsoleGlobalValue(
		global,
		runtimeConsoleHelperWorkspaceName,
		r.runtimeBridgeValue(func(call goja.FunctionCall) goja.Value {
			return r.runtimeBridgeValue(r.runtimeConsoleWorkspacePayload(workspaceLimitFromGojaArguments(call, 0)))
		}),
		false,
	)
	r.defineRuntimeConsoleGlobalValue(
		global,
		runtimeConsoleHelperInspectName,
		r.runtimeBridgeValue(func(call goja.FunctionCall) goja.Value {
			return r.runtimeBridgeValue(r.runtimeConsoleInspectPayload(call))
		}),
		false,
	)
	r.defineRuntimeConsoleGlobalValue(
		global,
		runtimeConsoleHelperClearOutputName,
		r.runtimeBridgeValue(func(call goja.FunctionCall) goja.Value {
			return r.runtimeBridgeValue(r.runtimeConsoleClearOutputPayload())
		}),
		false,
	)
}

func runtimeConsoleInspectDepthFromGojaArguments(call goja.FunctionCall, index int) int {
	if len(call.Arguments) <= index {
		return defaultRuntimeConsoleInspectDepth
	}
	value := argumentAt(call, index)
	if value == nil || goja.IsNull(value) || goja.IsUndefined(value) {
		return defaultRuntimeConsoleInspectDepth
	}
	exported := value.Export()
	switch typed := exported.(type) {
	case int:
		return normalizeRuntimeConsoleInspectDepth(typed)
	case int64:
		return normalizeRuntimeConsoleInspectDepth(int(typed))
	case float64:
		return normalizeRuntimeConsoleInspectDepth(int(typed))
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return normalizeRuntimeConsoleInspectDepth(parsed)
		}
	}
	return defaultRuntimeConsoleInspectDepth
}

func (r *persistentPluginRuntime) runtimeConsoleCommandsPayload() map[string]interface{} {
	r.refreshWorkspaceCommandAliasesLocked()
	callablePaths := r.snapshotCallablePathsLocked()
	workspaceCommands := r.snapshotWorkspaceCommandHelpEntriesLocked()
	workspaceCommandNames := runtimeWorkspaceCommandHelpNames(workspaceCommands)
	return map[string]interface{}{
		"helpers": []string{
			"help(topic?)",
			"keys(value?)",
			"runtimeState()",
			"commands()",
			"permissions()",
			"workspaceState(limit?)",
			"inspect(value?, depth?)",
			"clearOutput()",
		},
		"meta_commands": []string{
			":inspect [--depth N] <expression>",
		},
		"available_topics": []string{
			"Plugin",
			"Plugin.workspace",
			"Plugin.storage",
			"Plugin.secret",
			"Plugin.webhook",
			"Plugin.http",
			"Plugin.host",
			"Plugin.fs",
			"Worker",
			"sandbox",
			"results",
			"module.exports",
			"URLSearchParams",
			"TextEncoder",
			"TextDecoder",
			"atob",
			"btoa",
			"structuredClone",
			"queueMicrotask",
			"setTimeout",
			"clearTimeout",
			"commands",
			"permissions",
			"workspaceState",
			"inspect",
			"clearOutput",
		},
		"workspace_aliases": workspaceCommandNames,
		"plugin_exports":    callablePaths,
	}
}

func (r *persistentPluginRuntime) runtimeConsolePermissionsPayload() map[string]interface{} {
	invocation := r.currentInvocation()
	if invocation == nil {
		return map[string]interface{}{
			"current_action":            "",
			"requested_permissions":     []string{},
			"granted_permissions":       []string{},
			"missing_permissions":       []string{},
			"allow_network":             false,
			"allow_file_system":         false,
			"allow_execute_api":         false,
			"allow_hook_execute":        false,
			"allow_hook_block":          false,
			"allow_payload_patch":       false,
			"allow_frontend_extensions": false,
			"storage_access_mode":       storageAccessNone,
		}
	}
	requested := append([]string{}, invocation.sandboxCfg.RequestedPermissions...)
	granted := append([]string{}, invocation.sandboxCfg.GrantedPermissions...)
	grantedSet := make(map[string]struct{}, len(granted))
	for _, item := range granted {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		grantedSet[normalized] = struct{}{}
	}
	missing := make([]string, 0, len(requested))
	for _, item := range requested {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		if _, exists := grantedSet[normalized]; exists {
			continue
		}
		missing = append(missing, normalized)
	}
	sort.Strings(requested)
	sort.Strings(granted)
	sort.Strings(missing)
	return map[string]interface{}{
		"current_action":            invocation.sandboxCfg.CurrentAction,
		"requested_permissions":     requested,
		"granted_permissions":       granted,
		"missing_permissions":       missing,
		"allow_network":             invocation.effectiveOpts.allowNetwork,
		"allow_file_system":         invocation.effectiveOpts.allowFS,
		"allow_execute_api":         invocation.sandboxCfg.AllowExecuteAPI,
		"allow_hook_execute":        invocation.sandboxCfg.AllowHookExecute,
		"allow_hook_block":          invocation.sandboxCfg.AllowHookBlock,
		"allow_payload_patch":       invocation.sandboxCfg.AllowPayloadPatch,
		"allow_frontend_extensions": invocation.sandboxCfg.AllowFrontendExtensions,
		"declared_storage_access":   invocation.sandboxCfg.DeclaredStorageAccess,
		"effective_storage_access":  invocation.storageState.accessMode(),
		"default_timeout_ms":        invocation.effectiveOpts.timeoutMs,
		"max_concurrency":           invocation.effectiveOpts.maxConcurrency,
		"max_memory_mb":             invocation.effectiveOpts.maxMemoryMB,
	}
}

func (r *persistentPluginRuntime) runtimeConsoleWorkspacePayload(limit int) map[string]interface{} {
	workspaceState := r.currentWorkspaceState()
	if workspaceState == nil {
		return map[string]interface{}{
			"enabled":             false,
			"max_entries":         0,
			"entry_count":         0,
			"entries":             []map[string]interface{}{},
			"command_name":        "",
			"command_entry":       "",
			"command_id":          "",
			"command_raw":         "",
			"command_argv":        []string{},
			"pending_input_count": 0,
		}
	}
	payload := workspaceState.snapshot(limit)
	payload["command_name"] = strings.TrimSpace(workspaceState.commandName)
	payload["command_entry"] = strings.TrimSpace(workspaceState.commandEntry)
	payload["command_id"] = strings.TrimSpace(workspaceState.commandID)
	payload["command_raw"] = strings.TrimSpace(workspaceState.commandRaw)
	payload["command_argv"] = append([]string{}, workspaceState.commandArgv...)
	payload["pending_input_count"] = len(workspaceState.inputQueue)
	if len(workspaceState.inputQueue) > 0 {
		payload["pending_inputs"] = append([]string{}, workspaceState.inputQueue...)
	}
	return payload
}

func (r *persistentPluginRuntime) runtimeConsoleInspectPayload(call goja.FunctionCall) map[string]interface{} {
	target := argumentAt(call, 0)
	if target == nil || goja.IsUndefined(target) || goja.IsNull(target) {
		if r != nil && r.vm != nil {
			target = r.vm.GlobalObject()
		}
	}
	depth := runtimeConsoleInspectDepthFromGojaArguments(call, 1)
	payload := runtimeConsolePreviewToMap(buildRuntimeConsolePreview(r.vm, target, depth))
	payload["depth"] = depth
	return payload
}

func (r *persistentPluginRuntime) runtimeConsoleClearOutputPayload() map[string]interface{} {
	workspaceState := r.currentWorkspaceState()
	if workspaceState == nil {
		return map[string]interface{}{
			"cleared":            false,
			"workspace_enabled":  false,
			"entry_count_before": 0,
		}
	}
	entryCountBefore := len(workspaceState.history)
	return map[string]interface{}{
		"cleared":            workspaceState.clear(),
		"workspace_enabled":  workspaceState.enabled,
		"entry_count_before": entryCountBefore,
	}
}

func (r *persistentPluginRuntime) runtimeBridgeValue(value interface{}) goja.Value {
	if r == nil || r.vm == nil {
		return goja.Undefined()
	}
	return runtimeBridgeValue(r.vm, value)
}

func runtimeBridgeValue(vm *goja.Runtime, value interface{}) goja.Value {
	if vm == nil {
		return goja.Undefined()
	}
	switch typed := value.(type) {
	case nil:
		return goja.Null()
	case goja.Value:
		return typed
	}

	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return goja.Null()
	}

	switch reflected.Kind() {
	case reflect.Interface, reflect.Pointer:
		if reflected.IsNil() {
			return goja.Null()
		}
		return runtimeBridgeValue(vm, reflected.Elem().Interface())
	case reflect.Map:
		if reflected.IsNil() {
			return goja.Null()
		}
		if reflected.Type().Key().Kind() != reflect.String {
			return vm.ToValue(value)
		}
		object := vm.NewObject()
		keys := reflected.MapKeys()
		names := make([]string, 0, len(keys))
		values := make(map[string]reflect.Value, len(keys))
		for _, key := range keys {
			name := key.String()
			names = append(names, name)
			values[name] = reflected.MapIndex(key)
		}
		sort.Strings(names)
		for _, name := range names {
			_ = object.Set(name, runtimeBridgeValue(vm, values[name].Interface()))
		}
		return object
	case reflect.Slice, reflect.Array:
		if reflected.Kind() == reflect.Slice && reflected.IsNil() {
			return goja.Null()
		}
		items := make([]interface{}, 0, reflected.Len())
		for index := 0; index < reflected.Len(); index++ {
			items = append(items, runtimeBridgeValue(vm, reflected.Index(index).Interface()))
		}
		return vm.NewArray(items...)
	default:
		return vm.ToValue(value)
	}
}

func runtimeConsoleResultGlobalNames() []string {
	names := make([]string, 0, runtimeConsoleResultHistoryDepth+1)
	names = append(names, "$_")
	for index := 1; index <= runtimeConsoleResultHistoryDepth; index++ {
		names = append(names, fmt.Sprintf("$%d", index))
	}
	return names
}

type runtimeWorkspaceCommandHelpEntry struct {
	Name        string   `json:"name"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Entry       string   `json:"entry"`
	Alias       string   `json:"alias,omitempty"`
	Interactive bool     `json:"interactive"`
	Permissions []string `json:"permissions,omitempty"`
}

func (r *persistentPluginRuntime) collectWorkspaceCommandHelpEntriesLocked() []runtimeWorkspaceCommandHelpEntry {
	if r == nil || r.vm == nil || r.entryModule == nil {
		return nil
	}
	moduleExports, err := runtimeConsoleGetObjectProperty(r.entryModule, "exports")
	if err != nil || moduleExports == nil || goja.IsUndefined(moduleExports) || goja.IsNull(moduleExports) {
		return nil
	}
	moduleExportsObject := moduleExports.ToObject(r.vm)
	if moduleExportsObject == nil {
		return nil
	}
	workspaceValue, err := runtimeConsoleGetObjectProperty(moduleExportsObject, "workspace")
	if err != nil || workspaceValue == nil || goja.IsUndefined(workspaceValue) || goja.IsNull(workspaceValue) {
		return nil
	}
	workspaceObject := workspaceValue.ToObject(r.vm)
	if workspaceObject == nil {
		return nil
	}

	keys := runtimeCompletionObjectKeys(workspaceObject, false)
	if len(keys) == 0 {
		return nil
	}

	out := make([]runtimeWorkspaceCommandHelpEntry, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if key == "" || strings.HasPrefix(key, "__auralogic") {
			continue
		}
		rawDefinition, definitionErr := runtimeConsoleGetObjectProperty(workspaceObject, key)
		if definitionErr != nil || rawDefinition == nil || goja.IsUndefined(rawDefinition) || goja.IsNull(rawDefinition) {
			continue
		}

		entry := strings.TrimSpace(key)
		command := runtimeWorkspaceCommandHelpEntry{
			Name:        strings.ReplaceAll(entry, ".", "/"),
			Entry:       entry,
			Interactive: false,
		}
		if _, ok := goja.AssertFunction(rawDefinition); ok {
			if _, exists := seen[strings.ToLower(command.Name)]; exists {
				continue
			}
			seen[strings.ToLower(command.Name)] = struct{}{}
			out = append(out, command)
			continue
		}

		definitionObject := rawDefinition.ToObject(r.vm)
		if definitionObject == nil {
			continue
		}
		handlerValue, handlerErr := runtimeConsoleGetObjectProperty(definitionObject, "handler")
		if handlerErr != nil || handlerValue == nil || goja.IsUndefined(handlerValue) || goja.IsNull(handlerValue) {
			continue
		}
		if _, ok := goja.AssertFunction(handlerValue); !ok {
			continue
		}

		if rawEntry, entryErr := runtimeConsoleGetObjectProperty(definitionObject, "entry"); entryErr == nil && rawEntry != nil && !goja.IsUndefined(rawEntry) && !goja.IsNull(rawEntry) {
			if normalized := strings.TrimSpace(rawEntry.String()); normalized != "" {
				command.Entry = normalized
			}
		}
		if rawName, nameErr := runtimeConsoleGetObjectProperty(definitionObject, "name"); nameErr == nil && rawName != nil && !goja.IsUndefined(rawName) && !goja.IsNull(rawName) {
			if normalized := strings.TrimSpace(rawName.String()); normalized != "" {
				command.Name = normalized
			}
		}
		if rawTitle, titleErr := runtimeConsoleGetObjectProperty(definitionObject, "title"); titleErr == nil && rawTitle != nil && !goja.IsUndefined(rawTitle) && !goja.IsNull(rawTitle) {
			command.Title = strings.TrimSpace(rawTitle.String())
		}
		if rawDescription, descriptionErr := runtimeConsoleGetObjectProperty(definitionObject, "description"); descriptionErr == nil && rawDescription != nil && !goja.IsUndefined(rawDescription) && !goja.IsNull(rawDescription) {
			command.Description = strings.TrimSpace(rawDescription.String())
		}
		if rawInteractive, interactiveErr := runtimeConsoleGetObjectProperty(definitionObject, "interactive"); interactiveErr == nil && rawInteractive != nil && !goja.IsUndefined(rawInteractive) && !goja.IsNull(rawInteractive) {
			if exported, ok := rawInteractive.Export().(bool); ok {
				command.Interactive = exported
			}
		}
		if rawPermissions, permissionsErr := runtimeConsoleGetObjectProperty(definitionObject, "permissions"); permissionsErr == nil && rawPermissions != nil && !goja.IsUndefined(rawPermissions) && !goja.IsNull(rawPermissions) {
			if exported, ok := rawPermissions.Export().([]interface{}); ok {
				permissions := make([]string, 0, len(exported))
				for _, item := range exported {
					normalized := strings.TrimSpace(fmt.Sprintf("%v", item))
					if normalized == "" {
						continue
					}
					permissions = append(permissions, normalized)
				}
				command.Permissions = permissions
			}
		}

		normalizedName := strings.ToLower(strings.TrimSpace(command.Name))
		if normalizedName == "" {
			command.Name = strings.ReplaceAll(command.Entry, ".", "/")
			normalizedName = strings.ToLower(command.Name)
		}
		if _, exists := seen[normalizedName]; exists {
			continue
		}
		seen[normalizedName] = struct{}{}
		out = append(out, command)
	}

	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func (r *persistentPluginRuntime) snapshotWorkspaceCommandHelpEntriesLocked() []runtimeWorkspaceCommandHelpEntry {
	if r == nil || r.vm == nil {
		return nil
	}
	r.refreshWorkspaceCommandAliasesLocked()
	if len(r.workspaceAliasOrder) == 0 || len(r.workspaceAliasCatalog) == 0 {
		return nil
	}
	out := make([]runtimeWorkspaceCommandHelpEntry, 0, len(r.workspaceAliasOrder))
	for _, alias := range r.workspaceAliasOrder {
		entry, ok := r.workspaceAliasCatalog[strings.ToLower(strings.TrimSpace(alias))]
		if !ok {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func (r *persistentPluginRuntime) refreshWorkspaceCommandAliasesLocked() {
	if r == nil || r.vm == nil {
		return
	}
	global := r.vm.GlobalObject()
	if global == nil {
		return
	}

	for _, alias := range r.workspaceAliasOrder {
		normalized := strings.TrimSpace(alias)
		if normalized == "" {
			continue
		}
		_ = global.Delete(normalized)
	}
	r.workspaceAliasCatalog = nil
	r.workspaceAliasOrder = nil

	entries := r.collectWorkspaceCommandHelpEntriesLocked()
	if len(entries) == 0 {
		return
	}

	usedAliases := make(map[string]struct{}, 64)
	for _, key := range runtimeCompletionObjectKeys(global, true) {
		normalized := strings.ToLower(strings.TrimSpace(key))
		if normalized == "" {
			continue
		}
		usedAliases[normalized] = struct{}{}
	}

	catalog := make(map[string]runtimeWorkspaceCommandHelpEntry, len(entries))
	order := make([]string, 0, len(entries))
	for _, entry := range entries {
		alias := generateRuntimeWorkspaceCommandAlias(entry, usedAliases)
		if alias == "" {
			continue
		}
		entry.Alias = alias
		normalizedAlias := strings.ToLower(alias)
		usedAliases[normalizedAlias] = struct{}{}
		catalog[normalizedAlias] = entry
		order = append(order, alias)

		aliasEntry := entry
		r.defineRuntimeConsoleGlobalValue(
			global,
			alias,
			r.vm.ToValue(func(call goja.FunctionCall) goja.Value {
				return r.invokeWorkspaceCommandAlias(aliasEntry, call)
			}),
			true,
		)
	}
	sort.Slice(order, func(i, j int) bool {
		return strings.ToLower(order[i]) < strings.ToLower(order[j])
	})
	r.workspaceAliasCatalog = catalog
	r.workspaceAliasOrder = order
}

func generateRuntimeWorkspaceCommandAlias(
	entry runtimeWorkspaceCommandHelpEntry,
	used map[string]struct{},
) string {
	candidates := make([]string, 0, 8)
	appendCandidate := func(value string) {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			return
		}
		candidates = append(candidates, normalized)
	}

	appendCandidate(runtimeWorkspaceIdentifierCandidate(entry.Entry))
	appendCandidate(runtimeWorkspaceIdentifierCandidate(entry.Name))
	appendCandidate(runtimeWorkspaceCamelAlias(entry.Entry))
	appendCandidate(runtimeWorkspaceCamelAlias(entry.Name))
	appendCandidate(runtimeWorkspaceCamelAlias(entry.Title))

	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		normalizedCandidate := strings.TrimSpace(candidate)
		if normalizedCandidate == "" {
			continue
		}
		lowerCandidate := strings.ToLower(normalizedCandidate)
		if _, exists := seen[lowerCandidate]; exists {
			continue
		}
		seen[lowerCandidate] = struct{}{}

		resolved := normalizedCandidate
		if runtimeWorkspaceAliasReserved(resolved) {
			resolved += "Command"
		}
		if _, exists := used[strings.ToLower(resolved)]; exists {
			if !strings.HasSuffix(strings.ToLower(resolved), "command") {
				resolved += "Command"
			}
		}
		if runtimeWorkspaceAliasReserved(resolved) {
			continue
		}
		if _, exists := used[strings.ToLower(resolved)]; exists {
			for index := 2; index <= 128; index++ {
				suffixed := fmt.Sprintf("%s%d", resolved, index)
				if runtimeWorkspaceAliasReserved(suffixed) {
					continue
				}
				if _, exists := used[strings.ToLower(suffixed)]; exists {
					continue
				}
				return suffixed
			}
			continue
		}
		return resolved
	}
	return ""
}

func runtimeWorkspaceIdentifierCandidate(source string) string {
	normalized := strings.TrimSpace(source)
	if !isValidRuntimeWorkspaceAlias(normalized) || runtimeWorkspaceAliasReserved(normalized) {
		return ""
	}
	return normalized
}

func runtimeWorkspaceCamelAlias(source string) string {
	words := splitRuntimeWorkspaceAliasWords(source)
	if len(words) == 0 {
		return ""
	}
	var builder strings.Builder
	for index, word := range words {
		if word == "" {
			continue
		}
		lowerWord := strings.ToLower(word)
		if index == 0 {
			builder.WriteString(lowerWord)
			continue
		}
		builder.WriteString(strings.ToUpper(lowerWord[:1]))
		if len(lowerWord) > 1 {
			builder.WriteString(lowerWord[1:])
		}
	}
	return builder.String()
}

func splitRuntimeWorkspaceAliasWords(source string) []string {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return nil
	}
	words := make([]string, 0, 8)
	var builder strings.Builder
	flush := func() {
		if builder.Len() == 0 {
			return
		}
		words = append(words, builder.String())
		builder.Reset()
	}
	for _, ch := range trimmed {
		isAlphaNum := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
		if isAlphaNum {
			builder.WriteRune(ch)
			continue
		}
		if ch == '_' || ch == '-' || ch == '.' || ch == '/' || ch == ' ' || ch == ':' {
			flush()
			continue
		}
		flush()
	}
	flush()
	return words
}

func isValidRuntimeWorkspaceAlias(candidate string) bool {
	trimmed := strings.TrimSpace(candidate)
	if trimmed == "" {
		return false
	}
	for index, ch := range trimmed {
		if index == 0 {
			if ch == '$' || ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') {
				continue
			}
			return false
		}
		if ch == '$' || ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		return false
	}
	return true
}

func runtimeWorkspaceAliasReserved(candidate string) bool {
	switch strings.ToLower(strings.TrimSpace(candidate)) {
	case "",
		"break",
		"case",
		"catch",
		"class",
		"const",
		"continue",
		"debugger",
		"default",
		"delete",
		"do",
		"else",
		"enum",
		"export",
		"extends",
		"false",
		"finally",
		"for",
		"function",
		"if",
		"import",
		"in",
		"instanceof",
		"new",
		"null",
		"return",
		"super",
		"switch",
		"this",
		"throw",
		"true",
		"try",
		"typeof",
		"var",
		"void",
		"while",
		"with",
		"yield",
		"let",
		"static",
		"implements",
		"interface",
		"package",
		"private",
		"protected",
		"public",
		"await":
		return true
	default:
		return isRuntimeConsoleBuiltinGlobalCallableName(candidate)
	}
}

func (r *persistentPluginRuntime) invokeWorkspaceCommandAlias(
	entry runtimeWorkspaceCommandHelpEntry,
	call goja.FunctionCall,
) goja.Value {
	handler, err := r.resolveWorkspaceRuntimeHandlerLocked(entry.Entry)
	if err != nil {
		r.throwJSError(err)
	}

	commandValue := r.runtimeBridgeValue(r.buildRuntimeWorkspaceCommandContext(entry, call.Arguments))
	contextValue := r.runtimeBridgeValue(buildRuntimeExecutionContextMap(r.currentInvocation()))
	configValue := r.runtimeBridgeValue(buildRuntimePluginConfigMap(r.currentInvocation()))

	sandboxValue := goja.Value(goja.Undefined())
	if r.sandboxObj != nil {
		sandboxValue = r.sandboxObj
	}
	workspaceValue := goja.Value(goja.Undefined())
	if r.workspaceObj != nil {
		workspaceValue = r.workspaceObj
	}

	result, callErr := callScriptFunction(
		r.vm,
		handler,
		goja.Undefined(),
		entry.Alias,
		[]interface{}{commandValue, contextValue, configValue, sandboxValue, workspaceValue},
	)
	if callErr != nil {
		r.throwJSError(callErr)
	}
	return result
}

func (r *persistentPluginRuntime) resolveWorkspaceRuntimeHandlerLocked(entry string) (goja.Callable, error) {
	if r == nil || r.vm == nil || r.entryModule == nil {
		return nil, fmt.Errorf("workspace runtime is unavailable")
	}
	moduleExports, err := runtimeConsoleGetObjectProperty(r.entryModule, "exports")
	if err != nil || moduleExports == nil || goja.IsUndefined(moduleExports) || goja.IsNull(moduleExports) {
		return nil, fmt.Errorf("workspace exports are unavailable")
	}
	moduleExportsObject := moduleExports.ToObject(r.vm)
	if moduleExportsObject == nil {
		return nil, fmt.Errorf("workspace exports are unavailable")
	}
	workspaceValue, err := runtimeConsoleGetObjectProperty(moduleExportsObject, "workspace")
	if err != nil || workspaceValue == nil || goja.IsUndefined(workspaceValue) || goja.IsNull(workspaceValue) {
		return nil, fmt.Errorf("workspace exports are unavailable")
	}
	workspaceObject := workspaceValue.ToObject(r.vm)
	if workspaceObject == nil {
		return nil, fmt.Errorf("workspace exports are unavailable")
	}

	normalizedEntry := strings.TrimSpace(entry)
	if normalizedEntry == "" {
		return nil, fmt.Errorf("workspace entry is required")
	}
	definition, err := runtimeConsoleGetObjectProperty(workspaceObject, normalizedEntry)
	if err != nil || definition == nil || goja.IsUndefined(definition) || goja.IsNull(definition) {
		return nil, fmt.Errorf("workspace function %s is unavailable", normalizedEntry)
	}
	if handler, ok := goja.AssertFunction(definition); ok && handler != nil {
		return handler, nil
	}
	definitionObject := definition.ToObject(r.vm)
	if definitionObject == nil {
		return nil, fmt.Errorf("workspace function %s is unavailable", normalizedEntry)
	}
	handlerValue, err := runtimeConsoleGetObjectProperty(definitionObject, "handler")
	if err != nil || handlerValue == nil || goja.IsUndefined(handlerValue) || goja.IsNull(handlerValue) {
		return nil, fmt.Errorf("workspace function %s is unavailable", normalizedEntry)
	}
	handler, ok := goja.AssertFunction(handlerValue)
	if !ok || handler == nil {
		return nil, fmt.Errorf("workspace function %s is unavailable", normalizedEntry)
	}
	return handler, nil
}

func (r *persistentPluginRuntime) buildRuntimeWorkspaceCommandContext(
	entry runtimeWorkspaceCommandHelpEntry,
	arguments []goja.Value,
) map[string]interface{} {
	argv, raw := parseRuntimeWorkspaceCommandArguments(arguments)
	name := strings.TrimSpace(entry.Name)
	if name == "" {
		name = strings.TrimSpace(entry.Entry)
	}
	if raw == "" {
		rawParts := make([]string, 0, len(argv)+1)
		if strings.TrimSpace(entry.Alias) != "" {
			rawParts = append(rawParts, strings.TrimSpace(entry.Alias))
		} else if name != "" {
			rawParts = append(rawParts, name)
		}
		rawParts = append(rawParts, argv...)
		raw = strings.TrimSpace(strings.Join(rawParts, " "))
	}
	payload := map[string]interface{}{
		"name":        name,
		"entry":       strings.TrimSpace(entry.Entry),
		"raw":         raw,
		"argv":        argv,
		"interactive": entry.Interactive,
	}
	if workspaceState := r.currentWorkspaceState(); workspaceState != nil {
		if commandID := strings.TrimSpace(workspaceState.commandID); commandID != "" {
			payload["command_id"] = commandID
		}
	}
	return payload
}

func parseRuntimeWorkspaceCommandArguments(arguments []goja.Value) ([]string, string) {
	if len(arguments) == 0 {
		return []string{}, ""
	}
	if len(arguments) == 1 {
		switch typed := exportGojaValue(arguments[0]).(type) {
		case []interface{}:
			return runtimeWorkspaceStringsFromUnknown(typed), ""
		case map[string]interface{}:
			argv := runtimeWorkspaceStringsFromUnknown(typed["argv"])
			if len(argv) == 0 {
				argv = runtimeWorkspaceStringsFromUnknown(typed["args"])
			}
			raw := strings.TrimSpace(runtimeWorkspaceStringFromUnknown(typed["raw"]))
			return argv, raw
		}
	}

	argv := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		normalized := strings.TrimSpace(runtimeWorkspaceStringFromUnknown(exportGojaValue(argument)))
		if normalized == "" {
			continue
		}
		argv = append(argv, normalized)
	}
	return argv, ""
}

func runtimeWorkspaceStringsFromUnknown(value interface{}) []string {
	switch typed := value.(type) {
	case nil:
		return []string{}
	case []string:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			normalized := strings.TrimSpace(item)
			if normalized == "" {
				continue
			}
			out = append(out, normalized)
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			normalized := strings.TrimSpace(runtimeWorkspaceStringFromUnknown(item))
			if normalized == "" {
				continue
			}
			out = append(out, normalized)
		}
		return out
	default:
		normalized := strings.TrimSpace(runtimeWorkspaceStringFromUnknown(value))
		if normalized == "" {
			return []string{}
		}
		return []string{normalized}
	}
}

func runtimeWorkspaceStringFromUnknown(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", value)
	}
}

func buildRuntimeExecutionContextMap(invocation *persistentPluginRuntimeInvocation) map[string]interface{} {
	out := map[string]interface{}{}
	if invocation == nil || invocation.executionCtx == nil {
		return out
	}
	if invocation.executionCtx.UserID > 0 {
		out["user_id"] = invocation.executionCtx.UserID
	}
	if invocation.executionCtx.OrderID > 0 {
		out["order_id"] = invocation.executionCtx.OrderID
	}
	if normalizedSessionID := strings.TrimSpace(invocation.executionCtx.SessionID); normalizedSessionID != "" {
		out["session_id"] = normalizedSessionID
	}
	if len(invocation.executionCtx.Metadata) > 0 {
		out["metadata"] = mergeStringMaps(invocation.executionCtx.Metadata, nil)
	}
	return out
}

func buildRuntimePluginConfigMap(invocation *persistentPluginRuntimeInvocation) map[string]interface{} {
	if invocation == nil || len(invocation.pluginConfig) == 0 {
		return map[string]interface{}{}
	}
	return clonePluginConfigMap(invocation.pluginConfig)
}

func runtimeWorkspaceCommandHelpNames(entries []runtimeWorkspaceCommandHelpEntry) []string {
	if len(entries) == 0 {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		normalized := strings.TrimSpace(entry.Alias)
		if normalized == "" {
			normalized = strings.TrimSpace(entry.Name)
		}
		if normalized == "" {
			normalized = strings.TrimSpace(entry.Entry)
		}
		if normalized == "" {
			continue
		}
		out = append(out, normalized)
	}
	return out
}

func runtimeWorkspaceCommandHelpPayloadEntries(entries []runtimeWorkspaceCommandHelpEntry) []map[string]interface{} {
	if len(entries) == 0 {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(entries))
	for _, entry := range entries {
		item := map[string]interface{}{
			"name":        entry.Name,
			"entry":       entry.Entry,
			"interactive": entry.Interactive,
		}
		if entry.Alias != "" {
			item["alias"] = entry.Alias
		}
		if entry.Title != "" {
			item["title"] = entry.Title
		}
		if entry.Description != "" {
			item["description"] = entry.Description
		}
		if len(entry.Permissions) > 0 {
			item["permissions"] = append([]string(nil), entry.Permissions...)
		}
		out = append(out, item)
	}
	return out
}

func runtimeConsoleFilterPathsWithPrefix(paths []string, prefix string) []string {
	normalizedPrefix := strings.ToLower(strings.TrimSpace(prefix))
	if normalizedPrefix == "" || len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, item := range paths {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(normalized), normalizedPrefix) {
			out = append(out, normalized)
		}
	}
	return out
}

func runtimeConsoleFindCallablePath(paths []string, topic string) string {
	normalizedTopic := strings.ToLower(strings.TrimSpace(topic))
	if normalizedTopic == "" {
		return ""
	}
	for _, item := range paths {
		if strings.ToLower(strings.TrimSpace(item)) == normalizedTopic {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func runtimeConsoleFindWorkspaceCommand(entries []runtimeWorkspaceCommandHelpEntry, topic string) *runtimeWorkspaceCommandHelpEntry {
	normalizedTopic := strings.ToLower(strings.TrimSpace(topic))
	if normalizedTopic == "" {
		return nil
	}
	for idx := range entries {
		if strings.ToLower(strings.TrimSpace(entries[idx].Alias)) == normalizedTopic ||
			strings.ToLower(strings.TrimSpace(entries[idx].Name)) == normalizedTopic ||
			strings.ToLower(strings.TrimSpace(entries[idx].Entry)) == normalizedTopic {
			return &entries[idx]
		}
	}
	return nil
}

func (r *persistentPluginRuntime) runtimeConsoleHelpPayload(topic string) map[string]interface{} {
	r.refreshWorkspaceCommandAliasesLocked()
	normalized := strings.ToLower(strings.TrimSpace(topic))
	callablePaths := r.snapshotCallablePathsLocked()
	workspaceCommands := r.snapshotWorkspaceCommandHelpEntriesLocked()
	workspaceCommandNames := runtimeWorkspaceCommandHelpNames(workspaceCommands)
	switch {
	case normalized == "", normalized == "console", normalized == "workspace", normalized == "runtime":
		payload := map[string]interface{}{
			"topic":   "console",
			"summary": "Workspace JS console attached to the plugin's live VM.",
			"helpers": []string{
				"help(topic?)",
				"keys(value?)",
				"runtimeState()",
				"commands()",
				"permissions()",
				"workspaceState(limit?)",
				"inspect(value?, depth?)",
				"clearOutput()",
			},
			"commands": []string{
				":inspect [--depth N] <expression>",
			},
			"globals": []string{
				"Plugin",
				"sandbox",
				"console",
				"globalThis",
				"Worker",
				"URLSearchParams",
				"TextEncoder",
				"TextDecoder",
				"atob",
				"btoa",
				"structuredClone",
				"queueMicrotask",
				"setTimeout",
				"clearTimeout",
				"$_",
				"$1",
				"$2",
				"$3",
				"$4",
				"$5",
			},
			"examples": []string{
				"help('Plugin.host')",
				"help('Worker')",
				"structuredClone({ hello: 'world' })",
				"queueMicrotask(function () { console.log('tick'); })",
				"setTimeout(function () { console.log('later'); }, 10)",
				"keys(Plugin.host)",
				"runtimeState()",
				"commands()",
				"permissions()",
				"workspaceState(10)",
				"inspect(Plugin.host, 3)",
				"clearOutput()",
				":inspect --depth 3 Plugin.host.order",
			},
		}
		if len(callablePaths) > 0 {
			payload["plugin_exports"] = callablePaths
		}
		if len(workspaceCommandNames) > 0 {
			payload["examples"] = append(payload["examples"].([]string), workspaceCommandNames[0]+"()")
		}
		return payload
	case normalized == "plugin", normalized == "plugin.":
		return map[string]interface{}{
			"topic":   "Plugin",
			"summary": "Root plugin API surface exposed inside the persistent JS worker runtime.",
			"members": []string{
				"Plugin.workspace",
				"Plugin.storage",
				"Plugin.secret",
				"Plugin.webhook",
				"Plugin.http",
				"Plugin.host",
				"Plugin.fs",
				"Plugin.order",
				"Plugin.user",
				"Plugin.product",
				"Plugin.inventory",
			},
		}
	case normalized == "worker", normalized == "worker()":
		return map[string]interface{}{
			"topic":   "Worker",
			"summary": "Dedicated child JS runtime for parallel work. Use new Worker(script), await worker.request(payload), worker.postMessage(payload), worker.onmessage, worker.onerror, and worker.terminate().",
			"members": []string{
				"new Worker(script)",
				"worker.id",
				"worker.scriptPath",
				"worker.request(payload)",
				"worker.postMessage(payload)",
				"worker.onmessage = fn",
				"worker.onerror = fn",
				"worker.terminate()",
			},
			"examples": []string{
				`const worker = new Worker("./child.js")`,
				`const result = await worker.request({ order_id: 1 })`,
			},
		}
	case normalized == "urlsearchparams":
		return map[string]interface{}{
			"topic":   "URLSearchParams",
			"summary": "Browser-compatible query-string helper for parsing and formatting URL parameters.",
			"examples": []string{
				`new URLSearchParams("?tab=timeline&tag=alpha").get("tab")`,
				`new URLSearchParams({ page: 1, limit: 20 }).toString()`,
			},
		}
	case normalized == "textencoder":
		return map[string]interface{}{
			"topic":   "TextEncoder",
			"summary": "UTF-8 encoder that converts strings into Uint8Array values for binary-safe runtime processing.",
			"examples": []string{
				`new TextEncoder().encode("Hello, world")`,
				`new TextEncoder().encodeInto("Hi!", new Uint8Array(8))`,
			},
		}
	case normalized == "textdecoder":
		return map[string]interface{}{
			"topic":   "TextDecoder",
			"summary": "UTF-8 decoder that converts Uint8Array or ArrayBuffer data back into JavaScript strings.",
			"examples": []string{
				`new TextDecoder().decode(new Uint8Array([72, 105]))`,
				`new TextDecoder("utf-8").decode(new TextEncoder().encode("Aurora"))`,
			},
		}
	case normalized == "atob", normalized == "btoa":
		return map[string]interface{}{
			"topic":   strings.TrimSpace(topic),
			"summary": "Base64 helpers compatible with browser atob/btoa semantics for binary-string conversion.",
			"examples": []string{
				`btoa("Hello!")`,
				`atob("SGVsbG8h")`,
			},
		}
	case normalized == "structuredclone", normalized == "structuredclone()":
		return map[string]interface{}{
			"topic":   "structuredClone",
			"summary": "structuredClone(value) deep-clones JSON-serializable values inside the runtime.",
			"examples": []string{
				`structuredClone({ nested: { value: 1 } })`,
			},
		}
	case normalized == "queuemicrotask", normalized == "queuemicrotask()":
		return map[string]interface{}{
			"topic":   "queueMicrotask",
			"summary": "queueMicrotask(callback) schedules a callback on the persistent runtime microtask queue and flushes it before the current expression returns.",
			"examples": []string{
				`queueMicrotask(function () { console.log("tick"); })`,
			},
		}
	case normalized == "settimeout", normalized == "settimeout()":
		return map[string]interface{}{
			"topic":   "setTimeout",
			"summary": "setTimeout(callback, delayMs, ...args) schedules a callback on the persistent runtime timer queue and returns a timer id.",
			"examples": []string{
				`setTimeout(function (label) { console.log(label); }, 10, "later")`,
			},
		}
	case normalized == "cleartimeout", normalized == "cleartimeout()":
		return map[string]interface{}{
			"topic":   "clearTimeout",
			"summary": "clearTimeout(timerId) cancels a pending timer created by setTimeout().",
			"examples": []string{
				`const id = setTimeout(function () {}, 50)`,
				`clearTimeout(id)`,
			},
		}
	case normalized == "exports", normalized == "module.exports", normalized == "plugin.exports":
		return map[string]interface{}{
			"topic":     "module.exports",
			"summary":   "Callable exports currently exposed by the plugin entry module.",
			"members":   runtimeConsoleFilterPathsWithPrefix(callablePaths, "module.exports"),
			"functions": callablePaths,
		}
	case normalized == "plugin.workspace":
		return map[string]interface{}{
			"topic":   "Plugin.workspace",
			"summary": "Append output, read admin input, or inspect the retained workspace buffer.",
			"members": []string{
				"enabled",
				"commandName",
				"commandId",
				"write(message, metadata?)",
				"writeln(message?, metadata?)",
				"info(message, metadata?)",
				"warn(message, metadata?)",
				"error(message, metadata?)",
				"clear()",
				"tail(limit?)",
				"snapshot(limit?)",
				"read(options?)",
				"readLine(promptOrOptions?, options?)",
			},
		}
	case normalized == "plugin.storage":
		return map[string]interface{}{
			"topic":   "Plugin.storage",
			"summary": "Persistent plugin KV storage governed by the declared storage mode for the current action.",
			"members": []string{
				"get(key)",
				"set(key, value)",
				"delete(key)",
				"list()",
				"clear()",
			},
		}
	case normalized == "plugin.secret":
		return map[string]interface{}{
			"topic":   "Plugin.secret",
			"summary": "Read granted secret values from the current invocation snapshot.",
			"members": []string{
				"get(key)",
				"has(key)",
				"list()",
			},
		}
	case normalized == "plugin.webhook":
		return map[string]interface{}{
			"topic":   "Plugin.webhook",
			"summary": "Inspect the current webhook request when the plugin is executing from a webhook entry.",
			"members": []string{
				"enabled",
				"key",
				"method",
				"path",
				"queryString",
				"contentType",
				"remoteAddr",
				"headers",
				"queryParams",
				"bodyText",
				"bodyBase64",
				"header(name)",
				"query(name)",
				"text()",
				"json()",
			},
		}
	case normalized == "plugin.http":
		return map[string]interface{}{
			"topic":   "Plugin.http",
			"summary": "Network bridge for outbound HTTP requests when allow_network is enabled.",
			"members": []string{
				"enabled",
				"defaultTimeoutMs",
				"maxResponseBytes",
				"get(url, headers?)",
				"post(url, body?, headers?)",
				"request(options)",
			},
		}
	case normalized == "plugin.host":
		return map[string]interface{}{
			"topic":   "Plugin.host",
			"summary": "Host bridge for native AuraLogic resources and privileged actions granted to the plugin.",
			"members": []string{
				"enabled",
				"invoke(action, params?)",
				"order",
				"user",
				"product",
				"inventory",
				"inventoryBinding",
				"promo",
				"ticket",
				"serial",
				"announcement",
				"knowledge",
				"paymentMethod",
				"virtualInventory",
				"virtualInventoryBinding",
			},
			"example": "Plugin.host.order.list({ page: 1, limit: 20 })",
		}
	case normalized == "plugin.fs":
		return map[string]interface{}{
			"topic":   "Plugin.fs",
			"summary": "Sandboxed filesystem access rooted at the plugin code and data directories.",
			"members": []string{
				"enabled",
				"root",
				"codeRoot",
				"dataRoot",
				"pluginID",
				"pluginName",
				"maxFiles",
				"maxTotalBytes",
				"maxReadBytes",
				"exists(path)",
				"readText(path)",
				"readBase64(path)",
				"readJSON(path)",
				"writeText(path, content)",
				"writeJSON(path, value)",
				"writeBase64(path, payload)",
				"delete(path)",
				"mkdir(path)",
				"list(path?)",
				"stat(path)",
				"usage()",
				"recalculateUsage()",
			},
		}
	case normalized == "sandbox":
		return map[string]interface{}{
			"topic":   "sandbox",
			"summary": "Current sandbox policy and effective execution limits for this invocation.",
			"members": []string{
				"currentAction",
				"allowNetwork",
				"allowFileSystem",
				"allowExecuteAPI",
				"allowHookExecute",
				"allowHookBlock",
				"allowPayloadPatch",
				"allowFrontendExtensions",
				"requestedPermissions",
				"grantedPermissions",
				"storageAccessMode",
				"defaultTimeoutMs",
				"maxConcurrency",
				"maxMemoryMB",
			},
		}
	case normalized == "$_", normalized == "$1", normalized == "$2", normalized == "$3", normalized == "$4", normalized == "$5", normalized == "results":
		return map[string]interface{}{
			"topic":   "runtime results",
			"summary": "Non-enumerable references to the last successful workspace runtime results.",
			"members": []string{
				"$_",
				"$1",
				"$2",
				"$3",
				"$4",
				"$5",
			},
			"example": "$_ && $1",
		}
	case normalized == "keys", normalized == "keys()":
		return map[string]interface{}{
			"topic":   "keys(value?)",
			"summary": "keys(value?) returns sorted enumerable keys for the target object.",
			"examples": []string{
				"keys(Plugin.host)",
				"keys(sandbox)",
				"keys(help())",
			},
		}
	case normalized == "runtimestate", normalized == "runtimestate()":
		return map[string]interface{}{
			"topic":   "runtimeState()",
			"summary": "runtimeState() returns the current VM, sandbox, and workspace snapshot.",
			"examples": []string{
				"runtimeState().runtime.plugin_id",
				"runtimeState().sandbox.granted_permissions",
				"runtimeState().workspace",
			},
		}
	case normalized == "commands", normalized == "commands()":
		return map[string]interface{}{
			"topic":   "commands()",
			"summary": "commands() lists runtime helpers, meta commands, workspace aliases, and callable plugin exports.",
			"examples": []string{
				"commands().helpers",
				"commands().workspace_aliases",
				"commands().plugin_exports",
			},
		}
	case normalized == "permissions", normalized == "permissions()":
		return map[string]interface{}{
			"topic":   "permissions()",
			"summary": "permissions() returns the current sandbox capability flags plus requested, granted, and missing host permissions.",
			"examples": []string{
				"permissions().granted_permissions",
				"permissions().missing_permissions",
				"permissions().allow_network",
			},
		}
	case normalized == "workspacestate", normalized == "workspacestate()":
		return map[string]interface{}{
			"topic":   "workspaceState(limit?)",
			"summary": "workspaceState(limit?) returns the current workspace buffer snapshot, command metadata, and pending admin input count.",
			"examples": []string{
				"workspaceState()",
				"workspaceState(20).entries",
				"workspaceState().command_id",
			},
		}
	case normalized == "inspect", normalized == "inspect()":
		return map[string]interface{}{
			"topic":   "inspect(value?, depth?)",
			"summary": "inspect(value?, depth?) returns a structured preview map for any runtime value without leaving JS expression mode.",
			"examples": []string{
				"inspect(Plugin.host, 3)",
				"inspect(runtimeState(), 2)",
			},
		}
	case normalized == "clearoutput", normalized == "clearoutput()":
		return map[string]interface{}{
			"topic":   "clearOutput()",
			"summary": "clearOutput() clears the current workspace transcript buffer and returns whether the clear succeeded.",
			"examples": []string{
				"clearOutput()",
			},
		}
	default:
		if command := runtimeConsoleFindWorkspaceCommand(workspaceCommands, topic); command != nil {
			payload := map[string]interface{}{
				"topic":       command.Alias,
				"summary":     "Callable workspace function exported by the current plugin runtime.",
				"callable":    true,
				"entry":       command.Entry,
				"interactive": command.Interactive,
			}
			if command.Alias != "" {
				payload["alias"] = command.Alias
			}
			if command.Name != "" {
				payload["name"] = command.Name
			}
			if command.Title != "" {
				payload["title"] = command.Title
			}
			if command.Description != "" {
				payload["description"] = command.Description
			}
			if len(command.Permissions) > 0 {
				payload["permissions"] = append([]string(nil), command.Permissions...)
			}
			return payload
		}
		if callablePath := runtimeConsoleFindCallablePath(callablePaths, topic); callablePath != "" {
			return map[string]interface{}{
				"topic":    callablePath,
				"summary":  "Callable function exported by the current plugin runtime.",
				"callable": true,
			}
		}
		return map[string]interface{}{
			"topic":            strings.TrimSpace(topic),
			"summary":          "Unknown help topic.",
			"available_topics": []string{"Plugin", "Plugin.workspace", "Plugin.storage", "Plugin.secret", "Plugin.webhook", "Plugin.http", "Plugin.host", "Plugin.fs", "Worker", "sandbox", "results", "module.exports", "URLSearchParams", "TextEncoder", "TextDecoder", "atob", "btoa", "structuredClone", "queueMicrotask", "setTimeout", "clearTimeout", "commands", "permissions", "workspaceState", "inspect", "clearOutput"},
			"plugin_exports":   callablePaths,
		}
	}
}

func (r *persistentPluginRuntime) runtimeConsoleStatePayload() map[string]interface{} {
	runtimeState := runtimeStateToMap(r.snapshotStateLocked())
	out := map[string]interface{}{
		"runtime": runtimeState,
	}
	invocation := r.currentInvocation()
	if invocation == nil {
		return out
	}
	out["sandbox"] = map[string]interface{}{
		"current_action":            invocation.sandboxCfg.CurrentAction,
		"allow_network":             invocation.effectiveOpts.allowNetwork,
		"allow_file_system":         invocation.effectiveOpts.allowFS,
		"allow_execute_api":         invocation.sandboxCfg.AllowExecuteAPI,
		"allow_hook_execute":        invocation.sandboxCfg.AllowHookExecute,
		"allow_hook_block":          invocation.sandboxCfg.AllowHookBlock,
		"allow_payload_patch":       invocation.sandboxCfg.AllowPayloadPatch,
		"allow_frontend_extensions": invocation.sandboxCfg.AllowFrontendExtensions,
		"requested_permissions":     append([]string{}, invocation.sandboxCfg.RequestedPermissions...),
		"granted_permissions":       append([]string{}, invocation.sandboxCfg.GrantedPermissions...),
		"declared_storage_access":   invocation.sandboxCfg.DeclaredStorageAccess,
		"effective_storage_access":  invocation.storageState.accessMode(),
		"default_timeout_ms":        invocation.effectiveOpts.timeoutMs,
		"max_concurrency":           invocation.effectiveOpts.maxConcurrency,
		"max_memory_mb":             invocation.effectiveOpts.maxMemoryMB,
	}
	if invocation.workspaceState != nil {
		out["workspace"] = map[string]interface{}{
			"enabled":      invocation.workspaceState.enabled,
			"command_name": invocation.workspaceState.commandName,
			"command_id":   invocation.workspaceState.commandID,
		}
	} else {
		out["workspace"] = map[string]interface{}{
			"enabled": false,
		}
	}
	out["webhook"] = map[string]interface{}{
		"enabled":      invocation.webhookState.Enabled,
		"key":          invocation.webhookState.Key,
		"method":       invocation.webhookState.Method,
		"path":         invocation.webhookState.Path,
		"query_string": invocation.webhookState.QueryString,
		"content_type": invocation.webhookState.ContentType,
	}
	return out
}

func (r *persistentPluginRuntime) currentInvocation() *persistentPluginRuntimeInvocation {
	if r == nil {
		return nil
	}
	return r.currentInvoke
}

func (r *persistentPluginRuntime) currentWorkspaceState() *pluginWorkspaceState {
	invocation := r.currentInvocation()
	if invocation == nil {
		return nil
	}
	return invocation.workspaceState
}

func (r *persistentPluginRuntime) currentStorageState() *pluginStorageState {
	invocation := r.currentInvocation()
	if invocation == nil {
		return nil
	}
	return invocation.storageState
}

func (r *persistentPluginRuntime) applyInvocation(invocation *persistentPluginRuntimeInvocation) error {
	if r == nil || invocation == nil {
		return fmt.Errorf("runtime invocation is unavailable")
	}

	if workspaceState := invocation.workspaceState; workspaceState != nil {
		configureWorkerWorkspaceHostBridge(workspaceState, invocation.hostCfg, invocation.workspaceHostOK)
	}
	if r.allowWorkers && r.workerBindings == nil {
		if invocation.workerGroup == nil {
			invocation.workerGroup = newPluginRuntimeWorkerGroup(r, invocation)
		}
	} else {
		if invocation.workerGroup != nil {
			invocation.workerGroup.close()
		}
		invocation.workerGroup = nil
	}

	_ = r.sandboxObj.Set("allowNetwork", invocation.effectiveOpts.allowNetwork)
	_ = r.sandboxObj.Set("allowFileSystem", invocation.effectiveOpts.allowFS)
	_ = r.sandboxObj.Set("currentAction", invocation.sandboxCfg.CurrentAction)
	_ = r.sandboxObj.Set("declaredStorageAccessMode", invocation.sandboxCfg.DeclaredStorageAccess)
	_ = r.sandboxObj.Set("allowHookExecute", invocation.sandboxCfg.AllowHookExecute)
	_ = r.sandboxObj.Set("allowHookBlock", invocation.sandboxCfg.AllowHookBlock)
	_ = r.sandboxObj.Set("allowPayloadPatch", invocation.sandboxCfg.AllowPayloadPatch)
	_ = r.sandboxObj.Set("allowFrontendExtensions", invocation.sandboxCfg.AllowFrontendExtensions)
	_ = r.sandboxObj.Set("allowExecuteAPI", invocation.sandboxCfg.AllowExecuteAPI)
	_ = r.sandboxObj.Set("requestedPermissions", r.runtimeBridgeValue(append([]string{}, invocation.sandboxCfg.RequestedPermissions...)))
	_ = r.sandboxObj.Set("grantedPermissions", r.runtimeBridgeValue(append([]string{}, invocation.sandboxCfg.GrantedPermissions...)))
	_ = r.sandboxObj.Set("executeActionStorage", r.runtimeBridgeValue(mergeStringMaps(invocation.sandboxCfg.ExecuteActionStorage, nil)))
	_ = r.sandboxObj.Set("defaultTimeoutMs", invocation.effectiveOpts.timeoutMs)
	_ = r.sandboxObj.Set("maxConcurrency", invocation.effectiveOpts.maxConcurrency)
	_ = r.sandboxObj.Set("maxMemoryMB", invocation.effectiveOpts.maxMemoryMB)
	_ = r.sandboxObj.Set("fsMaxFiles", invocation.effectiveOpts.fsMaxFiles)
	_ = r.sandboxObj.Set("fsMaxTotalBytes", invocation.effectiveOpts.fsMaxTotalBytes)
	_ = r.sandboxObj.Set("fsMaxReadBytes", invocation.effectiveOpts.fsMaxReadBytes)
	_ = r.sandboxObj.Set("storageMaxKeys", invocation.effectiveOpts.storageMaxKeys)
	_ = r.sandboxObj.Set("storageMaxTotalBytes", invocation.effectiveOpts.storageMaxTotalBytes)
	_ = r.sandboxObj.Set("storageMaxValueBytes", invocation.effectiveOpts.storageMaxValueBytes)
	r.refreshSandboxStorageAccessMode()

	workspaceEnabled := invocation.workspaceState != nil && invocation.workspaceState.enabled
	_ = r.workspaceObj.Set("enabled", workspaceEnabled)
	if invocation.workspaceState != nil {
		_ = r.workspaceObj.Set("commandName", invocation.workspaceState.commandName)
		_ = r.workspaceObj.Set("commandId", invocation.workspaceState.commandID)
	} else {
		_ = r.workspaceObj.Set("commandName", "")
		_ = r.workspaceObj.Set("commandId", "")
	}

	_ = r.secretObj.Set("enabled", true)

	_ = r.webhookObj.Set("enabled", invocation.webhookState.Enabled)
	_ = r.webhookObj.Set("key", invocation.webhookState.Key)
	_ = r.webhookObj.Set("method", invocation.webhookState.Method)
	_ = r.webhookObj.Set("path", invocation.webhookState.Path)
	_ = r.webhookObj.Set("queryString", invocation.webhookState.QueryString)
	_ = r.webhookObj.Set("contentType", invocation.webhookState.ContentType)
	_ = r.webhookObj.Set("remoteAddr", invocation.webhookState.RemoteAddr)
	_ = r.webhookObj.Set("headers", r.runtimeBridgeValue(mergeStringMaps(invocation.webhookState.Headers, nil)))
	_ = r.webhookObj.Set("queryParams", r.runtimeBridgeValue(mergeStringMaps(invocation.webhookState.QueryParams, nil)))
	_ = r.webhookObj.Set("bodyText", invocation.webhookState.BodyText)
	_ = r.webhookObj.Set("bodyBase64", invocation.webhookState.BodyBase64)

	hostEnabled := invocation.hostCfg != nil &&
		strings.TrimSpace(invocation.hostCfg.Network) != "" &&
		strings.TrimSpace(invocation.hostCfg.Address) != "" &&
		strings.TrimSpace(invocation.hostCfg.AccessToken) != ""
	_ = r.hostObj.Set("enabled", hostEnabled)

	_ = r.httpObj.Set("enabled", invocation.effectiveOpts.allowNetwork)
	_ = r.httpObj.Set("defaultTimeoutMs", clampPluginHTTPTimeoutMs(0, invocation.effectiveOpts.timeoutMs))
	_ = r.httpObj.Set("maxResponseBytes", maxPluginHTTPResponseBytes)

	_ = r.fsObj.Set("enabled", invocation.effectiveOpts.allowFS)
	_ = r.fsObj.Set("root", "/")
	_ = r.fsObj.Set("codeRoot", filepath.ToSlash(r.fsCtx.CodeRoot))
	_ = r.fsObj.Set("dataRoot", filepath.ToSlash(r.fsCtx.DataRoot))
	_ = r.fsObj.Set("pluginID", r.fsCtx.PluginID)
	_ = r.fsObj.Set("pluginName", r.fsCtx.PluginName)
	_ = r.fsObj.Set("maxFiles", invocation.effectiveOpts.fsMaxFiles)
	_ = r.fsObj.Set("maxTotalBytes", invocation.effectiveOpts.fsMaxTotalBytes)
	_ = r.fsObj.Set("maxReadBytes", invocation.effectiveOpts.fsMaxReadBytes)

	return nil
}

func (r *persistentPluginRuntime) refreshSandboxStorageAccessMode() {
	mode := storageAccessNone
	if storageState := r.currentStorageState(); storageState != nil {
		mode = storageState.accessMode()
	}
	_ = r.sandboxObj.Set("storageAccessMode", mode)
}

func (r *persistentPluginRuntime) throwJSError(err error) {
	if err == nil {
		panic(r.vm.NewTypeError("unknown persistent runtime error"))
	}
	panic(r.vm.NewGoError(err))
}

func (r *persistentPluginRuntime) requireFS() *pluginFS {
	invocation := r.currentInvocation()
	if invocation == nil {
		r.throwJSError(fmt.Errorf("Plugin.fs is unavailable"))
	}
	if !invocation.effectiveOpts.allowFS {
		panic(r.vm.NewTypeError("Plugin.fs access denied: allow_file_system=false"))
	}
	if invocation.pluginFSError != nil {
		r.throwJSError(invocation.pluginFSError)
	}
	if invocation.pluginFSState == nil {
		r.throwJSError(fmt.Errorf("plugin fs is not available"))
	}
	return invocation.pluginFSState
}

func (r *persistentPluginRuntime) requireNetwork() {
	invocation := r.currentInvocation()
	if invocation == nil || !invocation.effectiveOpts.allowNetwork {
		panic(r.vm.NewTypeError("Plugin.http access denied: allow_network=false"))
	}
}

func (r *persistentPluginRuntime) requestWorkspaceInput(prompt string, masked bool, echo bool, source string) (string, bool) {
	invocation := r.currentInvocation()
	if invocation == nil || invocation.workspaceState == nil {
		return "", false
	}
	value, ok, err := requestWorkerWorkspaceInput(
		invocation.workspaceState,
		invocation.hostCfg,
		invocation.workspaceHostOK,
		prompt,
		masked,
		echo,
		source,
	)
	if err != nil {
		r.throwJSError(err)
	}
	return value, ok
}

func (r *persistentPluginRuntime) installWorkspaceGlobals() {
	_ = r.workspaceObj.Set("write", func(call goja.FunctionCall) goja.Value {
		workspaceState := r.currentWorkspaceState()
		if len(call.Arguments) < 1 || workspaceState == nil {
			return goja.Undefined()
		}
		workspaceState.write("stdout", "info", call.Arguments[0].String(), "plugin.workspace.write", workspaceMetadataFromGojaValue(argumentAt(call, 1)))
		return goja.Undefined()
	})
	_ = r.workspaceObj.Set("writeln", func(call goja.FunctionCall) goja.Value {
		workspaceState := r.currentWorkspaceState()
		if workspaceState == nil {
			return goja.Undefined()
		}
		message := ""
		if len(call.Arguments) > 0 && !goja.IsUndefined(call.Arguments[0]) && !goja.IsNull(call.Arguments[0]) {
			message = call.Arguments[0].String()
		}
		workspaceState.write("stdout", "info", message+"\n", "plugin.workspace.writeln", workspaceMetadataFromGojaValue(argumentAt(call, 1)))
		return goja.Undefined()
	})
	for _, definition := range []struct {
		name   string
		level  string
		source string
	}{
		{name: "info", level: "info", source: "plugin.workspace.info"},
		{name: "warn", level: "warn", source: "plugin.workspace.warn"},
		{name: "error", level: "error", source: "plugin.workspace.error"},
	} {
		level := definition.level
		source := definition.source
		_ = r.workspaceObj.Set(definition.name, func(call goja.FunctionCall) goja.Value {
			workspaceState := r.currentWorkspaceState()
			if len(call.Arguments) < 1 || workspaceState == nil {
				return goja.Undefined()
			}
			workspaceState.write("workspace", level, call.Arguments[0].String(), source, workspaceMetadataFromGojaValue(argumentAt(call, 1)))
			return goja.Undefined()
		})
	}
	_ = r.workspaceObj.Set("clear", func(call goja.FunctionCall) goja.Value {
		workspaceState := r.currentWorkspaceState()
		if workspaceState == nil {
			return r.vm.ToValue(false)
		}
		return r.vm.ToValue(workspaceState.clear())
	})
	_ = r.workspaceObj.Set("tail", func(call goja.FunctionCall) goja.Value {
		workspaceState := r.currentWorkspaceState()
		if workspaceState == nil {
			return r.runtimeBridgeValue([]map[string]interface{}{})
		}
		return r.runtimeBridgeValue(workspaceEntriesToMaps(workspaceState.tail(workspaceLimitFromGojaArguments(call, 0))))
	})
	_ = r.workspaceObj.Set("snapshot", func(call goja.FunctionCall) goja.Value {
		workspaceState := r.currentWorkspaceState()
		if workspaceState == nil {
			return r.runtimeBridgeValue(map[string]interface{}{"enabled": false, "max_entries": 0, "entry_count": 0, "entries": []map[string]interface{}{}})
		}
		return r.runtimeBridgeValue(workspaceState.snapshot(workspaceLimitFromGojaArguments(call, 0)))
	})
	_ = r.workspaceObj.Set("read", func(call goja.FunctionCall) goja.Value {
		if r.currentWorkspaceState() == nil {
			panic(r.vm.NewTypeError("Plugin.workspace is unavailable"))
		}
		echo, masked := parseWorkspaceReadOptions(argumentAt(call, 0))
		value, ok := r.requestWorkspaceInput("", masked, echo, "plugin.workspace.read")
		if !ok {
			panic(r.vm.NewTypeError("Plugin.workspace.read has no input available"))
		}
		return r.vm.ToValue(value)
	})
	_ = r.workspaceObj.Set("readLine", func(call goja.FunctionCall) goja.Value {
		if r.currentWorkspaceState() == nil {
			panic(r.vm.NewTypeError("Plugin.workspace is unavailable"))
		}
		prompt := ""
		optionsArgIndex := 0
		if len(call.Arguments) > 0 && call.Arguments[0] != nil && !goja.IsUndefined(call.Arguments[0]) && !goja.IsNull(call.Arguments[0]) {
			if exported := call.Arguments[0].Export(); exported != nil {
				if _, ok := exported.(map[string]interface{}); !ok {
					prompt = call.Arguments[0].String()
					optionsArgIndex = 1
				}
			}
		}
		echo, masked := parseWorkspaceReadOptions(argumentAt(call, optionsArgIndex))
		value, ok := r.requestWorkspaceInput(prompt, masked, echo, "plugin.workspace.readLine")
		if !ok {
			panic(r.vm.NewTypeError("Plugin.workspace.readLine has no input available"))
		}
		return r.vm.ToValue(value)
	})
}

func (r *persistentPluginRuntime) installStorageGlobals() {
	_ = r.storageObj.Set("get", func(call goja.FunctionCall) goja.Value {
		storageState := r.currentStorageState()
		if len(call.Arguments) < 1 || storageState == nil {
			return goja.Undefined()
		}
		key := strings.TrimSpace(call.Arguments[0].String())
		if key == "" {
			return goja.Undefined()
		}
		value, ok := storageState.get(key)
		r.refreshSandboxStorageAccessMode()
		if !ok {
			return goja.Undefined()
		}
		return r.vm.ToValue(value)
	})
	_ = r.storageObj.Set("set", func(call goja.FunctionCall) goja.Value {
		storageState := r.currentStorageState()
		if len(call.Arguments) < 2 || storageState == nil {
			return r.vm.ToValue(false)
		}
		ok := storageState.set(strings.TrimSpace(call.Arguments[0].String()), call.Arguments[1].String())
		r.refreshSandboxStorageAccessMode()
		return r.vm.ToValue(ok)
	})
	_ = r.storageObj.Set("delete", func(call goja.FunctionCall) goja.Value {
		storageState := r.currentStorageState()
		if len(call.Arguments) < 1 || storageState == nil {
			return r.vm.ToValue(false)
		}
		ok := storageState.delete(strings.TrimSpace(call.Arguments[0].String()))
		r.refreshSandboxStorageAccessMode()
		return r.vm.ToValue(ok)
	})
	_ = r.storageObj.Set("list", func(call goja.FunctionCall) goja.Value {
		storageState := r.currentStorageState()
		if storageState == nil {
			return r.vm.ToValue([]string{})
		}
		values := storageState.list()
		r.refreshSandboxStorageAccessMode()
		return r.vm.ToValue(values)
	})
	_ = r.storageObj.Set("clear", func(call goja.FunctionCall) goja.Value {
		storageState := r.currentStorageState()
		if storageState == nil {
			return r.vm.ToValue(false)
		}
		ok := storageState.clear()
		r.refreshSandboxStorageAccessMode()
		return r.vm.ToValue(ok)
	})
}

func (r *persistentPluginRuntime) installSecretGlobals() {
	_ = r.secretObj.Set("get", func(call goja.FunctionCall) goja.Value {
		invocation := r.currentInvocation()
		if len(call.Arguments) < 1 || invocation == nil {
			return goja.Undefined()
		}
		key := strings.TrimSpace(call.Arguments[0].String())
		if key == "" {
			return goja.Undefined()
		}
		value, ok := invocation.secretSnapshot[key]
		if !ok {
			return goja.Undefined()
		}
		return r.vm.ToValue(value)
	})
	_ = r.secretObj.Set("has", func(call goja.FunctionCall) goja.Value {
		invocation := r.currentInvocation()
		if len(call.Arguments) < 1 || invocation == nil {
			return r.vm.ToValue(false)
		}
		key := strings.TrimSpace(call.Arguments[0].String())
		if key == "" {
			return r.vm.ToValue(false)
		}
		_, ok := invocation.secretSnapshot[key]
		return r.vm.ToValue(ok)
	})
	_ = r.secretObj.Set("list", func(call goja.FunctionCall) goja.Value {
		invocation := r.currentInvocation()
		if invocation == nil || len(invocation.secretSnapshot) == 0 {
			return r.vm.ToValue([]string{})
		}
		keys := make([]string, 0, len(invocation.secretSnapshot))
		for key := range invocation.secretSnapshot {
			if strings.TrimSpace(key) == "" {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return r.vm.ToValue(keys)
	})
}

func (r *persistentPluginRuntime) installWebhookGlobals() {
	_ = r.webhookObj.Set("header", func(call goja.FunctionCall) goja.Value {
		invocation := r.currentInvocation()
		if len(call.Arguments) < 1 || invocation == nil {
			return goja.Undefined()
		}
		name := strings.ToLower(strings.TrimSpace(call.Arguments[0].String()))
		if name == "" {
			return goja.Undefined()
		}
		value, ok := invocation.webhookState.Headers[name]
		if !ok {
			return goja.Undefined()
		}
		return r.vm.ToValue(value)
	})
	_ = r.webhookObj.Set("query", func(call goja.FunctionCall) goja.Value {
		invocation := r.currentInvocation()
		if len(call.Arguments) < 1 || invocation == nil {
			return goja.Undefined()
		}
		name := strings.TrimSpace(call.Arguments[0].String())
		if name == "" {
			return goja.Undefined()
		}
		value, ok := invocation.webhookState.QueryParams[name]
		if !ok {
			return goja.Undefined()
		}
		return r.vm.ToValue(value)
	})
	_ = r.webhookObj.Set("text", func(call goja.FunctionCall) goja.Value {
		invocation := r.currentInvocation()
		if invocation == nil {
			return r.vm.ToValue("")
		}
		return r.vm.ToValue(invocation.webhookState.BodyText)
	})
	_ = r.webhookObj.Set("json", func(call goja.FunctionCall) goja.Value {
		invocation := r.currentInvocation()
		if invocation == nil || strings.TrimSpace(invocation.webhookState.BodyText) == "" {
			return goja.Undefined()
		}
		var decoded interface{}
		if err := json.Unmarshal([]byte(invocation.webhookState.BodyText), &decoded); err != nil {
			r.throwJSError(fmt.Errorf("Plugin.webhook.json() failed: %w", err))
		}
		return r.runtimeBridgeValue(decoded)
	})
}

func (r *persistentPluginRuntime) installHTTPGlobals() {
	_ = r.httpObj.Set("get", func(call goja.FunctionCall) goja.Value {
		r.requireNetwork()
		if len(call.Arguments) < 1 {
			r.throwJSError(fmt.Errorf("Plugin.http.get(url, headers?) requires url"))
		}
		headers := map[string]string{}
		if len(call.Arguments) > 1 {
			headers = normalizePluginHTTPHeaders(exportGojaValue(call.Arguments[1]))
		}
		invocation := r.currentInvocation()
		return r.runtimeBridgeValue(performPluginHTTPRequest(invocation.effectiveOpts, pluginHTTPRequestOptions{
			URL:     call.Arguments[0].String(),
			Method:  http.MethodGet,
			Headers: headers,
		}))
	})
	_ = r.httpObj.Set("post", func(call goja.FunctionCall) goja.Value {
		r.requireNetwork()
		if len(call.Arguments) < 1 {
			r.throwJSError(fmt.Errorf("Plugin.http.post(url, body?, headers?) requires url"))
		}
		var body interface{}
		headers := map[string]string{}
		if len(call.Arguments) > 1 {
			body = exportGojaValue(call.Arguments[1])
		}
		if len(call.Arguments) > 2 {
			headers = normalizePluginHTTPHeaders(exportGojaValue(call.Arguments[2]))
		}
		invocation := r.currentInvocation()
		return r.runtimeBridgeValue(performPluginHTTPRequest(invocation.effectiveOpts, pluginHTTPRequestOptions{
			URL:     call.Arguments[0].String(),
			Method:  http.MethodPost,
			Headers: headers,
			Body:    body,
		}))
	})
	_ = r.httpObj.Set("request", func(call goja.FunctionCall) goja.Value {
		r.requireNetwork()
		if len(call.Arguments) < 1 {
			r.throwJSError(fmt.Errorf("Plugin.http.request(options) requires options"))
		}
		invocation := r.currentInvocation()
		return r.runtimeBridgeValue(performPluginHTTPRequest(invocation.effectiveOpts, decodePluginHTTPRequestOptions(exportGojaValue(call.Arguments[0]))))
	})
}

func (r *persistentPluginRuntime) installHostGlobals() {
	hostInvoker := func(action string, params map[string]interface{}) map[string]interface{} {
		invocation := r.currentInvocation()
		if invocation == nil {
			r.throwJSError(fmt.Errorf("Plugin.host is unavailable"))
		}
		result, err := performPluginHostRequest(invocation.hostCfg, action, params)
		if err != nil {
			r.throwJSError(err)
		}
		return result
	}
	_ = r.hostObj.Set("invoke", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 1 {
			r.throwJSError(fmt.Errorf("Plugin.host.invoke(action, params?) requires action"))
		}
		action := strings.TrimSpace(call.Arguments[0].String())
		params := map[string]interface{}{}
		if len(call.Arguments) > 1 {
			params = normalizePluginHostParams(exportGojaValue(call.Arguments[1]))
		}
		return r.runtimeBridgeValue(hostInvoker(action, params))
	})

	setHostResource := func(name string, actions map[string]string) *goja.Object {
		obj := r.vm.NewObject()
		for method, action := range actions {
			methodName := method
			actionName := action
			_ = obj.Set(methodName, func(call goja.FunctionCall) goja.Value {
				var params map[string]interface{}
				if strings.EqualFold(methodName, "get") {
					params = buildPluginHostObjectParams(exportPluginHostArgument(call.Arguments, 0), "id")
				} else {
					params = normalizePluginHostParams(exportPluginHostArgument(call.Arguments, 0))
				}
				return r.runtimeBridgeValue(hostInvoker(actionName, params))
			})
		}
		_ = r.hostObj.Set(name, obj)
		_ = r.pluginObj.Set(name, obj)
		return obj
	}

	setHostResource("order", map[string]string{"get": "host.order.get", "list": "host.order.list", "assignTracking": "host.order.assign_tracking", "requestResubmit": "host.order.request_resubmit", "markPaid": "host.order.mark_paid", "updatePrice": "host.order.update_price"})
	setHostResource("user", map[string]string{"get": "host.user.get", "list": "host.user.list"})
	setHostResource("product", map[string]string{"get": "host.product.get", "list": "host.product.list"})
	setHostResource("inventory", map[string]string{"get": "host.inventory.get", "list": "host.inventory.list"})
	setHostResource("inventoryBinding", map[string]string{"get": "host.inventory_binding.get", "list": "host.inventory_binding.list"})
	setHostResource("promo", map[string]string{"get": "host.promo.get", "list": "host.promo.list"})
	setHostResource("ticket", map[string]string{"get": "host.ticket.get", "list": "host.ticket.list", "reply": "host.ticket.reply", "update": "host.ticket.update"})
	setHostResource("serial", map[string]string{"get": "host.serial.get", "list": "host.serial.list"})
	setHostResource("announcement", map[string]string{"get": "host.announcement.get", "list": "host.announcement.list"})
	setHostResource("knowledge", map[string]string{"get": "host.knowledge.get", "list": "host.knowledge.list", "categories": "host.knowledge.categories"})
	setHostResource("paymentMethod", map[string]string{"get": "host.payment_method.get", "list": "host.payment_method.list"})
	setHostResource("virtualInventory", map[string]string{"get": "host.virtual_inventory.get", "list": "host.virtual_inventory.list"})
	setHostResource("virtualInventoryBinding", map[string]string{"get": "host.virtual_inventory_binding.get", "list": "host.virtual_inventory_binding.list"})

	for key, value := range buildSharedPluginHostRootObjects(r.vm, hostInvoker) {
		_ = r.hostObj.Set(key, value)
		_ = r.pluginObj.Set(key, value)
	}
}

func (r *persistentPluginRuntime) installFSGlobals() {
	_ = r.fsObj.Set("exists", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		if len(call.Arguments) < 1 {
			return r.vm.ToValue(false)
		}
		ok, err := fsState.Exists(call.Arguments[0].String())
		if err != nil {
			r.throwJSError(err)
		}
		return r.vm.ToValue(ok)
	})
	_ = r.fsObj.Set("readText", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		if len(call.Arguments) < 1 {
			r.throwJSError(fmt.Errorf("Plugin.fs.readText(path) requires path"))
		}
		content, err := fsState.ReadText(call.Arguments[0].String())
		if err != nil {
			r.throwJSError(err)
		}
		return r.runtimeBridgeValue(content)
	})
	_ = r.fsObj.Set("readBase64", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		if len(call.Arguments) < 1 {
			r.throwJSError(fmt.Errorf("Plugin.fs.readBase64(path) requires path"))
		}
		content, err := fsState.ReadBase64(call.Arguments[0].String())
		if err != nil {
			r.throwJSError(err)
		}
		return r.vm.ToValue(content)
	})
	_ = r.fsObj.Set("readJSON", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		if len(call.Arguments) < 1 {
			r.throwJSError(fmt.Errorf("Plugin.fs.readJSON(path) requires path"))
		}
		content, err := fsState.ReadJSON(call.Arguments[0].String())
		if err != nil {
			r.throwJSError(err)
		}
		return r.vm.ToValue(content)
	})
	_ = r.fsObj.Set("writeText", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		if len(call.Arguments) < 2 {
			r.throwJSError(fmt.Errorf("Plugin.fs.writeText(path, content) requires path and content"))
		}
		if err := fsState.WriteText(call.Arguments[0].String(), call.Arguments[1].String()); err != nil {
			r.throwJSError(err)
		}
		return r.vm.ToValue(true)
	})
	_ = r.fsObj.Set("writeJSON", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		if len(call.Arguments) < 2 {
			r.throwJSError(fmt.Errorf("Plugin.fs.writeJSON(path, value) requires path and value"))
		}
		if err := fsState.WriteJSON(call.Arguments[0].String(), exportGojaValue(call.Arguments[1])); err != nil {
			r.throwJSError(err)
		}
		return r.vm.ToValue(true)
	})
	_ = r.fsObj.Set("writeBase64", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		if len(call.Arguments) < 2 {
			r.throwJSError(fmt.Errorf("Plugin.fs.writeBase64(path, base64) requires path and base64 payload"))
		}
		if err := fsState.WriteBase64(call.Arguments[0].String(), call.Arguments[1].String()); err != nil {
			r.throwJSError(err)
		}
		return r.vm.ToValue(true)
	})
	_ = r.fsObj.Set("delete", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		if len(call.Arguments) < 1 {
			return r.vm.ToValue(false)
		}
		ok, err := fsState.Delete(call.Arguments[0].String())
		if err != nil {
			r.throwJSError(err)
		}
		return r.vm.ToValue(ok)
	})
	_ = r.fsObj.Set("mkdir", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		if len(call.Arguments) < 1 {
			r.throwJSError(fmt.Errorf("Plugin.fs.mkdir(path) requires path"))
		}
		if err := fsState.MkdirAll(call.Arguments[0].String()); err != nil {
			r.throwJSError(err)
		}
		return r.vm.ToValue(true)
	})
	_ = r.fsObj.Set("list", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		target := "."
		if len(call.Arguments) > 0 {
			target = call.Arguments[0].String()
		}
		items, err := fsState.List(target)
		if err != nil {
			r.throwJSError(err)
		}
		return r.runtimeBridgeValue(items)
	})
	_ = r.fsObj.Set("stat", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		if len(call.Arguments) < 1 {
			r.throwJSError(fmt.Errorf("Plugin.fs.stat(path) requires path"))
		}
		stat, err := fsState.Stat(call.Arguments[0].String())
		if err != nil {
			r.throwJSError(err)
		}
		return r.runtimeBridgeValue(stat)
	})
	_ = r.fsObj.Set("usage", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		usage, err := fsState.Usage()
		if err != nil {
			r.throwJSError(err)
		}
		return r.runtimeBridgeValue(pluginFSUsageToMap(usage))
	})
	_ = r.fsObj.Set("recalculateUsage", func(call goja.FunctionCall) goja.Value {
		fsState := r.requireFS()
		usage, err := fsState.RecalculateUsage()
		if err != nil {
			r.throwJSError(err)
		}
		return r.runtimeBridgeValue(pluginFSUsageToMap(usage))
	})
}
