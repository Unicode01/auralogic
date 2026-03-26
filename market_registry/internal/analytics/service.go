package analytics

import (
	pubanalytics "auralogic/market_registry/pkg/analytics"

	"auralogic/market_registry/internal/storage"
)

const publicStatsPath = "index/stats/public.json"

type EventType = pubanalytics.EventType

const (
	EventSourceView   = pubanalytics.EventSourceView
	EventCatalogView  = pubanalytics.EventCatalogView
	EventArtifactView = pubanalytics.EventArtifactView
	EventReleaseView  = pubanalytics.EventReleaseView
	EventDownload     = pubanalytics.EventDownload
)

type Config = pubanalytics.Config
type Event = pubanalytics.Event
type Overview = pubanalytics.Overview
type Service = pubanalytics.Service

func NewService(store storage.Storage, cfg Config) *Service {
	return pubanalytics.NewService(store, cfg)
}
