package pluginobs

import "testing"

func TestSnapshotIncludesFrontendObservabilityMetrics(t *testing.T) {
	ResetForTest()

	RecordFrontendSlotRequest()
	RecordFrontendSlotRequest()
	RecordFrontendBootstrapRequest()
	RecordFrontendBatchRequest(5, 3)

	RecordFrontendResolverCacheHit("html_mode", 4)
	RecordFrontendResolverCacheMiss("html_mode", 2)
	RecordFrontendResolverSingleflightWait("html_mode", 1)
	RecordFrontendResolverCatalogHit("html_mode", 2)
	RecordFrontendResolverDBFallback("html_mode", 1)

	RecordFrontendResolverCacheMiss("execute_api", 3)
	RecordFrontendResolverCatalogHit("execute_api", 2)
	RecordFrontendResolverDBFallback("execute_api", 1)

	RecordFrontendResolverCacheHit("prepared_hook", 6)
	RecordFrontendResolverCacheMiss("prepared_hook", 2)
	RecordFrontendResolverSingleflightWait("prepared_hook", 1)

	snapshot := SnapshotNow()

	if snapshot.Frontend.SlotRequests != 2 {
		t.Fatalf("expected slot requests=2, got %+v", snapshot.Frontend)
	}
	if snapshot.Frontend.BootstrapRequests != 1 {
		t.Fatalf("expected bootstrap requests=1, got %+v", snapshot.Frontend)
	}
	if snapshot.Frontend.BatchRequests != 1 || snapshot.Frontend.BatchItems != 5 || snapshot.Frontend.BatchUniqueItems != 3 || snapshot.Frontend.BatchDedupedItems != 2 {
		t.Fatalf("expected batch counters to be aggregated, got %+v", snapshot.Frontend)
	}

	if snapshot.Frontend.HTMLMode.CacheHits != 4 || snapshot.Frontend.HTMLMode.CacheMisses != 2 {
		t.Fatalf("expected html_mode cache counters, got %+v", snapshot.Frontend.HTMLMode)
	}
	if snapshot.Frontend.HTMLMode.SingleflightWait != 1 || snapshot.Frontend.HTMLMode.CatalogHits != 2 || snapshot.Frontend.HTMLMode.DBFallbacks != 1 {
		t.Fatalf("expected html_mode resolver counters, got %+v", snapshot.Frontend.HTMLMode)
	}
	if snapshot.Frontend.HTMLMode.CacheHitRate != 0.6667 {
		t.Fatalf("expected html_mode cache hit rate=0.6667, got %+v", snapshot.Frontend.HTMLMode)
	}

	if snapshot.Frontend.ExecuteAPI.CacheMisses != 3 || snapshot.Frontend.ExecuteAPI.CatalogHits != 2 || snapshot.Frontend.ExecuteAPI.DBFallbacks != 1 {
		t.Fatalf("expected execute_api resolver counters, got %+v", snapshot.Frontend.ExecuteAPI)
	}
	if snapshot.Frontend.PreparedHook.CacheHits != 6 || snapshot.Frontend.PreparedHook.CacheMisses != 2 || snapshot.Frontend.PreparedHook.SingleflightWait != 1 {
		t.Fatalf("expected prepared_hook resolver counters, got %+v", snapshot.Frontend.PreparedHook)
	}
	if snapshot.Frontend.PreparedHook.CacheHitRate != 0.75 {
		t.Fatalf("expected prepared_hook cache hit rate=0.75, got %+v", snapshot.Frontend.PreparedHook)
	}
}
