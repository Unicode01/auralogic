package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
)

func TestExecutePluginHostActionListsAndRollsBackPaymentPackageHistory(t *testing.T) {
	buildPaymentPackage := func(version string, label string) []byte {
		t.Helper()
		manifest := map[string]interface{}{
			"name":                      "mock-checkout",
			"display_name":              "Mock Checkout",
			"description":               label,
			"icon":                      "CreditCard",
			"entry":                     "index.js",
			"version":                   version,
			"manifest_version":          "1.0.0",
			"protocol_version":          "1.0.0",
			"min_host_protocol_version": "1.0.0",
			"max_host_protocol_version": "1.0.0",
			"config_schema": map[string]interface{}{
				"fields": []map[string]interface{}{
					{
						"key":      "checkout_title",
						"type":     "string",
						"default":  label,
						"required": true,
					},
				},
			},
		}
		manifestRaw, err := json.Marshal(manifest)
		if err != nil {
			t.Fatalf("marshal payment manifest failed: %v", err)
		}
		return buildPluginMarketTestZip(t, map[string]string{
			"manifest.json": string(manifestRaw),
			"index.js": `
function onGeneratePaymentCard(order, config) {
  return { html: "<div>" + ((config && config.checkout_title) || "Mock") + "</div>" };
}
function onCheckPaymentStatus() { return { paid: false }; }
`,
		})
	}

	packageV100 := buildPaymentPackage("1.0.0", "Mock Checkout v1")
	packageV110 := buildPaymentPackage("1.1.0", "Mock Checkout v2")
	shaV100 := computeSHA256Hex(packageV100)
	shaV110 := computeSHA256Hex(packageV110)

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/payment_package/mock-checkout/releases/1.0.0":
			_ = json.NewEncoder(w).Encode(buildPaymentMarketReleaseEnvelope(server.URL, "1.0.0", packageV100))
		case "/v1/artifacts/payment_package/mock-checkout/releases/1.1.0":
			_ = json.NewEncoder(w).Encode(buildPaymentMarketReleaseEnvelope(server.URL, "1.1.0", packageV110))
		case "/download/mock-checkout-1.0.0.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(packageV100)
		case "/download/mock-checkout-1.1.0.zip":
			w.Header().Set("Content-Type", "application/zip")
			_, _ = w.Write(packageV110)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.PaymentMethod{}, &models.PaymentMethodVersion{}); err != nil {
		t.Fatalf("auto migrate payment method versions failed: %v", err)
	}

	marketPlugin := models.Plugin{
		Name:    "market-plugin",
		Runtime: PluginRuntimeJSWorker,
		Address: "index.js",
		Config: `{
			"sources": [
				{
					"source_id": "official",
					"source_base_url": "` + server.URL + `",
					"allowed_kinds": ["payment_package"]
				}
			]
		}`,
	}
	if err := db.Create(&marketPlugin).Error; err != nil {
		t.Fatalf("create market plugin failed: %v", err)
	}

	runtime := NewPluginHostRuntime(db, &config.Config{
		Plugin: config.PluginPlatformConfig{
			ArtifactDir: t.TempDir(),
		},
	}, nil)
	executeClaims := &PluginHostAccessClaims{
		PluginID: marketPlugin.ID,
		GrantedPermissions: []string{
			PluginPermissionHostMarketInstallExecute,
		},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}
	readClaims := &PluginHostAccessClaims{
		PluginID: marketPlugin.ID,
		GrantedPermissions: []string{
			PluginPermissionHostMarketInstallRead,
		},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}
	rollbackClaims := &PluginHostAccessClaims{
		PluginID: marketPlugin.ID,
		GrantedPermissions: []string{
			PluginPermissionHostMarketInstallRollback,
		},
		ScopeAuthenticated: true,
		ScopeSuperAdmin:    true,
	}

	resultV100, err := ExecutePluginHostActionWithRuntime(
		runtime,
		executeClaims,
		"host.market.install.execute",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "payment_package",
			"name":      "mock-checkout",
			"version":   "1.0.0",
		},
	)
	if err != nil {
		t.Fatalf("install 1.0.0 returned error: %v", err)
	}
	methodV100, ok := resultV100["item"].(*models.PaymentMethod)
	if !ok || methodV100 == nil || methodV100.ID == 0 {
		t.Fatalf("expected payment method payload after first install, got %#v", resultV100["item"])
	}
	if methodV100.PackageChecksum != shaV100 {
		t.Fatalf("expected first install checksum %s, got %#v", shaV100, methodV100.PackageChecksum)
	}

	resultV110, err := ExecutePluginHostActionWithRuntime(
		runtime,
		executeClaims,
		"host.market.install.execute",
		map[string]interface{}{
			"source_id":         "official",
			"kind":              "payment_package",
			"name":              "mock-checkout",
			"version":           "1.1.0",
			"payment_method_id": methodV100.ID,
		},
	)
	if err != nil {
		t.Fatalf("install 1.1.0 returned error: %v", err)
	}
	methodV110, ok := resultV110["item"].(*models.PaymentMethod)
	if !ok || methodV110 == nil {
		t.Fatalf("expected payment method payload after second install, got %#v", resultV110["item"])
	}
	if methodV110.ID != methodV100.ID {
		t.Fatalf("expected update in place for payment method %d, got %#v", methodV100.ID, methodV110.ID)
	}
	if methodV110.PackageChecksum != shaV110 {
		t.Fatalf("expected second install checksum %s, got %#v", shaV110, methodV110.PackageChecksum)
	}

	historyResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		readClaims,
		"host.market.install.history.list",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "payment_package",
			"name":      "mock-checkout",
			"limit":     10,
		},
	)
	if err != nil {
		t.Fatalf("history.list returned error: %v", err)
	}

	if typedItems, ok := historyResult["items"].([]map[string]interface{}); ok {
		if len(typedItems) != 2 {
			t.Fatalf("expected two payment history items, got %#v", typedItems)
		}
		if typedItems[0]["version"] != "1.1.0" || typedItems[0]["installed_target_type"] != "payment_method" {
			t.Fatalf("expected latest payment history item for 1.1.0, got %#v", typedItems[0])
		}
	} else {
		historyItems, ok := historyResult["items"].([]interface{})
		if !ok || len(historyItems) != 2 {
			t.Fatalf("expected two payment history items, got %#v", historyResult["items"])
		}
		firstHistory, ok := historyItems[0].(map[string]interface{})
		if !ok {
			t.Fatalf("expected first payment history item object, got %#v", historyItems[0])
		}
		if firstHistory["version"] != "1.1.0" || firstHistory["installed_target_type"] != "payment_method" {
			t.Fatalf("expected latest payment history item for 1.1.0, got %#v", firstHistory)
		}
	}

	rollbackResult, err := ExecutePluginHostActionWithRuntime(
		runtime,
		rollbackClaims,
		"host.market.install.rollback",
		map[string]interface{}{
			"source_id": "official",
			"kind":      "payment_package",
			"name":      "mock-checkout",
			"version":   "1.0.0",
		},
	)
	if err != nil {
		t.Fatalf("rollback returned error: %v", err)
	}
	if got := rollbackResult["status"]; got != "rolled_back" {
		t.Fatalf("expected rolled_back status, got %#v", got)
	}

	var installed models.PaymentMethod
	if err := db.First(&installed, methodV100.ID).Error; err != nil {
		t.Fatalf("load installed payment method failed: %v", err)
	}
	if installed.Version != "1.0.0" {
		t.Fatalf("expected payment method version rolled back to 1.0.0, got %+v", installed)
	}
	if installed.PackageChecksum != shaV100 {
		t.Fatalf("expected rolled back checksum %s, got %#v", shaV100, installed.PackageChecksum)
	}

	var activeVersion models.PaymentMethodVersion
	if err := db.Where("payment_method_id = ? AND is_active = ?", installed.ID, true).First(&activeVersion).Error; err != nil {
		t.Fatalf("load active payment method version failed: %v", err)
	}
	if activeVersion.MarketArtifactVersion != "1.0.0" {
		t.Fatalf("expected active payment version 1.0.0 after rollback, got %+v", activeVersion)
	}
}

func buildPaymentMarketReleaseEnvelope(baseURL string, version string, packageBytes []byte) map[string]interface{} {
	return map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"kind":    "payment_package",
			"name":    "mock-checkout",
			"version": version,
			"governance": map[string]interface{}{
				"mode": "host_managed",
			},
			"download": map[string]interface{}{
				"url":    baseURL + "/download/mock-checkout-" + version + ".zip",
				"sha256": computeSHA256Hex(packageBytes),
				"size":   len(packageBytes),
			},
			"compatibility": map[string]interface{}{
				"min_host_bridge_version": "1.0.0",
			},
		},
	}
}
