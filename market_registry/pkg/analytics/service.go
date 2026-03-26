package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"auralogic/market_registry/pkg/catalog"
	"auralogic/market_registry/pkg/storage"
)

const publicStatsPath = "index/stats/public.json"

type EventType string

const (
	EventSourceView   EventType = "source_view"
	EventCatalogView  EventType = "catalog_view"
	EventArtifactView EventType = "artifact_view"
	EventReleaseView  EventType = "release_view"
	EventDownload     EventType = "download"
)

type Config struct {
	Now func() time.Time
}

type Event struct {
	Type         EventType
	ArtifactKind string
	ArtifactName string
}

type Service struct {
	storage storage.Storage
	now     func() time.Time
	mu      sync.Mutex
}

type Overview struct {
	TotalArtifacts int                 `json:"totalArtifacts"`
	TotalDownloads int                 `json:"totalDownloads"`
	TodayVisits    int                 `json:"todayVisits"`
	TodayDownloads int                 `json:"todayDownloads"`
	Dates          []string            `json:"dates"`
	Downloads      []int               `json:"downloads"`
	Popular        []map[string]any    `json:"popular"`
	UpdatedAt      string              `json:"updatedAt"`
	Totals         map[string]int      `json:"totals"`
	Daily          map[string]dayStats `json:"daily"`
	Storage        StorageOverview     `json:"storage"`
}

type StorageOverview struct {
	Available   bool   `json:"available"`
	Backend     string `json:"backend,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Location    string `json:"location,omitempty"`
	FileCount   int    `json:"fileCount,omitempty"`
	TotalBytes  int64  `json:"totalBytes,omitempty"`
	Error       string `json:"error,omitempty"`
}

type statsState struct {
	UpdatedAt string                   `json:"updated_at,omitempty"`
	Totals    totals                   `json:"totals"`
	Daily     map[string]dayStats      `json:"daily"`
	Artifacts map[string]artifactStats `json:"artifacts"`
}

type totals struct {
	Requests  int `json:"requests"`
	Downloads int `json:"downloads"`
}

type dayStats struct {
	Requests  int `json:"requests"`
	Downloads int `json:"downloads"`
}

type artifactStats struct {
	Kind             string `json:"kind"`
	Name             string `json:"name"`
	Requests         int    `json:"requests"`
	Downloads        int    `json:"downloads"`
	LastRequestedAt  string `json:"last_requested_at,omitempty"`
	LastDownloadedAt string `json:"last_downloaded_at,omitempty"`
}

func NewService(store storage.Storage, cfg Config) *Service {
	if store == nil {
		panic("analytics: storage is required")
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		storage: store,
		now:     now,
	}
}

func (s *Service) RecordEvent(ctx context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.loadState(ctx)
	if err != nil {
		return err
	}

	now := s.now().UTC()
	dayKey := now.Format("2006-01-02")
	daily := state.Daily[dayKey]
	daily.Requests++
	state.Totals.Requests++

	key := statsKey(event.ArtifactKind, event.ArtifactName)
	if key != "" {
		artifact := state.Artifacts[key]
		artifact.Kind = strings.TrimSpace(event.ArtifactKind)
		artifact.Name = strings.TrimSpace(event.ArtifactName)
		artifact.Requests++
		artifact.LastRequestedAt = now.Format(time.RFC3339)
		if event.Type == EventDownload {
			artifact.Downloads++
			artifact.LastDownloadedAt = artifact.LastRequestedAt
		}
		state.Artifacts[key] = artifact
	}

	if event.Type == EventDownload {
		daily.Downloads++
		state.Totals.Downloads++
	}

	state.Daily[dayKey] = daily
	state.UpdatedAt = now.Format(time.RFC3339)
	return s.writeState(ctx, state)
}

func (s *Service) BuildOverview(ctx context.Context) (Overview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.loadState(ctx)
	if err != nil {
		return Overview{}, err
	}

	now := s.now().UTC()
	todayKey := now.Format("2006-01-02")
	artifactCount, err := s.countArtifacts(ctx)
	if err != nil {
		return Overview{}, err
	}
	storageOverview := s.buildStorageOverview(ctx)

	dates := make([]string, 0, 7)
	downloads := make([]int, 0, 7)
	for i := 6; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		dayKey := day.Format("2006-01-02")
		dates = append(dates, day.Format("01-02"))
		downloads = append(downloads, state.Daily[dayKey].Downloads)
	}

	return Overview{
		TotalArtifacts: artifactCount,
		TotalDownloads: state.Totals.Downloads,
		TodayVisits:    state.Daily[todayKey].Requests,
		TodayDownloads: state.Daily[todayKey].Downloads,
		Dates:          dates,
		Downloads:      downloads,
		Popular:        buildPopularArtifacts(state.Artifacts),
		UpdatedAt:      state.UpdatedAt,
		Totals: map[string]int{
			"requests":  state.Totals.Requests,
			"downloads": state.Totals.Downloads,
		},
		Daily:   state.Daily,
		Storage: storageOverview,
	}, nil
}

func (s *Service) buildStorageOverview(ctx context.Context) StorageOverview {
	reporter, ok := s.storage.(storage.UsageReporter)
	if !ok {
		return StorageOverview{
			Available: false,
			Error:     "storage backend does not expose usage data",
		}
	}
	summary, err := reporter.Usage(ctx)
	if err != nil {
		return StorageOverview{
			Available: false,
			Error:     err.Error(),
		}
	}
	return StorageOverview{
		Available:   true,
		Backend:     summary.Backend,
		DisplayName: summary.DisplayName,
		Location:    summary.Location,
		FileCount:   summary.FileCount,
		TotalBytes:  summary.TotalBytes,
	}
}

func (s *Service) loadState(ctx context.Context) (statsState, error) {
	payload, err := s.storage.Read(ctx, publicStatsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyState(), nil
		}
		return statsState{}, fmt.Errorf("read public stats: %w", err)
	}

	var state statsState
	if err := json.Unmarshal(payload, &state); err != nil {
		return statsState{}, fmt.Errorf("decode public stats: %w", err)
	}
	return normalizeState(state), nil
}

func (s *Service) writeState(ctx context.Context, state statsState) error {
	payload, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal public stats: %w", err)
	}
	if err := s.storage.Write(ctx, publicStatsPath, payload); err != nil {
		return fmt.Errorf("write public stats: %w", err)
	}
	return nil
}

func (s *Service) countArtifacts(ctx context.Context) (int, error) {
	payload, err := s.storage.Read(ctx, "index/catalog.json")
	if err == nil {
		var snapshot map[string]any
		if unmarshalErr := json.Unmarshal(payload, &snapshot); unmarshalErr == nil {
			if items := normalizeCatalogSnapshotItems(snapshot["items"]); len(items) > 0 {
				return len(items), nil
			}
		}
	} else if !os.IsNotExist(err) {
		return 0, fmt.Errorf("read catalog snapshot: %w", err)
	}

	paths, err := s.storage.List(ctx, "index/artifacts")
	if err != nil {
		return 0, fmt.Errorf("list artifact indexes: %w", err)
	}
	seen := map[string]struct{}{}
	for _, itemPath := range paths {
		kind, name, ok := catalog.ParseArtifactIndexPath(itemPath)
		if !ok {
			continue
		}
		seen[strings.ToLower(kind)+":"+strings.ToLower(name)] = struct{}{}
	}
	return len(seen), nil
}

func buildPopularArtifacts(items map[string]artifactStats) []map[string]any {
	entries := make([]artifactStats, 0, len(items))
	for _, item := range items {
		if item.Name == "" {
			continue
		}
		entries = append(entries, item)
	}
	sort.SliceStable(entries, func(i int, j int) bool {
		if entries[i].Downloads == entries[j].Downloads {
			if entries[i].Requests == entries[j].Requests {
				if entries[i].Kind == entries[j].Kind {
					return entries[i].Name < entries[j].Name
				}
				return entries[i].Kind < entries[j].Kind
			}
			return entries[i].Requests > entries[j].Requests
		}
		return entries[i].Downloads > entries[j].Downloads
	})
	if len(entries) > 5 {
		entries = entries[:5]
	}
	out := make([]map[string]any, 0, len(entries))
	for _, item := range entries {
		out = append(out, map[string]any{
			"kind":      item.Kind,
			"name":      item.Name,
			"downloads": item.Downloads,
			"requests":  item.Requests,
		})
	}
	return out
}

func emptyState() statsState {
	return statsState{
		Daily:     map[string]dayStats{},
		Artifacts: map[string]artifactStats{},
	}
}

func normalizeState(state statsState) statsState {
	if state.Daily == nil {
		state.Daily = map[string]dayStats{}
	}
	if state.Artifacts == nil {
		state.Artifacts = map[string]artifactStats{}
	}
	return state
}

func statsKey(kind string, name string) string {
	kind = strings.TrimSpace(kind)
	name = strings.TrimSpace(name)
	if kind == "" || name == "" {
		return ""
	}
	return strings.ToLower(kind) + ":" + strings.ToLower(name)
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
