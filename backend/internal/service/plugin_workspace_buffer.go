package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pluginipc"
)

const (
	defaultPluginWorkspaceBufferCapacity    = 512
	defaultPluginWorkspaceSeedLimit         = 128
	defaultPluginWorkspaceSnapshotLimit     = 200
	maxPluginWorkspaceSnapshotLimit         = 2000
	defaultPluginWorkspaceControlEventLimit = 24
	defaultPluginWorkspaceOwnerIdleTimeout  = 5 * time.Minute
	pluginWorkspaceSourceWrite              = "plugin.workspace.write"
	pluginWorkspaceSourceWriteln            = "plugin.workspace.writeln"
)

type PluginWorkspaceBufferEntry struct {
	Seq       int64             `json:"seq"`
	Timestamp time.Time         `json:"timestamp"`
	Channel   string            `json:"channel,omitempty"`
	Level     string            `json:"level,omitempty"`
	Message   string            `json:"message,omitempty"`
	Source    string            `json:"source,omitempty"`
	Action    string            `json:"action,omitempty"`
	Hook      string            `json:"hook,omitempty"`
	TaskID    string            `json:"task_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type PluginWorkspaceControlEvent struct {
	Seq             int64     `json:"seq"`
	Timestamp       time.Time `json:"timestamp"`
	Type            string    `json:"type"`
	AdminID         uint      `json:"admin_id,omitempty"`
	OwnerAdminID    uint      `json:"owner_admin_id,omitempty"`
	PreviousOwnerID uint      `json:"previous_owner_id,omitempty"`
	Signal          string    `json:"signal,omitempty"`
	Result          string    `json:"result,omitempty"`
	Message         string    `json:"message,omitempty"`
}

type PluginWorkspaceSnapshot struct {
	PluginID                  uint                          `json:"plugin_id"`
	PluginName                string                        `json:"plugin_name,omitempty"`
	Runtime                   string                        `json:"runtime,omitempty"`
	Enabled                   bool                          `json:"enabled"`
	OwnerAdminID              uint                          `json:"owner_admin_id,omitempty"`
	OwnerLastActiveAt         time.Time                     `json:"owner_last_active_at,omitempty"`
	ViewerCount               int                           `json:"viewer_count"`
	ControlGranted            bool                          `json:"control_granted"`
	ControlIdleTimeoutSeconds int                           `json:"control_idle_timeout_seconds"`
	Status                    string                        `json:"status,omitempty"`
	ActiveTaskID              string                        `json:"active_task_id,omitempty"`
	ActiveCommand             string                        `json:"active_command,omitempty"`
	ActiveCommandID           string                        `json:"active_command_id,omitempty"`
	Interactive               bool                          `json:"interactive,omitempty"`
	WaitingInput              bool                          `json:"waiting_input"`
	Prompt                    string                        `json:"prompt,omitempty"`
	StartedAt                 time.Time                     `json:"started_at,omitempty"`
	CompletedAt               *time.Time                    `json:"completed_at,omitempty"`
	CompletionReason          string                        `json:"completion_reason,omitempty"`
	LastError                 string                        `json:"last_error,omitempty"`
	BufferCapacity            int                           `json:"buffer_capacity"`
	EntryCount                int                           `json:"entry_count"`
	LastSeq                   int64                         `json:"last_seq"`
	UpdatedAt                 time.Time                     `json:"updated_at,omitempty"`
	HasMore                   bool                          `json:"has_more"`
	RecentControlEvents       []PluginWorkspaceControlEvent `json:"recent_control_events,omitempty"`
	Entries                   []PluginWorkspaceBufferEntry  `json:"entries"`
}

type PluginWorkspaceStreamEvent struct {
	Type       string                       `json:"type"`
	Workspace  *PluginWorkspaceSnapshot     `json:"workspace,omitempty"`
	Entries    []PluginWorkspaceBufferEntry `json:"entries,omitempty"`
	Cleared    bool                         `json:"cleared,omitempty"`
	LastSeq    int64                        `json:"last_seq,omitempty"`
	EntryCount int                          `json:"entry_count,omitempty"`
	UpdatedAt  time.Time                    `json:"updated_at,omitempty"`
}

type pluginWorkspaceBuffer struct {
	pluginID          uint
	pluginName        string
	runtime           string
	maxEntries        int
	status            string
	activeTaskID      string
	activeCommand     string
	activeCommandID   string
	interactive       bool
	waitingInput      bool
	prompt            string
	startedAt         time.Time
	completedAt       *time.Time
	completionReason  string
	lastError         string
	lastSeq           int64
	updatedAt         time.Time
	entries           []PluginWorkspaceBufferEntry
	nextSubscriberID  uint64
	subscribers       map[uint64]chan PluginWorkspaceStreamEvent
	ownerAdminID      uint
	ownerLastActiveAt time.Time
	attachedAdmins    map[uint]int
	subscriberAdmins  map[uint64]uint
	controlEvents     []PluginWorkspaceControlEvent
	nextControlSeq    int64
}

func newPluginWorkspaceBuffer(pluginID uint, pluginName string, runtime string, maxEntries int) *pluginWorkspaceBuffer {
	if maxEntries <= 0 {
		maxEntries = defaultPluginWorkspaceBufferCapacity
	}
	return &pluginWorkspaceBuffer{
		pluginID:         pluginID,
		pluginName:       strings.TrimSpace(pluginName),
		runtime:          strings.TrimSpace(runtime),
		maxEntries:       maxEntries,
		status:           "idle",
		entries:          make([]PluginWorkspaceBufferEntry, 0, maxEntries),
		subscribers:      make(map[uint64]chan PluginWorkspaceStreamEvent),
		attachedAdmins:   make(map[uint]int),
		subscriberAdmins: make(map[uint64]uint),
	}
}

func normalizePluginWorkspaceLimit(limit int, fallback int) int {
	if fallback <= 0 {
		fallback = defaultPluginWorkspaceSnapshotLimit
	}
	if limit <= 0 {
		limit = fallback
	}
	if limit > maxPluginWorkspaceSnapshotLimit {
		return maxPluginWorkspaceSnapshotLimit
	}
	return limit
}

func normalizePluginWorkspaceLevel(level string) string {
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

func normalizePluginWorkspaceChannel(channel string) string {
	trimmed := strings.ToLower(strings.TrimSpace(channel))
	if trimmed == "" {
		return "workspace"
	}
	return trimmed
}

func parsePluginWorkspaceTimestamp(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Now().UTC()
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed.UTC()
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed.UTC()
	}
	return time.Now().UTC()
}

func mergePluginWorkspaceMetadata(base map[string]string, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := cloneStringMap(base)
	if out == nil {
		out = make(map[string]string, len(extra))
	}
	for key, value := range extra {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		out[normalizedKey] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func pluginWorkspaceEntryFromIPC(entry pluginipc.WorkspaceBufferEntry, baseMetadata map[string]string) PluginWorkspaceBufferEntry {
	metadata := mergePluginWorkspaceMetadata(baseMetadata, entry.Metadata)
	action := ""
	hook := ""
	taskID := ""
	if len(metadata) > 0 {
		action = strings.TrimSpace(metadata["action"])
		hook = strings.TrimSpace(metadata["hook"])
		taskID = strings.TrimSpace(metadata["task_id"])
	}
	return PluginWorkspaceBufferEntry{
		Timestamp: parsePluginWorkspaceTimestamp(entry.Timestamp),
		Channel:   normalizePluginWorkspaceChannel(entry.Channel),
		Level:     normalizePluginWorkspaceLevel(entry.Level),
		Message:   entry.Message,
		Source:    strings.TrimSpace(entry.Source),
		Action:    action,
		Hook:      hook,
		TaskID:    taskID,
		Metadata:  metadata,
	}
}

func pluginWorkspaceEntryToIPC(entry PluginWorkspaceBufferEntry) pluginipc.WorkspaceBufferEntry {
	return pluginipc.WorkspaceBufferEntry{
		Timestamp: entry.Timestamp.UTC().Format(time.RFC3339Nano),
		Channel:   normalizePluginWorkspaceChannel(entry.Channel),
		Level:     normalizePluginWorkspaceLevel(entry.Level),
		Message:   entry.Message,
		Source:    strings.TrimSpace(entry.Source),
		Metadata:  cloneStringMap(entry.Metadata),
	}
}

func clonePluginWorkspaceEntries(entries []PluginWorkspaceBufferEntry) []PluginWorkspaceBufferEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]PluginWorkspaceBufferEntry, 0, len(entries))
	for _, entry := range entries {
		cloned = append(cloned, clonePluginWorkspaceEntry(entry))
	}
	return cloned
}

func clonePluginWorkspaceEntry(entry PluginWorkspaceBufferEntry) PluginWorkspaceBufferEntry {
	return PluginWorkspaceBufferEntry{
		Seq:       entry.Seq,
		Timestamp: entry.Timestamp.UTC(),
		Channel:   entry.Channel,
		Level:     entry.Level,
		Message:   entry.Message,
		Source:    entry.Source,
		Action:    entry.Action,
		Hook:      entry.Hook,
		TaskID:    entry.TaskID,
		Metadata:  cloneStringMap(entry.Metadata),
	}
}

func isPluginWorkspaceTerminalStreamEntry(entry PluginWorkspaceBufferEntry) bool {
	source := strings.ToLower(strings.TrimSpace(entry.Source))
	channel := normalizePluginWorkspaceChannel(entry.Channel)
	return channel == "stdout" &&
		(source == pluginWorkspaceSourceWrite || source == pluginWorkspaceSourceWriteln)
}

func equalPluginWorkspaceMetadata(left map[string]string, right map[string]string) bool {
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

func canMergePluginWorkspaceEntry(left PluginWorkspaceBufferEntry, right PluginWorkspaceBufferEntry) bool {
	if !isPluginWorkspaceTerminalStreamEntry(left) || !isPluginWorkspaceTerminalStreamEntry(right) {
		return false
	}
	if normalizePluginWorkspaceLevel(left.Level) != normalizePluginWorkspaceLevel(right.Level) {
		return false
	}
	return equalPluginWorkspaceMetadata(left.Metadata, right.Metadata)
}

func clonePluginWorkspaceControlEvents(events []PluginWorkspaceControlEvent) []PluginWorkspaceControlEvent {
	if len(events) == 0 {
		return nil
	}
	cloned := make([]PluginWorkspaceControlEvent, 0, len(events))
	for _, event := range events {
		cloned = append(cloned, PluginWorkspaceControlEvent{
			Seq:             event.Seq,
			Timestamp:       event.Timestamp.UTC(),
			Type:            event.Type,
			AdminID:         event.AdminID,
			OwnerAdminID:    event.OwnerAdminID,
			PreviousOwnerID: event.PreviousOwnerID,
			Signal:          event.Signal,
			Result:          event.Result,
			Message:         event.Message,
		})
	}
	return cloned
}

func (b *pluginWorkspaceBuffer) updateIdentity(pluginName string, runtime string) {
	if b == nil {
		return
	}
	if trimmed := strings.TrimSpace(pluginName); trimmed != "" {
		b.pluginName = trimmed
	}
	if trimmed := strings.TrimSpace(runtime); trimmed != "" {
		b.runtime = trimmed
	}
	if b.maxEntries <= 0 {
		b.maxEntries = defaultPluginWorkspaceBufferCapacity
	}
}

func (b *pluginWorkspaceBuffer) apply(entries []PluginWorkspaceBufferEntry, cleared bool) []PluginWorkspaceBufferEntry {
	if b == nil {
		return nil
	}
	now := time.Now().UTC()
	emitted := make([]PluginWorkspaceBufferEntry, 0, len(entries))
	if cleared {
		b.entries = nil
		b.updatedAt = now
	}
	for _, entry := range entries {
		if entry.Timestamp.IsZero() {
			entry.Timestamp = now
		} else {
			entry.Timestamp = entry.Timestamp.UTC()
		}
		if len(b.entries) > 0 && canMergePluginWorkspaceEntry(b.entries[len(b.entries)-1], entry) {
			last := &b.entries[len(b.entries)-1]
			last.Message += entry.Message
			last.Timestamp = entry.Timestamp
			b.updatedAt = entry.Timestamp
			emitted = append(emitted, clonePluginWorkspaceEntry(*last))
			continue
		}
		b.lastSeq++
		entry.Seq = b.lastSeq
		b.entries = append(b.entries, entry)
		b.updatedAt = entry.Timestamp
		emitted = append(emitted, clonePluginWorkspaceEntry(entry))
	}
	if b.maxEntries <= 0 {
		b.maxEntries = defaultPluginWorkspaceBufferCapacity
	}
	if len(b.entries) > b.maxEntries {
		b.entries = append([]PluginWorkspaceBufferEntry(nil), b.entries[len(b.entries)-b.maxEntries:]...)
	}
	return emitted
}

func (b *pluginWorkspaceBuffer) snapshot(limit int) PluginWorkspaceSnapshot {
	snapshot := PluginWorkspaceSnapshot{
		PluginID:                  b.pluginID,
		PluginName:                b.pluginName,
		Runtime:                   b.runtime,
		Enabled:                   strings.TrimSpace(b.runtime) == PluginRuntimeJSWorker,
		OwnerAdminID:              b.ownerAdminID,
		OwnerLastActiveAt:         b.ownerLastActiveAt,
		ViewerCount:               b.viewerCount(),
		ControlIdleTimeoutSeconds: int(defaultPluginWorkspaceOwnerIdleTimeout / time.Second),
		Status:                    b.status,
		ActiveTaskID:              b.activeTaskID,
		ActiveCommand:             b.activeCommand,
		ActiveCommandID:           b.activeCommandID,
		Interactive:               b.interactive,
		WaitingInput:              b.waitingInput,
		Prompt:                    b.prompt,
		StartedAt:                 b.startedAt,
		CompletedAt:               cloneOptionalTime(b.completedAt),
		CompletionReason:          b.completionReason,
		LastError:                 b.lastError,
		BufferCapacity:            b.maxEntries,
		EntryCount:                len(b.entries),
		LastSeq:                   b.lastSeq,
		UpdatedAt:                 b.updatedAt,
	}
	snapshot.Entries = b.tail(limit)
	snapshot.RecentControlEvents = clonePluginWorkspaceControlEvents(b.controlEvents)
	snapshot.HasMore = snapshot.EntryCount > len(snapshot.Entries)
	return snapshot
}

func (b *pluginWorkspaceBuffer) recordControlEvent(
	eventType string,
	adminID uint,
	previousOwnerID uint,
	ownerAdminID uint,
	message string,
) {
	b.recordControlEventDetailed(eventType, adminID, previousOwnerID, ownerAdminID, "", "", message)
}

func (b *pluginWorkspaceBuffer) recordControlEventDetailed(
	eventType string,
	adminID uint,
	previousOwnerID uint,
	ownerAdminID uint,
	signal string,
	result string,
	message string,
) {
	if b == nil {
		return
	}
	normalizedType := strings.TrimSpace(eventType)
	if normalizedType == "" {
		return
	}
	b.nextControlSeq++
	event := PluginWorkspaceControlEvent{
		Seq:             b.nextControlSeq,
		Timestamp:       time.Now().UTC(),
		Type:            normalizedType,
		AdminID:         adminID,
		OwnerAdminID:    ownerAdminID,
		PreviousOwnerID: previousOwnerID,
		Signal:          strings.ToLower(strings.TrimSpace(signal)),
		Result:          strings.ToLower(strings.TrimSpace(result)),
		Message:         strings.TrimSpace(message),
	}
	b.controlEvents = append(b.controlEvents, event)
	if len(b.controlEvents) > defaultPluginWorkspaceControlEventLimit {
		b.controlEvents = append([]PluginWorkspaceControlEvent(nil), b.controlEvents[len(b.controlEvents)-defaultPluginWorkspaceControlEventLimit:]...)
	}
}

func (b *pluginWorkspaceBuffer) touchOwnerActivity(adminID uint) bool {
	if b == nil || adminID == 0 || b.ownerAdminID == 0 || b.ownerAdminID != adminID {
		return false
	}
	b.ownerLastActiveAt = time.Now().UTC()
	return true
}

func (b *pluginWorkspaceBuffer) expireIdleOwner(now time.Time, timeout time.Duration) (uint, uint, bool) {
	if b == nil || b.ownerAdminID == 0 || timeout <= 0 {
		return 0, 0, false
	}
	if b.ownerLastActiveAt.IsZero() || now.UTC().Sub(b.ownerLastActiveAt.UTC()) < timeout {
		return 0, 0, false
	}
	previousOwner := b.ownerAdminID
	nextOwner := uint(0)
	attached := make([]uint, 0, len(b.attachedAdmins))
	for adminID, count := range b.attachedAdmins {
		if count <= 0 || adminID == previousOwner {
			continue
		}
		attached = append(attached, adminID)
	}
	if len(attached) > 0 {
		sort.Slice(attached, func(i, j int) bool {
			return attached[i] < attached[j]
		})
		nextOwner = attached[0]
	}
	b.ownerAdminID = nextOwner
	if nextOwner != 0 {
		b.ownerLastActiveAt = now.UTC()
		b.recordControlEvent(
			"control_auto_transferred",
			0,
			previousOwner,
			nextOwner,
			fmt.Sprintf("Workspace control auto-transferred from admin #%d to admin #%d after inactivity.", previousOwner, nextOwner),
		)
	} else {
		b.ownerLastActiveAt = time.Time{}
		b.recordControlEvent(
			"control_auto_released",
			0,
			previousOwner,
			0,
			fmt.Sprintf("Workspace control held by admin #%d was auto-released after inactivity.", previousOwner),
		)
	}
	return previousOwner, nextOwner, true
}

func (b *pluginWorkspaceBuffer) viewerCount() int {
	if b == nil || len(b.attachedAdmins) == 0 {
		return 0
	}
	count := 0
	for adminID, attached := range b.attachedAdmins {
		if attached <= 0 {
			continue
		}
		if b.ownerAdminID != 0 && adminID == b.ownerAdminID {
			continue
		}
		count++
	}
	return count
}

func (b *pluginWorkspaceBuffer) isAdminAttached(adminID uint) bool {
	if b == nil || adminID == 0 || len(b.attachedAdmins) == 0 {
		return false
	}
	return b.attachedAdmins[adminID] > 0
}

func (b *pluginWorkspaceBuffer) selectNextOwner() uint {
	if b == nil || len(b.attachedAdmins) == 0 {
		return 0
	}
	attached := make([]uint, 0, len(b.attachedAdmins))
	for adminID, count := range b.attachedAdmins {
		if count <= 0 {
			continue
		}
		attached = append(attached, adminID)
	}
	if len(attached) == 0 {
		return 0
	}
	sort.Slice(attached, func(i, j int) bool {
		return attached[i] < attached[j]
	})
	return attached[0]
}

func (b *pluginWorkspaceBuffer) reconcileOwner() bool {
	if b == nil {
		return false
	}
	previousOwner := b.ownerAdminID
	if previousOwner != 0 && b.isAdminAttached(previousOwner) {
		return false
	}
	nextOwner := b.selectNextOwner()
	if nextOwner == previousOwner {
		return false
	}
	b.ownerAdminID = nextOwner
	if nextOwner == 0 {
		b.ownerLastActiveAt = time.Time{}
	} else {
		b.ownerLastActiveAt = time.Now().UTC()
	}
	return true
}

func (b *pluginWorkspaceBuffer) ensureControl(adminID uint) error {
	if b == nil {
		return fmt.Errorf("plugin workspace is unavailable")
	}
	if adminID == 0 {
		return fmt.Errorf("plugin workspace controller is required")
	}
	_ = b.reconcileOwner()
	if b.ownerAdminID == 0 {
		b.ownerAdminID = adminID
		b.ownerLastActiveAt = time.Now().UTC()
		b.recordControlEvent(
			"control_assigned",
			adminID,
			0,
			adminID,
			fmt.Sprintf("Workspace control assigned to admin #%d.", adminID),
		)
		return nil
	}
	if b.ownerAdminID == adminID {
		b.ownerLastActiveAt = time.Now().UTC()
		return nil
	}
	return fmt.Errorf("plugin workspace is currently controlled by admin #%d", b.ownerAdminID)
}

func (b *pluginWorkspaceBuffer) claimControl(adminID uint) (uint, bool, error) {
	if b == nil {
		return 0, false, fmt.Errorf("plugin workspace is unavailable")
	}
	if adminID == 0 {
		return 0, false, fmt.Errorf("plugin workspace controller is required")
	}
	_ = b.reconcileOwner()
	if !b.isAdminAttached(adminID) {
		return b.ownerAdminID, false, fmt.Errorf("attach to the plugin workspace before claiming control")
	}
	previousOwner := b.ownerAdminID
	if previousOwner == adminID {
		b.ownerLastActiveAt = time.Now().UTC()
		return previousOwner, false, nil
	}
	b.ownerAdminID = adminID
	b.ownerLastActiveAt = time.Now().UTC()
	b.recordControlEvent(
		"control_claimed",
		adminID,
		previousOwner,
		adminID,
		fmt.Sprintf("Workspace control claimed by admin #%d.", adminID),
	)
	return previousOwner, true, nil
}

func (b *pluginWorkspaceBuffer) setSessionState(
	status string,
	taskID string,
	commandName string,
	commandID string,
	interactive bool,
	waitingInput bool,
	prompt string,
	startedAt time.Time,
	completedAt *time.Time,
	completionReason string,
	lastError string,
) {
	if b == nil {
		return
	}
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	if normalizedStatus == "" {
		normalizedStatus = "idle"
	}
	b.status = normalizedStatus
	b.activeTaskID = strings.TrimSpace(taskID)
	b.activeCommand = strings.TrimSpace(commandName)
	b.activeCommandID = strings.TrimSpace(commandID)
	b.interactive = interactive
	b.waitingInput = waitingInput
	b.prompt = strings.TrimSpace(prompt)
	b.completionReason = strings.TrimSpace(completionReason)
	b.lastError = strings.TrimSpace(lastError)
	if !startedAt.IsZero() {
		b.startedAt = startedAt.UTC()
	} else if normalizedStatus == "idle" {
		b.startedAt = time.Time{}
	}
	b.completedAt = cloneOptionalTime(completedAt)
	b.updatedAt = time.Now().UTC()
}

func (b *pluginWorkspaceBuffer) snapshotEvent(eventType string) PluginWorkspaceStreamEvent {
	snapshot := b.snapshot(b.maxEntries)
	return PluginWorkspaceStreamEvent{
		Type:       strings.TrimSpace(eventType),
		Workspace:  &snapshot,
		LastSeq:    snapshot.LastSeq,
		EntryCount: snapshot.EntryCount,
		UpdatedAt:  snapshot.UpdatedAt,
	}
}

func clonePluginWorkspaceSnapshot(snapshot *PluginWorkspaceSnapshot) *PluginWorkspaceSnapshot {
	if snapshot == nil {
		return nil
	}
	cloned := *snapshot
	cloned.CompletedAt = cloneOptionalTime(snapshot.CompletedAt)
	cloned.Entries = clonePluginWorkspaceEntries(snapshot.Entries)
	cloned.RecentControlEvents = clonePluginWorkspaceControlEvents(snapshot.RecentControlEvents)
	return &cloned
}

func (b *pluginWorkspaceBuffer) subscribe(limit int, adminID uint) (uint64, chan PluginWorkspaceStreamEvent, PluginWorkspaceSnapshot, bool) {
	if b.subscribers == nil {
		b.subscribers = make(map[uint64]chan PluginWorkspaceStreamEvent)
	}
	if b.attachedAdmins == nil {
		b.attachedAdmins = make(map[uint]int)
	}
	if b.subscriberAdmins == nil {
		b.subscriberAdmins = make(map[uint64]uint)
	}
	_ = b.reconcileOwner()
	previousOwner := b.ownerAdminID
	previousViewerCount := b.viewerCount()
	b.nextSubscriberID++
	id := b.nextSubscriberID
	ch := make(chan PluginWorkspaceStreamEvent, 32)
	b.subscribers[id] = ch
	if adminID > 0 {
		previousAttachCount := b.attachedAdmins[adminID]
		b.subscriberAdmins[id] = adminID
		b.attachedAdmins[adminID]++
		if b.ownerAdminID == 0 || !b.isAdminAttached(b.ownerAdminID) {
			previousOwner := b.ownerAdminID
			b.ownerAdminID = adminID
			b.ownerLastActiveAt = time.Now().UTC()
			b.recordControlEvent(
				"control_assigned",
				adminID,
				previousOwner,
				adminID,
				fmt.Sprintf("Workspace control assigned to admin #%d.", adminID),
			)
		} else if previousAttachCount == 0 {
			b.recordControlEvent(
				"viewer_attached",
				adminID,
				b.ownerAdminID,
				b.ownerAdminID,
				fmt.Sprintf("Admin #%d attached to the workspace as viewer.", adminID),
			)
		}
	}
	snapshot := b.snapshot(limit)
	return id, ch, snapshot, previousOwner != b.ownerAdminID || previousViewerCount != snapshot.ViewerCount
}

func (b *pluginWorkspaceBuffer) unsubscribe(id uint64) bool {
	if b == nil || id == 0 || b.subscribers == nil {
		return false
	}
	previousOwner := b.ownerAdminID
	previousViewerCount := b.viewerCount()
	ch, ok := b.subscribers[id]
	if !ok {
		return false
	}
	delete(b.subscribers, id)
	if adminID := b.subscriberAdmins[id]; adminID > 0 {
		delete(b.subscriberAdmins, id)
		if attached := b.attachedAdmins[adminID]; attached <= 1 {
			delete(b.attachedAdmins, adminID)
			if adminID == b.ownerAdminID {
				nextOwner := b.selectNextOwner()
				b.ownerAdminID = nextOwner
				if nextOwner != 0 {
					b.ownerLastActiveAt = time.Now().UTC()
					b.recordControlEvent(
						"control_transferred",
						0,
						adminID,
						nextOwner,
						fmt.Sprintf("Workspace control transferred from admin #%d to admin #%d.", adminID, nextOwner),
					)
				} else {
					b.ownerLastActiveAt = time.Time{}
					b.recordControlEvent(
						"control_released",
						0,
						adminID,
						0,
						fmt.Sprintf("Workspace control released after admin #%d detached.", adminID),
					)
				}
			} else {
				b.recordControlEvent(
					"viewer_detached",
					adminID,
					b.ownerAdminID,
					b.ownerAdminID,
					fmt.Sprintf("Admin #%d detached from the workspace viewer session.", adminID),
				)
			}
		} else {
			b.attachedAdmins[adminID] = attached - 1
		}
	} else {
		delete(b.subscriberAdmins, id)
	}
	close(ch)
	return previousOwner != b.ownerAdminID || previousViewerCount != b.viewerCount()
}

func (b *pluginWorkspaceBuffer) closeAllSubscribers() {
	if b == nil || len(b.subscribers) == 0 {
		return
	}
	for id := range b.subscribers {
		b.unsubscribe(id)
	}
}

func (b *pluginWorkspaceBuffer) tail(limit int) []PluginWorkspaceBufferEntry {
	if b == nil || len(b.entries) == 0 {
		return nil
	}
	if limit <= 0 || limit >= len(b.entries) {
		return clonePluginWorkspaceEntries(b.entries)
	}
	return clonePluginWorkspaceEntries(b.entries[len(b.entries)-limit:])
}

func (s *PluginManagerService) ensurePluginWorkspaceBufferLocked(pluginID uint, pluginName string, runtime string) *pluginWorkspaceBuffer {
	if s.workspaceBuffers == nil {
		s.workspaceBuffers = make(map[uint]*pluginWorkspaceBuffer)
	}
	buffer := s.workspaceBuffers[pluginID]
	if buffer == nil {
		buffer = newPluginWorkspaceBuffer(pluginID, pluginName, runtime, defaultPluginWorkspaceBufferCapacity)
		s.workspaceBuffers[pluginID] = buffer
	}
	buffer.updateIdentity(pluginName, runtime)
	return buffer
}

func shouldSeedPluginWorkspaceHistory(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case strings.ToLower(pluginWorkspaceCommandExecuteAction),
		strings.ToLower(pluginWorkspaceRuntimeEvalAction),
		strings.ToLower(pluginWorkspaceRuntimeInspectAction):
		return true
	default:
		return false
	}
}

func (s *PluginManagerService) PreparePluginWorkspaceConfig(plugin *models.Plugin, action string, execCtx *ExecutionContext, limit int) *pluginipc.WorkspaceConfig {
	if s == nil || plugin == nil {
		return nil
	}
	_ = execCtx

	runtime := strings.ToLower(strings.TrimSpace(plugin.Runtime))
	if runtime != PluginRuntimeJSWorker {
		return nil
	}

	seedLimit := normalizePluginWorkspaceLimit(limit, defaultPluginWorkspaceSeedLimit)
	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, runtime)
	maxEntries := buffer.maxEntries
	history := []PluginWorkspaceBufferEntry(nil)
	if shouldSeedPluginWorkspaceHistory(action) {
		history = buffer.tail(seedLimit)
	}
	s.workspaceMu.Unlock()

	historyEntries := make([]pluginipc.WorkspaceBufferEntry, 0, len(history))
	for _, entry := range history {
		historyEntries = append(historyEntries, pluginWorkspaceEntryToIPC(entry))
	}
	return &pluginipc.WorkspaceConfig{
		Enabled:    true,
		MaxEntries: maxEntries,
		History:    historyEntries,
	}
}

func (s *PluginManagerService) ApplyPluginWorkspaceDelta(
	pluginID uint,
	pluginName string,
	runtime string,
	baseMetadata map[string]string,
	entries []pluginipc.WorkspaceBufferEntry,
	cleared bool,
) {
	if s == nil || pluginID == 0 {
		return
	}
	if !cleared && len(entries) == 0 {
		return
	}

	normalizedEntries := make([]PluginWorkspaceBufferEntry, 0, len(entries))
	for _, entry := range entries {
		normalizedEntries = append(normalizedEntries, pluginWorkspaceEntryFromIPC(entry, baseMetadata))
	}

	var subscribers []chan PluginWorkspaceStreamEvent
	var event PluginWorkspaceStreamEvent
	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(pluginID, pluginName, runtime)
	emittedEntries := buffer.apply(normalizedEntries, cleared)
	if len(buffer.subscribers) > 0 {
		subscribers = make([]chan PluginWorkspaceStreamEvent, 0, len(buffer.subscribers))
		for _, ch := range buffer.subscribers {
			subscribers = append(subscribers, ch)
		}
		event = PluginWorkspaceStreamEvent{
			Type:       "delta",
			Entries:    clonePluginWorkspaceEntries(emittedEntries),
			Cleared:    cleared,
			LastSeq:    buffer.lastSeq,
			EntryCount: len(buffer.entries),
			UpdatedAt:  buffer.updatedAt,
		}
	}
	s.workspaceMu.Unlock()
	if len(subscribers) == 0 {
		return
	}
	for _, ch := range subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (s *PluginManagerService) GetPluginWorkspaceSnapshot(plugin *models.Plugin, limit int) PluginWorkspaceSnapshot {
	snapshot := PluginWorkspaceSnapshot{}
	if plugin != nil {
		snapshot.PluginID = plugin.ID
		snapshot.PluginName = strings.TrimSpace(plugin.Name)
		snapshot.Runtime = strings.ToLower(strings.TrimSpace(plugin.Runtime))
		snapshot.Enabled = snapshot.Runtime == PluginRuntimeJSWorker
	}
	if s == nil || plugin == nil {
		return snapshot
	}

	viewLimit := normalizePluginWorkspaceLimit(limit, defaultPluginWorkspaceSnapshotLimit)
	_, _, _ = s.maybeExpirePluginWorkspaceOwner(plugin)
	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, snapshot.Runtime)
	_ = buffer.reconcileOwner()
	snapshot = buffer.snapshot(viewLimit)
	s.workspaceMu.Unlock()
	return snapshot
}

func (s *PluginManagerService) SubscribePluginWorkspace(
	plugin *models.Plugin,
	adminID uint,
	limit int,
) (PluginWorkspaceSnapshot, <-chan PluginWorkspaceStreamEvent, func(), error) {
	if s == nil || plugin == nil {
		return PluginWorkspaceSnapshot{}, nil, func() {}, nil
	}
	runtime := strings.ToLower(strings.TrimSpace(plugin.Runtime))
	if runtime != PluginRuntimeJSWorker {
		return PluginWorkspaceSnapshot{}, nil, func() {}, nil
	}

	viewLimit := normalizePluginWorkspaceLimit(limit, defaultPluginWorkspaceSnapshotLimit)
	var subscribers []chan PluginWorkspaceStreamEvent
	var event PluginWorkspaceStreamEvent
	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, runtime)
	if _, _, changed := buffer.expireIdleOwner(time.Now().UTC(), defaultPluginWorkspaceOwnerIdleTimeout); changed {
		subscribers = clonePluginWorkspaceStreamSubscribers(buffer.subscribers)
		event = buffer.snapshotEvent("control")
	}
	subscriptionID, ch, snapshot, presenceChanged := buffer.subscribe(viewLimit, adminID)
	if presenceChanged {
		subscribers = clonePluginWorkspaceStreamSubscribersExcept(buffer.subscribers, subscriptionID)
		event = buffer.snapshotEvent("presence")
	}
	s.workspaceMu.Unlock()
	dispatchPluginWorkspaceStreamEvent(subscribers, event)

	cancel := func() {
		var unsubscribeSubscribers []chan PluginWorkspaceStreamEvent
		var unsubscribeEvent PluginWorkspaceStreamEvent
		s.workspaceMu.Lock()
		buffer := s.workspaceBuffers[plugin.ID]
		if buffer != nil {
			presenceChanged := buffer.unsubscribe(subscriptionID)
			if presenceChanged {
				unsubscribeSubscribers = clonePluginWorkspaceStreamSubscribers(buffer.subscribers)
				unsubscribeEvent = buffer.snapshotEvent("presence")
			}
		}
		s.workspaceMu.Unlock()
		dispatchPluginWorkspaceStreamEvent(unsubscribeSubscribers, unsubscribeEvent)
	}
	return snapshot, ch, cancel, nil
}

func (s *PluginManagerService) ClearPluginWorkspace(plugin *models.Plugin, adminID uint) (PluginWorkspaceSnapshot, error) {
	if s == nil || plugin == nil {
		return PluginWorkspaceSnapshot{}, fmt.Errorf("plugin workspace is unavailable")
	}
	runtime := strings.ToLower(strings.TrimSpace(plugin.Runtime))
	if runtime != PluginRuntimeJSWorker {
		return PluginWorkspaceSnapshot{}, fmt.Errorf("workspace is only available for js_worker plugins")
	}

	var (
		snapshot    PluginWorkspaceSnapshot
		subscribers []chan PluginWorkspaceStreamEvent
		event       PluginWorkspaceStreamEvent
	)
	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, runtime)
	if err := buffer.ensureControl(adminID); err != nil {
		snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return snapshot, err
	}
	buffer.touchOwnerActivity(adminID)
	buffer.recordControlEvent(
		"workspace_cleared",
		adminID,
		buffer.ownerAdminID,
		buffer.ownerAdminID,
		fmt.Sprintf("Workspace buffer cleared by admin #%d.", adminID),
	)
	buffer.apply(nil, true)
	snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
	subscribers = clonePluginWorkspaceStreamSubscribers(buffer.subscribers)
	event = buffer.snapshotEvent("state")
	s.workspaceMu.Unlock()
	dispatchPluginWorkspaceStreamEvent(subscribers, event)
	return snapshot, nil
}

func (s *PluginManagerService) maybeExpirePluginWorkspaceOwner(plugin *models.Plugin) (uint, uint, bool) {
	if s == nil || plugin == nil {
		return 0, 0, false
	}
	runtime := strings.ToLower(strings.TrimSpace(plugin.Runtime))
	if runtime != PluginRuntimeJSWorker {
		return 0, 0, false
	}

	var (
		previousOwner uint
		nextOwner     uint
		changed       bool
		subscribers   []chan PluginWorkspaceStreamEvent
		event         PluginWorkspaceStreamEvent
	)
	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, runtime)
	previousOwner, nextOwner, changed = buffer.expireIdleOwner(time.Now().UTC(), defaultPluginWorkspaceOwnerIdleTimeout)
	if changed {
		subscribers = clonePluginWorkspaceStreamSubscribers(buffer.subscribers)
		event = buffer.snapshotEvent("control")
	}
	s.workspaceMu.Unlock()
	dispatchPluginWorkspaceStreamEvent(subscribers, event)
	return previousOwner, nextOwner, changed
}

func (s *PluginManagerService) TickPluginWorkspace(plugin *models.Plugin) {
	if s == nil || plugin == nil {
		return
	}
	_, _, _ = s.maybeExpirePluginWorkspaceOwner(plugin)
}

func (s *PluginManagerService) NotePluginWorkspaceControlActivity(
	plugin *models.Plugin,
	adminID uint,
	eventType string,
	message string,
) (PluginWorkspaceSnapshot, error) {
	return s.notePluginWorkspaceControlActivityDetailed(plugin, adminID, eventType, "", "", message)
}

func (s *PluginManagerService) notePluginWorkspaceControlActivityDetailed(
	plugin *models.Plugin,
	adminID uint,
	eventType string,
	signal string,
	result string,
	message string,
) (PluginWorkspaceSnapshot, error) {
	if s == nil || plugin == nil {
		return PluginWorkspaceSnapshot{}, fmt.Errorf("plugin workspace is unavailable")
	}
	runtime := strings.ToLower(strings.TrimSpace(plugin.Runtime))
	if runtime != PluginRuntimeJSWorker {
		return PluginWorkspaceSnapshot{}, fmt.Errorf("workspace is only available for js_worker plugins")
	}

	var (
		snapshot    PluginWorkspaceSnapshot
		subscribers []chan PluginWorkspaceStreamEvent
		event       PluginWorkspaceStreamEvent
	)
	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, runtime)
	if _, _, changed := buffer.expireIdleOwner(time.Now().UTC(), defaultPluginWorkspaceOwnerIdleTimeout); changed {
		subscribers = clonePluginWorkspaceStreamSubscribers(buffer.subscribers)
		event = buffer.snapshotEvent("control")
	}
	if strings.TrimSpace(eventType) != "" {
		if adminID == 0 || buffer.ownerAdminID == 0 || buffer.ownerAdminID == adminID {
			buffer.touchOwnerActivity(func() uint {
				if adminID != 0 {
					return adminID
				}
				return buffer.ownerAdminID
			}())
		}
		buffer.recordControlEventDetailed(
			eventType,
			adminID,
			buffer.ownerAdminID,
			buffer.ownerAdminID,
			signal,
			result,
			message,
		)
		snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		subscribers = clonePluginWorkspaceStreamSubscribers(buffer.subscribers)
		event = buffer.snapshotEvent("control")
	} else if event.Workspace == nil {
		snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
	}
	s.workspaceMu.Unlock()
	dispatchPluginWorkspaceStreamEvent(subscribers, event)
	return snapshot, nil
}

func (s *PluginManagerService) ClaimPluginWorkspaceControl(
	plugin *models.Plugin,
	adminID uint,
) (PluginWorkspaceSnapshot, uint, bool, error) {
	if s == nil || plugin == nil {
		return PluginWorkspaceSnapshot{}, 0, false, fmt.Errorf("plugin workspace is unavailable")
	}
	runtime := strings.ToLower(strings.TrimSpace(plugin.Runtime))
	if runtime != PluginRuntimeJSWorker {
		return PluginWorkspaceSnapshot{}, 0, false, fmt.Errorf("workspace is only available for js_worker plugins")
	}

	var (
		snapshot      PluginWorkspaceSnapshot
		previousOwner uint
		claimed       bool
		subscribers   []chan PluginWorkspaceStreamEvent
		event         PluginWorkspaceStreamEvent
	)
	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, runtime)
	var err error
	previousOwner, claimed, err = buffer.claimControl(adminID)
	if err != nil {
		snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return snapshot, previousOwner, false, err
	}
	snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
	if claimed {
		subscribers = clonePluginWorkspaceStreamSubscribers(buffer.subscribers)
		event = buffer.snapshotEvent("control")
	}
	s.workspaceMu.Unlock()
	dispatchPluginWorkspaceStreamEvent(subscribers, event)
	return snapshot, previousOwner, claimed, nil
}

func (s *PluginManagerService) RemovePluginWorkspace(pluginID uint) {
	if s == nil || pluginID == 0 {
		return
	}
	s.workspaceMu.Lock()
	if buffer := s.workspaceBuffers[pluginID]; buffer != nil {
		buffer.closeAllSubscribers()
	}
	delete(s.workspaceBuffers, pluginID)
	delete(s.workspaceSessions, pluginID)
	s.workspaceMu.Unlock()
}
