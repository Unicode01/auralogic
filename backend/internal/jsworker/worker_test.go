package jsworker

import (
	"context"
	"encoding/json"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"auralogic/internal/pluginipc"
	"github.com/dop251/goja"
)

func TestRunScriptFunctionWithStorageExecutesCurrentJSMarketBootstrapBundle(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "..", "plugins", "js_market", "dist", "index.js")
	if _, err := os.Stat(scriptPath); err != nil {
		t.Skipf("skipping js_market bootstrap bundle test because generated dist asset is unavailable: %v", err)
	}

	pluginRoot := filepath.Dir(filepath.Dir(scriptPath))
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   pluginRoot,
		DataRoot:   filepath.Join(t.TempDir(), "data"),
		PluginID:   13,
		PluginName: "js_market",
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:           "hook.execute",
		AllowHookExecute:        true,
		AllowFrontendExtensions: true,
		AllowExecuteAPI:         true,
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{
			"hook.execute",
			map[string]string{
				"hook":    "frontend.bootstrap",
				"payload": `{"area":"admin","full_path":"/admin/plugins","path":"/admin/plugins","query_params":null,"query_string":"","slot":"bootstrap"}`,
			},
			map[string]interface{}{
				"metadata": map[string]string{
					"locale": "zh-CN",
				},
			},
			map[string]interface{}{},
			sandboxCfg,
		},
		workerOptions{
			timeoutMs:      3000,
			maxConcurrency: 1,
			maxMemoryMB:    128,
		},
		sandboxCfg,
		nil,
		3*time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}

	extensionsRaw, ok := responseData["frontend_extensions"]
	if !ok {
		t.Fatalf("expected frontend_extensions in response data, got %#v", responseData)
	}
	extensions, ok := extensionsRaw.([]interface{})
	if !ok || len(extensions) == 0 {
		t.Fatalf("expected non-empty frontend_extensions, got %#v", extensionsRaw)
	}
}

func TestCommonJSLoaderResolvePackageModulePath(t *testing.T) {
	rootDir := t.TempDir()
	entryPath := filepath.Join(rootDir, "index.js")
	packageRoot := filepath.Join(rootDir, "node_modules", "@auralogic", "plugin-sdk")
	packageDist := filepath.Join(packageRoot, "dist")

	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, entryPath, []byte(`module.exports = {};`))
	mustWriteFile(t, filepath.Join(packageRoot, "package.json"), []byte(`{"name":"@auralogic/plugin-sdk","main":"dist/index.js"}`))
	mustWriteFile(t, filepath.Join(packageDist, "index.js"), []byte(`module.exports = { ok: true };`))

	loader := newCommonJSLoader(goja.New(), entryPath)
	resolvedPath, err := loader.resolveModulePath("", "@auralogic/plugin-sdk")
	if err != nil {
		t.Fatalf("resolveModulePath returned error: %v", err)
	}

	expectedPath := filepath.Clean(filepath.Join(packageDist, "index.js"))
	if filepath.Clean(resolvedPath) != expectedPath {
		t.Fatalf("expected %s, got %s", expectedPath, resolvedPath)
	}
}

func TestCommonJSLoaderRejectsSymlinkEscapingPluginRoot(t *testing.T) {
	rootDir := t.TempDir()
	entryPath := filepath.Join(rootDir, "index.js")
	outsidePath := filepath.Join(t.TempDir(), "outside.js")
	escapedPath := filepath.Join(rootDir, "escape.js")

	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, entryPath, []byte(`module.exports = {};`))
	mustWriteFile(t, outsidePath, []byte(`module.exports = { escaped: true };`))
	if err := os.Symlink(outsidePath, escapedPath); err != nil {
		t.Skipf("symlink is unavailable in current environment: %v", err)
	}

	loader := newCommonJSLoader(goja.New(), entryPath)
	if _, err := loader.resolveModulePath("", "./escape"); err == nil {
		t.Fatalf("expected symlinked module outside plugin root to be rejected")
	}
}

func TestCommonJSLoaderRejectsSymlinkedPackageDirectoryOutsidePluginRoot(t *testing.T) {
	rootDir := t.TempDir()
	entryPath := filepath.Join(rootDir, "index.js")
	nodeModulesDir := filepath.Join(rootDir, "node_modules")
	externalPackageRoot := filepath.Join(t.TempDir(), "package")
	linkedPackageRoot := filepath.Join(nodeModulesDir, "escape-pkg")

	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, entryPath, []byte(`module.exports = {};`))
	mustWriteFile(t, filepath.Join(externalPackageRoot, "package.json"), []byte(`{"name":"escape-pkg","main":"index.js"}`))
	mustWriteFile(t, filepath.Join(externalPackageRoot, "index.js"), []byte(`module.exports = { escaped: true };`))
	if err := os.MkdirAll(nodeModulesDir, 0o755); err != nil {
		t.Fatalf("create node_modules failed: %v", err)
	}
	if err := os.Symlink(externalPackageRoot, linkedPackageRoot); err != nil {
		t.Skipf("directory symlink is unavailable in current environment: %v", err)
	}

	loader := newCommonJSLoader(goja.New(), entryPath)
	if _, err := loader.resolveModulePath("", "escape-pkg"); err == nil {
		t.Fatalf("expected symlinked package outside plugin root to be rejected")
	}
}

func TestPluginFSUsageToMapUsesSnakeCaseKeys(t *testing.T) {
	usage := pluginFSUsage{
		FileCount:  2,
		TotalBytes: 128,
		MaxFiles:   2048,
		MaxBytes:   4096,
	}

	result := pluginFSUsageToMap(usage)

	if got, ok := result["file_count"]; !ok || got != 2 {
		t.Fatalf("expected file_count=2, got %#v", result)
	}
	if got, ok := result["total_bytes"]; !ok || got != int64(128) {
		t.Fatalf("expected total_bytes=128, got %#v", result)
	}
	if got, ok := result["max_files"]; !ok || got != 2048 {
		t.Fatalf("expected max_files=2048, got %#v", result)
	}
	if got, ok := result["max_bytes"]; !ok || got != int64(4096) {
		t.Fatalf("expected max_bytes=4096, got %#v", result)
	}
	if _, ok := result["FileCount"]; ok {
		t.Fatalf("unexpected Go-style key in result: %#v", result)
	}
}

func TestClampPluginHTTPTimeoutMs(t *testing.T) {
	if got := clampPluginHTTPTimeoutMs(0, 30000); got != defaultPluginHTTPTimeoutMs {
		t.Fatalf("expected default timeout %d, got %d", defaultPluginHTTPTimeoutMs, got)
	}
	if got := clampPluginHTTPTimeoutMs(50000, 1200); got != 1200 {
		t.Fatalf("expected timeout to clamp to runtime limit, got %d", got)
	}
	if got := clampPluginHTTPTimeoutMs(10, 30000); got != 100 {
		t.Fatalf("expected timeout floor 100ms, got %d", got)
	}
}

func TestNormalizeExecutionMemoryMonitorInterval(t *testing.T) {
	if got := normalizeExecutionMemoryMonitorInterval(0); got != memoryMonitorMinInterval {
		t.Fatalf("expected zero interval to clamp to min interval %s, got %s", memoryMonitorMinInterval, got)
	}
	if got := normalizeExecutionMemoryMonitorInterval(time.Millisecond); got != memoryMonitorMinInterval {
		t.Fatalf("expected tiny interval to clamp to min interval %s, got %s", memoryMonitorMinInterval, got)
	}
	if got := normalizeExecutionMemoryMonitorInterval(time.Second); got != memoryMonitorMaxInterval {
		t.Fatalf("expected large interval to clamp to max interval %s, got %s", memoryMonitorMaxInterval, got)
	}
	if got := normalizeExecutionMemoryMonitorInterval(120 * time.Millisecond); got != 120*time.Millisecond {
		t.Fatalf("expected in-range interval to remain unchanged, got %s", got)
	}
}

func TestNewPluginHTTPClientReusesSharedTransport(t *testing.T) {
	clientA := newPluginHTTPClient(time.Second)
	clientB := newPluginHTTPClient(2 * time.Second)
	if clientA.Transport != sharedPluginHTTPTransport {
		t.Fatalf("expected clientA to reuse shared transport")
	}
	if clientB.Transport != sharedPluginHTTPTransport {
		t.Fatalf("expected clientB to reuse shared transport")
	}
}

func TestValidatePluginHTTPURLBlocksLocalTargets(t *testing.T) {
	blockedCases := []string{
		"http://localhost:8080",
		"http://service.local",
		"http://127.0.0.1:8080",
		"http://10.0.0.1",
		"http://100.64.0.1",
	}
	for _, raw := range blockedCases {
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse %s failed: %v", raw, err)
		}
		if err := validatePluginHTTPURL(parsed); err == nil {
			t.Fatalf("expected %s to be blocked", raw)
		}
	}

	allowed, err := url.Parse("https://example.com/resource")
	if err != nil {
		t.Fatalf("parse allowed url failed: %v", err)
	}
	if err := validatePluginHTTPURL(allowed); err != nil {
		t.Fatalf("expected public url to be allowed, got %v", err)
	}
}

func TestIsBlockedPluginHTTPIP(t *testing.T) {
	if !isBlockedPluginHTTPIP(netip.MustParseAddr("127.0.0.1")) {
		t.Fatalf("expected loopback ip to be blocked")
	}
	if !isBlockedPluginHTTPIP(netip.MustParseAddr("10.0.0.5")) {
		t.Fatalf("expected private ip to be blocked")
	}
	if !isBlockedPluginHTTPIP(netip.MustParseAddr("100.64.0.1")) {
		t.Fatalf("expected cgnat ip to be blocked")
	}
	if !isBlockedPluginHTTPIP(netip.MustParseAddr("::ffff:100.64.0.1")) {
		t.Fatalf("expected ipv4-mapped cgnat ip to be blocked")
	}
	if isBlockedPluginHTTPIP(netip.MustParseAddr("8.8.8.8")) {
		t.Fatalf("expected public ip to be allowed")
	}
}

func TestRunScriptFunctionWithStorageUsesLiveSandboxObject(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function(action, params, context, config, sandbox) {
  const before = sandbox.storageAccessMode;
  const sameSandbox = sandbox === globalThis.sandbox;
  const stored = Plugin.storage.get("demo");
  const afterRead = sandbox.storageAccessMode;
  Plugin.storage.set("demo", "next");
  const afterWrite = sandbox.storageAccessMode;
  return {
    success: true,
    data: {
      current_action: sandbox.currentAction,
      declared_storage_access_mode: sandbox.declaredStorageAccessMode,
      mapped_storage_access_mode: sandbox.executeActionStorage["template.page.save"],
      before,
      after_read: afterRead,
      after_write: afterWrite,
      same_sandbox: sameSandbox,
      stored
    }
  };
};
`))

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          128,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:         "template.page.save",
		DeclaredStorageAccess: storageAccessWrite,
		ExecuteActionStorage: map[string]string{
			"template.page.save": storageAccessWrite,
		},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, storageSnapshot, storageChanged, storageAccessMode, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{
			"template.page.save",
			map[string]string{},
			map[string]interface{}{},
			map[string]interface{}{},
			sandboxCfg,
		},
		opts,
		sandboxCfg,
		nil,
		time.Second,
		map[string]string{"demo": "value"},
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	if !storageChanged {
		t.Fatalf("expected storageChanged=true")
	}
	if got := storageAccessMode; got != storageAccessWrite {
		t.Fatalf("expected storage access mode write, got %q", got)
	}
	if got := storageSnapshot["demo"]; got != "next" {
		t.Fatalf("expected updated storage snapshot demo=next, got %#v", storageSnapshot)
	}

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["current_action"]); got != "template.page.save" {
		t.Fatalf("expected current_action, got %#v", responseData)
	}
	if got := interfaceToString(responseData["declared_storage_access_mode"]); got != storageAccessWrite {
		t.Fatalf("expected declared storage access write, got %#v", responseData)
	}
	if got := interfaceToString(responseData["mapped_storage_access_mode"]); got != storageAccessWrite {
		t.Fatalf("expected executeActionStorage mapping, got %#v", responseData)
	}
	if got := interfaceToString(responseData["before"]); got != storageAccessNone {
		t.Fatalf("expected initial storage access none, got %#v", responseData)
	}
	if got := interfaceToString(responseData["after_read"]); got != storageAccessRead {
		t.Fatalf("expected storage access read after get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["after_write"]); got != storageAccessWrite {
		t.Fatalf("expected storage access write after set, got %#v", responseData)
	}
	if same, ok := interfaceToBool(responseData["same_sandbox"]); !ok || !same {
		t.Fatalf("expected sandbox argument to reference live global sandbox, got %#v", responseData)
	}
	if got := interfaceToString(responseData["stored"]); got != "value" {
		t.Fatalf("expected stored demo=value, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageProvidesURLSearchParams(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const params = new URLSearchParams("?tab=timeline&tag=alpha");
  params.set("order_id", "42");
  params.append("tag", "beta");
  return {
    success: true,
    data: {
      query: params.toString(),
      order_id: params.get("order_id"),
      has_order_id: params.has("order_id"),
      tags: params.getAll("tag")
    }
  };
};
`))

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          128,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction: "test.url_search_params",
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		nil,
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["order_id"]); got != "42" {
		t.Fatalf("expected order_id=42, got %#v", responseData)
	}
	if ok, exists := interfaceToBool(responseData["has_order_id"]); !exists || !ok {
		t.Fatalf("expected has_order_id=true, got %#v", responseData)
	}
	if got := interfaceToString(responseData["query"]); got != "tab=timeline&tag=alpha&order_id=42&tag=beta" {
		t.Fatalf("unexpected query string: %#v", responseData)
	}
	tags, ok := responseData["tags"].([]interface{})
	if !ok || len(tags) != 2 {
		t.Fatalf("expected two tags, got %#v", responseData)
	}
	if got := interfaceToString(tags[0]); got != "alpha" {
		t.Fatalf("expected first tag alpha, got %#v", tags)
	}
	if got := interfaceToString(tags[1]); got != "beta" {
		t.Fatalf("expected second tag beta, got %#v", tags)
	}
}

func TestRunScriptFunctionWithStorageProvidesWebEncodingGlobals(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const encoded = new TextEncoder().encode("Hello, 世界");
  const decoded = new TextDecoder().decode(encoded);
  const buffer = new Uint8Array(16);
  const encodeIntoResult = new TextEncoder().encodeInto("Hi!", buffer);
  return {
    success: true,
    data: {
      has_text_encoder: typeof TextEncoder === "function",
      has_text_decoder: typeof TextDecoder === "function",
      has_atob: typeof atob === "function",
      has_btoa: typeof btoa === "function",
      decoded: decoded,
      encoded_length: encoded.length,
      encoded_bytes: Array.prototype.slice.call(encoded),
      encode_into_read: encodeIntoResult.read,
      encode_into_written: encodeIntoResult.written,
      encode_into_bytes: Array.prototype.slice.call(buffer).slice(0, encodeIntoResult.written),
      base64: btoa("Hello!"),
      roundtrip: atob("SGVsbG8h")
    }
  };
};
`))

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          128,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction: "test.web_encoding",
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		nil,
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if ok, exists := interfaceToBool(responseData["has_text_encoder"]); !exists || !ok {
		t.Fatalf("expected TextEncoder, got %#v", responseData)
	}
	if ok, exists := interfaceToBool(responseData["has_text_decoder"]); !exists || !ok {
		t.Fatalf("expected TextDecoder, got %#v", responseData)
	}
	if ok, exists := interfaceToBool(responseData["has_atob"]); !exists || !ok {
		t.Fatalf("expected atob, got %#v", responseData)
	}
	if ok, exists := interfaceToBool(responseData["has_btoa"]); !exists || !ok {
		t.Fatalf("expected btoa, got %#v", responseData)
	}
	if got := interfaceToString(responseData["decoded"]); got != "Hello, 世界" {
		t.Fatalf("expected decoded text, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["encoded_length"]); got != 13 {
		t.Fatalf("expected UTF-8 encoded length 13, got %#v", responseData)
	}
	encodedBytes, ok := responseData["encoded_bytes"].([]interface{})
	if !ok || len(encodedBytes) != 13 {
		t.Fatalf("expected encoded bytes, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["encode_into_read"]); got != 3 {
		t.Fatalf("expected encodeInto read=3, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["encode_into_written"]); got != 3 {
		t.Fatalf("expected encodeInto written=3, got %#v", responseData)
	}
	encodeIntoBytes, ok := responseData["encode_into_bytes"].([]interface{})
	if !ok || len(encodeIntoBytes) != 3 {
		t.Fatalf("expected encodeInto bytes, got %#v", responseData)
	}
	if got := testInterfaceToInt64(encodeIntoBytes[0]); got != 72 {
		t.Fatalf("expected first encodeInto byte 72, got %#v", encodeIntoBytes)
	}
	if got := testInterfaceToInt64(encodeIntoBytes[1]); got != 105 {
		t.Fatalf("expected second encodeInto byte 105, got %#v", encodeIntoBytes)
	}
	if got := testInterfaceToInt64(encodeIntoBytes[2]); got != 33 {
		t.Fatalf("expected third encodeInto byte 33, got %#v", encodeIntoBytes)
	}
	if got := interfaceToString(responseData["base64"]); got != "SGVsbG8h" {
		t.Fatalf("expected btoa output, got %#v", responseData)
	}
	if got := interfaceToString(responseData["roundtrip"]); got != "Hello!" {
		t.Fatalf("expected atob output, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageProvidesStructuredClone(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const original = {
    nested: { value: 1 },
    list: [1, 2, 3]
  };
  const cloned = structuredClone(original);
  cloned.nested.value = 9;
  cloned.list.push(4);
  return {
    success: true,
    data: {
      has_structured_clone: typeof structuredClone === "function",
      original_value: original.nested.value,
      cloned_value: cloned.nested.value,
      original_list_length: original.list.length,
      cloned_list_length: cloned.list.length
    }
  };
};
`))

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          128,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction: "test.structured_clone",
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		nil,
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
		nil,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if ok, exists := interfaceToBool(responseData["has_structured_clone"]); !exists || !ok {
		t.Fatalf("expected structuredClone, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["original_value"]); got != 1 {
		t.Fatalf("expected original nested value to stay 1, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["cloned_value"]); got != 9 {
		t.Fatalf("expected cloned nested value to become 9, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["original_list_length"]); got != 3 {
		t.Fatalf("expected original list length=3, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["cloned_list_length"]); got != 4 {
		t.Fatalf("expected cloned list length=4, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageProvidesAsyncRuntimeGlobals(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  return new Promise(function(resolve) {
    const original = {
      nested: { value: 1 },
      list: [1, 2, 3]
    };
    const cloned = structuredClone(original);
    cloned.nested.value = 7;
    cloned.list.push(4);

    const order = [];
    queueMicrotask(function() {
      order.push("micro");
    });

    const cancelled = setTimeout(function() {
      order.push("cancelled");
    }, 5);
    const cleared = clearTimeout(cancelled);

    setTimeout(function(label) {
      order.push(label);
      resolve({
        success: true,
        data: {
          order: order,
          cleared: cleared,
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

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          128,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction: "test.async_globals",
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		nil,
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
		nil,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	order, ok := responseData["order"].([]interface{})
	if !ok || len(order) != 2 {
		t.Fatalf("expected microtask/timer order output, got %#v", responseData)
	}
	if got := interfaceToString(order[0]); got != "micro" {
		t.Fatalf("expected first async event to be microtask, got %#v", order)
	}
	if got := interfaceToString(order[1]); got != "timer" {
		t.Fatalf("expected second async event to be timer, got %#v", order)
	}
	if got, ok := interfaceToBool(responseData["cleared"]); !ok || !got {
		t.Fatalf("expected clearTimeout() to cancel the timer, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["original_value"]); got != 1 {
		t.Fatalf("expected original nested value to stay 1, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["cloned_value"]); got != 7 {
		t.Fatalf("expected cloned nested value to become 7, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["original_list_length"]); got != 3 {
		t.Fatalf("expected original list length=3, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["cloned_list_length"]); got != 4 {
		t.Fatalf("expected cloned list length=4, got %#v", responseData)
	}
}

func TestPerformPluginHostRequestUsesSocketBridge(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.AccessToken != "token-demo" {
			t.Errorf("expected access token token-demo, got %q", req.AccessToken)
		}
		if req.Action != "host.order.get" {
			t.Errorf("expected action host.order.get, got %q", req.Action)
		}
		if got := testInterfaceToInt64(req.Params["id"]); got != 42 {
			t.Errorf("expected params.id=42, got %#v", got)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"ok":     true,
				"action": req.Action,
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	result, err := performPluginHostRequest(&pluginipc.HostAPIConfig{
		Network:     "tcp",
		Address:     listener.Addr().String(),
		AccessToken: "token-demo",
		TimeoutMs:   1000,
	}, "host.order.get", map[string]interface{}{
		"id": 42,
	})
	if err != nil {
		t.Fatalf("performPluginHostRequest returned error: %v", err)
	}
	<-done

	if ok, exists := result["ok"].(bool); !exists || !ok {
		t.Fatalf("expected ok=true, got %#v", result)
	}
	if got := result["action"]; got != "host.order.get" {
		t.Fatalf("expected action echoed back, got %#v", result)
	}
}

func TestPerformPluginHostRequestReusesAttachedSessionConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))

		decoder := json.NewDecoder(conn)
		encoder := json.NewEncoder(conn)
		for index := 0; index < 2; index++ {
			var req pluginipc.HostRequest
			if err := decoder.Decode(&req); err != nil {
				t.Errorf("decode host request #%d failed: %v", index+1, err)
				return
			}
			if req.AccessToken != "token-demo" {
				t.Errorf("expected access token token-demo, got %q", req.AccessToken)
			}
			if err := encoder.Encode(pluginipc.HostResponse{
				Success: true,
				Status:  200,
				Data: map[string]interface{}{
					"seq": index + 1,
				},
			}); err != nil {
				t.Errorf("encode host response #%d failed: %v", index+1, err)
				return
			}
		}
	}()

	hostCfg := &pluginipc.HostAPIConfig{
		Network:     "tcp",
		Address:     listener.Addr().String(),
		AccessToken: "token-demo",
		TimeoutMs:   1000,
	}
	release := attachPluginHostSession(hostCfg)
	defer release()

	first, err := performPluginHostRequest(hostCfg, "host.order.get", map[string]interface{}{"id": 1})
	if err != nil {
		t.Fatalf("first performPluginHostRequest returned error: %v", err)
	}
	if got := testInterfaceToInt64(first["seq"]); got != 1 {
		t.Fatalf("expected first response seq=1, got %#v", first)
	}

	second, err := performPluginHostRequest(hostCfg, "host.order.get", map[string]interface{}{"id": 2})
	if err != nil {
		t.Fatalf("second performPluginHostRequest returned error: %v", err)
	}
	if got := testInterfaceToInt64(second["seq"]); got != 2 {
		t.Fatalf("expected second response seq=2, got %#v", second)
	}

	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatalf("timed out waiting for reused host session requests to complete")
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginOrderAssignTrackingHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.order.assignTracking({ id: 42, tracking_no: "TRACK-42" });
  return {
    success: true,
    data: {
      action: result.action,
      id: result.id,
      tracking_no: result.tracking_no,
      status: result.status
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.order.assign_tracking" {
			t.Errorf("expected action host.order.assign_tracking, got %q", req.Action)
		}
		if got := testInterfaceToInt64(req.Params["id"]); got != 42 {
			t.Errorf("expected params.id=42, got %#v", got)
		}
		if got := interfaceToString(req.Params["tracking_no"]); got != "TRACK-42" {
			t.Errorf("expected params.tracking_no=TRACK-42, got %#v", got)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action":      req.Action,
				"id":          req.Params["id"],
				"tracking_no": req.Params["tracking_no"],
				"status":      "shipped",
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.order.assign_tracking",
		GrantedPermissions: []string{"host.order.assign_tracking"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.order.assign_tracking" {
		t.Fatalf("expected action host.order.assign_tracking, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["id"]); got != 42 {
		t.Fatalf("expected id=42, got %#v", responseData)
	}
	if got := interfaceToString(responseData["tracking_no"]); got != "TRACK-42" {
		t.Fatalf("expected tracking_no=TRACK-42, got %#v", responseData)
	}
	if got := interfaceToString(responseData["status"]); got != "shipped" {
		t.Fatalf("expected status=shipped, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginOrderRequestResubmitHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.order.requestResubmit({ id: 42, reason: "Need updated address" });
  return {
    success: true,
    data: {
      action: result.action,
      id: result.id,
      status: result.status
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.order.request_resubmit" {
			t.Errorf("expected action host.order.request_resubmit, got %q", req.Action)
		}
		if got := testInterfaceToInt64(req.Params["id"]); got != 42 {
			t.Errorf("expected params.id=42, got %#v", got)
		}
		if got := interfaceToString(req.Params["reason"]); got != "Need updated address" {
			t.Errorf("expected params.reason, got %#v", got)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action": req.Action,
				"id":     req.Params["id"],
				"status": "need_resubmit",
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.order.request_resubmit",
		GrantedPermissions: []string{"host.order.request_resubmit"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.order.request_resubmit" {
		t.Fatalf("expected action host.order.request_resubmit, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["id"]); got != 42 {
		t.Fatalf("expected id=42, got %#v", responseData)
	}
	if got := interfaceToString(responseData["status"]); got != "need_resubmit" {
		t.Fatalf("expected status=need_resubmit, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginProductHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.product.get({ id: 5 });
  return {
    success: true,
    data: {
      action: result.action,
      entity: result.entity,
      id: result.id
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.product.get" {
			t.Errorf("expected action host.product.get, got %q", req.Action)
		}
		if got := testInterfaceToInt64(req.Params["id"]); got != 5 {
			t.Errorf("expected params.id=5, got %#v", got)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action": req.Action,
				"entity": "product",
				"id":     req.Params["id"],
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.product",
		GrantedPermissions: []string{"host.product.read"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.product.get" {
		t.Fatalf("expected action host.product.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["entity"]); got != "product" {
		t.Fatalf("expected entity product, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["id"]); got != 5 {
		t.Fatalf("expected id=5, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginTicketReplyHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.ticket.reply({ id: 9, content: "Reply from worker" });
  return {
    success: true,
    data: {
      action: result.action,
      ticket_id: result.ticket_id,
      status: result.status
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.ticket.reply" {
			t.Errorf("expected action host.ticket.reply, got %q", req.Action)
		}
		if got := testInterfaceToInt64(req.Params["id"]); got != 9 {
			t.Errorf("expected params.id=9, got %#v", got)
		}
		if got := interfaceToString(req.Params["content"]); got != "Reply from worker" {
			t.Errorf("expected params.content, got %#v", got)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action":    req.Action,
				"ticket_id": req.Params["id"],
				"status":    "processing",
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.ticket.reply",
		GrantedPermissions: []string{"host.ticket.reply"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.ticket.reply" {
		t.Fatalf("expected action host.ticket.reply, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["ticket_id"]); got != 9 {
		t.Fatalf("expected ticket_id=9, got %#v", responseData)
	}
	if got := interfaceToString(responseData["status"]); got != "processing" {
		t.Fatalf("expected status=processing, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginAnnouncementHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.announcement.get({ id: 11 });
  return {
    success: true,
    data: {
      action: result.action,
      entity: result.entity,
      id: result.id
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.announcement.get" {
			t.Errorf("expected action host.announcement.get, got %q", req.Action)
		}
		if got := testInterfaceToInt64(req.Params["id"]); got != 11 {
			t.Errorf("expected params.id=11, got %#v", got)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action": req.Action,
				"entity": "announcement",
				"id":     req.Params["id"],
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.announcement",
		GrantedPermissions: []string{"host.announcement.read"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.announcement.get" {
		t.Fatalf("expected action host.announcement.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["entity"]); got != "announcement" {
		t.Fatalf("expected entity announcement, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["id"]); got != 11 {
		t.Fatalf("expected id=11, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginPaymentMethodHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.paymentMethod.get({ id: 13 });
  return {
    success: true,
    data: {
      action: result.action,
      entity: result.entity,
      id: result.id
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.payment_method.get" {
			t.Errorf("expected action host.payment_method.get, got %q", req.Action)
		}
		if got := testInterfaceToInt64(req.Params["id"]); got != 13 {
			t.Errorf("expected params.id=13, got %#v", got)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action": req.Action,
				"entity": "payment_method",
				"id":     req.Params["id"],
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.payment_method",
		GrantedPermissions: []string{"host.payment_method.read"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.payment_method.get" {
		t.Fatalf("expected action host.payment_method.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["entity"]); got != "payment_method" {
		t.Fatalf("expected entity payment_method, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["id"]); got != 13 {
		t.Fatalf("expected id=13, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginMarketHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.market.source.list({ source_id: "official" });
  return {
    success: true,
    data: {
      action: result.action,
      source_id: result.source_id
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.market.source.list" {
			t.Errorf("expected action host.market.source.list, got %q", req.Action)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action":    req.Action,
				"source_id": "official",
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.market",
		GrantedPermissions: []string{"host.market.source.read"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.market.source.list" {
		t.Fatalf("expected action host.market.source.list, got %#v", responseData)
	}
	if got := interfaceToString(responseData["source_id"]); got != "official" {
		t.Fatalf("expected source_id=official, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginMarketSourceGetHelper(t *testing.T) {
	responseData := runPluginMarketHostHelperTest(
		t,
		"host.market.source.get",
		[]string{"host.market.source.read"},
		`
module.exports.execute = function() {
  const result = Plugin.market.source.get({ source_id: "official" });
  return {
    success: true,
    data: {
      action: result.action,
      source_id: result.source_id,
      enabled: result.enabled
    }
  };
};
`,
		func(req pluginipc.HostRequest) map[string]interface{} {
			if got := interfaceToString(req.Params["source_id"]); got != "official" {
				t.Errorf("expected params.source_id=official, got %#v", req.Params)
			}
			return map[string]interface{}{
				"action":    req.Action,
				"source_id": "official",
				"enabled":   true,
			}
		},
	)

	if got := interfaceToString(responseData["action"]); got != "host.market.source.get" {
		t.Fatalf("expected action host.market.source.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["source_id"]); got != "official" {
		t.Fatalf("expected source_id=official, got %#v", responseData)
	}
	if enabled, ok := interfaceToBool(responseData["enabled"]); !ok || !enabled {
		t.Fatalf("expected enabled=true, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginMarketArtifactGetHelper(t *testing.T) {
	responseData := runPluginMarketHostHelperTest(
		t,
		"host.market.artifact.get",
		[]string{"host.market.catalog.read"},
		`
module.exports.execute = function() {
  const result = Plugin.market.artifact.get({
    source_id: "official",
    kind: "plugin_package",
    name: "debugger"
  });
  return {
    success: true,
    data: {
      action: result.action,
      name: result.name,
      latest_version: result.latest_version
    }
  };
};
`,
		func(req pluginipc.HostRequest) map[string]interface{} {
			if got := interfaceToString(req.Params["source_id"]); got != "official" {
				t.Errorf("expected params.source_id=official, got %#v", req.Params)
			}
			if got := interfaceToString(req.Params["kind"]); got != "plugin_package" {
				t.Errorf("expected params.kind=plugin_package, got %#v", req.Params)
			}
			if got := interfaceToString(req.Params["name"]); got != "debugger" {
				t.Errorf("expected params.name=debugger, got %#v", req.Params)
			}
			return map[string]interface{}{
				"action":         req.Action,
				"name":           "debugger",
				"latest_version": "1.2.0",
			}
		},
	)

	if got := interfaceToString(responseData["action"]); got != "host.market.artifact.get" {
		t.Fatalf("expected action host.market.artifact.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["name"]); got != "debugger" {
		t.Fatalf("expected name=debugger, got %#v", responseData)
	}
	if got := interfaceToString(responseData["latest_version"]); got != "1.2.0" {
		t.Fatalf("expected latest_version=1.2.0, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginMarketReleaseGetHelper(t *testing.T) {
	responseData := runPluginMarketHostHelperTest(
		t,
		"host.market.release.get",
		[]string{"host.market.catalog.read"},
		`
module.exports.execute = function() {
  const result = Plugin.market.release.get({
    source_id: "official",
    kind: "plugin_package",
    name: "debugger",
    version: "1.2.0"
  });
  return {
    success: true,
    data: {
      action: result.action,
      version: result.version,
      channel: result.channel
    }
  };
};
`,
		func(req pluginipc.HostRequest) map[string]interface{} {
			if got := interfaceToString(req.Params["version"]); got != "1.2.0" {
				t.Errorf("expected params.version=1.2.0, got %#v", req.Params)
			}
			return map[string]interface{}{
				"action":  req.Action,
				"version": "1.2.0",
				"channel": "stable",
			}
		},
	)

	if got := interfaceToString(responseData["action"]); got != "host.market.release.get" {
		t.Fatalf("expected action host.market.release.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["version"]); got != "1.2.0" {
		t.Fatalf("expected version=1.2.0, got %#v", responseData)
	}
	if got := interfaceToString(responseData["channel"]); got != "stable" {
		t.Fatalf("expected channel=stable, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginMarketInstallExecuteHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.market.install.execute({
    source_id: "official",
    kind: "plugin_package",
    name: "debugger",
    version: "1.2.0",
    options: {
      activate: true
    }
  });
  return {
    success: true,
    data: {
      action: result.action,
      status: result.status
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.market.install.execute" {
			t.Errorf("expected action host.market.install.execute, got %q", req.Action)
		}
		if got := interfaceToString(req.Params["name"]); got != "debugger" {
			t.Errorf("expected params.name=debugger, got %#v", req.Params)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action": req.Action,
				"status": "activated",
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.market.install.execute",
		GrantedPermissions: []string{"host.market.install.execute"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.market.install.execute" {
		t.Fatalf("expected action host.market.install.execute, got %#v", responseData)
	}
	if got := interfaceToString(responseData["status"]); got != "activated" {
		t.Fatalf("expected status=activated, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginMarketInstallTaskGetHelper(t *testing.T) {
	responseData := runPluginMarketHostHelperTest(
		t,
		"host.market.install.task.get",
		[]string{"host.market.install.read"},
		`
module.exports.execute = function() {
  const result = Plugin.market.install.task.get({ task_id: "market-install-17" });
  return {
    success: true,
    data: {
      action: result.action,
      task_id: result.task_id,
      status: result.status
    }
  };
};
`,
		func(req pluginipc.HostRequest) map[string]interface{} {
			if got := interfaceToString(req.Params["task_id"]); got != "market-install-17" {
				t.Errorf("expected params.task_id=market-install-17, got %#v", req.Params)
			}
			return map[string]interface{}{
				"action":  req.Action,
				"task_id": "market-install-17",
				"status":  "succeeded",
			}
		},
	)

	if got := interfaceToString(responseData["action"]); got != "host.market.install.task.get" {
		t.Fatalf("expected action host.market.install.task.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["task_id"]); got != "market-install-17" {
		t.Fatalf("expected task_id=market-install-17, got %#v", responseData)
	}
	if got := interfaceToString(responseData["status"]); got != "succeeded" {
		t.Fatalf("expected status=succeeded, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginMarketInstallTaskListHelper(t *testing.T) {
	responseData := runPluginMarketHostHelperTest(
		t,
		"host.market.install.task.list",
		[]string{"host.market.install.read"},
		`
module.exports.execute = function() {
  const result = Plugin.market.install.task.list({ source_id: "official", kind: "plugin_package" });
  return {
    success: true,
    data: {
      action: result.action,
      total: result.items ? result.items.length : 0,
      first_task_id: result.items && result.items[0] ? result.items[0].task_id : ""
    }
  };
};
`,
		func(req pluginipc.HostRequest) map[string]interface{} {
			if got := interfaceToString(req.Params["source_id"]); got != "official" {
				t.Errorf("expected params.source_id=official, got %#v", req.Params)
			}
			return map[string]interface{}{
				"action": req.Action,
				"items": []map[string]interface{}{
					{
						"task_id": "market-install-19",
					},
				},
			}
		},
	)

	if got := interfaceToString(responseData["action"]); got != "host.market.install.task.list" {
		t.Fatalf("expected action host.market.install.task.list, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["total"]); got != 1 {
		t.Fatalf("expected total=1, got %#v", responseData)
	}
	if got := interfaceToString(responseData["first_task_id"]); got != "market-install-19" {
		t.Fatalf("expected first_task_id=market-install-19, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginMarketInstallHistoryHelper(t *testing.T) {
	responseData := runPluginMarketHostHelperTest(
		t,
		"host.market.install.history.list",
		[]string{"host.market.install.read"},
		`
module.exports.execute = function() {
  const result = Plugin.market.install.history.list({ source_id: "official", kind: "plugin_package", name: "debugger" });
  return {
    success: true,
    data: {
      action: result.action,
      total: result.items ? result.items.length : 0,
      version: result.items && result.items[0] ? result.items[0].version : ""
    }
  };
};
`,
		func(req pluginipc.HostRequest) map[string]interface{} {
			if got := interfaceToString(req.Params["name"]); got != "debugger" {
				t.Errorf("expected params.name=debugger, got %#v", req.Params)
			}
			return map[string]interface{}{
				"action": req.Action,
				"items": []map[string]interface{}{
					{
						"version": "1.2.0",
					},
				},
			}
		},
	)

	if got := interfaceToString(responseData["action"]); got != "host.market.install.history.list" {
		t.Fatalf("expected action host.market.install.history.list, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["total"]); got != 1 {
		t.Fatalf("expected total=1, got %#v", responseData)
	}
	if got := interfaceToString(responseData["version"]); got != "1.2.0" {
		t.Fatalf("expected version=1.2.0, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginMarketInstallRollbackHelper(t *testing.T) {
	responseData := runPluginMarketHostHelperTest(
		t,
		"host.market.install.rollback",
		[]string{"host.market.install.rollback"},
		`
module.exports.execute = function() {
  const result = Plugin.market.install.rollback({
    source_id: "official",
    kind: "plugin_package",
    name: "debugger",
    version: "1.1.0"
  });
  return {
    success: true,
    data: {
      action: result.action,
      status: result.status
    }
  };
};
`,
		func(req pluginipc.HostRequest) map[string]interface{} {
			if got := interfaceToString(req.Params["version"]); got != "1.1.0" {
				t.Errorf("expected params.version=1.1.0, got %#v", req.Params)
			}
			return map[string]interface{}{
				"action": req.Action,
				"status": "rolled_back",
			}
		},
	)

	if got := interfaceToString(responseData["action"]); got != "host.market.install.rollback" {
		t.Fatalf("expected action host.market.install.rollback, got %#v", responseData)
	}
	if got := interfaceToString(responseData["status"]); got != "rolled_back" {
		t.Fatalf("expected status=rolled_back, got %#v", responseData)
	}
}

func runPluginMarketHostHelperTest(
	t *testing.T,
	action string,
	grantedPermissions []string,
	scriptBody string,
	responseBuilder func(req pluginipc.HostRequest) map[string]interface{},
) map[string]interface{} {
	t.Helper()

	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(scriptBody))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != action {
			t.Errorf("expected action %s, got %q", action, req.Action)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data:    responseBuilder(req),
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test." + action,
		GrantedPermissions: grantedPermissions,
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	return responseData
}

func TestRunScriptFunctionWithStorageInjectsPluginEmailTemplateHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.emailTemplate.get({ key: "order_paid" });
  return {
    success: true,
    data: {
      action: result.action,
      key: result.key
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.email_template.get" {
			t.Errorf("expected action host.email_template.get, got %q", req.Action)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action": req.Action,
				"key":    "order_paid",
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.email_template",
		GrantedPermissions: []string{"host.email_template.read"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.email_template.get" {
		t.Fatalf("expected action host.email_template.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["key"]); got != "order_paid" {
		t.Fatalf("expected key=order_paid, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginLandingPageHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.landingPage.reset({ page_key: "home" });
  return {
    success: true,
    data: {
      action: result.action,
      page_key: result.page_key
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.landing_page.reset" {
			t.Errorf("expected action host.landing_page.reset, got %q", req.Action)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action":   req.Action,
				"page_key": "home",
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.landing_page",
		GrantedPermissions: []string{"host.landing_page.write"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.landing_page.reset" {
		t.Fatalf("expected action host.landing_page.reset, got %#v", responseData)
	}
	if got := interfaceToString(responseData["page_key"]); got != "home" {
		t.Fatalf("expected page_key=home, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginInvoiceTemplateHelper(t *testing.T) {
	responseData := runPluginMarketHostHelperTest(
		t,
		"host.invoice_template.reset",
		[]string{"host.invoice_template.write"},
		`
module.exports.execute = function() {
  const result = Plugin.invoiceTemplate.reset({ target_key: "invoice" });
  return {
    success: true,
    data: {
      action: result.action,
      target_key: result.target_key
    }
  };
};
`,
		func(req pluginipc.HostRequest) map[string]interface{} {
			return map[string]interface{}{
				"action":     req.Action,
				"target_key": req.Params["target_key"],
			}
		},
	)

	if got := interfaceToString(responseData["action"]); got != "host.invoice_template.reset" {
		t.Fatalf("expected action host.invoice_template.reset, got %#v", responseData)
	}
	if got := interfaceToString(responseData["target_key"]); got != "invoice" {
		t.Fatalf("expected target_key=invoice, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginAuthBrandingHelper(t *testing.T) {
	responseData := runPluginMarketHostHelperTest(
		t,
		"host.auth_branding.get",
		[]string{"host.auth_branding.read"},
		`
module.exports.execute = function() {
  const result = Plugin.authBranding.get({ target_key: "auth_branding" });
  return {
    success: true,
    data: {
      action: result.action,
      target_key: result.target_key
    }
  };
};
`,
		func(req pluginipc.HostRequest) map[string]interface{} {
			return map[string]interface{}{
				"action":     req.Action,
				"target_key": req.Params["target_key"],
			}
		},
	)

	if got := interfaceToString(responseData["action"]); got != "host.auth_branding.get" {
		t.Fatalf("expected action host.auth_branding.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["target_key"]); got != "auth_branding" {
		t.Fatalf("expected target_key=auth_branding, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginPageRulePackHelper(t *testing.T) {
	responseData := runPluginMarketHostHelperTest(
		t,
		"host.page_rule_pack.get",
		[]string{"host.page_rule_pack.read"},
		`
module.exports.execute = function() {
  const result = Plugin.pageRulePack.get({ target_key: "page_rules" });
  return {
    success: true,
    data: {
      action: result.action,
      target_key: result.target_key
    }
  };
};
`,
		func(req pluginipc.HostRequest) map[string]interface{} {
			return map[string]interface{}{
				"action":     req.Action,
				"target_key": req.Params["target_key"],
			}
		},
	)

	if got := interfaceToString(responseData["action"]); got != "host.page_rule_pack.get" {
		t.Fatalf("expected action host.page_rule_pack.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["target_key"]); got != "page_rules" {
		t.Fatalf("expected target_key=page_rules, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginInventoryBindingHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.inventoryBinding.get({ id: 17 });
  return {
    success: true,
    data: {
      action: result.action,
      entity: result.entity,
      id: result.id
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.inventory_binding.get" {
			t.Errorf("expected action host.inventory_binding.get, got %q", req.Action)
		}
		if got := testInterfaceToInt64(req.Params["id"]); got != 17 {
			t.Errorf("expected params.id=17, got %#v", got)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action": req.Action,
				"entity": "inventory_binding",
				"id":     req.Params["id"],
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.inventory_binding",
		GrantedPermissions: []string{"host.inventory_binding.read"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.inventory_binding.get" {
		t.Fatalf("expected action host.inventory_binding.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["entity"]); got != "inventory_binding" {
		t.Fatalf("expected entity inventory_binding, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["id"]); got != 17 {
		t.Fatalf("expected id=17, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginVirtualInventoryHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.virtualInventory.get({ id: 23 });
  return {
    success: true,
    data: {
      action: result.action,
      entity: result.entity,
      id: result.id
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.virtual_inventory.get" {
			t.Errorf("expected action host.virtual_inventory.get, got %q", req.Action)
		}
		if got := testInterfaceToInt64(req.Params["id"]); got != 23 {
			t.Errorf("expected params.id=23, got %#v", got)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action": req.Action,
				"entity": "virtual_inventory",
				"id":     req.Params["id"],
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.virtual_inventory",
		GrantedPermissions: []string{"host.virtual_inventory.read"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.virtual_inventory.get" {
		t.Fatalf("expected action host.virtual_inventory.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["entity"]); got != "virtual_inventory" {
		t.Fatalf("expected entity virtual_inventory, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["id"]); got != 23 {
		t.Fatalf("expected id=23, got %#v", responseData)
	}
}

func TestRunScriptFunctionWithStorageInjectsPluginVirtualInventoryBindingHelper(t *testing.T) {
	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, filepath.Join(rootDir, "manifest.json"), []byte(`{"name":"test-plugin"}`))
	mustWriteFile(t, scriptPath, []byte(`
module.exports.execute = function() {
  const result = Plugin.virtualInventoryBinding.get({ id: 29 });
  return {
    success: true,
    data: {
      action: result.action,
      entity: result.entity,
      id: result.id
    }
  };
};
`))

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen test host bridge failed: %v", err)
	}
	defer listener.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			t.Errorf("accept host bridge conn failed: %v", acceptErr)
			return
		}
		defer conn.Close()

		var req pluginipc.HostRequest
		if err := json.NewDecoder(conn).Decode(&req); err != nil {
			t.Errorf("decode host request failed: %v", err)
			return
		}
		if req.Action != "host.virtual_inventory_binding.get" {
			t.Errorf("expected action host.virtual_inventory_binding.get, got %q", req.Action)
		}
		if got := testInterfaceToInt64(req.Params["id"]); got != 29 {
			t.Errorf("expected params.id=29, got %#v", got)
		}

		if err := json.NewEncoder(conn).Encode(pluginipc.HostResponse{
			Success: true,
			Status:  200,
			Data: map[string]interface{}{
				"action": req.Action,
				"entity": "virtual_inventory_binding",
				"id":     req.Params["id"],
			},
		}); err != nil {
			t.Errorf("encode host response failed: %v", err)
		}
	}()

	opts := workerOptions{
		timeoutMs:            1000,
		maxConcurrency:       1,
		maxMemoryMB:          32,
		storageMaxKeys:       16,
		storageMaxTotalBytes: 4096,
		storageMaxValueBytes: 1024,
	}
	sandboxCfg := pluginipc.SandboxConfig{
		CurrentAction:      "test.host.virtual_inventory_binding",
		GrantedPermissions: []string{"host.virtual_inventory_binding.read"},
	}
	fsCtx := pluginFSRuntimeContext{
		CodeRoot:   rootDir,
		DataRoot:   filepath.Join(rootDir, "data"),
		PluginID:   1,
		PluginName: "test-plugin",
	}

	result, _, _, _, err := runScriptFunctionWithStorage(
		scriptPath,
		"execute",
		[]interface{}{},
		opts,
		sandboxCfg,
		&pluginipc.HostAPIConfig{
			Network:     "tcp",
			Address:     listener.Addr().String(),
			AccessToken: "token-demo",
			TimeoutMs:   1000,
		},
		time.Second,
		nil,
		nil,
		nil,
		context.Background(),
		fsCtx,
	)
	if err != nil {
		t.Fatalf("runScriptFunctionWithStorage returned error: %v", err)
	}
	<-done

	payload, responseData, success, errMsg := parseExecutionPayload(exportGojaValue(result))
	if !success || errMsg != "" {
		t.Fatalf("expected success payload, got success=%v error=%q payload=%#v", success, errMsg, payload)
	}
	if got := interfaceToString(responseData["action"]); got != "host.virtual_inventory_binding.get" {
		t.Fatalf("expected action host.virtual_inventory_binding.get, got %#v", responseData)
	}
	if got := interfaceToString(responseData["entity"]); got != "virtual_inventory_binding" {
		t.Fatalf("expected entity virtual_inventory_binding, got %#v", responseData)
	}
	if got := testInterfaceToInt64(responseData["id"]); got != 29 {
		t.Fatalf("expected id=29, got %#v", responseData)
	}
}

func mustWriteFile(t *testing.T, targetPath string, payload []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("mkdir %s failed: %v", targetPath, err)
	}
	if err := os.WriteFile(targetPath, payload, 0o644); err != nil {
		t.Fatalf("write %s failed: %v", targetPath, err)
	}
}

func testInterfaceToInt64(value interface{}) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int8:
		return int64(typed)
	case int16:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case uint:
		return int64(typed)
	case uint8:
		return int64(typed)
	case uint16:
		return int64(typed)
	case uint32:
		return int64(typed)
	case uint64:
		return int64(typed)
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}
