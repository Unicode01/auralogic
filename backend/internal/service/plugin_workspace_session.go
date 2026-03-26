package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"auralogic/internal/models"
	"auralogic/internal/pluginipc"
)

const (
	pluginWorkspaceStatusIdle         = "idle"
	pluginWorkspaceStatusRunning      = "running"
	pluginWorkspaceStatusWaitingInput = "waiting_input"

	pluginWorkspaceCompletionReasonInterrupted = "interrupted"
	pluginWorkspaceCompletionReasonTerminated  = "terminated"

	defaultPluginWorkspaceCommandTimeout     = 3 * time.Minute
	defaultPluginWorkspaceInteractiveTimeout = 15 * time.Minute
	defaultPluginWorkspaceInputWaitTimeout   = 15 * time.Minute
)

type pluginWorkspaceSession struct {
	pluginID         uint
	pluginName       string
	runtime          string
	taskID           string
	commandName      string
	commandID        string
	interactive      bool
	status           string
	prompt           string
	startedAt        time.Time
	updatedAt        time.Time
	completedAt      *time.Time
	completionReason string
	lastError        string
	cancelSignal     string
	cancel           context.CancelFunc
	inputWaiter      *pluginWorkspaceInputWaiter
	inputQueue       []string
}

type pluginWorkspaceInputWaiter struct {
	response chan pluginWorkspaceInputResponse
}

type pluginWorkspaceInputResponse struct {
	value string
	err   error
}

type PluginWorkspaceStartResult struct {
	TaskID      string                  `json:"task_id,omitempty"`
	Command     PluginWorkspaceCommand  `json:"command"`
	Interactive bool                    `json:"interactive"`
	Metadata    map[string]string       `json:"metadata,omitempty"`
	Workspace   PluginWorkspaceSnapshot `json:"workspace"`
}

type PluginWorkspaceTerminalLineResult struct {
	Mode        string                  `json:"mode,omitempty"`
	TaskID      string                  `json:"task_id,omitempty"`
	Queued      bool                    `json:"queued,omitempty"`
	Interactive bool                    `json:"interactive,omitempty"`
	Success     bool                    `json:"success"`
	Error       string                  `json:"error,omitempty"`
	Data        interface{}             `json:"data,omitempty"`
	Metadata    map[string]string       `json:"metadata,omitempty"`
	Workspace   PluginWorkspaceSnapshot `json:"workspace"`
}

func pluginWorkspaceEchoCommandMessage(line string) string {
	trimmed := strings.TrimRight(line, "\r\n")
	if trimmed == "" {
		return "$"
	}
	return "$ " + trimmed
}

func pluginWorkspaceEchoInputMessage(input string) string {
	trimmed := strings.TrimRight(input, "\r\n")
	if trimmed == "" {
		return ">"
	}
	return "> " + trimmed
}

func (s *PluginManagerService) appendPluginWorkspaceEchoEntry(
	pluginID uint,
	pluginName string,
	runtime string,
	source string,
	channel string,
	message string,
	metadata map[string]string,
) {
	if s == nil || pluginID == 0 || strings.TrimSpace(message) == "" {
		return
	}
	s.ApplyPluginWorkspaceDelta(
		pluginID,
		pluginName,
		runtime,
		metadata,
		[]pluginipc.WorkspaceBufferEntry{{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			Channel:   channel,
			Level:     "info",
			Message:   message,
			Source:    source,
		}},
		false,
	)
}

func clonePluginWorkspaceStreamSubscribers(subscribers map[uint64]chan PluginWorkspaceStreamEvent) []chan PluginWorkspaceStreamEvent {
	if len(subscribers) == 0 {
		return nil
	}
	out := make([]chan PluginWorkspaceStreamEvent, 0, len(subscribers))
	for _, ch := range subscribers {
		out = append(out, ch)
	}
	return out
}

func clonePluginWorkspaceStreamSubscribersExcept(
	subscribers map[uint64]chan PluginWorkspaceStreamEvent,
	excludedID uint64,
) []chan PluginWorkspaceStreamEvent {
	if len(subscribers) == 0 {
		return nil
	}
	out := make([]chan PluginWorkspaceStreamEvent, 0, len(subscribers))
	for id, ch := range subscribers {
		if id == excludedID {
			continue
		}
		out = append(out, ch)
	}
	return out
}

func dispatchPluginWorkspaceStreamEvent(subscribers []chan PluginWorkspaceStreamEvent, event PluginWorkspaceStreamEvent) {
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

func clonePluginWorkspaceStartResult(result *PluginWorkspaceStartResult) *PluginWorkspaceStartResult {
	if result == nil {
		return nil
	}
	cloned := *result
	cloned.Metadata = cloneStringMap(result.Metadata)
	cloned.Workspace = PluginWorkspaceSnapshot{}
	if snapshot := clonePluginWorkspaceSnapshot(&result.Workspace); snapshot != nil {
		cloned.Workspace = *snapshot
	}
	return &cloned
}

func clonePluginWorkspaceTerminalLineResult(result *PluginWorkspaceTerminalLineResult) *PluginWorkspaceTerminalLineResult {
	if result == nil {
		return nil
	}
	cloned := *result
	cloned.Metadata = cloneStringMap(result.Metadata)
	cloned.Workspace = PluginWorkspaceSnapshot{}
	if snapshot := clonePluginWorkspaceSnapshot(&result.Workspace); snapshot != nil {
		cloned.Workspace = *snapshot
	}
	return &cloned
}

func (s *PluginManagerService) resolvePluginWorkspaceSessionLocked(pluginID uint, taskID string) (*pluginWorkspaceSession, *pluginWorkspaceBuffer, error) {
	if s == nil || pluginID == 0 {
		return nil, nil, fmt.Errorf("plugin workspace is unavailable")
	}
	buffer := s.workspaceBuffers[pluginID]
	if buffer == nil {
		return nil, nil, fmt.Errorf("plugin workspace is unavailable")
	}
	session := s.workspaceSessions[pluginID]
	if session == nil {
		return nil, buffer, fmt.Errorf("plugin workspace session is not running")
	}
	normalizedTaskID := strings.TrimSpace(taskID)
	if normalizedTaskID != "" && session.taskID != normalizedTaskID {
		return nil, buffer, fmt.Errorf("plugin workspace session %s is not active", normalizedTaskID)
	}
	if strings.TrimSpace(session.taskID) == "" {
		return nil, buffer, fmt.Errorf("plugin workspace session is not running")
	}
	return session, buffer, nil
}

func (s *PluginManagerService) resolvePluginWorkspaceActiveSessionLocked(
	pluginID uint,
) (*pluginWorkspaceSession, *pluginWorkspaceBuffer, error) {
	if s == nil || pluginID == 0 {
		return nil, nil, fmt.Errorf("plugin workspace is unavailable")
	}
	buffer := s.workspaceBuffers[pluginID]
	if buffer == nil {
		return nil, nil, nil
	}
	session := s.workspaceSessions[pluginID]
	if session == nil || strings.TrimSpace(session.taskID) == "" || session.completedAt != nil {
		return nil, buffer, nil
	}
	return session, buffer, nil
}

func buildPluginWorkspaceSessionMetadata(session *pluginWorkspaceSession) map[string]string {
	if session == nil {
		return nil
	}
	metadata := map[string]string{}
	if action := strings.TrimSpace(session.commandName); action != "" {
		if strings.ContainsAny(action, " \t\r\n;&|") {
			metadata["action"] = "workspace.command.shell"
			metadata["command"] = "shell"
			metadata["command_line"] = action
		} else {
			metadata["action"] = "workspace.command." + action
			metadata["command"] = action
		}
	}
	if commandID := strings.TrimSpace(session.commandID); commandID != "" {
		metadata["command_id"] = commandID
	}
	if taskID := strings.TrimSpace(session.taskID); taskID != "" {
		metadata["task_id"] = taskID
	}
	if completionReason := strings.TrimSpace(session.completionReason); completionReason != "" {
		metadata["completion_reason"] = completionReason
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func resolvePluginWorkspaceCompletionReason(status string, cancelSignal string) string {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	switch normalizedStatus {
	case PluginExecutionStatusCompleted:
		return PluginExecutionStatusCompleted
	case PluginExecutionStatusFailed:
		return PluginExecutionStatusFailed
	case PluginExecutionStatusTimedOut:
		return PluginExecutionStatusTimedOut
	case PluginExecutionStatusCanceled:
		switch strings.ToLower(strings.TrimSpace(cancelSignal)) {
		case "interrupt":
			return pluginWorkspaceCompletionReasonInterrupted
		case "terminate":
			return pluginWorkspaceCompletionReasonTerminated
		default:
			return PluginExecutionStatusCanceled
		}
	default:
		return normalizedStatus
	}
}

func (s *PluginManagerService) setPluginWorkspaceSessionStateLocked(
	buffer *pluginWorkspaceBuffer,
	session *pluginWorkspaceSession,
	status string,
	waitingInput bool,
	prompt string,
	completedAt *time.Time,
	completionReason string,
	lastError string,
	eventType string,
) ([]chan PluginWorkspaceStreamEvent, PluginWorkspaceStreamEvent) {
	if session != nil {
		session.status = strings.ToLower(strings.TrimSpace(status))
		if session.status == "" {
			session.status = pluginWorkspaceStatusIdle
		}
		session.prompt = strings.TrimSpace(prompt)
		session.updatedAt = time.Now().UTC()
		session.completedAt = cloneOptionalTime(completedAt)
		session.completionReason = strings.TrimSpace(completionReason)
		session.lastError = strings.TrimSpace(lastError)
	}
	if buffer == nil {
		return nil, PluginWorkspaceStreamEvent{}
	}
	buffer.setSessionState(
		status,
		func() string {
			if session == nil {
				return ""
			}
			return session.taskID
		}(),
		func() string {
			if session == nil {
				return ""
			}
			return session.commandName
		}(),
		func() string {
			if session == nil {
				return ""
			}
			return session.commandID
		}(),
		session != nil && session.interactive,
		waitingInput,
		prompt,
		func() time.Time {
			if session == nil {
				return time.Time{}
			}
			return session.startedAt
		}(),
		completedAt,
		completionReason,
		lastError,
	)
	return clonePluginWorkspaceStreamSubscribers(buffer.subscribers), buffer.snapshotEvent(eventType)
}

func (s *PluginManagerService) startPluginWorkspaceCommandSession(
	plugin *models.Plugin,
	command *PluginWorkspaceCommand,
	taskID string,
	adminID uint,
	startedAt time.Time,
) error {
	if s == nil || plugin == nil || command == nil {
		return fmt.Errorf("plugin workspace session is unavailable")
	}
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("plugin workspace task id is required")
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, strings.ToLower(strings.TrimSpace(plugin.Runtime)))
	if err := buffer.ensureControl(adminID); err != nil {
		s.workspaceMu.Unlock()
		return err
	}
	if existing := s.workspaceSessions[plugin.ID]; existing != nil && strings.TrimSpace(existing.taskID) != "" && existing.completedAt == nil {
		s.workspaceMu.Unlock()
		return fmt.Errorf("workspace command %s is already running", existing.commandName)
	}

	session := &pluginWorkspaceSession{
		pluginID:    plugin.ID,
		pluginName:  strings.TrimSpace(plugin.Name),
		runtime:     strings.ToLower(strings.TrimSpace(plugin.Runtime)),
		taskID:      strings.TrimSpace(taskID),
		commandName: strings.TrimSpace(command.Name),
		commandID:   strings.TrimSpace(taskID),
		interactive: command.Interactive,
		status:      pluginWorkspaceStatusRunning,
		startedAt:   startedAt.UTC(),
		updatedAt:   startedAt.UTC(),
	}
	s.workspaceSessions[plugin.ID] = session
	subscribers, event := s.setPluginWorkspaceSessionStateLocked(
		buffer,
		session,
		pluginWorkspaceStatusRunning,
		false,
		"",
		nil,
		"",
		"",
		"state",
	)
	s.workspaceMu.Unlock()
	dispatchPluginWorkspaceStreamEvent(subscribers, event)
	return nil
}

func (s *PluginManagerService) startPluginWorkspaceShellSession(
	plugin *models.Plugin,
	commandLine string,
	taskID string,
	adminID uint,
	startedAt time.Time,
	cancel context.CancelFunc,
) error {
	if s == nil || plugin == nil {
		return fmt.Errorf("plugin workspace session is unavailable")
	}
	if strings.TrimSpace(taskID) == "" {
		return fmt.Errorf("plugin workspace task id is required")
	}
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, strings.ToLower(strings.TrimSpace(plugin.Runtime)))
	if err := buffer.ensureControl(adminID); err != nil {
		s.workspaceMu.Unlock()
		return err
	}
	if existing := s.workspaceSessions[plugin.ID]; existing != nil && strings.TrimSpace(existing.taskID) != "" && existing.completedAt == nil {
		s.workspaceMu.Unlock()
		return fmt.Errorf("workspace command %s is already running", existing.commandName)
	}

	session := &pluginWorkspaceSession{
		pluginID:    plugin.ID,
		pluginName:  strings.TrimSpace(plugin.Name),
		runtime:     strings.ToLower(strings.TrimSpace(plugin.Runtime)),
		taskID:      strings.TrimSpace(taskID),
		commandName: strings.TrimSpace(commandLine),
		commandID:   strings.TrimSpace(taskID),
		interactive: false,
		status:      pluginWorkspaceStatusRunning,
		startedAt:   startedAt.UTC(),
		updatedAt:   startedAt.UTC(),
		cancel:      cancel,
	}
	s.workspaceSessions[plugin.ID] = session
	subscribers, event := s.setPluginWorkspaceSessionStateLocked(
		buffer,
		session,
		pluginWorkspaceStatusRunning,
		false,
		"",
		nil,
		"",
		"",
		"state",
	)
	s.workspaceMu.Unlock()
	dispatchPluginWorkspaceStreamEvent(subscribers, event)
	return nil
}

func pluginWorkspaceCommandTimeout(command *PluginWorkspaceCommand) time.Duration {
	if command == nil || !command.Interactive {
		return defaultPluginWorkspaceCommandTimeout
	}
	return defaultPluginWorkspaceInteractiveTimeout
}

func clonePluginExecutionContextForWorkspace(execCtx *ExecutionContext) *ExecutionContext {
	cloned := clonePluginExecutionContext(execCtx)
	if cloned == nil {
		cloned = &ExecutionContext{}
	}
	cloned.RequestContext = context.Background()
	if cloned.Metadata == nil {
		cloned.Metadata = make(map[string]string)
	}
	return cloned
}

func (s *PluginManagerService) StartPluginWorkspaceCommand(
	pluginID uint,
	adminID uint,
	commandName string,
	argv []string,
	inputLines []string,
	execCtx *ExecutionContext,
) (*PluginWorkspaceStartResult, error) {
	plugin, runtime, capabilityPolicy, err, _ := s.getPluginByIDWithCatalog(pluginID)
	if err != nil {
		return nil, err
	}
	if runtime != PluginRuntimeJSWorker {
		return nil, fmt.Errorf("workspace commands are only available for js_worker plugins")
	}

	command, err := s.ResolvePluginWorkspaceCommandForPlugin(plugin, commandName)
	if err != nil {
		return nil, err
	}
	if len(command.MissingPermissions) > 0 {
		return nil, fmt.Errorf(
			"workspace command %s is missing required permissions: %s",
			command.Name,
			strings.Join(command.MissingPermissions, ", "),
		)
	}

	preparedExecCtx := clonePluginExecutionContextForWorkspace(execCtx)
	taskID := EnsurePluginExecutionMetadata(preparedExecCtx, true)
	params, err := buildPluginWorkspaceCommandParams(command, argv, inputLines, taskID)
	if err != nil {
		return nil, err
	}
	preparedExecCtx.Metadata["workspace_command"] = strings.TrimSpace(command.Name)
	preparedExecCtx.Metadata["workspace_command_entry"] = strings.TrimSpace(command.Entry)
	preparedExecCtx.Metadata["workspace_command_interactive"] = fmt.Sprintf("%t", command.Interactive)
	startedAt := parsePluginExecutionStartedAt(preparedExecCtx.Metadata)
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	if err := s.startPluginWorkspaceCommandSession(plugin, command, taskID, adminID, startedAt); err != nil {
		return nil, err
	}

	go func(
		executionPlugin *models.Plugin,
		executionRuntime string,
		executionCapabilityPolicy pluginCapabilityPolicy,
		commandParams map[string]string,
		commandExecCtx *ExecutionContext,
		commandSpec *PluginWorkspaceCommand,
		commandTaskID string,
	) {
		result, execErr := s.executePluginResolvedStreamWithTimeout(
			executionPlugin,
			executionRuntime,
			executionCapabilityPolicy,
			pluginWorkspaceCommandExecuteAction,
			commandParams,
			commandExecCtx,
			pluginWorkspaceCommandTimeout(commandSpec),
			nil,
		)
		s.finishPluginWorkspaceCommandSession(executionPlugin.ID, commandTaskID, result, execErr)
	}(plugin, runtime, capabilityPolicy, cloneStringMap(params), preparedExecCtx, command, taskID)

	_, _ = s.NotePluginWorkspaceControlActivity(
		plugin,
		adminID,
		"command_started",
		fmt.Sprintf("Workspace command %s started by admin #%d.", strings.TrimSpace(command.Name), adminID),
	)

	snapshot := s.GetPluginWorkspaceSnapshot(plugin, defaultPluginWorkspaceSnapshotLimit)
	return &PluginWorkspaceStartResult{
		TaskID:      taskID,
		Command:     *command,
		Interactive: command.Interactive,
		Metadata:    cloneStringMap(preparedExecCtx.Metadata),
		Workspace:   snapshot,
	}, nil
}

func (s *PluginManagerService) EnterPluginWorkspaceTerminalLine(
	pluginID uint,
	adminID uint,
	line string,
	execCtx *ExecutionContext,
) (*PluginWorkspaceTerminalLineResult, error) {
	plugin, runtime, _, err, _ := s.getPluginByIDWithCatalog(pluginID)
	if err != nil {
		return nil, err
	}
	if runtime != PluginRuntimeJSWorker {
		return nil, fmt.Errorf("workspace commands are only available for js_worker plugins")
	}

	normalizedLine := strings.TrimRight(line, "\r\n")

	var (
		hasActiveSession  bool
		activeWorkspace   PluginWorkspaceSnapshot
		activeCommandName string
		activeTaskID      string
		activeInteractive bool
		idleWorkspace     PluginWorkspaceSnapshot
	)

	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, runtime)
	if controlErr := buffer.ensureControl(adminID); controlErr != nil {
		idleWorkspace = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return nil, controlErr
	}
	session, _, resolveErr := s.resolvePluginWorkspaceActiveSessionLocked(plugin.ID)
	if resolveErr != nil {
		idleWorkspace = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return nil, resolveErr
	}
	if session != nil {
		hasActiveSession = true
		activeWorkspace = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		activeCommandName = strings.TrimSpace(session.commandName)
		activeTaskID = strings.TrimSpace(session.taskID)
		activeInteractive = session.interactive
	} else {
		idleWorkspace = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
	}
	s.workspaceMu.Unlock()

	if hasActiveSession {
		if !activeInteractive {
			commandLabel := activeCommandName
			if commandLabel == "" {
				commandLabel = activeTaskID
			}
			return nil, fmt.Errorf("workspace command %s is still running", commandLabel)
		}
		submittedSnapshot, submitErr := s.SubmitPluginWorkspaceInput(pluginID, adminID, activeTaskID, normalizedLine)
		if submitErr != nil {
			return nil, submitErr
		}
		return &PluginWorkspaceTerminalLineResult{
			Mode:        "input",
			TaskID:      activeTaskID,
			Queued:      !activeWorkspace.WaitingInput,
			Interactive: true,
			Success:     true,
			Workspace:   submittedSnapshot,
		}, nil
	}

	if strings.TrimSpace(normalizedLine) == "" {
		return &PluginWorkspaceTerminalLineResult{
			Mode:      "noop",
			Success:   true,
			Workspace: idleWorkspace,
		}, nil
	}

	preparedExecCtx := clonePluginExecutionContext(execCtx)
	if preparedExecCtx == nil {
		preparedExecCtx = &ExecutionContext{}
	}
	if preparedExecCtx.Metadata == nil {
		preparedExecCtx.Metadata = make(map[string]string, 1)
	}
	preparedExecCtx.Metadata["workspace_terminal_line"] = normalizedLine

	workspaceForShell := s.GetPluginWorkspaceSnapshot(plugin, 0)
	shellVariables := BuildPluginWorkspaceShellVariables(plugin, adminID, &workspaceForShell, preparedExecCtx)
	shellProgram, resolveErr := s.ResolvePluginWorkspaceShellProgramWithVariablesForPlugin(
		plugin,
		normalizedLine,
		shellVariables,
	)
	if resolveErr != nil {
		return nil, resolveErr
	}
	if len(shellProgram) == 0 {
		return nil, fmt.Errorf("workspace command line is empty")
	}

	pipelineMode := false
	sequenceMode := len(shellProgram) > 1
	for _, statement := range shellProgram {
		if len(statement.Stages) > 1 {
			pipelineMode = true
		}
		for _, stage := range statement.Stages {
			if stage.Command == nil || !stage.Command.Interactive {
				continue
			}
			if len(statement.Stages) > 1 {
				return nil, fmt.Errorf("interactive workspace command %s cannot be used in a pipeline", stage.Command.Name)
			}
			if len(shellProgram) > 1 {
				return nil, fmt.Errorf(
					"interactive workspace command %s cannot be used in a chained sequence",
					stage.Command.Name,
				)
			}
		}
	}

	if !pipelineMode && !sequenceMode && len(shellProgram[0].Stages) == 1 {
		stage := shellProgram[0].Stages[0]
		if stage.Command != nil && stage.Command.Interactive {
			started, startErr := s.StartPluginWorkspaceCommand(
				pluginID,
				adminID,
				stage.Command.Name,
				stage.Argv,
				nil,
				preparedExecCtx,
			)
			if startErr != nil {
				return nil, startErr
			}
			s.appendPluginWorkspaceEchoEntry(
				pluginID,
				plugin.Name,
				runtime,
				"host.workspace.command",
				"command",
				pluginWorkspaceEchoCommandMessage(normalizedLine),
				map[string]string{
					"action":     "workspace.command." + strings.TrimSpace(stage.Command.Name),
					"command":    strings.TrimSpace(stage.Command.Name),
					"command_id": strings.TrimSpace(started.TaskID),
					"task_id":    strings.TrimSpace(started.TaskID),
				},
			)
			started.Workspace = s.GetPluginWorkspaceSnapshot(plugin, defaultPluginWorkspaceSnapshotLimit)
			return &PluginWorkspaceTerminalLineResult{
				Mode:        "command_started",
				TaskID:      strings.TrimSpace(started.TaskID),
				Interactive: true,
				Success:     true,
				Metadata:    cloneStringMap(started.Metadata),
				Workspace:   started.Workspace,
			}, nil
		}
	}

	preparedExecCtx = clonePluginExecutionContextForWorkspace(preparedExecCtx)
	taskID := EnsurePluginExecutionMetadata(preparedExecCtx, false)
	startedAt := parsePluginExecutionStartedAt(preparedExecCtx.Metadata)
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	sessionCtx, sessionCancel := context.WithCancel(executionRequestContext(preparedExecCtx))
	preparedExecCtx.RequestContext = sessionCtx
	preparedExecCtx.Metadata["workspace_command_line"] = normalizedLine
	preparedExecCtx.Metadata["workspace_command"] = "shell"
	preparedExecCtx.Metadata["workspace_command_interactive"] = "false"
	if startErr := s.startPluginWorkspaceShellSession(
		plugin,
		normalizedLine,
		taskID,
		adminID,
		startedAt,
		sessionCancel,
	); startErr != nil {
		sessionCancel()
		return nil, startErr
	}
	s.appendPluginWorkspaceEchoEntry(
		pluginID,
		plugin.Name,
		runtime,
		"host.workspace.command",
		"command",
		pluginWorkspaceEchoCommandMessage(normalizedLine),
		map[string]string{
			"action":       "workspace.command.shell",
			"command":      "shell",
			"command_line": normalizedLine,
			"command_id":   strings.TrimSpace(taskID),
			"task_id":      strings.TrimSpace(taskID),
		},
	)

	go func(shellPluginID uint, shellTaskID string, shellCommandLine string, shellExecCtx *ExecutionContext) {
		defer sessionCancel()
		result, execErr := s.ExecutePluginWorkspaceShellCommand(
			shellPluginID,
			adminID,
			shellCommandLine,
			nil,
			shellExecCtx,
		)
		s.finishPluginWorkspaceCommandSession(shellPluginID, shellTaskID, result, execErr)
	}(pluginID, taskID, normalizedLine, preparedExecCtx)

	postSnapshot := s.GetPluginWorkspaceSnapshot(plugin, defaultPluginWorkspaceSnapshotLimit)
	if notedSnapshot, noteErr := s.NotePluginWorkspaceControlActivity(
		plugin,
		adminID,
		"command_started",
		fmt.Sprintf("Workspace shell line started by admin #%d.", adminID),
	); noteErr == nil {
		postSnapshot = notedSnapshot
	}
	return &PluginWorkspaceTerminalLineResult{
		Mode:        "command_started",
		TaskID:      strings.TrimSpace(taskID),
		Interactive: false,
		Success:     true,
		Metadata:    cloneStringMap(preparedExecCtx.Metadata),
		Workspace:   postSnapshot,
	}, nil
}

func (s *PluginManagerService) finishPluginWorkspaceCommandSession(
	pluginID uint,
	taskID string,
	result *ExecutionResult,
	execErr error,
) {
	if s == nil || pluginID == 0 || strings.TrimSpace(taskID) == "" {
		return
	}
	finishedAt := time.Now().UTC()
	status := resolvePluginExecutionStatus(result, execErr)
	lastError := ""
	if execErr != nil {
		lastError = strings.TrimSpace(execErr.Error())
	} else if result != nil {
		lastError = strings.TrimSpace(result.Error)
	}

	var waiter *pluginWorkspaceInputWaiter
	var subscribers []chan PluginWorkspaceStreamEvent
	var event PluginWorkspaceStreamEvent
	var pluginName string
	var runtime string
	var commandName string
	var finishSignal string
	var completionReason string
	var cancel context.CancelFunc
	s.workspaceMu.Lock()
	session, buffer, err := s.resolvePluginWorkspaceSessionLocked(pluginID, taskID)
	if err == nil && session != nil && session.inputWaiter != nil {
		waiter = session.inputWaiter
		session.inputWaiter = nil
	}
	if err == nil && session != nil {
		completionReason = resolvePluginWorkspaceCompletionReason(status, session.cancelSignal)
		pluginName = session.pluginName
		runtime = session.runtime
		commandName = strings.TrimSpace(session.commandName)
		finishSignal = strings.ToLower(strings.TrimSpace(session.cancelSignal))
		session.inputQueue = nil
		cancel = session.cancel
		session.cancel = nil
		subscribers, event = s.setPluginWorkspaceSessionStateLocked(
			buffer,
			session,
			status,
			false,
			"",
			&finishedAt,
			completionReason,
			lastError,
			"state",
		)
		if buffer != nil && buffer.reconcileOwner() {
			event = buffer.snapshotEvent("state")
		}
		session.cancelSignal = ""
	}
	s.workspaceMu.Unlock()

	if waiter != nil {
		select {
		case waiter.response <- pluginWorkspaceInputResponse{err: fmt.Errorf("workspace command exited")}:
		default:
		}
	}
	if cancel != nil {
		cancel()
	}
	dispatchPluginWorkspaceStreamEvent(subscribers, event)

	if strings.TrimSpace(commandName) != "" {
		completionResult := strings.TrimSpace(completionReason)
		if completionResult == "" {
			completionResult = strings.ToLower(strings.TrimSpace(status))
		}
		message := fmt.Sprintf(
			"Workspace command %s finished with %s.",
			commandName,
			completionResult,
		)
		_, _ = s.notePluginWorkspaceControlActivityDetailed(
			&models.Plugin{ID: pluginID, Name: pluginName, Runtime: runtime},
			0,
			"command_finished",
			finishSignal,
			completionResult,
			message,
		)
	}
}

func (s *PluginManagerService) WaitPluginWorkspaceInput(
	pluginID uint,
	taskID string,
	prompt string,
	timeout time.Duration,
) (string, error) {
	if s == nil || pluginID == 0 {
		return "", fmt.Errorf("plugin workspace is unavailable")
	}
	if timeout <= 0 {
		timeout = defaultPluginWorkspaceInputWaitTimeout
	}
	waiter := &pluginWorkspaceInputWaiter{
		response: make(chan pluginWorkspaceInputResponse, 1),
	}

	var subscribers []chan PluginWorkspaceStreamEvent
	var event PluginWorkspaceStreamEvent
	s.workspaceMu.Lock()
	session, buffer, err := s.resolvePluginWorkspaceSessionLocked(pluginID, taskID)
	if err != nil {
		s.workspaceMu.Unlock()
		return "", err
	}
	if session.completedAt != nil {
		s.workspaceMu.Unlock()
		return "", fmt.Errorf("plugin workspace session is already completed")
	}
	if len(session.inputQueue) > 0 {
		value := session.inputQueue[0]
		if len(session.inputQueue) == 1 {
			session.inputQueue = nil
		} else {
			session.inputQueue = append([]string(nil), session.inputQueue[1:]...)
		}
		session.updatedAt = time.Now().UTC()
		s.workspaceMu.Unlock()
		return value, nil
	}
	if session.inputWaiter != nil {
		s.workspaceMu.Unlock()
		return "", fmt.Errorf("plugin workspace is already waiting for input")
	}
	session.inputWaiter = waiter
	subscribers, event = s.setPluginWorkspaceSessionStateLocked(
		buffer,
		session,
		pluginWorkspaceStatusWaitingInput,
		true,
		prompt,
		nil,
		"",
		session.lastError,
		"state",
	)
	s.workspaceMu.Unlock()
	dispatchPluginWorkspaceStreamEvent(subscribers, event)

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var response pluginWorkspaceInputResponse
	select {
	case response = <-waiter.response:
	case <-timer.C:
		response.err = fmt.Errorf("workspace input timed out after %s", timeout.Round(time.Second))
	}

	s.workspaceMu.Lock()
	session, buffer, err = s.resolvePluginWorkspaceSessionLocked(pluginID, taskID)
	if err == nil && session != nil && session.inputWaiter == waiter {
		session.inputWaiter = nil
		nextStatus := session.status
		completedAt := session.completedAt
		if completedAt == nil {
			nextStatus = pluginWorkspaceStatusRunning
		}
		subscribers, event = s.setPluginWorkspaceSessionStateLocked(
			buffer,
			session,
			nextStatus,
			false,
			"",
			completedAt,
			session.completionReason,
			session.lastError,
			"state",
		)
	} else {
		subscribers = nil
		event = PluginWorkspaceStreamEvent{}
	}
	s.workspaceMu.Unlock()
	dispatchPluginWorkspaceStreamEvent(subscribers, event)

	if response.err != nil {
		return "", response.err
	}
	return response.value, nil
}

func (s *PluginManagerService) SubmitPluginWorkspaceInput(
	pluginID uint,
	adminID uint,
	taskID string,
	input string,
) (PluginWorkspaceSnapshot, error) {
	if s == nil || pluginID == 0 {
		return PluginWorkspaceSnapshot{}, fmt.Errorf("plugin workspace is unavailable")
	}
	normalizedInput := strings.TrimRight(input, "\r\n")

	var waiter *pluginWorkspaceInputWaiter
	var snapshot PluginWorkspaceSnapshot
	var subscribers []chan PluginWorkspaceStreamEvent
	var event PluginWorkspaceStreamEvent
	var echoMetadata map[string]string
	var pluginName string
	var runtime string
	s.workspaceMu.Lock()
	session, buffer, err := s.resolvePluginWorkspaceSessionLocked(pluginID, taskID)
	if err != nil {
		s.workspaceMu.Unlock()
		return snapshot, err
	}
	if err := buffer.ensureControl(adminID); err != nil {
		snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return snapshot, err
	}
	if session.completedAt != nil {
		snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return snapshot, fmt.Errorf("plugin workspace session is already completed")
	}
	if session.inputWaiter == nil {
		if !session.interactive {
			snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
			s.workspaceMu.Unlock()
			return snapshot, fmt.Errorf("plugin workspace command is not accepting input")
		}
		session.inputQueue = append(session.inputQueue, normalizedInput)
		session.updatedAt = time.Now().UTC()
		buffer.touchOwnerActivity(adminID)
		echoMetadata = mergePluginWorkspaceMetadata(buildPluginWorkspaceSessionMetadata(session), map[string]string{
			"input_kind": "stdin",
		})
		pluginName = buffer.pluginName
		runtime = buffer.runtime
		snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		s.appendPluginWorkspaceEchoEntry(
			pluginID,
			pluginName,
			runtime,
			"host.workspace.stdin",
			"stdin",
			pluginWorkspaceEchoInputMessage(normalizedInput),
			echoMetadata,
		)
		if updatedSnapshot, noteErr := s.NotePluginWorkspaceControlActivity(
			&models.Plugin{ID: pluginID, Name: pluginName, Runtime: runtime},
			adminID,
			"input_submitted",
			fmt.Sprintf("Workspace input submitted by admin #%d.", adminID),
		); noteErr == nil {
			snapshot = updatedSnapshot
		}
		return snapshot, nil
	}
	waiter = session.inputWaiter
	session.inputWaiter = nil
	echoMetadata = mergePluginWorkspaceMetadata(buildPluginWorkspaceSessionMetadata(session), map[string]string{
		"input_kind": "stdin",
	})
	pluginName = buffer.pluginName
	runtime = buffer.runtime
	subscribers, event = s.setPluginWorkspaceSessionStateLocked(
		buffer,
		session,
		pluginWorkspaceStatusRunning,
		false,
		"",
		nil,
		"",
		session.lastError,
		"state",
	)
	snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
	s.workspaceMu.Unlock()

	select {
	case waiter.response <- pluginWorkspaceInputResponse{value: normalizedInput}:
	default:
	}
	dispatchPluginWorkspaceStreamEvent(subscribers, event)
	s.appendPluginWorkspaceEchoEntry(
		pluginID,
		pluginName,
		runtime,
		"host.workspace.stdin",
		"stdin",
		pluginWorkspaceEchoInputMessage(normalizedInput),
		echoMetadata,
	)
	if updatedSnapshot, noteErr := s.NotePluginWorkspaceControlActivity(
		&models.Plugin{ID: pluginID, Name: pluginName, Runtime: runtime},
		adminID,
		"input_submitted",
		fmt.Sprintf("Workspace input submitted by admin #%d.", adminID),
	); noteErr == nil {
		snapshot = updatedSnapshot
	}
	return snapshot, nil
}

func (s *PluginManagerService) SignalPluginWorkspace(
	pluginID uint,
	adminID uint,
	taskID string,
	signal string,
) (PluginWorkspaceSnapshot, error) {
	if s == nil || pluginID == 0 {
		return PluginWorkspaceSnapshot{}, fmt.Errorf("plugin workspace is unavailable")
	}
	normalizedSignal := strings.ToLower(strings.TrimSpace(signal))
	if normalizedSignal == "" {
		normalizedSignal = "interrupt"
	}
	if normalizedSignal != "interrupt" && normalizedSignal != "terminate" {
		return PluginWorkspaceSnapshot{}, fmt.Errorf("unsupported workspace signal %q", signal)
	}

	var (
		session        *pluginWorkspaceSession
		waiter         *pluginWorkspaceInputWaiter
		snapshot       PluginWorkspaceSnapshot
		subscribers    []chan PluginWorkspaceStreamEvent
		event          PluginWorkspaceStreamEvent
		signalResult   string
		bufferMessage  string
		waiterErrText  string
		cancelRequired bool
		sessionCancel  context.CancelFunc
	)
	s.workspaceMu.Lock()
	resolvedSession, buffer, err := s.resolvePluginWorkspaceSessionLocked(pluginID, taskID)
	if err != nil {
		s.workspaceMu.Unlock()
		return snapshot, err
	}
	if err := buffer.ensureControl(adminID); err != nil {
		snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return snapshot, err
	}
	session = resolvedSession
	if session.inputWaiter != nil {
		waiter = session.inputWaiter
		session.inputWaiter = nil
		subscribers, event = s.setPluginWorkspaceSessionStateLocked(
			buffer,
			session,
			pluginWorkspaceStatusRunning,
			false,
			"",
			nil,
			"",
			session.lastError,
			"state",
		)
	}
	switch {
	case normalizedSignal == "interrupt" && waiter != nil:
		signalResult = "input_wait_interrupted"
		waiterErrText = "workspace input interrupted"
		bufferMessage = "Workspace input wait interrupted by admin; the command remains active."
	case normalizedSignal == "interrupt":
		signalResult = "cancel_requested"
		waiterErrText = "workspace interrupted"
		bufferMessage = "Workspace interrupt requested by admin; host cancellation requested for the active command."
		session.cancelSignal = normalizedSignal
		sessionCancel = session.cancel
		cancelRequired = true
	case normalizedSignal == "terminate":
		signalResult = "cancel_requested"
		waiterErrText = "workspace command terminated"
		bufferMessage = "Workspace terminate requested by admin; host cancellation requested for the active command."
		session.cancelSignal = normalizedSignal
		sessionCancel = session.cancel
		cancelRequired = true
	}
	snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
	s.workspaceMu.Unlock()

	if waiter != nil {
		select {
		case waiter.response <- pluginWorkspaceInputResponse{err: fmt.Errorf("%s", waiterErrText)}:
		default:
		}
	}
	dispatchPluginWorkspaceStreamEvent(subscribers, event)

	if session != nil && cancelRequired {
		if sessionCancel != nil {
			sessionCancel()
		}
		if _, cancelErr := s.CancelPluginExecutionTask(pluginID, session.taskID); cancelErr != nil &&
			!strings.Contains(strings.ToLower(cancelErr.Error()), "is not running") &&
			!(sessionCancel != nil && strings.Contains(strings.ToLower(cancelErr.Error()), "not found")) {
			return snapshot, cancelErr
		} else if cancelErr != nil {
			cancelErrText := strings.ToLower(cancelErr.Error())
			if !(sessionCancel != nil && strings.Contains(cancelErrText, "not found")) {
				signalResult = "already_stopped"
				if normalizedSignal == "terminate" {
					bufferMessage = "Workspace terminate requested by admin, but the active command was already stopped."
				} else {
					bufferMessage = "Workspace interrupt requested by admin, but the active command was already stopped."
				}
				s.workspaceMu.Lock()
				if current, _, resolveErr := s.resolvePluginWorkspaceSessionLocked(pluginID, session.taskID); resolveErr == nil && current != nil {
					current.cancelSignal = ""
				}
				s.workspaceMu.Unlock()
			}
		}
	}

	baseMetadata := buildPluginWorkspaceSessionMetadata(session)
	if session != nil && strings.TrimSpace(bufferMessage) != "" {
		entryMetadata := mergePluginWorkspaceMetadata(baseMetadata, map[string]string{
			"signal":        normalizedSignal,
			"signal_result": signalResult,
		})
		s.ApplyPluginWorkspaceDelta(
			pluginID,
			session.pluginName,
			session.runtime,
			entryMetadata,
			[]pluginipc.WorkspaceBufferEntry{{
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				Channel:   "system",
				Level:     "warn",
				Message:   bufferMessage,
				Source:    "host.workspace.signal",
			}},
			false,
		)
	}

	if session != nil && s.db != nil {
		var plugin models.Plugin
		if err := s.db.First(&plugin, pluginID).Error; err == nil {
			snapshot = s.GetPluginWorkspaceSnapshot(&plugin, defaultPluginWorkspaceSnapshotLimit)
		}
	}
	if session != nil {
		if updatedSnapshot, noteErr := s.notePluginWorkspaceControlActivityDetailed(
			&models.Plugin{ID: pluginID, Name: session.pluginName, Runtime: session.runtime},
			adminID,
			"signal_sent",
			normalizedSignal,
			signalResult,
			fmt.Sprintf("Workspace signal %s sent by admin #%d.", normalizedSignal, adminID),
		); noteErr == nil {
			snapshot = updatedSnapshot
		}
	}
	return snapshot, nil
}

func (s *PluginManagerService) ResetPluginWorkspace(
	plugin *models.Plugin,
	adminID uint,
) (PluginWorkspaceSnapshot, error) {
	if s == nil || plugin == nil {
		return PluginWorkspaceSnapshot{}, fmt.Errorf("plugin workspace is unavailable")
	}
	runtime := strings.ToLower(strings.TrimSpace(plugin.Runtime))
	if runtime != PluginRuntimeJSWorker {
		return PluginWorkspaceSnapshot{}, fmt.Errorf("workspace is only available for js_worker plugins")
	}

	var (
		waiter          *pluginWorkspaceInputWaiter
		snapshot        PluginWorkspaceSnapshot
		subscribers     []chan PluginWorkspaceStreamEvent
		event           PluginWorkspaceStreamEvent
		activeTaskID    string
		previousCommand string
		resetEntry      PluginWorkspaceBufferEntry
		sessionCancel   context.CancelFunc
	)

	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, runtime)
	if err := buffer.ensureControl(adminID); err != nil {
		snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
		s.workspaceMu.Unlock()
		return snapshot, err
	}
	buffer.touchOwnerActivity(adminID)
	if session := s.workspaceSessions[plugin.ID]; session != nil {
		if session.inputWaiter != nil {
			waiter = session.inputWaiter
			session.inputWaiter = nil
		}
		session.inputQueue = nil
		sessionCancel = session.cancel
		session.cancel = nil
		activeTaskID = strings.TrimSpace(session.taskID)
		previousCommand = strings.TrimSpace(session.commandName)
		delete(s.workspaceSessions, plugin.ID)
	}
	buffer.recordControlEvent(
		"workspace_reset",
		adminID,
		buffer.ownerAdminID,
		buffer.ownerAdminID,
		fmt.Sprintf("Workspace reset by admin #%d.", adminID),
	)
	buffer.setSessionState(
		pluginWorkspaceStatusIdle,
		"",
		"",
		"",
		false,
		false,
		"",
		time.Time{},
		nil,
		"",
		"",
	)
	resetMessage := fmt.Sprintf("Workspace reset by admin #%d.", adminID)
	if previousCommand != "" {
		resetMessage = fmt.Sprintf(
			"Workspace reset by admin #%d. Previous command %s was discarded.",
			adminID,
			previousCommand,
		)
	}
	resetMetadata := map[string]string{
		"action": "workspace.reset",
	}
	if activeTaskID != "" {
		resetMetadata["task_id"] = activeTaskID
	}
	if previousCommand != "" {
		resetMetadata["command"] = previousCommand
	}
	resetEntry = PluginWorkspaceBufferEntry{
		Timestamp: time.Now().UTC(),
		Channel:   "system",
		Level:     "warn",
		Message:   resetMessage,
		Source:    "host.workspace.reset",
		Metadata:  resetMetadata,
	}
	buffer.apply([]PluginWorkspaceBufferEntry{resetEntry}, true)
	snapshot = buffer.snapshot(defaultPluginWorkspaceSnapshotLimit)
	subscribers = clonePluginWorkspaceStreamSubscribers(buffer.subscribers)
	event = buffer.snapshotEvent("state")
	s.workspaceMu.Unlock()

	if waiter != nil {
		select {
		case waiter.response <- pluginWorkspaceInputResponse{err: fmt.Errorf("workspace reset by admin")}:
		default:
		}
	}
	if sessionCancel != nil {
		sessionCancel()
	}
	if activeTaskID != "" {
		if _, cancelErr := s.CancelPluginExecutionTask(plugin.ID, activeTaskID); cancelErr != nil {
			cancelErrText := strings.ToLower(strings.TrimSpace(cancelErr.Error()))
			if !strings.Contains(cancelErrText, "is not running") && !strings.Contains(cancelErrText, "not found") {
				return snapshot, cancelErr
			}
		}
	}
	dispatchPluginWorkspaceStreamEvent(subscribers, event)
	return snapshot, nil
}

func (s *PluginManagerService) AppendPluginWorkspaceSessionEntries(
	pluginID uint,
	taskID string,
	entries []pluginipc.WorkspaceBufferEntry,
	cleared bool,
) error {
	if s == nil || pluginID == 0 {
		return fmt.Errorf("plugin workspace is unavailable")
	}
	if !cleared && len(entries) == 0 {
		return nil
	}
	s.workspaceMu.RLock()
	buffer := s.workspaceBuffers[pluginID]
	if buffer == nil {
		s.workspaceMu.RUnlock()
		return fmt.Errorf("plugin workspace is unavailable")
	}
	session := s.workspaceSessions[pluginID]
	pluginName := buffer.pluginName
	runtime := buffer.runtime
	baseMetadata := map[string]string{}
	if session != nil && strings.TrimSpace(session.taskID) != "" {
		if normalizedTaskID := strings.TrimSpace(taskID); normalizedTaskID != "" && session.taskID != normalizedTaskID {
			s.workspaceMu.RUnlock()
			return fmt.Errorf("plugin workspace session %s is not active", normalizedTaskID)
		}
		baseMetadata = buildPluginWorkspaceSessionMetadata(session)
	}
	s.workspaceMu.RUnlock()

	s.ApplyPluginWorkspaceDelta(pluginID, pluginName, runtime, baseMetadata, entries, cleared)
	return nil
}
