package jsworker

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"auralogic/internal/pluginipc"
	"github.com/dop251/goja"
)

type persistentPluginRuntime struct {
	key            persistentPluginRuntimeKey
	scriptPath     string
	baseOpts       workerOptions
	fsCtx          pluginFSRuntimeContext
	entryDir       string
	allowWorkers   bool
	workerBindings *pluginRuntimeWorkerBindings

	instanceID    string
	createdAt     time.Time
	lastUsedAt    time.Time
	lastBootAt    time.Time
	bootCount     int64
	totalRequests int64
	executeCount  int64
	streamCount   int64
	evalCount     int64
	inspectCount  int64
	lastAction    string
	lastError     string

	mu                    sync.Mutex
	vm                    *goja.Runtime
	moduleLoader          *commonJSLoader
	entryModule           *goja.Object
	loaded                bool
	sandboxObj            *goja.Object
	pluginObj             *goja.Object
	workspaceObj          *goja.Object
	storageObj            *goja.Object
	secretObj             *goja.Object
	webhookObj            *goja.Object
	httpObj               *goja.Object
	hostObj               *goja.Object
	fsObj                 *goja.Object
	workspaceAliasCatalog map[string]runtimeWorkspaceCommandHelpEntry
	workspaceAliasOrder   []string
	currentAction         string
	currentInvoke         *persistentPluginRuntimeInvocation

	asyncMu           sync.Mutex
	asyncTasks        []runtimeAsyncTask
	asyncWake         chan struct{}
	asyncStop         chan struct{}
	asyncClosed       bool
	asyncRetryPending bool
	timerSeq          int64
	timerOwners       map[int64]*persistentPluginRuntimeInvocation
}

type persistentPluginRuntimeInvocation struct {
	effectiveOpts   workerOptions
	sandboxCfg      pluginipc.SandboxConfig
	hostCfg         *pluginipc.HostAPIConfig
	timeout         time.Duration
	executionCtx    *pluginipc.ExecutionContext
	pluginConfig    map[string]interface{}
	storageState    *pluginStorageState
	secretSnapshot  map[string]string
	webhookState    persistentPluginRuntimeWebhookState
	execCtx         context.Context
	workspaceState  *pluginWorkspaceState
	pluginFSState   *pluginFS
	pluginFSError   error
	hostSessionStop func()
	workspaceHostOK bool
	deadline        time.Time
	workerGroup     *pluginRuntimeWorkerGroup
	timerMu         sync.Mutex
	timers          map[int64]*persistentPluginRuntimeTimer
	requestActive   bool
	asyncRefs       int
	closed          bool
}

type persistentPluginRuntimeTimer struct {
	timer *time.Timer
	done  chan struct{}
}

type persistentPluginRuntimeWebhookState struct {
	Enabled     bool
	Key         string
	Method      string
	Path        string
	QueryString string
	ContentType string
	RemoteAddr  string
	BodyText    string
	BodyBase64  string
	Headers     map[string]string
	QueryParams map[string]string
}

var persistentPluginRuntimeInstanceSequence atomic.Uint64

func nextPersistentPluginRuntimeInstanceID(key persistentPluginRuntimeKey) string {
	seq := persistentPluginRuntimeInstanceSequence.Add(1)
	return fmt.Sprintf(
		"jsrt_%d_%d_%d_%s",
		key.PluginID,
		key.Generation,
		seq,
		time.Now().UTC().Format("20060102T150405.000000000Z"),
	)
}

func newPersistentPluginRuntime(
	key persistentPluginRuntimeKey,
	scriptPath string,
	opts workerOptions,
	fsCtx pluginFSRuntimeContext,
	allowWorkers bool,
	workerBindings *pluginRuntimeWorkerBindings,
) (*persistentPluginRuntime, error) {
	scriptPath = normalizeScriptPath(scriptPath)
	if scriptPath == "" {
		return nil, fmt.Errorf("script path is required")
	}
	if !fileExists(scriptPath) {
		return nil, fmt.Errorf("script not found: %s", scriptPath)
	}

	moduleRoot := strings.TrimSpace(fsCtx.CodeRoot)
	if moduleRoot == "" {
		moduleRoot = detectModuleRoot(scriptPath)
		fsCtx.CodeRoot = moduleRoot
	}

	runtime := &persistentPluginRuntime{
		key:            key,
		scriptPath:     scriptPath,
		baseOpts:       opts,
		fsCtx:          fsCtx,
		entryDir:       filepath.Dir(scriptPath),
		allowWorkers:   allowWorkers,
		workerBindings: workerBindings,
		instanceID:     nextPersistentPluginRuntimeInstanceID(key),
		createdAt:      time.Now().UTC(),
		asyncWake:      make(chan struct{}, 1),
		asyncStop:      make(chan struct{}),
		timerOwners:    make(map[int64]*persistentPluginRuntimeInvocation),
	}
	if err := runtime.resetLocked(); err != nil {
		return nil, err
	}
	go runtime.runAsyncLoop()
	return runtime, nil
}

func (r *persistentPluginRuntime) matchesScript(scriptPath string) bool {
	if r == nil {
		return false
	}
	return r.scriptPath == normalizeScriptPath(scriptPath)
}

func (r *persistentPluginRuntime) close() {
	if r == nil {
		return
	}
	invocationSet := make(map[*persistentPluginRuntimeInvocation]struct{}, 8)
	r.mu.Lock()
	if r.currentInvoke != nil {
		invocationSet[r.currentInvoke] = struct{}{}
	}
	r.currentInvoke = nil
	if r.vm != nil {
		r.vm.ClearInterrupt()
	}
	r.currentAction = ""
	r.mu.Unlock()

	r.asyncMu.Lock()
	if !r.asyncClosed {
		r.asyncClosed = true
		close(r.asyncStop)
	}
	for _, task := range r.asyncTasks {
		if task.invocation != nil {
			invocationSet[task.invocation] = struct{}{}
		}
	}
	r.asyncTasks = nil
	for _, invocation := range r.timerOwners {
		if invocation != nil {
			invocationSet[invocation] = struct{}{}
		}
	}
	r.timerOwners = nil
	r.asyncMu.Unlock()

	for invocation := range invocationSet {
		invocation.forceClose()
	}
}

func (r *persistentPluginRuntime) resetLocked() error {
	if r == nil {
		return fmt.Errorf("persistent runtime is unavailable")
	}
	now := time.Now().UTC()

	vm := goja.New()
	moduleLoader := newCommonJSLoader(vm, r.scriptPath)
	entryModule := vm.NewObject()
	entryExports := vm.NewObject()
	_ = entryModule.Set("exports", entryExports)

	_ = vm.Set("require", moduleLoader.makeRequire(r.entryDir))
	_ = vm.Set("module", entryModule)
	_ = vm.Set("exports", entryExports)
	_ = vm.Set("__filename", filepath.ToSlash(r.scriptPath))
	_ = vm.Set("__dirname", filepath.ToSlash(r.entryDir))

	r.vm = vm
	r.moduleLoader = moduleLoader
	r.entryModule = entryModule
	r.loaded = false
	r.sandboxObj = nil
	r.pluginObj = nil
	r.workspaceObj = nil
	r.storageObj = nil
	r.secretObj = nil
	r.webhookObj = nil
	r.httpObj = nil
	r.hostObj = nil
	r.fsObj = nil
	r.workspaceAliasCatalog = nil
	r.workspaceAliasOrder = nil
	r.currentAction = ""
	r.currentInvoke = nil
	r.lastBootAt = now
	r.bootCount++

	if err := r.installGlobals(); err != nil {
		return err
	}
	return nil
}

func newPersistentPluginRuntimeInvocation(
	baseOpts workerOptions,
	sandboxCfg pluginipc.SandboxConfig,
	hostCfg *pluginipc.HostAPIConfig,
	timeout time.Duration,
	executionCtx *pluginipc.ExecutionContext,
	pluginConfig map[string]interface{},
	storageSnapshot map[string]string,
	secretSnapshot map[string]string,
	webhookReq *pluginipc.WebhookRequest,
	execCtx context.Context,
	fsCtx pluginFSRuntimeContext,
	workspaceState *pluginWorkspaceState,
) *persistentPluginRuntimeInvocation {
	effectiveOpts := applyRequestSandbox(baseOpts, sandboxCfg)
	if execCtx == nil {
		execCtx = context.Background()
	}
	if timeout <= 0 {
		timeout = time.Duration(effectiveOpts.timeoutMs) * time.Millisecond
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	invocation := &persistentPluginRuntimeInvocation{
		effectiveOpts:   effectiveOpts,
		sandboxCfg:      sandboxCfg,
		hostCfg:         clonePluginHostAPIConfig(hostCfg),
		timeout:         timeout,
		executionCtx:    clonePluginIPCExecutionContext(executionCtx),
		pluginConfig:    clonePluginConfigMap(pluginConfig),
		storageState:    newPluginStorageState(storageSnapshot, pluginStorageLimitsFromOptions(effectiveOpts)),
		secretSnapshot:  mergeStringMaps(secretSnapshot, nil),
		webhookState:    buildPersistentPluginRuntimeWebhookState(webhookReq),
		execCtx:         execCtx,
		workspaceState:  workspaceState,
		workspaceHostOK: workspaceState != nil && isWorkerWorkspaceForwardAction(sandboxCfg.CurrentAction) && hostCfg != nil && strings.TrimSpace(workspaceState.commandID) != "",
		deadline:        time.Now().UTC().Add(timeout),
		requestActive:   true,
	}
	invocation.hostSessionStop = attachPluginHostSession(invocation.hostCfg)
	invocation.pluginFSState, invocation.pluginFSError = newPluginFS(fsCtx, effectiveOpts.fsMaxFiles, effectiveOpts.fsMaxTotalBytes, effectiveOpts.fsMaxReadBytes)
	return invocation
}

func buildPersistentPluginRuntimeWebhookState(webhookReq *pluginipc.WebhookRequest) persistentPluginRuntimeWebhookState {
	state := persistentPluginRuntimeWebhookState{
		Headers:     map[string]string{},
		QueryParams: map[string]string{},
	}
	if webhookReq == nil {
		return state
	}

	state.Enabled = true
	state.Key = strings.TrimSpace(webhookReq.Key)
	state.Method = strings.TrimSpace(webhookReq.Method)
	state.Path = strings.TrimSpace(webhookReq.Path)
	state.QueryString = strings.TrimSpace(webhookReq.QueryString)
	state.ContentType = strings.TrimSpace(webhookReq.ContentType)
	state.RemoteAddr = strings.TrimSpace(webhookReq.RemoteAddr)
	state.BodyText = webhookReq.BodyText
	state.BodyBase64 = strings.TrimSpace(webhookReq.BodyBase64)
	for key, value := range webhookReq.Headers {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		if normalizedKey == "" {
			continue
		}
		state.Headers[normalizedKey] = value
	}
	for key, value := range webhookReq.QueryParams {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		state.QueryParams[normalizedKey] = value
	}
	return state
}

func clonePluginIPCExecutionContext(ctx *pluginipc.ExecutionContext) *pluginipc.ExecutionContext {
	if ctx == nil {
		return nil
	}
	return &pluginipc.ExecutionContext{
		UserID:    ctx.UserID,
		OrderID:   ctx.OrderID,
		SessionID: strings.TrimSpace(ctx.SessionID),
		Metadata:  mergeStringMaps(ctx.Metadata, nil),
	}
}

func clonePluginConfigMap(value map[string]interface{}) map[string]interface{} {
	if len(value) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(value))
	for key, item := range value {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		out[normalizedKey] = item
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func clonePluginHostAPIConfig(hostCfg *pluginipc.HostAPIConfig) *pluginipc.HostAPIConfig {
	if hostCfg == nil {
		return nil
	}
	cloned := *hostCfg
	return &cloned
}

func (r *persistentPluginRuntime) execute(
	functionName string,
	args []interface{},
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
) (goja.Value, map[string]string, bool, string, error) {
	invocation := newPersistentPluginRuntimeInvocation(r.baseOpts, sandboxCfg, hostCfg, timeout, executionCtx, pluginConfig, storageSnapshot, secretSnapshot, webhookReq, execCtx, r.fsCtx, workspaceState)
	return r.runWithInvocation(invocation, func(vm *goja.Runtime, entryModule *goja.Object) (goja.Value, error) {
		fn, thisArg, resolveErr := resolveScriptFunction(vm, entryModule, functionName)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if fn == nil {
			return nil, errFunctionNotFound
		}
		return callScriptFunction(vm, fn, thisArg, functionName, args)
	})
}

func (r *persistentPluginRuntime) executeStream(
	args []interface{},
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
	emit func(map[string]interface{}, map[string]string) error,
) (goja.Value, map[string]string, bool, string, error) {
	invocation := newPersistentPluginRuntimeInvocation(r.baseOpts, sandboxCfg, hostCfg, timeout, executionCtx, pluginConfig, storageSnapshot, secretSnapshot, webhookReq, execCtx, r.fsCtx, workspaceState)
	return r.runWithInvocation(invocation, func(vm *goja.Runtime, entryModule *goja.Object) (goja.Value, error) {
		streamWriter := newPluginStreamWriter(vm, emit)

		fn, thisArg, resolveErr := resolveScriptFunction(vm, entryModule, "executeStream")
		if resolveErr == nil && fn != nil {
			return callScriptFunction(vm, fn, thisArg, "executeStream", append(args, streamWriter))
		}
		if resolveErr != nil && !errors.Is(resolveErr, errFunctionNotFound) {
			return nil, resolveErr
		}

		fn, thisArg, resolveErr = resolveScriptFunction(vm, entryModule, "execute")
		if resolveErr != nil {
			return nil, resolveErr
		}
		if fn == nil {
			return nil, errFunctionNotFound
		}
		return callScriptFunction(vm, fn, thisArg, "execute", append(args, streamWriter))
	})
}

func (r *persistentPluginRuntime) runWithInvocation(
	invocation *persistentPluginRuntimeInvocation,
	invoke func(vm *goja.Runtime, entryModule *goja.Object) (goja.Value, error),
) (goja.Value, map[string]string, bool, string, error) {
	if r == nil || r.vm == nil {
		return nil, nil, false, storageAccessNone, fmt.Errorf("persistent runtime is unavailable")
	}
	if invocation == nil {
		return nil, nil, false, storageAccessNone, fmt.Errorf("runtime invocation is nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.vm.ClearInterrupt()
	r.currentInvoke = invocation
	action := strings.ToLower(strings.TrimSpace(invocation.sandboxCfg.CurrentAction))
	if action == "" {
		action = "execute"
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
		return nil, invocation.storageState.snapshot(), invocation.storageState.changed, invocation.storageState.accessMode(), err
	}
	if err := r.ensureLoaded(); err != nil {
		return nil, invocation.storageState.snapshot(), invocation.storageState.changed, invocation.storageState.accessMode(), err
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
		value goja.Value
		err   error
	)
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("panic in script runtime: %v", recovered)
				r.lastError = err.Error()
				_ = r.resetLocked()
			}
		}()
		value, err = invoke(r.vm, r.entryModule)
		if err != nil {
			return
		}
		value, err = r.resolveAsyncResultLocked(invocation, value)
	}()
	if r.loaded {
		r.refreshWorkspaceCommandAliasesLocked()
	}

	stopMemoryMonitor()
	if memoryExceeded.Load() {
		err = fmt.Errorf("%w (limit=%dMB)", errExecutionMemoryLimitExceeded, invocation.effectiveOpts.maxMemoryMB)
		r.lastError = err.Error()
		return nil, invocation.storageState.snapshot(), invocation.storageState.changed, invocation.storageState.accessMode(), err
	}
	if err != nil {
		r.lastError = err.Error()
		return nil, invocation.storageState.snapshot(), invocation.storageState.changed, invocation.storageState.accessMode(), err
	}
	r.lastError = ""
	return value, invocation.storageState.snapshot(), invocation.storageState.changed, invocation.storageState.accessMode(), nil
}

func (r *persistentPluginRuntime) ensureLoaded() error {
	if r.loaded {
		return nil
	}
	program, err := loadWorkerProgram(r.scriptPath, workerProgramWrapperNone)
	if err != nil {
		return err
	}
	if _, err := r.vm.RunProgram(program); err != nil {
		r.lastError = err.Error()
		_ = r.resetLocked()
		return fmt.Errorf("run script failed: %w", err)
	}
	r.refreshWorkspaceCommandAliasesLocked()
	r.loaded = true
	return nil
}

func (r *persistentPluginRuntime) bootRequestStatsLocked(action string) {
	r.totalRequests++
	switch action {
	case "workspace.runtime.eval":
		r.evalCount++
	case "workspace.runtime.inspect":
		r.inspectCount++
	case "execute_stream":
		r.streamCount++
	default:
		r.executeCount++
	}
}
