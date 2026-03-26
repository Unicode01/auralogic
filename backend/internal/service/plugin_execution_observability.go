package service

import (
	"regexp"
	"strings"
)

const pluginExecutionErrorSignatureMaxLength = 96

var pluginExecutionDigitsPattern = regexp.MustCompile(`\d+`)

func NormalizePluginExecutionErrorText(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "unknown error"
	}
	lines := strings.Split(trimmed, "\n")
	normalized := strings.Join(strings.Fields(strings.TrimSpace(lines[0])), " ")
	if normalized == "" {
		return "unknown error"
	}
	return normalized
}

func NormalizePluginExecutionErrorSignature(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "unknown error"
	}
	normalized = pluginExecutionDigitsPattern.ReplaceAllString(normalized, "#")
	normalized = strings.Join(strings.Fields(normalized), " ")
	if normalized == "" {
		return "unknown error"
	}
	if len(normalized) > pluginExecutionErrorSignatureMaxLength {
		return normalized[:pluginExecutionErrorSignatureMaxLength-1] + "…"
	}
	return normalized
}
