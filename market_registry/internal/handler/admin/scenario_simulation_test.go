package admin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"auralogic/market_registry/internal/analytics"
	"auralogic/market_registry/internal/auth"
	"auralogic/market_registry/internal/publish"
	"auralogic/market_registry/internal/signing"
	"auralogic/market_registry/internal/storage"
)

type scenarioArtifactOptions struct {
	Channel     string
	Title       string
	Summary     string
	Description string
	Labels      []string
}

func TestAdminScenarioSimulations(t *testing.T) {
	t.Run("empty_registry_then_reindex", func(t *testing.T) {
		handler, token := newScenarioHandler(t)

		listPayload := performAdminJSONRequest(t, handler.ListArtifacts, token, http.MethodGet, "/admin/artifacts", nil)
		listData := mapData(t, listPayload["data"])
		if len(listData) != 0 {
			t.Fatalf("expected empty artifact list, got %#v", listData)
		}

		statsPayload := performAdminJSONRequest(t, handler.GetStats, token, http.MethodGet, "/admin/stats/overview", nil)
		statsData := mapData(t, statsPayload["data"])
		if got := intValue(statsData["totalArtifacts"]); got != 0 {
			t.Fatalf("expected totalArtifacts=0, got %d", got)
		}

		statusPayload := performAdminJSONRequest(t, handler.GetRegistryStatus, token, http.MethodGet, "/admin/registry/status", nil)
		statusData := mapData(t, statusPayload["data"])
		if healthy, _ := statusData["healthy"].(bool); healthy {
			t.Fatalf("expected empty registry to be unhealthy before reindex, got %#v", statusData)
		}

		reindexPayload := performAdminJSONRequest(t, handler.ReindexRegistry, token, http.MethodPost, "/admin/registry/reindex", nil)
		reindexData := mapData(t, reindexPayload["data"])
		reindexStatus := mapData(t, reindexData["status"])
		if healthy, _ := reindexStatus["healthy"].(bool); !healthy {
			t.Fatalf("expected registry to be healthy after reindex, got %#v", reindexStatus)
		}
		if got := intValue(reindexStatus["artifactCount"]); got != 0 {
			t.Fatalf("expected artifactCount=0 after reindex, got %d", got)
		}

		t.Logf("empty scenario: totalArtifacts=%d healthy_after_reindex=%v", intValue(statsData["totalArtifacts"]), reindexStatus["healthy"])
	})

	t.Run("mixed_artifacts_and_versions", func(t *testing.T) {
		handler, token := newScenarioHandler(t)

		publishScenarioArtifact(t, handler, "plugin_package", "scenario-plugin", "1.0.0", scenarioArtifactOptions{
			Title:   "Scenario Plugin 1.0.0",
			Summary: "Baseline plugin release",
			Labels:  []string{"scenario", "plugin"},
		})
		publishScenarioArtifact(t, handler, "plugin_package", "scenario-plugin", "1.1.0", scenarioArtifactOptions{
			Title:   "Scenario Plugin 1.1.0",
			Summary: "Latest plugin release",
			Labels:  []string{"scenario", "plugin"},
		})
		publishScenarioArtifact(t, handler, "plugin_package", "scenario-plugin-aux", "0.9.0", scenarioArtifactOptions{
			Title:   "Scenario Aux Plugin",
			Summary: "Auxiliary plugin",
			Labels:  []string{"scenario", "plugin"},
		})
		publishScenarioArtifact(t, handler, "payment_package", "scenario-pay", "2.0.0", scenarioArtifactOptions{
			Title:   "Scenario Pay",
			Summary: "Scenario payment method",
			Labels:  []string{"scenario", "payment"},
		})
		publishScenarioArtifact(t, handler, "email_template", "scenario-order-paid", "1.0.0", scenarioArtifactOptions{
			Title:   "Scenario Order Paid",
			Summary: "Scenario email template",
			Labels:  []string{"scenario", "email"},
		})
		publishScenarioArtifact(t, handler, "landing_page_template", "scenario-home", "1.0.0", scenarioArtifactOptions{
			Title:   "Scenario Home",
			Summary: "Scenario landing template",
			Labels:  []string{"scenario", "landing"},
		})

		recordScenarioDownloads(t, handler, "plugin_package", "scenario-plugin", 12)
		recordScenarioDownloads(t, handler, "payment_package", "scenario-pay", 5)
		recordScenarioRequests(t, handler, "landing_page_template", "scenario-home", 3)
		rebuildScenarioRegistry(t, handler)

		listPayload := performAdminJSONRequest(t, handler.ListArtifacts, token, http.MethodGet, "/admin/artifacts", nil)
		listData := mapData(t, listPayload["data"])
		if got := countCatalogArtifacts(listData); got != 5 {
			t.Fatalf("expected 5 artifacts, got %d", got)
		}
		if got := len(mapData(t, listData["plugin_package"])); got != 2 {
			t.Fatalf("expected 2 plugin artifacts, got %d", got)
		}

		statsPayload := performAdminJSONRequest(t, handler.GetStats, token, http.MethodGet, "/admin/stats/overview", nil)
		statsData := mapData(t, statsPayload["data"])
		if got := intValue(statsData["totalArtifacts"]); got != 5 {
			t.Fatalf("expected stats totalArtifacts=5, got %d", got)
		}
		if got := intValue(statsData["totalDownloads"]); got != 17 {
			t.Fatalf("expected totalDownloads=17, got %d", got)
		}
		popular := sliceData(t, statsData["popular"])
		topPopular := mapData(t, popular[0])
		if got := stringValue(topPopular["name"]); got != "scenario-plugin" {
			t.Fatalf("expected top popular artifact scenario-plugin, got %#v", topPopular)
		}

		versionsPayload := performAdminJSONRequest(t, handler.GetArtifactVersions, token, http.MethodGet, "/admin/artifacts/plugin_package/scenario-plugin", nil)
		versionsData := mapData(t, versionsPayload["data"])
		if got := stringValue(versionsData["latest_version"]); got != "1.1.0" {
			t.Fatalf("expected latest_version=1.1.0, got %#v", versionsData["latest_version"])
		}
		releases := sliceData(t, versionsData["releases"])
		if len(releases) != 2 {
			t.Fatalf("expected 2 releases, got %#v", releases)
		}

		releasePayload := performAdminJSONRequest(t, handler.GetArtifactRelease, token, http.MethodGet, "/admin/artifacts/plugin_package/scenario-plugin/1.1.0", nil)
		releaseData := mapData(t, releasePayload["data"])
		if got := stringValue(releaseData["title"]); got != "Scenario Plugin 1.1.0" {
			t.Fatalf("expected release title override, got %#v", releaseData["title"])
		}

		statusPayload := performAdminJSONRequest(t, handler.GetRegistryStatus, token, http.MethodGet, "/admin/registry/status", nil)
		statusData := mapData(t, statusPayload["data"])
		if healthy, _ := statusData["healthy"].(bool); !healthy {
			t.Fatalf("expected healthy registry after mixed scenario, got %#v", statusData)
		}

		t.Logf("mixed scenario: artifacts=%d downloads=%d top_popular=%s latest_version=%s", countCatalogArtifacts(listData), intValue(statsData["totalDownloads"]), stringValue(topPopular["name"]), stringValue(versionsData["latest_version"]))
	})

	t.Run("large_catalog", func(t *testing.T) {
		handler, token := newScenarioHandler(t)
		const (
			pluginArtifacts  = 640
			paymentArtifacts = 220
			emailArtifacts   = 80
			landingArtifacts = 60
			hotVersions      = 32
		)
		expectedTotal := pluginArtifacts + paymentArtifacts + emailArtifacts + landingArtifacts + 1

		seedStartedAt := time.Now()
		for i := 0; i < pluginArtifacts; i++ {
			publishScenarioArtifact(t, handler, "plugin_package", fmt.Sprintf("bulk-plugin-%04d", i), "1.0.0", scenarioArtifactOptions{
				Title:   fmt.Sprintf("Bulk Plugin %04d", i),
				Summary: "Large catalog plugin fixture",
				Labels:  []string{"bulk", "plugin"},
			})
		}
		for i := 0; i < paymentArtifacts; i++ {
			publishScenarioArtifact(t, handler, "payment_package", fmt.Sprintf("bulk-payment-%04d", i), "1.0.0", scenarioArtifactOptions{
				Title:   fmt.Sprintf("Bulk Payment %04d", i),
				Summary: "Large catalog payment fixture",
				Labels:  []string{"bulk", "payment"},
			})
		}
		for i := 0; i < emailArtifacts; i++ {
			publishScenarioArtifact(t, handler, "email_template", fmt.Sprintf("bulk-email-%04d", i), "1.0.0", scenarioArtifactOptions{
				Title:   fmt.Sprintf("Bulk Email %04d", i),
				Summary: "Large catalog email fixture",
				Labels:  []string{"bulk", "email"},
			})
		}
		for i := 0; i < landingArtifacts; i++ {
			publishScenarioArtifact(t, handler, "landing_page_template", fmt.Sprintf("bulk-landing-%04d", i), "1.0.0", scenarioArtifactOptions{
				Title:   fmt.Sprintf("Bulk Landing %04d", i),
				Summary: "Large catalog landing fixture",
				Labels:  []string{"bulk", "landing"},
			})
		}
		for version := 1; version <= hotVersions; version++ {
			publishScenarioArtifact(t, handler, "plugin_package", "bulk-hot-plugin", fmt.Sprintf("1.0.%d", version), scenarioArtifactOptions{
				Title:   fmt.Sprintf("Bulk Hot Plugin 1.0.%d", version),
				Summary: "Hot plugin with deep version history",
				Labels:  []string{"bulk", "plugin", "hot"},
			})
		}
		seedDuration := time.Since(seedStartedAt)

		recordScenarioDownloads(t, handler, "plugin_package", "bulk-hot-plugin", 240)
		recordScenarioDownloads(t, handler, "plugin_package", "bulk-plugin-0000", 160)
		recordScenarioDownloads(t, handler, "payment_package", "bulk-payment-0000", 120)

		reindexStartedAt := time.Now()
		rebuildScenarioRegistry(t, handler)
		reindexDuration := time.Since(reindexStartedAt)

		listStartedAt := time.Now()
		listPayload := performAdminJSONRequest(t, handler.ListArtifacts, token, http.MethodGet, "/admin/artifacts", nil)
		listDuration := time.Since(listStartedAt)
		listData := mapData(t, listPayload["data"])
		if got := countCatalogArtifacts(listData); got != expectedTotal {
			t.Fatalf("expected %d artifacts, got %d", expectedTotal, got)
		}

		statsStartedAt := time.Now()
		statsPayload := performAdminJSONRequest(t, handler.GetStats, token, http.MethodGet, "/admin/stats/overview", nil)
		statsDuration := time.Since(statsStartedAt)
		statsData := mapData(t, statsPayload["data"])
		if got := intValue(statsData["totalArtifacts"]); got != expectedTotal {
			t.Fatalf("expected stats totalArtifacts=%d, got %d", expectedTotal, got)
		}

		versionsStartedAt := time.Now()
		versionsPayload := performAdminJSONRequest(t, handler.GetArtifactVersions, token, http.MethodGet, "/admin/artifacts/plugin_package/bulk-hot-plugin", nil)
		versionsDuration := time.Since(versionsStartedAt)
		versionsData := mapData(t, versionsPayload["data"])
		releases := sliceData(t, versionsData["releases"])
		if len(releases) != hotVersions {
			t.Fatalf("expected %d versions for bulk-hot-plugin, got %d", hotVersions, len(releases))
		}
		if got := stringValue(versionsData["latest_version"]); got != fmt.Sprintf("1.0.%d", hotVersions) {
			t.Fatalf("expected latest_version=1.0.%d, got %#v", hotVersions, versionsData["latest_version"])
		}

		statusPayload := performAdminJSONRequest(t, handler.GetRegistryStatus, token, http.MethodGet, "/admin/registry/status", nil)
		statusData := mapData(t, statusPayload["data"])
		if healthy, _ := statusData["healthy"].(bool); !healthy {
			t.Fatalf("expected healthy registry after large catalog simulation, got %#v", statusData)
		}

		t.Logf(
			"large catalog scenario: artifacts=%d hot_versions=%d seed=%s reindex=%s list=%s stats=%s versions=%s list_bytes=%d",
			expectedTotal,
			hotVersions,
			seedDuration,
			reindexDuration,
			listDuration,
			statsDuration,
			versionsDuration,
			marshalLen(t, listPayload),
		)
	})
}

func BenchmarkAdminLargeCatalogSimulation(b *testing.B) {
	handler, token := newScenarioHandler(b)
	const (
		pluginArtifacts  = 1400
		paymentArtifacts = 420
		emailArtifacts   = 100
		landingArtifacts = 80
		hotVersions      = 48
	)
	totalArtifacts := pluginArtifacts + paymentArtifacts + emailArtifacts + landingArtifacts + 1

	seedStartedAt := time.Now()
	for i := 0; i < pluginArtifacts; i++ {
		publishScenarioArtifact(b, handler, "plugin_package", fmt.Sprintf("bench-plugin-%04d", i), "1.0.0", scenarioArtifactOptions{
			Title:   fmt.Sprintf("Bench Plugin %04d", i),
			Summary: "Benchmark plugin fixture",
			Labels:  []string{"bench", "plugin"},
		})
	}
	for i := 0; i < paymentArtifacts; i++ {
		publishScenarioArtifact(b, handler, "payment_package", fmt.Sprintf("bench-payment-%04d", i), "1.0.0", scenarioArtifactOptions{
			Title:   fmt.Sprintf("Bench Payment %04d", i),
			Summary: "Benchmark payment fixture",
			Labels:  []string{"bench", "payment"},
		})
	}
	for i := 0; i < emailArtifacts; i++ {
		publishScenarioArtifact(b, handler, "email_template", fmt.Sprintf("bench-email-%04d", i), "1.0.0", scenarioArtifactOptions{
			Title:   fmt.Sprintf("Bench Email %04d", i),
			Summary: "Benchmark email fixture",
			Labels:  []string{"bench", "email"},
		})
	}
	for i := 0; i < landingArtifacts; i++ {
		publishScenarioArtifact(b, handler, "landing_page_template", fmt.Sprintf("bench-landing-%04d", i), "1.0.0", scenarioArtifactOptions{
			Title:   fmt.Sprintf("Bench Landing %04d", i),
			Summary: "Benchmark landing fixture",
			Labels:  []string{"bench", "landing"},
		})
	}
	for version := 1; version <= hotVersions; version++ {
		publishScenarioArtifact(b, handler, "plugin_package", "bench-hot-plugin", fmt.Sprintf("2.0.%d", version), scenarioArtifactOptions{
			Title:   fmt.Sprintf("Bench Hot Plugin 2.0.%d", version),
			Summary: "Benchmark hot plugin fixture",
			Labels:  []string{"bench", "plugin", "hot"},
		})
	}
	recordScenarioDownloads(b, handler, "plugin_package", "bench-hot-plugin", 320)
	recordScenarioDownloads(b, handler, "plugin_package", "bench-plugin-0000", 220)
	recordScenarioDownloads(b, handler, "payment_package", "bench-payment-0000", 180)
	rebuildScenarioRegistry(b, handler)
	seedDuration := time.Since(seedStartedAt)

	listPayload := performAdminJSONRequest(b, handler.ListArtifacts, token, http.MethodGet, "/admin/artifacts", nil)
	listBytes := marshalLen(b, listPayload)
	statsPayload := performAdminJSONRequest(b, handler.GetStats, token, http.MethodGet, "/admin/stats/overview", nil)
	statsData := mapData(b, statsPayload["data"])
	if got := intValue(statsData["totalArtifacts"]); got != totalArtifacts {
		b.Fatalf("expected totalArtifacts=%d, got %d", totalArtifacts, got)
	}

	b.ReportMetric(float64(totalArtifacts), "artifacts")
	b.ReportMetric(float64(hotVersions), "hot_versions")
	b.ReportMetric(float64(seedDuration.Milliseconds()), "seed_ms")
	b.ReportMetric(float64(listBytes), "list_bytes")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		listPayload = performAdminJSONRequest(b, handler.ListArtifacts, token, http.MethodGet, "/admin/artifacts", nil)
		statsPayload = performAdminJSONRequest(b, handler.GetStats, token, http.MethodGet, "/admin/stats/overview", nil)
		_ = performAdminJSONRequest(b, handler.GetArtifactVersions, token, http.MethodGet, "/admin/artifacts/plugin_package/bench-hot-plugin", nil)
		if got := countCatalogArtifacts(mapData(b, listPayload["data"])); got != totalArtifacts {
			b.Fatalf("expected total artifacts=%d, got %d", totalArtifacts, got)
		}
		if got := intValue(mapData(b, statsPayload["data"])["totalArtifacts"]); got != totalArtifacts {
			b.Fatalf("expected stats totalArtifacts=%d, got %d", totalArtifacts, got)
		}
	}
}

func newScenarioHandler(tb testing.TB) (*Handler, string) {
	tb.Helper()

	root := tb.TempDir()
	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		tb.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		tb.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	authSvc := auth.NewServiceWithConfig(auth.Config{
		AdminUsername: "admin",
		AdminPassword: "password",
	})
	pubSvc := publish.NewService(store, signSvc, "official-test")
	token, err := authSvc.Login("admin", "password")
	if err != nil {
		tb.Fatalf("Login returned error: %v", err)
	}
	return NewHandler(authSvc, pubSvc, store, signSvc), token
}

func publishScenarioArtifact(tb testing.TB, handler *Handler, kind string, name string, version string, opts scenarioArtifactOptions) {
	tb.Helper()

	channel := firstNonEmptyLocal(opts.Channel, "stable")
	title := firstNonEmptyLocal(opts.Title, fmt.Sprintf("%s %s", strings.ReplaceAll(name, "-", " "), version))
	summary := firstNonEmptyLocal(opts.Summary, fmt.Sprintf("%s %s summary", kind, name))
	description := firstNonEmptyLocal(opts.Description, summary)

	req := publish.Request{
		Kind:        kind,
		Name:        name,
		Version:     version,
		Channel:     channel,
		ArtifactZip: buildScenarioArtifactZip(tb, kind, name, version, title, summary, description),
		Metadata: publish.Metadata{
			Title:       title,
			Summary:     summary,
			Description: description,
			Publisher: publish.Publisher{
				ID:   "scenario-suite",
				Name: "Scenario Suite",
			},
			Labels: append([]string(nil), opts.Labels...),
		},
	}

	if err := handler.publish.Publish(context.Background(), req); err != nil {
		tb.Fatalf("Publish(%s:%s:%s) returned error: %v", kind, name, version, err)
	}
}

func buildScenarioArtifactZip(tb testing.TB, kind string, name string, version string, title string, summary string, description string) []byte {
	tb.Helper()

	switch kind {
	case "plugin_package":
		return buildScenarioZip(tb, map[string]string{
			"manifest.json": fmt.Sprintf(`{
  "kind": "plugin_package",
  "name": %q,
  "display_name": %q,
  "version": %q,
  "description": %q,
  "runtime": "js_worker",
  "entry": "index.js",
  "protocol_version": "1.0.0",
  "manifest_version": "1.0.0",
  "min_host_protocol_version": "1.0.0",
  "max_host_protocol_version": "1.0.0"
}`, name, title, version, description),
			"index.js": "module.exports = { execute: function execute() { return { ok: true }; } };",
		})
	case "payment_package":
		return buildScenarioZip(tb, map[string]string{
			"manifest.json": fmt.Sprintf(`{
  "kind": "payment_package",
  "name": %q,
  "display_name": %q,
  "version": %q,
  "description": %q,
  "runtime": "payment_js",
  "entry": "index.js",
  "icon": "CreditCard"
}`, name, title, version, description),
			"index.js": "module.exports = { onGeneratePaymentCard: function () { return { title: 'Scenario Payment', html: '<div>scenario</div>' }; } };",
		})
	case "email_template":
		return buildScenarioZip(tb, map[string]string{
			"manifest.json": fmt.Sprintf(`{
  "kind": "email_template",
  "name": %q,
  "title": %q,
  "version": %q,
  "event": "order_paid",
  "engine": "go_template",
  "content_file": "template.html",
  "description": %q
}`, name, title, version, description),
			"template.html": fmt.Sprintf("<html><body><h1>%s</h1><p>%s</p></body></html>", title, summary),
		})
	case "landing_page_template":
		return buildScenarioZip(tb, map[string]string{
			"manifest.json": fmt.Sprintf(`{
  "kind": "landing_page_template",
  "name": %q,
  "title": %q,
  "version": %q,
  "engine": "go_template",
  "content_file": "landing.html",
  "description": %q
}`, name, title, version, description),
			"landing.html": fmt.Sprintf("<html><body><section><h1>%s</h1><p>%s</p></section></body></html>", title, summary),
		})
	default:
		tb.Fatalf("unsupported scenario artifact kind: %s", kind)
		return nil
	}
}

func buildScenarioZip(tb testing.TB, files map[string]string) []byte {
	tb.Helper()

	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	keys := make([]string, 0, len(files))
	for name := range files {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		entry, err := writer.Create(strings.TrimSpace(name))
		if err != nil {
			tb.Fatalf("Create zip entry %s returned error: %v", name, err)
		}
		if _, err := entry.Write([]byte(files[name])); err != nil {
			tb.Fatalf("Write zip entry %s returned error: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		tb.Fatalf("Close zip writer returned error: %v", err)
	}
	return buffer.Bytes()
}

func recordScenarioDownloads(tb testing.TB, handler *Handler, kind string, name string, count int) {
	tb.Helper()

	statsSvc := analytics.NewService(handler.storage, analytics.Config{})
	for i := 0; i < count; i++ {
		if err := statsSvc.RecordEvent(context.Background(), analytics.Event{
			Type:         analytics.EventDownload,
			ArtifactKind: kind,
			ArtifactName: name,
		}); err != nil {
			tb.Fatalf("RecordEvent download returned error: %v", err)
		}
	}
}

func recordScenarioRequests(tb testing.TB, handler *Handler, kind string, name string, count int) {
	tb.Helper()

	statsSvc := analytics.NewService(handler.storage, analytics.Config{})
	for i := 0; i < count; i++ {
		if err := statsSvc.RecordEvent(context.Background(), analytics.Event{
			Type:         analytics.EventArtifactView,
			ArtifactKind: kind,
			ArtifactName: name,
		}); err != nil {
			tb.Fatalf("RecordEvent request returned error: %v", err)
		}
	}
}

func rebuildScenarioRegistry(tb testing.TB, handler *Handler) {
	tb.Helper()

	if _, err := handler.publish.RebuildRegistry(context.Background()); err != nil {
		tb.Fatalf("RebuildRegistry returned error: %v", err)
	}
}

func performAdminJSONRequest(
	tb testing.TB,
	handlerFunc func(http.ResponseWriter, *http.Request),
	token string,
	method string,
	target string,
	body []byte,
) map[string]any {
	tb.Helper()

	var reader *bytes.Reader
	if len(body) == 0 {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Authorization", "Bearer "+token)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handlerFunc(recorder, req)
	if recorder.Code != http.StatusOK {
		tb.Fatalf("%s %s expected 200, got %d with body %s", method, target, recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		tb.Fatalf("Unmarshal %s %s returned error: %v", method, target, err)
	}
	return payload
}

func mapData(tb testing.TB, value any) map[string]any {
	tb.Helper()

	mapped, ok := value.(map[string]any)
	if !ok || mapped == nil {
		tb.Fatalf("expected map payload, got %#v", value)
	}
	return mapped
}

func sliceData(tb testing.TB, value any) []any {
	tb.Helper()

	items, ok := value.([]any)
	if !ok {
		tb.Fatalf("expected slice payload, got %#v", value)
	}
	return items
}

func countCatalogArtifacts(payload map[string]any) int {
	total := 0
	for _, value := range payload {
		items, ok := value.(map[string]any)
		if !ok {
			continue
		}
		total += len(items)
	}
	return total
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

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func firstNonEmptyLocal(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func marshalLen(tb testing.TB, value any) int {
	tb.Helper()

	payload, err := json.Marshal(value)
	if err != nil {
		tb.Fatalf("Marshal payload returned error: %v", err)
	}
	return len(payload)
}
