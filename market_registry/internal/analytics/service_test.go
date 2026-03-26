package analytics

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"auralogic/market_registry/internal/storage"
)

func TestRecordEventAndBuildOverview(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	service := NewService(store, Config{
		Now: func() time.Time {
			return time.Date(2026, 3, 15, 9, 0, 0, 0, time.UTC)
		},
	})

	if err := store.Write(ctx, "index/catalog.json", []byte(`{
  "items": [
    {"kind":"plugin_package","name":"demo-plugin"},
    {"kind":"payment_package","name":"demo-payment"}
  ]
}`)); err != nil {
		t.Fatalf("Write catalog snapshot returned error: %v", err)
	}

	if err := service.RecordEvent(ctx, Event{Type: EventCatalogView}); err != nil {
		t.Fatalf("RecordEvent catalog view returned error: %v", err)
	}
	if err := service.RecordEvent(ctx, Event{
		Type:         EventDownload,
		ArtifactKind: "plugin_package",
		ArtifactName: "demo-plugin",
	}); err != nil {
		t.Fatalf("RecordEvent download returned error: %v", err)
	}

	overview, err := service.BuildOverview(ctx)
	if err != nil {
		t.Fatalf("BuildOverview returned error: %v", err)
	}
	if overview.TotalArtifacts != 2 {
		t.Fatalf("expected TotalArtifacts 2, got %d", overview.TotalArtifacts)
	}
	if overview.TotalDownloads != 1 {
		t.Fatalf("expected TotalDownloads 1, got %d", overview.TotalDownloads)
	}
	if overview.TodayVisits != 2 {
		t.Fatalf("expected TodayVisits 2, got %d", overview.TodayVisits)
	}
	if len(overview.Popular) != 1 || overview.Popular[0]["name"] != "demo-plugin" {
		t.Fatalf("expected demo-plugin to be most popular, got %#v", overview.Popular)
	}
	if !overview.Storage.Available {
		t.Fatalf("expected storage overview to be available, got %#v", overview.Storage)
	}
	if overview.Storage.Backend != "local" {
		t.Fatalf("expected local storage backend, got %#v", overview.Storage.Backend)
	}
	if overview.Storage.FileCount < 2 {
		t.Fatalf("expected at least 2 storage files, got %#v", overview.Storage.FileCount)
	}
	if overview.Storage.TotalBytes <= 0 {
		t.Fatalf("expected storage bytes to be positive, got %#v", overview.Storage.TotalBytes)
	}

	statsPayload, err := store.Read(ctx, publicStatsPath)
	if err != nil {
		t.Fatalf("Read stats payload returned error: %v", err)
	}
	var stats map[string]any
	if err := json.Unmarshal(statsPayload, &stats); err != nil {
		t.Fatalf("Unmarshal stats payload returned error: %v", err)
	}
	if got := stats["updated_at"]; got == "" {
		t.Fatalf("expected updated_at to be set, got %#v", got)
	}
}
