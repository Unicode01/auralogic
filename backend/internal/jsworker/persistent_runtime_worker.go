package jsworker

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"auralogic/internal/pluginipc"
	"github.com/dop251/goja"
)

type pluginRuntimeWorkerBindings struct {
	parent       *pluginRuntimeWorker
	closePending atomic.Bool
}

type pluginRuntimeWorkerGroup struct {
	parentRuntime    *persistentPluginRuntime
	parentInvocation *persistentPluginRuntimeInvocation

	mu      sync.Mutex
	nextID  uint64
	closed  bool
	workers map[string]*pluginRuntimeWorker
}

type pluginRuntimeWorker struct {
	id         string
	scriptPath string
	group      *pluginRuntimeWorkerGroup
	runtime    *persistentPluginRuntime
	jsObject   *goja.Object
	ctx        context.Context
	cancel     context.CancelFunc

	mu     sync.Mutex
	closed bool
}

func newPluginRuntimeWorkerGroup(
	parentRuntime *persistentPluginRuntime,
	parentInvocation *persistentPluginRuntimeInvocation,
) *pluginRuntimeWorkerGroup {
	if parentRuntime == nil || parentInvocation == nil {
		return nil
	}
	return &pluginRuntimeWorkerGroup{
		parentRuntime:    parentRuntime,
		parentInvocation: parentInvocation,
		workers:          make(map[string]*pluginRuntimeWorker),
	}
}

func (g *pluginRuntimeWorkerGroup) close() {
	if g == nil {
		return
	}
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return
	}
	g.closed = true
	workers := make([]*pluginRuntimeWorker, 0, len(g.workers))
	for _, worker := range g.workers {
		workers = append(workers, worker)
	}
	g.workers = nil
	g.mu.Unlock()
	for _, worker := range workers {
		worker.terminate()
	}
}

func (g *pluginRuntimeWorkerGroup) removeWorker(id string) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.workers == nil {
		return
	}
	delete(g.workers, strings.TrimSpace(id))
}

func (g *pluginRuntimeWorkerGroup) createWorker(scriptSpec string) (*pluginRuntimeWorker, error) {
	if g == nil || g.parentRuntime == nil || g.parentInvocation == nil {
		return nil, fmt.Errorf("worker runtime group is unavailable")
	}
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return nil, fmt.Errorf("worker runtime group is closed")
	}
	workerSeq := g.nextID + 1
	g.nextID = workerSeq
	g.mu.Unlock()

	scriptPath, err := g.parentRuntime.resolveWorkerScriptPath(scriptSpec)
	if err != nil {
		return nil, err
	}

	workerID := fmt.Sprintf("worker_%d", workerSeq)
	workerCtx, workerCancel := context.WithCancel(g.parentInvocation.execCtx)
	worker := &pluginRuntimeWorker{
		id:         workerID,
		scriptPath: scriptPath,
		group:      g,
		ctx:        workerCtx,
		cancel:     workerCancel,
	}
	bindings := &pluginRuntimeWorkerBindings{parent: worker}
	runtime, err := newPersistentPluginRuntime(
		persistentPluginRuntimeKey{},
		scriptPath,
		g.parentRuntime.baseOpts,
		g.parentRuntime.fsCtx,
		false,
		bindings,
	)
	if err != nil {
		workerCancel()
		return nil, err
	}
	worker.runtime = runtime
	worker.jsObject = g.parentRuntime.buildWorkerJSObject(worker)

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		worker.terminate()
		return nil, fmt.Errorf("worker runtime group is closed")
	}
	g.workers[worker.id] = worker
	return worker, nil
}

func (r *persistentPluginRuntime) installWorkerRuntimeGlobals() {
	if r == nil || r.vm == nil {
		return
	}
	global := r.vm.GlobalObject()
	if global == nil {
		return
	}
	if r.allowWorkers {
		_ = r.vm.Set("Worker", func(call goja.ConstructorCall) *goja.Object {
			invocation := r.currentInvocation()
			if invocation == nil || invocation.workerGroup == nil {
				r.throwJSError(fmt.Errorf("Worker is unavailable for the current invocation"))
			}
			if len(call.Arguments) < 1 {
				r.throwJSError(fmt.Errorf("new Worker(script) requires script"))
			}
			scriptSpec := strings.TrimSpace(call.Argument(0).String())
			worker, err := invocation.workerGroup.createWorker(scriptSpec)
			if err != nil {
				r.throwJSError(err)
			}
			return worker.jsObject
		},
		)
	}
	if r.workerBindings != nil && r.workerBindings.parent != nil {
		_ = r.vm.Set("self", r.vm.GlobalObject())
		_ = r.vm.Set("onmessage", goja.Undefined())
		_ = r.vm.Set("postMessage", func(call goja.FunctionCall) goja.Value {
			payload, err := cloneWorkerTransferValue(exportGojaValue(argumentAt(call, 0)))
			if err != nil {
				r.throwJSError(err)
			}
			if err := r.workerBindings.parent.emitMessageToParent(payload); err != nil {
				r.throwJSError(err)
			}
			return goja.Undefined()
		})
		_ = r.vm.Set("close", func(call goja.FunctionCall) goja.Value {
			r.workerBindings.closePending.Store(true)
			return goja.Undefined()
		})
	}
}

func (r *persistentPluginRuntime) buildWorkerJSObject(worker *pluginRuntimeWorker) *goja.Object {
	workerObj := r.vm.NewObject()
	_ = workerObj.Set("id", worker.id)
	_ = workerObj.Set("scriptPath", filepath.ToSlash(worker.scriptPath))
	_ = workerObj.Set("terminated", false)
	_ = workerObj.Set("request", func(call goja.FunctionCall) goja.Value {
		return worker.request(argumentAt(call, 0))
	})
	_ = workerObj.Set("postMessage", func(call goja.FunctionCall) goja.Value {
		if err := worker.postMessage(argumentAt(call, 0)); err != nil {
			r.throwJSError(err)
		}
		return goja.Undefined()
	})
	_ = workerObj.Set("terminate", func(call goja.FunctionCall) goja.Value {
		worker.terminate()
		_ = workerObj.Set("terminated", true)
		return goja.Undefined()
	})
	_ = workerObj.Set("onmessage", goja.Undefined())
	_ = workerObj.Set("onerror", goja.Undefined())
	return workerObj
}

func (r *persistentPluginRuntime) resolveWorkerScriptPath(scriptSpec string) (string, error) {
	normalized := strings.TrimSpace(scriptSpec)
	if normalized == "" {
		return "", fmt.Errorf("worker script is required")
	}
	if r != nil && r.moduleLoader != nil {
		resolved, err := r.moduleLoader.resolveModulePath(r.entryDir, normalized)
		if err == nil && strings.TrimSpace(resolved) != "" {
			return normalizeScriptPath(resolved), nil
		}
	}
	candidate := filepath.Clean(filepath.Join(r.entryDir, filepath.FromSlash(normalized)))
	resolved, err := resolvePathWithinRoot(r.fsCtx.CodeRoot, candidate)
	if err != nil {
		return "", err
	}
	if !fileExists(resolved) {
		return "", fmt.Errorf("worker script not found: %s", filepath.ToSlash(normalized))
	}
	return normalizeScriptPath(resolved), nil
}

func cloneWorkerTransferValue(value interface{}) (interface{}, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("worker payload must be JSON-serializable: %w", err)
	}
	var cloned interface{}
	if err := json.Unmarshal(payload, &cloned); err != nil {
		return nil, fmt.Errorf("decode worker payload failed: %w", err)
	}
	return cloned, nil
}

func newWorkerRuntimeInvocation(
	parent *persistentPluginRuntimeInvocation,
	fsCtx pluginFSRuntimeContext,
	action string,
	workerID string,
	scriptPath string,
	execCtx context.Context,
) *persistentPluginRuntimeInvocation {
	if parent == nil {
		return nil
	}
	timeout := parent.timeout
	if !parent.deadline.IsZero() {
		timeout = timeUntilOrZero(parent.deadline)
	}
	if timeout <= 0 {
		timeout = parent.timeout
	}
	if timeout <= 0 {
		timeout = time.Second
	}

	sandboxCfg := parent.sandboxCfg
	sandboxCfg.CurrentAction = strings.TrimSpace(action)
	sandboxCfg.DeclaredStorageAccess = storageAccessNone
	sandboxCfg.StorageAccessMode = storageAccessNone
	sandboxCfg.ExecuteActionStorage = nil

	executionCtx := clonePluginIPCExecutionContext(parent.executionCtx)
	if executionCtx == nil {
		executionCtx = &pluginipc.ExecutionContext{}
	}
	executionCtx.Metadata = mergeStringMaps(executionCtx.Metadata, map[string]string{
		"worker_id":     strings.TrimSpace(workerID),
		"worker_action": strings.TrimSpace(action),
		"worker_script": filepath.ToSlash(scriptPath),
	})

	invocation := newPersistentPluginRuntimeInvocation(
		parent.effectiveOpts,
		sandboxCfg,
		parent.hostCfg,
		timeout,
		executionCtx,
		parent.pluginConfig,
		nil,
		parent.secretSnapshot,
		buildPluginIPCWebhookRequest(parent.webhookState),
		execCtx,
		fsCtx,
		nil,
	)
	if invocation != nil && invocation.storageState != nil {
		invocation.storageState.readOnly = true
	}
	return invocation
}

func buildPluginIPCWebhookRequest(state persistentPluginRuntimeWebhookState) *pluginipc.WebhookRequest {
	if !state.Enabled {
		return nil
	}
	return &pluginipc.WebhookRequest{
		Key:         state.Key,
		Method:      state.Method,
		Path:        state.Path,
		QueryString: state.QueryString,
		QueryParams: mergeStringMaps(state.QueryParams, nil),
		Headers:     mergeStringMaps(state.Headers, nil),
		BodyText:    state.BodyText,
		BodyBase64:  state.BodyBase64,
		ContentType: state.ContentType,
		RemoteAddr:  state.RemoteAddr,
	}
}

func timeUntilOrZero(deadline time.Time) time.Duration {
	if deadline.IsZero() {
		return 0
	}
	return time.Until(deadline)
}

func resolveWorkerMessageHandler(vm *goja.Runtime, entryModule *goja.Object) (goja.Callable, goja.Value, error) {
	if vm == nil {
		return nil, nil, errFunctionNotFound
	}
	if fn, ok := goja.AssertFunction(vm.Get("onmessage")); ok && fn != nil {
		return fn, goja.Undefined(), nil
	}
	if entryModule == nil {
		return nil, nil, errFunctionNotFound
	}
	moduleExports := entryModule.Get("exports")
	exportsObj := moduleExports.ToObject(vm)
	if exportsObj == nil {
		return nil, nil, errFunctionNotFound
	}
	if fn, ok := goja.AssertFunction(exportsObj.Get("onmessage")); ok && fn != nil {
		return fn, exportsObj, nil
	}
	if fn, ok := goja.AssertFunction(exportsObj.Get("handleMessage")); ok && fn != nil {
		return fn, exportsObj, nil
	}
	return nil, nil, errFunctionNotFound
}

func buildWorkerMessageEvent(payload interface{}, workerID string, scriptPath string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "message",
		"data":        payload,
		"worker_id":   strings.TrimSpace(workerID),
		"script_path": filepath.ToSlash(scriptPath),
	}
}

func (w *pluginRuntimeWorker) request(payload goja.Value) goja.Value {
	if w == nil || w.group == nil || w.group.parentRuntime == nil {
		panic("worker runtime is unavailable")
	}
	parentRuntime := w.group.parentRuntime
	clonedPayload, err := cloneWorkerTransferValue(exportGojaValue(payload))
	if err != nil {
		parentRuntime.throwJSError(err)
	}
	promise, resolve, reject := parentRuntime.vm.NewPromise()
	parentInvocation := w.group.parentInvocation
	go func() {
		result, runErr := w.dispatch(clonedPayload)
		if runErr != nil {
			w.notifyParentError(runErr)
			_ = parentRuntime.enqueueInvocationAsyncTask(parentInvocation, "worker.request.reject", func() error {
				return reject(runErr.Error())
			})
			return
		}
		_ = parentRuntime.enqueueInvocationAsyncTask(parentInvocation, "worker.request.resolve", func() error {
			return resolve(result)
		})
	}()
	return parentRuntime.vm.ToValue(promise)
}

func (w *pluginRuntimeWorker) postMessage(payload goja.Value) error {
	if w == nil || w.group == nil || w.group.parentRuntime == nil {
		return fmt.Errorf("worker runtime is unavailable")
	}
	clonedPayload, err := cloneWorkerTransferValue(exportGojaValue(payload))
	if err != nil {
		return err
	}
	go func() {
		if _, runErr := w.dispatch(clonedPayload); runErr != nil {
			w.notifyParentError(runErr)
		}
	}()
	return nil
}

func (w *pluginRuntimeWorker) dispatch(payload interface{}) (interface{}, error) {
	if w == nil || w.runtime == nil || w.group == nil || w.group.parentInvocation == nil {
		return nil, fmt.Errorf("worker runtime is unavailable")
	}
	w.mu.Lock()
	closed := w.closed
	w.mu.Unlock()
	if closed {
		return nil, fmt.Errorf("worker %s is terminated", strings.TrimSpace(w.id))
	}

	childInvocation := newWorkerRuntimeInvocation(
		w.group.parentInvocation,
		w.group.parentRuntime.fsCtx,
		"worker.message",
		w.id,
		w.scriptPath,
		w.ctx,
	)
	if childInvocation == nil {
		return nil, fmt.Errorf("worker invocation is unavailable")
	}

	result, _, _, _, err := w.runtime.runWithInvocation(childInvocation, func(vm *goja.Runtime, entryModule *goja.Object) (goja.Value, error) {
		handler, thisArg, resolveErr := resolveWorkerMessageHandler(vm, entryModule)
		if resolveErr != nil {
			return nil, resolveErr
		}
		event := buildWorkerMessageEvent(payload, w.id, w.scriptPath)
		return callScriptFunction(vm, handler, thisArg, "onmessage", []interface{}{event})
	})
	if w.runtime.workerBindings != nil && w.runtime.workerBindings.closePending.Load() {
		w.terminate()
	}
	if err != nil {
		return nil, err
	}
	return cloneWorkerTransferValue(exportGojaValue(result))
}

func (w *pluginRuntimeWorker) emitMessageToParent(payload interface{}) error {
	if w == nil || w.group == nil || w.group.parentRuntime == nil {
		return fmt.Errorf("parent runtime is unavailable")
	}
	clonedPayload, err := cloneWorkerTransferValue(payload)
	if err != nil {
		return err
	}
	if !w.group.parentRuntime.enqueueInvocationAsyncTask(w.group.parentInvocation, "worker.postMessage", func() error {
		return w.emitMessageToParentLocked(clonedPayload)
	}) {
		return fmt.Errorf("parent runtime is unavailable")
	}
	return nil
}

func (w *pluginRuntimeWorker) emitMessageToParentLocked(payload interface{}) error {
	if w == nil || w.jsObject == nil || w.group == nil || w.group.parentRuntime == nil {
		return nil
	}
	handlerValue, err := runtimeConsoleGetObjectProperty(w.jsObject, "onmessage")
	if err != nil || handlerValue == nil || goja.IsUndefined(handlerValue) || goja.IsNull(handlerValue) {
		return err
	}
	handler, ok := goja.AssertFunction(handlerValue)
	if !ok || handler == nil {
		return nil
	}
	event := buildWorkerMessageEvent(payload, w.id, w.scriptPath)
	_, callErr := handler(w.jsObject, w.group.parentRuntime.runtimeBridgeValue(event))
	return callErr
}

func (w *pluginRuntimeWorker) notifyParentError(err error) {
	if w == nil || err == nil || w.group == nil || w.group.parentRuntime == nil {
		return
	}
	_ = w.group.parentRuntime.enqueueInvocationAsyncTask(w.group.parentInvocation, "worker.error", func() error {
		return w.emitParentErrorLocked(err)
	})
}

func (w *pluginRuntimeWorker) emitParentErrorLocked(err error) error {
	if w == nil || err == nil || w.jsObject == nil || w.group == nil || w.group.parentRuntime == nil {
		return nil
	}
	handlerValue, handlerErr := runtimeConsoleGetObjectProperty(w.jsObject, "onerror")
	if handlerErr != nil || handlerValue == nil || goja.IsUndefined(handlerValue) || goja.IsNull(handlerValue) {
		return handlerErr
	}
	handler, ok := goja.AssertFunction(handlerValue)
	if !ok || handler == nil {
		return nil
	}
	event := map[string]interface{}{
		"type":        "error",
		"error":       err.Error(),
		"worker_id":   strings.TrimSpace(w.id),
		"script_path": filepath.ToSlash(w.scriptPath),
	}
	_, callErr := handler(w.jsObject, w.group.parentRuntime.runtimeBridgeValue(event))
	return callErr
}

func (w *pluginRuntimeWorker) terminate() {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	cancel := w.cancel
	runtime := w.runtime
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if runtime != nil {
		runtime.close()
	}
	if w.group != nil {
		w.group.removeWorker(w.id)
	}
}
