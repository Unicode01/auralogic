package service

import "testing"

func TestShouldSuppressPluginWorkspaceRuntimeResult(t *testing.T) {
	if !shouldSuppressPluginWorkspaceRuntimeResult(
		`console.log("a")`,
		false,
		map[string]interface{}{"type": "undefined"},
	) {
		t.Fatalf("expected console.log(undefined result) to be suppressed")
	}

	if !shouldSuppressPluginWorkspaceRuntimeResult(
		`Plugin.workspace.write("x")`,
		false,
		map[string]interface{}{"type": "undefined"},
	) {
		t.Fatalf("expected Plugin.workspace.write(undefined result) to be suppressed")
	}

	if shouldSuppressPluginWorkspaceRuntimeResult(
		`help()`,
		false,
		map[string]interface{}{"type": "object"},
	) {
		t.Fatalf("did not expect non-side-effect expression to be suppressed")
	}

	if shouldSuppressPluginWorkspaceRuntimeResult(
		`:inspect Plugin.host`,
		true,
		map[string]interface{}{"type": "undefined"},
	) {
		t.Fatalf("did not expect inspect mode to be suppressed")
	}
}

func TestSanitizePluginWorkspaceRuntimePreviewPayloadStripsRuntimeState(t *testing.T) {
	sanitized := sanitizePluginWorkspaceRuntimePreviewPayload(map[string]interface{}{
		"type":    "number",
		"summary": "2",
		"value":   2,
		"runtime_state": map[string]interface{}{
			"plugin_id":        12,
			"completion_paths": []string{"debug", "module.exports.execute"},
		},
	})
	if _, exists := sanitized["runtime_state"]; exists {
		t.Fatalf("expected runtime_state to be stripped from workspace preview metadata, got %#v", sanitized)
	}
	if got := sanitized["summary"]; got != "2" {
		t.Fatalf("expected summary to be preserved, got %#v", sanitized)
	}
}
