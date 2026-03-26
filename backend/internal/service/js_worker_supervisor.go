package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pkg/pluginutil"
	"auralogic/internal/pluginipc"
	"gorm.io/gorm"
)

const (
	defaultWorkerReadyTimeout = 8 * time.Second
)

type jsWorkerStreamEmitError struct {
	cause error
}

func (e *jsWorkerStreamEmitError) Error() string {
	if e == nil || e.cause == nil {
		return "js worker stream emit failed"
	}
	return e.cause.Error()
}

func (e *jsWorkerStreamEmitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

var defaultJSWorkerSocketPath = defaultJSWorkerSocketPathForOS()

type JSWorkerSupervisor struct {
	db            *gorm.DB
	cfg           *config.Config
	pluginManager *PluginManagerService

	mu           sync.Mutex
	cmd          *exec.Cmd
	hostListener net.Listener
	hostNetwork  string
	hostAddress  string
}

func NewJSWorkerSupervisor(db *gorm.DB, cfg *config.Config) *JSWorkerSupervisor {
	return &JSWorkerSupervisor{db: db, cfg: cfg}
}

func (s *JSWorkerSupervisor) Execute(plugin *models.Plugin, action string, params map[string]string, execCtx *ExecutionContext) (*ExecutionResult, error) {
	return s.ExecuteWithTimeoutAndStorage(plugin, action, params, nil, nil, execCtx, 0)
}

func (s *JSWorkerSupervisor) ExecuteWithTimeout(plugin *models.Plugin, action string, params map[string]string, execCtx *ExecutionContext, timeout time.Duration) (*ExecutionResult, error) {
	return s.ExecuteWithTimeoutAndStorage(plugin, action, params, nil, nil, execCtx, timeout)
}

func (s *JSWorkerSupervisor) ExecuteStreamWithTimeoutAndStorage(plugin *models.Plugin, action string, params map[string]string, storage map[string]string, secrets map[string]string, execCtx *ExecutionContext, timeout time.Duration, emit ExecutionStreamEmitter) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	scriptPath, err := s.resolveScriptPath(plugin)
	if err != nil {
		return nil, err
	}
	hostAPI, err := s.buildHostAPIConfig(plugin, execCtx, timeout)
	if err != nil {
		return nil, err
	}
	var workspaceCfg *pluginipc.WorkspaceConfig
	if s.pluginManager != nil {
		workspaceCfg = s.pluginManager.PreparePluginWorkspaceConfig(plugin, action, execCtx, 0)
	}

	req := pluginipc.Request{
		Type:          "execute_stream",
		PluginID:      plugin.ID,
		PluginGeneration: resolvePluginAppliedGeneration(plugin),
		PluginName:    plugin.Name,
		Action:        strings.TrimSpace(action),
		ScriptPath:    scriptPath,
		Params:        params,
		Storage:       cloneStringMap(storage),
		Context:       toIPCExecutionContext(execCtx),
		PluginConfig:  decodeJSONToObject(plugin.Config),
		PluginSecrets: cloneStringMap(secrets),
		Webhook:       toIPCWebhookRequest(execCtx),
		HostAPI:       hostAPI,
		Workspace:     workspaceCfg,
		Sandbox:       s.toSandboxConfigForAction(plugin, action, timeout),
	}

	return s.sendStreamRequestWithRetry(executionRequestContext(execCtx), req, emit)
}

func (s *JSWorkerSupervisor) ExecuteWithTimeoutAndStorage(plugin *models.Plugin, action string, params map[string]string, storage map[string]string, secrets map[string]string, execCtx *ExecutionContext, timeout time.Duration) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	scriptPath, err := s.resolveScriptPath(plugin)
	if err != nil {
		return nil, err
	}
	hostAPI, err := s.buildHostAPIConfig(plugin, execCtx, timeout)
	if err != nil {
		return nil, err
	}
	var workspaceCfg *pluginipc.WorkspaceConfig
	if s.pluginManager != nil {
		workspaceCfg = s.pluginManager.PreparePluginWorkspaceConfig(plugin, action, execCtx, 0)
	}

	req := pluginipc.Request{
		Type:          "execute",
		PluginID:      plugin.ID,
		PluginGeneration: resolvePluginAppliedGeneration(plugin),
		PluginName:    plugin.Name,
		Action:        strings.TrimSpace(action),
		ScriptPath:    scriptPath,
		Params:        params,
		Storage:       cloneStringMap(storage),
		Context:       toIPCExecutionContext(execCtx),
		PluginConfig:  decodeJSONToObject(plugin.Config),
		PluginSecrets: cloneStringMap(secrets),
		Webhook:       toIPCWebhookRequest(execCtx),
		HostAPI:       hostAPI,
		Workspace:     workspaceCfg,
		Sandbox:       s.toSandboxConfigForAction(plugin, action, timeout),
	}

	resp, err := s.sendRequestWithRetry(executionRequestContext(execCtx), req)
	if err != nil {
		return nil, err
	}

	result := &ExecutionResult{
		Success:        resp.Success,
		Data:           resp.Data,
		Storage:        cloneStringMap(resp.Storage),
		StorageChanged: resp.StorageChanged,
		Error:          strings.TrimSpace(resp.Error),
		Metadata:       resp.Metadata,
	}
	return result, nil
}

func (s *JSWorkerSupervisor) EvaluateRuntimeWithTimeoutAndStorage(
	plugin *models.Plugin,
	code string,
	storage map[string]string,
	secrets map[string]string,
	execCtx *ExecutionContext,
	timeout time.Duration,
) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	scriptPath, err := s.resolveScriptPath(plugin)
	if err != nil {
		return nil, err
	}
	hostAPI, err := s.buildHostAPIConfig(plugin, execCtx, timeout)
	if err != nil {
		return nil, err
	}
	action := "workspace.runtime.eval"
	var workspaceCfg *pluginipc.WorkspaceConfig
	if s.pluginManager != nil {
		workspaceCfg = s.pluginManager.PreparePluginWorkspaceConfig(plugin, action, execCtx, 0)
	}
	req := pluginipc.Request{
		Type:             "runtime_eval",
		PluginID:         plugin.ID,
		PluginGeneration: resolvePluginAppliedGeneration(plugin),
		PluginName:       plugin.Name,
		Action:           action,
		ScriptPath:       scriptPath,
		RuntimeCode:      code,
		Storage:          cloneStringMap(storage),
		Context:          toIPCExecutionContext(execCtx),
		PluginConfig:     decodeJSONToObject(plugin.Config),
		PluginSecrets:    cloneStringMap(secrets),
		Webhook:          toIPCWebhookRequest(execCtx),
		HostAPI:          hostAPI,
		Workspace:        workspaceCfg,
		Sandbox:          s.toSandboxConfigForAction(plugin, action, timeout),
	}

	resp, err := s.sendRequestWithRetry(executionRequestContext(execCtx), req)
	if err != nil {
		return nil, err
	}
	return &ExecutionResult{
		Success:        resp.Success,
		Data:           resp.Data,
		Storage:        cloneStringMap(resp.Storage),
		StorageChanged: resp.StorageChanged,
		Error:          strings.TrimSpace(resp.Error),
		Metadata:       resp.Metadata,
	}, nil
}

func (s *JSWorkerSupervisor) InspectRuntimeWithTimeoutAndStorage(
	plugin *models.Plugin,
	expression string,
	depth int,
	storage map[string]string,
	secrets map[string]string,
	execCtx *ExecutionContext,
	timeout time.Duration,
) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	scriptPath, err := s.resolveScriptPath(plugin)
	if err != nil {
		return nil, err
	}
	hostAPI, err := s.buildHostAPIConfig(plugin, execCtx, timeout)
	if err != nil {
		return nil, err
	}
	action := "workspace.runtime.inspect"
	var workspaceCfg *pluginipc.WorkspaceConfig
	if s.pluginManager != nil {
		workspaceCfg = s.pluginManager.PreparePluginWorkspaceConfig(plugin, action, execCtx, 0)
	}
	req := pluginipc.Request{
		Type:                     "runtime_inspect",
		PluginID:                 plugin.ID,
		PluginGeneration:         resolvePluginAppliedGeneration(plugin),
		PluginName:               plugin.Name,
		Action:                   action,
		ScriptPath:               scriptPath,
		RuntimeInspectExpression: expression,
		RuntimeInspectDepth:      depth,
		Storage:                  cloneStringMap(storage),
		Context:                  toIPCExecutionContext(execCtx),
		PluginConfig:             decodeJSONToObject(plugin.Config),
		PluginSecrets:            cloneStringMap(secrets),
		Webhook:                  toIPCWebhookRequest(execCtx),
		HostAPI:                  hostAPI,
		Workspace:                workspaceCfg,
		Sandbox:                  s.toSandboxConfigForAction(plugin, action, timeout),
	}

	resp, err := s.sendRequestWithRetry(executionRequestContext(execCtx), req)
	if err != nil {
		return nil, err
	}
	return &ExecutionResult{
		Success:        resp.Success,
		Data:           resp.Data,
		Storage:        cloneStringMap(resp.Storage),
		StorageChanged: resp.StorageChanged,
		Error:          strings.TrimSpace(resp.Error),
		Metadata:       resp.Metadata,
	}, nil
}

func (s *JSWorkerSupervisor) GetRuntimeState(plugin *models.Plugin) (map[string]interface{}, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	scriptPath, err := s.resolveScriptPath(plugin)
	if err != nil {
		return nil, err
	}
	req := pluginipc.Request{
		Type:             "runtime_state",
		PluginID:         plugin.ID,
		PluginGeneration: resolvePluginAppliedGeneration(plugin),
		PluginName:       plugin.Name,
		ScriptPath:       scriptPath,
		Sandbox:          s.toSandboxConfig(),
	}
	resp, err := s.sendRequestWithRetry(context.Background(), req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("js worker runtime state response is unavailable")
	}
	if !resp.Success {
		errMsg := strings.TrimSpace(resp.Error)
		if errMsg == "" {
			errMsg = "js worker runtime state request failed"
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	return clonePayloadMap(resp.Data), nil
}

func (s *JSWorkerSupervisor) HealthCheck(plugin *models.Plugin, secrets map[string]string) (*PluginHealth, error) {
	req := pluginipc.Request{
		Type:    "health",
		Sandbox: s.toSandboxConfig(),
	}
	if plugin != nil {
		scriptPath, err := s.resolveScriptPath(plugin)
		if err != nil {
			return nil, err
		}
		req.PluginID = plugin.ID
		req.PluginGeneration = resolvePluginAppliedGeneration(plugin)
		req.PluginName = plugin.Name
		req.ScriptPath = scriptPath
		req.PluginConfig = decodeJSONToObject(plugin.Config)
		req.PluginSecrets = cloneStringMap(secrets)
		req.Sandbox = s.toSandboxConfigForPlugin(plugin, 0)
	}

	resp, err := s.sendRequestWithRetry(context.Background(), req)
	if err != nil {
		return nil, err
	}
	if !resp.Success || !resp.Healthy {
		errMsg := strings.TrimSpace(resp.Error)
		if errMsg == "" {
			errMsg = "js worker reported unhealthy"
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	return &PluginHealth{
		Healthy:  true,
		Version:  resp.Version,
		Metadata: resp.Metadata,
	}, nil
}

func (s *JSWorkerSupervisor) DisposePluginRuntime(pluginID uint, generation uint) error {
	if pluginID == 0 {
		return nil
	}
	req := pluginipc.Request{
		Type:             "dispose_runtime",
		PluginID:         pluginID,
		PluginGeneration: generation,
		Sandbox:          s.toSandboxConfig(),
	}
	_, err := s.sendRequestWithRetry(context.Background(), req)
	return err
}

func (s *JSWorkerSupervisor) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopWorkerLocked()
	s.stopHostBridgeLocked()
}

func (s *JSWorkerSupervisor) RestartManagedWorker() error {
	pluginCfg := s.getPluginPlatformConfig()
	if !pluginCfg.Sandbox.JSWorkerAutoStart {
		return fmt.Errorf("js worker auto-start is disabled; restart the external worker manually")
	}
	return s.restartWorker()
}

func (s *JSWorkerSupervisor) sendRequestWithRetry(ctx context.Context, req pluginipc.Request) (*pluginipc.Response, error) {
	pluginCfg := s.getPluginPlatformConfig()
	if pluginCfg.Sandbox.JSWorkerAutoStart {
		if err := s.ensureWorkerRunning(); err != nil {
			return nil, err
		}
	}

	resp, err := s.sendRequest(ctx, req)
	if err == nil {
		return resp, nil
	}
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if !pluginCfg.Sandbox.JSWorkerAutoStart {
		return nil, err
	}

	if restartErr := s.restartWorker(); restartErr != nil {
		return nil, fmt.Errorf("js worker request failed: %w; restart failed: %v", err, restartErr)
	}

	return s.sendRequest(ctx, req)
}

func (s *JSWorkerSupervisor) sendStreamRequestWithRetry(ctx context.Context, req pluginipc.Request, emit ExecutionStreamEmitter) (*ExecutionResult, error) {
	pluginCfg := s.getPluginPlatformConfig()
	if pluginCfg.Sandbox.JSWorkerAutoStart {
		if err := s.ensureWorkerRunning(); err != nil {
			return nil, err
		}
	}

	resp, err := s.sendStreamRequest(ctx, req, emit)
	if err == nil {
		return resp, nil
	}
	var emitErr *jsWorkerStreamEmitError
	if errors.As(err, &emitErr) {
		return resp, emitErr
	}
	if ctx != nil && ctx.Err() != nil {
		return resp, ctx.Err()
	}

	if !pluginCfg.Sandbox.JSWorkerAutoStart {
		return nil, err
	}

	if restartErr := s.restartWorker(); restartErr != nil {
		return nil, fmt.Errorf("js worker stream request failed: %w; restart failed: %v", err, restartErr)
	}

	return s.sendStreamRequest(ctx, req, emit)
}

func (s *JSWorkerSupervisor) sendRequest(ctx context.Context, req pluginipc.Request) (*pluginipc.Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	network, address, err := s.workerEndpoint()
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(s.getPluginPlatformConfig().Sandbox.ExecTimeoutMs) * time.Millisecond
	if req.Sandbox.TimeoutMs > 0 {
		timeout = time.Duration(req.Sandbox.TimeoutMs) * time.Millisecond
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := timeout + 3*time.Second

	dialCtx, cancelDial := context.WithTimeout(ctx, 3*time.Second)
	defer cancelDial()

	conn, err := (&net.Dialer{}).DialContext(dialCtx, network, address)
	if err != nil {
		return nil, fmt.Errorf("dial js worker %s(%s): %w", network, address, err)
	}
	defer conn.Close()
	stopCloseOnCancel := closeJSWorkerConnOnContextCancel(ctx, conn)
	defer stopCloseOnCancel()

	_ = conn.SetDeadline(time.Now().Add(deadline))

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("encode js worker request: %w", err)
	}

	var resp pluginipc.Response
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&resp); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("decode js worker response: %w", err)
	}
	if s.pluginManager != nil {
		s.pluginManager.ApplyPluginWorkspaceDelta(
			req.PluginID,
			req.PluginName,
			PluginRuntimeJSWorker,
			buildJSWorkerWorkspaceBaseMetadata(req),
			resp.WorkspaceEntries,
			resp.WorkspaceCleared,
		)
	}
	return &resp, nil
}

func (s *JSWorkerSupervisor) sendStreamRequest(ctx context.Context, req pluginipc.Request, emit ExecutionStreamEmitter) (*ExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	network, address, err := s.workerEndpoint()
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(s.getPluginPlatformConfig().Sandbox.ExecTimeoutMs) * time.Millisecond
	if req.Sandbox.TimeoutMs > 0 {
		timeout = time.Duration(req.Sandbox.TimeoutMs) * time.Millisecond
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := timeout + 3*time.Second

	dialCtx, cancelDial := context.WithTimeout(ctx, 3*time.Second)
	defer cancelDial()

	conn, err := (&net.Dialer{}).DialContext(dialCtx, network, address)
	if err != nil {
		return nil, fmt.Errorf("dial js worker %s(%s): %w", network, address, err)
	}
	defer conn.Close()
	stopCloseOnCancel := closeJSWorkerConnOnContextCancel(ctx, conn)
	defer stopCloseOnCancel()

	_ = conn.SetDeadline(time.Now().Add(deadline))

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("encode js worker stream request: %w", err)
	}

	aggregator := newExecutionStreamAggregator()
	decoder := json.NewDecoder(conn)
	var lastResp *pluginipc.Response
	index := 0
	for {
		var resp pluginipc.Response
		if err := decoder.Decode(&resp); err != nil {
			if err == io.EOF {
				break
			}
			if ctx.Err() != nil {
				return buildJSWorkerStreamExecutionResult(aggregator, lastResp), ctx.Err()
			}
			return buildJSWorkerStreamExecutionResult(aggregator, lastResp), fmt.Errorf("decode js worker stream response: %w", err)
		}

		if s.pluginManager != nil {
			s.pluginManager.ApplyPluginWorkspaceDelta(
				req.PluginID,
				req.PluginName,
				PluginRuntimeJSWorker,
				buildJSWorkerWorkspaceBaseMetadata(req),
				resp.WorkspaceEntries,
				resp.WorkspaceCleared,
			)
		}
		lastResp = &resp
		chunk := &ExecutionStreamChunk{
			Index:    index,
			Success:  resp.Success,
			Data:     clonePayloadMap(resp.Data),
			Error:    strings.TrimSpace(resp.Error),
			Metadata: cloneStringMap(resp.Metadata),
			IsFinal:  resp.IsFinal,
		}
		aggregator.Add(chunk)
		if emit != nil {
			if emitErr := emit(cloneExecutionStreamChunk(chunk)); emitErr != nil {
				return buildJSWorkerStreamExecutionResult(aggregator, lastResp), &jsWorkerStreamEmitError{cause: emitErr}
			}
		}
		index++
		if resp.IsFinal {
			break
		}
	}

	if !aggregator.HasChunks() {
		return nil, fmt.Errorf("js worker stream returned no chunks")
	}
	if syntheticFinal := aggregator.SyntheticFinalChunk(); syntheticFinal != nil && emit != nil {
		if emitErr := emit(syntheticFinal); emitErr != nil {
			return buildJSWorkerStreamExecutionResult(aggregator, lastResp), &jsWorkerStreamEmitError{cause: emitErr}
		}
	}

	return buildJSWorkerStreamExecutionResult(aggregator, lastResp), nil
}

func buildJSWorkerWorkspaceBaseMetadata(req pluginipc.Request) map[string]string {
	metadata := map[string]string{}
	if action := strings.TrimSpace(req.Action); action != "" {
		metadata["action"] = action
		if action == pluginWorkspaceCommandExecuteAction {
			if commandName := strings.TrimSpace(req.Params[pluginWorkspaceCommandNameParam]); commandName != "" {
				metadata["action"] = "workspace.command." + commandName
				metadata["command"] = commandName
			}
			if commandID := strings.TrimSpace(req.Params[pluginWorkspaceCommandIDParam]); commandID != "" {
				metadata["command_id"] = commandID
			}
		}
	}
	if strings.EqualFold(strings.TrimSpace(req.Action), "hook.execute") {
		if hook := strings.TrimSpace(req.Params["hook"]); hook != "" {
			metadata["hook"] = hook
		}
	}
	if req.Context != nil {
		if taskID := strings.TrimSpace(req.Context.SessionID); taskID != "" {
			metadata["session_id"] = taskID
		}
		if req.Context.Metadata != nil {
			if taskID := strings.TrimSpace(req.Context.Metadata[PluginExecutionMetadataID]); taskID != "" {
				metadata["task_id"] = taskID
			}
		}
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func closeJSWorkerConnOnContextCancel(ctx context.Context, conn net.Conn) func() {
	if ctx == nil || conn == nil {
		return func() {}
	}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.SetDeadline(time.Now())
			_ = conn.Close()
		case <-done:
		}
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			close(done)
		})
	}
}

func buildJSWorkerStreamExecutionResult(aggregator *executionStreamAggregator, lastResp *pluginipc.Response) *ExecutionResult {
	result := aggregator.FinalResult()
	if result == nil && lastResp == nil {
		return nil
	}
	if result == nil {
		result = &ExecutionResult{
			Success:  lastResp.Success,
			Data:     clonePayloadMap(lastResp.Data),
			Error:    strings.TrimSpace(lastResp.Error),
			Metadata: cloneStringMap(lastResp.Metadata),
		}
	}
	if lastResp != nil {
		result.Storage = cloneStringMap(lastResp.Storage)
		result.StorageChanged = lastResp.StorageChanged
		if len(result.Metadata) == 0 && len(lastResp.Metadata) > 0 {
			result.Metadata = cloneStringMap(lastResp.Metadata)
		}
	}
	return result
}

func (s *JSWorkerSupervisor) ensureWorkerRunning() error {
	s.mu.Lock()
	alreadyRunning := s.cmd != nil && s.cmd.Process != nil
	if alreadyRunning {
		s.mu.Unlock()
		return nil
	}
	if err := s.startWorkerLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	return s.waitForWorkerReady(defaultWorkerReadyTimeout)
}

func (s *JSWorkerSupervisor) restartWorker() error {
	s.mu.Lock()
	s.stopWorkerLocked()
	if err := s.startWorkerLocked(); err != nil {
		s.mu.Unlock()
		return err
	}
	s.mu.Unlock()
	return s.waitForWorkerReady(defaultWorkerReadyTimeout)
}

func (s *JSWorkerSupervisor) startWorkerLocked() error {
	pluginCfg := s.getPluginPlatformConfig()
	network, address, err := s.workerEndpoint()
	if err != nil {
		return err
	}
	if network == "unix" {
		_ = os.Remove(address)
	}

	binary, args, err := s.resolveWorkerCommand(pluginCfg)
	if err != nil {
		return err
	}
	args = append(args, buildManagedWorkerArgs(pluginCfg, network, address)...)

	cmd := exec.Command(binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start js worker command failed: %w", err)
	}

	s.cmd = cmd
	go s.watchWorkerProcess(cmd)
	log.Printf("JS worker started: %s %s", binary, strings.Join(args, " "))
	return nil
}

func buildManagedWorkerArgs(pluginCfg config.PluginPlatformConfig, network string, address string) []string {
	return []string{
		"-network", network,
		"-socket", address,
		"-timeout-ms", strconv.Itoa(pluginCfg.Sandbox.ExecTimeoutMs),
		"-max-concurrency", strconv.Itoa(pluginCfg.Sandbox.MaxConcurrency),
		"-max-memory-mb", strconv.Itoa(pluginCfg.Sandbox.MaxMemoryMB),
		fmt.Sprintf("-allow-network=%t", pluginCfg.Sandbox.JSAllowNetwork),
		fmt.Sprintf("-allow-fs=%t", pluginCfg.Sandbox.JSAllowFileSystem),
		"-artifact-root", pluginCfg.ArtifactDir,
		"-fs-max-files", strconv.Itoa(pluginCfg.JSFSMaxFiles),
		"-fs-max-total-bytes", strconv.FormatInt(pluginCfg.JSFSMaxTotalBytes, 10),
		"-fs-max-read-bytes", strconv.FormatInt(pluginCfg.JSFSMaxReadBytes, 10),
		"-storage-max-keys", strconv.Itoa(pluginCfg.JSStorageMaxKeys),
		"-storage-max-total-bytes", strconv.FormatInt(pluginCfg.JSStorageMaxTotalBytes, 10),
		"-storage-max-value-bytes", strconv.FormatInt(pluginCfg.JSStorageMaxValueBytes, 10),
	}
}

func (s *JSWorkerSupervisor) stopWorkerLocked() {
	if s.cmd == nil {
		return
	}
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.cmd = nil
}

func (s *JSWorkerSupervisor) watchWorkerProcess(cmd *exec.Cmd) {
	_ = cmd.Wait()
	s.mu.Lock()
	if s.cmd == cmd {
		s.cmd = nil
	}
	s.mu.Unlock()
}

func (s *JSWorkerSupervisor) waitForWorkerReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("wait for js worker ready timeout after %s", timeout)
		}
		resp, err := s.sendRequest(context.Background(), pluginipc.Request{
			Type:    "health",
			Sandbox: s.toSandboxConfig(),
		})
		if err == nil && resp != nil && resp.Success && resp.Healthy {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func (s *JSWorkerSupervisor) workerEndpoint() (string, string, error) {
	socketPath := strings.TrimSpace(s.getPluginPlatformConfig().Sandbox.JSWorkerSocketPath)
	if socketPath == "" {
		socketPath = defaultJSWorkerSocketPath
	}
	network, address, err := pluginutil.ResolveJSWorkerSocketEndpoint(socketPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve js worker endpoint: %w", err)
	}
	return network, address, nil
}

func (s *JSWorkerSupervisor) resolveWorkerCommand(pluginCfg config.PluginPlatformConfig) (string, []string, error) {
	binary := strings.TrimSpace(pluginCfg.Sandbox.JSWorkerBinary)
	args := append([]string{}, pluginCfg.Sandbox.JSWorkerArgs...)
	for i := range args {
		args[i] = strings.TrimSpace(args[i])
	}
	args = filterEmptyStrings(args)

	if binary != "" {
		return binary, args, nil
	}

	// 默认直接复用当前 API 二进制进入 --js-worker 模式，
	// 满足“单二进制双用途（API + JS Worker）”部署模型。
	if selfBinary, selfErr := os.Executable(); selfErr == nil && strings.TrimSpace(selfBinary) != "" {
		return selfBinary, append([]string{"--js-worker"}, args...), nil
	}

	// 兼容旧部署：尝试与 API 同目录的独立 worker 二进制。
	if bundled := findBundledJSWorkerBinary(); bundled != "" {
		return bundled, args, nil
	}

	// 仅在调试环境保留 go run 回退，便于本地开发。
	if s.cfg != nil && s.cfg.App.Debug {
		return "go", []string{"run", "./cmd/jsworker"}, nil
	}

	return "", nil, fmt.Errorf("js worker command is not resolvable: set plugin.sandbox.js_worker_binary or run API binary with --js-worker support")
}

func findBundledJSWorkerBinary() string {
	executablePath, err := os.Executable()
	if err != nil {
		return ""
	}
	execDir := filepath.Dir(executablePath)
	if strings.TrimSpace(execDir) == "" {
		return ""
	}

	candidates := []string{"jsworker", "auralogic-jsworker"}
	if runtime.GOOS == "windows" {
		withExt := make([]string, 0, len(candidates))
		for _, name := range candidates {
			withExt = append(withExt, name+".exe")
		}
		candidates = withExt
	}
	for _, name := range candidates {
		candidate := filepath.Join(execDir, name)
		info, statErr := os.Stat(candidate)
		if statErr != nil || info.IsDir() {
			continue
		}
		return candidate
	}
	return ""
}

func (s *JSWorkerSupervisor) resolveScriptPath(plugin *models.Plugin) (string, error) {
	if plugin == nil {
		return "", fmt.Errorf("plugin is nil")
	}
	return ResolveJSWorkerScriptPath(plugin.Address, plugin.PackagePath)
}

func (s *JSWorkerSupervisor) toSandboxConfig() pluginipc.SandboxConfig {
	return s.toSandboxConfigWithTimeout(0)
}

func (s *JSWorkerSupervisor) toSandboxConfigForPlugin(plugin *models.Plugin, timeout time.Duration) pluginipc.SandboxConfig {
	sandboxCfg := s.toSandboxConfigWithTimeout(timeout)
	if plugin == nil {
		return sandboxCfg
	}

	policy := resolvePluginCapabilityPolicy(plugin)
	sandboxCfg.ExecuteActionStorage = cloneStringMap(policy.ExecuteActionStorage)
	sandboxCfg.AllowHookExecute = policy.AllowHookExecute
	sandboxCfg.AllowHookBlock = policy.AllowBlock
	sandboxCfg.AllowPayloadPatch = policy.AllowPayloadPatch
	sandboxCfg.AllowFrontendExtensions = policy.AllowFrontendExtensions
	sandboxCfg.AllowExecuteAPI = policy.AllowExecuteAPI
	sandboxCfg.RequestedPermissions = append([]string{}, policy.RequestedPermissions...)
	sandboxCfg.GrantedPermissions = append([]string{}, policy.GrantedPermissions...)
	sandboxCfg.AllowNetwork = sandboxCfg.AllowNetwork && policy.AllowsRuntimeNetwork()
	sandboxCfg.AllowFileSystem = sandboxCfg.AllowFileSystem && policy.AllowsRuntimeFileSystem()
	return sandboxCfg
}

func (s *JSWorkerSupervisor) toSandboxConfigForAction(plugin *models.Plugin, action string, timeout time.Duration) pluginipc.SandboxConfig {
	sandboxCfg := s.toSandboxConfigForPlugin(plugin, timeout)
	sandboxCfg.CurrentAction = strings.ToLower(strings.TrimSpace(action))
	if plugin != nil {
		policy := resolvePluginCapabilityPolicy(plugin)
		sandboxCfg.DeclaredStorageAccess = policy.ResolveExecuteActionStorageMode(action)
	}
	return sandboxCfg
}

func (s *JSWorkerSupervisor) toSandboxConfigWithTimeout(timeout time.Duration) pluginipc.SandboxConfig {
	pluginCfg := s.getPluginPlatformConfig()
	timeoutMs := pluginCfg.Sandbox.ExecTimeoutMs
	if timeout > 0 {
		timeoutMs = int(timeout / time.Millisecond)
	}
	if timeoutMs <= 0 {
		timeoutMs = 30000
	}
	return pluginipc.SandboxConfig{
		Level:                pluginCfg.Sandbox.Level,
		TimeoutMs:            timeoutMs,
		MaxMemoryMB:          pluginCfg.Sandbox.MaxMemoryMB,
		MaxConcurrency:       pluginCfg.Sandbox.MaxConcurrency,
		AllowNetwork:         pluginCfg.Sandbox.JSAllowNetwork,
		AllowFileSystem:      pluginCfg.Sandbox.JSAllowFileSystem,
		FSMaxFiles:           pluginCfg.JSFSMaxFiles,
		FSMaxTotalBytes:      pluginCfg.JSFSMaxTotalBytes,
		FSMaxReadBytes:       pluginCfg.JSFSMaxReadBytes,
		StorageMaxKeys:       pluginCfg.JSStorageMaxKeys,
		StorageMaxTotalBytes: pluginCfg.JSStorageMaxTotalBytes,
		StorageMaxValueBytes: pluginCfg.JSStorageMaxValueBytes,
	}
}

func (s *JSWorkerSupervisor) getPluginPlatformConfig() config.PluginPlatformConfig {
	cfg := config.PluginPlatformConfig{
		Enabled:                true,
		AllowedRuntimes:        []string{PluginRuntimeGRPC},
		DefaultRuntime:         PluginRuntimeGRPC,
		ArtifactDir:            filepath.Join("data", "plugins"),
		JSFSMaxFiles:           2048,
		JSFSMaxTotalBytes:      128 * 1024 * 1024,
		JSFSMaxReadBytes:       4 * 1024 * 1024,
		JSStorageMaxKeys:       512,
		JSStorageMaxTotalBytes: 4 * 1024 * 1024,
		JSStorageMaxValueBytes: 64 * 1024,
		Sandbox: config.PluginSandboxConfig{
			Level:              "balanced",
			ExecTimeoutMs:      30000,
			MaxMemoryMB:        128,
			MaxConcurrency:     4,
			JSWorkerSocketPath: defaultJSWorkerSocketPath,
		},
	}
	if s.cfg == nil {
		return cfg
	}

	cfg = s.cfg.Plugin
	if cfg.Sandbox.ExecTimeoutMs <= 0 {
		cfg.Sandbox.ExecTimeoutMs = 30000
	}
	if cfg.Sandbox.MaxConcurrency <= 0 {
		cfg.Sandbox.MaxConcurrency = 4
	}
	if cfg.Sandbox.MaxMemoryMB <= 0 {
		cfg.Sandbox.MaxMemoryMB = 128
	}
	cfg.ArtifactDir = filepath.Clean(filepath.FromSlash(strings.TrimSpace(cfg.ArtifactDir)))
	if cfg.ArtifactDir == "" || cfg.ArtifactDir == "." {
		cfg.ArtifactDir = filepath.Join("data", "plugins")
	}
	if cfg.JSFSMaxFiles <= 0 {
		cfg.JSFSMaxFiles = 2048
	}
	if cfg.JSFSMaxTotalBytes <= 0 {
		cfg.JSFSMaxTotalBytes = 128 * 1024 * 1024
	}
	if cfg.JSFSMaxReadBytes <= 0 {
		cfg.JSFSMaxReadBytes = 4 * 1024 * 1024
	}
	if cfg.JSFSMaxReadBytes > cfg.JSFSMaxTotalBytes {
		cfg.JSFSMaxReadBytes = cfg.JSFSMaxTotalBytes
	}
	if cfg.JSStorageMaxKeys <= 0 {
		cfg.JSStorageMaxKeys = 512
	}
	if cfg.JSStorageMaxTotalBytes <= 0 {
		cfg.JSStorageMaxTotalBytes = 4 * 1024 * 1024
	}
	if cfg.JSStorageMaxValueBytes <= 0 {
		cfg.JSStorageMaxValueBytes = 64 * 1024
	}
	if cfg.JSStorageMaxValueBytes > cfg.JSStorageMaxTotalBytes {
		cfg.JSStorageMaxValueBytes = cfg.JSStorageMaxTotalBytes
	}
	if strings.TrimSpace(cfg.Sandbox.JSWorkerSocketPath) == "" {
		cfg.Sandbox.JSWorkerSocketPath = defaultJSWorkerSocketPath
	}
	return cfg
}

func decodeJSONToObject(raw string) map[string]interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return map[string]interface{}{}
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil || decoded == nil {
		return map[string]interface{}{}
	}
	return decoded
}

func toIPCExecutionContext(execCtx *ExecutionContext) *pluginipc.ExecutionContext {
	if execCtx == nil {
		return nil
	}
	ctx := &pluginipc.ExecutionContext{
		SessionID: execCtx.SessionID,
		Metadata:  execCtx.Metadata,
	}
	if execCtx.UserID != nil {
		ctx.UserID = *execCtx.UserID
	}
	if execCtx.OrderID != nil {
		ctx.OrderID = *execCtx.OrderID
	}
	return ctx
}

func toIPCWebhookRequest(execCtx *ExecutionContext) *pluginipc.WebhookRequest {
	if execCtx == nil || execCtx.Webhook == nil {
		return nil
	}
	return &pluginipc.WebhookRequest{
		Key:         strings.TrimSpace(execCtx.Webhook.Key),
		Method:      strings.TrimSpace(execCtx.Webhook.Method),
		Path:        strings.TrimSpace(execCtx.Webhook.Path),
		QueryString: strings.TrimSpace(execCtx.Webhook.QueryString),
		QueryParams: cloneStringMap(execCtx.Webhook.QueryParams),
		Headers:     cloneStringMap(execCtx.Webhook.Headers),
		BodyText:    execCtx.Webhook.BodyText,
		BodyBase64:  strings.TrimSpace(execCtx.Webhook.BodyBase64),
		ContentType: strings.TrimSpace(execCtx.Webhook.ContentType),
		RemoteAddr:  strings.TrimSpace(execCtx.Webhook.RemoteAddr),
	}
}

func (s *JSWorkerSupervisor) buildHostAPIConfig(plugin *models.Plugin, execCtx *ExecutionContext, timeout time.Duration) (*pluginipc.HostAPIConfig, error) {
	if s == nil || s.cfg == nil || s.db == nil || plugin == nil {
		return nil, nil
	}
	secret := strings.TrimSpace(s.cfg.JWT.Secret)
	if secret == "" {
		return nil, nil
	}
	if err := s.ensureHostBridgeRunning(); err != nil {
		return nil, err
	}

	requestTimeout := timeout
	if requestTimeout <= 0 {
		requestTimeout = s.defaultExecuteTimeout()
	}
	if requestTimeout <= 0 {
		requestTimeout = 30 * time.Second
	}
	timeoutMs := int(requestTimeout / time.Millisecond)
	if timeoutMs <= 0 {
		timeoutMs = 30000
	}

	token, err := GeneratePluginHostAccessToken(secret, BuildPluginHostAccessClaims(plugin, execCtx, requestTimeout+time.Minute))
	if err != nil {
		return nil, fmt.Errorf("generate plugin host token failed: %w", err)
	}

	s.mu.Lock()
	network := s.hostNetwork
	address := s.hostAddress
	s.mu.Unlock()
	if strings.TrimSpace(network) == "" || strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("plugin host bridge endpoint is unavailable")
	}

	return &pluginipc.HostAPIConfig{
		Network:     network,
		Address:     address,
		AccessToken: token,
		TimeoutMs:   timeoutMs,
	}, nil
}

func (s *JSWorkerSupervisor) defaultExecuteTimeout() time.Duration {
	timeoutMs := s.getPluginPlatformConfig().Sandbox.ExecTimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 30000
	}
	return time.Duration(timeoutMs) * time.Millisecond
}

func filterEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
