package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
)

func TestPaymentMethodMarketPreviewAndImport(t *testing.T) {
	manifest := map[string]interface{}{
		"name":                      "mock-checkout",
		"display_name":              "Mock Checkout",
		"description":               "Market payment package",
		"icon":                      "CreditCard",
		"entry":                     "index.js",
		"version":                   "1.0.0",
		"poll_interval":             30,
		"manifest_version":          "1.0.0",
		"protocol_version":          "1.0.0",
		"min_host_protocol_version": "1.0.0",
		"max_host_protocol_version": "1.0.0",
		"config_schema": map[string]interface{}{
			"fields": []map[string]interface{}{
				{
					"key":      "checkout_title",
					"type":     "string",
					"default":  "Mock Checkout",
					"required": true,
				},
			},
		},
		"webhooks": []map[string]interface{}{
			{
				"key":       "payment.notify",
				"method":    "POST",
				"auth_mode": "none",
			},
		},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest failed: %v", err)
	}
	zipPayload := buildPluginMarketTestZip(t, map[string]string{
		"manifest.json": string(manifestRaw),
		"index.js": `
function onGeneratePaymentCard(order, config) {
  return { html: "<div>" + ((config && config.checkout_title) || "Mock") + "</div>" };
}
function onCheckPaymentStatus() { return { paid: false }; }
`,
	})
	sha := computeSHA256Hex(zipPayload)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/payment_package/mock-checkout/releases/1.0.0":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"kind":    "payment_package",
					"name":    "mock-checkout",
					"version": "1.0.0",
					"governance": map[string]interface{}{
						"mode": "host_managed",
					},
					"download": map[string]interface{}{
						"url":    server.URL + "/v1/artifacts/payment_package/mock-checkout/releases/1.0.0/download",
						"sha256": sha,
						"size":   len(zipPayload),
					},
					"compatibility": map[string]interface{}{
						"min_host_bridge_version": "1.0.0",
					},
				},
			})
		case "/v1/artifacts/payment_package/mock-checkout/releases/1.0.0/download":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(zipPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.PaymentMethod{}, &models.PaymentMethodVersion{}); err != nil {
		t.Fatalf("auto migrate payment methods failed: %v", err)
	}

	cfg := &config.Config{}
	cfg.Plugin.ArtifactDir = t.TempDir()
	runtime := NewPluginHostRuntime(db, cfg, nil)
	source := PluginMarketSource{
		SourceID:       "official",
		Name:           "Official",
		BaseURL:        server.URL,
		DefaultChannel: "stable",
		AllowedKinds:   []string{"payment_package"},
		Enabled:        true,
	}

	preview, err := PreviewPaymentMethodMarketPackageWithSource(runtime, source, "mock-checkout", "1.0.0", 0)
	if err != nil {
		t.Fatalf("preview payment package returned error: %v", err)
	}
	resolved, ok := preview["resolved"].(*marketPaymentPackageResolved)
	if !ok || resolved == nil {
		t.Fatalf("expected resolved preview payload, got %#v", preview["resolved"])
	}
	if resolved.Entry != "index.js" {
		t.Fatalf("expected resolved entry index.js, got %#v", resolved.Entry)
	}
	if resolved.Config == "" {
		t.Fatalf("expected resolved config, got %#v", resolved.Config)
	}

	result, err := ExecutePaymentMethodMarketPackageWithSource(runtime, source, "mock-checkout", "1.0.0", map[string]interface{}{
		"name":   "Mock Checkout",
		"config": `{"checkout_title":"Custom Checkout"}`,
	})
	if err != nil {
		t.Fatalf("execute payment package returned error: %v", err)
	}
	if created, _ := result["created"].(bool); !created {
		t.Fatalf("expected created=true, got %#v", result["created"])
	}
	method, ok := result["item"].(*models.PaymentMethod)
	if !ok || method == nil {
		t.Fatalf("expected created payment method, got %#v", result["item"])
	}
	if method.Name != "Mock Checkout" {
		t.Fatalf("expected payment method name Mock Checkout, got %#v", method.Name)
	}
	if method.PackageChecksum != sha {
		t.Fatalf("expected checksum %s, got %#v", sha, method.PackageChecksum)
	}

	historyEntry, ok := result["history_entry"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected history_entry payload, got %#v", result["history_entry"])
	}
	if got := historyEntry["payment_method_id"]; got != method.ID {
		t.Fatalf("expected history payment_method_id=%d, got %#v", method.ID, got)
	}

	var versions []models.PaymentMethodVersion
	if err := db.Order("id ASC").Find(&versions).Error; err != nil {
		t.Fatalf("query payment method versions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected one payment method version, got %d", len(versions))
	}
	if versions[0].MarketSourceID != "official" || versions[0].MarketArtifactKind != "payment_package" || versions[0].MarketArtifactName != "mock-checkout" || versions[0].MarketArtifactVersion != "1.0.0" {
		t.Fatalf("expected persisted market coordinates, got %+v", versions[0])
	}
	if !versions[0].IsActive {
		t.Fatalf("expected imported version to be active, got %+v", versions[0])
	}
}

func TestResolveMarketPaymentPackageEntryPathAutoDetectsIndexMJS(t *testing.T) {
	extractDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(extractDir, "helpers"), 0o755); err != nil {
		t.Fatalf("create helper dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extractDir, "helpers", "bootstrap.js"), []byte("module.exports = {};"), 0o644); err != nil {
		t.Fatalf("write helper entry failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(extractDir, "index.mjs"), []byte("export function onGeneratePaymentCard() { return { html: '<div>ok</div>' }; }"), 0o644); err != nil {
		t.Fatalf("write index.mjs failed: %v", err)
	}

	entryPath, entryPublic, err := resolveMarketPaymentPackageEntryPath(extractDir, "")
	if err != nil {
		t.Fatalf("resolveMarketPaymentPackageEntryPath returned error: %v", err)
	}
	if entryPublic != "index.mjs" {
		t.Fatalf("expected detected entry index.mjs, got %q", entryPublic)
	}
	if filepath.Clean(entryPath) != filepath.Join(extractDir, "index.mjs") {
		t.Fatalf("expected entry path %q, got %q", filepath.Join(extractDir, "index.mjs"), entryPath)
	}
}

func TestPaymentMethodMarketCatalogUsesConfiguredSources(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/catalog":
			if got := r.URL.Query().Get("kind"); got != "payment_package" {
				t.Fatalf("expected payment_package catalog kind, got %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"items": []map[string]interface{}{
						{
							"kind":           "payment_package",
							"name":           "mock-checkout",
							"title":          "Mock Checkout",
							"description":    "Hosted payment checkout",
							"latest_version": "1.1.0",
						},
					},
					"pagination": map[string]interface{}{
						"offset":   0,
						"limit":    20,
						"total":    1,
						"has_more": false,
					},
				},
			})
		case "/v1/artifacts/payment_package/mock-checkout":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": true,
				"data": map[string]interface{}{
					"kind":           "payment_package",
					"name":           "mock-checkout",
					"title":          "Mock Checkout",
					"description":    "Hosted payment checkout",
					"latest_version": "1.1.0",
					"versions": []map[string]interface{}{
						{
							"version":      "1.1.0",
							"channel":      "stable",
							"published_at": "2026-03-01T00:00:00Z",
						},
						{
							"version":      "1.0.0",
							"channel":      "stable",
							"published_at": "2026-02-01T00:00:00Z",
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)
	if err := db.Create(&models.Plugin{
		Name:    "js-market",
		Runtime: PluginRuntimeJSWorker,
		Address: "index.js",
		Config: `{
			"market": {
				"sources": [
					{
						"source_id": "official",
						"name": "Official",
						"source_base_url": "` + server.URL + `",
						"default_channel": "stable",
						"allowed_kinds": ["plugin_package", "payment_package"]
					}
				]
			}
		}`,
	}).Error; err != nil {
		t.Fatalf("create market plugin failed: %v", err)
	}
	if err := db.Create(&models.Plugin{
		Name:    "plugin-only-market",
		Runtime: PluginRuntimeJSWorker,
		Address: "index.js",
		Config: `{
			"sources": [
				{
					"source_id": "plugin-only",
					"source_base_url": "https://plugin-only.example.test",
					"allowed_kinds": ["plugin_package"]
				}
			]
		}`,
	}).Error; err != nil {
		t.Fatalf("create plugin-only market plugin failed: %v", err)
	}

	sources, err := ListPaymentMethodMarketSources(db)
	if err != nil {
		t.Fatalf("list payment market sources returned error: %v", err)
	}
	sourceItems, ok := sources["items"].([]map[string]interface{})
	if !ok {
		rawItems, rawOK := sources["items"].([]interface{})
		if !rawOK || len(rawItems) != 1 {
			t.Fatalf("expected one payment market source, got %#v", sources["items"])
		}
		item, itemOK := rawItems[0].(map[string]interface{})
		if !itemOK {
			t.Fatalf("expected source item object, got %#v", rawItems[0])
		}
		if got := item["source_id"]; got != "official" {
			t.Fatalf("expected official source_id, got %#v", got)
		}
		if got := item["allowed_kinds"]; got == nil {
			t.Fatalf("expected allowed_kinds payload, got %#v", item)
		}
	} else {
		if len(sourceItems) != 1 {
			t.Fatalf("expected one payment market source, got %#v", sourceItems)
		}
		if got := sourceItems[0]["source_id"]; got != "official" {
			t.Fatalf("expected official source_id, got %#v", got)
		}
	}

	catalog, err := ListPaymentMethodMarketCatalog(db, map[string]interface{}{
		"source_id": "official",
		"limit":     20,
	})
	if err != nil {
		t.Fatalf("list payment market catalog returned error: %v", err)
	}
	catalogSource, ok := catalog["source"].(map[string]interface{})
	if !ok || catalogSource["source_id"] != "official" {
		t.Fatalf("expected embedded payment market source summary, got %#v", catalog["source"])
	}
	catalogItems, ok := catalog["items"].([]interface{})
	if !ok || len(catalogItems) != 1 {
		t.Fatalf("expected one payment market catalog item, got %#v", catalog["items"])
	}
	firstCatalogItem, ok := catalogItems[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected catalog item object, got %#v", catalogItems[0])
	}
	if got := firstCatalogItem["name"]; got != "mock-checkout" {
		t.Fatalf("expected catalog item mock-checkout, got %#v", got)
	}

	artifact, err := GetPaymentMethodMarketArtifact(db, map[string]interface{}{
		"source_id": "official",
		"name":      "mock-checkout",
	})
	if err != nil {
		t.Fatalf("get payment market artifact returned error: %v", err)
	}
	if got := artifact["latest_version"]; got != "1.1.0" {
		t.Fatalf("expected latest_version 1.1.0, got %#v", got)
	}
	versions, ok := artifact["versions"].([]interface{})
	if !ok || len(versions) != 2 {
		t.Fatalf("expected two artifact versions, got %#v", artifact["versions"])
	}
}
