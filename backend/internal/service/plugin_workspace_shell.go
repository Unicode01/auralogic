package service

import (
	"encoding/json"
	"fmt"
	"strings"

	"auralogic/internal/models"
)

var pluginWorkspaceShellBuiltinAliases = map[string]string{
	"?":        pluginWorkspaceBuiltinCommandHelp,
	"help":     pluginWorkspaceBuiltinCommandHelp,
	"clear":    pluginWorkspaceBuiltinCommandClear,
	"log.tail": pluginWorkspaceBuiltinCommandLogTail,
	"pwd":      pluginWorkspaceBuiltinCommandPWD,
	"ls":       pluginWorkspaceBuiltinCommandLS,
	"stat":     pluginWorkspaceBuiltinCommandStat,
	"cat":      pluginWorkspaceBuiltinCommandCat,
	"mkdir":    pluginWorkspaceBuiltinCommandMkdir,
	"find":     pluginWorkspaceBuiltinCommandFind,
	"grep":     pluginWorkspaceBuiltinCommandGrep,
	"kv.get":   pluginWorkspaceBuiltinCommandKVGet,
	"kv.set":   pluginWorkspaceBuiltinCommandKVSet,
	"kv.list":  pluginWorkspaceBuiltinCommandKVList,
	"kv.del":   pluginWorkspaceBuiltinCommandKVDel,
}

type PluginWorkspaceShellSequenceOperator string

const (
	pluginWorkspaceShellSequenceOperatorNone      PluginWorkspaceShellSequenceOperator = ""
	pluginWorkspaceShellSequenceOperatorAlways    PluginWorkspaceShellSequenceOperator = ";"
	pluginWorkspaceShellSequenceOperatorOnSuccess PluginWorkspaceShellSequenceOperator = "&&"
)

type PluginWorkspaceShellSequenceSegment struct {
	Raw      string
	Operator PluginWorkspaceShellSequenceOperator
}

type PluginWorkspaceShellResolvedCommand struct {
	Raw     string
	Command *PluginWorkspaceCommand
	Argv    []string
}

type PluginWorkspaceShellResolvedPipeline struct {
	Raw      string
	Operator PluginWorkspaceShellSequenceOperator
	Stages   []PluginWorkspaceShellResolvedCommand
}

func ParsePluginWorkspaceShellLine(input string) ([]string, error) {
	return ParsePluginWorkspaceShellLineWithVariables(input, nil)
}

func ParsePluginWorkspaceShellLineWithVariables(input string, variables map[string]string) ([]string, error) {
	argv := make([]string, 0, 8)
	var current strings.Builder
	quote := rune(0)
	escaping := false
	tokenActive := false
	normalizedVariables := normalizePluginWorkspaceShellVariables(variables)

	runes := []rune(strings.TrimSpace(input))
	for idx := 0; idx < len(runes); idx++ {
		ch := runes[idx]
		if escaping {
			current.WriteRune(ch)
			escaping = false
			tokenActive = true
			continue
		}

		if ch == '\\' && quote != '\'' {
			escaping = true
			tokenActive = true
			continue
		}

		if quote != 0 {
			if ch == quote {
				quote = 0
			} else if quote == '"' && ch == '$' {
				variableName, nextIndex, ok, variableErr := parsePluginWorkspaceShellVariableReference(runes, idx+1)
				if variableErr != nil {
					return nil, variableErr
				}
				if ok {
					current.WriteString(resolvePluginWorkspaceShellVariableValue(normalizedVariables, variableName))
					tokenActive = true
					idx = nextIndex - 1
					continue
				}
				current.WriteRune(ch)
			} else {
				current.WriteRune(ch)
			}
			tokenActive = true
			continue
		}

		if ch == '"' || ch == '\'' {
			quote = ch
			tokenActive = true
			continue
		}
		if ch == '$' && quote != '\'' {
			variableName, nextIndex, ok, variableErr := parsePluginWorkspaceShellVariableReference(runes, idx+1)
			if variableErr != nil {
				return nil, variableErr
			}
			if ok {
				current.WriteString(resolvePluginWorkspaceShellVariableValue(normalizedVariables, variableName))
				tokenActive = true
				idx = nextIndex - 1
				continue
			}
		}

		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			if tokenActive {
				argv = append(argv, current.String())
				current.Reset()
				tokenActive = false
			}
			continue
		}

		current.WriteRune(ch)
		tokenActive = true
	}

	if escaping {
		current.WriteRune('\\')
		tokenActive = true
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in workspace command line")
	}
	if tokenActive {
		argv = append(argv, current.String())
	}
	if len(argv) == 0 {
		return nil, fmt.Errorf("workspace command line is empty")
	}
	return argv, nil
}

func NormalizePluginWorkspaceShellCommandName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	if alias, exists := pluginWorkspaceShellBuiltinAliases[lower]; exists {
		return alias
	}
	return trimmed
}

func ResolvePluginWorkspaceShellCommand(
	plugin *models.Plugin,
	commandLine string,
) (*PluginWorkspaceCommand, []string, error) {
	return ResolvePluginWorkspaceShellCommandWithVariables(plugin, commandLine, nil)
}

func ResolvePluginWorkspaceShellCommandWithVariables(
	plugin *models.Plugin,
	commandLine string,
	variables map[string]string,
) (*PluginWorkspaceCommand, []string, error) {
	return resolvePluginWorkspaceShellCommandWithVariables(
		func(commandName string) (*PluginWorkspaceCommand, error) {
			return ResolvePluginWorkspaceCommand(plugin, commandName)
		},
		commandLine,
		variables,
	)
}

func resolvePluginWorkspaceShellCommandWithVariables(
	resolveCommand func(commandName string) (*PluginWorkspaceCommand, error),
	commandLine string,
	variables map[string]string,
) (*PluginWorkspaceCommand, []string, error) {
	argv, err := ParsePluginWorkspaceShellLineWithVariables(commandLine, variables)
	if err != nil {
		return nil, nil, err
	}
	commandName := NormalizePluginWorkspaceShellCommandName(argv[0])
	command, err := resolveCommand(commandName)
	if err != nil {
		return nil, nil, err
	}
	return command, argv[1:], nil
}

func ParsePluginWorkspaceShellSequence(commandLine string) ([]PluginWorkspaceShellSequenceSegment, error) {
	trimmed := strings.TrimSpace(commandLine)
	if trimmed == "" {
		return nil, fmt.Errorf("workspace command line is empty")
	}

	segments := make([]PluginWorkspaceShellSequenceSegment, 0, 4)
	var current strings.Builder
	quote := rune(0)
	escaping := false
	pendingOperator := pluginWorkspaceShellSequenceOperatorNone
	runes := []rune(trimmed)

	pushSegment := func(nextOperator PluginWorkspaceShellSequenceOperator) error {
		segment := strings.TrimSpace(current.String())
		current.Reset()
		if segment == "" {
			return fmt.Errorf("workspace command sequence contains an empty statement")
		}
		segments = append(segments, PluginWorkspaceShellSequenceSegment{
			Raw:      segment,
			Operator: pendingOperator,
		})
		pendingOperator = nextOperator
		return nil
	}

	for idx := 0; idx < len(runes); idx++ {
		ch := runes[idx]
		if escaping {
			current.WriteRune(ch)
			escaping = false
			continue
		}
		if ch == '\\' && quote != '\'' {
			escaping = true
			current.WriteRune(ch)
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			current.WriteRune(ch)
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			current.WriteRune(ch)
			continue
		}
		if ch == ';' {
			if err := pushSegment(pluginWorkspaceShellSequenceOperatorAlways); err != nil {
				return nil, err
			}
			continue
		}
		if ch == '&' && idx+1 < len(runes) && runes[idx+1] == '&' {
			if err := pushSegment(pluginWorkspaceShellSequenceOperatorOnSuccess); err != nil {
				return nil, err
			}
			idx++
			continue
		}
		if ch == '|' && idx+1 < len(runes) && runes[idx+1] == '|' {
			return nil, fmt.Errorf("workspace shell operator || is not supported")
		}
		current.WriteRune(ch)
	}

	if escaping {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in workspace command line")
	}
	if err := pushSegment(pluginWorkspaceShellSequenceOperatorNone); err != nil {
		return nil, err
	}
	return segments, nil
}

func ParsePluginWorkspaceShellPipeline(commandLine string) ([]string, error) {
	segments := make([]string, 0, 4)
	var current strings.Builder
	quote := rune(0)
	escaping := false

	pushSegment := func() error {
		segment := strings.TrimSpace(current.String())
		current.Reset()
		if segment == "" {
			return fmt.Errorf("workspace pipeline contains an empty command segment")
		}
		segments = append(segments, segment)
		return nil
	}

	for _, ch := range strings.TrimSpace(commandLine) {
		if escaping {
			current.WriteRune(ch)
			escaping = false
			continue
		}
		if ch == '\\' && quote != '\'' {
			escaping = true
			current.WriteRune(ch)
			continue
		}
		if quote != 0 {
			if ch == quote {
				quote = 0
			}
			current.WriteRune(ch)
			continue
		}
		if ch == '"' || ch == '\'' {
			quote = ch
			current.WriteRune(ch)
			continue
		}
		if ch == '|' {
			if err := pushSegment(); err != nil {
				return nil, err
			}
			continue
		}
		current.WriteRune(ch)
	}

	if escaping {
		current.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in workspace command line")
	}
	if err := pushSegment(); err != nil {
		return nil, err
	}
	return segments, nil
}

func ResolvePluginWorkspaceShellProgram(
	plugin *models.Plugin,
	commandLine string,
) ([]PluginWorkspaceShellResolvedPipeline, error) {
	return ResolvePluginWorkspaceShellProgramWithVariables(plugin, commandLine, nil)
}

func ResolvePluginWorkspaceShellProgramWithVariables(
	plugin *models.Plugin,
	commandLine string,
	variables map[string]string,
) ([]PluginWorkspaceShellResolvedPipeline, error) {
	return resolvePluginWorkspaceShellProgramWithVariables(
		func(commandName string) (*PluginWorkspaceCommand, error) {
			return ResolvePluginWorkspaceCommand(plugin, commandName)
		},
		commandLine,
		variables,
	)
}

func resolvePluginWorkspaceShellProgramWithVariables(
	resolveCommand func(commandName string) (*PluginWorkspaceCommand, error),
	commandLine string,
	variables map[string]string,
) ([]PluginWorkspaceShellResolvedPipeline, error) {
	segments, err := ParsePluginWorkspaceShellSequence(commandLine)
	if err != nil {
		return nil, err
	}
	out := make([]PluginWorkspaceShellResolvedPipeline, 0, len(segments))
	for _, segment := range segments {
		stages, resolveErr := resolvePluginWorkspaceShellPipelineWithVariables(resolveCommand, segment.Raw, variables)
		if resolveErr != nil {
			return nil, resolveErr
		}
		out = append(out, PluginWorkspaceShellResolvedPipeline{
			Raw:      segment.Raw,
			Operator: segment.Operator,
			Stages:   stages,
		})
	}
	return out, nil
}

func ResolvePluginWorkspaceShellPipeline(
	plugin *models.Plugin,
	commandLine string,
) ([]PluginWorkspaceShellResolvedCommand, error) {
	return ResolvePluginWorkspaceShellPipelineWithVariables(plugin, commandLine, nil)
}

func ResolvePluginWorkspaceShellPipelineWithVariables(
	plugin *models.Plugin,
	commandLine string,
	variables map[string]string,
) ([]PluginWorkspaceShellResolvedCommand, error) {
	return resolvePluginWorkspaceShellPipelineWithVariables(
		func(commandName string) (*PluginWorkspaceCommand, error) {
			return ResolvePluginWorkspaceCommand(plugin, commandName)
		},
		commandLine,
		variables,
	)
}

func resolvePluginWorkspaceShellPipelineWithVariables(
	resolveCommand func(commandName string) (*PluginWorkspaceCommand, error),
	commandLine string,
	variables map[string]string,
) ([]PluginWorkspaceShellResolvedCommand, error) {
	segments, err := ParsePluginWorkspaceShellPipeline(commandLine)
	if err != nil {
		return nil, err
	}
	out := make([]PluginWorkspaceShellResolvedCommand, 0, len(segments))
	for _, segment := range segments {
		command, argv, resolveErr := resolvePluginWorkspaceShellCommandWithVariables(resolveCommand, segment, variables)
		if resolveErr != nil {
			return nil, resolveErr
		}
		out = append(out, PluginWorkspaceShellResolvedCommand{
			Raw:     segment,
			Command: command,
			Argv:    argv,
		})
	}
	return out, nil
}

func (s *PluginManagerService) ResolvePluginWorkspaceShellCommandWithVariablesForPlugin(
	plugin *models.Plugin,
	commandLine string,
	variables map[string]string,
) (*PluginWorkspaceCommand, []string, error) {
	return resolvePluginWorkspaceShellCommandWithVariables(
		func(commandName string) (*PluginWorkspaceCommand, error) {
			return s.ResolvePluginWorkspaceCommandForPlugin(plugin, commandName)
		},
		commandLine,
		variables,
	)
}

func (s *PluginManagerService) ResolvePluginWorkspaceShellProgramWithVariablesForPlugin(
	plugin *models.Plugin,
	commandLine string,
	variables map[string]string,
) ([]PluginWorkspaceShellResolvedPipeline, error) {
	return resolvePluginWorkspaceShellProgramWithVariables(
		func(commandName string) (*PluginWorkspaceCommand, error) {
			return s.ResolvePluginWorkspaceCommandForPlugin(plugin, commandName)
		},
		commandLine,
		variables,
	)
}

func BuildPluginWorkspaceShellVariables(
	plugin *models.Plugin,
	adminID uint,
	workspace *PluginWorkspaceSnapshot,
	execCtx *ExecutionContext,
) map[string]string {
	variables := make(map[string]string, 12)
	if plugin != nil {
		variables["PLUGIN_ID"] = fmt.Sprintf("%d", plugin.ID)
		variables["PLUGIN_NAME"] = strings.TrimSpace(plugin.Name)
		variables["PLUGIN_RUNTIME"] = strings.TrimSpace(plugin.Runtime)
	}
	if adminID > 0 {
		variables["ADMIN_ID"] = fmt.Sprintf("%d", adminID)
	}
	if workspace != nil {
		variables["WORKSPACE_STATUS"] = strings.TrimSpace(workspace.Status)
		variables["ACTIVE_TASK_ID"] = strings.TrimSpace(workspace.ActiveTaskID)
		variables["ACTIVE_COMMAND"] = strings.TrimSpace(workspace.ActiveCommand)
		variables["ACTIVE_COMMAND_ID"] = strings.TrimSpace(workspace.ActiveCommandID)
	}
	if execCtx != nil {
		if execCtx.UserID != nil {
			variables["USER_ID"] = fmt.Sprintf("%d", *execCtx.UserID)
		}
		if execCtx.OrderID != nil {
			variables["ORDER_ID"] = fmt.Sprintf("%d", *execCtx.OrderID)
		}
		if trimmed := strings.TrimSpace(execCtx.SessionID); trimmed != "" {
			variables["SESSION_ID"] = trimmed
		}
		if execCtx.Metadata != nil {
			if trimmed := strings.TrimSpace(execCtx.Metadata[PluginExecutionMetadataID]); trimmed != "" {
				variables["TASK_ID"] = trimmed
			}
		}
	}
	return variables
}

func normalizePluginWorkspaceShellVariables(variables map[string]string) map[string]string {
	if len(variables) == 0 {
		return nil
	}
	out := make(map[string]string, len(variables))
	for key, value := range variables {
		normalizedKey := strings.ToUpper(strings.TrimSpace(key))
		if normalizedKey == "" {
			continue
		}
		out[normalizedKey] = value
	}
	return out
}

func resolvePluginWorkspaceShellVariableValue(variables map[string]string, name string) string {
	if len(variables) == 0 {
		return ""
	}
	return variables[strings.ToUpper(strings.TrimSpace(name))]
}

func parsePluginWorkspaceShellVariableReference(
	runes []rune,
	start int,
) (string, int, bool, error) {
	if start >= len(runes) {
		return "", start, false, nil
	}
	if runes[start] == '{' {
		end := start + 1
		for end < len(runes) && runes[end] != '}' {
			end++
		}
		if end >= len(runes) {
			return "", 0, false, fmt.Errorf("unterminated workspace shell variable reference")
		}
		name := string(runes[start+1 : end])
		if !isValidPluginWorkspaceShellVariableName(name) {
			return "", 0, false, fmt.Errorf("invalid workspace shell variable name %q", name)
		}
		return name, end + 1, true, nil
	}
	if !isPluginWorkspaceShellVariableStart(runes[start]) {
		return "", start, false, nil
	}
	end := start + 1
	for end < len(runes) && isPluginWorkspaceShellVariablePart(runes[end]) {
		end++
	}
	return string(runes[start:end]), end, true, nil
}

func isValidPluginWorkspaceShellVariableName(name string) bool {
	runes := []rune(strings.TrimSpace(name))
	if len(runes) == 0 || !isPluginWorkspaceShellVariableStart(runes[0]) {
		return false
	}
	for _, ch := range runes[1:] {
		if !isPluginWorkspaceShellVariablePart(ch) {
			return false
		}
	}
	return true
}

func isPluginWorkspaceShellVariableStart(ch rune) bool {
	return ch == '_' ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z')
}

func isPluginWorkspaceShellVariablePart(ch rune) bool {
	return isPluginWorkspaceShellVariableStart(ch) || (ch >= '0' && ch <= '9')
}

func splitPluginWorkspaceShellPipeInputLines(value string) []string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if strings.TrimSpace(normalized) == "" {
		return nil
	}
	return strings.Split(normalized, "\n")
}

func extractPluginWorkspaceShellPipeOutput(result *ExecutionResult) string {
	if result == nil {
		return ""
	}
	if output, ok := result.Data["output"].(string); ok && strings.TrimSpace(output) != "" {
		return output
	}
	if content, ok := result.Data["content"].(string); ok && strings.TrimSpace(content) != "" {
		return content
	}
	if value, ok := result.Data["value"].(string); ok && strings.TrimSpace(value) != "" {
		return value
	}
	if len(result.Data) == 0 {
		return strings.TrimSpace(result.Error)
	}
	encoded, err := json.Marshal(result.Data)
	if err != nil {
		return strings.TrimSpace(result.Error)
	}
	return string(encoded)
}
