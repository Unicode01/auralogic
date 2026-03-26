package service

import (
	"path/filepath"
	"strings"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
)

func TestParsePluginWorkspaceShellLineSupportsQuotesAndEscapes(t *testing.T) {
	argv, err := ParsePluginWorkspaceShellLine(`grep "hello world" notes/info\.txt`)
	if err != nil {
		t.Fatalf("ParsePluginWorkspaceShellLine returned error: %v", err)
	}
	if len(argv) != 3 || argv[0] != "grep" || argv[1] != "hello world" || argv[2] != "notes/info.txt" {
		t.Fatalf("unexpected argv: %+v", argv)
	}
}

func TestParsePluginWorkspaceShellLineRejectsUnterminatedQuote(t *testing.T) {
	if _, err := ParsePluginWorkspaceShellLine(`grep "hello`); err == nil {
		t.Fatalf("expected unterminated quote to fail")
	}
}

func TestParsePluginWorkspaceShellLineWithVariables(t *testing.T) {
	argv, err := ParsePluginWorkspaceShellLineWithVariables(
		`grep "$PLUGIN_NAME" ${PLUGIN_ID} '$PLUGIN_ID' \$PLUGIN_ID`,
		map[string]string{
			"PLUGIN_NAME": "shell demo",
			"PLUGIN_ID":   "88",
		},
	)
	if err != nil {
		t.Fatalf("ParsePluginWorkspaceShellLineWithVariables returned error: %v", err)
	}
	expected := []string{"grep", "shell demo", "88", "$PLUGIN_ID", "$PLUGIN_ID"}
	if len(argv) != len(expected) {
		t.Fatalf("unexpected argv length: %+v", argv)
	}
	for idx, item := range expected {
		if argv[idx] != item {
			t.Fatalf("unexpected argv at %d: got %q want %q", idx, argv[idx], item)
		}
	}
}

func TestResolvePluginWorkspaceShellCommandWithVariablesKeepsVariableValuesWithinSameToken(t *testing.T) {
	plugin := &models.Plugin{
		ID:          88,
		Name:        "shell-demo",
		Runtime:     PluginRuntimeJSWorker,
		PackagePath: filepath.ToSlash(t.TempDir()),
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": ["runtime.file_system"]
		}`,
	}

	command, argv, err := ResolvePluginWorkspaceShellCommandWithVariables(
		plugin,
		`ls "$PLUGIN_NAME"`,
		map[string]string{
			"PLUGIN_NAME": "notes archive",
		},
	)
	if err != nil {
		t.Fatalf("ResolvePluginWorkspaceShellCommandWithVariables returned error: %v", err)
	}
	if command == nil || command.Name != pluginWorkspaceBuiltinCommandLS {
		t.Fatalf("expected builtin ls command, got %+v", command)
	}
	if len(argv) != 1 || argv[0] != "notes archive" {
		t.Fatalf("unexpected argv: %+v", argv)
	}
}

func TestResolvePluginWorkspaceShellCommandMapsBuiltinAlias(t *testing.T) {
	plugin := &models.Plugin{
		ID:          88,
		Name:        "shell-demo",
		Runtime:     PluginRuntimeJSWorker,
		PackagePath: filepath.ToSlash(t.TempDir()),
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": ["runtime.file_system"]
		}`,
	}

	command, argv, err := ResolvePluginWorkspaceShellCommand(plugin, `ls "notes dir"`)
	if err != nil {
		t.Fatalf("ResolvePluginWorkspaceShellCommand returned error: %v", err)
	}
	if command == nil || command.Name != pluginWorkspaceBuiltinCommandLS || !command.Builtin {
		t.Fatalf("expected builtin ls command, got %+v", command)
	}
	if len(argv) != 1 || argv[0] != "notes dir" {
		t.Fatalf("unexpected argv: %+v", argv)
	}

	command, argv, err = ResolvePluginWorkspaceShellCommand(plugin, `kv.set pref.token "hello world"`)
	if err != nil {
		t.Fatalf("ResolvePluginWorkspaceShellCommand kv.set returned error: %v", err)
	}
	if command == nil || command.Name != pluginWorkspaceBuiltinCommandKVSet || !command.Builtin {
		t.Fatalf("expected builtin kv.set command, got %+v", command)
	}
	if len(argv) != 2 || argv[0] != "pref.token" || argv[1] != "hello world" {
		t.Fatalf("unexpected kv.set argv: %+v", argv)
	}

	command, argv, err = ResolvePluginWorkspaceShellCommand(plugin, `clear`)
	if err != nil {
		t.Fatalf("ResolvePluginWorkspaceShellCommand clear returned error: %v", err)
	}
	if command == nil || command.Name != pluginWorkspaceBuiltinCommandClear || !command.Builtin {
		t.Fatalf("expected builtin clear command, got %+v", command)
	}
	if len(argv) != 0 {
		t.Fatalf("unexpected clear argv: %+v", argv)
	}
}

func TestParsePluginWorkspaceShellPipelineSplitsSegments(t *testing.T) {
	segments, err := ParsePluginWorkspaceShellPipeline(`cat notes.txt | grep "a|b" | help`)
	if err != nil {
		t.Fatalf("ParsePluginWorkspaceShellPipeline returned error: %v", err)
	}
	if len(segments) != 3 {
		t.Fatalf("expected 3 pipeline segments, got %+v", segments)
	}
	if segments[1] != `grep "a|b"` {
		t.Fatalf("unexpected middle segment: %q", segments[1])
	}
}

func TestParsePluginWorkspaceShellSequenceSplitsStatements(t *testing.T) {
	statements, err := ParsePluginWorkspaceShellSequence(`ls assets && grep "a;b|c" notes.txt ; help`)
	if err != nil {
		t.Fatalf("ParsePluginWorkspaceShellSequence returned error: %v", err)
	}
	if len(statements) != 3 {
		t.Fatalf("expected 3 statements, got %+v", statements)
	}
	if statements[0].Operator != pluginWorkspaceShellSequenceOperatorNone || statements[0].Raw != "ls assets" {
		t.Fatalf("unexpected first statement: %+v", statements[0])
	}
	if statements[1].Operator != pluginWorkspaceShellSequenceOperatorOnSuccess || statements[1].Raw != `grep "a;b|c" notes.txt` {
		t.Fatalf("unexpected second statement: %+v", statements[1])
	}
	if statements[2].Operator != pluginWorkspaceShellSequenceOperatorAlways || statements[2].Raw != "help" {
		t.Fatalf("unexpected third statement: %+v", statements[2])
	}
}

func TestParsePluginWorkspaceShellSequenceRejectsUnsupportedOperator(t *testing.T) {
	if _, err := ParsePluginWorkspaceShellSequence(`help || ls`); err == nil {
		t.Fatalf("expected unsupported || operator to fail")
	}
}

func TestResolvePluginWorkspaceShellProgramWithVariablesDoesNotCreateNewPipelineOrSequence(t *testing.T) {
	plugin := &models.Plugin{
		ID:          88,
		Name:        "shell-demo",
		Runtime:     PluginRuntimeJSWorker,
		PackagePath: filepath.ToSlash(t.TempDir()),
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": ["runtime.file_system"]
		}`,
	}

	program, err := ResolvePluginWorkspaceShellProgramWithVariables(
		plugin,
		`cat $TARGET`,
		map[string]string{
			"TARGET": `notes/info.txt | help ; clear && ls`,
		},
	)
	if err != nil {
		t.Fatalf("ResolvePluginWorkspaceShellProgramWithVariables returned error: %v", err)
	}
	if len(program) != 1 {
		t.Fatalf("expected a single statement, got %+v", program)
	}
	if len(program[0].Stages) != 1 {
		t.Fatalf("expected a single pipeline stage, got %+v", program[0].Stages)
	}
	if len(program[0].Stages[0].Argv) != 1 || program[0].Stages[0].Argv[0] != `notes/info.txt | help ; clear && ls` {
		t.Fatalf("unexpected stage argv: %+v", program[0].Stages[0].Argv)
	}
}

func TestBuildPluginWorkspaceShellVariables(t *testing.T) {
	userID := uint(11)
	orderID := uint(22)
	plugin := &models.Plugin{
		ID:      88,
		Name:    "shell-demo",
		Runtime: PluginRuntimeJSWorker,
	}
	workspace := &PluginWorkspaceSnapshot{
		Status:          "waiting_input",
		ActiveTaskID:    "pex_123",
		ActiveCommand:   "debugger/prompt",
		ActiveCommandID: "cmd_123",
	}
	execCtx := &ExecutionContext{
		UserID:    &userID,
		OrderID:   &orderID,
		SessionID: "session-1",
		Metadata: map[string]string{
			PluginExecutionMetadataID: "pex_current",
		},
	}

	variables := BuildPluginWorkspaceShellVariables(plugin, 7, workspace, execCtx)
	expected := map[string]string{
		"PLUGIN_ID":         "88",
		"PLUGIN_NAME":       "shell-demo",
		"PLUGIN_RUNTIME":    PluginRuntimeJSWorker,
		"ADMIN_ID":          "7",
		"WORKSPACE_STATUS":  "waiting_input",
		"ACTIVE_TASK_ID":    "pex_123",
		"ACTIVE_COMMAND":    "debugger/prompt",
		"ACTIVE_COMMAND_ID": "cmd_123",
		"USER_ID":           "11",
		"ORDER_ID":          "22",
		"SESSION_ID":        "session-1",
		"TASK_ID":           "pex_current",
	}
	for key, value := range expected {
		if variables[key] != value {
			t.Fatalf("unexpected variable %s: got %q want %q", key, variables[key], value)
		}
	}
}

func TestExecutePluginWorkspaceShellCommandPipeline(t *testing.T) {
	artifactRoot := t.TempDir()
	pluginRoot := createPluginWorkspaceBuiltinTestPackage(t, artifactRoot, map[string]string{
		"notes/info.txt": "alpha\nbeta\n",
	})
	db := openPluginManagerE2ETestDB(t)
	service := NewPluginManagerService(db, &config.Config{Plugin: config.PluginPlatformConfig{
		Enabled:         true,
		AllowedRuntimes: []string{PluginRuntimeJSWorker},
		DefaultRuntime:  PluginRuntimeJSWorker,
		ArtifactDir:     artifactRoot,
	}})
	plugin := models.Plugin{
		ID:              92,
		Name:            "shell-pipeline",
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		PackagePath:     pluginRoot,
		Address:         filepath.ToSlash(filepath.Join(pluginRoot, "index.js")),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": ["runtime.file_system"]
		}`,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("prepare pipeline plugin failed: %v", err)
	}

	result, err := service.ExecutePluginWorkspaceShellCommand(
		plugin.ID,
		1,
		`cat notes/info.txt | grep beta`,
		nil,
		&ExecutionContext{},
	)
	if err != nil {
		t.Fatalf("ExecutePluginWorkspaceShellCommand returned error: %v", err)
	}
	output, _ := result.Data["output"].(string)
	if !strings.Contains(output, "beta") {
		t.Fatalf("expected pipeline grep output to contain beta, got %q", output)
	}
	stageCount, _ := result.Data["pipeline_stage_count"].(int)
	if stageCount != 2 {
		t.Fatalf("expected pipeline stage count 2, got %#v", result.Data["pipeline_stage_count"])
	}
}

func TestExecutePluginWorkspaceShellCommandSequenceStopsAtFailedAndConditionallySkips(t *testing.T) {
	artifactRoot := t.TempDir()
	pluginRoot := createPluginWorkspaceBuiltinTestPackage(t, artifactRoot, map[string]string{
		"notes/info.txt": "alpha\nbeta\n",
	})
	db := openPluginManagerE2ETestDB(t)
	service := NewPluginManagerService(db, &config.Config{Plugin: config.PluginPlatformConfig{
		Enabled:         true,
		AllowedRuntimes: []string{PluginRuntimeJSWorker},
		DefaultRuntime:  PluginRuntimeJSWorker,
		ArtifactDir:     artifactRoot,
	}})
	plugin := models.Plugin{
		ID:              93,
		Name:            "shell-sequence-skip",
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		PackagePath:     pluginRoot,
		Address:         filepath.ToSlash(filepath.Join(pluginRoot, "index.js")),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": ["runtime.file_system"]
		}`,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("prepare sequence plugin failed: %v", err)
	}

	result, err := service.ExecutePluginWorkspaceShellCommand(
		plugin.ID,
		1,
		`cat missing.txt && help`,
		nil,
		&ExecutionContext{},
	)
	if err != nil {
		t.Fatalf("ExecutePluginWorkspaceShellCommand returned error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected sequence with missing file to fail")
	}
	statementCount, _ := result.Data["statement_count"].(int)
	if statementCount != 2 {
		t.Fatalf("expected statement count 2, got %#v", result.Data["statement_count"])
	}
	statementResults, ok := result.Data["statement_results"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected statement results, got %#v", result.Data["statement_results"])
	}
	if len(statementResults) != 2 {
		t.Fatalf("expected 2 statement results, got %+v", statementResults)
	}
	if executed, _ := statementResults[1]["executed"].(bool); executed {
		t.Fatalf("expected second statement to be skipped, got %+v", statementResults[1])
	}
	if skipped, _ := statementResults[1]["skipped"].(bool); !skipped {
		t.Fatalf("expected second statement to be marked skipped, got %+v", statementResults[1])
	}
}

func TestExecutePluginWorkspaceShellCommandSequenceContinuesAfterSemicolonFailure(t *testing.T) {
	artifactRoot := t.TempDir()
	pluginRoot := createPluginWorkspaceBuiltinTestPackage(t, artifactRoot, map[string]string{
		"notes/info.txt": "alpha\nbeta\n",
	})
	db := openPluginManagerE2ETestDB(t)
	service := NewPluginManagerService(db, &config.Config{Plugin: config.PluginPlatformConfig{
		Enabled:         true,
		AllowedRuntimes: []string{PluginRuntimeJSWorker},
		DefaultRuntime:  PluginRuntimeJSWorker,
		ArtifactDir:     artifactRoot,
	}})
	plugin := models.Plugin{
		ID:              94,
		Name:            "shell-sequence-continue",
		Type:            "custom",
		Runtime:         PluginRuntimeJSWorker,
		PackagePath:     pluginRoot,
		Address:         filepath.ToSlash(filepath.Join(pluginRoot, "index.js")),
		Enabled:         true,
		Status:          "unknown",
		LifecycleStatus: models.PluginLifecycleInstalled,
		Capabilities: `{
			"requested_permissions": ["runtime.file_system"],
			"granted_permissions": ["runtime.file_system"]
		}`,
	}
	if err := db.Create(&plugin).Error; err != nil {
		t.Fatalf("prepare semicolon sequence plugin failed: %v", err)
	}

	result, err := service.ExecutePluginWorkspaceShellCommand(
		plugin.ID,
		1,
		`cat missing.txt ; help`,
		nil,
		&ExecutionContext{},
	)
	if err != nil {
		t.Fatalf("ExecutePluginWorkspaceShellCommand returned error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected sequence to report failure because the first statement failed")
	}
	statementResults, ok := result.Data["statement_results"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected statement results, got %#v", result.Data["statement_results"])
	}
	if len(statementResults) != 2 {
		t.Fatalf("expected 2 statement results, got %+v", statementResults)
	}
	if executed, _ := statementResults[1]["executed"].(bool); !executed {
		t.Fatalf("expected second statement to execute after semicolon, got %+v", statementResults[1])
	}
	statementData, _ := statementResults[1]["data"].(map[string]interface{})
	output, _ := statementData["output"].(string)
	if !strings.Contains(output, "help") {
		t.Fatalf("expected second statement output to contain help listing, got %#v", statementData)
	}
}
