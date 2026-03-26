package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pluginobs"
	"gorm.io/gorm"
)

// PluginManagerService 插件管理服务
type PluginManagerService struct {
	db                       *gorm.DB
	cfg                      *config.Config
	grpcClients              map[uint]*PluginClient
	jsWorker                 *JSWorkerSupervisor
	lifecycleMu              sync.Mutex
	mu                       sync.RWMutex
	auditMu                  sync.RWMutex
	executionPersistMu       sync.RWMutex
	taskMu                   sync.RWMutex
	registrationMu           sync.RWMutex
	storageMu                sync.RWMutex
	secretMu                 sync.RWMutex
	workspaceMu              sync.RWMutex
	catalogMu                sync.RWMutex
	breakerMu                sync.Mutex
	registration             map[uint]PluginRegistrationInspection
	executionTasks           map[string]*pluginExecutionTask
	executionTaskOrder       []string
	executionBreakers        map[uint]*pluginExecutionBreakerRuntime
	runtimeSlots             map[uint]*pluginRuntimeSlotSet
	executionCatalog         pluginExecutionCatalog
	storageSnapshots         map[uint]map[string]string
	secretSnapshots          map[uint]map[string]string
	workspaceBuffers         map[uint]*pluginWorkspaceBuffer
	workspaceSessions        map[uint]*pluginWorkspaceSession
	maxExecutionTaskHistory  int
	hookLimiterMu            sync.Mutex
	hookLimiter              chan struct{}
	hookLimiterCap           int
	auditLogQueue            chan pluginExecutionAuditEntry
	auditLogWorkerWG         sync.WaitGroup
	auditLogDropped          atomic.Uint64
	executionPersistQueue    chan *models.PluginExecution
	executionPersistWorkerWG sync.WaitGroup
	stopChan                 chan struct{}
	started                  bool
	healthCheckTick          time.Duration
	executionRetentionTick   time.Duration
}

func (s *PluginManagerService) Config() *config.Config {
	if s == nil {
		return nil
	}
	return s.cfg
}

const (
	PluginRuntimeGRPC          = "grpc"
	PluginRuntimeJSWorker      = "js_worker"
	pluginExecutionLogMaxChars = 8192
	pluginAuditLogQueueSize    = 256
	pluginAuditLogFlushTick    = 5 * time.Second
	pluginBreakerStateClosed   = "closed"
	pluginBreakerStateOpen     = "open"
	pluginBreakerStateHalfOpen = "half_open"

	PluginScopeMetadataAuthenticated = "plugin_scope_authenticated"
	PluginScopeMetadataSuperAdmin    = "plugin_scope_super_admin"
	PluginScopeMetadataPermissions   = "plugin_scope_permissions"
)

// HookExecutionRequest 插件 Hook 执行请求
type HookExecutionRequest struct {
	Hook    string                 `json:"hook"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// FrontendExtension 前端可渲染扩展块
type FrontendExtension struct {
	ID         string                 `json:"id,omitempty"`
	Slot       string                 `json:"slot,omitempty"`
	Type       string                 `json:"type,omitempty"`
	Title      string                 `json:"title,omitempty"`
	Content    string                 `json:"content,omitempty"`
	Link       string                 `json:"link,omitempty"`
	Priority   int                    `json:"priority,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Metadata   map[string]string      `json:"metadata,omitempty"`
	PluginID   uint                   `json:"plugin_id,omitempty"`
	PluginName string                 `json:"plugin_name,omitempty"`
}

// HookPluginResult 单个插件 Hook 执行结果
type HookPluginResult struct {
	PluginID   uint   `json:"plugin_id"`
	PluginName string `json:"plugin_name"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	Duration   int    `json:"duration"`
}

// HookExecutionResult Hook 聚合执行结果
type HookExecutionResult struct {
	Hook               string                 `json:"hook"`
	Payload            map[string]interface{} `json:"payload,omitempty"`
	Blocked            bool                   `json:"blocked"`
	BlockReason        string                 `json:"block_reason,omitempty"`
	FrontendExtensions []FrontendExtension    `json:"frontend_extensions,omitempty"`
	PluginResults      []HookPluginResult     `json:"plugin_results,omitempty"`
}

type PluginRuntimeInspection struct {
	ConfiguredRuntime   string     `json:"configured_runtime"`
	ResolvedRuntime     string     `json:"resolved_runtime,omitempty"`
	Valid               bool       `json:"valid"`
	Enabled             bool       `json:"enabled"`
	LifecycleStatus     string     `json:"lifecycle_status,omitempty"`
	HealthStatus        string     `json:"health_status,omitempty"`
	ConnectionState     string     `json:"connection_state,omitempty"`
	AddressPresent      bool       `json:"address_present"`
	PackagePathPresent  bool       `json:"package_path_present"`
	Ready               bool       `json:"ready"`
	BreakerState        string     `json:"breaker_state,omitempty"`
	FailureCount        int        `json:"failure_count"`
	FailureThreshold    int        `json:"failure_threshold"`
	CooldownActive      bool       `json:"cooldown_active"`
	CooldownUntil       *time.Time `json:"cooldown_until,omitempty"`
	CooldownReason      string     `json:"cooldown_reason,omitempty"`
	ProbeInFlight       bool       `json:"probe_in_flight"`
	ProbeStartedAt      *time.Time `json:"probe_started_at,omitempty"`
	ActiveGeneration    uint       `json:"active_generation,omitempty"`
	ActiveInFlight      int64      `json:"active_in_flight"`
	DrainingSlotCount   int        `json:"draining_slot_count"`
	DrainingInFlight    int64      `json:"draining_in_flight"`
	DrainingGenerations []uint     `json:"draining_generations,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
}

type PluginRegistrationInspection struct {
	State       string    `json:"state"`
	Trigger     string    `json:"trigger,omitempty"`
	Runtime     string    `json:"runtime,omitempty"`
	AttemptedAt time.Time `json:"attempted_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	DurationMs  int64     `json:"duration_ms,omitempty"`
	Detail      string    `json:"detail,omitempty"`
}

type PluginHookParticipationDiagnosis struct {
	Hook                    string `json:"hook"`
	Area                    string `json:"area,omitempty"`
	Path                    string `json:"path,omitempty"`
	Slot                    string `json:"slot,omitempty"`
	Participates            bool   `json:"participates"`
	SupportsHook            bool   `json:"supports_hook"`
	AccessAllowed           bool   `json:"access_allowed"`
	SupportsFrontendArea    bool   `json:"supports_frontend_area"`
	SupportsFrontendSlot    bool   `json:"supports_frontend_slot"`
	AllowBlock              bool   `json:"allow_block"`
	AllowPayloadPatch       bool   `json:"allow_payload_patch"`
	AllowFrontendExtensions bool   `json:"allow_frontend_extensions"`
	ValidCapabilityPolicy   bool   `json:"valid_capability_policy"`
	ReasonCode              string `json:"reason_code,omitempty"`
	Reason                  string `json:"reason,omitempty"`
}

type preparedHookPlugin struct {
	Plugin           models.Plugin
	Runtime          string
	CapabilityPolicy pluginCapabilityPolicy
}

type ExecutionRequestCache struct {
	mu                            sync.Mutex
	frontendPreparedHookByKey     map[string][]preparedHookPlugin
	frontendPreparedInflightByKey map[string]*preparedHookPluginsResolveCall
}

type preparedHookPluginsResolveCall struct {
	done chan struct{}
}

type hookPluginExecutionOutcome struct {
	Plugin           models.Plugin
	CapabilityPolicy pluginCapabilityPolicy
	ExecResult       *ExecutionResult
	ExecErr          error
	HookParamsErr    error
	Duration         int
}

type pluginExecutionBreakerRuntime struct {
	ConsecutiveFailures int
	LastFailureAt       time.Time
	OpenUntil           time.Time
	ProbeInFlight       bool
	ProbeStartedAt      time.Time
}

type pluginExecutionBreakerState struct {
	State               string
	ConsecutiveFailures int
	FailureThreshold    int
	CooldownActive      bool
	CooldownUntil       *time.Time
	ProbeInFlight       bool
	ProbeStartedAt      *time.Time
	ReasonCode          string
	Reason              string
}

type pluginExecutionPermit struct {
	PluginID      uint
	BreakerState  pluginExecutionBreakerState
	HalfOpenProbe bool
}

type pluginExecutionAuditEntry struct {
	Timestamp       time.Time
	PluginID        uint
	PluginName      string
	Runtime         string
	Action          string
	DurationMs      int
	Params          map[string]string
	HasContext      bool
	SessionID       string
	ContextMetadata map[string]string
	UserID          *uint
	OrderID         *uint
	Success         bool
	Error           string
	ResultData      map[string]interface{}
}

type pluginExecutionAuditQueueStatus uint8

const (
	pluginExecutionAuditQueueSyncFallback pluginExecutionAuditQueueStatus = iota
	pluginExecutionAuditQueueEnqueued
	pluginExecutionAuditQueueDropped
)

// NewPluginManagerService 创建插件管理服务
func NewPluginManagerService(db *gorm.DB, cfg *config.Config) *PluginManagerService {
	service := &PluginManagerService{
		db:                      db,
		cfg:                     cfg,
		grpcClients:             make(map[uint]*PluginClient),
		registration:            make(map[uint]PluginRegistrationInspection),
		executionTasks:          make(map[string]*pluginExecutionTask),
		executionBreakers:       make(map[uint]*pluginExecutionBreakerRuntime),
		runtimeSlots:            make(map[uint]*pluginRuntimeSlotSet),
		executionCatalog:        newPluginExecutionCatalog(),
		storageSnapshots:        make(map[uint]map[string]string),
		secretSnapshots:         make(map[uint]map[string]string),
		workspaceBuffers:        make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions:       make(map[uint]*pluginWorkspaceSession),
		jsWorker:                NewJSWorkerSupervisor(db, cfg),
		healthCheckTick:         30 * time.Second,
		executionRetentionTick:  6 * time.Hour,
		maxExecutionTaskHistory: defaultPluginExecutionHistoryLimit,
	}
	if service.jsWorker != nil {
		service.jsWorker.pluginManager = service
	}
	return service
}

// Start 启动服务
func (s *PluginManagerService) Start() {
	if s == nil {
		return
	}

	s.lifecycleMu.Lock()
	if s.started {
		s.lifecycleMu.Unlock()
		return
	}
	stopChan := make(chan struct{})
	s.stopChan = stopChan
	s.started = true
	s.lifecycleMu.Unlock()

	s.startPluginAuditLogWorker(stopChan)
	s.startPluginExecutionPersistWorker(stopChan)
	go s.executionRetentionLoop(stopChan)
	if !s.isPluginPlatformEnabled() {
		log.Println("PluginManagerService runtime skipped: plugin platform disabled by config")
		return
	}

	// 加载已启用的插件
	if err := s.loadPlugins(); err != nil {
		log.Printf("Failed to load plugins: %v", err)
	}

	// 启动健康检查循环
	go s.healthCheckLoop(stopChan)
	log.Println("PluginManagerService started")
}

// Stop 停止服务
func (s *PluginManagerService) Stop() {
	if s == nil {
		return
	}

	s.lifecycleMu.Lock()
	if !s.started {
		s.lifecycleMu.Unlock()
		return
	}
	stopChan := s.stopChan
	s.stopChan = nil
	s.started = false
	s.lifecycleMu.Unlock()

	s.auditMu.Lock()
	s.auditLogQueue = nil
	s.auditMu.Unlock()
	s.executionPersistMu.Lock()
	s.executionPersistQueue = nil
	s.executionPersistMu.Unlock()
	if stopChan != nil {
		close(stopChan)
	}
	s.auditLogWorkerWG.Wait()
	s.executionPersistWorkerWG.Wait()

	s.closeAllRuntimeSlots()

	s.taskMu.Lock()
	for _, task := range s.executionTasks {
		if task != nil {
			task.cancelTask()
		}
	}
	s.taskMu.Unlock()
	s.storageMu.Lock()
	s.storageSnapshots = make(map[uint]map[string]string)
	s.storageMu.Unlock()
	s.secretMu.Lock()
	s.secretSnapshots = make(map[uint]map[string]string)
	s.secretMu.Unlock()
	s.workspaceMu.Lock()
	s.workspaceBuffers = make(map[uint]*pluginWorkspaceBuffer)
	s.workspaceSessions = make(map[uint]*pluginWorkspaceSession)
	s.workspaceMu.Unlock()
	s.clearPluginExecutionCatalog()
	s.breakerMu.Lock()
	s.executionBreakers = make(map[uint]*pluginExecutionBreakerRuntime)
	s.breakerMu.Unlock()
	if s.jsWorker != nil {
		s.jsWorker.Stop()
	}
	log.Println("PluginManagerService stopped")
}

func (s *PluginManagerService) RestartJSWorker() error {
	if s == nil || s.jsWorker == nil {
		return fmt.Errorf("js worker supervisor is not initialized")
	}
	return s.jsWorker.RestartManagedWorker()
}

// loadPlugins 从数据库加载插件
func (s *PluginManagerService) loadPlugins() error {
	var plugins []models.Plugin
	if err := s.db.Where("enabled = ?", true).Order("id ASC").Find(&plugins).Error; err != nil {
		return err
	}
	s.setPluginExecutionCatalogFromPlugins(plugins)

	for i := range plugins {
		if err := s.registerPluginWithTrigger(&plugins[i], "startup_load"); err != nil {
			log.Printf("Failed to register plugin %s: %v", plugins[i].Name, err)
		}
	}

	log.Printf("Loaded %d plugins", len(plugins))
	return nil
}

// registerPlugin 注册插件
func (s *PluginManagerService) registerPlugin(plugin *models.Plugin) error {
	return s.registerPluginWithTrigger(plugin, "reload")
}

func (s *PluginManagerService) registerPluginWithTrigger(plugin *models.Plugin, trigger string) (err error) {
	startedAt := time.Now().UTC()
	resolvedRuntime := ""
	defer func() {
		s.recordPluginRegistrationOutcome(plugin, trigger, resolvedRuntime, startedAt, err)
	}()

	if plugin == nil {
		return fmt.Errorf("plugin is nil")
	}
	resolvedRuntime = strings.ToLower(strings.TrimSpace(plugin.Runtime))

	runtime, err := s.ResolveRuntime(plugin.Runtime)
	if err != nil {
		return err
	}
	resolvedRuntime = runtime
	if err := s.ValidatePluginProfile(runtime, plugin.Type); err != nil {
		return err
	}
	if err := ValidatePluginProtocolCompatibility(plugin); err != nil {
		return err
	}

	var (
		slot        *pluginRuntimeSlot
		registerErr error
	)
	switch runtime {
	case PluginRuntimeGRPC:
		slot, registerErr = s.prepareGRPCPluginSlot(plugin)
	case PluginRuntimeJSWorker:
		slot, registerErr = s.prepareJSPluginSlot(plugin)
	default:
		registerErr = fmt.Errorf("unsupported plugin runtime %q", runtime)
	}
	if registerErr != nil {
		s.updatePluginStatus(plugin.ID, "unhealthy")
		s.updatePluginLifecycle(plugin.ID, models.PluginLifecycleDegraded, map[string]interface{}{
			"last_error": registerErr.Error(),
		})
		return registerErr
	}
	s.activatePreparedRuntimeSlot(plugin.ID, slot)

	s.updatePluginStatus(plugin.ID, "healthy")
	now := time.Now()
	s.updatePluginLifecycle(plugin.ID, models.PluginLifecycleRunning, map[string]interface{}{
		"last_error":   "",
		"installed_at": now,
		"started_at":   now,
		"stopped_at":   nil,
		"retired_at":   nil,
	})
	log.Printf("Plugin registered: %s [%s]", plugin.Name, runtime)
	return nil
}

// healthCheckLoop 健康检查循环
func (s *PluginManagerService) healthCheckLoop(stopChan <-chan struct{}) {
	ticker := time.NewTicker(s.healthCheckTick)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			s.performHealthChecks()
			s.sweepExpiredDrainingSlots()
		}
	}
}

// performHealthChecks 执行健康检查
func (s *PluginManagerService) performHealthChecks() {
	slots := s.listActiveRuntimeSlots(PluginRuntimeGRPC)
	for _, slot := range slots {
		if slot == nil || slot.GRPCClient == nil {
			s.releasePluginRuntimeSlot(slot)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := slot.GRPCClient.HealthCheck(ctx)
		cancel()

		if err != nil {
			slot.GRPCClient.RecordFailure()
			s.updatePluginStatus(slot.PluginID, "unhealthy")
			s.releasePluginRuntimeSlot(slot)
			continue
		} else {
			s.updatePluginStatus(slot.PluginID, "healthy")
		}
		s.releasePluginRuntimeSlot(slot)
	}

	s.performJSPluginHealthChecks()
}

// updatePluginStatus 更新插件状态
func (s *PluginManagerService) updatePluginStatus(pluginID uint, status string) {
	now := time.Now()
	update := map[string]interface{}{"status": status}
	if status == "healthy" {
		update["last_healthy"] = now
		update["fail_count"] = 0
	} else {
		if err := s.db.Model(&models.Plugin{}).Where("id = ?", pluginID).
			UpdateColumn("fail_count", gorm.Expr("fail_count + 1")).Error; err != nil {
			log.Printf("Failed to update plugin %d fail_count: %v", pluginID, err)
		}
	}
	if err := s.db.Model(&models.Plugin{}).Where("id = ?", pluginID).Updates(update).Error; err != nil {
		log.Printf("Failed to update plugin %d status: %v", pluginID, err)
	}
	s.syncPluginExecutionBreakerStatus(pluginID, status, now.UTC())

	if status == "healthy" {
		s.updatePluginLifecycle(pluginID, models.PluginLifecycleRunning, map[string]interface{}{
			"last_error": "",
		})
		return
	}
	s.updatePluginLifecycle(pluginID, models.PluginLifecycleDegraded, nil)
}

func (s *PluginManagerService) syncPluginExecutionBreakerStatus(pluginID uint, status string, now time.Time) {
	if pluginID == 0 {
		return
	}

	policy := s.getHookExecutionPolicy()
	threshold := normalizePluginFailureThreshold(policy)
	cooldown := time.Duration(policy.FailureCooldownMs) * time.Millisecond

	s.breakerMu.Lock()
	defer s.breakerMu.Unlock()
	if s.executionBreakers == nil {
		s.executionBreakers = make(map[uint]*pluginExecutionBreakerRuntime)
	}
	entry, exists := s.executionBreakers[pluginID]
	if !exists || entry == nil {
		entry = &pluginExecutionBreakerRuntime{}
		s.executionBreakers[pluginID] = entry
	}

	if strings.EqualFold(strings.TrimSpace(status), "healthy") {
		entry.ConsecutiveFailures = 0
		entry.LastFailureAt = time.Time{}
		entry.OpenUntil = time.Time{}
		entry.ProbeInFlight = false
		entry.ProbeStartedAt = time.Time{}
		return
	}

	entry.ConsecutiveFailures++
	entry.LastFailureAt = now
	if entry.ConsecutiveFailures >= threshold && cooldown > 0 {
		entry.OpenUntil = now.Add(cooldown)
		return
	}
	entry.OpenUntil = time.Time{}
}

func (s *PluginManagerService) getGRPCClient(pluginID uint) (*PluginClient, bool) {
	s.mu.RLock()
	client, exists := s.grpcClients[pluginID]
	s.mu.RUnlock()
	return client, exists
}

func (s *PluginManagerService) resolveExecutionPluginRuntime(
	plugin *models.Plugin,
	runtime string,
	reloadTrigger string,
) (*models.Plugin, *pluginRuntimeSlot, func(), error) {
	if plugin == nil {
		return nil, nil, nil, fmt.Errorf("plugin is nil")
	}

	expectedGeneration := resolvePluginAppliedGeneration(plugin)
	slot, exists := s.acquirePluginRuntimeSlot(plugin.ID, runtime, expectedGeneration)
	if !exists {
		if err := s.reloadPluginWithTrigger(plugin.ID, reloadTrigger); err != nil {
			return nil, nil, nil, fmt.Errorf("plugin not available: %w", err)
		}
		slot, exists = s.acquirePluginRuntimeSlot(plugin.ID, runtime, resolvePluginAppliedGeneration(plugin))
	}
	if !exists || slot == nil {
		return nil, nil, nil, fmt.Errorf("plugin %d runtime slot not available", plugin.ID)
	}

	executionPlugin := slot.PluginSnapshot
	release := func() {
		s.releasePluginRuntimeSlot(slot)
	}
	return &executionPlugin, slot, release, nil
}

// ExecutePlugin 执行插件
func (s *PluginManagerService) ExecutePlugin(pluginID uint, action string, params map[string]string, execCtx *ExecutionContext) (*ExecutionResult, error) {
	return s.executePluginByIDWithTimeout(pluginID, action, params, execCtx, 0)
}

// ExecutePluginStream 流式执行插件动作
func (s *PluginManagerService) ExecutePluginStream(pluginID uint, action string, params map[string]string, execCtx *ExecutionContext, emit ExecutionStreamEmitter) (*ExecutionResult, error) {
	return s.executePluginByIDStreamWithTimeout(pluginID, action, params, execCtx, 0, emit)
}

func (s *PluginManagerService) executePluginByIDWithTimeout(pluginID uint, action string, params map[string]string, execCtx *ExecutionContext, timeoutOverride time.Duration) (*ExecutionResult, error) {
	plugin, runtime, capabilityPolicy, err, _ := s.getPluginByIDWithCatalog(pluginID)
	if err != nil {
		return nil, err
	}
	if !capabilityPolicy.AllowExecuteAPI {
		return nil, fmt.Errorf("plugin %s manual execute is disabled by capabilities", plugin.Name)
	}

	return s.executePluginResolvedWithTimeout(plugin, runtime, capabilityPolicy, action, params, execCtx, timeoutOverride)
}

func (s *PluginManagerService) executePluginByIDStreamWithTimeout(pluginID uint, action string, params map[string]string, execCtx *ExecutionContext, timeoutOverride time.Duration, emit ExecutionStreamEmitter) (*ExecutionResult, error) {
	plugin, runtime, capabilityPolicy, err, _ := s.getPluginByIDWithCatalog(pluginID)
	if err != nil {
		return nil, err
	}
	if !capabilityPolicy.AllowExecuteAPI {
		return nil, fmt.Errorf("plugin %s manual execute is disabled by capabilities", plugin.Name)
	}

	return s.executePluginResolvedStreamWithTimeout(plugin, runtime, capabilityPolicy, action, params, execCtx, timeoutOverride, emit)
}

func (s *PluginManagerService) executePluginResolvedWithTimeout(plugin *models.Plugin, runtime string, capabilityPolicy pluginCapabilityPolicy, action string, params map[string]string, execCtx *ExecutionContext, timeoutOverride time.Duration) (result *ExecutionResult, execErr error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	executionPlugin, runtimeSlot, releaseRuntimeSlot, resolveErr := s.resolveExecutionPluginRuntime(plugin, runtime, "execute_auto_reload")
	if resolveErr != nil {
		return nil, resolveErr
	}
	if releaseRuntimeSlot != nil {
		defer releaseRuntimeSlot()
	}
	mergedParams, mergeErr := mergeRuntimeParams(executionPlugin.RuntimeParams, params)
	if mergeErr != nil {
		return nil, fmt.Errorf("invalid plugin runtime_params: %w", mergeErr)
	}
	permit, permitErr := s.acquirePluginExecutionPermit(executionPlugin)
	if permitErr != nil {
		return nil, permitErr
	}
	defer func() {
		s.completePluginExecutionPermit(executionPlugin, permit, result, execErr)
	}()

	preparedExecCtx, task := s.startPluginExecutionTask(executionPlugin, runtime, action, mergedParams, execCtx, false)
	startTime := time.Now()
	result, execErr = s.executeWithRuntime(executionPlugin, runtimeSlot, runtime, capabilityPolicy, action, mergedParams, preparedExecCtx, timeoutOverride)
	if task != nil {
		snapshot := s.completePluginExecutionTask(task, result, execErr)
		preparedExecCtx.Metadata = mergePluginExecutionTaskMetadata(preparedExecCtx.Metadata, snapshot)
		result = applyPluginExecutionTaskToResult(result, snapshot)
	}
	duration := int(time.Since(startTime).Milliseconds())
	if duration < 0 {
		duration = 0
	}
	success := execErr == nil && result != nil && result.Success
	timedOut := isPluginExecutionTimeoutError(execErr)
	pluginobs.RecordExecution(executionPlugin.ID, executionPlugin.Name, runtime, action, int64(duration), success, timedOut)
	s.emitPluginExecutionAuditEvent(executionPlugin, runtime, action, mergedParams, preparedExecCtx, result, execErr, duration)

	// 记录执行历史
	s.recordExecution(executionPlugin.ID, action, mergedParams, preparedExecCtx, result, execErr, duration)

	return result, execErr
}

func (s *PluginManagerService) executePluginResolvedStreamWithTimeout(plugin *models.Plugin, runtime string, capabilityPolicy pluginCapabilityPolicy, action string, params map[string]string, execCtx *ExecutionContext, timeoutOverride time.Duration, emit ExecutionStreamEmitter) (result *ExecutionResult, execErr error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	executionPlugin, runtimeSlot, releaseRuntimeSlot, resolveErr := s.resolveExecutionPluginRuntime(plugin, runtime, "execute_stream_auto_reload")
	if resolveErr != nil {
		return nil, resolveErr
	}
	if releaseRuntimeSlot != nil {
		defer releaseRuntimeSlot()
	}
	mergedParams, mergeErr := mergeRuntimeParams(executionPlugin.RuntimeParams, params)
	if mergeErr != nil {
		return nil, fmt.Errorf("invalid plugin runtime_params: %w", mergeErr)
	}
	permit, permitErr := s.acquirePluginExecutionPermit(executionPlugin)
	if permitErr != nil {
		return nil, permitErr
	}
	defer func() {
		s.completePluginExecutionPermit(executionPlugin, permit, result, execErr)
	}()

	preparedExecCtx, task := s.startPluginExecutionTask(executionPlugin, runtime, action, mergedParams, execCtx, true)
	wrappedEmit := emit
	if task != nil {
		wrappedEmit = func(chunk *ExecutionStreamChunk) error {
			task.recordChunk()
			snapshot := task.snapshot()
			if chunk != nil {
				status := PluginExecutionStatusRunning
				if chunk.IsFinal {
					status = resolvePluginExecutionStatus(&ExecutionResult{
						TaskID:   snapshot.ID,
						Success:  chunk.Success,
						Data:     clonePayloadMap(chunk.Data),
						Error:    strings.TrimSpace(chunk.Error),
						Metadata: cloneStringMap(chunk.Metadata),
					}, nil)
				}
				chunk = applyPluginExecutionTaskToChunk(chunk, snapshot, status)
			}
			if emit == nil {
				return nil
			}
			return emit(chunk)
		}
	}
	startTime := time.Now()
	result, execErr = s.executeWithRuntimeStream(executionPlugin, runtimeSlot, runtime, capabilityPolicy, action, mergedParams, preparedExecCtx, timeoutOverride, wrappedEmit)
	if task != nil {
		snapshot := s.completePluginExecutionTask(task, result, execErr)
		preparedExecCtx.Metadata = mergePluginExecutionTaskMetadata(preparedExecCtx.Metadata, snapshot)
		result = applyPluginExecutionTaskToResult(result, snapshot)
	}
	duration := int(time.Since(startTime).Milliseconds())
	if duration < 0 {
		duration = 0
	}
	success := execErr == nil && result != nil && result.Success
	timedOut := isPluginExecutionTimeoutError(execErr)
	pluginobs.RecordExecution(executionPlugin.ID, executionPlugin.Name, runtime, action, int64(duration), success, timedOut)
	s.emitPluginExecutionAuditEvent(executionPlugin, runtime, action, mergedParams, preparedExecCtx, result, execErr, duration)
	s.recordExecution(executionPlugin.ID, action, mergedParams, preparedExecCtx, result, execErr, duration)

	return result, execErr
}

// ExecuteHook 执行业务 Hook（聚合所有启用插件结果）
func (s *PluginManagerService) ExecuteHook(req HookExecutionRequest, execCtx *ExecutionContext) (*HookExecutionResult, error) {
	hook := strings.TrimSpace(req.Hook)
	if hook == "" {
		return nil, fmt.Errorf("hook is required")
	}

	payload := clonePayloadMap(req.Payload)
	if payload == nil {
		payload = make(map[string]interface{})
	}

	result := &HookExecutionResult{
		Hook:               hook,
		Payload:            clonePayloadMap(payload),
		FrontendExtensions: make([]FrontendExtension, 0),
		PluginResults:      make([]HookPluginResult, 0),
	}
	if !s.isPluginPlatformEnabled() {
		return result, nil
	}

	catalogEntries := s.listHookExecutionCatalogEntries(hook)
	if len(catalogEntries) == 0 {
		return result, nil
	}

	executionPolicy := s.getHookExecutionPolicy()
	hookDefinition := resolveHookDefinition(hook)
	hookTimeout := s.resolveHookTimeoutForPhase(hookDefinition.Phase, executionPolicy)
	maxAttempts := executionPolicy.HookMaxRetries + 1
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	retryBackoff := time.Duration(executionPolicy.HookRetryBackoffMs) * time.Millisecond
	if retryBackoff < 0 {
		retryBackoff = 0
	}
	requestScope := resolveHookRequestAccessScope(execCtx)

	prepared := s.prepareHookPlugins(catalogEntries, hook, payload, requestScope, execCtx)
	if len(prepared) == 0 {
		result.Payload = payload
		return result, nil
	}

	if canExecuteHookPluginsInParallel(hookDefinition, prepared) {
		outcomes := s.executeHookPluginsInParallel(
			prepared,
			hook,
			payload,
			execCtx,
			hookTimeout,
			maxAttempts,
			retryBackoff,
			executionPolicy.HookMaxInFlight,
		)
		for _, outcome := range outcomes {
			var stop bool
			payload, stop = s.applyHookExecutionOutcome(result, hook, payload, outcome)
			if stop {
				break
			}
		}
	} else {
		for _, item := range prepared {
			outcome := s.executePreparedHookPlugin(
				item,
				hook,
				payload,
				execCtx,
				hookTimeout,
				maxAttempts,
				retryBackoff,
			)
			var stop bool
			payload, stop = s.applyHookExecutionOutcome(result, hook, payload, outcome)
			if stop {
				break
			}
		}
	}

	if len(result.FrontendExtensions) > 1 {
		sort.SliceStable(result.FrontendExtensions, func(i, j int) bool {
			return result.FrontendExtensions[i].Priority < result.FrontendExtensions[j].Priority
		})
	}
	result.Payload = payload
	return result, nil
}

func NewExecutionRequestCache() *ExecutionRequestCache {
	return &ExecutionRequestCache{
		frontendPreparedHookByKey:     make(map[string][]preparedHookPlugin),
		frontendPreparedInflightByKey: make(map[string]*preparedHookPluginsResolveCall),
	}
}

func clonePreparedHookPlugins(items []preparedHookPlugin) []preparedHookPlugin {
	if len(items) == 0 {
		return nil
	}
	out := make([]preparedHookPlugin, len(items))
	copy(out, items)
	return out
}

func (c *ExecutionRequestCache) resolveFrontendPreparedHookPlugins(
	cacheKey string,
	resolver func() []preparedHookPlugin,
) []preparedHookPlugin {
	if c == nil || strings.TrimSpace(cacheKey) == "" {
		return clonePreparedHookPlugins(resolver())
	}

	c.mu.Lock()
	if cached, exists := c.frontendPreparedHookByKey[cacheKey]; exists {
		pluginobs.RecordFrontendResolverCacheHit("prepared_hook", 1)
		out := clonePreparedHookPlugins(cached)
		c.mu.Unlock()
		return out
	}
	if call, exists := c.frontendPreparedInflightByKey[cacheKey]; exists && call != nil {
		pluginobs.RecordFrontendResolverSingleflightWait("prepared_hook", 1)
		c.mu.Unlock()
		<-call.done
		c.mu.Lock()
		cached := c.frontendPreparedHookByKey[cacheKey]
		out := clonePreparedHookPlugins(cached)
		c.mu.Unlock()
		return out
	}
	pluginobs.RecordFrontendResolverCacheMiss("prepared_hook", 1)
	call := &preparedHookPluginsResolveCall{done: make(chan struct{})}
	c.frontendPreparedInflightByKey[cacheKey] = call
	c.mu.Unlock()

	prepared := clonePreparedHookPlugins(resolver())

	c.mu.Lock()
	c.frontendPreparedHookByKey[cacheKey] = clonePreparedHookPlugins(prepared)
	if inflight, exists := c.frontendPreparedInflightByKey[cacheKey]; exists && inflight != nil {
		close(inflight.done)
		delete(c.frontendPreparedInflightByKey, cacheKey)
	}
	c.mu.Unlock()
	return prepared
}

func (s *PluginManagerService) prepareHookPlugins(
	entries []pluginExecutionCatalogEntry,
	hook string,
	payload map[string]interface{},
	requestScope hookRequestAccessScope,
	execCtx *ExecutionContext,
) []preparedHookPlugin {
	cacheKey := buildPreparedHookPluginsCacheKey(hook, payload, requestScope)
	var staticPrepared []preparedHookPlugin
	if execCtx != nil && execCtx.RequestCache != nil && cacheKey != "" {
		staticPrepared = execCtx.RequestCache.resolveFrontendPreparedHookPlugins(cacheKey, func() []preparedHookPlugin {
			return s.prepareHookPluginsStatic(entries, hook, payload, requestScope)
		})
	} else {
		staticPrepared = s.prepareHookPluginsStatic(entries, hook, payload, requestScope)
	}
	return s.filterPreparedHookPluginsByBreaker(staticPrepared, hook)
}

func (s *PluginManagerService) prepareHookPluginsStatic(
	entries []pluginExecutionCatalogEntry,
	hook string,
	payload map[string]interface{},
	requestScope hookRequestAccessScope,
) []preparedHookPlugin {
	prepared := make([]preparedHookPlugin, 0, len(entries))
	for i := range entries {
		entry := entries[i]
		plugin := entry.Plugin
		if strings.TrimSpace(entry.ValidationError) != "" {
			log.Printf("Skip plugin %s for hook %s: %s", plugin.Name, hook, entry.ValidationError)
			continue
		}
		capabilityPolicy := entry.CapabilityPolicy
		if !capabilityPolicy.SupportsHook(hook) {
			continue
		}
		if !capabilityPolicy.AllowsHookRequestAccess(hook, payload, requestScope) {
			continue
		}
		prepared = append(prepared, preparedHookPlugin{
			Plugin:           plugin,
			Runtime:          entry.Runtime,
			CapabilityPolicy: capabilityPolicy,
		})
	}
	return prepared
}

func (s *PluginManagerService) filterPreparedHookPluginsByBreaker(
	prepared []preparedHookPlugin,
	hook string,
) []preparedHookPlugin {
	if len(prepared) == 0 {
		return nil
	}
	filtered := make([]preparedHookPlugin, 0, len(prepared))
	for _, item := range prepared {
		breaker := s.inspectPluginExecutionBreaker(&item.Plugin)
		if breaker.State == pluginBreakerStateOpen || (breaker.State == pluginBreakerStateHalfOpen && breaker.ProbeInFlight) {
			log.Printf("Skip plugin %s for hook %s: %s", item.Plugin.Name, hook, breaker.Reason)
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func canExecuteHookPluginsInParallel(hookDefinition hookDefinition, prepared []preparedHookPlugin) bool {
	if len(prepared) <= 1 {
		return false
	}
	payloadPatchCanMutate := !hookDefinition.RestrictPayloadPatch || len(hookDefinition.WritablePayloadFields) > 0
	for _, item := range prepared {
		if item.CapabilityPolicy.AllowBlock {
			return false
		}
		if payloadPatchCanMutate && item.CapabilityPolicy.AllowPayloadPatch {
			return false
		}
	}
	return true
}

func resolveHookParallelWorkers(total int, maxInFlight int) int {
	if total <= 0 {
		return 0
	}
	if maxInFlight > 0 && total > maxInFlight {
		return maxInFlight
	}
	return total
}

func (s *PluginManagerService) executeHookPluginsInParallel(
	prepared []preparedHookPlugin,
	hook string,
	payload map[string]interface{},
	execCtx *ExecutionContext,
	hookTimeout time.Duration,
	maxAttempts int,
	retryBackoff time.Duration,
	maxInFlight int,
) []hookPluginExecutionOutcome {
	outcomes := make([]hookPluginExecutionOutcome, len(prepared))
	workerCount := resolveHookParallelWorkers(len(prepared), maxInFlight)
	if workerCount <= 0 {
		return outcomes
	}

	jobs := make(chan int)
	var wg sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				outcomes[idx] = s.executePreparedHookPlugin(
					prepared[idx],
					hook,
					payload,
					execCtx,
					hookTimeout,
					maxAttempts,
					retryBackoff,
				)
			}
		}()
	}
	for idx := range prepared {
		jobs <- idx
	}
	close(jobs)
	wg.Wait()
	return outcomes
}

func (s *PluginManagerService) executePreparedHookPlugin(
	item preparedHookPlugin,
	hook string,
	payload map[string]interface{},
	execCtx *ExecutionContext,
	hookTimeout time.Duration,
	maxAttempts int,
	retryBackoff time.Duration,
) hookPluginExecutionOutcome {
	outcome := hookPluginExecutionOutcome{
		Plugin:           item.Plugin,
		CapabilityPolicy: item.CapabilityPolicy,
	}

	hookParams, err := buildHookExecuteParams(hook, payload)
	if err != nil {
		outcome.HookParamsErr = err
		return outcome
	}

	startTime := time.Now()
	requestCtx := executionRequestContext(execCtx)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := requestCtx.Err(); err != nil {
			outcome.ExecErr = err
			break
		}
		release, limiterErr := s.acquireHookExecutionSlot(requestCtx, hookTimeout)
		if limiterErr != nil {
			pluginobs.RecordHookLimiterHit(hook)
			outcome.ExecErr = limiterErr
			break
		}
		outcome.ExecResult, outcome.ExecErr = s.executePluginResolvedWithTimeout(
			&item.Plugin,
			item.Runtime,
			item.CapabilityPolicy,
			"hook.execute",
			hookParams,
			execCtx,
			hookTimeout,
		)
		release()
		if outcome.ExecErr == nil {
			break
		}
		if attempt < maxAttempts && retryBackoff > 0 {
			if err := waitForExecutionRetryBackoff(requestCtx, retryBackoff); err != nil {
				outcome.ExecErr = err
				break
			}
		}
	}
	outcome.Duration = int(time.Since(startTime).Milliseconds())
	return outcome
}

func (s *PluginManagerService) applyHookExecutionOutcome(
	result *HookExecutionResult,
	hook string,
	payload map[string]interface{},
	outcome hookPluginExecutionOutcome,
) (map[string]interface{}, bool) {
	plugin := outcome.Plugin
	capabilityPolicy := outcome.CapabilityPolicy
	if outcome.HookParamsErr != nil {
		log.Printf("Failed to encode hook payload for plugin %s: %v", plugin.Name, outcome.HookParamsErr)
		return payload, false
	}

	pluginResult := HookPluginResult{
		PluginID:   plugin.ID,
		PluginName: plugin.Name,
		Duration:   outcome.Duration,
	}

	if outcome.ExecErr != nil {
		pluginResult.Success = false
		pluginResult.Error = outcome.ExecErr.Error()
		result.PluginResults = append(result.PluginResults, pluginResult)
		log.Printf("Hook %s execute failed on plugin %s: %v", hook, plugin.Name, outcome.ExecErr)
		return payload, false
	}
	if outcome.ExecResult == nil {
		pluginResult.Success = false
		pluginResult.Error = "empty plugin result"
		result.PluginResults = append(result.PluginResults, pluginResult)
		log.Printf("Hook %s execute returned empty result on plugin %s", hook, plugin.Name)
		return payload, false
	}

	pluginResult.Success = outcome.ExecResult.Success
	if outcome.ExecResult.Error != "" {
		pluginResult.Error = outcome.ExecResult.Error
	}
	result.PluginResults = append(result.PluginResults, pluginResult)

	if !outcome.ExecResult.Success {
		return payload, false
	}

	hookResp := parseHookResponse(outcome.ExecResult.Data)
	if hookResp == nil {
		return payload, false
	}

	if hookResp.Payload != nil && capabilityPolicy.AllowPayloadPatch {
		filteredPatch, droppedFields := sanitizeHookPayloadPatch(hook, hookResp.Payload)
		if droppedFields > 0 {
			log.Printf("Hook %s dropped %d non-writable payload fields from plugin %s", hook, droppedFields, plugin.Name)
		}
		if filteredPatch != nil {
			payload = mergePayloadMap(payload, filteredPatch)
		}
	} else if hookResp.Payload != nil {
		log.Printf("Hook %s ignored payload patch from plugin %s by capabilities policy", hook, plugin.Name)
	}

	if len(hookResp.FrontendExtensions) > 0 && capabilityPolicy.AllowFrontendExtensions {
		normalizedHook := normalizeHookName(hook)
		hookSlot := normalizedSlotValue(payload["slot"])
		for idx := range hookResp.FrontendExtensions {
			ext := hookResp.FrontendExtensions[idx]
			slot := normalizedSlotValue(ext.Slot)
			if slot == "" {
				slot = hookSlot
			}
			if normalizedHook != "frontend.bootstrap" && !capabilityPolicy.SupportsFrontendSlot(slot) {
				log.Printf("Hook %s dropped frontend extension from plugin %s due to slot policy (slot=%s)", hook, plugin.Name, slot)
				continue
			}
			ext.PluginID = plugin.ID
			if ext.PluginName == "" {
				if plugin.DisplayName != "" {
					ext.PluginName = plugin.DisplayName
				} else {
					ext.PluginName = plugin.Name
				}
			}
			result.FrontendExtensions = append(result.FrontendExtensions, ext)
		}
	} else if len(hookResp.FrontendExtensions) > 0 {
		log.Printf("Hook %s ignored frontend extensions from plugin %s by capabilities policy", hook, plugin.Name)
	}

	if hookResp.Blocked && !result.Blocked && capabilityPolicy.AllowBlock {
		result.Blocked = true
		if reason := strings.TrimSpace(hookResp.BlockReason); reason != "" {
			result.BlockReason = reason
		} else if plugin.DisplayName != "" {
			result.BlockReason = fmt.Sprintf("blocked by plugin %s", plugin.DisplayName)
		} else {
			result.BlockReason = fmt.Sprintf("blocked by plugin %s", plugin.Name)
		}
		return payload, true
	} else if hookResp.Blocked && !capabilityPolicy.AllowBlock {
		log.Printf("Hook %s ignored block response from plugin %s by capabilities policy", hook, plugin.Name)
	}

	return payload, false
}

// TestPlugin 主动测试插件健康状态
func (s *PluginManagerService) TestPlugin(pluginID uint) (*PluginHealth, error) {
	plugin, err := s.getPluginByID(pluginID)
	if err != nil {
		return nil, err
	}
	runtime, err := s.ResolveRuntime(plugin.Runtime)
	if err != nil {
		return nil, err
	}
	if err := s.ValidatePluginProfile(runtime, plugin.Type); err != nil {
		return nil, err
	}
	if err := ValidatePluginProtocolCompatibility(plugin); err != nil {
		return nil, err
	}

	health, err := s.healthCheckPluginByRuntime(plugin, runtime)
	if err != nil {
		s.updatePluginStatus(pluginID, "unhealthy")
		return nil, err
	}
	s.updatePluginStatus(pluginID, "healthy")
	return health, nil
}

// recordExecution 记录执行历史
func (s *PluginManagerService) recordExecution(pluginID uint, action string, params map[string]string, execCtx *ExecutionContext, result *ExecutionResult, err error, duration int) {
	if s == nil || s.db == nil {
		return
	}
	paramsJSON, _ := json.Marshal(sanitizeExecutionLogParams(params))

	execution := &models.PluginExecution{
		PluginID: pluginID,
		Action:   action,
		Params:   truncateExecutionLogText(string(paramsJSON)),
		Metadata: buildPersistedPluginExecutionMetadata(execCtx, result),
		Duration: duration,
	}
	execution.Hook = resolvePersistedPluginExecutionHook(action, params, execution.Metadata)

	if execCtx != nil {
		execution.UserID = execCtx.UserID
		execution.OrderID = execCtx.OrderID
	}

	if err != nil {
		execution.Success = false
		execution.Error = truncateExecutionLogText(err.Error())
	} else if result != nil {
		execution.Success = result.Success
		if result.Data != nil {
			resultJSON, _ := json.Marshal(sanitizeExecutionLogValue("", result.Data))
			execution.Result = truncateExecutionLogText(string(resultJSON))
		}
		execution.Error = truncateExecutionLogText(result.Error)
	}
	if !execution.Success {
		execution.ErrorSignature = NormalizePluginExecutionErrorSignature(
			NormalizePluginExecutionErrorText(execution.Error),
		)
	}
	if execution.CreatedAt.IsZero() {
		execution.CreatedAt = time.Now().UTC()
	}

	s.persistPluginExecutionRecord(execution)
}

func (s *PluginManagerService) startPluginAuditLogWorker(stopChan <-chan struct{}) {
	if s == nil || pluginAuditLogQueueSize <= 0 || stopChan == nil {
		return
	}

	queue := make(chan pluginExecutionAuditEntry, pluginAuditLogQueueSize)

	s.auditMu.Lock()
	s.auditLogQueue = queue
	s.auditMu.Unlock()
	s.auditLogDropped.Store(0)

	s.auditLogWorkerWG.Add(1)
	go func(stopSignal <-chan struct{}, auditQueue <-chan pluginExecutionAuditEntry) {
		defer s.auditLogWorkerWG.Done()
		s.pluginExecutionAuditLoop(stopSignal, auditQueue)
	}(stopChan, queue)
}

func (s *PluginManagerService) getPluginAuditLogQueue() chan pluginExecutionAuditEntry {
	if s == nil {
		return nil
	}

	s.auditMu.RLock()
	defer s.auditMu.RUnlock()
	return s.auditLogQueue
}

func (s *PluginManagerService) tryEnqueuePluginExecutionAuditEntry(entry pluginExecutionAuditEntry) pluginExecutionAuditQueueStatus {
	queue := s.getPluginAuditLogQueue()
	if queue == nil {
		return pluginExecutionAuditQueueSyncFallback
	}

	select {
	case queue <- entry:
		return pluginExecutionAuditQueueEnqueued
	default:
		s.auditLogDropped.Add(1)
		return pluginExecutionAuditQueueDropped
	}
}

func (s *PluginManagerService) pluginExecutionAuditLoop(stopChan <-chan struct{}, queue <-chan pluginExecutionAuditEntry) {
	if queue == nil {
		return
	}

	ticker := time.NewTicker(pluginAuditLogFlushTick)
	defer ticker.Stop()
	defer s.flushDroppedPluginExecutionAuditLogs()

	for {
		select {
		case entry := <-queue:
			s.writePluginExecutionAuditEntry(entry)
		case <-ticker.C:
			s.flushDroppedPluginExecutionAuditLogs()
		case <-stopChan:
			for {
				select {
				case entry := <-queue:
					s.writePluginExecutionAuditEntry(entry)
				default:
					return
				}
			}
		}
	}
}

func (s *PluginManagerService) flushDroppedPluginExecutionAuditLogs() {
	if s == nil {
		return
	}

	dropped := s.auditLogDropped.Swap(0)
	if dropped == 0 {
		return
	}

	log.Printf("[plugin.audit] dropped %d execution audit events because async log queue was full", dropped)
}

func (s *PluginManagerService) preparePluginExecutionAuditEntry(
	plugin *models.Plugin,
	runtime string,
	action string,
	params map[string]string,
	execCtx *ExecutionContext,
	result *ExecutionResult,
	execErr error,
	durationMs int,
) pluginExecutionAuditEntry {
	if durationMs < 0 {
		durationMs = 0
	}

	entry := pluginExecutionAuditEntry{
		Timestamp:  time.Now().UTC(),
		PluginID:   plugin.ID,
		PluginName: plugin.Name,
		Runtime:    strings.ToLower(strings.TrimSpace(runtime)),
		Action:     strings.TrimSpace(action),
		DurationMs: durationMs,
		Params:     cloneStringMap(params),
	}

	if execCtx != nil {
		entry.HasContext = true
		entry.SessionID = strings.TrimSpace(execCtx.SessionID)
		entry.ContextMetadata = cloneStringMap(execCtx.Metadata)
		entry.UserID = cloneOptionalExecutionUint(execCtx.UserID)
		entry.OrderID = cloneOptionalExecutionUint(execCtx.OrderID)
	}

	switch {
	case execErr != nil:
		entry.Success = false
		entry.Error = truncateExecutionLogText(execErr.Error())
	case result != nil:
		entry.Success = result.Success
		entry.Error = truncateExecutionLogText(result.Error)
		entry.ResultData = clonePayloadMap(result.Data)
	default:
		entry.Success = false
		entry.Error = "empty execution result"
	}

	return entry
}

func (s *PluginManagerService) buildPluginExecutionAuditPayload(entry pluginExecutionAuditEntry) map[string]interface{} {
	payload := map[string]interface{}{
		"type":        "plugin.execution",
		"timestamp":   entry.Timestamp.UTC().Format(time.RFC3339Nano),
		"plugin_id":   entry.PluginID,
		"plugin_name": entry.PluginName,
		"runtime":     entry.Runtime,
		"action":      entry.Action,
		"duration_ms": entry.DurationMs,
		"params":      sanitizeExecutionLogParams(entry.Params),
		"success":     entry.Success,
		"error":       truncateExecutionLogText(entry.Error),
	}

	if entry.HasContext {
		contextPayload := map[string]interface{}{
			"session_id": entry.SessionID,
			"metadata":   sanitizeExecutionLogParams(entry.ContextMetadata),
		}
		if entry.UserID != nil {
			contextPayload["user_id"] = *entry.UserID
		}
		if entry.OrderID != nil {
			contextPayload["order_id"] = *entry.OrderID
		}
		payload["context"] = contextPayload
	}

	if entry.ResultData != nil {
		payload["result"] = sanitizeExecutionLogValue("", entry.ResultData)
	}

	return payload
}

func (s *PluginManagerService) writePluginExecutionAuditEntry(entry pluginExecutionAuditEntry) {
	payload, err := json.Marshal(s.buildPluginExecutionAuditPayload(entry))
	if err != nil {
		log.Printf("Failed to marshal plugin audit event for plugin %d: %v", entry.PluginID, err)
		return
	}
	log.Printf("[plugin.audit] %s", truncateExecutionLogText(string(payload)))
}

func (s *PluginManagerService) emitPluginExecutionAuditEvent(plugin *models.Plugin, runtime string, action string, params map[string]string, execCtx *ExecutionContext, result *ExecutionResult, execErr error, durationMs int) {
	if plugin == nil {
		return
	}

	entry := s.preparePluginExecutionAuditEntry(plugin, runtime, action, params, execCtx, result, execErr, durationMs)
	switch s.tryEnqueuePluginExecutionAuditEntry(entry) {
	case pluginExecutionAuditQueueEnqueued, pluginExecutionAuditQueueDropped:
		return
	default:
		s.writePluginExecutionAuditEntry(entry)
	}
}

func isPluginExecutionTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	errMsg := strings.ToLower(strings.TrimSpace(err.Error()))
	if errMsg == "" {
		return false
	}
	return strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "deadline exceeded")
}

func cloneExecutionContextMetadata(input map[string]string) map[string]string {
	if len(input) == 0 {
		if input == nil {
			return nil
		}
		return map[string]string{}
	}
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func buildPersistedPluginExecutionMetadata(execCtx *ExecutionContext, result *ExecutionResult) models.JSONMap {
	merged := cloneExecutionContextMetadata(nil)
	if execCtx != nil && len(execCtx.Metadata) > 0 {
		merged = cloneExecutionContextMetadata(execCtx.Metadata)
	}
	if result != nil && len(result.Metadata) > 0 {
		if merged == nil {
			merged = make(map[string]string, len(result.Metadata))
		}
		for key, value := range result.Metadata {
			merged[key] = value
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return sanitizeExecutionRecordMetadata(merged)
}

func sanitizeExecutionRecordMetadata(metadata map[string]string) models.JSONMap {
	if len(metadata) == 0 {
		return nil
	}

	sanitized := make(models.JSONMap, len(metadata))
	for key, value := range metadata {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		if isSensitiveExecutionLogKey(normalizedKey) {
			sanitized[key] = "[REDACTED]"
			continue
		}
		sanitized[key] = truncateExecutionLogText(value)
	}
	return sanitized
}

func resolvePersistedPluginExecutionHook(
	action string,
	params map[string]string,
	metadata map[string]string,
) string {
	if !strings.EqualFold(strings.TrimSpace(action), "hook.execute") {
		return ""
	}
	if hook := normalizeHookName(metadata[PluginExecutionMetadataHook]); hook != "" {
		return truncateExecutionLogText(hook)
	}
	return truncateExecutionLogText(extractPluginExecutionHook(action, params))
}

func sanitizeExecutionLogParams(params map[string]string) map[string]interface{} {
	if len(params) == 0 {
		return map[string]interface{}{}
	}

	out := make(map[string]interface{}, len(params))
	for key, value := range params {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		switch {
		case isSensitiveExecutionLogKey(normalizedKey):
			out[key] = "[REDACTED]"
		case normalizedKey == "payload":
			var decoded interface{}
			if err := json.Unmarshal([]byte(value), &decoded); err != nil {
				out[key] = truncateExecutionLogText(value)
				continue
			}
			payloadJSON, marshalErr := json.Marshal(sanitizeExecutionLogValue(normalizedKey, decoded))
			if marshalErr != nil {
				out[key] = "[REDACTED]"
				continue
			}
			out[key] = truncateExecutionLogText(string(payloadJSON))
		default:
			out[key] = truncateExecutionLogText(value)
		}
	}
	return out
}

func sanitizeExecutionLogValue(key string, value interface{}) interface{} {
	normalizedKey := strings.ToLower(strings.TrimSpace(key))
	if isSensitiveExecutionLogKey(normalizedKey) {
		return "[REDACTED]"
	}

	switch typed := value.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for itemKey, itemValue := range typed {
			out[itemKey] = sanitizeExecutionLogValue(itemKey, itemValue)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for idx, item := range typed {
			out[idx] = sanitizeExecutionLogValue(normalizedKey, item)
		}
		return out
	case string:
		return truncateExecutionLogText(typed)
	default:
		return typed
	}
}

func isSensitiveExecutionLogKey(key string) bool {
	if key == "" {
		return false
	}

	containsPatterns := []string{
		"password",
		"passwd",
		"pwd",
		"token",
		"secret",
		"authorization",
		"api_key",
		"apikey",
		"access_key",
		"refresh_key",
	}
	for _, pattern := range containsPatterns {
		if strings.Contains(key, pattern) {
			return true
		}
	}
	return false
}

func truncateExecutionLogText(text string) string {
	if text == "" {
		return ""
	}

	runes := []rune(text)
	if len(runes) <= pluginExecutionLogMaxChars {
		return text
	}
	return string(runes[:pluginExecutionLogMaxChars]) + "...(truncated)"
}

// UnregisterPlugin 卸载插件并关闭连接
func (s *PluginManagerService) UnregisterPlugin(pluginID uint) {
	defer releasePluginStorageLock(pluginID)
	s.invalidatePluginStorageSnapshot(pluginID)
	s.invalidatePluginSecretSnapshot(pluginID)
	s.breakerMu.Lock()
	delete(s.executionBreakers, pluginID)
	s.breakerMu.Unlock()

	plugin, _ := s.getPluginByID(pluginID)
	slot := s.deactivateActiveRuntimeSlot(pluginID)
	if slot != nil {
		log.Printf("Plugin unregistered: %d (generation=%d, runtime=%s)", pluginID, slot.Generation, slot.Runtime)
		return
	}

	if plugin != nil {
		runtime, err := s.ResolveRuntime(plugin.Runtime)
		if err == nil && runtime == PluginRuntimeJSWorker {
			log.Printf("JS plugin unregistered: %d", pluginID)
		}
	}
}

// ReloadPlugin 重新加载插件
func (s *PluginManagerService) ReloadPlugin(pluginID uint) error {
	return s.reloadPluginWithTrigger(pluginID, "reload")
}

func (s *PluginManagerService) reloadPluginWithTrigger(pluginID uint, trigger string) error {
	var plugin models.Plugin
	if err := s.db.First(&plugin, pluginID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if plugin.Enabled {
		if err := s.registerPluginWithTrigger(&plugin, trigger); err != nil {
			return err
		}
		return s.RefreshPluginExecutionCatalog()
	}
	s.UnregisterPlugin(pluginID)
	return s.RefreshPluginExecutionCatalog()
}

// InstallPlugin 标记插件已安装（可执行但未运行）
func (s *PluginManagerService) InstallPlugin(pluginID uint) error {
	now := time.Now()
	return s.updatePluginLifecycle(pluginID, models.PluginLifecycleInstalled, map[string]interface{}{
		"installed_at": now,
		"last_error":   "",
		"retired_at":   nil,
	})
}

// StartPlugin 启动插件（建立连接并开始健康检查）
func (s *PluginManagerService) StartPlugin(pluginID uint) error {
	var plugin models.Plugin
	if err := s.db.First(&plugin, pluginID).Error; err != nil {
		return err
	}

	update := map[string]interface{}{
		"enabled":          true,
		"retired_at":       nil,
		"lifecycle_status": models.PluginLifecycleInstalled,
		"last_error":       "",
	}
	if err := s.db.Model(&models.Plugin{}).Where("id = ?", pluginID).Updates(update).Error; err != nil {
		return err
	}

	if err := s.reloadPluginWithTrigger(pluginID, "start"); err != nil {
		s.updatePluginLifecycle(pluginID, models.PluginLifecycleDegraded, map[string]interface{}{
			"last_error": err.Error(),
		})
		return err
	}
	return nil
}

// PausePlugin 暂停插件（断开连接但保留安装信息）
func (s *PluginManagerService) PausePlugin(pluginID uint) error {
	s.UnregisterPlugin(pluginID)
	now := time.Now()
	if err := s.db.Model(&models.Plugin{}).Where("id = ?", pluginID).Updates(map[string]interface{}{
		"enabled":          false,
		"lifecycle_status": models.PluginLifecyclePaused,
		"stopped_at":       now,
		"last_error":       "",
	}).Error; err != nil {
		return err
	}
	s.removePluginExecutionCatalogEntry(pluginID)
	return nil
}

// RestartPlugin 重启插件
func (s *PluginManagerService) RestartPlugin(pluginID uint) error {
	s.UnregisterPlugin(pluginID)
	return s.StartPlugin(pluginID)
}

// RetirePlugin 退役插件（停止并禁用）
func (s *PluginManagerService) RetirePlugin(pluginID uint) error {
	s.UnregisterPlugin(pluginID)
	now := time.Now()
	if err := s.db.Model(&models.Plugin{}).Where("id = ?", pluginID).Updates(map[string]interface{}{
		"enabled":          false,
		"lifecycle_status": models.PluginLifecycleRetired,
		"retired_at":       now,
		"stopped_at":       now,
		"last_error":       "",
	}).Error; err != nil {
		return err
	}
	s.removePluginExecutionCatalogEntry(pluginID)
	return nil
}

func (s *PluginManagerService) updatePluginLifecycle(pluginID uint, lifecycle string, extra map[string]interface{}) error {
	update := map[string]interface{}{
		"lifecycle_status": lifecycle,
	}
	for key, value := range extra {
		update[key] = value
	}
	if err := s.db.Model(&models.Plugin{}).Where("id = ?", pluginID).Updates(update).Error; err != nil {
		log.Printf("Failed to update plugin %d lifecycle: %v", pluginID, err)
		return err
	}
	if strings.TrimSpace(lifecycle) == "" {
		return nil
	}
	if err := s.db.Model(&models.PluginVersion{}).
		Where("plugin_id = ? AND is_active = ?", pluginID, true).
		Update("lifecycle_status", lifecycle).Error; err != nil {
		// The plugin table is the source of truth for runtime state. Keep the main
		// lifecycle update even if the active version mirror cannot be synchronized.
		log.Printf("Failed to sync active plugin version lifecycle for plugin %d: %v", pluginID, err)
	}
	return nil
}

func (s *PluginManagerService) prepareGRPCPluginSlot(plugin *models.Plugin) (*pluginRuntimeSlot, error) {
	if strings.TrimSpace(plugin.Address) == "" {
		return nil, fmt.Errorf("plugin %s address is empty", plugin.Name)
	}

	pluginCfg := s.getPluginPlatformConfig()
	client := NewPluginClient(plugin.ID, plugin.Name, plugin.Address, pluginCfg.GRPC)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return nil, err
	}

	return newPluginRuntimeSlot(plugin, PluginRuntimeGRPC, client), nil
}

func (s *PluginManagerService) prepareJSPluginSlot(plugin *models.Plugin) (*pluginRuntimeSlot, error) {
	if strings.TrimSpace(plugin.Address) == "" {
		return nil, fmt.Errorf("plugin %s address is empty", plugin.Name)
	}
	if s.jsWorker == nil {
		return nil, fmt.Errorf("js worker supervisor is not initialized")
	}
	_, err := s.healthCheckJSPlugin(plugin)
	if err != nil {
		return nil, err
	}
	slot := newPluginRuntimeSlot(plugin, PluginRuntimeJSWorker, nil)
	if slot != nil {
		pluginID := plugin.ID
		generation := resolvePluginAppliedGeneration(plugin)
		slot.onClose = func() {
			if s == nil || s.jsWorker == nil {
				return
			}
			if err := s.jsWorker.DisposePluginRuntime(pluginID, generation); err != nil {
				log.Printf("Dispose js runtime failed for plugin %d generation=%d: %v", pluginID, generation, err)
			}
		}
	}
	return slot, nil
}

func (s *PluginManagerService) performJSPluginHealthChecks() {
	if !s.isRuntimeAllowed(PluginRuntimeJSWorker) {
		return
	}

	slots := s.listActiveRuntimeSlots(PluginRuntimeJSWorker)
	for _, slot := range slots {
		if slot == nil {
			continue
		}
		plugin := slot.PluginSnapshot
		if _, err := s.healthCheckJSPlugin(&plugin); err != nil {
			s.updatePluginStatus(plugin.ID, "unhealthy")
			s.releasePluginRuntimeSlot(slot)
			continue
		}
		s.updatePluginStatus(plugin.ID, "healthy")
		s.releasePluginRuntimeSlot(slot)
	}
}

func (s *PluginManagerService) executeWithRuntime(plugin *models.Plugin, runtimeSlot *pluginRuntimeSlot, runtime string, capabilityPolicy pluginCapabilityPolicy, action string, params map[string]string, execCtx *ExecutionContext, timeoutOverride time.Duration) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	if !plugin.Enabled {
		return nil, fmt.Errorf("plugin %d is disabled", plugin.ID)
	}
	switch runtime {
	case PluginRuntimeGRPC:
		return s.executeWithGRPC(runtimeSlot, action, params, execCtx, timeoutOverride)
	case PluginRuntimeJSWorker:
		return s.executeWithJSPlugin(plugin, capabilityPolicy, action, params, execCtx, timeoutOverride)
	default:
		return nil, fmt.Errorf("unsupported plugin runtime %q", runtime)
	}
}

func (s *PluginManagerService) executeWithRuntimeStream(plugin *models.Plugin, runtimeSlot *pluginRuntimeSlot, runtime string, capabilityPolicy pluginCapabilityPolicy, action string, params map[string]string, execCtx *ExecutionContext, timeoutOverride time.Duration, emit ExecutionStreamEmitter) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	if !plugin.Enabled {
		return nil, fmt.Errorf("plugin %d is disabled", plugin.ID)
	}
	switch runtime {
	case PluginRuntimeGRPC:
		return s.executeWithGRPCStream(runtimeSlot, action, params, execCtx, timeoutOverride, emit)
	case PluginRuntimeJSWorker:
		return s.executeWithJSPluginStream(plugin, capabilityPolicy, action, params, execCtx, timeoutOverride, emit)
	default:
		return nil, fmt.Errorf("unsupported plugin runtime %q", runtime)
	}
}

func (s *PluginManagerService) executeWithGRPC(slot *pluginRuntimeSlot, action string, params map[string]string, execCtx *ExecutionContext, timeoutOverride time.Duration) (*ExecutionResult, error) {
	if slot == nil || slot.GRPCClient == nil {
		return nil, fmt.Errorf("grpc runtime slot is unavailable")
	}

	ctx, cancel := s.executeTimeoutContext(execCtx, timeoutOverride)
	defer cancel()

	return slot.GRPCClient.Execute(ctx, action, params, execCtx)
}

func (s *PluginManagerService) executeWithGRPCStream(slot *pluginRuntimeSlot, action string, params map[string]string, execCtx *ExecutionContext, timeoutOverride time.Duration, emit ExecutionStreamEmitter) (*ExecutionResult, error) {
	if slot == nil || slot.GRPCClient == nil {
		return nil, fmt.Errorf("grpc runtime slot is unavailable")
	}

	ctx, cancel := s.executeTimeoutContext(execCtx, timeoutOverride)
	defer cancel()

	return slot.GRPCClient.ExecuteStream(ctx, action, params, execCtx, emit)
}

func (s *PluginManagerService) executeWithJSPlugin(plugin *models.Plugin, capabilityPolicy pluginCapabilityPolicy, action string, params map[string]string, execCtx *ExecutionContext, timeoutOverride time.Duration) (*ExecutionResult, error) {
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

	result, err := s.jsWorker.ExecuteWithTimeoutAndStorage(plugin, action, params, storageSnapshot, secretSnapshot, execCtx, timeoutOverride)
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

func (s *PluginManagerService) executeWithJSPluginStream(plugin *models.Plugin, capabilityPolicy pluginCapabilityPolicy, action string, params map[string]string, execCtx *ExecutionContext, timeoutOverride time.Duration, emit ExecutionStreamEmitter) (*ExecutionResult, error) {
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

	result, err := s.jsWorker.ExecuteStreamWithTimeoutAndStorage(plugin, action, params, storageSnapshot, secretSnapshot, execCtx, timeoutOverride, emit)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("plugin returned empty execution result")
	}
	if validateErr := validatePluginStorageAccessMode(
		declaredStorageMode,
		resolvePluginStorageAccessModeFromMetadata(result.Metadata),
		result.StorageChanged,
	); validateErr != nil {
		return nil, validateErr
	}
	if result.StorageChanged {
		if persistErr := s.replacePluginStorageSnapshot(plugin.ID, result.Storage); persistErr != nil {
			return nil, fmt.Errorf("persist plugin storage failed: %w", persistErr)
		}
	}
	return result, nil
}

func (s *PluginManagerService) healthCheckPluginByRuntime(plugin *models.Plugin, runtime string) (*PluginHealth, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	switch runtime {
	case PluginRuntimeGRPC:
		return s.healthCheckGRPCPlugin(plugin.ID)
	case PluginRuntimeJSWorker:
		return s.healthCheckJSPlugin(plugin)
	default:
		return nil, fmt.Errorf("unsupported plugin runtime %q", runtime)
	}
}

func (s *PluginManagerService) healthCheckGRPCPlugin(pluginID uint) (*PluginHealth, error) {
	slot, exists := s.acquirePluginRuntimeSlot(pluginID, PluginRuntimeGRPC, 0)
	if !exists {
		if err := s.reloadPluginWithTrigger(pluginID, "healthcheck_auto_reload"); err != nil {
			return nil, err
		}
		slot, exists = s.acquirePluginRuntimeSlot(pluginID, PluginRuntimeGRPC, 0)
	}
	if !exists {
		return nil, fmt.Errorf("plugin %d not loaded", pluginID)
	}
	defer s.releasePluginRuntimeSlot(slot)
	if slot.GRPCClient == nil {
		return nil, fmt.Errorf("plugin %d grpc runtime slot has no client", pluginID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health, err := slot.GRPCClient.HealthCheck(ctx)
	if err != nil {
		slot.GRPCClient.RecordFailure()
		return nil, err
	}
	return health, nil
}

func (s *PluginManagerService) healthCheckJSPlugin(plugin *models.Plugin) (*PluginHealth, error) {
	if s.jsWorker == nil {
		return nil, fmt.Errorf("js worker supervisor is not initialized")
	}
	secretSnapshot, err := s.loadPluginSecretSnapshot(plugin.ID)
	if err != nil {
		return nil, fmt.Errorf("load plugin secrets failed: %w", err)
	}
	return s.jsWorker.HealthCheck(plugin, secretSnapshot)
}

func (s *PluginManagerService) getPluginByID(pluginID uint) (*models.Plugin, error) {
	var plugin models.Plugin
	if err := s.db.First(&plugin, pluginID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("plugin %d not found", pluginID)
		}
		return nil, err
	}
	return &plugin, nil
}

func (s *PluginManagerService) ResolveRuntime(runtime string) (string, error) {
	if !s.isPluginPlatformEnabled() {
		return "", fmt.Errorf("plugin platform is disabled by system settings")
	}

	pluginCfg := s.getPluginPlatformConfig()
	resolved := strings.ToLower(strings.TrimSpace(runtime))
	if resolved == "" {
		resolved = pluginCfg.DefaultRuntime
	}
	switch resolved {
	case PluginRuntimeGRPC, PluginRuntimeJSWorker:
	default:
		return "", fmt.Errorf("unsupported plugin runtime %q", resolved)
	}
	if !containsString(pluginCfg.AllowedRuntimes, resolved) {
		return "", fmt.Errorf("plugin runtime %q is not allowed by system settings", resolved)
	}
	return resolved, nil
}

func (s *PluginManagerService) ValidatePluginProfile(runtime string, pluginType string) error {
	if _, err := s.ResolveRuntime(runtime); err != nil {
		return err
	}

	normalizedType := strings.ToLower(strings.TrimSpace(pluginType))
	if normalizedType == "" {
		return fmt.Errorf("plugin type is required")
	}

	pluginCfg := s.getPluginPlatformConfig()
	if len(pluginCfg.AllowedTypes) == 0 {
		return nil
	}
	if !containsString(pluginCfg.AllowedTypes, normalizedType) {
		return fmt.Errorf("plugin type %q is not allowed by system settings", normalizedType)
	}
	return nil
}

func (s *PluginManagerService) isPluginPlatformEnabled() bool {
	return s.getPluginPlatformConfig().Enabled
}

func (s *PluginManagerService) isRuntimeAllowed(runtime string) bool {
	pluginCfg := s.getPluginPlatformConfig()
	resolved := strings.ToLower(strings.TrimSpace(runtime))
	if resolved == "" {
		return false
	}
	return containsString(pluginCfg.AllowedRuntimes, resolved)
}

func (s *PluginManagerService) resolveExecuteTimeout(timeoutOverride time.Duration) time.Duration {
	if timeoutOverride > 0 {
		return timeoutOverride
	}
	pluginCfg := s.getPluginPlatformConfig()
	timeout := pluginCfg.Sandbox.ExecTimeoutMs
	if timeout <= 0 {
		timeout = 30000
	}
	return time.Duration(timeout) * time.Millisecond
}

func (s *PluginManagerService) executeTimeoutContext(execCtx *ExecutionContext, timeoutOverride time.Duration) (context.Context, context.CancelFunc) {
	timeout := s.resolveExecuteTimeout(timeoutOverride)
	return context.WithTimeout(executionRequestContext(execCtx), timeout)
}

func (s *PluginManagerService) getHookExecutionPolicy() config.PluginExecutionPolicyConfig {
	pluginCfg := s.getPluginPlatformConfig()
	return pluginCfg.Execution
}

func normalizePluginFailureThreshold(policy config.PluginExecutionPolicyConfig) int {
	if policy.FailureThreshold <= 0 {
		return 3
	}
	return policy.FailureThreshold
}

func buildPluginExecutionBreakerRuntimeFromPlugin(
	plugin *models.Plugin,
	policy config.PluginExecutionPolicyConfig,
) pluginExecutionBreakerRuntime {
	runtime := pluginExecutionBreakerRuntime{}
	if plugin == nil {
		return runtime
	}

	runtime.ConsecutiveFailures = plugin.FailCount
	if runtime.ConsecutiveFailures < 0 {
		runtime.ConsecutiveFailures = 0
	}
	if runtime.ConsecutiveFailures <= 0 {
		return runtime
	}

	runtime.LastFailureAt = plugin.UpdatedAt.UTC()
	if threshold := normalizePluginFailureThreshold(policy); runtime.ConsecutiveFailures >= threshold {
		cooldown := time.Duration(policy.FailureCooldownMs) * time.Millisecond
		if cooldown > 0 && !runtime.LastFailureAt.IsZero() {
			runtime.OpenUntil = runtime.LastFailureAt.Add(cooldown)
		}
	}
	return runtime
}

func resolvePluginExecutionBreakerState(
	plugin *models.Plugin,
	runtime pluginExecutionBreakerRuntime,
	policy config.PluginExecutionPolicyConfig,
	now time.Time,
) pluginExecutionBreakerState {
	state := pluginExecutionBreakerState{
		State:               pluginBreakerStateClosed,
		ConsecutiveFailures: runtime.ConsecutiveFailures,
		FailureThreshold:    normalizePluginFailureThreshold(policy),
		ProbeInFlight:       runtime.ProbeInFlight,
	}
	if !runtime.ProbeStartedAt.IsZero() {
		probeStartedAt := runtime.ProbeStartedAt.UTC()
		state.ProbeStartedAt = &probeStartedAt
	}
	if plugin == nil {
		return state
	}

	if state.ConsecutiveFailures < state.FailureThreshold {
		return state
	}

	if !runtime.OpenUntil.IsZero() && now.Before(runtime.OpenUntil) {
		cooldownUntil := runtime.OpenUntil.UTC()
		state.State = pluginBreakerStateOpen
		state.CooldownActive = true
		state.CooldownUntil = &cooldownUntil
		state.ReasonCode = "failure_cooldown"
		state.Reason = fmt.Sprintf(
			"plugin circuit breaker is open after %d consecutive failures until %s",
			state.ConsecutiveFailures,
			cooldownUntil.Format(time.RFC3339),
		)
		return state
	}

	state.State = pluginBreakerStateHalfOpen
	if state.ProbeInFlight {
		state.ReasonCode = "half_open_probe_in_flight"
		state.Reason = "plugin circuit breaker is half-open and a recovery probe is already running"
		return state
	}
	state.ReasonCode = "half_open_ready"
	state.Reason = "plugin circuit breaker is half-open and ready for a single recovery probe"
	return state
}

func (s *PluginManagerService) getOrSyncPluginExecutionBreakerLocked(
	plugin *models.Plugin,
	policy config.PluginExecutionPolicyConfig,
) *pluginExecutionBreakerRuntime {
	if plugin == nil {
		return nil
	}
	if s.executionBreakers == nil {
		s.executionBreakers = make(map[uint]*pluginExecutionBreakerRuntime)
	}

	snapshot := buildPluginExecutionBreakerRuntimeFromPlugin(plugin, policy)
	entry, exists := s.executionBreakers[plugin.ID]
	if !exists || entry == nil {
		entry = &snapshot
		s.executionBreakers[plugin.ID] = entry
		return entry
	}

	switch {
	case snapshot.ConsecutiveFailures == 0 &&
		strings.EqualFold(strings.TrimSpace(plugin.Status), "healthy") &&
		!strings.EqualFold(strings.TrimSpace(plugin.LifecycleStatus), models.PluginLifecycleDegraded):
		entry.ConsecutiveFailures = 0
		entry.LastFailureAt = time.Time{}
		entry.OpenUntil = time.Time{}
	case snapshot.ConsecutiveFailures > entry.ConsecutiveFailures ||
		(snapshot.LastFailureAt.After(entry.LastFailureAt) && snapshot.ConsecutiveFailures >= entry.ConsecutiveFailures):
		entry.ConsecutiveFailures = snapshot.ConsecutiveFailures
		entry.LastFailureAt = snapshot.LastFailureAt
		entry.OpenUntil = snapshot.OpenUntil
	}
	return entry
}

func (s *PluginManagerService) inspectPluginExecutionBreaker(plugin *models.Plugin) pluginExecutionBreakerState {
	policy := s.getHookExecutionPolicy()
	s.breakerMu.Lock()
	entry := s.getOrSyncPluginExecutionBreakerLocked(plugin, policy)
	state := resolvePluginExecutionBreakerState(plugin, pluginExecutionBreakerRuntime{}, policy, time.Now().UTC())
	if entry != nil {
		state = resolvePluginExecutionBreakerState(plugin, *entry, policy, time.Now().UTC())
	}
	s.breakerMu.Unlock()
	return state
}

func (s *PluginManagerService) acquirePluginExecutionPermit(plugin *models.Plugin) (pluginExecutionPermit, error) {
	permit := pluginExecutionPermit{}
	if plugin == nil {
		return permit, fmt.Errorf("plugin is nil")
	}

	policy := s.getHookExecutionPolicy()
	now := time.Now().UTC()

	s.breakerMu.Lock()
	entry := s.getOrSyncPluginExecutionBreakerLocked(plugin, policy)
	if entry == nil {
		s.breakerMu.Unlock()
		return permit, fmt.Errorf("plugin breaker is unavailable")
	}
	state := resolvePluginExecutionBreakerState(plugin, *entry, policy, now)
	permit = pluginExecutionPermit{
		PluginID:     plugin.ID,
		BreakerState: state,
	}
	switch state.State {
	case pluginBreakerStateOpen:
		s.breakerMu.Unlock()
		return permit, formatPluginExecutionBreakerError(plugin, state)
	case pluginBreakerStateHalfOpen:
		if entry.ProbeInFlight {
			s.breakerMu.Unlock()
			return permit, formatPluginExecutionBreakerError(plugin, state)
		}
		entry.ProbeInFlight = true
		entry.ProbeStartedAt = now
		permit.HalfOpenProbe = true
	}
	s.breakerMu.Unlock()
	return permit, nil
}

func (s *PluginManagerService) completePluginExecutionPermit(
	plugin *models.Plugin,
	permit pluginExecutionPermit,
	result *ExecutionResult,
	execErr error,
) {
	if plugin == nil || permit.PluginID == 0 {
		return
	}

	s.breakerMu.Lock()
	if entry := s.executionBreakers[permit.PluginID]; entry != nil && permit.HalfOpenProbe {
		entry.ProbeInFlight = false
		entry.ProbeStartedAt = time.Time{}
	}
	s.breakerMu.Unlock()

	if shouldCountPluginExecutionFailure(execErr) {
		s.recordPluginExecutionFailure(plugin.ID, execErr)
		return
	}

	needsRecoveryWrite := permit.HalfOpenProbe ||
		permit.BreakerState.ConsecutiveFailures > 0 ||
		!strings.EqualFold(strings.TrimSpace(plugin.Status), "healthy") ||
		strings.EqualFold(strings.TrimSpace(plugin.LifecycleStatus), models.PluginLifecycleDegraded)
	if needsRecoveryWrite {
		s.updatePluginStatus(plugin.ID, "healthy")
	}
}

func shouldCountPluginExecutionFailure(err error) bool {
	if err == nil {
		return false
	}
	return !isPluginExecutionCanceledError(err)
}

func (s *PluginManagerService) recordPluginExecutionFailure(pluginID uint, execErr error) {
	s.updatePluginStatus(pluginID, "unhealthy")
	if execErr == nil {
		return
	}
	lastError := truncateExecutionLogText(strings.TrimSpace(execErr.Error()))
	if lastError == "" {
		return
	}
	s.updatePluginLifecycle(pluginID, models.PluginLifecycleDegraded, map[string]interface{}{
		"last_error": lastError,
	})
}

func formatPluginExecutionBreakerError(plugin *models.Plugin, state pluginExecutionBreakerState) error {
	name := "unknown"
	if plugin != nil {
		if strings.TrimSpace(plugin.DisplayName) != "" {
			name = strings.TrimSpace(plugin.DisplayName)
		} else if strings.TrimSpace(plugin.Name) != "" {
			name = strings.TrimSpace(plugin.Name)
		}
	}

	if state.State == pluginBreakerStateHalfOpen && state.ProbeInFlight {
		return fmt.Errorf("plugin %s is probing for recovery in half-open state; try again after the probe finishes", name)
	}
	if state.CooldownUntil != nil {
		return fmt.Errorf("plugin %s circuit breaker is open until %s", name, state.CooldownUntil.Format(time.RFC3339))
	}
	return fmt.Errorf("plugin %s circuit breaker is open after repeated failures", name)
}

func (s *PluginManagerService) resolveHookTimeoutForPhase(phase hookPhase, policy config.PluginExecutionPolicyConfig) time.Duration {
	timeoutMs := policy.HookAfterTimeoutMs
	if phase == hookPhaseBefore {
		timeoutMs = policy.HookBeforeTimeoutMs
	}
	if timeoutMs <= 0 {
		timeoutMs = s.getPluginPlatformConfig().Sandbox.ExecTimeoutMs
	}
	if timeoutMs <= 0 {
		timeoutMs = 30000
	}
	return time.Duration(timeoutMs) * time.Millisecond
}

func waitForExecutionRetryBackoff(ctx context.Context, retryBackoff time.Duration) error {
	if retryBackoff <= 0 {
		return nil
	}

	timer := time.NewTimer(retryBackoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *PluginManagerService) acquireHookExecutionSlot(ctx context.Context, waitTimeout time.Duration) (func(), error) {
	policy := s.getHookExecutionPolicy()
	maxInFlight := policy.HookMaxInFlight
	if maxInFlight <= 0 {
		return func() {}, nil
	}

	limiter := s.ensureHookLimiter(maxInFlight)
	if limiter == nil {
		return func() {}, nil
	}

	if waitTimeout <= 0 {
		waitTimeout = 3 * time.Second
	}

	timer := time.NewTimer(waitTimeout)
	defer timer.Stop()

	select {
	case limiter <- struct{}{}:
		released := false
		return func() {
			if released {
				return
			}
			released = true
			<-limiter
		}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, fmt.Errorf("hook inflight limit reached (max=%d)", maxInFlight)
	}
}

func (s *PluginManagerService) ensureHookLimiter(capacity int) chan struct{} {
	if capacity <= 0 {
		return nil
	}

	s.hookLimiterMu.Lock()
	defer s.hookLimiterMu.Unlock()
	if s.hookLimiter != nil && s.hookLimiterCap == capacity {
		return s.hookLimiter
	}

	s.hookLimiter = make(chan struct{}, capacity)
	s.hookLimiterCap = capacity
	return s.hookLimiter
}

func (s *PluginManagerService) getPluginPlatformConfig() config.PluginPlatformConfig {
	slotAnimationsEnabled := true
	pluginCfg := config.PluginPlatformConfig{
		Enabled:           true,
		AllowedRuntimes:   []string{PluginRuntimeGRPC},
		DefaultRuntime:    PluginRuntimeGRPC,
		ArtifactDir:       filepath.Join("data", "plugins"),
		JSFSMaxFiles:      2048,
		JSFSMaxTotalBytes: 128 * 1024 * 1024,
		JSFSMaxReadBytes:  4 * 1024 * 1024,
		Frontend: config.PluginFrontendPolicyConfig{
			ForceSanitizeHTML:     false,
			SlotAnimationsEnabled: &slotAnimationsEnabled,
		},
		GRPC: config.PluginGRPCTransportConfig{
			Mode: "insecure_local",
		},
		Sandbox: config.PluginSandboxConfig{
			Level:              "balanced",
			ExecTimeoutMs:      30000,
			MaxMemoryMB:        128,
			MaxConcurrency:     4,
			JSWorkerSocketPath: defaultJSWorkerSocketPath,
		},
		Execution: config.PluginExecutionPolicyConfig{
			HookMaxInFlight:           64,
			HookMaxRetries:            0,
			HookRetryBackoffMs:        100,
			HookBeforeTimeoutMs:       30000,
			HookAfterTimeoutMs:        30000,
			FailureThreshold:          3,
			FailureCooldownMs:         30000,
			ExecutionLogRetentionDays: 90,
		},
	}
	if s.cfg == nil {
		return pluginCfg
	}

	pluginCfg = s.cfg.Plugin
	if len(pluginCfg.AllowedRuntimes) == 0 {
		pluginCfg.AllowedRuntimes = []string{PluginRuntimeGRPC}
	}
	pluginCfg.AllowedRuntimes = normalizeLowerStringList(pluginCfg.AllowedRuntimes)
	if len(pluginCfg.AllowedRuntimes) == 0 {
		pluginCfg.AllowedRuntimes = []string{PluginRuntimeGRPC}
	}
	pluginCfg.AllowedTypes = normalizeLowerStringList(pluginCfg.AllowedTypes)

	pluginCfg.DefaultRuntime = strings.ToLower(strings.TrimSpace(pluginCfg.DefaultRuntime))
	if pluginCfg.DefaultRuntime == "" {
		pluginCfg.DefaultRuntime = pluginCfg.AllowedRuntimes[0]
	}
	pluginCfg.ArtifactDir = filepath.Clean(filepath.FromSlash(strings.TrimSpace(pluginCfg.ArtifactDir)))
	if pluginCfg.ArtifactDir == "" || pluginCfg.ArtifactDir == "." {
		pluginCfg.ArtifactDir = filepath.Join("data", "plugins")
	}
	if pluginCfg.JSFSMaxFiles <= 0 {
		pluginCfg.JSFSMaxFiles = 2048
	}
	if pluginCfg.JSFSMaxTotalBytes <= 0 {
		pluginCfg.JSFSMaxTotalBytes = 128 * 1024 * 1024
	}
	if pluginCfg.JSFSMaxReadBytes <= 0 {
		pluginCfg.JSFSMaxReadBytes = 4 * 1024 * 1024
	}
	if pluginCfg.JSFSMaxReadBytes > pluginCfg.JSFSMaxTotalBytes {
		pluginCfg.JSFSMaxReadBytes = pluginCfg.JSFSMaxTotalBytes
	}
	pluginCfg.GRPC.Mode = strings.ToLower(strings.TrimSpace(pluginCfg.GRPC.Mode))
	if pluginCfg.GRPC.Mode == "" {
		pluginCfg.GRPC.Mode = "insecure_local"
	}
	switch pluginCfg.GRPC.Mode {
	case "insecure", "insecure_local", "tls":
	default:
		pluginCfg.GRPC.Mode = "insecure_local"
	}
	pluginCfg.GRPC.CAFile = strings.TrimSpace(pluginCfg.GRPC.CAFile)
	pluginCfg.GRPC.CertFile = strings.TrimSpace(pluginCfg.GRPC.CertFile)
	pluginCfg.GRPC.KeyFile = strings.TrimSpace(pluginCfg.GRPC.KeyFile)
	pluginCfg.GRPC.ServerName = strings.TrimSpace(pluginCfg.GRPC.ServerName)
	if (pluginCfg.GRPC.CertFile == "") != (pluginCfg.GRPC.KeyFile == "") {
		pluginCfg.GRPC.CertFile = ""
		pluginCfg.GRPC.KeyFile = ""
	}

	if pluginCfg.Sandbox.ExecTimeoutMs <= 0 {
		pluginCfg.Sandbox.ExecTimeoutMs = 30000
	}
	if pluginCfg.Sandbox.MaxMemoryMB <= 0 {
		pluginCfg.Sandbox.MaxMemoryMB = 128
	}
	if pluginCfg.Sandbox.MaxConcurrency <= 0 {
		pluginCfg.Sandbox.MaxConcurrency = 4
	}
	if strings.TrimSpace(pluginCfg.Sandbox.JSWorkerSocketPath) == "" {
		pluginCfg.Sandbox.JSWorkerSocketPath = defaultJSWorkerSocketPath
	}
	pluginCfg.Sandbox.Level = strings.ToLower(strings.TrimSpace(pluginCfg.Sandbox.Level))
	if pluginCfg.Sandbox.Level == "" {
		pluginCfg.Sandbox.Level = "balanced"
	}
	if pluginCfg.Execution.HookMaxInFlight <= 0 {
		pluginCfg.Execution.HookMaxInFlight = 64
	}
	if pluginCfg.Execution.HookMaxRetries < 0 {
		pluginCfg.Execution.HookMaxRetries = 0
	}
	if pluginCfg.Execution.HookRetryBackoffMs <= 0 {
		pluginCfg.Execution.HookRetryBackoffMs = 100
	}
	if pluginCfg.Execution.HookBeforeTimeoutMs <= 0 {
		pluginCfg.Execution.HookBeforeTimeoutMs = pluginCfg.Sandbox.ExecTimeoutMs
	}
	if pluginCfg.Execution.HookAfterTimeoutMs <= 0 {
		pluginCfg.Execution.HookAfterTimeoutMs = pluginCfg.Sandbox.ExecTimeoutMs
	}
	if pluginCfg.Execution.FailureThreshold <= 0 {
		pluginCfg.Execution.FailureThreshold = 3
	}
	if pluginCfg.Execution.FailureCooldownMs < 0 {
		pluginCfg.Execution.FailureCooldownMs = 0
	}
	if pluginCfg.Execution.FailureCooldownMs == 0 {
		pluginCfg.Execution.FailureCooldownMs = 30000
	}
	if pluginCfg.Execution.ExecutionLogRetentionDays == 0 {
		pluginCfg.Execution.ExecutionLogRetentionDays = 90
	}
	return pluginCfg
}

type legacyPluginHookConfig struct {
	Hooks         []string `json:"hooks"`
	DisabledHooks []string `json:"disabled_hooks"`
}

type pluginCapabilityConfig struct {
	Hooks                   []string          `json:"hooks"`
	DisabledHooks           []string          `json:"disabled_hooks"`
	FrontendHTMLMode        string            `json:"frontend_html_mode"`
	HTMLMode                string            `json:"html_mode"`
	RequestedPermissions    []string          `json:"requested_permissions"`
	GrantedPermissions      []string          `json:"granted_permissions"`
	ExecuteActionStorage    map[string]string `json:"execute_action_storage"`
	FrontendMinScope        string            `json:"frontend_min_scope"`
	FrontendRequiredPerms   []string          `json:"frontend_required_permissions"`
	FrontendAllowedAreas    []string          `json:"frontend_allowed_areas"`
	AllowBlock              *bool             `json:"allow_block"`
	AllowPayloadPatch       *bool             `json:"allow_payload_patch"`
	AllowFrontendExtensions *bool             `json:"allow_frontend_extensions"`
	AllowExecuteAPI         *bool             `json:"allow_execute_api"`
	AllowNetwork            *bool             `json:"allow_network"`
	AllowFileSystem         *bool             `json:"allow_file_system"`
	AllowedFrontendSlots    []string          `json:"allowed_frontend_slots"`
}

type EffectivePluginCapabilityPolicy struct {
	Hooks                   []string          `json:"hooks"`
	DisabledHooks           []string          `json:"disabled_hooks"`
	FrontendHTMLMode        string            `json:"frontend_html_mode"`
	RequestedPermissions    []string          `json:"requested_permissions"`
	GrantedPermissions      []string          `json:"granted_permissions"`
	ExecuteActionStorage    map[string]string `json:"execute_action_storage,omitempty"`
	FrontendMinScope        string            `json:"frontend_min_scope"`
	FrontendRequiredPerms   []string          `json:"frontend_required_permissions"`
	FrontendAllowedAreas    []string          `json:"frontend_allowed_areas"`
	AllowedFrontendSlots    []string          `json:"allowed_frontend_slots"`
	AllowHookExecute        bool              `json:"allow_hook_execute"`
	AllowBlock              bool              `json:"allow_block"`
	AllowPayloadPatch       bool              `json:"allow_payload_patch"`
	AllowFrontendExtensions bool              `json:"allow_frontend_extensions"`
	AllowExecuteAPI         bool              `json:"allow_execute_api"`
	AllowNetwork            bool              `json:"allow_network"`
	AllowFileSystem         bool              `json:"allow_file_system"`
	Valid                   bool              `json:"valid"`
}

type pluginCapabilityPolicy struct {
	Hooks                   []string
	DisabledHooks           []string
	FrontendHTMLMode        string
	RequestedPermissions    []string
	GrantedPermissions      []string
	ExecuteActionStorage    map[string]string
	FrontendMinScope        string
	FrontendRequiredPerms   []string
	FrontendAllowedAreas    []string
	AllowedFrontendSlots    []string
	AllowHookExecute        bool
	AllowBlock              bool
	AllowPayloadPatch       bool
	AllowFrontendExtensions bool
	AllowExecuteAPI         bool
	AllowNetwork            bool
	AllowFileSystem         bool
	Valid                   bool
}

type parsedHookResponse struct {
	Blocked            bool                   `json:"blocked"`
	BlockReason        string                 `json:"block_reason"`
	Payload            map[string]interface{} `json:"payload"`
	FrontendExtensions []FrontendExtension    `json:"frontend_extensions"`
}

func defaultPluginCapabilityPolicy() pluginCapabilityPolicy {
	return pluginCapabilityPolicy{
		AllowHookExecute:        true,
		AllowBlock:              true,
		AllowPayloadPatch:       true,
		AllowFrontendExtensions: true,
		AllowExecuteAPI:         true,
		AllowNetwork:            true,
		AllowFileSystem:         true,
		Valid:                   true,
	}
}

func denyPluginCapabilityPolicy() pluginCapabilityPolicy {
	return pluginCapabilityPolicy{
		AllowHookExecute:        false,
		AllowBlock:              false,
		AllowPayloadPatch:       false,
		AllowFrontendExtensions: false,
		AllowExecuteAPI:         false,
		AllowNetwork:            false,
		AllowFileSystem:         false,
		Valid:                   false,
	}
}

func resolvePluginCapabilityPolicy(plugin *models.Plugin) pluginCapabilityPolicy {
	if plugin == nil {
		return denyPluginCapabilityPolicy()
	}

	policy := defaultPluginCapabilityPolicy()
	capabilityStr := strings.TrimSpace(plugin.Capabilities)
	if capabilityStr == "" {
		capabilityStr = "{}"
	}

	var cfg pluginCapabilityConfig
	if err := json.Unmarshal([]byte(capabilityStr), &cfg); err != nil {
		log.Printf("Invalid capabilities on plugin %s: %v", plugin.Name, err)
		return denyPluginCapabilityPolicy()
	}
	policy.Hooks = normalizeLowerStringList(cfg.Hooks)
	policy.DisabledHooks = normalizeLowerStringList(cfg.DisabledHooks)
	policy.ExecuteActionStorage = normalizePluginExecuteActionStorageProfiles(cfg.ExecuteActionStorage)
	policy.AllowedFrontendSlots = normalizeLowerStringList(cfg.AllowedFrontendSlots)
	policy.FrontendMinScope = normalizePluginFrontendMinScope(cfg.FrontendMinScope)
	policy.FrontendRequiredPerms = NormalizePluginPermissionList(cfg.FrontendRequiredPerms)
	policy.FrontendAllowedAreas = normalizePluginFrontendAreas(cfg.FrontendAllowedAreas)
	policy.RequestedPermissions = NormalizePluginPermissionList(cfg.RequestedPermissions)
	policy.GrantedPermissions = NormalizePluginPermissionList(cfg.GrantedPermissions)
	policy.FrontendHTMLMode = resolvePluginFrontendHTMLModeCapability(
		cfg.FrontendHTMLMode,
		cfg.HTMLMode,
		policy.RequestedPermissions,
		policy.GrantedPermissions,
	)
	policy.AllowBlock = boolPointerOrDefault(cfg.AllowBlock, true)
	policy.AllowPayloadPatch = boolPointerOrDefault(cfg.AllowPayloadPatch, true)
	policy.AllowFrontendExtensions = boolPointerOrDefault(cfg.AllowFrontendExtensions, true)
	policy.AllowExecuteAPI = boolPointerOrDefault(cfg.AllowExecuteAPI, true)
	policy.AllowNetwork = boolPointerOrDefault(cfg.AllowNetwork, true)
	policy.AllowFileSystem = boolPointerOrDefault(cfg.AllowFileSystem, true)

	policy.AllowHookExecute = IsPluginPermissionGranted(policy.RequestedPermissions, policy.GrantedPermissions, PluginPermissionHookExecute)
	policy.AllowPayloadPatch = policy.AllowPayloadPatch && IsPluginPermissionGranted(policy.RequestedPermissions, policy.GrantedPermissions, PluginPermissionHookPayloadPatch)
	policy.AllowBlock = policy.AllowBlock && IsPluginPermissionGranted(policy.RequestedPermissions, policy.GrantedPermissions, PluginPermissionHookBlock)
	policy.AllowFrontendExtensions = policy.AllowFrontendExtensions && IsPluginPermissionGranted(policy.RequestedPermissions, policy.GrantedPermissions, PluginPermissionFrontendExtension)
	policy.AllowExecuteAPI = policy.AllowExecuteAPI && IsPluginPermissionGranted(policy.RequestedPermissions, policy.GrantedPermissions, PluginPermissionExecuteAPI)

	// 兼容旧版：hooks/disabled_hooks 可能放在 plugin.config 内。
	if len(policy.Hooks) == 0 && len(policy.DisabledHooks) == 0 {
		configStr := strings.TrimSpace(plugin.Config)
		if configStr != "" {
			var legacy legacyPluginHookConfig
			if err := json.Unmarshal([]byte(configStr), &legacy); err == nil {
				policy.Hooks = normalizeLowerStringList(legacy.Hooks)
				policy.DisabledHooks = normalizeLowerStringList(legacy.DisabledHooks)
			}
		}
	}
	return policy
}

func ResolveEffectivePluginCapabilityPolicy(plugin *models.Plugin) EffectivePluginCapabilityPolicy {
	policy := resolvePluginCapabilityPolicy(plugin)
	return EffectivePluginCapabilityPolicy{
		Hooks:                   append([]string(nil), policy.Hooks...),
		DisabledHooks:           append([]string(nil), policy.DisabledHooks...),
		FrontendHTMLMode:        policy.FrontendHTMLMode,
		RequestedPermissions:    append([]string(nil), policy.RequestedPermissions...),
		GrantedPermissions:      append([]string(nil), policy.GrantedPermissions...),
		ExecuteActionStorage:    cloneStringMap(policy.ExecuteActionStorage),
		FrontendMinScope:        policy.FrontendMinScope,
		FrontendRequiredPerms:   append([]string(nil), policy.FrontendRequiredPerms...),
		FrontendAllowedAreas:    append([]string(nil), policy.FrontendAllowedAreas...),
		AllowedFrontendSlots:    append([]string(nil), policy.AllowedFrontendSlots...),
		AllowHookExecute:        policy.AllowHookExecute,
		AllowBlock:              policy.AllowBlock,
		AllowPayloadPatch:       policy.AllowPayloadPatch,
		AllowFrontendExtensions: policy.AllowFrontendExtensions,
		AllowExecuteAPI:         policy.AllowExecuteAPI,
		AllowNetwork:            policy.AllowsRuntimeNetwork(),
		AllowFileSystem:         policy.AllowsRuntimeFileSystem(),
		Valid:                   policy.Valid,
	}
}

func normalizePluginFrontendHTMLModeCapability(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "trusted":
		return "trusted"
	default:
		return "sanitize"
	}
}

func resolvePluginFrontendHTMLModeCapability(
	frontendHTMLMode string,
	htmlMode string,
	requestedPermissions []string,
	grantedPermissions []string,
) string {
	modeRaw := strings.TrimSpace(frontendHTMLMode)
	if modeRaw == "" {
		modeRaw = strings.TrimSpace(htmlMode)
	}
	mode := normalizePluginFrontendHTMLModeCapability(modeRaw)
	if mode != "trusted" {
		return "sanitize"
	}
	if !IsPluginPermissionGranted(requestedPermissions, grantedPermissions, PluginPermissionFrontendHTMLTrust) {
		return "sanitize"
	}
	return "trusted"
}

func (s *PluginManagerService) InspectPluginRuntime(plugin *models.Plugin) PluginRuntimeInspection {
	inspection := PluginRuntimeInspection{
		Valid:              false,
		ConnectionState:    "unknown",
		ConfiguredRuntime:  "",
		ResolvedRuntime:    "",
		Enabled:            false,
		LifecycleStatus:    "",
		HealthStatus:       "",
		AddressPresent:     false,
		PackagePathPresent: false,
		Ready:              false,
		LastError:          "",
	}
	if plugin == nil {
		inspection.ConnectionState = "unavailable"
		inspection.LastError = "plugin record is unavailable"
		return inspection
	}

	inspection.ConfiguredRuntime = strings.TrimSpace(plugin.Runtime)
	inspection.Enabled = plugin.Enabled
	inspection.LifecycleStatus = strings.TrimSpace(plugin.LifecycleStatus)
	inspection.HealthStatus = strings.TrimSpace(plugin.Status)
	inspection.AddressPresent = strings.TrimSpace(plugin.Address) != ""
	inspection.PackagePathPresent = strings.TrimSpace(plugin.PackagePath) != ""
	inspection.LastError = strings.TrimSpace(plugin.LastError)
	slotInspection := s.inspectPluginRuntimeSlots(plugin.ID)
	inspection.ActiveGeneration = slotInspection.ActiveGeneration
	inspection.ActiveInFlight = slotInspection.ActiveInFlight
	inspection.DrainingSlotCount = slotInspection.DrainingSlotCount
	inspection.DrainingInFlight = slotInspection.DrainingInFlight
	inspection.DrainingGenerations = append([]uint{}, slotInspection.DrainingGenerations...)
	breaker := s.inspectPluginExecutionBreaker(plugin)
	inspection.BreakerState = breaker.State
	inspection.FailureCount = breaker.ConsecutiveFailures
	inspection.FailureThreshold = breaker.FailureThreshold
	inspection.CooldownActive = breaker.CooldownActive
	inspection.CooldownReason = strings.TrimSpace(breaker.Reason)
	inspection.ProbeInFlight = breaker.ProbeInFlight
	if breaker.CooldownUntil != nil {
		cooldownUntil := breaker.CooldownUntil.UTC()
		inspection.CooldownUntil = &cooldownUntil
	}
	if breaker.ProbeStartedAt != nil {
		probeStartedAt := breaker.ProbeStartedAt.UTC()
		inspection.ProbeStartedAt = &probeStartedAt
	}

	runtime, err := s.ResolveRuntime(plugin.Runtime)
	if err != nil {
		inspection.ConnectionState = "unsupported"
		return inspection
	}

	inspection.Valid = true
	inspection.ResolvedRuntime = runtime
	switch runtime {
	case PluginRuntimeGRPC:
		_, connected := s.getGRPCClient(plugin.ID)
		if connected {
			inspection.ConnectionState = "connected"
			inspection.Ready = true
		} else {
			inspection.ConnectionState = "disconnected"
		}
	case PluginRuntimeJSWorker:
		inspection.ConnectionState = "stateless"
		inspection.Ready = s != nil && s.jsWorker != nil
	default:
		inspection.ConnectionState = "unsupported"
		inspection.Valid = false
	}

	return inspection
}

func (s *PluginManagerService) InspectPluginRegistration(plugin *models.Plugin) PluginRegistrationInspection {
	inspection := PluginRegistrationInspection{
		State: "never_attempted",
	}
	if plugin == nil {
		inspection.State = "unavailable"
		inspection.Detail = "plugin record is unavailable"
		return inspection
	}
	if s == nil {
		inspection.State = "unavailable"
		inspection.Detail = "plugin manager is unavailable"
		inspection.Runtime = strings.TrimSpace(plugin.Runtime)
		return inspection
	}

	s.registrationMu.RLock()
	record, exists := s.registration[plugin.ID]
	s.registrationMu.RUnlock()
	if !exists {
		inspection.Runtime = strings.TrimSpace(plugin.Runtime)
		return inspection
	}
	return record
}

func (s *PluginManagerService) recordPluginRegistrationOutcome(
	plugin *models.Plugin,
	trigger string,
	runtime string,
	startedAt time.Time,
	err error,
) {
	if s == nil || plugin == nil || plugin.ID == 0 {
		return
	}

	completedAt := time.Now().UTC()
	inspection := PluginRegistrationInspection{
		State:       "success",
		Trigger:     strings.ToLower(strings.TrimSpace(trigger)),
		Runtime:     strings.TrimSpace(runtime),
		AttemptedAt: startedAt,
		CompletedAt: completedAt,
		DurationMs:  completedAt.Sub(startedAt).Milliseconds(),
		Detail:      "plugin registered successfully",
	}
	if inspection.AttemptedAt.IsZero() {
		inspection.AttemptedAt = completedAt
	}
	if inspection.DurationMs < 0 {
		inspection.DurationMs = 0
	}
	if err != nil {
		inspection.State = "error"
		inspection.Detail = err.Error()
	}
	if inspection.Runtime == "" {
		inspection.Runtime = strings.TrimSpace(plugin.Runtime)
	}

	s.registrationMu.Lock()
	if s.registration == nil {
		s.registration = make(map[uint]PluginRegistrationInspection)
	}
	s.registration[plugin.ID] = inspection
	s.registrationMu.Unlock()
}

func DiagnosePluginHookParticipation(
	plugin *models.Plugin,
	hook string,
	payload map[string]interface{},
	execCtx *ExecutionContext,
) PluginHookParticipationDiagnosis {
	diagnosis := PluginHookParticipationDiagnosis{
		Hook:                 normalizeHookName(hook),
		SupportsFrontendArea: true,
		SupportsFrontendSlot: true,
	}
	if payload != nil {
		if path, ok := payload["path"].(string); ok {
			diagnosis.Path = strings.TrimSpace(path)
		}
	}
	if plugin == nil {
		diagnosis.ReasonCode = "invalid_plugin"
		diagnosis.Reason = "plugin record is unavailable"
		return diagnosis
	}

	policy := resolvePluginCapabilityPolicy(plugin)
	diagnosis.AllowBlock = policy.AllowBlock
	diagnosis.AllowPayloadPatch = policy.AllowPayloadPatch
	diagnosis.AllowFrontendExtensions = policy.AllowFrontendExtensions
	diagnosis.ValidCapabilityPolicy = policy.Valid
	diagnosis.Area = resolveFrontendRequestArea(hook, payload)
	diagnosis.Slot = resolveFrontendRequestSlot(hook, payload)

	if !policy.Valid {
		diagnosis.ReasonCode = "invalid_capabilities"
		diagnosis.Reason = "capabilities JSON is invalid"
		return diagnosis
	}

	if !policy.AllowHookExecute {
		diagnosis.ReasonCode = "hook_execute_permission_denied"
		diagnosis.Reason = "hook execution is disabled by effective permissions"
		return diagnosis
	}

	normalizedHook := normalizeHookName(hook)
	if normalizedHook == "" {
		diagnosis.ReasonCode = "invalid_hook"
		diagnosis.Reason = "hook name is required"
		return diagnosis
	}

	if hookInList(policy.DisabledHooks, normalizedHook) {
		diagnosis.ReasonCode = "hook_disabled"
		diagnosis.Reason = "hook is explicitly disabled by plugin capabilities"
		return diagnosis
	}
	if len(policy.Hooks) > 0 && !hookInList(policy.Hooks, normalizedHook) {
		diagnosis.ReasonCode = "hook_not_subscribed"
		diagnosis.Reason = "hook is not listed in plugin capabilities"
		return diagnosis
	}
	diagnosis.SupportsHook = true

	if diagnosis.Area != "" {
		diagnosis.SupportsFrontendArea = policy.SupportsFrontendArea(diagnosis.Area)
		if !diagnosis.SupportsFrontendArea {
			diagnosis.ReasonCode = "frontend_area_denied"
			diagnosis.Reason = "frontend area is not allowed by plugin capabilities"
			return diagnosis
		}
	}

	if diagnosis.Slot != "" {
		diagnosis.SupportsFrontendSlot = policy.SupportsFrontendSlot(diagnosis.Slot)
		if !diagnosis.SupportsFrontendSlot {
			diagnosis.ReasonCode = "frontend_slot_denied"
			diagnosis.Reason = "frontend slot is not allowed by plugin capabilities"
			return diagnosis
		}
	}

	scope := resolveHookRequestAccessScope(execCtx)
	diagnosis.AccessAllowed = policy.AllowsHookRequestAccess(normalizedHook, payload, scope)
	if !diagnosis.AccessAllowed {
		switch {
		case policy.FrontendMinScope == "authenticated" && !scope.Authenticated:
			diagnosis.ReasonCode = "frontend_scope_requires_authenticated"
			diagnosis.Reason = "frontend scope requires an authenticated user"
		case policy.FrontendMinScope == "super_admin" && !scope.SuperAdmin:
			diagnosis.ReasonCode = "frontend_scope_requires_super_admin"
			diagnosis.Reason = "frontend scope requires a super admin"
		case !scope.SuperAdmin && !scope.hasAllPermissions(policy.FrontendRequiredPerms):
			diagnosis.ReasonCode = "frontend_required_permissions_missing"
			diagnosis.Reason = "frontend required permissions are missing from current scope"
		default:
			diagnosis.ReasonCode = "hook_access_denied"
			diagnosis.Reason = "hook access is denied for the current request scope"
		}
		return diagnosis
	}

	diagnosis.Participates = true
	diagnosis.ReasonCode = "allowed"
	diagnosis.Reason = "plugin is eligible to participate in this hook"
	return diagnosis
}

func pluginSupportsHook(plugin *models.Plugin, hook string) bool {
	return resolvePluginCapabilityPolicy(plugin).SupportsHook(hook)
}

func (p pluginCapabilityPolicy) SupportsHook(hook string) bool {
	if !p.Valid {
		return false
	}
	if !p.AllowHookExecute {
		return false
	}
	normalizedHook := normalizeHookName(hook)
	if normalizedHook == "" {
		return false
	}
	if hookInList(p.DisabledHooks, normalizedHook) {
		return false
	}
	if len(p.Hooks) == 0 {
		return true
	}
	return hookInList(p.Hooks, normalizedHook)
}

func (p pluginCapabilityPolicy) SupportsFrontendSlot(slot string) bool {
	if !p.Valid {
		return false
	}
	if len(p.AllowedFrontendSlots) == 0 {
		return true
	}
	normalizedSlot := normalizedSlotValue(slot)
	return hookInList(p.AllowedFrontendSlots, normalizedSlot)
}

type hookRequestAccessScope struct {
	Known         bool
	Authenticated bool
	SuperAdmin    bool
	Permissions   map[string]struct{}
}

func resolveHookRequestAccessScope(execCtx *ExecutionContext) hookRequestAccessScope {
	scope := hookRequestAccessScope{
		Known:         false,
		Authenticated: false,
		SuperAdmin:    false,
		Permissions:   map[string]struct{}{},
	}
	if execCtx == nil || len(execCtx.Metadata) == 0 {
		return scope
	}

	metadata := execCtx.Metadata
	_, hasAuth := metadata[PluginScopeMetadataAuthenticated]
	_, hasSuperAdmin := metadata[PluginScopeMetadataSuperAdmin]
	_, hasPermissions := metadata[PluginScopeMetadataPermissions]
	if !hasAuth && !hasSuperAdmin && !hasPermissions {
		return scope
	}

	scope.Known = true
	scope.Authenticated = parseExecutionMetadataBool(metadata[PluginScopeMetadataAuthenticated], false)
	scope.SuperAdmin = parseExecutionMetadataBool(metadata[PluginScopeMetadataSuperAdmin], false)
	if scope.SuperAdmin {
		scope.Authenticated = true
	}
	for _, item := range parseExecutionMetadataList(metadata[PluginScopeMetadataPermissions]) {
		scope.Permissions[item] = struct{}{}
	}
	return scope
}

func parseExecutionMetadataBool(raw string, defaultValue bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		return defaultValue
	}
}

func parseExecutionMetadataList(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	return NormalizePluginPermissionList(parts)
}

func buildHookRequestAccessScopeCacheKey(scope hookRequestAccessScope) string {
	permissions := make([]string, 0, len(scope.Permissions))
	for permission := range scope.Permissions {
		permissions = append(permissions, permission)
	}
	sort.Strings(permissions)
	return fmt.Sprintf(
		"known=%t|auth=%t|super=%t|permissions=%s",
		scope.Known,
		scope.Authenticated,
		scope.SuperAdmin,
		strings.Join(permissions, ","),
	)
}

func buildPreparedHookPluginsCacheKey(
	hook string,
	payload map[string]interface{},
	scope hookRequestAccessScope,
) string {
	normalizedHook := normalizeHookName(hook)
	switch normalizedHook {
	case "frontend.bootstrap", "frontend.slot.render":
	default:
		return ""
	}
	return fmt.Sprintf(
		"hook=%s|slot=%s|area=%s|scope=%s",
		normalizedHook,
		resolveFrontendRequestSlot(normalizedHook, payload),
		resolveFrontendRequestArea(normalizedHook, payload),
		buildHookRequestAccessScopeCacheKey(scope),
	)
}

func normalizePluginFrontendMinScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "authenticated", "auth", "user", "member":
		return "authenticated"
	case "super_admin", "superadmin", "root":
		return "super_admin"
	default:
		return "guest"
	}
}

func normalizePluginFrontendAreas(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := normalizePluginFrontendArea(value)
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

func normalizePluginFrontendArea(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "admin":
		return "admin"
	case "user":
		return "user"
	case "*":
		return "*"
	default:
		return ""
	}
}

func resolveFrontendRequestSlot(hook string, payload map[string]interface{}) string {
	normalizedHook := normalizeHookName(hook)
	switch normalizedHook {
	case "frontend.slot.render":
		return normalizedSlotValue(payload["slot"])
	default:
		return ""
	}
}

func resolveFrontendRequestArea(hook string, payload map[string]interface{}) string {
	normalizedHook := normalizeHookName(hook)
	switch normalizedHook {
	case "frontend.bootstrap":
		if areaRaw, ok := payload["area"].(string); ok {
			normalized := normalizePluginFrontendArea(areaRaw)
			if normalized != "" && normalized != "*" {
				return normalized
			}
		}
		return "user"
	case "frontend.slot.render":
		slot := normalizedSlotValue(payload["slot"])
		if strings.HasPrefix(slot, "admin.") {
			return "admin"
		}
		return "user"
	default:
		return ""
	}
}

func (s hookRequestAccessScope) hasAllPermissions(required []string) bool {
	normalizedRequired := NormalizePluginPermissionList(required)
	if len(normalizedRequired) == 0 {
		return true
	}
	if len(s.Permissions) == 0 {
		return false
	}
	for _, key := range normalizedRequired {
		if _, exists := s.Permissions[key]; !exists {
			return false
		}
	}
	return true
}

func (p pluginCapabilityPolicy) SupportsFrontendArea(area string) bool {
	if !p.Valid {
		return false
	}
	if len(p.FrontendAllowedAreas) == 0 {
		return true
	}
	normalizedArea := normalizePluginFrontendArea(area)
	if normalizedArea == "" {
		return false
	}
	return hookInList(p.FrontendAllowedAreas, normalizedArea)
}

func (p pluginCapabilityPolicy) AllowsHookRequestAccess(
	hook string,
	payload map[string]interface{},
	scope hookRequestAccessScope,
) bool {
	normalizedHook := normalizeHookName(hook)
	if !strings.HasPrefix(normalizedHook, "frontend.") {
		return true
	}

	if slot := resolveFrontendRequestSlot(normalizedHook, payload); slot != "" && !p.SupportsFrontendSlot(slot) {
		return false
	}
	if area := resolveFrontendRequestArea(normalizedHook, payload); area != "" && !p.SupportsFrontendArea(area) {
		return false
	}

	if !scope.Known {
		return true
	}

	switch p.FrontendMinScope {
	case "authenticated":
		if !scope.Authenticated {
			return false
		}
	case "super_admin":
		if !scope.SuperAdmin {
			return false
		}
	}

	if scope.SuperAdmin {
		return true
	}
	return scope.hasAllPermissions(p.FrontendRequiredPerms)
}

func (p pluginCapabilityPolicy) AllowsRuntimeNetwork() bool {
	if !p.Valid || !p.AllowNetwork {
		return false
	}
	return IsPluginPermissionGranted(p.RequestedPermissions, p.GrantedPermissions, PluginPermissionRuntimeNetwork)
}

func (p pluginCapabilityPolicy) AllowsRuntimeFileSystem() bool {
	if !p.Valid || !p.AllowFileSystem {
		return false
	}
	return IsPluginPermissionGranted(p.RequestedPermissions, p.GrantedPermissions, PluginPermissionRuntimeFileSystem)
}

func normalizePluginExecuteActionStorageProfiles(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for action, mode := range values {
		normalizedAction := strings.ToLower(strings.TrimSpace(action))
		if normalizedAction == "" {
			continue
		}
		normalizedMode := normalizePluginStorageAccessMode(mode)
		if normalizedMode == pluginStorageAccessUnknown {
			continue
		}
		out[normalizedAction] = normalizedMode
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (p pluginCapabilityPolicy) ResolveExecuteActionStorageMode(action string) string {
	if len(p.ExecuteActionStorage) == 0 {
		return pluginStorageAccessUnknown
	}
	normalizedAction := strings.ToLower(strings.TrimSpace(action))
	if normalizedAction == "" {
		return pluginStorageAccessUnknown
	}
	if mode, exists := p.ExecuteActionStorage[normalizedAction]; exists {
		return normalizePluginStorageAccessMode(mode)
	}
	return pluginStorageAccessUnknown
}

func boolPointerOrDefault(value *bool, defaultValue bool) bool {
	if value == nil {
		return defaultValue
	}
	return *value
}

func hookInList(hooks []string, hook string) bool {
	normalizedHook := strings.ToLower(strings.TrimSpace(hook))
	if normalizedHook == "" {
		return false
	}
	for _, item := range hooks {
		candidate := strings.ToLower(strings.TrimSpace(item))
		if candidate == "" {
			continue
		}
		if candidate == "*" || candidate == normalizedHook {
			return true
		}
	}
	return false
}

func normalizedSlotValue(value interface{}) string {
	raw, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(raw))
}

func buildHookExecuteParams(hook string, payload map[string]interface{}) (map[string]string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"hook":    hook,
		"payload": string(body),
	}, nil
}

func parseHookResponse(data map[string]interface{}) *parsedHookResponse {
	if len(data) == 0 {
		return nil
	}

	var parsed parsedHookResponse
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil
	}

	if parsed.Payload == nil {
		if payloadRaw, exists := data["payload"]; exists {
			if payloadMap, ok := payloadRaw.(map[string]interface{}); ok {
				parsed.Payload = payloadMap
			}
		}
	}

	if len(parsed.FrontendExtensions) == 0 {
		// 兼容 extensions 字段名
		if extRaw, exists := data["extensions"]; exists {
			if extList, ok := decodeFrontendExtensions(extRaw); ok {
				parsed.FrontendExtensions = extList
			}
		}
	}

	if blockedRaw, exists := data["blocked"]; exists {
		if blocked, ok := interfaceToBool(blockedRaw); ok {
			parsed.Blocked = blocked
		}
	}
	if reasonRaw, exists := data["block_reason"]; exists {
		if reason, ok := reasonRaw.(string); ok {
			parsed.BlockReason = reason
		}
	}
	if parsed.BlockReason == "" {
		if reasonRaw, exists := data["message"]; exists {
			if reason, ok := reasonRaw.(string); ok && strings.TrimSpace(reason) != "" {
				parsed.BlockReason = reason
			}
		}
	}

	if parsed.Payload == nil && !hasHookControlFields(data) {
		parsed.Payload = data
	}

	return &parsed
}

func decodeFrontendExtensions(value interface{}) ([]FrontendExtension, bool) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var extensions []FrontendExtension
	if err := json.Unmarshal(raw, &extensions); err != nil {
		return nil, false
	}
	return extensions, true
}

func interfaceToBool(value interface{}) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		lowered := strings.ToLower(strings.TrimSpace(typed))
		switch lowered {
		case "true", "1", "yes", "y":
			return true, true
		case "false", "0", "no", "n":
			return false, true
		}
	case float64:
		return typed != 0, true
	case int:
		return typed != 0, true
	}
	return false, false
}

func hasHookControlFields(data map[string]interface{}) bool {
	keys := []string{"blocked", "block_reason", "payload", "frontend_extensions", "extensions"}
	for _, key := range keys {
		if _, exists := data[key]; exists {
			return true
		}
	}
	return false
}

func mergePayloadMap(base map[string]interface{}, patch map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = make(map[string]interface{})
	}
	result := clonePayloadMap(base)
	for key, value := range patch {
		existing, exists := result[key]
		patchMap, patchIsMap := value.(map[string]interface{})
		existingMap, existingIsMap := existing.(map[string]interface{})
		if exists && patchIsMap && existingIsMap {
			result[key] = mergePayloadMap(existingMap, patchMap)
			continue
		}
		result[key] = clonePayloadValue(value)
	}
	return result
}

func clonePayloadMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for key, value := range src {
		dst[key] = clonePayloadValue(value)
	}
	return dst
}

func clonePayloadValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return clonePayloadMap(typed)
	case []interface{}:
		cloned := make([]interface{}, len(typed))
		for i := range typed {
			cloned[i] = clonePayloadValue(typed[i])
		}
		return cloned
	default:
		return typed
	}
}

func mergeRuntimeParams(runtimeParamsJSON string, requestParams map[string]string) (map[string]string, error) {
	merged := make(map[string]string)

	trimmed := strings.TrimSpace(runtimeParamsJSON)
	if trimmed != "" {
		var defaults map[string]interface{}
		if err := json.Unmarshal([]byte(trimmed), &defaults); err != nil {
			return nil, err
		}
		for key, value := range defaults {
			strKey := strings.TrimSpace(key)
			if strKey == "" {
				continue
			}
			merged[strKey] = interfaceToString(value)
		}
	}

	for key, value := range requestParams {
		strKey := strings.TrimSpace(key)
		if strKey == "" {
			continue
		}
		merged[strKey] = value
	}

	return merged, nil
}

func interfaceToString(value interface{}) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.12f", typed), "0"), ".")
	case float32:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", typed), "0"), ".")
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", typed)
	default:
		body, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(body)
	}
}

func normalizeLowerStringList(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
