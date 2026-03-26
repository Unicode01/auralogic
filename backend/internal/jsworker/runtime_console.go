package jsworker

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"auralogic/internal/pluginipc"
	"github.com/dop251/goja"
)

const (
	defaultRuntimeConsoleInspectDepth  = 2
	maxRuntimeConsoleInspectDepth      = 4
	defaultRuntimeConsolePreviewLimit  = 24
	defaultRuntimeConsoleStringMaxRune = 160
	workspaceConsolePreviewsJSONKey    = "workspace_console_previews_json"
)

type runtimeConsolePreview struct {
	Type      string
	ClassName string
	Summary   string
	Value     interface{}
	Length    int
	Keys      []string
	Entries   []runtimeConsolePreviewEntry
	Truncated bool
}

type runtimeConsolePreviewEntry struct {
	Key   string
	Value runtimeConsolePreview
}

type runtimeConsolePreviewConfig struct {
	Depth          int
	MaxEntries     int
	MaxStringRunes int
}

type runtimeConsoleHelperRewrite struct {
	pattern    *regexp.Regexp
	hiddenName string
}

var runtimeConsoleHelperRewrites = []runtimeConsoleHelperRewrite{
	newRuntimeConsoleHelperRewrite("help", runtimeConsoleHelperHelpName),
	newRuntimeConsoleHelperRewrite("keys", runtimeConsoleHelperKeysName),
	newRuntimeConsoleHelperRewrite("runtimeState", runtimeConsoleHelperRuntimeStateName),
	newRuntimeConsoleHelperRewrite("commands", runtimeConsoleHelperCommandsName),
	newRuntimeConsoleHelperRewrite("permissions", runtimeConsoleHelperPermissionsName),
	newRuntimeConsoleHelperRewrite("workspaceState", runtimeConsoleHelperWorkspaceName),
	newRuntimeConsoleHelperRewrite("inspect", runtimeConsoleHelperInspectName),
	newRuntimeConsoleHelperRewrite("clearOutput", runtimeConsoleHelperClearOutputName),
}

func newRuntimeConsoleHelperRewrite(publicName string, hiddenName string) runtimeConsoleHelperRewrite {
	return runtimeConsoleHelperRewrite{
		pattern:    regexp.MustCompile(`(^|[^.$\w])` + regexp.QuoteMeta(publicName) + `(\s*\()`),
		hiddenName: hiddenName,
	}
}

func normalizeRuntimeConsoleInspectDepth(depth int) int {
	if depth <= 0 {
		return defaultRuntimeConsoleInspectDepth
	}
	if depth > maxRuntimeConsoleInspectDepth {
		return maxRuntimeConsoleInspectDepth
	}
	return depth
}

func defaultRuntimeConsolePreviewConfig(depth int) runtimeConsolePreviewConfig {
	return runtimeConsolePreviewConfig{
		Depth:          normalizeRuntimeConsoleInspectDepth(depth),
		MaxEntries:     defaultRuntimeConsolePreviewLimit,
		MaxStringRunes: defaultRuntimeConsoleStringMaxRune,
	}
}

func (r *persistentPluginRuntime) evaluateRuntime(
	code string,
	sandboxCfg pluginipc.SandboxConfig,
	hostCfg *pluginipc.HostAPIConfig,
	timeout time.Duration,
	executionCtx *pluginipc.ExecutionContext,
	pluginConfig map[string]interface{},
	storageSnapshot map[string]string,
	secretSnapshot map[string]string,
	webhookReq *pluginipc.WebhookRequest,
	execCtx context.Context,
	workspaceState *pluginWorkspaceState,
) (runtimeConsolePreview, map[string]string, bool, string, error) {
	rewrittenCode := rewriteRuntimeConsoleHelperCalls(code)
	if strings.TrimSpace(rewrittenCode) == "" {
		return runtimeConsolePreview{}, nil, false, storageAccessNone, fmt.Errorf("runtime_code is required")
	}
	invocation := newPersistentPluginRuntimeInvocation(
		r.baseOpts,
		sandboxCfg,
		hostCfg,
		timeout,
		executionCtx,
		pluginConfig,
		storageSnapshot,
		secretSnapshot,
		webhookReq,
		execCtx,
		r.fsCtx,
		workspaceState,
	)
	return r.runWithInvocationPreview(invocation, defaultRuntimeConsoleInspectDepth, func(vm *goja.Runtime, _ *goja.Object) (goja.Value, error) {
		return vm.RunString(rewrittenCode)
	})
}

func (r *persistentPluginRuntime) inspectRuntime(
	expression string,
	depth int,
	sandboxCfg pluginipc.SandboxConfig,
	hostCfg *pluginipc.HostAPIConfig,
	timeout time.Duration,
	executionCtx *pluginipc.ExecutionContext,
	pluginConfig map[string]interface{},
	storageSnapshot map[string]string,
	secretSnapshot map[string]string,
	webhookReq *pluginipc.WebhookRequest,
	execCtx context.Context,
	workspaceState *pluginWorkspaceState,
) (runtimeConsolePreview, map[string]string, bool, string, error) {
	trimmed := strings.TrimSpace(rewriteRuntimeConsoleHelperCalls(expression))
	if trimmed == "" {
		trimmed = "globalThis"
	}
	invocation := newPersistentPluginRuntimeInvocation(
		r.baseOpts,
		sandboxCfg,
		hostCfg,
		timeout,
		executionCtx,
		pluginConfig,
		storageSnapshot,
		secretSnapshot,
		webhookReq,
		execCtx,
		r.fsCtx,
		workspaceState,
	)
	return r.runWithInvocationPreview(invocation, depth, func(vm *goja.Runtime, _ *goja.Object) (goja.Value, error) {
		return vm.RunString("(" + trimmed + "\n)")
	})
}

func rewriteRuntimeConsoleHelperCalls(line string) string {
	if strings.TrimSpace(line) == "" {
		return line
	}
	rewritten := line
	for _, helper := range runtimeConsoleHelperRewrites {
		if helper.pattern == nil || helper.hiddenName == "" {
			continue
		}
		rewritten = helper.pattern.ReplaceAllString(rewritten, `${1}`+helper.hiddenName+`${2}`)
	}
	return rewritten
}

func (r *persistentPluginRuntime) runWithInvocationPreview(
	invocation *persistentPluginRuntimeInvocation,
	depth int,
	invoke func(vm *goja.Runtime, entryModule *goja.Object) (goja.Value, error),
) (runtimeConsolePreview, map[string]string, bool, string, error) {
	if r == nil || r.vm == nil {
		return runtimeConsolePreview{}, nil, false, storageAccessNone, fmt.Errorf("persistent runtime is unavailable")
	}
	if invocation == nil {
		return runtimeConsolePreview{}, nil, false, storageAccessNone, fmt.Errorf("runtime invocation is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.vm.ClearInterrupt()
	r.currentInvoke = invocation
	action := strings.ToLower(strings.TrimSpace(invocation.sandboxCfg.CurrentAction))
	if action == "" {
		action = "workspace.runtime.eval"
	}
	r.currentAction = action
	r.lastUsedAt = time.Now().UTC()
	r.bootRequestStatsLocked(action)
	defer func() {
		invocation.endRequest()
		r.currentInvoke = nil
		r.currentAction = ""
		r.lastAction = action
		r.lastUsedAt = time.Now().UTC()
		if r.vm != nil {
			r.vm.ClearInterrupt()
		}
		r.wakeAsyncLoop()
	}()

	if err := r.applyInvocation(invocation); err != nil {
		return runtimeConsolePreview{}, invocation.storageState.snapshot(), invocation.storageState.changed, invocation.storageState.accessMode(), err
	}
	if err := r.ensureLoaded(); err != nil {
		return runtimeConsolePreview{}, invocation.storageState.snapshot(), invocation.storageState.changed, invocation.storageState.accessMode(), err
	}

	var memoryExceeded atomic.Bool
	stopMemoryMonitor := func() {}
	if invocation.effectiveOpts.maxMemoryMB > 0 {
		limitBytes := uint64(invocation.effectiveOpts.maxMemoryMB) * 1024 * 1024
		stopMemoryMonitor = startExecutionMemoryMonitor(limitBytes, memoryMonitorInterval, readRuntimeHeapAlloc, func(_ uint64) {
			memoryExceeded.Store(true)
			r.vm.Interrupt("execution memory limit exceeded")
		})
	}

	timer := time.AfterFunc(invocation.timeout, func() {
		r.vm.Interrupt("execution timeout")
	})
	defer timer.Stop()

	stopContextInterrupt := context.AfterFunc(invocation.execCtx, func() {
		if err := invocation.execCtx.Err(); err != nil {
			r.vm.Interrupt(err)
		}
	})
	defer stopContextInterrupt()

	var (
		preview runtimeConsolePreview
		err     error
	)
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("panic in script runtime: %v", recovered)
				r.lastError = err.Error()
				_ = r.resetLocked()
			}
		}()
		value, invokeErr := invoke(r.vm, r.entryModule)
		if invokeErr != nil {
			err = invokeErr
			return
		}
		value, err = r.resolveAsyncResultLocked(invocation, value)
		if err != nil {
			return
		}
		preview = buildRuntimeConsolePreview(r.vm, value, depth)
		r.rememberRuntimeConsoleResult(value)
	}()
	if r.loaded {
		r.refreshWorkspaceCommandAliasesLocked()
	}

	stopMemoryMonitor()
	if memoryExceeded.Load() {
		err = fmt.Errorf("%w (limit=%dMB)", errExecutionMemoryLimitExceeded, invocation.effectiveOpts.maxMemoryMB)
		r.lastError = err.Error()
		return runtimeConsolePreview{}, invocation.storageState.snapshot(), invocation.storageState.changed, invocation.storageState.accessMode(), err
	}
	if err != nil {
		r.lastError = err.Error()
		return runtimeConsolePreview{}, invocation.storageState.snapshot(), invocation.storageState.changed, invocation.storageState.accessMode(), err
	}
	r.lastError = ""
	return preview, invocation.storageState.snapshot(), invocation.storageState.changed, invocation.storageState.accessMode(), nil
}

func buildRuntimeConsolePreview(vm *goja.Runtime, value goja.Value, depth int) runtimeConsolePreview {
	cfg := defaultRuntimeConsolePreviewConfig(depth)
	return buildRuntimeConsolePreviewWithState(vm, value, cfg.Depth, cfg, make(map[*goja.Object]struct{}, 8))
}

func buildRuntimeConsolePreviewWithState(
	vm *goja.Runtime,
	value goja.Value,
	depth int,
	cfg runtimeConsolePreviewConfig,
	seen map[*goja.Object]struct{},
) runtimeConsolePreview {
	if value == nil || goja.IsUndefined(value) {
		return runtimeConsolePreview{
			Type:    "undefined",
			Summary: "undefined",
		}
	}
	if goja.IsNull(value) {
		return runtimeConsolePreview{
			Type:    "null",
			Summary: "null",
		}
	}
	if _, ok := goja.AssertFunction(value); ok {
		return buildRuntimeConsoleFunctionPreview(vm, value)
	}

	switch typed := value.Export().(type) {
	case bool:
		return runtimeConsolePreview{
			Type:    "boolean",
			Summary: strconv.FormatBool(typed),
			Value:   typed,
		}
	case string:
		return buildRuntimeConsoleStringPreview(typed, cfg)
	case int:
		return buildRuntimeConsoleNumberPreview(typed)
	case int8:
		return buildRuntimeConsoleNumberPreview(typed)
	case int16:
		return buildRuntimeConsoleNumberPreview(typed)
	case int32:
		return buildRuntimeConsoleNumberPreview(typed)
	case int64:
		return buildRuntimeConsoleNumberPreview(typed)
	case uint:
		return buildRuntimeConsoleNumberPreview(typed)
	case uint8:
		return buildRuntimeConsoleNumberPreview(typed)
	case uint16:
		return buildRuntimeConsoleNumberPreview(typed)
	case uint32:
		return buildRuntimeConsoleNumberPreview(typed)
	case uint64:
		return buildRuntimeConsoleNumberPreview(typed)
	case float32:
		return buildRuntimeConsoleNumberPreview(typed)
	case float64:
		return buildRuntimeConsoleNumberPreview(typed)
	}

	obj := value.ToObject(vm)
	if obj == nil {
		summary := strings.TrimSpace(value.String())
		if summary == "" {
			summary = "<value>"
		}
		return runtimeConsolePreview{
			Type:    "value",
			Summary: summary,
		}
	}
	if _, exists := seen[obj]; exists {
		return runtimeConsolePreview{
			Type:    "circular",
			Summary: "[Circular]",
		}
	}
	seen[obj] = struct{}{}
	defer delete(seen, obj)

	className := strings.TrimSpace(obj.ClassName())
	switch className {
	case "Array":
		return buildRuntimeConsoleArrayPreview(vm, obj, depth, cfg, seen)
	case "Date":
		return runtimeConsolePreview{
			Type:      "date",
			ClassName: className,
			Summary:   value.String(),
		}
	}
	if className == "Function" {
		return buildRuntimeConsoleFunctionPreview(vm, value)
	}
	if strings.HasSuffix(className, "Error") || className == "Error" {
		return buildRuntimeConsoleObjectPreview(vm, obj, className, "error", depth, cfg, seen)
	}
	return buildRuntimeConsoleObjectPreview(vm, obj, className, "object", depth, cfg, seen)
}

func buildRuntimeConsoleStringPreview(value string, cfg runtimeConsolePreviewConfig) runtimeConsolePreview {
	quoted, truncated := quoteRuntimeConsoleString(value, cfg.MaxStringRunes)
	return runtimeConsolePreview{
		Type:      "string",
		Summary:   quoted,
		Value:     value,
		Length:    len([]rune(value)),
		Truncated: truncated,
	}
}

func buildRuntimeConsoleNumberPreview(value interface{}) runtimeConsolePreview {
	return runtimeConsolePreview{
		Type:    "number",
		Summary: formatRuntimeConsoleNumber(value),
		Value:   value,
	}
}

func buildRuntimeConsoleFunctionPreview(vm *goja.Runtime, value goja.Value) runtimeConsolePreview {
	name := ""
	if obj := value.ToObject(vm); obj != nil {
		if rawName, err := runtimeConsoleGetObjectProperty(obj, "name"); err == nil && rawName != nil && !goja.IsUndefined(rawName) && !goja.IsNull(rawName) {
			name = strings.TrimSpace(rawName.String())
		}
	}
	summary := "[Function]"
	if name != "" {
		summary = "[Function " + name + "]"
	}
	return runtimeConsolePreview{
		Type:      "function",
		ClassName: "Function",
		Summary:   summary,
	}
}

func buildRuntimeConsoleArrayPreview(
	vm *goja.Runtime,
	obj *goja.Object,
	depth int,
	cfg runtimeConsolePreviewConfig,
	seen map[*goja.Object]struct{},
) runtimeConsolePreview {
	length := runtimeConsoleArrayLength(obj)
	preview := runtimeConsolePreview{
		Type:      "array",
		ClassName: "Array",
		Length:    length,
	}
	if depth <= 0 {
		preview.Summary = fmt.Sprintf("[Array(%d)]", length)
		return preview
	}

	limit := length
	if cfg.MaxEntries > 0 && limit > cfg.MaxEntries {
		limit = cfg.MaxEntries
		preview.Truncated = true
	}
	entries := make([]runtimeConsolePreviewEntry, 0, limit)
	for index := 0; index < limit; index++ {
		rawValue, err := runtimeConsoleGetObjectProperty(obj, strconv.Itoa(index))
		if err != nil {
			entries = append(entries, runtimeConsolePreviewEntry{
				Key: strconv.Itoa(index),
				Value: runtimeConsolePreview{
					Type:    "error",
					Summary: "[Thrown: " + err.Error() + "]",
				},
			})
			continue
		}
		entries = append(entries, runtimeConsolePreviewEntry{
			Key:   strconv.Itoa(index),
			Value: buildRuntimeConsolePreviewWithState(vm, rawValue, depth-1, cfg, seen),
		})
	}
	preview.Entries = entries
	preview.Summary = formatRuntimeConsolePreview(preview)
	return preview
}

func buildRuntimeConsoleObjectPreview(
	vm *goja.Runtime,
	obj *goja.Object,
	className string,
	previewType string,
	depth int,
	cfg runtimeConsolePreviewConfig,
	seen map[*goja.Object]struct{},
) runtimeConsolePreview {
	keys := obj.Keys()
	preview := runtimeConsolePreview{
		Type:      previewType,
		ClassName: className,
	}
	if len(keys) > 0 {
		preview.Keys = append([]string(nil), keys...)
	}

	errorSummary := runtimeConsoleErrorSummary(obj, className)
	if depth <= 0 {
		switch {
		case errorSummary != "":
			preview.Summary = errorSummary
		case len(keys) == 0:
			if className != "" && className != "Object" {
				preview.Summary = className + " {}"
			} else {
				preview.Summary = "{}"
			}
		case className != "" && className != "Object":
			preview.Summary = className + " {...}"
		default:
			preview.Summary = "{...}"
		}
		return preview
	}

	limit := len(keys)
	if cfg.MaxEntries > 0 && limit > cfg.MaxEntries {
		limit = cfg.MaxEntries
		preview.Truncated = true
	}
	entries := make([]runtimeConsolePreviewEntry, 0, limit)
	for _, key := range keys[:limit] {
		rawValue, err := runtimeConsoleGetObjectProperty(obj, key)
		if err != nil {
			entries = append(entries, runtimeConsolePreviewEntry{
				Key: key,
				Value: runtimeConsolePreview{
					Type:    "error",
					Summary: "[Thrown: " + err.Error() + "]",
				},
			})
			continue
		}
		entries = append(entries, runtimeConsolePreviewEntry{
			Key:   key,
			Value: buildRuntimeConsolePreviewWithState(vm, rawValue, depth-1, cfg, seen),
		})
	}
	preview.Entries = entries
	if errorSummary != "" {
		preview.Summary = errorSummary
		return preview
	}
	preview.Summary = formatRuntimeConsolePreview(preview)
	return preview
}

func runtimeConsoleArrayLength(obj *goja.Object) int {
	if obj == nil {
		return 0
	}
	lengthValue, err := runtimeConsoleGetObjectProperty(obj, "length")
	if err != nil || lengthValue == nil || goja.IsUndefined(lengthValue) || goja.IsNull(lengthValue) {
		return 0
	}
	return int(lengthValue.ToInteger())
}

func runtimeConsoleErrorSummary(obj *goja.Object, className string) string {
	if obj == nil {
		return ""
	}
	if className != "Error" && !strings.HasSuffix(className, "Error") {
		return ""
	}
	messageValue, err := runtimeConsoleGetObjectProperty(obj, "message")
	if err != nil || messageValue == nil || goja.IsUndefined(messageValue) || goja.IsNull(messageValue) {
		if className == "" {
			return "Error"
		}
		return className
	}
	message := strings.TrimSpace(messageValue.String())
	if message == "" {
		if className == "" {
			return "Error"
		}
		return className
	}
	if className == "" {
		className = "Error"
	}
	return className + ": " + message
}

func runtimeConsoleGetObjectProperty(obj *goja.Object, key string) (value goja.Value, err error) {
	if obj == nil {
		return nil, fmt.Errorf("object is unavailable")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%v", recovered)
		}
	}()
	return obj.Get(key), nil
}

func truncateRuntimeConsoleString(value string, maxRunes int) (string, bool) {
	if maxRunes <= 0 {
		return value, false
	}
	runes := []rune(value)
	if len(runes) <= maxRunes {
		return value, false
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes]), true
	}
	return string(runes[:maxRunes-3]) + "...", true
}

func quoteRuntimeConsoleString(value string, maxRunes int) (string, bool) {
	truncatedValue, truncated := truncateRuntimeConsoleString(value, maxRunes)
	return strconv.Quote(truncatedValue), truncated
}

func formatRuntimeConsoleNumber(value interface{}) string {
	switch typed := value.(type) {
	case int:
		return strconv.Itoa(typed)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 64)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", value)
	}
}

func formatRuntimeConsolePreview(preview runtimeConsolePreview) string {
	switch preview.Type {
	case "undefined", "null", "boolean", "number", "string", "function", "date", "circular":
		if preview.Summary != "" {
			return preview.Summary
		}
		return "<value>"
	case "error":
		if preview.Summary != "" {
			return preview.Summary
		}
		return "Error"
	case "array":
		if len(preview.Entries) == 0 {
			if preview.Summary != "" {
				return preview.Summary
			}
			return "[]"
		}
		parts := make([]string, 0, len(preview.Entries)+1)
		for _, entry := range preview.Entries {
			parts = append(parts, formatRuntimeConsolePreview(entry.Value))
		}
		if preview.Truncated {
			parts = append(parts, "...")
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case "object":
		if len(preview.Entries) == 0 {
			if preview.Summary != "" {
				return preview.Summary
			}
			if preview.ClassName != "" && preview.ClassName != "Object" {
				return preview.ClassName + " {}"
			}
			return "{}"
		}
		parts := make([]string, 0, len(preview.Entries)+1)
		for _, entry := range preview.Entries {
			parts = append(parts, entry.Key+": "+formatRuntimeConsolePreview(entry.Value))
		}
		if preview.Truncated {
			parts = append(parts, "...")
		}
		prefix := ""
		if preview.ClassName != "" && preview.ClassName != "Object" {
			prefix = preview.ClassName + " "
		}
		return prefix + "{ " + strings.Join(parts, ", ") + " }"
	default:
		if preview.Summary != "" {
			return preview.Summary
		}
		return "<value>"
	}
}

func runtimeConsolePreviewToMap(preview runtimeConsolePreview) map[string]interface{} {
	out := map[string]interface{}{
		"type":    preview.Type,
		"summary": preview.Summary,
	}
	if preview.ClassName != "" {
		out["class_name"] = preview.ClassName
	}
	switch preview.Type {
	case "boolean", "number", "string":
		out["value"] = preview.Value
	}
	if preview.Type == "string" || preview.Type == "array" {
		out["length"] = preview.Length
	}
	if len(preview.Keys) > 0 {
		out["keys"] = append([]string(nil), preview.Keys...)
	}
	if len(preview.Entries) > 0 {
		entries := make([]map[string]interface{}, 0, len(preview.Entries))
		for _, entry := range preview.Entries {
			entries = append(entries, map[string]interface{}{
				"key":   entry.Key,
				"value": runtimeConsolePreviewToMap(entry.Value),
			})
		}
		out["entries"] = entries
	}
	if preview.Truncated {
		out["truncated"] = true
	}
	return out
}

func buildRuntimeConsoleCallOutput(
	vm *goja.Runtime,
	arguments []goja.Value,
) (string, map[string]string) {
	if vm == nil || len(arguments) == 0 {
		return "", nil
	}

	messageParts := make([]string, 0, len(arguments))
	serializedPreviews := make([]map[string]interface{}, 0, len(arguments))
	for _, argument := range arguments {
		preview := buildRuntimeConsolePreview(vm, argument, defaultRuntimeConsoleInspectDepth)
		messageParts = append(messageParts, formatRuntimeConsoleCallPreview(preview))
		serializedPreviews = append(serializedPreviews, runtimeConsolePreviewToMap(preview))
	}

	metadata := map[string]string{}
	if payload, err := json.Marshal(serializedPreviews); err == nil && len(payload) > 0 {
		metadata[workspaceConsolePreviewsJSONKey] = string(payload)
	}
	if len(metadata) == 0 {
		metadata = nil
	}

	return strings.Join(messageParts, " "), metadata
}

func formatRuntimeConsoleLogOutput(arguments []goja.Value) string {
	if len(arguments) == 0 {
		return ""
	}

	parts := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		switch {
		case argument == nil || goja.IsUndefined(argument):
			parts = append(parts, "undefined")
		case goja.IsNull(argument):
			parts = append(parts, "null")
		default:
			parts = append(parts, argument.String())
		}
	}
	return strings.Join(parts, " ")
}

func formatRuntimeConsoleCallPreview(preview runtimeConsolePreview) string {
	switch preview.Type {
	case "string":
		if typed, ok := preview.Value.(string); ok {
			return typed
		}
	case "undefined":
		return "undefined"
	}
	if strings.TrimSpace(preview.Summary) != "" {
		return preview.Summary
	}
	return "<value>"
}
