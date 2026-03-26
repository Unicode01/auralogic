package service

import (
	"testing"

	"auralogic/internal/models"
	"auralogic/internal/pluginipc"
)

func TestPluginWorkspaceDeltaTrimsAndClears(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers: map[uint]*pluginWorkspaceBuffer{
			7: newPluginWorkspaceBuffer(7, "demo", PluginRuntimeJSWorker, 2),
		},
	}
	plugin := &models.Plugin{ID: 7, Name: "demo", Runtime: PluginRuntimeJSWorker}

	service.ApplyPluginWorkspaceDelta(7, "demo", PluginRuntimeJSWorker, map[string]string{"action": "hook.execute"}, []pluginipc.WorkspaceBufferEntry{
		{Message: "first"},
		{Message: "second"},
		{Message: "third"},
	}, false)

	snapshot := service.GetPluginWorkspaceSnapshot(plugin, 10)
	if snapshot.EntryCount != 2 {
		t.Fatalf("expected 2 retained entries, got %d", snapshot.EntryCount)
	}
	if len(snapshot.Entries) != 2 {
		t.Fatalf("expected 2 snapshot entries, got %d", len(snapshot.Entries))
	}
	if snapshot.Entries[0].Message != "second" || snapshot.Entries[1].Message != "third" {
		t.Fatalf("unexpected retained messages: %#v", snapshot.Entries)
	}
	if snapshot.Entries[0].Seq != 2 || snapshot.Entries[1].Seq != 3 {
		t.Fatalf("unexpected sequence numbers: %#v", snapshot.Entries)
	}
	if snapshot.Entries[0].Action != "hook.execute" {
		t.Fatalf("expected action metadata to be merged into entries, got %#v", snapshot.Entries[0])
	}

	service.ApplyPluginWorkspaceDelta(7, "demo", PluginRuntimeJSWorker, nil, nil, true)
	snapshot = service.GetPluginWorkspaceSnapshot(plugin, 10)
	if snapshot.EntryCount != 0 || len(snapshot.Entries) != 0 {
		t.Fatalf("expected workspace to be cleared, got %+v", snapshot)
	}
	if snapshot.LastSeq != 3 {
		t.Fatalf("expected last sequence to remain monotonic after clear, got %d", snapshot.LastSeq)
	}
}

func TestPreparePluginWorkspaceConfigReturnsSeedHistory(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers: make(map[uint]*pluginWorkspaceBuffer),
	}
	plugin := &models.Plugin{ID: 11, Name: "seed", Runtime: PluginRuntimeJSWorker}

	service.ApplyPluginWorkspaceDelta(11, "seed", PluginRuntimeJSWorker, map[string]string{
		"action":  "hook.execute",
		"hook":    "frontend.bootstrap",
		"task_id": "pex_test",
	}, []pluginipc.WorkspaceBufferEntry{
		{Message: "older", Channel: "console", Level: "info"},
		{Message: "newer", Channel: "workspace", Level: "warn"},
	}, false)

	cfg := service.PreparePluginWorkspaceConfig(plugin, pluginWorkspaceCommandExecuteAction, nil, 1)
	if cfg == nil || !cfg.Enabled {
		t.Fatalf("expected workspace config to be enabled, got %#v", cfg)
	}
	if cfg.MaxEntries != defaultPluginWorkspaceBufferCapacity {
		t.Fatalf("expected default max entries %d, got %d", defaultPluginWorkspaceBufferCapacity, cfg.MaxEntries)
	}
	if len(cfg.History) != 1 {
		t.Fatalf("expected one history entry, got %d", len(cfg.History))
	}
	if cfg.History[0].Message != "newer" {
		t.Fatalf("expected newest entry in seed history, got %#v", cfg.History[0])
	}
	if cfg.History[0].Metadata["hook"] != "frontend.bootstrap" {
		t.Fatalf("expected metadata to survive roundtrip, got %#v", cfg.History[0].Metadata)
	}
}

func TestPreparePluginWorkspaceConfigDoesNotSeedHistoryForNonWorkspaceAction(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers: make(map[uint]*pluginWorkspaceBuffer),
	}
	plugin := &models.Plugin{ID: 12, Name: "seed", Runtime: PluginRuntimeJSWorker}

	service.ApplyPluginWorkspaceDelta(12, "seed", PluginRuntimeJSWorker, map[string]string{
		"action": "workspace.runtime.eval",
	}, []pluginipc.WorkspaceBufferEntry{
		{Message: "secret output", Channel: "stdout", Level: "info"},
	}, false)

	cfg := service.PreparePluginWorkspaceConfig(plugin, "hook.execute", nil, 10)
	if cfg == nil || !cfg.Enabled {
		t.Fatalf("expected workspace config to stay enabled, got %#v", cfg)
	}
	if len(cfg.History) != 0 {
		t.Fatalf("expected non-workspace execution to receive empty workspace history, got %#v", cfg.History)
	}
}

func TestPluginWorkspaceDeltaMergesTerminalStreamWrites(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers: map[uint]*pluginWorkspaceBuffer{
			9: newPluginWorkspaceBuffer(9, "terminal", PluginRuntimeJSWorker, 8),
		},
	}
	plugin := &models.Plugin{ID: 9, Name: "terminal", Runtime: PluginRuntimeJSWorker}

	service.ApplyPluginWorkspaceDelta(9, "terminal", PluginRuntimeJSWorker, nil, []pluginipc.WorkspaceBufferEntry{
		{Message: "hello", Channel: "stdout", Level: "info", Source: "plugin.workspace.write"},
		{Message: " world", Channel: "stdout", Level: "info", Source: "plugin.workspace.write"},
		{Message: "!\n", Channel: "stdout", Level: "info", Source: "plugin.workspace.writeln"},
	}, false)

	snapshot := service.GetPluginWorkspaceSnapshot(plugin, 10)
	if snapshot.EntryCount != 1 || len(snapshot.Entries) != 1 {
		t.Fatalf("expected terminal stream to merge into one retained entry, got %+v", snapshot)
	}
	if snapshot.Entries[0].Seq != 1 {
		t.Fatalf("expected merged terminal entry to keep first sequence id, got %+v", snapshot.Entries[0])
	}
	if snapshot.Entries[0].Message != "hello world!\n" {
		t.Fatalf("expected merged terminal output, got %+v", snapshot.Entries[0])
	}
}

func TestAppendPluginWorkspaceSessionEntriesFallsBackWithoutActiveSession(t *testing.T) {
	service := &PluginManagerService{
		workspaceBuffers:  make(map[uint]*pluginWorkspaceBuffer),
		workspaceSessions: make(map[uint]*pluginWorkspaceSession),
	}
	plugin := &models.Plugin{ID: 21, Name: "runtime", Runtime: PluginRuntimeJSWorker}

	service.ApplyPluginWorkspaceDelta(plugin.ID, plugin.Name, plugin.Runtime, nil, []pluginipc.WorkspaceBufferEntry{
		{Message: "seed", Channel: "console", Level: "info"},
	}, false)

	if err := service.AppendPluginWorkspaceSessionEntries(
		plugin.ID,
		"pex_runtime_eval",
		[]pluginipc.WorkspaceBufferEntry{{
			Message: "async",
			Channel: "console",
			Level:   "info",
			Source:  "console.log",
		}},
		false,
	); err != nil {
		t.Fatalf("expected async runtime append without active session to succeed, got %v", err)
	}

	snapshot := service.GetPluginWorkspaceSnapshot(plugin, 10)
	if snapshot.EntryCount != 2 || len(snapshot.Entries) != 2 {
		t.Fatalf("expected fallback append to retain both entries, got %+v", snapshot)
	}
	if snapshot.Entries[1].Message != "async" {
		t.Fatalf("expected appended async entry to be retained, got %+v", snapshot.Entries[1])
	}
}
