package registrycli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"auralogic/market_registry/pkg/artifactmanifest"
	"auralogic/market_registry/pkg/registryruntime"
)

const defaultManifestPath = "manifest.json"

type localManifest struct {
	Path   string
	Values map[string]any
}

func loadOptionalLocalManifest(path string) (*localManifest, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		if _, err := os.Stat(defaultManifestPath); err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("stat local manifest: %w", err)
		}
		trimmed = defaultManifestPath
	}

	payload, err := os.ReadFile(trimmed)
	if err != nil {
		return nil, fmt.Errorf("read local manifest %s: %w", strings.TrimSpace(trimmed), err)
	}

	values := map[string]any{}
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil, fmt.Errorf("parse local manifest %s: %w", strings.TrimSpace(trimmed), err)
	}

	return &localManifest{
		Path:   strings.TrimSpace(trimmed),
		Values: values,
	}, nil
}

func applyLocalManifestAutofill(
	kind string,
	name string,
	version string,
	metadata registryruntime.Metadata,
	manifest *localManifest,
) (string, string, string, registryruntime.Metadata) {
	if manifest == nil || len(manifest.Values) == 0 {
		return strings.TrimSpace(kind), strings.TrimSpace(name), strings.TrimSpace(version), metadata
	}

	kind = artifactmanifest.FirstNonEmpty(kind, artifactmanifest.InferKind(manifest.Values))
	name = artifactmanifest.FirstNonEmpty(name, artifactmanifest.String(manifest.Values, "name"))
	version = artifactmanifest.FirstNonEmpty(version, artifactmanifest.String(manifest.Values, "version"))
	metadata.Title = artifactmanifest.FirstNonEmpty(metadata.Title, artifactmanifest.Title(manifest.Values))
	metadata.Summary = artifactmanifest.FirstNonEmpty(metadata.Summary, artifactmanifest.Summary(manifest.Values))
	metadata.Description = artifactmanifest.FirstNonEmpty(metadata.Description, artifactmanifest.Description(manifest.Values))

	return kind, name, version, metadata
}
