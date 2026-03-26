package jsworker

import (
	"testing"

	"auralogic/internal/pluginipc"
)

func TestPluginWorkspaceStateWriteFlushAndClear(t *testing.T) {
	state := newPluginWorkspaceState(&pluginipc.WorkspaceConfig{
		Enabled:    true,
		MaxEntries: 2,
		History: []pluginipc.WorkspaceBufferEntry{
			{Message: "seed", Channel: "workspace", Level: "info"},
		},
	})

	state.write("workspace", "info", "first", "plugin.workspace.info", map[string]string{"action": "ping"})
	state.write("console", "warn", "second", "console.warn", nil)

	tail := state.tail(10)
	if len(tail) != 2 {
		t.Fatalf("expected tail to keep max 2 entries, got %d", len(tail))
	}
	if tail[0].Message != "first" || tail[1].Message != "second" {
		t.Fatalf("unexpected tail contents: %#v", tail)
	}
	if tail[0].Metadata["action"] != "ping" {
		t.Fatalf("expected metadata to be retained, got %#v", tail[0].Metadata)
	}

	entries, cleared := state.flushDelta()
	if cleared {
		t.Fatalf("expected initial flush not to report clear")
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 pending entries, got %d", len(entries))
	}

	if !state.clear() {
		t.Fatalf("expected clear to succeed")
	}
	entries, cleared = state.flushDelta()
	if !cleared {
		t.Fatalf("expected clear flag to be propagated on flush")
	}
	if len(entries) != 0 {
		t.Fatalf("expected no entries after clear flush, got %d", len(entries))
	}
}

func TestPluginWorkspaceStateConfigureCommandAndReadInput(t *testing.T) {
	state := newPluginWorkspaceState(&pluginipc.WorkspaceConfig{
		Enabled:    true,
		MaxEntries: 8,
	})

	state.configureCommand("workspace.command.execute", map[string]string{
		"workspace_command_name":             "debugger/prompt",
		"workspace_command_entry":            "debugger.prompt",
		"workspace_command_id":               "cmd_1",
		"workspace_command_raw":              "debugger/prompt alpha",
		"workspace_command_argv_json":        `["alpha"]`,
		"workspace_command_input_lines_json": `["hello workspace"]`,
	})

	if state.commandName != "debugger/prompt" {
		t.Fatalf("expected command name to be configured, got %q", state.commandName)
	}
	if state.commandEntry != "debugger.prompt" {
		t.Fatalf("expected command entry to be configured, got %q", state.commandEntry)
	}
	if state.commandID != "cmd_1" {
		t.Fatalf("expected command id to be configured, got %q", state.commandID)
	}
	if len(state.commandArgv) != 1 || state.commandArgv[0] != "alpha" {
		t.Fatalf("expected command argv to be configured, got %#v", state.commandArgv)
	}

	value, ok := state.readInput("debugger> ", false, true, "plugin.workspace.readLine")
	if !ok {
		t.Fatalf("expected workspace input to be available")
	}
	if value != "hello workspace" {
		t.Fatalf("expected workspace input to match, got %q", value)
	}

	entries := state.tail(10)
	if len(entries) != 2 {
		t.Fatalf("expected prompt + input to be recorded, got %d", len(entries))
	}
	if entries[0].Channel != "prompt" || entries[0].Message != "debugger>" {
		t.Fatalf("expected first entry to be prompt, got %#v", entries[0])
	}
	if entries[1].Channel != "input" || entries[1].Message != "hello workspace" {
		t.Fatalf("expected second entry to be echoed input, got %#v", entries[1])
	}
	if entries[1].Metadata["command"] != "debugger/prompt" {
		t.Fatalf("expected input entry metadata to include command, got %#v", entries[1].Metadata)
	}
}

func TestPluginWorkspaceStateMergesTerminalStreamWrites(t *testing.T) {
	state := newPluginWorkspaceState(&pluginipc.WorkspaceConfig{
		Enabled:    true,
		MaxEntries: 8,
	})

	state.write("stdout", "info", "hello", "plugin.workspace.write", nil)
	state.write("stdout", "info", " world", "plugin.workspace.write", nil)
	state.write("stdout", "info", "!\n", "plugin.workspace.writeln", nil)

	entries := state.tail(10)
	if len(entries) != 1 {
		t.Fatalf("expected terminal writes to merge into one entry, got %#v", entries)
	}
	if entries[0].Channel != "stdout" {
		t.Fatalf("expected merged terminal entry to stay on stdout, got %#v", entries[0])
	}
	if entries[0].Message != "hello world!\n" {
		t.Fatalf("expected merged terminal output, got %#v", entries[0])
	}

	pending, cleared := state.flushDelta()
	if cleared {
		t.Fatalf("expected no clear flag for merged terminal writes")
	}
	if len(pending) != 1 || pending[0].Message != "hello world!\n" {
		t.Fatalf("expected pending terminal output to merge as well, got %#v", pending)
	}
}

func TestPluginWorkspaceStateBatchesTerminalForwarderWrites(t *testing.T) {
	state := newPluginWorkspaceState(&pluginipc.WorkspaceConfig{
		Enabled:    true,
		MaxEntries: 8,
	})

	var forwarded [][]pluginipc.WorkspaceBufferEntry
	state.setForwarder(func(entries []pluginipc.WorkspaceBufferEntry, cleared bool) error {
		if cleared {
			t.Fatalf("did not expect clear forwarding in this test")
		}
		forwarded = append(forwarded, cloneWorkspaceBufferEntries(entries))
		return nil
	})

	state.write("stdout", "info", "hello", "plugin.workspace.write", nil)
	state.write("stdout", "info", " world", "plugin.workspace.write", nil)
	if len(forwarded) != 0 {
		t.Fatalf("expected partial terminal writes to stay buffered, got %#v", forwarded)
	}

	state.write("stdout", "info", "!\n", "plugin.workspace.writeln", nil)
	if len(forwarded) != 1 {
		t.Fatalf("expected a single forwarded batch, got %#v", forwarded)
	}
	if len(forwarded[0]) != 1 || forwarded[0][0].Message != "hello world!\n" {
		t.Fatalf("expected merged forwarded terminal output, got %#v", forwarded)
	}

	pending, cleared := state.flushDelta()
	if cleared {
		t.Fatalf("expected no clear flag for forwarded terminal writes")
	}
	if len(pending) != 0 {
		t.Fatalf("expected forwarded entries to be removed from pending, got %#v", pending)
	}
}
