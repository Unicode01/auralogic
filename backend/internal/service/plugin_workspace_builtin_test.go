package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"auralogic/internal/pluginipc"
)

func TestPluginWorkspaceBuiltinCatReadsCodeFileAndWritesTranscript(t *testing.T) {
	artifactRoot := t.TempDir()
	pluginRoot := createPluginWorkspaceBuiltinTestPackage(t, artifactRoot, map[string]string{
		"notes/info.txt": "hello workspace\n",
	})
	service := NewPluginManagerService(nil, &config.Config{Plugin: config.PluginPlatformConfig{
		ArtifactDir: artifactRoot,
	}})
	plugin := &models.Plugin{
		ID:          41,
		Name:        "builtin-demo",
		Runtime:     PluginRuntimeJSWorker,
		PackagePath: pluginRoot,
		Address:     filepath.ToSlash(filepath.Join(pluginRoot, "index.js")),
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": ["runtime.file_system"]
		}`,
	}

	command, err := ResolvePluginWorkspaceCommand(plugin, pluginWorkspaceBuiltinCommandCat)
	if err != nil {
		t.Fatalf("resolve builtin cat command failed: %v", err)
	}
	result, err := service.executePluginWorkspaceBuiltinCommand(
		plugin,
		PluginRuntimeJSWorker,
		command,
		[]string{"notes/info.txt"},
		nil,
		&ExecutionContext{},
	)
	if err != nil {
		t.Fatalf("execute builtin cat failed: %v", err)
	}
	if content, _ := result.Data["content"].(string); content != "hello workspace\n" {
		t.Fatalf("expected content to equal file text, got %q", content)
	}

	snapshot := service.GetPluginWorkspaceSnapshot(plugin, 8)
	if len(snapshot.Entries) < 2 {
		t.Fatalf("expected builtin transcript entries, got %+v", snapshot.Entries)
	}
	if snapshot.Entries[len(snapshot.Entries)-1].Source != pluginWorkspaceBuiltinSource {
		t.Fatalf("expected builtin transcript source %s, got %+v", pluginWorkspaceBuiltinSource, snapshot.Entries[len(snapshot.Entries)-1])
	}
}

func TestPluginWorkspaceBuiltinTranscriptSkipsInvocationEchoForTerminalLine(t *testing.T) {
	artifactRoot := t.TempDir()
	pluginRoot := createPluginWorkspaceBuiltinTestPackage(t, artifactRoot, map[string]string{})
	service := NewPluginManagerService(nil, &config.Config{Plugin: config.PluginPlatformConfig{
		ArtifactDir: artifactRoot,
	}})
	plugin := &models.Plugin{
		ID:          42,
		Name:        "builtin-help-terminal",
		Runtime:     PluginRuntimeJSWorker,
		PackagePath: pluginRoot,
		Address:     filepath.ToSlash(filepath.Join(pluginRoot, "index.js")),
	}

	command, err := ResolvePluginWorkspaceCommand(plugin, pluginWorkspaceBuiltinCommandHelp)
	if err != nil {
		t.Fatalf("resolve builtin help command failed: %v", err)
	}
	if _, err := service.executePluginWorkspaceBuiltinCommand(
		plugin,
		PluginRuntimeJSWorker,
		command,
		nil,
		nil,
		&ExecutionContext{Metadata: map[string]string{
			"workspace_terminal_line": "help",
		}},
	); err != nil {
		t.Fatalf("execute builtin help failed: %v", err)
	}

	snapshot := service.GetPluginWorkspaceSnapshot(plugin, 8)
	if len(snapshot.Entries) != 1 {
		t.Fatalf("expected only builtin output entry for terminal line, got %+v", snapshot.Entries)
	}
	entry := snapshot.Entries[0]
	if entry.Channel != "stdout" || entry.Source != pluginWorkspaceBuiltinSource {
		t.Fatalf("expected only builtin stdout entry, got %+v", entry)
	}
	if strings.Contains(entry.Message, "$ help") {
		t.Fatalf("expected terminal line transcript to skip builtin invocation echo, got %+v", entry)
	}
}

func TestPluginWorkspaceBuiltinLSMergesDataAndCode(t *testing.T) {
	artifactRoot := t.TempDir()
	pluginRoot := createPluginWorkspaceBuiltinTestPackage(t, artifactRoot, map[string]string{
		"assets/shared.txt":   "code",
		"assets/code-only.js": "console.log('code')",
	})
	dataRoot := filepath.Join(artifactRoot, "data", "plugin_52", "assets")
	mustMkdirAll(t, dataRoot)
	mustWriteTextFile(t, filepath.Join(dataRoot, "shared.txt"), "data")
	mustWriteTextFile(t, filepath.Join(dataRoot, "data-only.txt"), "only")

	service := NewPluginManagerService(nil, &config.Config{Plugin: config.PluginPlatformConfig{
		ArtifactDir: artifactRoot,
	}})
	plugin := &models.Plugin{
		ID:          52,
		Name:        "builtin-overlay",
		Runtime:     PluginRuntimeJSWorker,
		PackagePath: pluginRoot,
		Address:     filepath.ToSlash(filepath.Join(pluginRoot, "index.js")),
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": ["runtime.file_system"]
		}`,
	}

	command, err := ResolvePluginWorkspaceCommand(plugin, pluginWorkspaceBuiltinCommandLS)
	if err != nil {
		t.Fatalf("resolve builtin ls command failed: %v", err)
	}
	result, err := service.executePluginWorkspaceBuiltinCommand(
		plugin,
		PluginRuntimeJSWorker,
		command,
		[]string{"assets"},
		nil,
		&ExecutionContext{},
	)
	if err != nil {
		t.Fatalf("execute builtin ls failed: %v", err)
	}
	items, ok := result.Data["items"].([]pluginWorkspaceBuiltinListItem)
	if !ok {
		t.Fatalf("expected ls result items, got %#v", result.Data["items"])
	}
	if len(items) != 3 {
		t.Fatalf("expected merged ls to show 3 items, got %+v", items)
	}
	shared := findPluginWorkspaceBuiltinListItem(items, "assets/shared.txt")
	if shared == nil || shared.Layer != "data" {
		t.Fatalf("expected data layer to override shared.txt, got %+v", shared)
	}
}

func TestPluginWorkspaceBuiltinMkdirCreatesDataLayerDirectory(t *testing.T) {
	artifactRoot := t.TempDir()
	pluginRoot := createPluginWorkspaceBuiltinTestPackage(t, artifactRoot, map[string]string{
		"assets/shared.txt": "code",
	})
	service := NewPluginManagerService(nil, &config.Config{Plugin: config.PluginPlatformConfig{
		ArtifactDir: artifactRoot,
	}})
	plugin := &models.Plugin{
		ID:          57,
		Name:        "builtin-mkdir",
		Runtime:     PluginRuntimeJSWorker,
		PackagePath: pluginRoot,
		Address:     filepath.ToSlash(filepath.Join(pluginRoot, "index.js")),
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": ["runtime.file_system"]
		}`,
	}

	command, err := ResolvePluginWorkspaceCommand(plugin, pluginWorkspaceBuiltinCommandMkdir)
	if err != nil {
		t.Fatalf("resolve builtin mkdir command failed: %v", err)
	}
	result, err := service.executePluginWorkspaceBuiltinCommand(
		plugin,
		PluginRuntimeJSWorker,
		command,
		[]string{"assets/generated"},
		nil,
		&ExecutionContext{},
	)
	if err != nil {
		t.Fatalf("execute builtin mkdir failed: %v", err)
	}
	created, _ := result.Data["created"].(bool)
	if !created {
		t.Fatalf("expected mkdir to report created=true, got %#v", result.Data["created"])
	}
	if _, err := os.Stat(filepath.Join(artifactRoot, "data", "plugin_57", "assets", "generated")); err != nil {
		t.Fatalf("expected data layer directory to exist, got %v", err)
	}
}

func TestPluginWorkspaceBuiltinFindUsesMergedOverlayView(t *testing.T) {
	artifactRoot := t.TempDir()
	pluginRoot := createPluginWorkspaceBuiltinTestPackage(t, artifactRoot, map[string]string{
		"assets/shared.txt":   "code",
		"assets/code-only.js": "console.log('code')",
	})
	dataRoot := filepath.Join(artifactRoot, "data", "plugin_59", "assets")
	mustMkdirAll(t, dataRoot)
	mustWriteTextFile(t, filepath.Join(dataRoot, "shared.txt"), "data")
	mustWriteTextFile(t, filepath.Join(dataRoot, "data-only.txt"), "data")

	service := NewPluginManagerService(nil, &config.Config{Plugin: config.PluginPlatformConfig{
		ArtifactDir: artifactRoot,
	}})
	plugin := &models.Plugin{
		ID:          59,
		Name:        "builtin-find",
		Runtime:     PluginRuntimeJSWorker,
		PackagePath: pluginRoot,
		Address:     filepath.ToSlash(filepath.Join(pluginRoot, "index.js")),
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": ["runtime.file_system"]
		}`,
	}

	command, err := ResolvePluginWorkspaceCommand(plugin, pluginWorkspaceBuiltinCommandFind)
	if err != nil {
		t.Fatalf("resolve builtin find command failed: %v", err)
	}
	result, err := service.executePluginWorkspaceBuiltinCommand(
		plugin,
		PluginRuntimeJSWorker,
		command,
		[]string{"shared"},
		nil,
		&ExecutionContext{},
	)
	if err != nil {
		t.Fatalf("execute builtin find failed: %v", err)
	}
	items, ok := result.Data["items"].([]pluginWorkspaceBuiltinListItem)
	if !ok {
		t.Fatalf("expected find result items, got %#v", result.Data["items"])
	}
	if len(items) != 1 || items[0].Path != "assets/shared.txt" || items[0].Layer != "data" {
		t.Fatalf("expected merged shared.txt from data layer, got %+v", items)
	}
}

func TestPluginWorkspaceBuiltinClearResetsBufferBeforeTranscriptReplay(t *testing.T) {
	artifactRoot := t.TempDir()
	pluginRoot := createPluginWorkspaceBuiltinTestPackage(t, artifactRoot, map[string]string{})
	service := NewPluginManagerService(nil, &config.Config{Plugin: config.PluginPlatformConfig{
		ArtifactDir: artifactRoot,
	}})
	plugin := &models.Plugin{
		ID:          60,
		Name:        "builtin-clear",
		Runtime:     PluginRuntimeJSWorker,
		PackagePath: pluginRoot,
		Address:     filepath.ToSlash(filepath.Join(pluginRoot, "index.js")),
	}
	service.ApplyPluginWorkspaceDelta(plugin.ID, plugin.Name, PluginRuntimeJSWorker, map[string]string{
		"action": "hook.execute",
	}, []pluginipc.WorkspaceBufferEntry{{
		Channel: "stdout",
		Level:   "info",
		Message: "stale entry",
		Source:  "test.seed",
	}}, false)

	command, err := ResolvePluginWorkspaceCommand(plugin, pluginWorkspaceBuiltinCommandClear)
	if err != nil {
		t.Fatalf("resolve builtin clear command failed: %v", err)
	}
	if _, err := service.executePluginWorkspaceBuiltinCommand(
		plugin,
		PluginRuntimeJSWorker,
		command,
		nil,
		nil,
		&ExecutionContext{},
	); err != nil {
		t.Fatalf("execute builtin clear failed: %v", err)
	}

	snapshot := service.GetPluginWorkspaceSnapshot(plugin, 8)
	if len(snapshot.Entries) == 0 {
		t.Fatalf("expected transcript entries after clear, got empty snapshot")
	}
	for _, entry := range snapshot.Entries {
		if strings.Contains(entry.Message, "stale entry") {
			t.Fatalf("expected stale entries to be removed, got %+v", snapshot.Entries)
		}
	}
}

func TestPluginWorkspaceBuiltinLogTailReturnsRecentEntries(t *testing.T) {
	artifactRoot := t.TempDir()
	pluginRoot := createPluginWorkspaceBuiltinTestPackage(t, artifactRoot, map[string]string{})
	service := NewPluginManagerService(nil, &config.Config{Plugin: config.PluginPlatformConfig{
		ArtifactDir: artifactRoot,
	}})
	plugin := &models.Plugin{
		ID:          61,
		Name:        "builtin-log-tail",
		Runtime:     PluginRuntimeJSWorker,
		PackagePath: pluginRoot,
		Address:     filepath.ToSlash(filepath.Join(pluginRoot, "index.js")),
	}
	service.ApplyPluginWorkspaceDelta(plugin.ID, plugin.Name, PluginRuntimeJSWorker, map[string]string{
		"action": "hook.execute",
	}, []pluginipc.WorkspaceBufferEntry{
		{Channel: "stdout", Level: "info", Message: "first", Source: "test.seed"},
		{Channel: "stderr", Level: "error", Message: "second", Source: "test.seed"},
		{Channel: "console", Level: "warn", Message: "third", Source: "test.seed"},
	}, false)

	command, err := ResolvePluginWorkspaceCommand(plugin, pluginWorkspaceBuiltinCommandLogTail)
	if err != nil {
		t.Fatalf("resolve builtin log.tail command failed: %v", err)
	}
	result, err := service.executePluginWorkspaceBuiltinCommand(
		plugin,
		PluginRuntimeJSWorker,
		command,
		[]string{"2", "--level", "warn"},
		nil,
		&ExecutionContext{},
	)
	if err != nil {
		t.Fatalf("execute builtin log.tail failed: %v", err)
	}
	entries, ok := result.Data["entries"].([]PluginWorkspaceBufferEntry)
	if !ok {
		t.Fatalf("expected log.tail entries slice, got %#v", result.Data["entries"])
	}
	if len(entries) != 1 || entries[0].Message != "third" {
		t.Fatalf("expected filtered tail to return only the warn entry, got %+v", entries)
	}
	output, _ := result.Data["output"].(string)
	if strings.Contains(output, "first") || !strings.Contains(output, "third") {
		t.Fatalf("unexpected log.tail output: %q", output)
	}
}

func TestPluginWorkspaceBuiltinKVListReturnsSortedKeys(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	plugin := createPluginStorageTestPlugin(t, db, "workspace-kv-plugin")
	rows := []models.PluginStorageEntry{
		{PluginID: plugin.ID, Key: "pref.b", Value: "2"},
		{PluginID: plugin.ID, Key: "pref.a", Value: "1"},
		{PluginID: plugin.ID, Key: "other", Value: "3"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed plugin storage rows failed: %v", err)
	}

	service := NewPluginManagerService(db, &config.Config{Plugin: config.PluginPlatformConfig{
		ArtifactDir: t.TempDir(),
	}})
	plugin.Runtime = PluginRuntimeJSWorker
	command, err := ResolvePluginWorkspaceCommand(&plugin, pluginWorkspaceBuiltinCommandKVList)
	if err != nil {
		t.Fatalf("resolve builtin kv.list command failed: %v", err)
	}
	result, err := service.executePluginWorkspaceBuiltinCommand(
		&plugin,
		PluginRuntimeJSWorker,
		command,
		[]string{"pref."},
		nil,
		&ExecutionContext{},
	)
	if err != nil {
		t.Fatalf("execute builtin kv.list failed: %v", err)
	}
	keys, ok := result.Data["keys"].([]string)
	if !ok {
		t.Fatalf("expected kv.list keys slice, got %#v", result.Data["keys"])
	}
	if len(keys) != 2 || keys[0] != "pref.a" || keys[1] != "pref.b" {
		t.Fatalf("expected sorted prefixed keys, got %+v", keys)
	}
}

func TestPluginWorkspaceBuiltinKVSetAndDelPersistChanges(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	plugin := createPluginStorageTestPlugin(t, db, "workspace-kv-mutate-plugin")
	service := NewPluginManagerService(db, &config.Config{Plugin: config.PluginPlatformConfig{
		ArtifactDir: t.TempDir(),
	}})
	plugin.Runtime = PluginRuntimeJSWorker

	setCommand, err := ResolvePluginWorkspaceCommand(&plugin, pluginWorkspaceBuiltinCommandKVSet)
	if err != nil {
		t.Fatalf("resolve builtin kv.set command failed: %v", err)
	}
	if _, err := service.executePluginWorkspaceBuiltinCommand(
		&plugin,
		PluginRuntimeJSWorker,
		setCommand,
		[]string{"pref.token", "hello world"},
		nil,
		&ExecutionContext{},
	); err != nil {
		t.Fatalf("execute builtin kv.set failed: %v", err)
	}
	snapshot, err := service.loadPluginStorageSnapshot(plugin.ID)
	if err != nil {
		t.Fatalf("load plugin storage snapshot failed: %v", err)
	}
	if snapshot["pref.token"] != "hello world" {
		t.Fatalf("expected kv.set to persist value, got %+v", snapshot)
	}

	delCommand, err := ResolvePluginWorkspaceCommand(&plugin, pluginWorkspaceBuiltinCommandKVDel)
	if err != nil {
		t.Fatalf("resolve builtin kv.del command failed: %v", err)
	}
	result, err := service.executePluginWorkspaceBuiltinCommand(
		&plugin,
		PluginRuntimeJSWorker,
		delCommand,
		[]string{"pref.token"},
		nil,
		&ExecutionContext{},
	)
	if err != nil {
		t.Fatalf("execute builtin kv.del failed: %v", err)
	}
	deleted, _ := result.Data["deleted"].(bool)
	if !deleted {
		t.Fatalf("expected kv.del to report deleted=true, got %#v", result.Data["deleted"])
	}
	snapshot, err = service.loadPluginStorageSnapshot(plugin.ID)
	if err != nil {
		t.Fatalf("reload plugin storage snapshot failed: %v", err)
	}
	if _, exists := snapshot["pref.token"]; exists {
		t.Fatalf("expected kv.del to remove key, got %+v", snapshot)
	}
}

func TestPluginWorkspaceBuiltinRejectsTraversalPath(t *testing.T) {
	artifactRoot := t.TempDir()
	pluginRoot := createPluginWorkspaceBuiltinTestPackage(t, artifactRoot, map[string]string{
		"notes/info.txt": "hello workspace\n",
	})
	service := NewPluginManagerService(nil, &config.Config{Plugin: config.PluginPlatformConfig{
		ArtifactDir: artifactRoot,
	}})
	plugin := &models.Plugin{
		ID:          77,
		Name:        "builtin-traversal",
		Runtime:     PluginRuntimeJSWorker,
		PackagePath: pluginRoot,
		Address:     filepath.ToSlash(filepath.Join(pluginRoot, "index.js")),
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": ["runtime.file_system"]
		}`,
	}

	command, err := ResolvePluginWorkspaceCommand(plugin, pluginWorkspaceBuiltinCommandCat)
	if err != nil {
		t.Fatalf("resolve builtin cat command failed: %v", err)
	}
	_, err = service.executePluginWorkspaceBuiltinCommand(
		plugin,
		PluginRuntimeJSWorker,
		command,
		[]string{"../outside.txt"},
		nil,
		&ExecutionContext{},
	)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "outside plugin root") {
		t.Fatalf("expected traversal to be rejected, got %v", err)
	}
}

func TestPluginWorkspaceBuiltinHelpListsCommands(t *testing.T) {
	artifactRoot := t.TempDir()
	pluginRoot := createPluginWorkspaceBuiltinTestPackage(t, artifactRoot, map[string]string{})
	service := NewPluginManagerService(nil, &config.Config{Plugin: config.PluginPlatformConfig{
		ArtifactDir: artifactRoot,
	}})
	plugin := &models.Plugin{
		ID:          91,
		Name:        "builtin-help",
		Runtime:     PluginRuntimeJSWorker,
		PackagePath: pluginRoot,
		Address:     filepath.ToSlash(filepath.Join(pluginRoot, "index.js")),
	}

	command, err := ResolvePluginWorkspaceCommand(plugin, pluginWorkspaceBuiltinCommandHelp)
	if err != nil {
		t.Fatalf("resolve builtin help command failed: %v", err)
	}
	result, err := service.executePluginWorkspaceBuiltinCommand(
		plugin,
		PluginRuntimeJSWorker,
		command,
		nil,
		nil,
		&ExecutionContext{},
	)
	if err != nil {
		t.Fatalf("execute builtin help failed: %v", err)
	}
	output, _ := result.Data["output"].(string)
	if !strings.Contains(output, "help") || !strings.Contains(output, "kv.del") {
		t.Fatalf("expected help output to list builtin workspace commands, got %q", output)
	}
}

func createPluginWorkspaceBuiltinTestPackage(t *testing.T, artifactRoot string, files map[string]string) string {
	t.Helper()
	pluginRoot := filepath.Join(artifactRoot, "packages", "workspace-plugin")
	mustMkdirAll(t, pluginRoot)
	mustWriteTextFile(t, filepath.Join(pluginRoot, "manifest.json"), `{"name":"workspace-plugin"}`)
	mustWriteTextFile(t, filepath.Join(pluginRoot, "index.js"), `module.exports = {};`)
	for relPath, content := range files {
		mustWriteTextFile(t, filepath.Join(pluginRoot, filepath.FromSlash(relPath)), content)
	}
	return pluginRoot
}

func findPluginWorkspaceBuiltinListItem(items []pluginWorkspaceBuiltinListItem, path string) *pluginWorkspaceBuiltinListItem {
	for idx := range items {
		if items[idx].Path == path {
			return &items[idx]
		}
	}
	return nil
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s failed: %v", path, err)
	}
}

func mustWriteTextFile(t *testing.T, path string, content string) {
	t.Helper()
	mustMkdirAll(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s failed: %v", path, err)
	}
}
