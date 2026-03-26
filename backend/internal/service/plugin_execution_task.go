package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"auralogic/internal/models"
)

const (
	PluginExecutionMetadataID        = "plugin_execution_id"
	PluginExecutionMetadataStatus    = "plugin_execution_status"
	PluginExecutionMetadataStream    = "plugin_execution_stream"
	PluginExecutionMetadataRuntime   = "plugin_execution_runtime"
	PluginExecutionMetadataStartedAt = "plugin_execution_started_at"
	PluginExecutionMetadataHook      = "plugin_execution_hook"

	PluginExecutionStatusRunning   = "running"
	PluginExecutionStatusCompleted = "completed"
	PluginExecutionStatusFailed    = "failed"
	PluginExecutionStatusCanceled  = "canceled"
	PluginExecutionStatusTimedOut  = "timed_out"

	defaultPluginExecutionHistoryLimit = 128
)

var pluginExecutionHandleSeq atomic.Uint64

type PluginExecutionTaskSnapshot struct {
	ID             string            `json:"id"`
	PluginID       uint              `json:"plugin_id"`
	PluginName     string            `json:"plugin_name,omitempty"`
	Runtime        string            `json:"runtime,omitempty"`
	Action         string            `json:"action,omitempty"`
	Hook           string            `json:"hook,omitempty"`
	Stream         bool              `json:"stream"`
	Status         string            `json:"status"`
	Cancelable     bool              `json:"cancelable"`
	StartedAt      time.Time         `json:"started_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
	DurationMs     int64             `json:"duration_ms,omitempty"`
	ChunkCount     int               `json:"chunk_count,omitempty"`
	UserID         *uint             `json:"user_id,omitempty"`
	OrderID        *uint             `json:"order_id,omitempty"`
	SessionID      string            `json:"session_id,omitempty"`
	RequestPath    string            `json:"request_path,omitempty"`
	PluginPagePath string            `json:"plugin_page_path,omitempty"`
	Error          string            `json:"error,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type PluginExecutionTaskOverview struct {
	ActiveCount int                           `json:"active_count"`
	RecentCount int                           `json:"recent_count"`
	Active      []PluginExecutionTaskSnapshot `json:"active"`
	Recent      []PluginExecutionTaskSnapshot `json:"recent"`
}

type pluginExecutionTask struct {
	mu sync.RWMutex

	id             string
	pluginID       uint
	pluginName     string
	runtime        string
	action         string
	hook           string
	stream         bool
	status         string
	startedAt      time.Time
	updatedAt      time.Time
	completedAt    *time.Time
	durationMs     int64
	chunkCount     int
	userID         *uint
	orderID        *uint
	sessionID      string
	requestPath    string
	pluginPagePath string
	errorText      string
	metadata       map[string]string
	cancel         context.CancelFunc
}

func NewPluginExecutionHandle() string {
	now := time.Now().UTC()
	seq := pluginExecutionHandleSeq.Add(1)
	return fmt.Sprintf("pex_%s_%06d", now.Format("20060102T150405.000000000Z"), seq%1000000)
}

func EnsurePluginExecutionMetadata(execCtx *ExecutionContext, stream bool) string {
	taskID := ""
	if execCtx != nil && execCtx.Metadata != nil {
		taskID = strings.TrimSpace(execCtx.Metadata[PluginExecutionMetadataID])
	}
	if taskID == "" {
		taskID = NewPluginExecutionHandle()
	}
	applyPluginExecutionMetadata(execCtx, taskID, "", "", stream, PluginExecutionStatusRunning, time.Time{})
	return taskID
}

func applyPluginExecutionMetadata(
	execCtx *ExecutionContext,
	taskID string,
	runtime string,
	hook string,
	stream bool,
	status string,
	startedAt time.Time,
) {
	if execCtx == nil {
		return
	}
	if execCtx.Metadata == nil {
		execCtx.Metadata = make(map[string]string)
	}
	metadata := execCtx.Metadata
	if strings.TrimSpace(taskID) == "" {
		taskID = strings.TrimSpace(metadata[PluginExecutionMetadataID])
	}
	if taskID == "" {
		taskID = NewPluginExecutionHandle()
	}
	metadata[PluginExecutionMetadataID] = taskID
	metadata[PluginExecutionMetadataStream] = strconv.FormatBool(stream)
	if strings.TrimSpace(runtime) != "" {
		metadata[PluginExecutionMetadataRuntime] = strings.TrimSpace(runtime)
	}
	if strings.TrimSpace(hook) != "" {
		metadata[PluginExecutionMetadataHook] = strings.TrimSpace(hook)
	}
	if strings.TrimSpace(status) != "" {
		metadata[PluginExecutionMetadataStatus] = strings.TrimSpace(status)
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	metadata[PluginExecutionMetadataStartedAt] = startedAt.UTC().Format(time.RFC3339Nano)
}

func clonePluginExecutionContext(execCtx *ExecutionContext) *ExecutionContext {
	if execCtx == nil {
		return nil
	}

	cloned := &ExecutionContext{
		OperatorUserID: cloneOptionalExecutionUint(execCtx.OperatorUserID),
		UserID:         cloneOptionalExecutionUint(execCtx.UserID),
		OrderID:        cloneOptionalExecutionUint(execCtx.OrderID),
		SessionID:      strings.TrimSpace(execCtx.SessionID),
		Metadata:       cloneStringMap(execCtx.Metadata),
		RequestContext: execCtx.RequestContext,
	}
	return cloned
}

func cloneOptionalExecutionUint(value *uint) *uint {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func extractPluginExecutionHook(action string, params map[string]string) string {
	if !strings.EqualFold(strings.TrimSpace(action), "hook.execute") {
		return ""
	}
	return normalizeHookName(params["hook"])
}

func parsePluginExecutionStartedAt(metadata map[string]string) time.Time {
	if len(metadata) == 0 {
		return time.Time{}
	}
	raw := strings.TrimSpace(metadata[PluginExecutionMetadataStartedAt])
	if raw == "" {
		return time.Time{}
	}
	startedAt, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}
	}
	return startedAt.UTC()
}

func mergePluginExecutionTaskMetadata(base map[string]string, snapshot PluginExecutionTaskSnapshot) map[string]string {
	metadata := cloneStringMap(base)
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata[PluginExecutionMetadataID] = snapshot.ID
	metadata[PluginExecutionMetadataStatus] = snapshot.Status
	metadata[PluginExecutionMetadataStream] = strconv.FormatBool(snapshot.Stream)
	if strings.TrimSpace(snapshot.Runtime) != "" {
		metadata[PluginExecutionMetadataRuntime] = strings.TrimSpace(snapshot.Runtime)
	}
	if strings.TrimSpace(snapshot.Hook) != "" {
		metadata[PluginExecutionMetadataHook] = strings.TrimSpace(snapshot.Hook)
	}
	if !snapshot.StartedAt.IsZero() {
		metadata[PluginExecutionMetadataStartedAt] = snapshot.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	return metadata
}

func resolvePluginExecutionStatus(result *ExecutionResult, err error) string {
	switch {
	case isPluginExecutionCanceledError(err):
		return PluginExecutionStatusCanceled
	case errors.Is(err, context.DeadlineExceeded) || isPluginExecutionTimeoutError(err):
		return PluginExecutionStatusTimedOut
	case err != nil:
		return PluginExecutionStatusFailed
	case result != nil && !result.Success:
		return PluginExecutionStatusFailed
	default:
		return PluginExecutionStatusCompleted
	}
}

func isPluginExecutionCanceledError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return true
	}
	errText := strings.ToLower(strings.TrimSpace(err.Error()))
	if errText == "" {
		return false
	}
	return strings.Contains(errText, "context canceled") || strings.Contains(errText, "cancelled")
}

func newPluginExecutionTask(
	plugin *models.Plugin,
	runtime string,
	action string,
	hook string,
	stream bool,
	execCtx *ExecutionContext,
	cancel context.CancelFunc,
) *pluginExecutionTask {
	if execCtx == nil {
		execCtx = &ExecutionContext{}
	}
	startedAt := parsePluginExecutionStartedAt(execCtx.Metadata)
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	taskID := strings.TrimSpace(execCtx.Metadata[PluginExecutionMetadataID])
	if taskID == "" {
		taskID = NewPluginExecutionHandle()
	}
	task := &pluginExecutionTask{
		id:             taskID,
		runtime:        strings.TrimSpace(runtime),
		action:         strings.TrimSpace(action),
		hook:           strings.TrimSpace(hook),
		stream:         stream,
		status:         PluginExecutionStatusRunning,
		startedAt:      startedAt,
		updatedAt:      startedAt,
		userID:         cloneOptionalExecutionUint(execCtx.UserID),
		orderID:        cloneOptionalExecutionUint(execCtx.OrderID),
		sessionID:      strings.TrimSpace(execCtx.SessionID),
		requestPath:    strings.TrimSpace(execCtx.Metadata["request_path"]),
		pluginPagePath: strings.TrimSpace(execCtx.Metadata["plugin_page_path"]),
		metadata:       cloneStringMap(execCtx.Metadata),
		cancel:         cancel,
	}
	if plugin != nil {
		task.pluginID = plugin.ID
		task.pluginName = strings.TrimSpace(plugin.Name)
	}
	return task
}

func (t *pluginExecutionTask) snapshot() PluginExecutionTaskSnapshot {
	if t == nil {
		return PluginExecutionTaskSnapshot{}
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	return PluginExecutionTaskSnapshot{
		ID:             t.id,
		PluginID:       t.pluginID,
		PluginName:     t.pluginName,
		Runtime:        t.runtime,
		Action:         t.action,
		Hook:           t.hook,
		Stream:         t.stream,
		Status:         t.status,
		Cancelable:     t.cancel != nil && t.completedAt == nil,
		StartedAt:      t.startedAt,
		UpdatedAt:      t.updatedAt,
		CompletedAt:    cloneOptionalTime(t.completedAt),
		DurationMs:     t.durationMs,
		ChunkCount:     t.chunkCount,
		UserID:         cloneOptionalExecutionUint(t.userID),
		OrderID:        cloneOptionalExecutionUint(t.orderID),
		SessionID:      t.sessionID,
		RequestPath:    t.requestPath,
		PluginPagePath: t.pluginPagePath,
		Error:          t.errorText,
		Metadata:       cloneStringMap(t.metadata),
	}
}

func (t *pluginExecutionTask) isCompleted() bool {
	if t == nil {
		return true
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.completedAt != nil
}

func cloneOptionalTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func (t *pluginExecutionTask) recordChunk() {
	if t == nil {
		return
	}

	t.mu.Lock()
	t.chunkCount++
	t.updatedAt = time.Now().UTC()
	t.mu.Unlock()
}

func (t *pluginExecutionTask) complete(result *ExecutionResult, err error) PluginExecutionTaskSnapshot {
	if t == nil {
		return PluginExecutionTaskSnapshot{}
	}

	finishedAt := time.Now().UTC()
	status := resolvePluginExecutionStatus(result, err)
	errorText := ""
	switch {
	case err != nil:
		errorText = strings.TrimSpace(err.Error())
	case result != nil:
		errorText = strings.TrimSpace(result.Error)
	}

	t.mu.Lock()
	t.status = status
	t.updatedAt = finishedAt
	t.completedAt = &finishedAt
	t.durationMs = finishedAt.Sub(t.startedAt).Milliseconds()
	if t.durationMs < 0 {
		t.durationMs = 0
	}
	t.errorText = errorText
	t.cancel = nil
	if result != nil && len(result.Metadata) > 0 {
		mergedMetadata := cloneStringMap(t.metadata)
		if mergedMetadata == nil {
			mergedMetadata = map[string]string{}
		}
		for key, value := range result.Metadata {
			mergedMetadata[key] = value
		}
		t.metadata = mergedMetadata
	}
	t.metadata = mergePluginExecutionTaskMetadata(t.metadata, PluginExecutionTaskSnapshot{
		ID:        t.id,
		Runtime:   t.runtime,
		Hook:      t.hook,
		Stream:    t.stream,
		Status:    status,
		StartedAt: t.startedAt,
	})
	t.mu.Unlock()

	return t.snapshot()
}

func (t *pluginExecutionTask) cancelTask() PluginExecutionTaskSnapshot {
	if t == nil {
		return PluginExecutionTaskSnapshot{}
	}

	t.mu.Lock()
	cancel := t.cancel
	if t.cancel != nil {
		t.cancel = nil
		t.updatedAt = time.Now().UTC()
	}
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return t.snapshot()
}

func (s *PluginManagerService) startPluginExecutionTask(
	plugin *models.Plugin,
	runtime string,
	action string,
	params map[string]string,
	execCtx *ExecutionContext,
	stream bool,
) (*ExecutionContext, *pluginExecutionTask) {
	preparedExecCtx := clonePluginExecutionContext(execCtx)
	if preparedExecCtx == nil {
		preparedExecCtx = &ExecutionContext{}
	}

	taskID := EnsurePluginExecutionMetadata(preparedExecCtx, stream)
	startedAt := parsePluginExecutionStartedAt(preparedExecCtx.Metadata)
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	hook := extractPluginExecutionHook(action, params)
	taskCtx, cancel := context.WithCancel(executionRequestContext(preparedExecCtx))
	preparedExecCtx.RequestContext = taskCtx
	applyPluginExecutionMetadata(preparedExecCtx, taskID, runtime, hook, stream, PluginExecutionStatusRunning, startedAt)

	task := newPluginExecutionTask(plugin, runtime, action, hook, stream, preparedExecCtx, cancel)
	s.storePluginExecutionTask(task)
	return preparedExecCtx, task
}

func (s *PluginManagerService) storePluginExecutionTask(task *pluginExecutionTask) {
	if s == nil || task == nil || strings.TrimSpace(task.id) == "" {
		return
	}

	s.taskMu.Lock()
	defer s.taskMu.Unlock()

	if s.executionTasks == nil {
		s.executionTasks = make(map[string]*pluginExecutionTask)
	}
	s.executionTasks[task.id] = task
	s.executionTaskOrder = append(s.executionTaskOrder, task.id)
	s.prunePluginExecutionTasksLocked()
}

func (s *PluginManagerService) prunePluginExecutionTasksLocked() {
	limit := s.maxExecutionTaskHistory
	if limit <= 0 {
		limit = defaultPluginExecutionHistoryLimit
	}
	if len(s.executionTaskOrder) <= limit {
		return
	}

	finishedCount := 0
	for _, taskID := range s.executionTaskOrder {
		task := s.executionTasks[taskID]
		if task == nil {
			continue
		}
		if task.isCompleted() {
			finishedCount++
		}
	}
	if finishedCount <= limit {
		return
	}

	remainingFinished := finishedCount
	kept := make([]string, 0, len(s.executionTaskOrder))
	for _, taskID := range s.executionTaskOrder {
		task := s.executionTasks[taskID]
		if task == nil {
			continue
		}
		if !task.isCompleted() {
			kept = append(kept, taskID)
			continue
		}
		if remainingFinished > limit {
			delete(s.executionTasks, taskID)
			remainingFinished--
			continue
		}
		kept = append(kept, taskID)
	}
	s.executionTaskOrder = kept
}

func (s *PluginManagerService) completePluginExecutionTask(task *pluginExecutionTask, result *ExecutionResult, err error) PluginExecutionTaskSnapshot {
	if s == nil || task == nil {
		return PluginExecutionTaskSnapshot{}
	}

	snapshot := task.complete(result, err)

	s.taskMu.Lock()
	s.prunePluginExecutionTasksLocked()
	s.taskMu.Unlock()

	return snapshot
}

func applyPluginExecutionTaskToResult(result *ExecutionResult, snapshot PluginExecutionTaskSnapshot) *ExecutionResult {
	if result == nil || strings.TrimSpace(snapshot.ID) == "" {
		return result
	}
	result.TaskID = snapshot.ID
	result.Metadata = mergePluginExecutionTaskMetadata(result.Metadata, snapshot)
	return result
}

func applyPluginExecutionTaskToChunk(chunk *ExecutionStreamChunk, snapshot PluginExecutionTaskSnapshot, status string) *ExecutionStreamChunk {
	if chunk == nil || strings.TrimSpace(snapshot.ID) == "" {
		return chunk
	}
	if strings.TrimSpace(status) != "" {
		snapshot.Status = strings.TrimSpace(status)
	}
	cloned := cloneExecutionStreamChunk(chunk)
	cloned.TaskID = snapshot.ID
	cloned.Metadata = mergePluginExecutionTaskMetadata(cloned.Metadata, snapshot)
	return cloned
}

func (s *PluginManagerService) ListPluginExecutionTasks(pluginID uint, status string, limit int) []PluginExecutionTaskSnapshot {
	if s == nil {
		return []PluginExecutionTaskSnapshot{}
	}

	status = strings.ToLower(strings.TrimSpace(status))
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	s.taskMu.RLock()
	defer s.taskMu.RUnlock()

	out := make([]PluginExecutionTaskSnapshot, 0, limit)
	for i := len(s.executionTaskOrder) - 1; i >= 0; i-- {
		taskID := s.executionTaskOrder[i]
		task := s.executionTasks[taskID]
		if task == nil {
			continue
		}
		snapshot := task.snapshot()
		if pluginID > 0 && snapshot.PluginID != pluginID {
			continue
		}
		if !matchesPluginExecutionTaskStatus(snapshot, status) {
			continue
		}
		out = append(out, snapshot)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func matchesPluginExecutionTaskStatus(snapshot PluginExecutionTaskSnapshot, status string) bool {
	switch status {
	case "", "all":
		return true
	case "active", "running":
		return snapshot.Status == PluginExecutionStatusRunning
	case "finished", "recent":
		return snapshot.Status != PluginExecutionStatusRunning
	default:
		return snapshot.Status == status
	}
}

func (s *PluginManagerService) GetPluginExecutionTask(pluginID uint, taskID string) (*PluginExecutionTaskSnapshot, bool) {
	if s == nil {
		return nil, false
	}

	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, false
	}

	s.taskMu.RLock()
	task := s.executionTasks[taskID]
	s.taskMu.RUnlock()
	if task == nil {
		return nil, false
	}

	snapshot := task.snapshot()
	if pluginID > 0 && snapshot.PluginID != pluginID {
		return nil, false
	}
	return &snapshot, true
}

func (s *PluginManagerService) CancelPluginExecutionTask(pluginID uint, taskID string) (*PluginExecutionTaskSnapshot, error) {
	if s == nil {
		return nil, fmt.Errorf("plugin manager is unavailable")
	}

	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task id is required")
	}

	s.taskMu.RLock()
	task := s.executionTasks[taskID]
	s.taskMu.RUnlock()
	if task == nil {
		return nil, fmt.Errorf("plugin execution task %s not found", taskID)
	}

	snapshot := task.snapshot()
	if pluginID > 0 && snapshot.PluginID != pluginID {
		return nil, fmt.Errorf("plugin execution task %s not found", taskID)
	}
	if !snapshot.Cancelable {
		return &snapshot, fmt.Errorf("plugin execution task %s is not running", taskID)
	}

	updated := task.cancelTask()
	return &updated, nil
}

func (s *PluginManagerService) InspectPluginExecutionTasks(pluginID uint, activeLimit int, recentLimit int) PluginExecutionTaskOverview {
	overview := PluginExecutionTaskOverview{
		Active: []PluginExecutionTaskSnapshot{},
		Recent: []PluginExecutionTaskSnapshot{},
	}
	if s == nil {
		return overview
	}

	if activeLimit <= 0 {
		activeLimit = 10
	}
	if recentLimit <= 0 {
		recentLimit = 10
	}

	active := make([]PluginExecutionTaskSnapshot, 0, activeLimit)
	recent := make([]PluginExecutionTaskSnapshot, 0, recentLimit)
	s.taskMu.RLock()
	defer s.taskMu.RUnlock()
	for i := len(s.executionTaskOrder) - 1; i >= 0; i-- {
		taskID := s.executionTaskOrder[i]
		task := s.executionTasks[taskID]
		if task == nil {
			continue
		}
		snapshot := task.snapshot()
		if pluginID > 0 && snapshot.PluginID != pluginID {
			continue
		}
		if snapshot.Status == PluginExecutionStatusRunning {
			overview.ActiveCount++
			if len(active) < activeLimit {
				active = append(active, snapshot)
			}
			continue
		}
		overview.RecentCount++
		if len(recent) < recentLimit {
			recent = append(recent, snapshot)
		}
	}
	sort.SliceStable(active, func(i, j int) bool {
		return active[i].UpdatedAt.After(active[j].UpdatedAt)
	})
	sort.SliceStable(recent, func(i, j int) bool {
		return recent[i].UpdatedAt.After(recent[j].UpdatedAt)
	})
	overview.Active = active
	overview.Recent = recent
	return overview
}
