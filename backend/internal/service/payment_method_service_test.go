package service

import (
	"strings"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
)

func TestPaymentMethodServiceInitBuiltinPaymentMethodsImportsPackageBackedMethods(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.PaymentMethod{}, &models.PaymentMethodVersion{}); err != nil {
		t.Fatalf("auto migrate payment method tables failed: %v", err)
	}

	svc := NewPaymentMethodService(db, &config.Config{})
	if err := svc.InitBuiltinPaymentMethods(); err != nil {
		t.Fatalf("init builtin payment methods failed: %v", err)
	}

	var methods []models.PaymentMethod
	if err := db.Order("id ASC").Find(&methods).Error; err != nil {
		t.Fatalf("query payment methods failed: %v", err)
	}
	if len(methods) != 2 {
		t.Fatalf("expected 2 builtin payment methods, got %d", len(methods))
	}

	expectedArtifacts := map[string]string{
		"USDT TRC20":       "builtin-usdt-trc20",
		"USDT BEP20 (BSC)": "builtin-usdt-bep20-bsc",
	}
	methodByID := make(map[uint]models.PaymentMethod, len(methods))
	for _, method := range methods {
		methodByID[method.ID] = method
		artifactName, ok := expectedArtifacts[method.Name]
		if !ok {
			t.Fatalf("unexpected builtin payment method name %q", method.Name)
		}
		if method.Type != models.PaymentMethodTypeCustom {
			t.Fatalf("expected builtin method %q type=custom, got %s", method.Name, method.Type)
		}
		if strings.TrimSpace(method.PackageName) != artifactName+"-1.0.0.zip" {
			t.Fatalf("expected builtin method %q package_name=%s-1.0.0.zip, got %q", method.Name, artifactName, method.PackageName)
		}
		if strings.TrimSpace(method.PackageEntry) != "index.js" {
			t.Fatalf("expected builtin method %q package_entry=index.js, got %q", method.Name, method.PackageEntry)
		}
		if strings.TrimSpace(method.PackageChecksum) == "" {
			t.Fatalf("expected builtin method %q checksum to be populated", method.Name)
		}
		if strings.TrimSpace(method.Manifest) == "" {
			t.Fatalf("expected builtin method %q manifest to be populated", method.Name)
		}
		if !strings.Contains(method.Manifest, `"runtime":"payment_js"`) {
			t.Fatalf("expected builtin method %q manifest runtime payment_js, got %q", method.Name, method.Manifest)
		}
	}

	var versions []models.PaymentMethodVersion
	if err := db.Order("id ASC").Find(&versions).Error; err != nil {
		t.Fatalf("query payment method versions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 payment method versions after builtin init, got %d", len(versions))
	}
	for _, version := range versions {
		method, ok := methodByID[version.PaymentMethodID]
		if !ok {
			t.Fatalf("unexpected payment_method_id in version snapshot: %d", version.PaymentMethodID)
		}
		expectedArtifactName := expectedArtifacts[method.Name]
		if version.MarketSourceID != "builtin" {
			t.Fatalf("expected version market_source_id=builtin, got %q", version.MarketSourceID)
		}
		if version.MarketArtifactKind != "payment_package" {
			t.Fatalf("expected version market_artifact_kind=payment_package, got %q", version.MarketArtifactKind)
		}
		if version.MarketArtifactName != expectedArtifactName {
			t.Fatalf("expected version market_artifact_name=%q, got %q", expectedArtifactName, version.MarketArtifactName)
		}
		if version.MarketArtifactVersion != "1.0.0" {
			t.Fatalf("expected version market_artifact_version=1.0.0, got %q", version.MarketArtifactVersion)
		}
		if !version.IsActive {
			t.Fatalf("expected builtin version for %q to be active", method.Name)
		}
		if strings.TrimSpace(version.PackageName) == "" || strings.TrimSpace(version.PackageChecksum) == "" {
			t.Fatalf("expected builtin version snapshot for %q to contain package metadata", method.Name)
		}
	}

	if err := svc.InitBuiltinPaymentMethods(); err != nil {
		t.Fatalf("re-run builtin payment method init failed: %v", err)
	}

	var methodCount int64
	if err := db.Model(&models.PaymentMethod{}).Count(&methodCount).Error; err != nil {
		t.Fatalf("count payment methods failed: %v", err)
	}
	if methodCount != 2 {
		t.Fatalf("expected builtin init to stay idempotent with 2 payment methods, got %d", methodCount)
	}

	var versionCount int64
	if err := db.Model(&models.PaymentMethodVersion{}).Count(&versionCount).Error; err != nil {
		t.Fatalf("count payment method versions failed: %v", err)
	}
	if versionCount != 2 {
		t.Fatalf("expected builtin init to stay idempotent with 2 version snapshots, got %d", versionCount)
	}
}

func TestPaymentMethodServiceCreateAndUpdateLegacyPaymentMethodUsesPackageGovernance(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.PaymentMethod{}, &models.PaymentMethodVersion{}); err != nil {
		t.Fatalf("auto migrate payment method tables failed: %v", err)
	}

	svc := NewPaymentMethodService(db, &config.Config{})
	name := "Legacy Manual Method"
	description := "Legacy PaymentJS editor payload"
	icon := "Code"
	script := `
function onGeneratePaymentCard(order, config) {
  return { html: "<div>" + ((config && config.title) || "Legacy") + "</div>" };
}
function onCheckPaymentStatus() {
  return { paid: false };
}
`
	configRaw := `{"title":"Legacy"}`
	pollInterval := 45

	created, err := svc.CreateLegacyPaymentMethod(LegacyPaymentMethodUpsertInput{
		Name:         &name,
		Description:  &description,
		Icon:         &icon,
		Script:       &script,
		Config:       &configRaw,
		PollInterval: &pollInterval,
	})
	if err != nil {
		t.Fatalf("create legacy payment method failed: %v", err)
	}
	if created == nil || created.ID == 0 {
		t.Fatalf("expected created legacy payment method")
	}
	if strings.TrimSpace(created.PackageName) == "" || strings.TrimSpace(created.PackageEntry) == "" || strings.TrimSpace(created.Manifest) == "" {
		t.Fatalf("expected created legacy payment method to be package-backed, got %+v", created)
	}
	if created.Version != "1.0.0" {
		t.Fatalf("expected created legacy payment method version 1.0.0, got %q", created.Version)
	}

	updatedName := "Legacy Manual Method V2"
	updatedConfig := `{"title":"Legacy V2"}`
	updatedEnabled := false
	updated, err := svc.UpdateLegacyPaymentMethod(created.ID, LegacyPaymentMethodUpsertInput{
		Name:    &updatedName,
		Config:  &updatedConfig,
		Enabled: &updatedEnabled,
	})
	if err != nil {
		t.Fatalf("update legacy payment method failed: %v", err)
	}
	if updated.Name != updatedName {
		t.Fatalf("expected updated name %q, got %q", updatedName, updated.Name)
	}
	if updated.Enabled != updatedEnabled {
		t.Fatalf("expected updated enabled=%v, got %+v", updatedEnabled, updated)
	}
	if !strings.Contains(updated.Config, "Legacy V2") {
		t.Fatalf("expected updated config to be persisted, got %q", updated.Config)
	}

	var versions []models.PaymentMethodVersion
	if err := db.Where("payment_method_id = ?", created.ID).Order("id ASC").Find(&versions).Error; err != nil {
		t.Fatalf("query payment method versions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 version snapshots for legacy payment method, got %d", len(versions))
	}
	if versions[0].IsActive {
		t.Fatalf("expected first legacy version snapshot to be inactive after update")
	}
	if !versions[1].IsActive {
		t.Fatalf("expected latest legacy version snapshot to be active")
	}
	if versions[1].MarketSourceID != "" || versions[1].MarketArtifactKind != "" || versions[1].MarketArtifactName != "" || versions[1].MarketArtifactVersion != "" {
		t.Fatalf("expected legacy payment method snapshots to stay outside market coordinates, got %+v", versions[1])
	}
}

func TestPaymentMethodServiceInitBuiltinPaymentMethodsAutoMigratesLegacyRecords(t *testing.T) {
	db := openPluginManagerE2ETestDB(t)
	if err := db.AutoMigrate(&models.PaymentMethod{}, &models.PaymentMethodVersion{}); err != nil {
		t.Fatalf("auto migrate payment method tables failed: %v", err)
	}

	legacyMethod := models.PaymentMethod{
		Name:         "Legacy Existing Payment",
		Description:  "Old PaymentJS record",
		Type:         models.PaymentMethodTypeCustom,
		Enabled:      false,
		Icon:         "Code",
		Script:       "function onGeneratePaymentCard(){ return { html: '<div>legacy</div>' }; }",
		Config:       `{"mode":"legacy"}`,
		PollInterval: 55,
	}
	if err := db.Create(&legacyMethod).Error; err != nil {
		t.Fatalf("create legacy payment method failed: %v", err)
	}
	if err := db.Model(&models.PaymentMethod{}).Where("id = ?", legacyMethod.ID).Update("enabled", false).Error; err != nil {
		t.Fatalf("force legacy enabled=false failed: %v", err)
	}

	svc := NewPaymentMethodService(db, &config.Config{})
	if err := svc.InitBuiltinPaymentMethods(); err != nil {
		t.Fatalf("init builtin payment methods failed: %v", err)
	}

	var migrated models.PaymentMethod
	if err := db.First(&migrated, legacyMethod.ID).Error; err != nil {
		t.Fatalf("reload migrated legacy payment method failed: %v", err)
	}
	if migrated.Enabled {
		t.Fatalf("expected legacy enabled flag to be preserved, got %+v", migrated)
	}
	if strings.TrimSpace(migrated.PackageName) == "" || strings.TrimSpace(migrated.PackageEntry) == "" || strings.TrimSpace(migrated.PackageChecksum) == "" || strings.TrimSpace(migrated.Manifest) == "" {
		t.Fatalf("expected legacy payment method to be auto-migrated into package governance, got %+v", migrated)
	}

	var versions []models.PaymentMethodVersion
	if err := db.Where("payment_method_id = ?", migrated.ID).Order("id ASC").Find(&versions).Error; err != nil {
		t.Fatalf("query migrated payment method versions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 migrated legacy version snapshot, got %d", len(versions))
	}
	if versions[0].MarketSourceID != "" || versions[0].MarketArtifactKind != "" || versions[0].MarketArtifactName != "" || versions[0].MarketArtifactVersion != "" {
		t.Fatalf("expected migrated legacy record to have empty market coordinates, got %+v", versions[0])
	}

	var methodCount int64
	if err := db.Model(&models.PaymentMethod{}).Count(&methodCount).Error; err != nil {
		t.Fatalf("count payment methods failed: %v", err)
	}
	if methodCount != 3 {
		t.Fatalf("expected 3 payment methods after builtin init plus legacy migration, got %d", methodCount)
	}
}
