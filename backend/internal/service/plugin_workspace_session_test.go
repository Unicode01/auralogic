package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"auralogic/internal/config"
	"auralogic/internal/models"
)

func waitForWorkspaceCondition(t *testing.T, plugin *models.Plugin, service *PluginManagerService, fn func(PluginWorkspaceSnapshot) bool) PluginWorkspaceSnapshot {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		snapshot := service.GetPluginWorkspaceSnapshot(plugin, 32)
		if fn(snapshot) {
			return snapshot
		}
		if time.Now().After(deadline) {
			t.Fatalf("workspace condition was not met before timeout, last snapshot=%+v", snapshot)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPluginWorkspaceSessionWaitAndSubmitInput(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 7, Name: "demo", Runtime: PluginRuntimeJSWorker}
	command := &PluginWorkspaceCommand{Name: "debugger/prompt", Interactive: true}
	if err := service.startPluginWorkspaceCommandSession(plugin, command, "task_1", 1, time.Now().UTC()); err != nil {
		t.Fatalf("startPluginWorkspaceCommandSession returned error: %v", err)
	}

	type result struct {
		value string
		err   error
	}
	waitResult := make(chan result, 1)
	go func() {
		value, err := service.WaitPluginWorkspaceInput(plugin.ID, "task_1", "debugger> ", time.Second)
		waitResult <- result{value: value, err: err}
	}()

	waitSnapshot := waitForWorkspaceCondition(t, plugin, service, func(snapshot PluginWorkspaceSnapshot) bool {
		return snapshot.WaitingInput
	})
	if waitSnapshot.Status != pluginWorkspaceStatusWaitingInput {
		t.Fatalf("expected waiting_input status, got %+v", waitSnapshot)
	}
	if waitSnapshot.Prompt != "debugger>" {
		t.Fatalf("expected prompt to be retained, got %+v", waitSnapshot)
	}

	submitSnapshot, err := service.SubmitPluginWorkspaceInput(plugin.ID, 1, "task_1", "hello workspace")
	if err != nil {
		t.Fatalf("SubmitPluginWorkspaceInput returned error: %v", err)
	}
	if submitSnapshot.WaitingInput {
		t.Fatalf("expected snapshot to leave waiting_input state, got %+v", submitSnapshot)
	}

	resultValue := <-waitResult
	if resultValue.err != nil {
		t.Fatalf("expected input wait to succeed, got %v", resultValue.err)
	}
	if resultValue.value != "hello workspace" {
		t.Fatalf("expected input value to round-trip, got %q", resultValue.value)
	}

	runningSnapshot := waitForWorkspaceCondition(t, plugin, service, func(snapshot PluginWorkspaceSnapshot) bool {
		return !snapshot.WaitingInput && snapshot.Status == pluginWorkspaceStatusRunning
	})
	if runningSnapshot.ActiveTaskID != "task_1" {
		t.Fatalf("expected active task to be retained, got %+v", runningSnapshot)
	}
}

func TestPluginWorkspaceSessionInterruptWaitingInput(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 9, Name: "demo", Runtime: PluginRuntimeJSWorker}
	command := &PluginWorkspaceCommand{Name: "debugger/prompt", Interactive: true}
	if err := service.startPluginWorkspaceCommandSession(plugin, command, "task_interrupt", 1, time.Now().UTC()); err != nil {
		t.Fatalf("startPluginWorkspaceCommandSession returned error: %v", err)
	}

	waitErr := make(chan error, 1)
	go func() {
		_, err := service.WaitPluginWorkspaceInput(plugin.ID, "task_interrupt", "debugger> ", time.Second)
		waitErr <- err
	}()

	waitForWorkspaceCondition(t, plugin, service, func(snapshot PluginWorkspaceSnapshot) bool {
		return snapshot.WaitingInput
	})

	interruptSnapshot, err := service.SignalPluginWorkspace(plugin.ID, 1, "task_interrupt", "interrupt")
	if err != nil {
		t.Fatalf("SignalPluginWorkspace returned error: %v", err)
	}
	if interruptSnapshot.WaitingInput {
		t.Fatalf("expected interrupt snapshot to leave waiting state, got %+v", interruptSnapshot)
	}

	err = <-waitErr
	if err == nil {
		t.Fatalf("expected input wait to be interrupted")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "interrupt") {
		t.Fatalf("expected interrupted error, got %v", err)
	}
	if len(interruptSnapshot.RecentControlEvents) == 0 {
		t.Fatalf("expected control timeline to record interrupt")
	}
	lastEvent := interruptSnapshot.RecentControlEvents[len(interruptSnapshot.RecentControlEvents)-1]
	if lastEvent.Type != "signal_sent" || lastEvent.Signal != "interrupt" || lastEvent.Result != "input_wait_interrupted" {
		t.Fatalf("expected structured interrupt control event, got %+v", lastEvent)
	}
	if len(interruptSnapshot.Entries) == 0 {
		t.Fatalf("expected workspace entries to include interrupt system entry")
	}
	lastEntry := interruptSnapshot.Entries[len(interruptSnapshot.Entries)-1]
	if lastEntry.Source != "host.workspace.signal" {
		t.Fatalf("expected interrupt entry source host.workspace.signal, got %+v", lastEntry)
	}
	if lastEntry.Metadata["signal"] != "interrupt" || lastEntry.Metadata["signal_result"] != "input_wait_interrupted" {
		t.Fatalf("expected interrupt entry metadata to include signal/result, got %+v", lastEntry.Metadata)
	}
}

func TestPluginWorkspaceSessionInterruptRunningRequestsCancel(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 11, Name: "demo", Runtime: PluginRuntimeJSWorker}
	command := &PluginWorkspaceCommand{Name: "debugger/prompt", Interactive: true}
	execCtx := &ExecutionContext{
		Metadata: map[string]string{
			PluginExecutionMetadataID: "task_running",
		},
	}
	_, _ = service.startPluginExecutionTask(
		plugin,
		PluginRuntimeJSWorker,
		pluginWorkspaceCommandExecuteAction,
		nil,
		execCtx,
		true,
	)
	if err := service.startPluginWorkspaceCommandSession(plugin, command, "task_running", 1, time.Now().UTC()); err != nil {
		t.Fatalf("startPluginWorkspaceCommandSession returned error: %v", err)
	}

	snapshot, err := service.SignalPluginWorkspace(plugin.ID, 1, "task_running", "interrupt")
	if err != nil {
		t.Fatalf("SignalPluginWorkspace returned error: %v", err)
	}
	task, exists := service.GetPluginExecutionTask(plugin.ID, "task_running")
	if !exists {
		t.Fatalf("expected running task to remain inspectable after cancel request")
	}
	if task.Cancelable {
		t.Fatalf("expected task to become non-cancelable after interrupt request, got %+v", task)
	}
	if len(snapshot.RecentControlEvents) == 0 {
		t.Fatalf("expected control timeline to record running interrupt")
	}
	lastEvent := snapshot.RecentControlEvents[len(snapshot.RecentControlEvents)-1]
	if lastEvent.Type != "signal_sent" || lastEvent.Signal != "interrupt" || lastEvent.Result != "cancel_requested" {
		t.Fatalf("expected structured running interrupt event, got %+v", lastEvent)
	}
	if len(snapshot.Entries) == 0 {
		t.Fatalf("expected workspace entries to include signal system entry")
	}
	lastEntry := snapshot.Entries[len(snapshot.Entries)-1]
	if lastEntry.Metadata["signal"] != "interrupt" || lastEntry.Metadata["signal_result"] != "cancel_requested" {
		t.Fatalf("expected running interrupt entry metadata to include signal/result, got %+v", lastEntry.Metadata)
	}

	service.finishPluginWorkspaceCommandSession(plugin.ID, "task_running", nil, context.Canceled)
	finishedSnapshot := waitForWorkspaceCondition(t, plugin, service, func(snapshot PluginWorkspaceSnapshot) bool {
		return snapshot.CompletedAt != nil
	})
	if finishedSnapshot.Status != PluginExecutionStatusCanceled {
		t.Fatalf("expected canceled status after interrupt finish, got %+v", finishedSnapshot)
	}
	if finishedSnapshot.CompletionReason != pluginWorkspaceCompletionReasonInterrupted {
		t.Fatalf("expected interrupted completion reason, got %+v", finishedSnapshot)
	}
	if len(finishedSnapshot.RecentControlEvents) == 0 {
		t.Fatalf("expected finished snapshot to include control timeline")
	}
	lastFinishedEvent := finishedSnapshot.RecentControlEvents[len(finishedSnapshot.RecentControlEvents)-1]
	if lastFinishedEvent.Type != "command_finished" ||
		lastFinishedEvent.Signal != "interrupt" ||
		lastFinishedEvent.Result != pluginWorkspaceCompletionReasonInterrupted {
		t.Fatalf("expected structured command_finished interrupt event, got %+v", lastFinishedEvent)
	}
}

func TestPluginWorkspaceSessionTerminateRunningSetsCompletionReason(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 13, Name: "demo", Runtime: PluginRuntimeJSWorker}
	command := &PluginWorkspaceCommand{Name: "debugger/prompt", Interactive: true}
	execCtx := &ExecutionContext{
		Metadata: map[string]string{
			PluginExecutionMetadataID: "task_terminate",
		},
	}
	_, _ = service.startPluginExecutionTask(
		plugin,
		PluginRuntimeJSWorker,
		pluginWorkspaceCommandExecuteAction,
		nil,
		execCtx,
		true,
	)
	if err := service.startPluginWorkspaceCommandSession(plugin, command, "task_terminate", 1, time.Now().UTC()); err != nil {
		t.Fatalf("startPluginWorkspaceCommandSession returned error: %v", err)
	}

	snapshot, err := service.SignalPluginWorkspace(plugin.ID, 1, "task_terminate", "terminate")
	if err != nil {
		t.Fatalf("SignalPluginWorkspace returned error: %v", err)
	}
	if len(snapshot.RecentControlEvents) == 0 {
		t.Fatalf("expected control timeline to record running terminate")
	}
	lastEvent := snapshot.RecentControlEvents[len(snapshot.RecentControlEvents)-1]
	if lastEvent.Type != "signal_sent" || lastEvent.Signal != "terminate" || lastEvent.Result != "cancel_requested" {
		t.Fatalf("expected structured running terminate event, got %+v", lastEvent)
	}

	service.finishPluginWorkspaceCommandSession(plugin.ID, "task_terminate", nil, context.Canceled)
	finishedSnapshot := waitForWorkspaceCondition(t, plugin, service, func(snapshot PluginWorkspaceSnapshot) bool {
		return snapshot.CompletedAt != nil
	})
	if finishedSnapshot.Status != PluginExecutionStatusCanceled {
		t.Fatalf("expected canceled status after terminate finish, got %+v", finishedSnapshot)
	}
	if finishedSnapshot.CompletionReason != pluginWorkspaceCompletionReasonTerminated {
		t.Fatalf("expected terminated completion reason, got %+v", finishedSnapshot)
	}
	if len(finishedSnapshot.RecentControlEvents) == 0 {
		t.Fatalf("expected finished snapshot to include control timeline")
	}
	lastFinishedEvent := finishedSnapshot.RecentControlEvents[len(finishedSnapshot.RecentControlEvents)-1]
	if lastFinishedEvent.Type != "command_finished" ||
		lastFinishedEvent.Signal != "terminate" ||
		lastFinishedEvent.Result != pluginWorkspaceCompletionReasonTerminated {
		t.Fatalf("expected structured command_finished terminate event, got %+v", lastFinishedEvent)
	}
}

func TestPluginWorkspaceSessionCanceledWithoutSignalKeepsCanceledReason(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 15, Name: "demo", Runtime: PluginRuntimeJSWorker}
	command := &PluginWorkspaceCommand{Name: "debugger/prompt", Interactive: true}
	if err := service.startPluginWorkspaceCommandSession(plugin, command, "task_plain_canceled", 1, time.Now().UTC()); err != nil {
		t.Fatalf("startPluginWorkspaceCommandSession returned error: %v", err)
	}

	service.finishPluginWorkspaceCommandSession(plugin.ID, "task_plain_canceled", nil, context.Canceled)
	finishedSnapshot := waitForWorkspaceCondition(t, plugin, service, func(snapshot PluginWorkspaceSnapshot) bool {
		return snapshot.CompletedAt != nil
	})
	if finishedSnapshot.Status != PluginExecutionStatusCanceled {
		t.Fatalf("expected canceled status, got %+v", finishedSnapshot)
	}
	if finishedSnapshot.CompletionReason != PluginExecutionStatusCanceled {
		t.Fatalf("expected canceled completion reason, got %+v", finishedSnapshot)
	}
	if len(finishedSnapshot.RecentControlEvents) == 0 {
		t.Fatalf("expected finished snapshot to include control timeline")
	}
	lastFinishedEvent := finishedSnapshot.RecentControlEvents[len(finishedSnapshot.RecentControlEvents)-1]
	if lastFinishedEvent.Type != "command_finished" ||
		lastFinishedEvent.Signal != "" ||
		lastFinishedEvent.Result != PluginExecutionStatusCanceled {
		t.Fatalf("expected structured command_finished canceled event, got %+v", lastFinishedEvent)
	}
}

func TestResetPluginWorkspaceResetsInteractiveSession(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 16, Name: "demo", Runtime: PluginRuntimeJSWorker}
	command := &PluginWorkspaceCommand{Name: "debugger/prompt", Interactive: true}
	if err := service.startPluginWorkspaceCommandSession(plugin, command, "task_reset", 1, time.Now().UTC()); err != nil {
		t.Fatalf("startPluginWorkspaceCommandSession returned error: %v", err)
	}

	waitErr := make(chan error, 1)
	go func() {
		_, err := service.WaitPluginWorkspaceInput(plugin.ID, "task_reset", "debugger> ", time.Second)
		waitErr <- err
	}()

	waitForWorkspaceCondition(t, plugin, service, func(snapshot PluginWorkspaceSnapshot) bool {
		return snapshot.WaitingInput
	})

	resetSnapshot, err := service.ResetPluginWorkspace(plugin, 1)
	if err != nil {
		t.Fatalf("ResetPluginWorkspace returned error: %v", err)
	}
	if resetSnapshot.Status != pluginWorkspaceStatusIdle {
		t.Fatalf("expected idle status after reset, got %+v", resetSnapshot)
	}
	if resetSnapshot.ActiveTaskID != "" || resetSnapshot.WaitingInput {
		t.Fatalf("expected reset snapshot to clear active task state, got %+v", resetSnapshot)
	}
	if len(resetSnapshot.Entries) == 0 {
		t.Fatalf("expected reset snapshot to contain reset system entry")
	}
	lastEntry := resetSnapshot.Entries[len(resetSnapshot.Entries)-1]
	if lastEntry.Source != "host.workspace.reset" {
		t.Fatalf("expected reset entry source host.workspace.reset, got %+v", lastEntry)
	}
	if len(resetSnapshot.RecentControlEvents) == 0 {
		t.Fatalf("expected reset snapshot to include control timeline")
	}
	lastEvent := resetSnapshot.RecentControlEvents[len(resetSnapshot.RecentControlEvents)-1]
	if lastEvent.Type != "workspace_reset" {
		t.Fatalf("expected workspace_reset control event, got %+v", lastEvent)
	}

	err = <-waitErr
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "reset") {
		t.Fatalf("expected waiter to be interrupted by reset, got %v", err)
	}
}

func TestPluginWorkspaceSubscriptionOwnerViewerHandover(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 17, Name: "demo", Runtime: PluginRuntimeJSWorker}

	ownerSnapshot, ownerStream, ownerCancel, err := service.SubscribePluginWorkspace(plugin, 101, 32)
	if err != nil {
		t.Fatalf("SubscribePluginWorkspace(owner) returned error: %v", err)
	}
	defer ownerCancel()
	if ownerSnapshot.OwnerAdminID != 101 {
		t.Fatalf("expected first attached admin to become owner, got %+v", ownerSnapshot)
	}
	if ownerSnapshot.ViewerCount != 0 {
		t.Fatalf("expected zero viewers for owner snapshot, got %+v", ownerSnapshot)
	}

	viewerSnapshot, viewerStream, viewerCancel, err := service.SubscribePluginWorkspace(plugin, 202, 32)
	if err != nil {
		t.Fatalf("SubscribePluginWorkspace(viewer) returned error: %v", err)
	}
	defer viewerCancel()
	if viewerSnapshot.OwnerAdminID != 101 {
		t.Fatalf("expected owner to stay attached admin 101, got %+v", viewerSnapshot)
	}
	if viewerSnapshot.ViewerCount != 1 {
		t.Fatalf("expected one viewer after second admin attached, got %+v", viewerSnapshot)
	}

	select {
	case event := <-ownerStream:
		if event.Workspace == nil {
			t.Fatalf("expected presence event to include workspace snapshot, got %+v", event)
		}
		if event.Workspace.OwnerAdminID != 101 || event.Workspace.ViewerCount != 1 {
			t.Fatalf("unexpected owner presence event: %+v", event.Workspace)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for owner presence update")
	}

	ownerCancel()

	select {
	case event := <-viewerStream:
		if event.Workspace == nil {
			t.Fatalf("expected takeover event to include workspace snapshot, got %+v", event)
		}
		if event.Workspace.OwnerAdminID != 202 || event.Workspace.ViewerCount != 0 {
			t.Fatalf("expected viewer to take over ownership, got %+v", event.Workspace)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for owner handover event")
	}
}

func TestPluginWorkspaceViewerCannotSubmitOrSignal(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 19, Name: "demo", Runtime: PluginRuntimeJSWorker}
	command := &PluginWorkspaceCommand{Name: "debugger/prompt", Interactive: true}

	if _, _, ownerCancel, err := service.SubscribePluginWorkspace(plugin, 301, 16); err != nil {
		t.Fatalf("SubscribePluginWorkspace(owner) returned error: %v", err)
	} else {
		defer ownerCancel()
	}
	if _, _, viewerCancel, err := service.SubscribePluginWorkspace(plugin, 302, 16); err != nil {
		t.Fatalf("SubscribePluginWorkspace(viewer) returned error: %v", err)
	} else {
		defer viewerCancel()
	}
	if err := service.startPluginWorkspaceCommandSession(plugin, command, "task_control", 301, time.Now().UTC()); err != nil {
		t.Fatalf("startPluginWorkspaceCommandSession returned error: %v", err)
	}

	waitErr := make(chan error, 1)
	go func() {
		_, err := service.WaitPluginWorkspaceInput(plugin.ID, "task_control", "debugger> ", time.Second)
		waitErr <- err
	}()

	waitForWorkspaceCondition(t, plugin, service, func(snapshot PluginWorkspaceSnapshot) bool {
		return snapshot.WaitingInput
	})

	if _, err := service.SubmitPluginWorkspaceInput(plugin.ID, 302, "task_control", "viewer input"); err == nil {
		t.Fatalf("expected viewer submit input to be rejected")
	} else if !strings.Contains(strings.ToLower(err.Error()), "controlled by admin #301") {
		t.Fatalf("expected owner contention error, got %v", err)
	}

	if _, err := service.SignalPluginWorkspace(plugin.ID, 302, "task_control", "interrupt"); err == nil {
		t.Fatalf("expected viewer signal to be rejected")
	} else if !strings.Contains(strings.ToLower(err.Error()), "controlled by admin #301") {
		t.Fatalf("expected owner contention error, got %v", err)
	}

	if _, err := service.SubmitPluginWorkspaceInput(plugin.ID, 301, "task_control", "owner input"); err != nil {
		t.Fatalf("expected owner submit input to succeed, got %v", err)
	}

	if err := <-waitErr; err != nil {
		t.Fatalf("expected owner input to unblock waiter, got %v", err)
	}
}

func TestPluginWorkspaceClaimControlRequiresAttachment(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 23, Name: "demo", Runtime: PluginRuntimeJSWorker}

	if _, _, _, err := service.SubscribePluginWorkspace(plugin, 401, 16); err != nil {
		t.Fatalf("SubscribePluginWorkspace(owner) returned error: %v", err)
	}

	if _, previousOwner, claimed, err := service.ClaimPluginWorkspaceControl(plugin, 999); err == nil {
		t.Fatalf("expected unattached admin claim to fail")
	} else {
		if previousOwner != 401 {
			t.Fatalf("expected current owner to remain 401, got %d", previousOwner)
		}
		if claimed {
			t.Fatalf("expected failed claim to report claimed=false")
		}
	}
}

func TestPluginWorkspaceClaimControlTransfersOwnership(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 29, Name: "demo", Runtime: PluginRuntimeJSWorker}

	_, ownerStream, ownerCancel, err := service.SubscribePluginWorkspace(plugin, 501, 16)
	if err != nil {
		t.Fatalf("SubscribePluginWorkspace(owner) returned error: %v", err)
	}
	defer ownerCancel()

	_, viewerStream, viewerCancel, err := service.SubscribePluginWorkspace(plugin, 502, 16)
	if err != nil {
		t.Fatalf("SubscribePluginWorkspace(viewer) returned error: %v", err)
	}
	defer viewerCancel()

	select {
	case <-ownerStream:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for initial presence event")
	}

	snapshot, previousOwner, claimed, err := service.ClaimPluginWorkspaceControl(plugin, 502)
	if err != nil {
		t.Fatalf("ClaimPluginWorkspaceControl returned error: %v", err)
	}
	if previousOwner != 501 {
		t.Fatalf("expected previous owner 501, got %d", previousOwner)
	}
	if !claimed {
		t.Fatalf("expected claim to report changed=true")
	}
	if snapshot.OwnerAdminID != 502 || snapshot.ViewerCount != 1 {
		t.Fatalf("unexpected post-claim snapshot: %+v", snapshot)
	}

	select {
	case event := <-viewerStream:
		if event.Workspace == nil {
			t.Fatalf("expected control event to include workspace snapshot")
		}
		if event.Workspace.OwnerAdminID != 502 {
			t.Fatalf("expected viewer to become owner, got %+v", event.Workspace)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timed out waiting for control transfer event")
	}
}

func TestPluginWorkspaceAutoReleasesIdleOwner(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 31, Name: "demo", Runtime: PluginRuntimeJSWorker}

	if _, _, _, err := service.SubscribePluginWorkspace(plugin, 601, 16); err != nil {
		t.Fatalf("SubscribePluginWorkspace(owner) returned error: %v", err)
	}
	if _, _, _, err := service.SubscribePluginWorkspace(plugin, 602, 16); err != nil {
		t.Fatalf("SubscribePluginWorkspace(viewer) returned error: %v", err)
	}

	service.workspaceMu.Lock()
	buffer := service.workspaceBuffers[plugin.ID]
	if buffer == nil {
		service.workspaceMu.Unlock()
		t.Fatalf("expected workspace buffer to exist")
	}
	buffer.ownerLastActiveAt = time.Now().Add(-defaultPluginWorkspaceOwnerIdleTimeout - time.Minute).UTC()
	service.workspaceMu.Unlock()

	snapshot := service.GetPluginWorkspaceSnapshot(plugin, 16)
	if snapshot.OwnerAdminID != 602 {
		t.Fatalf("expected idle owner to auto-transfer to viewer, got %+v", snapshot)
	}
	if len(snapshot.RecentControlEvents) == 0 {
		t.Fatalf("expected control timeline to record auto release")
	}
	lastEvent := snapshot.RecentControlEvents[len(snapshot.RecentControlEvents)-1]
	if lastEvent.Type != "control_auto_transferred" {
		t.Fatalf("expected last control event to be control_auto_transferred, got %+v", lastEvent)
	}
}

func TestSubmitPluginWorkspaceInputQueuesBeforeWait(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 37, Name: "demo", Runtime: PluginRuntimeJSWorker}
	command := &PluginWorkspaceCommand{Name: "debugger/prompt", Interactive: true}
	if err := service.startPluginWorkspaceCommandSession(plugin, command, "task_queue", 1, time.Now().UTC()); err != nil {
		t.Fatalf("startPluginWorkspaceCommandSession returned error: %v", err)
	}

	snapshot, err := service.SubmitPluginWorkspaceInput(plugin.ID, 1, "task_queue", "queued input")
	if err != nil {
		t.Fatalf("SubmitPluginWorkspaceInput returned error: %v", err)
	}
	if snapshot.WaitingInput {
		t.Fatalf("expected queued snapshot to stay out of waiting state, got %+v", snapshot)
	}
	if len(snapshot.Entries) == 0 {
		t.Fatalf("expected queued input snapshot to include echoed stdin entry")
	}
	lastEntry := snapshot.Entries[len(snapshot.Entries)-1]
	if lastEntry.Source != "host.workspace.stdin" {
		t.Fatalf("expected echoed stdin source host.workspace.stdin, got %+v", lastEntry)
	}

	value, waitErr := service.WaitPluginWorkspaceInput(plugin.ID, "task_queue", "debugger> ", time.Second)
	if waitErr != nil {
		t.Fatalf("WaitPluginWorkspaceInput returned error: %v", waitErr)
	}
	if value != "queued input" {
		t.Fatalf("expected queued input to round-trip, got %q", value)
	}
}

func TestEnterPluginWorkspaceTerminalLineRoutesToInteractiveSession(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
		executionCatalog:  newPluginExecutionCatalog(),
	}
	plugin := &models.Plugin{ID: 41, Name: "demo", Runtime: PluginRuntimeJSWorker, Type: "tool", Enabled: true}
	service.executionCatalog.byID[plugin.ID] = pluginExecutionCatalogEntry{
		Plugin:  *plugin,
		Runtime: PluginRuntimeJSWorker,
	}

	command := &PluginWorkspaceCommand{Name: "debugger/prompt", Interactive: true}
	if err := service.startPluginWorkspaceCommandSession(plugin, command, "task_terminal", 11, time.Now().UTC()); err != nil {
		t.Fatalf("startPluginWorkspaceCommandSession returned error: %v", err)
	}

	result, err := service.EnterPluginWorkspaceTerminalLine(plugin.ID, 11, "terminal queued", nil)
	if err != nil {
		t.Fatalf("EnterPluginWorkspaceTerminalLine returned error: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected terminal line result success, got %+v", result)
	}
	if result.Mode != "input" || !result.Queued {
		t.Fatalf("expected active session input to be queued, got %+v", result)
	}
	if result.TaskID != "task_terminal" {
		t.Fatalf("expected active task id to be retained, got %+v", result)
	}

	value, waitErr := service.WaitPluginWorkspaceInput(plugin.ID, "task_terminal", "debugger> ", time.Second)
	if waitErr != nil {
		t.Fatalf("WaitPluginWorkspaceInput returned error: %v", waitErr)
	}
	if value != "terminal queued" {
		t.Fatalf("expected queued terminal line to round-trip, got %q", value)
	}
}

func TestEnterPluginWorkspaceTerminalLineStartsShellSession(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
		executionCatalog:  newPluginExecutionCatalog(),
	}
	plugin := &models.Plugin{ID: 43, Name: "demo", Runtime: PluginRuntimeJSWorker, Type: "tool", Enabled: true}
	service.executionCatalog.byID[plugin.ID] = pluginExecutionCatalogEntry{
		Plugin:  *plugin,
		Runtime: PluginRuntimeJSWorker,
	}

	result, err := service.EnterPluginWorkspaceTerminalLine(plugin.ID, 21, "help", nil)
	if err != nil {
		t.Fatalf("EnterPluginWorkspaceTerminalLine returned error: %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected shell terminal line result success, got %+v", result)
	}
	if result.Mode != "command_started" {
		t.Fatalf("expected non-interactive shell line to start a session, got %+v", result)
	}
	if strings.TrimSpace(result.TaskID) == "" {
		t.Fatalf("expected shell session to allocate task id, got %+v", result)
	}
	if result.Interactive {
		t.Fatalf("expected shell session to be non-interactive, got %+v", result)
	}
	if len(result.Workspace.Entries) == 0 {
		t.Fatalf("expected shell session snapshot to include echoed command entry")
	}
	foundCommandEcho := false
	for _, entry := range result.Workspace.Entries {
		if entry.Source == "host.workspace.command" && strings.Contains(entry.Message, "help") {
			foundCommandEcho = true
			break
		}
	}
	if !foundCommandEcho {
		t.Fatalf("expected echoed shell command entry, got %+v", result.Workspace.Entries)
	}
}

func TestEnterPluginWorkspaceTerminalLineRejectsDisabledPluginOutsideCatalog(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	service := NewPluginManagerService(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			Enabled:         true,
			AllowedRuntimes: []string{PluginRuntimeGRPC, PluginRuntimeJSWorker},
			DefaultRuntime:  PluginRuntimeJSWorker,
		},
	})

	plugin := models.Plugin{
		Name:    "disabled-workspace-plugin",
		Type:    "tool",
		Runtime: PluginRuntimeJSWorker,
		Address: "index.js",
		Enabled: false,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("create plugin failed: %v", err)
	}
	if err := db.Model(&plugin).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable plugin failed: %v", err)
	}

	if _, err := service.EnterPluginWorkspaceTerminalLine(plugin.ID, 21, "help", nil); err == nil {
		t.Fatalf("expected disabled plugin workspace terminal entry to be rejected")
	} else if !strings.Contains(strings.ToLower(err.Error()), "disabled") {
		t.Fatalf("expected disabled error, got %v", err)
	}
}

func TestSignalPluginWorkspaceUsesSessionCancelWhenTaskMissing(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 47, Name: "demo", Runtime: PluginRuntimeJSWorker}

	canceled := false
	if err := service.startPluginWorkspaceShellSession(
		plugin,
		"help",
		"task_shell_cancel",
		31,
		time.Now().UTC(),
		func() { canceled = true },
	); err != nil {
		t.Fatalf("startPluginWorkspaceShellSession returned error: %v", err)
	}

	snapshot, err := service.SignalPluginWorkspace(plugin.ID, 31, "task_shell_cancel", "interrupt")
	if err != nil {
		t.Fatalf("SignalPluginWorkspace returned error: %v", err)
	}
	if !canceled {
		t.Fatalf("expected shell session cancel function to be invoked")
	}
	if len(snapshot.RecentControlEvents) == 0 {
		t.Fatalf("expected control timeline to record shell interrupt")
	}
	lastEvent := snapshot.RecentControlEvents[len(snapshot.RecentControlEvents)-1]
	if lastEvent.Type != "signal_sent" || lastEvent.Signal != "interrupt" || lastEvent.Result != "cancel_requested" {
		t.Fatalf("expected shell interrupt to stay cancel_requested, got %+v", lastEvent)
	}
}
