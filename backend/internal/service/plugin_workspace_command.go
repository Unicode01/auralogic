package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"auralogic/internal/models"
)

const (
	pluginWorkspaceCommandExecuteAction    = "workspace.command.execute"
	pluginWorkspaceCommandNameParam        = "workspace_command_name"
	pluginWorkspaceCommandEntryParam       = "workspace_command_entry"
	pluginWorkspaceCommandIDParam          = "workspace_command_id"
	pluginWorkspaceCommandRawParam         = "workspace_command_raw"
	pluginWorkspaceCommandArgvJSONParam    = "workspace_command_argv_json"
	pluginWorkspaceCommandInputJSONParam   = "workspace_command_input_lines_json"
	pluginWorkspaceCommandInteractiveParam = "workspace_command_interactive"
)

type PluginWorkspaceCommand struct {
	Name               string   `json:"name"`
	Title              string   `json:"title,omitempty"`
	Description        string   `json:"description,omitempty"`
	Entry              string   `json:"entry,omitempty"`
	Interactive        bool     `json:"interactive"`
	Builtin            bool     `json:"builtin,omitempty"`
	Permissions        []string `json:"permissions,omitempty"`
	MissingPermissions []string `json:"missing_permissions,omitempty"`
	Granted            bool     `json:"granted"`
}

func ResolvePluginWorkspaceCommands(plugin *models.Plugin) []PluginWorkspaceCommand {
	if plugin == nil {
		return nil
	}
	if strings.TrimSpace(strings.ToLower(plugin.Runtime)) != PluginRuntimeJSWorker {
		return nil
	}

	candidates := resolvePluginWorkspaceBuiltinCommands(plugin)
	seen := make(map[string]struct{}, len(candidates))
	out := make([]PluginWorkspaceCommand, 0, len(candidates))
	for _, item := range candidates {
		normalizedName := strings.ToLower(strings.TrimSpace(item.Name))
		if normalizedName == "" {
			continue
		}
		if _, exists := seen[normalizedName]; exists {
			continue
		}
		seen[normalizedName] = struct{}{}
		out = append(out, item)
	}
	return out
}

func (s *PluginManagerService) ResolvePluginWorkspaceCommandsForPlugin(
	plugin *models.Plugin,
) []PluginWorkspaceCommand {
	return ResolvePluginWorkspaceCommands(plugin)
}

func (s *PluginManagerService) ResolvePluginWorkspaceCommandForPlugin(
	plugin *models.Plugin,
	commandName string,
) (*PluginWorkspaceCommand, error) {
	normalizedName := strings.ToLower(strings.TrimSpace(commandName))
	if normalizedName == "" {
		return nil, fmt.Errorf("workspace command name is required")
	}
	commands := s.ResolvePluginWorkspaceCommandsForPlugin(plugin)
	for idx := range commands {
		if strings.ToLower(strings.TrimSpace(commands[idx].Name)) != normalizedName {
			continue
		}
		command := commands[idx]
		return &command, nil
	}
	return nil, fmt.Errorf("workspace command %q is not available", commandName)
}

func ResolvePluginWorkspaceCommand(plugin *models.Plugin, commandName string) (*PluginWorkspaceCommand, error) {
	normalizedName := strings.ToLower(strings.TrimSpace(commandName))
	if normalizedName == "" {
		return nil, fmt.Errorf("workspace command name is required")
	}
	commands := ResolvePluginWorkspaceCommands(plugin)
	for idx := range commands {
		if strings.ToLower(strings.TrimSpace(commands[idx].Name)) != normalizedName {
			continue
		}
		command := commands[idx]
		return &command, nil
	}
	return nil, fmt.Errorf("workspace command %q is not available", commandName)
}

func buildPluginWorkspaceCommandParams(command *PluginWorkspaceCommand, argv []string, inputLines []string, commandID string) (map[string]string, error) {
	if command == nil {
		return nil, fmt.Errorf("workspace command is nil")
	}

	cleanArgv := make([]string, 0, len(argv))
	for _, item := range argv {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		cleanArgv = append(cleanArgv, trimmed)
	}
	argvJSON, err := json.Marshal(cleanArgv)
	if err != nil {
		return nil, fmt.Errorf("encode workspace command argv failed: %w", err)
	}

	cleanInputLines := make([]string, 0, len(inputLines))
	for _, item := range inputLines {
		trimmed := strings.TrimRight(item, "\r\n")
		if trimmed == "" {
			continue
		}
		cleanInputLines = append(cleanInputLines, trimmed)
	}
	inputJSON, err := json.Marshal(cleanInputLines)
	if err != nil {
		return nil, fmt.Errorf("encode workspace command input lines failed: %w", err)
	}

	rawParts := append([]string{strings.TrimSpace(command.Name)}, cleanArgv...)
	params := map[string]string{
		pluginWorkspaceCommandNameParam:        strings.TrimSpace(command.Name),
		pluginWorkspaceCommandEntryParam:       strings.TrimSpace(command.Entry),
		pluginWorkspaceCommandIDParam:          strings.TrimSpace(commandID),
		pluginWorkspaceCommandRawParam:         strings.TrimSpace(strings.Join(rawParts, " ")),
		pluginWorkspaceCommandArgvJSONParam:    string(argvJSON),
		pluginWorkspaceCommandInputJSONParam:   string(inputJSON),
		pluginWorkspaceCommandInteractiveParam: fmt.Sprintf("%t", command.Interactive),
	}
	return params, nil
}

func (s *PluginManagerService) ExecutePluginWorkspaceCommand(
	pluginID uint,
	adminID uint,
	commandName string,
	argv []string,
	inputLines []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
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
	return s.executePluginWorkspaceCommandResolved(
		plugin,
		runtime,
		capabilityPolicy,
		adminID,
		command,
		argv,
		inputLines,
		execCtx,
	)
}

func (s *PluginManagerService) executePluginWorkspaceCommandResolved(
	plugin *models.Plugin,
	runtime string,
	capabilityPolicy pluginCapabilityPolicy,
	adminID uint,
	command *PluginWorkspaceCommand,
	argv []string,
	inputLines []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	if plugin == nil {
		return nil, fmt.Errorf("plugin is nil")
	}
	if strings.TrimSpace(strings.ToLower(runtime)) != PluginRuntimeJSWorker {
		return nil, fmt.Errorf("workspace commands are only available for js_worker plugins")
	}
	if command == nil {
		return nil, fmt.Errorf("workspace command is required")
	}
	if len(command.MissingPermissions) > 0 {
		return nil, fmt.Errorf(
			"workspace command %s is missing required permissions: %s",
			command.Name,
			strings.Join(command.MissingPermissions, ", "),
		)
	}

	preparedExecCtx := clonePluginExecutionContext(execCtx)
	if preparedExecCtx == nil {
		preparedExecCtx = &ExecutionContext{}
	}
	taskID := EnsurePluginExecutionMetadata(preparedExecCtx, false)
	params, err := buildPluginWorkspaceCommandParams(command, argv, inputLines, taskID)
	if err != nil {
		return nil, err
	}
	if preparedExecCtx.Metadata == nil {
		preparedExecCtx.Metadata = make(map[string]string)
	}
	preparedExecCtx.Metadata["workspace_command"] = strings.TrimSpace(command.Name)
	preparedExecCtx.Metadata["workspace_command_entry"] = strings.TrimSpace(command.Entry)
	preparedExecCtx.Metadata["workspace_command_interactive"] = fmt.Sprintf("%t", command.Interactive)
	preparedExecCtx.Metadata["workspace_command_builtin"] = fmt.Sprintf("%t", command.Builtin)

	s.workspaceMu.Lock()
	buffer := s.ensurePluginWorkspaceBufferLocked(plugin.ID, plugin.Name, runtime)
	if err := buffer.ensureControl(adminID); err != nil {
		s.workspaceMu.Unlock()
		return nil, err
	}
	s.workspaceMu.Unlock()

	if command.Builtin {
		return s.executePluginWorkspaceBuiltinCommand(
			plugin,
			runtime,
			command,
			argv,
			inputLines,
			preparedExecCtx,
		)
	}

	return s.executePluginResolvedWithTimeout(
		plugin,
		runtime,
		capabilityPolicy,
		pluginWorkspaceCommandExecuteAction,
		params,
		preparedExecCtx,
		pluginWorkspaceCommandTimeout(command),
	)
}

func (s *PluginManagerService) ExecutePluginWorkspaceShellCommand(
	pluginID uint,
	adminID uint,
	commandLine string,
	inputLines []string,
	execCtx *ExecutionContext,
) (*ExecutionResult, error) {
	plugin, runtime, capabilityPolicy, err, _ := s.getPluginByIDWithCatalog(pluginID)
	if err != nil {
		return nil, err
	}
	if runtime != PluginRuntimeJSWorker {
		return nil, fmt.Errorf("workspace commands are only available for js_worker plugins")
	}

	workspaceSnapshot := s.GetPluginWorkspaceSnapshot(plugin, 0)
	shellVariables := BuildPluginWorkspaceShellVariables(plugin, adminID, &workspaceSnapshot, execCtx)
	statements, err := s.ResolvePluginWorkspaceShellProgramWithVariablesForPlugin(plugin, commandLine, shellVariables)
	if err != nil {
		return nil, err
	}
	if len(statements) == 0 {
		return nil, fmt.Errorf("workspace command line is empty")
	}

	trimmedCommandLine := strings.TrimSpace(commandLine)
	totalStageCount := 0
	for _, statement := range statements {
		totalStageCount += len(statement.Stages)
	}
	stageResults := make([]map[string]interface{}, 0, totalStageCount)
	statementResults := make([]map[string]interface{}, 0, len(statements))
	overallSuccess := true
	executedStatements := 0
	previousStatementSuccess := true
	firstFailure := ""
	var finalResult *ExecutionResult

	for statementIdx, statement := range statements {
		operator := string(statement.Operator)
		if len(statement.Stages) == 0 {
			return nil, fmt.Errorf("workspace command is required")
		}
		shouldExecute := statementIdx == 0 ||
			statement.Operator != pluginWorkspaceShellSequenceOperatorOnSuccess ||
			previousStatementSuccess
		if !shouldExecute {
			statementResults = append(statementResults, map[string]interface{}{
				"index":                statementIdx + 1,
				"operator":             operator,
				"raw":                  statement.Raw,
				"executed":             false,
				"skipped":              true,
				"skip_reason":          "previous_statement_failed",
				"pipeline_stage_count": len(statement.Stages),
				"success":              false,
			})
			overallSuccess = false
			previousStatementSuccess = false
			if firstFailure == "" {
				firstFailure = "previous statement failed"
			}
			continue
		}

		executedStatements++
		statementInputLines := append([]string(nil), inputLines...)
		if statementIdx > 0 {
			statementInputLines = nil
		}
		statementStageResults := make([]map[string]interface{}, 0, len(statement.Stages))
		statementSuccess := true
		statementError := ""
		var statementResult *ExecutionResult

		for stageIdx, stage := range statement.Stages {
			if stage.Command == nil {
				return nil, fmt.Errorf("workspace command is required")
			}
			if len(statement.Stages) > 1 && stage.Command.Interactive {
				statementSuccess = false
				statementError = fmt.Sprintf(
					"interactive workspace command %s cannot be used in a pipeline",
					stage.Command.Name,
				)
			} else if len(statements) > 1 && stage.Command.Interactive {
				statementSuccess = false
				statementError = fmt.Sprintf(
					"interactive workspace command %s cannot be used in a chained sequence",
					stage.Command.Name,
				)
			}
			if statementError != "" {
				overallSuccess = false
				if firstFailure == "" {
					firstFailure = statementError
				}
				stageEntry := map[string]interface{}{
					"statement_index": statementIdx + 1,
					"index":           len(stageResults) + 1,
					"command":         stage.Command.Name,
					"raw":             stage.Raw,
					"argv":            append([]string(nil), stage.Argv...),
					"executed":        false,
					"skipped":         false,
					"success":         false,
					"error":           statementError,
				}
				stageResults = append(stageResults, stageEntry)
				statementStageResults = append(statementStageResults, stageEntry)
				break
			}

			stageExecCtx := clonePluginExecutionContext(execCtx)
			if stageExecCtx == nil {
				stageExecCtx = &ExecutionContext{}
			}
			if stageExecCtx.Metadata == nil {
				stageExecCtx.Metadata = make(map[string]string, 6)
			}
			stageExecCtx.Metadata["workspace_command_line"] = trimmedCommandLine
			stageExecCtx.Metadata["workspace_statement"] = fmt.Sprintf("%d/%d", statementIdx+1, len(statements))
			stageExecCtx.Metadata["workspace_statement_operator"] = operator
			stageExecCtx.Metadata["workspace_statement_segment"] = statement.Raw
			stageExecCtx.Metadata["workspace_pipeline_stage"] = fmt.Sprintf("%d/%d", stageIdx+1, len(statement.Stages))
			stageExecCtx.Metadata["workspace_pipeline_segment"] = stage.Raw

			result, execErr := s.executePluginWorkspaceCommandResolved(
				plugin,
				runtime,
				capabilityPolicy,
				adminID,
				stage.Command,
				stage.Argv,
				statementInputLines,
				stageExecCtx,
			)
			stageEntry := map[string]interface{}{
				"statement_index": statementIdx + 1,
				"index":           len(stageResults) + 1,
				"command":         stage.Command.Name,
				"raw":             stage.Raw,
				"argv":            append([]string(nil), stage.Argv...),
				"executed":        true,
				"skipped":         false,
			}
			if result != nil {
				stageEntry["task_id"] = strings.TrimSpace(result.TaskID)
				stageEntry["data"] = clonePayloadMap(result.Data)
				stageEntry["success"] = result.Success
				if strings.TrimSpace(result.Error) != "" {
					stageEntry["error"] = strings.TrimSpace(result.Error)
				}
			} else {
				stageEntry["success"] = false
			}
			if execErr != nil {
				statementSuccess = false
				statementError = strings.TrimSpace(execErr.Error())
				stageEntry["success"] = false
				stageEntry["error"] = statementError
			}
			stageResults = append(stageResults, stageEntry)
			statementStageResults = append(statementStageResults, stageEntry)
			if execErr != nil {
				overallSuccess = false
				if firstFailure == "" {
					firstFailure = statementError
				}
				break
			}
			statementResult = result
			finalResult = result
			if result == nil || !result.Success {
				statementSuccess = false
				if result != nil {
					statementError = strings.TrimSpace(result.Error)
				}
				if statementError == "" {
					statementError = "workspace command failed"
				}
				overallSuccess = false
				if firstFailure == "" {
					firstFailure = statementError
				}
				break
			}
			statementInputLines = splitPluginWorkspaceShellPipeInputLines(
				extractPluginWorkspaceShellPipeOutput(result),
			)
		}

		statementTaskID := ""
		statementData := map[string]interface{}(nil)
		if statementResult != nil {
			statementTaskID = strings.TrimSpace(statementResult.TaskID)
			statementData = clonePayloadMap(statementResult.Data)
		}
		statementResults = append(statementResults, map[string]interface{}{
			"index":                statementIdx + 1,
			"operator":             operator,
			"raw":                  statement.Raw,
			"executed":             true,
			"skipped":              false,
			"success":              statementSuccess,
			"error":                statementError,
			"task_id":              statementTaskID,
			"data":                 statementData,
			"pipeline_stage_count": len(statement.Stages),
			"pipeline_stages":      statementStageResults,
		})
		previousStatementSuccess = statementSuccess
	}

	if finalResult == nil {
		finalResult = &ExecutionResult{
			Success: false,
			Error:   strings.TrimSpace(firstFailure),
			Data:    map[string]interface{}{},
		}
	}
	if !overallSuccess && strings.TrimSpace(firstFailure) != "" {
		finalResult.Error = strings.TrimSpace(firstFailure)
	}
	finalResult.Success = overallSuccess
	finalResult.Data = clonePayloadMap(finalResult.Data)
	if finalResult.Data == nil {
		finalResult.Data = make(map[string]interface{}, 8)
	}
	finalResult.Data["pipeline_stage_count"] = totalStageCount
	finalResult.Data["pipeline_stages"] = stageResults
	finalResult.Data["statement_count"] = len(statements)
	finalResult.Data["statement_executed_count"] = executedStatements
	finalResult.Data["statement_results"] = statementResults
	finalResult.Data["command_line"] = trimmedCommandLine
	if finalResult.Metadata == nil {
		finalResult.Metadata = make(map[string]string, 5)
	}
	finalResult.Metadata["workspace_command_line"] = trimmedCommandLine
	finalResult.Metadata["workspace_pipeline"] = fmt.Sprintf("%t", totalStageCount > 1)
	finalResult.Metadata["workspace_pipeline_stage_count"] = fmt.Sprintf("%d", totalStageCount)
	finalResult.Metadata["workspace_statement_count"] = fmt.Sprintf("%d", len(statements))
	finalResult.Metadata["workspace_sequence"] = fmt.Sprintf("%t", len(statements) > 1)
	return finalResult, nil
}
