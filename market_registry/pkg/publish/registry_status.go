package publish

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"auralogic/market_registry/pkg/catalog"
)

const publicStatsPath = "index/stats/public.json"

type RebuildResult struct {
	SourcePath     string
	CatalogPath    string
	TotalArtifacts int
	GeneratedAt    string
}

type SnapshotStatus struct {
	Path        string   `json:"path"`
	Exists      bool     `json:"exists"`
	Stale       bool     `json:"stale"`
	Status      string   `json:"status"`
	GeneratedAt string   `json:"generatedAt,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
	ItemCount   int      `json:"itemCount,omitempty"`
	Issues      []string `json:"issues,omitempty"`
}

type RegistryStatus struct {
	Healthy       bool           `json:"healthy"`
	Status        string         `json:"status"`
	Message       string         `json:"message"`
	CheckedAt     string         `json:"checkedAt"`
	ArtifactCount int            `json:"artifactCount"`
	Issues        []string       `json:"issues"`
	Source        SnapshotStatus `json:"source"`
	Catalog       SnapshotStatus `json:"catalog"`
	Stats         SnapshotStatus `json:"stats"`
}

func (s *Service) RebuildRegistry(ctx context.Context) (RebuildResult, error) {
	runtimeStore := catalog.NewRuntimeStore(s.storage, s.signing, catalog.RuntimeStoreConfig{
		SourceID:   s.sourceID,
		SourceName: s.sourceName,
		KeyID:      s.keyID,
		Settings:   s.settings,
		Now:        time.Now,
	})

	sourceDocument, err := runtimeStore.BuildSourceDocument(ctx, s.registryBaseURL())
	if err != nil {
		return RebuildResult{}, err
	}
	sourcePath := "index/source.json"
	if err := s.writeJSON(ctx, sourcePath, sourceDocument); err != nil {
		return RebuildResult{}, err
	}

	catalogSnapshot, err := runtimeStore.BuildCatalogSnapshot(ctx, s.registryBaseURL())
	if err != nil {
		return RebuildResult{}, err
	}
	catalogPath := "index/catalog.json"
	if err := s.writeJSON(ctx, catalogPath, catalogSnapshot); err != nil {
		return RebuildResult{}, err
	}

	totalArtifacts, _ := catalogSnapshot["total"].(int)
	generatedAt, _ := catalogSnapshot["generated_at"].(string)
	if totalArtifacts == 0 {
		if typed, ok := catalogSnapshot["total"].(float64); ok {
			totalArtifacts = int(typed)
		}
	}
	return RebuildResult{
		SourcePath:     sourcePath,
		CatalogPath:    catalogPath,
		TotalArtifacts: totalArtifacts,
		GeneratedAt:    strings.TrimSpace(generatedAt),
	}, nil
}

func (s *Service) RegistryStatus(ctx context.Context) (RegistryStatus, error) {
	now := time.Now().UTC()
	result := RegistryStatus{
		Healthy:   true,
		Status:    "healthy",
		Message:   "registry snapshots are healthy",
		CheckedAt: now.Format(time.RFC3339),
		Issues:    []string{},
		Source: SnapshotStatus{
			Path:   "index/source.json",
			Status: "healthy",
		},
		Catalog: SnapshotStatus{
			Path:   "index/catalog.json",
			Status: "healthy",
		},
		Stats: SnapshotStatus{
			Path:   publicStatsPath,
			Status: "idle",
		},
	}

	expectedCatalog, err := s.buildExpectedCatalogSnapshot(ctx)
	if err != nil {
		return RegistryStatus{}, err
	}
	result.ArtifactCount = intValue(expectedCatalog["total"])
	currentArtifactKinds := collectArtifactKindsFromCatalogSnapshot(expectedCatalog)

	currentCatalog, catalogExists, err := s.readSnapshot(ctx, result.Catalog.Path)
	if err != nil {
		return RegistryStatus{}, err
	}
	result.Catalog.Exists = catalogExists
	if !catalogExists {
		result.Catalog.Stale = true
		result.Catalog.Status = "missing"
		result.Catalog.Issues = append(result.Catalog.Issues, "catalog snapshot is missing")
		result.Issues = append(result.Issues, "catalog snapshot is missing")
	} else {
		result.Catalog.GeneratedAt = stringValue(currentCatalog["generated_at"])
		result.Catalog.ItemCount = intValue(currentCatalog["total"])
		if result.Catalog.ItemCount == 0 {
			result.Catalog.ItemCount = len(normalizeCatalogSnapshotItems(currentCatalog["items"]))
		}
		catalogIssues := compareCatalogSnapshot(currentCatalog, expectedCatalog)
		if len(catalogIssues) > 0 {
			result.Catalog.Stale = true
			result.Catalog.Status = "stale"
			result.Catalog.Issues = append(result.Catalog.Issues, catalogIssues...)
			result.Issues = append(result.Issues, prefixIssues("catalog: ", catalogIssues)...)
		}
	}

	expectedSource, err := s.buildExpectedSourceSnapshot(ctx, currentArtifactKinds)
	if err != nil {
		return RegistryStatus{}, err
	}
	currentSource, sourceExists, err := s.readSnapshot(ctx, result.Source.Path)
	if err != nil {
		return RegistryStatus{}, err
	}
	result.Source.Exists = sourceExists
	if !sourceExists {
		result.Source.Stale = true
		result.Source.Status = "missing"
		result.Source.Issues = append(result.Source.Issues, "source snapshot is missing")
		result.Issues = append(result.Issues, "source snapshot is missing")
	} else {
		result.Source.GeneratedAt = stringValue(currentSource["generated_at"])
		sourceIssues := compareSourceSnapshot(currentSource, expectedSource)
		if len(sourceIssues) > 0 {
			result.Source.Stale = true
			result.Source.Status = "stale"
			result.Source.Issues = append(result.Source.Issues, sourceIssues...)
			result.Issues = append(result.Issues, prefixIssues("source: ", sourceIssues)...)
		}
	}

	currentStats, statsExists, err := s.readSnapshot(ctx, result.Stats.Path)
	if err != nil {
		return RegistryStatus{}, err
	}
	result.Stats.Exists = statsExists
	if statsExists {
		result.Stats.Status = "healthy"
		result.Stats.UpdatedAt = stringValue(currentStats["updated_at"])
		result.Stats.ItemCount = len(mapValue(currentStats["artifacts"]))
		if result.Stats.UpdatedAt == "" {
			result.Stats.Status = "degraded"
			result.Stats.Issues = append(result.Stats.Issues, "stats file exists but updated_at is missing")
		}
	}

	if len(result.Issues) > 0 {
		result.Healthy = false
		result.Status = "stale"
		result.Message = "registry snapshots need repair"
	}

	return result, nil
}

func (s *Service) writeJSON(ctx context.Context, itemPath string, payload map[string]any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	return s.storage.Write(ctx, itemPath, data)
}

func (s *Service) registryBaseURL() string {
	if s.baseURL != "" {
		return s.baseURL
	}
	return strings.TrimRight(s.storage.PublicURL(""), "/")
}

func (s *Service) buildExpectedCatalogSnapshot(ctx context.Context) (map[string]any, error) {
	runtimeStore := catalog.NewRuntimeStore(s.storage, s.signing, catalog.RuntimeStoreConfig{
		SourceID:   s.sourceID,
		SourceName: s.sourceName,
		KeyID:      s.keyID,
		Settings:   s.settings,
		Now:        time.Now,
	})
	return runtimeStore.BuildCatalogSnapshot(ctx, s.registryBaseURL())
}

func (s *Service) buildExpectedSourceSnapshot(ctx context.Context, artifactKinds []string) (map[string]any, error) {
	source := map[string]any{
		"api_version":  "v1",
		"source_id":    s.sourceID,
		"name":         s.sourceName,
		"base_url":     s.registryBaseURL(),
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"capabilities": map[string]any{
			"artifact_kinds":     artifactKinds,
			"governance_modes":   []string{"host_managed"},
			"supports_delta":     false,
			"supports_signature": false,
		},
		"compatibility": map[string]any{
			"min_host_version":        "1.0.0",
			"min_host_bridge_version": "1.0.0",
		},
	}

	if s.signing != nil && strings.TrimSpace(s.keyID) != "" {
		publicKey, err := s.signing.ExportPublicKey(s.keyID)
		if err == nil {
			source["signing"] = map[string]any{
				"algorithm":  "ed25519",
				"key_id":     s.keyID,
				"public_key": publicKey,
			}
			capabilities := mapValue(source["capabilities"])
			capabilities["supports_signature"] = true
			source["capabilities"] = capabilities
		}
	}

	return source, nil
}

func (s *Service) readSnapshot(ctx context.Context, itemPath string) (map[string]any, bool, error) {
	payload, err := s.storage.Read(ctx, itemPath)
	if err != nil {
		if isNotExistError(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var out map[string]any
	if err := json.Unmarshal(payload, &out); err != nil {
		return nil, true, err
	}
	return out, true, nil
}

func compareCatalogSnapshot(current map[string]any, expected map[string]any) []string {
	currentNormalized := cloneMapDeep(current)
	expectedNormalized := cloneMapDeep(expected)

	currentItems := normalizeCatalogSnapshotItems(currentNormalized["items"])
	expectedItems := normalizeCatalogSnapshotItems(expectedNormalized["items"])
	currentTotal := intValue(currentNormalized["total"])
	expectedTotal := intValue(expectedNormalized["total"])

	issues := make([]string, 0, 4)
	if currentTotal != len(currentItems) {
		issues = append(issues, fmt.Sprintf("catalog total field mismatch: total=%d items=%d", currentTotal, len(currentItems)))
	}
	if currentTotal != expectedTotal {
		issues = append(issues, fmt.Sprintf("catalog total drifted: current=%d expected=%d", currentTotal, expectedTotal))
	}

	currentByID := catalogItemsByID(currentItems)
	expectedByID := catalogItemsByID(expectedItems)
	missingIDs := missingKeys(expectedByID, currentByID)
	unexpectedIDs := missingKeys(currentByID, expectedByID)

	if len(missingIDs) > 0 {
		issues = append(issues, "catalog missing artifacts: "+strings.Join(missingIDs, ", "))
	}
	if len(unexpectedIDs) > 0 {
		issues = append(issues, "catalog has unexpected artifacts: "+strings.Join(unexpectedIDs, ", "))
	}

	for _, artifactID := range intersectKeys(currentByID, expectedByID) {
		diffPaths := diffValuePaths(currentByID[artifactID], expectedByID[artifactID], "")
		if len(diffPaths) == 0 {
			continue
		}
		issues = append(issues, fmt.Sprintf("catalog artifact %s drifted: %s", artifactID, summarizePaths(diffPaths, 5)))
	}

	return issues
}

func compareSourceSnapshot(current map[string]any, expected map[string]any) []string {
	currentNormalized := cloneMapDeep(current)
	expectedNormalized := cloneMapDeep(expected)
	delete(currentNormalized, "generated_at")
	delete(expectedNormalized, "generated_at")

	diffPaths := diffValuePaths(currentNormalized, expectedNormalized, "")
	if len(diffPaths) == 0 {
		return []string{}
	}
	return []string{"source fields drifted: " + summarizePaths(diffPaths, 6)}
}

func cloneMapDeep(value map[string]any) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	data, err := json.Marshal(value)
	if err != nil {
		return cloneMap(value)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return cloneMap(value)
	}
	return out
}

func catalogItemsByID(items []map[string]any) map[string]map[string]any {
	out := make(map[string]map[string]any, len(items))
	for _, item := range items {
		artifactID := catalogItemID(item)
		if artifactID == "" {
			continue
		}
		out[artifactID] = cloneMapDeep(item)
	}
	return out
}

func catalogItemID(item map[string]any) string {
	kind := stringValue(item["kind"])
	name := stringValue(item["name"])
	if kind == "" || name == "" {
		return ""
	}
	return kind + ":" + name
}

func diffValuePaths(current any, expected any, prefix string) []string {
	currentMap, currentIsMap := current.(map[string]any)
	expectedMap, expectedIsMap := expected.(map[string]any)
	if currentIsMap || expectedIsMap {
		paths := make([]string, 0)
		for _, key := range unionKeys(currentMap, expectedMap) {
			nextPrefix := key
			if prefix != "" {
				nextPrefix = prefix + "." + key
			}
			currentValue, currentExists := currentMap[key]
			expectedValue, expectedExists := expectedMap[key]
			switch {
			case !currentExists:
				paths = append(paths, nextPrefix+" (missing)")
			case !expectedExists:
				paths = append(paths, nextPrefix+" (unexpected)")
			default:
				paths = append(paths, diffValuePaths(currentValue, expectedValue, nextPrefix)...)
			}
		}
		return paths
	}

	if jsonValueEqual(current, expected) {
		return []string{}
	}

	if _, currentIsSlice := current.([]any); currentIsSlice {
		return []string{prefix}
	}
	if _, expectedIsSlice := expected.([]any); expectedIsSlice {
		return []string{prefix}
	}

	return []string{prefix}
}

func unionKeys(left map[string]any, right map[string]any) []string {
	keys := make([]string, 0, len(left)+len(right))
	seen := make(map[string]struct{}, len(left)+len(right))
	for key := range left {
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range right {
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func missingKeys(left map[string]map[string]any, right map[string]map[string]any) []string {
	out := make([]string, 0)
	for key := range left {
		if _, exists := right[key]; exists {
			continue
		}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func intersectKeys(left map[string]map[string]any, right map[string]map[string]any) []string {
	out := make([]string, 0)
	for key := range left {
		if _, exists := right[key]; !exists {
			continue
		}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func summarizePaths(paths []string, limit int) string {
	if len(paths) == 0 {
		return ""
	}
	sort.Strings(paths)
	if limit <= 0 || len(paths) <= limit {
		return strings.Join(paths, ", ")
	}
	return strings.Join(paths[:limit], ", ") + fmt.Sprintf(" (+%d more)", len(paths)-limit)
}

func prefixIssues(prefix string, issues []string) []string {
	if len(issues) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		if strings.TrimSpace(issue) == "" {
			continue
		}
		out = append(out, prefix+issue)
	}
	return out
}

func jsonValueEqual(left any, right any) bool {
	leftData, leftErr := json.Marshal(left)
	rightData, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return bytes.Equal(leftData, rightData)
}

func collectArtifactKindsFromCatalogSnapshot(snapshot map[string]any) []string {
	items := normalizeCatalogSnapshotItems(snapshot["items"])
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		kind := stringValue(item["kind"])
		if kind == "" {
			continue
		}
		if _, exists := seen[kind]; exists {
			continue
		}
		seen[kind] = struct{}{}
		out = append(out, kind)
	}
	return out
}

func normalizeCatalogSnapshotItems(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return append([]map[string]any(nil), typed...)
	case []any:
		out := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			mapped, ok := item.(map[string]any)
			if !ok || mapped == nil {
				continue
			}
			out = append(out, mapped)
		}
		return out
	default:
		return []map[string]any{}
	}
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func isNotExistError(err error) bool {
	return os.IsNotExist(err)
}
