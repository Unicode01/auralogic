package artifactmanifest

import "testing"

func TestTitleUsesLocalizedDisplayName(t *testing.T) {
	manifest := map[string]any{
		"display_name": map[string]any{
			"zh-CN": "中文标题",
			"en":    "English Title",
		},
	}

	if got := Title(manifest); got != "English Title" {
		t.Fatalf("expected localized english display name, got %q", got)
	}
}

func TestSummaryFallsBackToLocalizedDescription(t *testing.T) {
	manifest := map[string]any{
		"description": map[string]any{
			"zh-CN": "中文说明",
			"en":    "Hosted checkout package",
		},
	}

	if got := Summary(manifest); got != "Hosted checkout package" {
		t.Fatalf("expected summary fallback from localized description, got %q", got)
	}
}

func TestDescriptionUsesLocalizedDescription(t *testing.T) {
	manifest := map[string]any{
		"description": map[string]any{
			"zh-CN": "中文说明",
		},
	}

	if got := Description(manifest); got != "中文说明" {
		t.Fatalf("expected localized description, got %q", got)
	}
}
