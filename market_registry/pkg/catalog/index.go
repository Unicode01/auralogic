package catalog

import (
	"path"
	"strings"
)

func ParseArtifactIndexPath(value string) (string, string, bool) {
	normalized := path.Clean(strings.TrimSpace(strings.ReplaceAll(value, "\\", "/")))
	parts := strings.Split(normalized, "/")
	if len(parts) != 5 {
		return "", "", false
	}
	if parts[0] != "index" || parts[1] != "artifacts" || parts[4] != "index.json" {
		return "", "", false
	}
	kind := strings.TrimSpace(parts[2])
	name := strings.TrimSpace(parts[3])
	if kind == "" || name == "" {
		return "", "", false
	}
	return kind, name, true
}

func NormalizeArtifactIndex(kind string, name string, index map[string]any) map[string]any {
	if index == nil {
		index = map[string]any{}
	}
	index["kind"] = firstNonEmptyString(index["kind"], kind)
	index["name"] = firstNonEmptyString(index["name"], name)
	versions := normalizeArtifactVersionList(index["versions"], index["releases"])
	index["versions"] = versions
	index["releases"] = versions
	topLevelChannels := dedupeStringValues(append(normalizedArtifactChannels(index["channels"]), collectArtifactChannels(versions)...)...)
	primaryChannel := firstNonEmptyString(index["channel"])
	if primaryChannel == "" && len(versions) > 0 {
		primaryChannel = ArtifactVersionPrimaryChannel(versions[0])
	}
	if primaryChannel != "" && !containsStringFold(topLevelChannels, primaryChannel) {
		topLevelChannels = append([]string{primaryChannel}, topLevelChannels...)
	}
	if len(topLevelChannels) > 0 {
		index["channels"] = topLevelChannels
		index["channel"] = firstNonEmpty(primaryChannel, topLevelChannels[0])
	}
	if firstNonEmptyString(index["latest_version"]) == "" && len(versions) > 0 {
		index["latest_version"] = firstNonEmptyString(versions[0]["version"])
	}
	return index
}

func NormalizeArtifactVersionEntries(value any) []map[string]any {
	return normalizeArtifactVersionList(value)
}

func ArtifactVersionChannels(entry map[string]any) []string {
	if len(entry) == 0 {
		return []string{}
	}
	channels := normalizedArtifactChannels(entry["channels"])
	primary := firstNonEmptyString(entry["channel"])
	if primary != "" && !containsStringFold(channels, primary) {
		channels = append([]string{primary}, channels...)
	}
	if len(channels) == 0 && primary != "" {
		return []string{primary}
	}
	return dedupeStringValues(channels...)
}

func ArtifactVersionPrimaryChannel(entry map[string]any) string {
	channels := ArtifactVersionChannels(entry)
	if len(channels) > 0 {
		return channels[0]
	}
	return firstNonEmptyString(entry["channel"])
}

func ArtifactVersionMatchesChannel(entry map[string]any, channel string) bool {
	target := strings.TrimSpace(channel)
	if target == "" {
		return true
	}
	for _, current := range ArtifactVersionChannels(entry) {
		if strings.EqualFold(current, target) {
			return true
		}
	}
	return false
}

func FindArtifactVersionEntry(entries []map[string]any, version string) map[string]any {
	target := strings.TrimSpace(version)
	if target == "" {
		return nil
	}
	for _, entry := range entries {
		if strings.EqualFold(firstNonEmptyString(entry["version"]), target) {
			return cloneVersionEntry(entry)
		}
	}
	return nil
}

func normalizeArtifactVersionList(values ...any) []map[string]any {
	rawEntries := make([]map[string]any, 0)
	for _, value := range values {
		rawEntries = append(rawEntries, rawArtifactVersionList(value)...)
	}
	if len(rawEntries) == 0 {
		return []map[string]any{}
	}

	merged := make([]map[string]any, 0, len(rawEntries))
	indexByVersion := map[string]int{}
	for _, raw := range rawEntries {
		entry := normalizeArtifactVersionEntry(raw)
		version := firstNonEmptyString(entry["version"])
		if version == "" {
			continue
		}
		key := strings.ToLower(version)
		if idx, exists := indexByVersion[key]; exists {
			merged[idx] = mergeArtifactVersionEntry(merged[idx], entry)
			continue
		}
		indexByVersion[key] = len(merged)
		merged = append(merged, entry)
	}
	return merged
}

func collectArtifactChannels(versions []map[string]any) []string {
	channels := make([]string, 0, len(versions))
	for _, version := range versions {
		channels = append(channels, ArtifactVersionChannels(version)...)
	}
	return dedupeStringValues(channels...)
}

func rawArtifactVersionList(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, cloneVersionEntry(item))
		}
		return out
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			mapped, ok := item.(map[string]any)
			if !ok || mapped == nil {
				continue
			}
			out = append(out, cloneVersionEntry(mapped))
		}
		return out
	default:
		return []map[string]any{}
	}
}

func normalizeArtifactVersionEntry(entry map[string]any) map[string]any {
	cloned := cloneVersionEntry(entry)
	channels := ArtifactVersionChannels(cloned)
	if len(channels) > 0 {
		cloned["channels"] = append([]string(nil), channels...)
		cloned["channel"] = channels[0]
	}
	return cloned
}

func mergeArtifactVersionEntry(base map[string]any, incoming map[string]any) map[string]any {
	out := cloneVersionEntry(base)
	mergedChannels := dedupeStringValues(append(ArtifactVersionChannels(base), ArtifactVersionChannels(incoming)...)...)
	if len(mergedChannels) > 0 {
		out["channels"] = mergedChannels
		out["channel"] = mergedChannels[0]
	}
	for _, key := range []string{"published_at", "sha256", "size", "content_type", "title", "summary", "description"} {
		if firstNonEmptyString(out[key]) == "" && firstNonEmptyString(incoming[key]) != "" {
			out[key] = incoming[key]
		}
		if _, exists := out[key]; !exists && incoming[key] != nil {
			out[key] = incoming[key]
		}
	}
	return out
}

func normalizedArtifactChannels(value any) []string {
	switch typed := value.(type) {
	case []string:
		return dedupeStringValues(typed...)
	case []any:
		values := make([]string, 0, len(typed))
		for _, item := range typed {
			text := firstNonEmptyString(item)
			if text == "" {
				continue
			}
			values = append(values, text)
		}
		return dedupeStringValues(values...)
	default:
		return []string{}
	}
}

func dedupeStringValues(values ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func containsStringFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func cloneVersionEntry(value map[string]any) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func latestVersionFromIndex(index map[string]any) string {
	if version := firstNonEmptyString(index["latest_version"]); version != "" {
		return version
	}
	versions := normalizeArtifactVersionList(index["versions"])
	if len(versions) == 0 {
		versions = normalizeArtifactVersionList(index["releases"])
	}
	if len(versions) == 0 {
		return ""
	}
	return firstNonEmptyString(versions[0]["version"])
}

func firstNonEmptyString(values ...any) string {
	for _, value := range values {
		text, _ := value.(string)
		if trimmed := strings.TrimSpace(text); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
