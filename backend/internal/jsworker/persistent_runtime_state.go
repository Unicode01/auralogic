package jsworker

import (
	"sort"
	"strings"
	"time"

	"github.com/dop251/goja"
)

type persistentPluginRuntimeState struct {
	Available       bool
	Exists          bool
	InstanceID      string
	PluginID        uint
	Generation      uint
	ScriptPath      string
	Loaded          bool
	Busy            bool
	CurrentAction   string
	LastAction      string
	CreatedAt       time.Time
	LastUsedAt      time.Time
	LastBootAt      time.Time
	BootCount       int64
	TotalRequests   int64
	ExecuteCount    int64
	EvalCount       int64
	InspectCount    int64
	LastError       string
	RefCount        int
	Disposed        bool
	CompletionPaths []string
	CallablePaths   []string
}

func (r *persistentPluginRuntime) snapshotState() persistentPluginRuntimeState {
	if r == nil {
		return persistentPluginRuntimeState{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.snapshotStateLocked()
}

func (r *persistentPluginRuntime) snapshotStateLocked() persistentPluginRuntimeState {
	if r == nil {
		return persistentPluginRuntimeState{}
	}
	if r.loaded {
		r.refreshWorkspaceCommandAliasesLocked()
	}
	return persistentPluginRuntimeState{
		Available:       true,
		Exists:          true,
		InstanceID:      r.instanceID,
		PluginID:        r.key.PluginID,
		Generation:      r.key.Generation,
		ScriptPath:      r.scriptPath,
		Loaded:          r.loaded,
		Busy:            r.currentInvoke != nil,
		CurrentAction:   r.currentAction,
		LastAction:      r.lastAction,
		CreatedAt:       r.createdAt.UTC(),
		LastUsedAt:      r.lastUsedAt.UTC(),
		LastBootAt:      r.lastBootAt.UTC(),
		BootCount:       r.bootCount,
		TotalRequests:   r.totalRequests,
		ExecuteCount:    r.executeCount,
		EvalCount:       r.evalCount,
		InspectCount:    r.inspectCount,
		LastError:       r.lastError,
		CompletionPaths: r.snapshotCompletionPathsLocked(),
		CallablePaths:   r.snapshotCallablePathsLocked(),
	}
}

func (m *persistentPluginRuntimeManager) snapshot(pluginID uint, generation uint) persistentPluginRuntimeState {
	state := persistentPluginRuntimeState{
		Available:  true,
		PluginID:   pluginID,
		Generation: normalizePersistentPluginGeneration(generation),
	}
	if m == nil || pluginID == 0 {
		return state
	}

	key := buildPersistentPluginRuntimeKey(pluginID, generation)
	m.mu.Lock()
	defer m.mu.Unlock()
	ref := m.runtimes[key]
	if ref == nil || ref.runtime == nil {
		return state
	}
	state = ref.runtime.snapshotStateLocked()
	state.RefCount = ref.refCount
	state.Disposed = ref.disposed
	return state
}

func runtimeStateToMap(state persistentPluginRuntimeState) map[string]interface{} {
	out := map[string]interface{}{
		"available":      state.Available,
		"exists":         state.Exists,
		"plugin_id":      state.PluginID,
		"generation":     state.Generation,
		"loaded":         state.Loaded,
		"busy":           state.Busy,
		"boot_count":     state.BootCount,
		"total_requests": state.TotalRequests,
		"execute_count":  state.ExecuteCount,
		"eval_count":     state.EvalCount,
		"inspect_count":  state.InspectCount,
		"ref_count":      state.RefCount,
		"disposed":       state.Disposed,
		"current_action": state.CurrentAction,
		"last_action":    state.LastAction,
		"last_error":     state.LastError,
	}
	if state.InstanceID != "" {
		out["instance_id"] = state.InstanceID
	}
	if state.ScriptPath != "" {
		out["script_path"] = state.ScriptPath
	}
	if !state.CreatedAt.IsZero() {
		out["created_at"] = state.CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	if !state.LastUsedAt.IsZero() {
		out["last_used_at"] = state.LastUsedAt.UTC().Format(time.RFC3339Nano)
	}
	if !state.LastBootAt.IsZero() {
		out["last_boot_at"] = state.LastBootAt.UTC().Format(time.RFC3339Nano)
	}
	if len(state.CompletionPaths) > 0 {
		out["completion_paths"] = append([]string(nil), state.CompletionPaths...)
	}
	if len(state.CallablePaths) > 0 {
		out["callable_paths"] = append([]string(nil), state.CallablePaths...)
	}
	return out
}

func runtimeCompletionObjectForValue(vm *goja.Runtime, value goja.Value) *goja.Object {
	if vm == nil || value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
		return nil
	}
	if fn, ok := goja.AssertFunction(value); ok && fn != nil {
		return nil
	}
	switch value.Export().(type) {
	case bool, string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return nil
	}
	object := value.ToObject(vm)
	if object == nil {
		return nil
	}
	switch strings.TrimSpace(object.ClassName()) {
	case "", "Object":
		return object
	case "Array", "String", "Number", "Boolean", "Date", "RegExp", "Function":
		return nil
	default:
		return object
	}
}

func runtimeCompletionObjectKeys(object *goja.Object, includeOwnPropertyNames bool) []string {
	if object == nil {
		return nil
	}
	keys := append([]string(nil), object.Keys()...)
	if includeOwnPropertyNames {
		keys = append(keys, object.GetOwnPropertyNames()...)
	}
	if len(keys) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(keys))
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		normalized := strings.TrimSpace(key)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}

func isRuntimeConsoleBuiltinGlobalCallableName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "",
		"help",
		"keys",
		"runtimestate",
		"worker",
		"urlsearchparams",
		"textencoder",
		"textdecoder",
		"atob",
		"btoa",
		"commands",
		"permissions",
		"workspacestate",
		"inspect",
		"clearoutput",
		"structuredclone",
		"queuemicrotask",
		"settimeout",
		"cleartimeout",
		"require",
		"console",
		"plugin",
		"sandbox",
		"module",
		"exports",
		"globalthis",
		"object",
		"array",
		"json",
		"promise",
		"math",
		"date",
		"parseint",
		"parsefloat",
		"isnan",
		"isfinite",
		"encodeuri",
		"encodeuricomponent",
		"decodeuri",
		"decodeuricomponent",
		"eval":
		return true
	default:
		return false
	}
}

func isRuntimeConsoleStaticCompletionRoot(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "",
		"help",
		"keys",
		"runtimestate",
		"commands",
		"permissions",
		"workspacestate",
		"inspect",
		"clearoutput",
		"plugin",
		"sandbox",
		"console",
		"globalthis",
		"worker",
		"urlsearchparams",
		"textencoder",
		"textdecoder",
		"atob",
		"btoa",
		"structuredclone",
		"queuemicrotask",
		"settimeout",
		"cleartimeout",
		"module",
		"exports",
		"require",
		"__filename",
		"__dirname",
		"$_",
		"$1",
		"$2",
		"$3",
		"$4",
		"$5",
		"object",
		"array",
		"json",
		"promise",
		"math",
		"date",
		"function",
		"string",
		"number",
		"boolean",
		"regexp",
		"error",
		"typeerror",
		"rangeerror",
		"syntaxerror",
		"referenceerror",
		"urierror",
		"evalerror",
		"map",
		"set",
		"weakmap",
		"weakset",
		"symbol",
		"reflect",
		"proxy",
		"uint8array",
		"uint16array",
		"uint32array",
		"int8array",
		"int16array",
		"int32array",
		"float32array",
		"float64array",
		"arraybuffer",
		"dataview",
		"parseint",
		"parsefloat",
		"isnan",
		"isfinite",
		"encodeuri",
		"encodeuricomponent",
		"decodeuri",
		"decodeuricomponent",
		"eval",
		"nan",
		"infinity",
		"undefined":
		return true
	default:
		return false
	}
}

func isRuntimeConsoleRelevantDynamicGlobalName(name string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || isRuntimeConsoleStaticCompletionRoot(trimmed) {
		return false
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "process", "buffer":
		return false
	}
	if strings.HasPrefix(trimmed, "$") || strings.HasPrefix(trimmed, "_") {
		return len(trimmed) >= 4
	}
	if len(trimmed) <= 2 {
		return false
	}
	hasLower := false
	for _, ch := range trimmed {
		if ch >= 'a' && ch <= 'z' {
			hasLower = true
			break
		}
	}
	return hasLower
}

func (r *persistentPluginRuntime) snapshotCompletionPathsLocked() []string {
	if r == nil || r.vm == nil {
		return nil
	}

	seen := make(map[string]struct{})
	out := make([]string, 0, 96)
	addPath := func(path string) {
		normalized := strings.TrimSpace(path)
		if normalized == "" {
			return
		}
		if _, exists := seen[normalized]; exists {
			return
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	visited := make(map[*goja.Object]struct{})
	var addValuePaths func(prefix string, value goja.Value, depth int)
	var addObjectPaths func(prefix string, object *goja.Object, depth int)
	addObjectPaths = func(prefix string, object *goja.Object, depth int) {
		if object == nil || depth <= 0 {
			return
		}
		if _, exists := visited[object]; exists {
			return
		}
		visited[object] = struct{}{}
		defer delete(visited, object)
		keys := runtimeCompletionObjectKeys(object, true)
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key == "" || key == "__proto__" || strings.HasPrefix(key, "__auralogic") {
				continue
			}
			if (prefix == "module.exports" || prefix == "exports") && key == "workspace" {
				continue
			}
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			addPath(path)
			if depth <= 1 {
				continue
			}
			value, err := runtimeConsoleGetObjectProperty(object, key)
			if err != nil {
				continue
			}
			addValuePaths(path, value, depth-1)
		}
	}
	addValuePaths = func(prefix string, value goja.Value, depth int) {
		nextObject := runtimeCompletionObjectForValue(r.vm, value)
		if nextObject == nil {
			return
		}
		addObjectPaths(prefix, nextObject, depth)
	}

	global := r.vm.GlobalObject()
	if global != nil {
		keys := append([]string(nil), global.Keys()...)
		sort.Strings(keys)
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key == "" || strings.HasPrefix(key, "__auralogic") || !isRuntimeConsoleRelevantDynamicGlobalName(key) {
				continue
			}
			value, err := runtimeConsoleGetObjectProperty(global, key)
			if err != nil {
				continue
			}
			addPath(key)
			addPath("globalThis." + key)
			addValuePaths(key, value, 2)
			addValuePaths("globalThis."+key, value, 2)
		}
	}

	if r.entryModule != nil {
		moduleExports, err := runtimeConsoleGetObjectProperty(r.entryModule, "exports")
		if err == nil && moduleExports != nil && !goja.IsUndefined(moduleExports) && !goja.IsNull(moduleExports) {
			addPath("module.exports")
			addValuePaths("module.exports", moduleExports, 3)
			addPath("exports")
			addValuePaths("exports", moduleExports, 3)
		}
	}

	for _, alias := range r.workspaceAliasOrder {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		addPath(alias)
	}

	sort.Strings(out)
	return out
}

func (r *persistentPluginRuntime) snapshotCallablePathsLocked() []string {
	if r == nil || r.vm == nil {
		return nil
	}

	seen := make(map[string]struct{})
	out := make([]string, 0, 32)
	addPath := func(path string) {
		normalized := strings.TrimSpace(path)
		if normalized == "" {
			return
		}
		if _, exists := seen[normalized]; exists {
			return
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}

	visited := make(map[*goja.Object]struct{})
	var walkValue func(prefix string, value goja.Value, depth int)
	var walkObject func(prefix string, object *goja.Object, depth int)
	walkObject = func(prefix string, object *goja.Object, depth int) {
		if object == nil || depth <= 0 {
			return
		}
		if _, exists := visited[object]; exists {
			return
		}
		visited[object] = struct{}{}
		defer delete(visited, object)

		keys := runtimeCompletionObjectKeys(object, true)
		for _, key := range keys {
			if key == "__proto__" || strings.HasPrefix(key, "__auralogic") {
				continue
			}
			if (prefix == "module.exports" || prefix == "exports") && key == "workspace" {
				continue
			}
			value, err := runtimeConsoleGetObjectProperty(object, key)
			if err != nil {
				continue
			}
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			walkValue(path, value, depth-1)
		}
	}
	walkValue = func(prefix string, value goja.Value, depth int) {
		if depth < 0 || value == nil || goja.IsUndefined(value) || goja.IsNull(value) {
			return
		}
		if fn, ok := goja.AssertFunction(value); ok && fn != nil {
			addPath(prefix)
			return
		}
		object := runtimeCompletionObjectForValue(r.vm, value)
		if object == nil || depth == 0 {
			return
		}
		walkObject(prefix, object, depth)
	}

	global := r.vm.GlobalObject()
	if global != nil {
		keys := runtimeCompletionObjectKeys(global, false)
		for _, key := range keys {
			if strings.HasPrefix(key, "__auralogic") ||
				isRuntimeConsoleBuiltinGlobalCallableName(key) ||
				!isRuntimeConsoleRelevantDynamicGlobalName(key) {
				continue
			}
			value, err := runtimeConsoleGetObjectProperty(global, key)
			if err != nil {
				continue
			}
			if fn, ok := goja.AssertFunction(value); ok && fn != nil {
				addPath(key)
			}
		}
	}

	if r.entryModule != nil {
		if moduleExports, err := runtimeConsoleGetObjectProperty(r.entryModule, "exports"); err == nil {
			walkValue("module.exports", moduleExports, 4)
		}
	}

	sort.Strings(out)
	return out
}
