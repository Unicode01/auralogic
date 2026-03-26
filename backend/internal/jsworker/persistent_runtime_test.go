package jsworker

import (
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"auralogic/internal/pluginipc"
)

func TestPersistentRuntimeSharesGlobalStateAndRefreshesInvocationState(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
let counter = 0;
let saved = null;

module.exports.execute = function(action, params, context, config, sandbox) {
  counter += 1;
  if (params && params.value) {
    saved = params.value;
  }
  return {
    success: true,
    data: {
      counter: counter,
      saved: saved,
      current_action: sandbox.currentAction,
      token: Plugin.secret.get("token"),
      stored: Plugin.storage.get("demo")
    }
  };
};
`))

	pluginID := uint(4101)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := func(params map[string]string, storage map[string]string, secrets map[string]string) pluginipc.Request {
		return pluginipc.Request{
			Type:             "execute",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "demo.execute",
			ScriptPath:       scriptPath,
			Params:           params,
			Storage:          storage,
			PluginSecrets:    secrets,
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "demo.execute",
			},
		}
	}

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       2,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}

	first := handleExecuteRequest(nil, req(
		map[string]string{"value": "persisted"},
		map[string]string{"demo": "first"},
		map[string]string{"token": "alpha"},
	), opts)
	if !first.Success || first.Error != "" {
		t.Fatalf("first execute failed: %+v", first)
	}
	if got := testInterfaceToInt64(first.Data["counter"]); got != 1 {
		t.Fatalf("expected first counter=1, got %#v", first.Data)
	}
	if got := interfaceToString(first.Data["saved"]); got != "persisted" {
		t.Fatalf("expected saved value to persist, got %#v", first.Data)
	}
	if got := interfaceToString(first.Data["token"]); got != "alpha" {
		t.Fatalf("expected first token alpha, got %#v", first.Data)
	}
	if got := interfaceToString(first.Data["stored"]); got != "first" {
		t.Fatalf("expected first stored value, got %#v", first.Data)
	}

	second := handleExecuteRequest(nil, req(
		map[string]string{},
		map[string]string{"demo": "second"},
		map[string]string{"token": "beta"},
	), opts)
	if !second.Success || second.Error != "" {
		t.Fatalf("second execute failed: %+v", second)
	}
	if got := testInterfaceToInt64(second.Data["counter"]); got != 2 {
		t.Fatalf("expected second counter=2, got %#v", second.Data)
	}
	if got := interfaceToString(second.Data["saved"]); got != "persisted" {
		t.Fatalf("expected saved global state to remain persisted, got %#v", second.Data)
	}
	if got := interfaceToString(second.Data["token"]); got != "beta" {
		t.Fatalf("expected second token beta, got %#v", second.Data)
	}
	if got := interfaceToString(second.Data["stored"]); got != "second" {
		t.Fatalf("expected second stored value to refresh from invocation snapshot, got %#v", second.Data)
	}
	if got := interfaceToString(second.Data["current_action"]); got != "demo.execute" {
		t.Fatalf("expected current_action to refresh, got %#v", second.Data)
	}
}

func TestPersistentRuntimeIsolatedByGeneration(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
let counter = 0;

module.exports.execute = function() {
  counter += 1;
  return {
    success: true,
    data: {
      counter: counter
    }
  };
};
`))

	pluginID := uint(4102)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, 1)
		globalPersistentPluginRuntimeManager.dispose(pluginID, 2)
	})

	makeRequest := func(generation uint) pluginipc.Request {
		return pluginipc.Request{
			Type:             "execute",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "demo.execute",
			ScriptPath:       scriptPath,
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "demo.execute",
			},
		}
	}

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       2,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}

	firstGenFirst := handleExecuteRequest(nil, makeRequest(1), opts)
	firstGenSecond := handleExecuteRequest(nil, makeRequest(1), opts)
	secondGenFirst := handleExecuteRequest(nil, makeRequest(2), opts)

	if !firstGenFirst.Success || !firstGenSecond.Success || !secondGenFirst.Success {
		t.Fatalf("unexpected execute failure: gen1_first=%+v gen1_second=%+v gen2_first=%+v", firstGenFirst, firstGenSecond, secondGenFirst)
	}
	if got := testInterfaceToInt64(firstGenFirst.Data["counter"]); got != 1 {
		t.Fatalf("expected generation 1 first counter=1, got %#v", firstGenFirst.Data)
	}
	if got := testInterfaceToInt64(firstGenSecond.Data["counter"]); got != 2 {
		t.Fatalf("expected generation 1 second counter=2, got %#v", firstGenSecond.Data)
	}
	if got := testInterfaceToInt64(secondGenFirst.Data["counter"]); got != 1 {
		t.Fatalf("expected generation 2 to start from a fresh runtime, got %#v", secondGenFirst.Data)
	}
}

func TestPersistentRuntimeDisposeCreatesFreshVM(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
let counter = 0;

module.exports.execute = function() {
  counter += 1;
  return {
    success: true,
    data: { counter: counter }
  };
};
`))

	pluginID := uint(4103)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	makeRequest := func() pluginipc.Request {
		return pluginipc.Request{
			Type:             "execute",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "demo.execute",
			ScriptPath:       scriptPath,
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "demo.execute",
			},
		}
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	beforeDispose := handleExecuteRequest(nil, makeRequest(), opts)
	if !beforeDispose.Success {
		t.Fatalf("execute before dispose failed: %+v", beforeDispose)
	}
	if got := testInterfaceToInt64(beforeDispose.Data["counter"]); got != 1 {
		t.Fatalf("expected first counter=1, got %#v", beforeDispose.Data)
	}

	secondBeforeDispose := handleExecuteRequest(nil, makeRequest(), opts)
	if !secondBeforeDispose.Success {
		t.Fatalf("second execute before dispose failed: %+v", secondBeforeDispose)
	}
	if got := testInterfaceToInt64(secondBeforeDispose.Data["counter"]); got != 2 {
		t.Fatalf("expected second counter=2, got %#v", secondBeforeDispose.Data)
	}

	globalPersistentPluginRuntimeManager.dispose(pluginID, generation)

	afterDispose := handleExecuteRequest(nil, makeRequest(), opts)
	if !afterDispose.Success {
		t.Fatalf("execute after dispose failed: %+v", afterDispose)
	}
	if got := testInterfaceToInt64(afterDispose.Data["counter"]); got != 1 {
		t.Fatalf("expected counter to reset after dispose, got %#v", afterDispose.Data)
	}
}

func TestPersistentRuntimeRecoversAfterInitFailure(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
throw new Error("boom during init");
`))

	pluginID := uint(4104)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := pluginipc.Request{
		Type:             "execute",
		PluginID:         pluginID,
		PluginGeneration: generation,
		PluginName:       "persistent-plugin",
		Action:           "demo.execute",
		ScriptPath:       scriptPath,
		Sandbox: pluginipc.SandboxConfig{
			CurrentAction: "demo.execute",
		},
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	failed := handleExecuteRequest(nil, req, opts)
	if failed.Success {
		t.Fatalf("expected init failure, got %+v", failed)
	}

	mustWriteFile(t, scriptPath, []byte(`
let counter = 0;
module.exports.execute = function() {
  counter += 1;
  return {
    success: true,
    data: { counter: counter }
  };
};
`))

	recovered := handleExecuteRequest(nil, req, opts)
	if !recovered.Success || recovered.Error != "" {
		t.Fatalf("expected runtime to recover after script fix, got %+v", recovered)
	}
	if got := testInterfaceToInt64(recovered.Data["counter"]); got != 1 {
		t.Fatalf("expected recovered runtime to start clean, got %#v", recovered.Data)
	}
}

func TestPersistentRuntimeConsoleResultReferencesTrackLastSuccessfulValues(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
globalThis.seed = 40;
`))

	pluginID := uint(4105)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := func(code string) pluginipc.Request {
		return pluginipc.Request{
			Type:             "runtime_eval",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "workspace.runtime.eval",
			ScriptPath:       scriptPath,
			RuntimeCode:      code,
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "workspace.runtime.eval",
			},
		}
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	first := handleRuntimeEvalRequest(nil, req("seed + 2"), opts)
	if !first.Success || first.Error != "" {
		t.Fatalf("first runtime eval failed: %+v", first)
	}
	if got := testInterfaceToInt64(first.Data["value"]); got != 42 {
		t.Fatalf("expected first eval value=42, got %#v", first.Data)
	}
	runtimeState, ok := first.Data["runtime_state"].(map[string]interface{})
	if !ok || runtimeState == nil {
		t.Fatalf("expected runtime_state payload, got %#v", first.Data)
	}
	if got := interfaceToString(runtimeState["instance_id"]); got == "" {
		t.Fatalf("expected runtime instance_id, got %#v", runtimeState)
	}

	second := handleRuntimeEvalRequest(nil, req("$_ + 1"), opts)
	if !second.Success || second.Error != "" {
		t.Fatalf("second runtime eval failed: %+v", second)
	}
	if got := testInterfaceToInt64(second.Data["value"]); got != 43 {
		t.Fatalf("expected second eval value=43, got %#v", second.Data)
	}

	third := handleRuntimeEvalRequest(nil, req("$1 + $2"), opts)
	if !third.Success || third.Error != "" {
		t.Fatalf("third runtime eval failed: %+v", third)
	}
	if got := testInterfaceToInt64(third.Data["value"]); got != 85 {
		t.Fatalf("expected third eval value=85, got %#v", third.Data)
	}

	failed := handleRuntimeEvalRequest(nil, req(`throw new Error("boom")`), opts)
	if failed.Success {
		t.Fatalf("expected runtime eval failure, got %+v", failed)
	}

	afterFailure := handleRuntimeEvalRequest(nil, req("$_"), opts)
	if !afterFailure.Success || afterFailure.Error != "" {
		t.Fatalf("runtime eval after failure failed: %+v", afterFailure)
	}
	if got := testInterfaceToInt64(afterFailure.Data["value"]); got != 85 {
		t.Fatalf("expected $_ to keep the last successful result, got %#v", afterFailure.Data)
	}
}

func TestPersistentRuntimeConsoleResultReferencesStayHiddenFromObjectKeys(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
globalThis.seed = 1;
`))

	pluginID := uint(4106)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := func(code string) pluginipc.Request {
		return pluginipc.Request{
			Type:             "runtime_eval",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "workspace.runtime.eval",
			ScriptPath:       scriptPath,
			RuntimeCode:      code,
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "workspace.runtime.eval",
			},
		}
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	seed := handleRuntimeEvalRequest(nil, req("seed"), opts)
	if !seed.Success || seed.Error != "" {
		t.Fatalf("seed runtime eval failed: %+v", seed)
	}

	hidden := handleRuntimeEvalRequest(
		nil,
		req(`Object.keys(globalThis).includes("$_") || Object.keys(globalThis).includes("$1")`),
		opts,
	)
	if !hidden.Success || hidden.Error != "" {
		t.Fatalf("hidden globals runtime eval failed: %+v", hidden)
	}
	if got, ok := interfaceToBool(hidden.Data["value"]); !ok || got {
		t.Fatalf("expected runtime result globals to stay non-enumerable, got %#v", hidden.Data)
	}
}

func TestPersistentRuntimeConsoleHelpersExposeWorkspaceUtilities(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
globalThis.seed = 1;
function debuggerConsole(command) {
  return Array.isArray(command.argv) && command.argv.length === 0 ? "ok" : "bad";
}
module.exports.execute = function execute() {
  return { success: true };
};
module.exports.workspace = {
  debug: debuggerConsole
};
const keys = ["plugin-defined"];
`))

	pluginID := uint(4107)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := func(code string) pluginipc.Request {
		return pluginipc.Request{
			Type:             "runtime_eval",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "workspace.runtime.eval",
			ScriptPath:       scriptPath,
			RuntimeCode:      code,
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction:           "workspace.runtime.eval",
				AllowHookExecute:        true,
				AllowFrontendExtensions: true,
				RequestedPermissions:    []string{"host.order.read"},
				GrantedPermissions:      []string{"host.order.read"},
			},
			Workspace: &pluginipc.WorkspaceConfig{
				Enabled:    true,
				MaxEntries: 32,
			},
		}
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	helpers := handleRuntimeEvalRequest(nil, req(`help("Plugin.host").topic`), opts)
	if !helpers.Success || helpers.Error != "" {
		t.Fatalf("helper globals runtime eval failed: %+v", helpers)
	}
	if got := interfaceToString(helpers.Data["value"]); got != "Plugin.host" {
		t.Fatalf("expected help helper topic Plugin.host, got %#v", helpers.Data)
	}

	helpPreview := handleRuntimeEvalRequest(nil, req(`help()`), opts)
	if !helpPreview.Success || helpPreview.Error != "" {
		t.Fatalf("help preview runtime eval failed: %+v", helpPreview)
	}
	if got := interfaceToString(helpPreview.Data["summary"]); !strings.Contains(got, `topic: "console"`) {
		t.Fatalf("expected help preview summary to expose enumerable object fields, got %#v", helpPreview.Data)
	}
	helpKeys, ok := helpPreview.Data["keys"].([]string)
	if !ok || len(helpKeys) == 0 {
		t.Fatalf("expected help preview keys, got %#v", helpPreview.Data)
	}
	if !containsString(helpKeys, "topic") || !containsString(helpKeys, "helpers") {
		t.Fatalf("expected help preview keys to include topic/helpers, got %#v", helpKeys)
	}
	if !containsString(helpKeys, "plugin_exports") || containsString(helpKeys, "workspace_commands") {
		t.Fatalf("expected help preview keys to expose runtime exports without legacy workspace_commands, got %#v", helpKeys)
	}
	helperCatalog := handleRuntimeEvalRequest(
		nil,
		req(`help().helpers.includes("commands" + "()") && help().helpers.includes("permissions" + "()") && help().helpers.includes("workspaceState" + "(limit?)") && help().helpers.includes("inspect" + "(value?, depth?)") && help().helpers.includes("clearOutput" + "()")`),
		opts,
	)
	if !helperCatalog.Success || helperCatalog.Error != "" {
		t.Fatalf("runtime helper catalog eval failed: %+v", helperCatalog)
	}
	if got, ok := interfaceToBool(helperCatalog.Data["value"]); !ok || !got {
		t.Fatalf("expected help() to list expanded runtime helpers, got %#v", helperCatalog.Data)
	}
	workerHelp := handleRuntimeEvalRequest(
		nil,
		req(`help().globals.includes("Worker") && help("Worker").members.includes("worker.request(payload)")`),
		opts,
	)
	if !workerHelp.Success || workerHelp.Error != "" {
		t.Fatalf("worker help runtime eval failed: %+v", workerHelp)
	}
	if got, ok := interfaceToBool(workerHelp.Data["value"]); !ok || !got {
		t.Fatalf("expected help() to expose Worker runtime support, got %#v", workerHelp.Data)
	}
	asyncHelp := handleRuntimeEvalRequest(
		nil,
		req(`help().globals.includes("structuredClone") && help().globals.includes("queueMicrotask") && help().globals.includes("setTimeout") && help().globals.includes("clearTimeout") && help("setTimeout").topic === "setTimeout" && help("clearTimeout").topic === "clearTimeout"`),
		opts,
	)
	if !asyncHelp.Success || asyncHelp.Error != "" {
		t.Fatalf("async helper runtime eval failed: %+v", asyncHelp)
	}
	if got, ok := interfaceToBool(asyncHelp.Data["value"]); !ok || !got {
		t.Fatalf("expected help() to expose runtime async globals, got %#v", asyncHelp.Data)
	}

	exportHelp := handleRuntimeEvalRequest(
		nil,
		req(`help().plugin_exports.includes("debug") && help().plugin_exports.includes("module.exports.execute")`),
		opts,
	)
	if !exportHelp.Success || exportHelp.Error != "" {
		t.Fatalf("runtime export help failed: %+v", exportHelp)
	}
	if got, ok := interfaceToBool(exportHelp.Data["value"]); !ok || !got {
		t.Fatalf("expected help() to include live callable aliases and module exports, got %#v", exportHelp.Data)
	}

	workspaceCatalogHelp := handleRuntimeEvalRequest(
		nil,
		req(`help("debug").alias === "debug" && help("debug").entry === "debug" && help("debug").callable === true`),
		opts,
	)
	if !workspaceCatalogHelp.Success || workspaceCatalogHelp.Error != "" {
		t.Fatalf("workspace alias help failed: %+v", workspaceCatalogHelp)
	}
	if got, ok := interfaceToBool(workspaceCatalogHelp.Data["value"]); !ok || !got {
		t.Fatalf("expected help() to resolve live workspace aliases as callable functions, got %#v", workspaceCatalogHelp.Data)
	}

	callableTopicHelp := handleRuntimeEvalRequest(nil, req(`debug() === "ok"`), opts)
	if !callableTopicHelp.Success || callableTopicHelp.Error != "" {
		t.Fatalf("workspace alias call failed: %+v", callableTopicHelp)
	}
	if got, ok := interfaceToBool(callableTopicHelp.Data["value"]); !ok || !got {
		t.Fatalf("expected workspace alias to invoke live workspace function, got %#v", callableTopicHelp.Data)
	}

	keysResult := handleRuntimeEvalRequest(
		nil,
		req(`keys(Plugin.workspace).includes("write") && !keys().includes("__auralogicConsoleHelp")`),
		opts,
	)
	if !keysResult.Success || keysResult.Error != "" {
		t.Fatalf("keys helper runtime eval failed: %+v", keysResult)
	}
	if got, ok := interfaceToBool(keysResult.Data["value"]); !ok || !got {
		t.Fatalf("expected keys helper to expose helper globals and workspace methods, got %#v", keysResult.Data)
	}
	commandsResult := handleRuntimeEvalRequest(
		nil,
		req(`commands().workspace_aliases.includes("debug") && commands().plugin_exports.includes("module.exports.execute")`),
		opts,
	)
	if !commandsResult.Success || commandsResult.Error != "" {
		t.Fatalf("commands helper runtime eval failed: %+v", commandsResult)
	}
	if got, ok := interfaceToBool(commandsResult.Data["value"]); !ok || !got {
		t.Fatalf("expected commands() helper to expose aliases and exports, got %#v", commandsResult.Data)
	}
	permissionsResult := handleRuntimeEvalRequest(
		nil,
		req(`permissions().granted_permissions.includes("host.order.read") && permissions().missing_permissions.length === 0 && permissions().allow_hook_execute === true`),
		opts,
	)
	if !permissionsResult.Success || permissionsResult.Error != "" {
		t.Fatalf("permissions helper runtime eval failed: %+v", permissionsResult)
	}
	if got, ok := interfaceToBool(permissionsResult.Data["value"]); !ok || !got {
		t.Fatalf("expected permissions() helper to expose granted permissions and sandbox flags, got %#v", permissionsResult.Data)
	}
	workspaceStateResult := handleRuntimeEvalRequest(
		nil,
		req(`keys(workspaceState()).includes("enabled") && keys(workspaceState()).includes("entries") && keys(workspaceState()).includes("command_argv") && keys(workspaceState()).includes("pending_input_count")`),
		opts,
	)
	if !workspaceStateResult.Success || workspaceStateResult.Error != "" {
		t.Fatalf("workspaceState helper runtime eval failed: %+v", workspaceStateResult)
	}
	if got, ok := interfaceToBool(workspaceStateResult.Data["value"]); !ok || !got {
		t.Fatalf("expected workspaceState() helper to expose workspace snapshot, got %#v", workspaceStateResult.Data)
	}
	inspectResult := handleRuntimeEvalRequest(
		nil,
		req(`inspect({ alpha: 1 }, 2).type === "object" && inspect({ alpha: 1 }, 2).entries.length === 1`),
		opts,
	)
	if !inspectResult.Success || inspectResult.Error != "" {
		t.Fatalf("inspect helper runtime eval failed: %+v", inspectResult)
	}
	if got, ok := interfaceToBool(inspectResult.Data["value"]); !ok || !got {
		t.Fatalf("expected inspect() helper to expose structured previews, got %#v", inspectResult.Data)
	}
	clearOutputResult := handleRuntimeEvalRequest(
		nil,
		req(`typeof clearOutput().cleared === "boolean" && typeof clearOutput().workspace_enabled === "boolean"`),
		opts,
	)
	if !clearOutputResult.Success || clearOutputResult.Error != "" {
		t.Fatalf("clearOutput helper runtime eval failed: %+v", clearOutputResult)
	}
	if got, ok := interfaceToBool(clearOutputResult.Data["value"]); !ok || !got {
		t.Fatalf("expected clearOutput() helper to clear the workspace buffer, got %#v", clearOutputResult.Data)
	}

	pluginKeysResult := handleRuntimeEvalRequest(nil, req(`globalThis.seed`), opts)
	if !pluginKeysResult.Success || pluginKeysResult.Error != "" {
		t.Fatalf("expected runtime to load even when plugin declares const keys, got %+v", pluginKeysResult)
	}
	if got := testInterfaceToInt64(pluginKeysResult.Data["value"]); got != 1 {
		t.Fatalf("expected plugin runtime to stay usable after helper injection rewrite, got %#v", pluginKeysResult.Data)
	}

	runtimeStateResult := handleRuntimeEvalRequest(nil, req(`runtimeState().runtime.plugin_id`), opts)
	if !runtimeStateResult.Success || runtimeStateResult.Error != "" {
		t.Fatalf("runtimeState helper runtime eval failed: %+v", runtimeStateResult)
	}
	if got := testInterfaceToInt64(runtimeStateResult.Data["value"]); got != int64(pluginID) {
		t.Fatalf("expected runtimeState helper to expose plugin id %d, got %#v", pluginID, runtimeStateResult.Data)
	}

	runtimeStateSnapshot := handleRuntimeStateRequest(pluginipc.Request{
		Type:             "runtime_state",
		PluginID:         pluginID,
		PluginGeneration: generation,
		PluginName:       "persistent-plugin",
		ScriptPath:       scriptPath,
	})
	if !runtimeStateSnapshot.Success || runtimeStateSnapshot.Error != "" {
		t.Fatalf("runtime state request failed: %+v", runtimeStateSnapshot)
	}
	completionPaths, ok := runtimeStateSnapshot.Data["completion_paths"].([]string)
	if !ok || len(completionPaths) == 0 {
		t.Fatalf("expected runtime state completion paths, got %#v", runtimeStateSnapshot.Data)
	}
	if !containsString(completionPaths, "globalThis.seed") {
		t.Fatalf("expected runtime completion paths to include globalThis.seed, got %#v", completionPaths)
	}
	if !containsString(completionPaths, "seed") {
		t.Fatalf("expected runtime completion paths to include bare global seed, got %#v", completionPaths)
	}
	if !containsString(completionPaths, "module.exports.execute") {
		t.Fatalf("expected runtime completion paths to include module.exports.execute, got %#v", completionPaths)
	}
	if !containsString(completionPaths, "globalThis.debuggerConsole") {
		t.Fatalf("expected runtime completion paths to include top-level plugin function path, got %#v", completionPaths)
	}
	if !containsString(completionPaths, "debuggerConsole") {
		t.Fatalf("expected runtime completion paths to include bare top-level plugin function name, got %#v", completionPaths)
	}
	if !containsString(completionPaths, "debug") {
		t.Fatalf("expected runtime completion paths to include workspace alias debug, got %#v", completionPaths)
	}
	if containsString(completionPaths, "module.exports.workspace") {
		t.Fatalf("expected runtime completion paths to hide internal workspace export tree, got %#v", completionPaths)
	}
	if containsString(completionPaths, "console.log") {
		t.Fatalf("expected runtime completion paths to avoid duplicating static console builtins, got %#v", completionPaths)
	}
	if containsString(completionPaths, "Plugin.workspace.write") {
		t.Fatalf("expected runtime completion paths to avoid duplicating static Plugin workspace helpers, got %#v", completionPaths)
	}
	if containsString(completionPaths, "globalThis.help") || containsString(completionPaths, "help") {
		t.Fatalf("expected runtime completion paths to avoid duplicating static helper globals, got %#v", completionPaths)
	}
	if containsCompletionPathPrefix(completionPaths, "sandbox.currentAction.") {
		t.Fatalf("expected runtime completion paths to skip sandbox string character indexes, got %#v", completionPaths)
	}
	if containsCompletionPathPrefix(completionPaths, "Plugin.fs.codeRoot.") {
		t.Fatalf("expected runtime completion paths to skip filesystem string character indexes, got %#v", completionPaths)
	}
	if containsCompletionPathPrefix(completionPaths, "globalThis.__dirname.") {
		t.Fatalf("expected runtime completion paths to skip __dirname character indexes, got %#v", completionPaths)
	}
}

func TestPersistentRuntimeConsoleLogEmitsStructuredWorkspacePreview(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`globalThis.seed = 1;`))

	pluginID := uint(4108)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := pluginipc.Request{
		Type:             "runtime_eval",
		PluginID:         pluginID,
		PluginGeneration: generation,
		PluginName:       "persistent-plugin",
		Action:           "workspace.runtime.eval",
		ScriptPath:       scriptPath,
		RuntimeCode:      `console.log({ alpha: 1, beta: true })`,
		Workspace: &pluginipc.WorkspaceConfig{
			Enabled:    true,
			MaxEntries: 32,
		},
		Sandbox: pluginipc.SandboxConfig{
			CurrentAction: "workspace.runtime.eval",
		},
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}
	resp := handleRuntimeEvalRequest(nil, req, opts)
	if !resp.Success || resp.Error != "" {
		t.Fatalf("runtime eval failed: %+v", resp)
	}
	if len(resp.WorkspaceEntries) != 1 {
		t.Fatalf("expected a single workspace console entry, got %#v", resp.WorkspaceEntries)
	}
	entry := resp.WorkspaceEntries[0]
	if entry.Channel != "console" {
		t.Fatalf("expected console channel, got %#v", entry)
	}
	if got := strings.TrimSpace(entry.Message); got != "{ alpha: 1, beta: true }" {
		t.Fatalf("expected structured console summary, got %#v", entry)
	}
	rawPreviews := strings.TrimSpace(entry.Metadata[workspaceConsolePreviewsJSONKey])
	if rawPreviews == "" {
		t.Fatalf("expected structured console preview metadata, got %#v", entry.Metadata)
	}
	var previews []map[string]interface{}
	if err := json.Unmarshal([]byte(rawPreviews), &previews); err != nil {
		t.Fatalf("decode console preview metadata failed: %v", err)
	}
	if len(previews) != 1 {
		t.Fatalf("expected one console preview, got %#v", previews)
	}
	if got := interfaceToString(previews[0]["type"]); got != "object" {
		t.Fatalf("expected object preview type, got %#v", previews[0])
	}
}

func TestPersistentRuntimeAsyncGlobalsResolveWithinInvocation(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  return new Promise(function(resolve) {
    const original = {
      nested: { value: 1 },
      list: [1, 2, 3]
    };
    const cloned = structuredClone(original);
    cloned.nested.value = 9;
    cloned.list.push(4);

    const order = [];
    queueMicrotask(function() {
      order.push("micro");
    });

    const cancelled = setTimeout(function() {
      order.push("cancelled");
    }, 5);
    const cleared = clearTimeout(cancelled);
    const clearedAgain = clearTimeout(cancelled);

    setTimeout(function(label) {
      order.push(label);
      resolve({
        success: true,
        data: {
          order: order,
          cleared: cleared,
          cleared_again: clearedAgain,
          original_value: original.nested.value,
          cloned_value: cloned.nested.value,
          original_list_length: original.list.length,
          cloned_list_length: cloned.list.length
        }
      });
    }, 5, "timer");
  });
};
`))

	pluginID := uint(4112)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	resp := handleExecuteRequest(nil, pluginipc.Request{
		Type:             "execute",
		PluginID:         pluginID,
		PluginGeneration: generation,
		PluginName:       "persistent-plugin",
		Action:           "demo.execute",
		ScriptPath:       scriptPath,
		Sandbox: pluginipc.SandboxConfig{
			CurrentAction: "demo.execute",
		},
	}, workerOptions{
		timeoutMs:      3000,
		maxConcurrency: 2,
		maxMemoryMB:    64,
	})
	if !resp.Success || resp.Error != "" {
		t.Fatalf("async globals execute failed: %+v", resp)
	}

	order, ok := resp.Data["order"].([]interface{})
	if !ok || len(order) != 2 {
		t.Fatalf("expected microtask/timer order output, got %#v", resp.Data)
	}
	if got := interfaceToString(order[0]); got != "micro" {
		t.Fatalf("expected first async event to be microtask, got %#v", order)
	}
	if got := interfaceToString(order[1]); got != "timer" {
		t.Fatalf("expected second async event to be timer, got %#v", order)
	}
	if got, ok := interfaceToBool(resp.Data["cleared"]); !ok || !got {
		t.Fatalf("expected clearTimeout() to cancel the timer, got %#v", resp.Data)
	}
	if got, ok := interfaceToBool(resp.Data["cleared_again"]); !ok || got {
		t.Fatalf("expected second clearTimeout() call to report false, got %#v", resp.Data)
	}
	if got := testInterfaceToInt64(resp.Data["original_value"]); got != 1 {
		t.Fatalf("expected structuredClone() to preserve original nested value, got %#v", resp.Data)
	}
	if got := testInterfaceToInt64(resp.Data["cloned_value"]); got != 9 {
		t.Fatalf("expected structuredClone() clone nested value=9, got %#v", resp.Data)
	}
	if got := testInterfaceToInt64(resp.Data["original_list_length"]); got != 3 {
		t.Fatalf("expected original list length=3, got %#v", resp.Data)
	}
	if got := testInterfaceToInt64(resp.Data["cloned_list_length"]); got != 4 {
		t.Fatalf("expected cloned list length=4, got %#v", resp.Data)
	}
}

func TestPersistentRuntimeSetTimeoutWithoutDelayDefaultsToZero(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`globalThis.seed = 1;`))

	pluginID := uint(4113)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := func(code string) pluginipc.Request {
		return pluginipc.Request{
			Type:             "runtime_eval",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "workspace.runtime.eval",
			ScriptPath:       scriptPath,
			RuntimeCode:      code,
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "workspace.runtime.eval",
			},
		}
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	defined := handleRuntimeEvalRequest(nil, req(`let ppp = function(){ return "ok"; }; typeof ppp`), opts)
	if !defined.Success || defined.Error != "" {
		t.Fatalf("expected runtime eval to define ppp, got %+v", defined)
	}
	if got := interfaceToString(defined.Data["value"]); got != "function" {
		t.Fatalf("expected ppp to be a function, got %#v", defined.Data)
	}

	timerResult := handleRuntimeEvalRequest(
		nil,
		req(`new Promise(function(resolve) { setTimeout(function() { resolve(ppp()); }); })`),
		opts,
	)
	if !timerResult.Success || timerResult.Error != "" {
		t.Fatalf("expected setTimeout without delay to resolve, got %+v", timerResult)
	}
	if got := interfaceToString(timerResult.Data["value"]); got != "ok" {
		t.Fatalf("expected omitted delay timer to resolve with ppp() result, got %#v", timerResult.Data)
	}

	afterTimer := handleRuntimeEvalRequest(nil, req(`typeof ppp`), opts)
	if !afterTimer.Success || afterTimer.Error != "" {
		t.Fatalf("expected runtime to stay alive after setTimeout without delay, got %+v", afterTimer)
	}
	if got := interfaceToString(afterTimer.Data["value"]); got != "function" {
		t.Fatalf("expected ppp to persist after timer invocation, got %#v", afterTimer.Data)
	}
}

func TestPersistentRuntimeSetTimeoutWithoutDelayDoesNotResetRuntime(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`globalThis.seed = 1;`))

	pluginID := uint(4114)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := func(code string) pluginipc.Request {
		return pluginipc.Request{
			Type:             "runtime_eval",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "workspace.runtime.eval",
			ScriptPath:       scriptPath,
			RuntimeCode:      code,
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "workspace.runtime.eval",
			},
		}
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	defined := handleRuntimeEvalRequest(nil, req(`let ppp = function(){ return "ok"; }; typeof ppp`), opts)
	if !defined.Success || defined.Error != "" {
		t.Fatalf("expected runtime eval to define ppp, got %+v", defined)
	}

	timerID := handleRuntimeEvalRequest(nil, req(`setTimeout(ppp)`), opts)
	if !timerID.Success || timerID.Error != "" {
		t.Fatalf("expected setTimeout(ppp) to return a timer id instead of panic, got %+v", timerID)
	}
	if got := testInterfaceToInt64(timerID.Data["value"]); got <= 0 {
		t.Fatalf("expected positive timer id from setTimeout(ppp), got %#v", timerID.Data)
	}

	afterTimer := handleRuntimeEvalRequest(nil, req(`typeof ppp`), opts)
	if !afterTimer.Success || afterTimer.Error != "" {
		t.Fatalf("expected runtime to stay alive after bare setTimeout(ppp), got %+v", afterTimer)
	}
	if got := interfaceToString(afterTimer.Data["value"]); got != "function" {
		t.Fatalf("expected ppp to persist after bare setTimeout(ppp), got %#v", afterTimer.Data)
	}
}

func TestPersistentRuntimeSetTimeoutFiresAfterEvalReturns(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`globalThis.fired = [];`))

	pluginID := uint(4115)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := func(code string) pluginipc.Request {
		return pluginipc.Request{
			Type:             "runtime_eval",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "workspace.runtime.eval",
			ScriptPath:       scriptPath,
			RuntimeCode:      code,
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "workspace.runtime.eval",
			},
		}
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	scheduled := handleRuntimeEvalRequest(
		nil,
		req(`globalThis.ppp = function(){ fired.push("a"); return fired.length; }; setTimeout(ppp, 20); "scheduled"`),
		opts,
	)
	if !scheduled.Success || scheduled.Error != "" {
		t.Fatalf("expected timer scheduling eval to succeed, got %+v", scheduled)
	}
	if got := interfaceToString(scheduled.Data["value"]); got != "scheduled" {
		t.Fatalf("expected scheduling eval result to be scheduled, got %#v", scheduled.Data)
	}

	time.Sleep(60 * time.Millisecond)

	after := handleRuntimeEvalRequest(nil, req(`fired.length + ":" + fired.join(",")`), opts)
	if !after.Success || after.Error != "" {
		t.Fatalf("expected post-timer eval to succeed, got %+v", after)
	}
	if got := interfaceToString(after.Data["value"]); got != "1:a" {
		t.Fatalf("expected timer callback to mutate persistent runtime state, got %#v", after.Data)
	}
}

func TestPersistentRuntimeSetTimeoutConsoleLogForwardsWorkspaceAppend(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`globalThis.ready = true;`))

	pluginID := uint(4117)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	appendRequests := make(chan pluginipc.HostRequest, 4)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

		decoder := json.NewDecoder(conn)
		encoder := json.NewEncoder(conn)
		for {
			var req pluginipc.HostRequest
			if err := decoder.Decode(&req); err != nil {
				return
			}
			appendRequests <- req
			if err := encoder.Encode(pluginipc.HostResponse{
				Success: true,
				Status:  200,
				Data: map[string]interface{}{
					"appended": 1,
				},
			}); err != nil {
				t.Errorf("encode host response failed: %v", err)
				return
			}
		}
	}()

	req := func(code string) pluginipc.Request {
		return pluginipc.Request{
			Type:             "runtime_eval",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "workspace.runtime.eval",
			ScriptPath:       scriptPath,
			RuntimeCode:      code,
			Context: &pluginipc.ExecutionContext{
				Metadata: map[string]string{
					"plugin_execution_id":   "pex_runtime_eval",
					"workspace_terminal_line": code,
				},
			},
			HostAPI: &pluginipc.HostAPIConfig{
				Network:     "tcp",
				Address:     listener.Addr().String(),
				AccessToken: "token-demo",
				TimeoutMs:   1000,
			},
			Workspace: &pluginipc.WorkspaceConfig{
				Enabled:    true,
				MaxEntries: 32,
			},
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "workspace.runtime.eval",
			},
		}
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	scheduled := handleRuntimeEvalRequest(
		nil,
		req(`setTimeout(function(){ console.log("a") }, 20); "scheduled"`),
		opts,
	)
	if !scheduled.Success || scheduled.Error != "" {
		t.Fatalf("expected timer scheduling eval to succeed, got %+v", scheduled)
	}

	var appendReq pluginipc.HostRequest
	select {
	case appendReq = <-appendRequests:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async workspace append request")
	}

	if appendReq.Action != "host.workspace.append" {
		t.Fatalf("expected async timer to forward host.workspace.append, got %#v", appendReq)
	}
	if got := strings.TrimSpace(interfaceToString(appendReq.Params["command_id"])); got != "pex_runtime_eval" {
		t.Fatalf("expected command_id=pex_runtime_eval, got %#v", appendReq.Params["command_id"])
	}

	rawEntries, ok := appendReq.Params["entries"].([]interface{})
	if !ok || len(rawEntries) != 1 {
		t.Fatalf("expected one workspace append entry, got %#v", appendReq.Params["entries"])
	}
	entry, ok := rawEntries[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected workspace append entry object, got %#v", rawEntries[0])
	}
	if got := strings.TrimSpace(interfaceToString(entry["message"])); got != "a" {
		t.Fatalf("expected console log message a, got %#v", entry["message"])
	}
	if got := strings.TrimSpace(interfaceToString(entry["channel"])); got != "console" {
		t.Fatalf("expected console channel, got %#v", entry["channel"])
	}
	if got := strings.TrimSpace(interfaceToString(entry["source"])); got != "console.log" {
		t.Fatalf("expected console.log source, got %#v", entry["source"])
	}

	closeReq := handleRuntimeEvalRequest(nil, req(`"done"`), opts)
	if !closeReq.Success || closeReq.Error != "" {
		t.Fatalf("expected follow-up eval to succeed, got %+v", closeReq)
	}
	listener.Close()
	<-serverDone
}

func TestPersistentRuntimeSetTimeoutVariableCallbackForwardsWorkspaceAppend(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(``))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	appendRequests := make(chan pluginipc.HostRequest, 4)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

		decoder := json.NewDecoder(conn)
		encoder := json.NewEncoder(conn)
		for {
			var req pluginipc.HostRequest
			if err := decoder.Decode(&req); err != nil {
				return
			}
			if req.Action == "host.workspace.append" {
				select {
				case appendRequests <- req:
				default:
				}
			}
			if err := encoder.Encode(pluginipc.HostResponse{
				Success: true,
				Status:  200,
				Data: map[string]interface{}{
					"appended": 1,
				},
			}); err != nil {
				t.Errorf("encode host response failed: %v", err)
				return
			}
		}
	}()

	pluginID := uint(4118)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := func(code string) pluginipc.Request {
		return pluginipc.Request{
			Type:             "runtime_eval",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "workspace.runtime.eval",
			ScriptPath:       scriptPath,
			RuntimeCode:      code,
			Context: &pluginipc.ExecutionContext{
				Metadata: map[string]string{
					"plugin_execution_id":    "pex_runtime_eval",
					"workspace_terminal_line": code,
				},
			},
			HostAPI: &pluginipc.HostAPIConfig{
				Network:     "tcp",
				Address:     listener.Addr().String(),
				AccessToken: "token-demo",
				TimeoutMs:   1000,
			},
			Workspace: &pluginipc.WorkspaceConfig{
				Enabled:    true,
				MaxEntries: 32,
			},
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "workspace.runtime.eval",
			},
		}
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	defined := handleRuntimeEvalRequest(
		nil,
		req(`let ppp = function(){ console.log("a") }; typeof ppp`),
		opts,
	)
	if !defined.Success || defined.Error != "" {
		t.Fatalf("expected runtime eval to define ppp, got %+v", defined)
	}
	if got := interfaceToString(defined.Data["value"]); got != "function" {
		t.Fatalf("expected ppp to be a function, got %#v", defined.Data)
	}

	scheduled := handleRuntimeEvalRequest(nil, req(`setTimeout(ppp, 20)`), opts)
	if !scheduled.Success || scheduled.Error != "" {
		t.Fatalf("expected timer scheduling eval to succeed, got %+v", scheduled)
	}
	if got := testInterfaceToInt64(scheduled.Data["value"]); got <= 0 {
		t.Fatalf("expected positive timer id from setTimeout(ppp, 20), got %#v", scheduled.Data)
	}

	var appendReq pluginipc.HostRequest
	select {
	case appendReq = <-appendRequests:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for variable callback async workspace append request")
	}

	if appendReq.Action != "host.workspace.append" {
		t.Fatalf("expected async timer to forward host.workspace.append, got %#v", appendReq)
	}
	if got := strings.TrimSpace(interfaceToString(appendReq.Params["command_id"])); got != "pex_runtime_eval" {
		t.Fatalf("expected command_id=pex_runtime_eval, got %#v", appendReq.Params["command_id"])
	}
	rawEntries, ok := appendReq.Params["entries"].([]interface{})
	if !ok || len(rawEntries) != 1 {
		t.Fatalf("expected one workspace append entry, got %#v", appendReq.Params["entries"])
	}
	entry, ok := rawEntries[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected workspace append entry object, got %#v", rawEntries[0])
	}
	if got := strings.TrimSpace(interfaceToString(entry["message"])); got != "a" {
		t.Fatalf("expected console log message a, got %#v", entry["message"])
	}
	if got := strings.TrimSpace(interfaceToString(entry["channel"])); got != "console" {
		t.Fatalf("expected console channel, got %#v", entry["channel"])
	}

	listener.Close()
	<-serverDone
}

func TestPersistentRuntimeSetTimeoutWithoutDelayForwardsWorkspaceAppend(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(``))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	appendRequests := make(chan pluginipc.HostRequest, 4)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

		decoder := json.NewDecoder(conn)
		encoder := json.NewEncoder(conn)
		for {
			var req pluginipc.HostRequest
			if err := decoder.Decode(&req); err != nil {
				return
			}
			if req.Action == "host.workspace.append" {
				select {
				case appendRequests <- req:
				default:
				}
			}
			if err := encoder.Encode(pluginipc.HostResponse{
				Success: true,
				Status:  200,
				Data: map[string]interface{}{
					"appended": 1,
				},
			}); err != nil {
				t.Errorf("encode host response failed: %v", err)
				return
			}
		}
	}()

	pluginID := uint(4119)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := func(code string) pluginipc.Request {
		return pluginipc.Request{
			Type:             "runtime_eval",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "workspace.runtime.eval",
			ScriptPath:       scriptPath,
			RuntimeCode:      code,
			Context: &pluginipc.ExecutionContext{
				Metadata: map[string]string{
					"plugin_execution_id":    "pex_runtime_eval",
					"workspace_terminal_line": code,
				},
			},
			HostAPI: &pluginipc.HostAPIConfig{
				Network:     "tcp",
				Address:     listener.Addr().String(),
				AccessToken: "token-demo",
				TimeoutMs:   1000,
			},
			Workspace: &pluginipc.WorkspaceConfig{
				Enabled:    true,
				MaxEntries: 32,
			},
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "workspace.runtime.eval",
			},
		}
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	scheduled := handleRuntimeEvalRequest(
		nil,
		req(`setTimeout(function(){ console.log("a") }); "scheduled"`),
		opts,
	)
	if !scheduled.Success || scheduled.Error != "" {
		t.Fatalf("expected delay-free timer scheduling eval to succeed, got %+v", scheduled)
	}
	if got := interfaceToString(scheduled.Data["value"]); got != "scheduled" {
		t.Fatalf("expected scheduling eval result to be scheduled, got %#v", scheduled.Data)
	}

	var appendReq pluginipc.HostRequest
	select {
	case appendReq = <-appendRequests:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for zero-delay async workspace append request")
	}

	if appendReq.Action != "host.workspace.append" {
		t.Fatalf("expected async timer to forward host.workspace.append, got %#v", appendReq)
	}
	rawEntries, ok := appendReq.Params["entries"].([]interface{})
	if !ok || len(rawEntries) != 1 {
		t.Fatalf("expected one workspace append entry, got %#v", appendReq.Params["entries"])
	}
	entry, ok := rawEntries[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected workspace append entry object, got %#v", rawEntries[0])
	}
	if got := strings.TrimSpace(interfaceToString(entry["message"])); got != "a" {
		t.Fatalf("expected console log message a, got %#v", entry["message"])
	}

	listener.Close()
	<-serverDone
}

func TestPersistentRuntimeSetTimeoutArrowCallbackForwardsWorkspaceAppend(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(``))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	appendRequests := make(chan pluginipc.HostRequest, 4)
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

		decoder := json.NewDecoder(conn)
		encoder := json.NewEncoder(conn)
		for {
			var req pluginipc.HostRequest
			if err := decoder.Decode(&req); err != nil {
				return
			}
			if req.Action == "host.workspace.append" {
				select {
				case appendRequests <- req:
				default:
				}
			}
			if err := encoder.Encode(pluginipc.HostResponse{
				Success: true,
				Status:  200,
				Data: map[string]interface{}{
					"appended": 1,
				},
			}); err != nil {
				t.Errorf("encode host response failed: %v", err)
				return
			}
		}
	}()

	pluginID := uint(4120)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := func(code string) pluginipc.Request {
		return pluginipc.Request{
			Type:             "runtime_eval",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "workspace.runtime.eval",
			ScriptPath:       scriptPath,
			RuntimeCode:      code,
			Context: &pluginipc.ExecutionContext{
				Metadata: map[string]string{
					"plugin_execution_id":    "pex_runtime_eval",
					"workspace_terminal_line": code,
				},
			},
			HostAPI: &pluginipc.HostAPIConfig{
				Network:     "tcp",
				Address:     listener.Addr().String(),
				AccessToken: "token-demo",
				TimeoutMs:   1000,
			},
			Workspace: &pluginipc.WorkspaceConfig{
				Enabled:    true,
				MaxEntries: 32,
			},
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "workspace.runtime.eval",
			},
		}
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	scheduled := handleRuntimeEvalRequest(
		nil,
		req(`setTimeout(() => console.log("a"), 20); "scheduled"`),
		opts,
	)
	if !scheduled.Success || scheduled.Error != "" {
		t.Fatalf("expected arrow-function timer scheduling eval to succeed, got %+v", scheduled)
	}
	if got := interfaceToString(scheduled.Data["value"]); got != "scheduled" {
		t.Fatalf("expected scheduling eval result to be scheduled, got %#v", scheduled.Data)
	}

	var appendReq pluginipc.HostRequest
	select {
	case appendReq = <-appendRequests:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for arrow callback async workspace append request")
	}

	if appendReq.Action != "host.workspace.append" {
		t.Fatalf("expected async timer to forward host.workspace.append, got %#v", appendReq)
	}
	rawEntries, ok := appendReq.Params["entries"].([]interface{})
	if !ok || len(rawEntries) != 1 {
		t.Fatalf("expected one workspace append entry, got %#v", appendReq.Params["entries"])
	}
	entry, ok := rawEntries[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected workspace append entry object, got %#v", rawEntries[0])
	}
	if got := strings.TrimSpace(interfaceToString(entry["message"])); got != "a" {
		t.Fatalf("expected console log message a, got %#v", entry["message"])
	}

	listener.Close()
	<-serverDone
}

func TestPersistentRuntimeQueueMicrotaskFlushesWithoutPromise(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`globalThis.micro = [];`))

	pluginID := uint(4116)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	req := func(code string) pluginipc.Request {
		return pluginipc.Request{
			Type:             "runtime_eval",
			PluginID:         pluginID,
			PluginGeneration: generation,
			PluginName:       "persistent-plugin",
			Action:           "workspace.runtime.eval",
			ScriptPath:       scriptPath,
			RuntimeCode:      code,
			Sandbox: pluginipc.SandboxConfig{
				CurrentAction: "workspace.runtime.eval",
			},
		}
	}

	opts := workerOptions{timeoutMs: 1000, maxConcurrency: 1, maxMemoryMB: 32}

	result := handleRuntimeEvalRequest(
		nil,
		req(`queueMicrotask(function(){ micro.push("tick"); }); micro.length`),
		opts,
	)
	if !result.Success || result.Error != "" {
		t.Fatalf("expected microtask eval to succeed, got %+v", result)
	}
	if got := testInterfaceToInt64(result.Data["value"]); got != 0 {
		t.Fatalf("expected immediate expression result before microtask mutation, got %#v", result.Data)
	}

	after := handleRuntimeEvalRequest(nil, req(`micro.length + ":" + micro.join(",")`), opts)
	if !after.Success || after.Error != "" {
		t.Fatalf("expected microtask follow-up eval to succeed, got %+v", after)
	}
	if got := interfaceToString(after.Data["value"]); got != "1:tick" {
		t.Fatalf("expected queueMicrotask to flush before follow-up eval, got %#v", after.Data)
	}
}

func TestPersistentRuntimeWorkerRequestRunsInChildRuntime(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	workerPath := filepath.Join(rootDir, "child.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = async function() {
  const worker = new Worker("./child.js");
  const first = await worker.request({ value: 10 });
  const second = await worker.request({ value: 21 });
  return {
    success: true,
    data: {
      first: first.doubled,
      second: second.doubled,
      calls: second.calls
    }
  };
};
`))
	mustWriteFile(t, workerPath, []byte(`
let calls = 0;

onmessage = async function(event) {
  calls += 1;
  return {
    doubled: event.data.value * 2,
    calls: calls
  };
};
`))

	pluginID := uint(4109)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	resp := handleExecuteRequest(nil, pluginipc.Request{
		Type:             "execute",
		PluginID:         pluginID,
		PluginGeneration: generation,
		PluginName:       "persistent-plugin",
		Action:           "demo.execute",
		ScriptPath:       scriptPath,
		Sandbox: pluginipc.SandboxConfig{
			CurrentAction: "demo.execute",
		},
	}, workerOptions{
		timeoutMs:      3000,
		maxConcurrency: 4,
		maxMemoryMB:    64,
	})
	if !resp.Success || resp.Error != "" {
		t.Fatalf("worker request execute failed: %+v", resp)
	}
	if got := testInterfaceToInt64(resp.Data["first"]); got != 20 {
		t.Fatalf("expected first worker result=20, got %#v", resp.Data)
	}
	if got := testInterfaceToInt64(resp.Data["second"]); got != 42 {
		t.Fatalf("expected second worker result=42, got %#v", resp.Data)
	}
	if got := testInterfaceToInt64(resp.Data["calls"]); got != 2 {
		t.Fatalf("expected child worker runtime to preserve state across requests, got %#v", resp.Data)
	}
}

func TestPersistentRuntimeWorkerPostMessageDeliversParentEvents(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	workerPath := filepath.Join(rootDir, "child.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  return new Promise(function(resolve, reject) {
    const worker = new Worker("./child.js");
    worker.onerror = function(event) {
      reject(new Error(event.error));
    };
    worker.onmessage = function(event) {
      resolve({
        success: true,
        data: {
          value: event.data.value,
          type: event.type,
          worker_id: event.worker_id
        }
      });
    };
    worker.postMessage({ value: 6 });
  });
};
`))
	mustWriteFile(t, workerPath, []byte(`
onmessage = function(event) {
  postMessage({ value: event.data.value + 1 });
};
`))

	pluginID := uint(4110)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	resp := handleExecuteRequest(nil, pluginipc.Request{
		Type:             "execute",
		PluginID:         pluginID,
		PluginGeneration: generation,
		PluginName:       "persistent-plugin",
		Action:           "demo.execute",
		ScriptPath:       scriptPath,
		Sandbox: pluginipc.SandboxConfig{
			CurrentAction: "demo.execute",
		},
	}, workerOptions{
		timeoutMs:      3000,
		maxConcurrency: 4,
		maxMemoryMB:    64,
	})
	if !resp.Success || resp.Error != "" {
		t.Fatalf("worker postMessage execute failed: %+v", resp)
	}
	if got := testInterfaceToInt64(resp.Data["value"]); got != 7 {
		t.Fatalf("expected parent onmessage callback payload value=7, got %#v", resp.Data)
	}
	if got := interfaceToString(resp.Data["type"]); got != "message" {
		t.Fatalf("expected message event type, got %#v", resp.Data)
	}
	if got := interfaceToString(resp.Data["worker_id"]); got == "" {
		t.Fatalf("expected worker event to expose worker_id, got %#v", resp.Data)
	}
}

func TestPersistentRuntimeWorkerStorageIsReadOnly(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	workerPath := filepath.Join(rootDir, "child.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"persistent-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = async function() {
  const worker = new Worker("./child.js");
  const result = await worker.request({});
  return {
    success: true,
    data: result
  };
};
`))
	mustWriteFile(t, workerPath, []byte(`
onmessage = function() {
  return {
    write_ok: Plugin.storage.set("child", "1"),
    has_child_value: typeof Plugin.storage.get("child") !== "undefined"
  };
};
`))

	pluginID := uint(4111)
	generation := uint(1)
	t.Cleanup(func() {
		globalPersistentPluginRuntimeManager.dispose(pluginID, generation)
	})

	resp := handleExecuteRequest(nil, pluginipc.Request{
		Type:             "execute",
		PluginID:         pluginID,
		PluginGeneration: generation,
		PluginName:       "persistent-plugin",
		Action:           "demo.execute",
		ScriptPath:       scriptPath,
		Sandbox: pluginipc.SandboxConfig{
			CurrentAction: "demo.execute",
		},
	}, workerOptions{
		timeoutMs:      3000,
		maxConcurrency: 4,
		maxMemoryMB:    64,
	})
	if !resp.Success || resp.Error != "" {
		t.Fatalf("worker storage execute failed: %+v", resp)
	}
	if got, ok := interfaceToBool(resp.Data["write_ok"]); !ok || got {
		t.Fatalf("expected child worker storage writes to be denied, got %#v", resp.Data)
	}
	if got, ok := interfaceToBool(resp.Data["has_child_value"]); !ok || got {
		t.Fatalf("expected child worker storage reads to stay empty after denied write, got %#v", resp.Data)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsCompletionPathPrefix(values []string, prefix string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
