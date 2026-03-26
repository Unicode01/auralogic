package catalog_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"auralogic/market_registry/pkg/catalog"
	"auralogic/market_registry/pkg/publish"
	"auralogic/market_registry/pkg/registrysettings"
	"auralogic/market_registry/pkg/signing"
	"auralogic/market_registry/pkg/storage"
)

func TestRuntimeStoreReadsArtifactFromConfiguredStorageProfile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	primaryStore, err := storage.NewLocalStorage(filepath.Join(root, "primary"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage primary returned error: %v", err)
	}
	settingsSvc := registrysettings.NewService(primaryStore, registrysettings.ArtifactStorageProfile{
		ID:      registrysettings.CanonicalArtifactStorageProfileID,
		Name:    "Canonical Storage",
		Type:    "local",
		BaseDir: filepath.Join(root, "primary"),
		BaseURL: "http://localhost:18080",
	})
	_, err = settingsSvc.Update(ctx, registrysettings.Document{
		ArtifactStorage: registrysettings.ArtifactStorageSettings{
			DefaultProfileID: "secondary-local",
			Profiles: []registrysettings.ArtifactStorageProfile{
				{
					ID:      "secondary-local",
					Name:    "Secondary Local",
					Type:    "local",
					BaseDir: filepath.Join(root, "secondary"),
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Update settings returned error: %v", err)
	}

	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}

	service := publish.NewServiceWithOptions(primaryStore, signSvc, "official-test", publish.Options{
		BaseURL:  "http://localhost:18080",
		Settings: settingsSvc,
	})
	zipPayload := buildCatalogTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "storage-demo",
  "version": "1.0.0",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})
	if err := service.Publish(ctx, publish.Request{
		Channel:                  "stable",
		ArtifactStorageProfileID: "secondary-local",
		ArtifactZip:              zipPayload,
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	artifactPath := "artifacts/plugin_package/storage-demo/1.0.0/storage-demo-1.0.0.zip"
	exists, err := primaryStore.Exists(ctx, artifactPath)
	if err != nil {
		t.Fatalf("primary Exists returned error: %v", err)
	}
	if exists {
		t.Fatalf("expected artifact zip to be absent from canonical store")
	}

	secondaryStore, err := storage.NewLocalStorage(filepath.Join(root, "secondary"), "")
	if err != nil {
		t.Fatalf("NewLocalStorage secondary returned error: %v", err)
	}
	exists, err = secondaryStore.Exists(ctx, artifactPath)
	if err != nil {
		t.Fatalf("secondary Exists returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected artifact zip to exist in secondary store")
	}

	originBody, err := primaryStore.Read(ctx, "artifacts/plugin_package/storage-demo/1.0.0/origin.json")
	if err != nil {
		t.Fatalf("Read origin returned error: %v", err)
	}
	var origin map[string]any
	if err := json.Unmarshal(originBody, &origin); err != nil {
		t.Fatalf("Unmarshal origin returned error: %v", err)
	}
	if got := origin["storage_profile_id"]; got != "secondary-local" {
		t.Fatalf("expected storage_profile_id secondary-local, got %#v", got)
	}

	runtimeStore := catalog.NewRuntimeStore(primaryStore, signSvc, catalog.RuntimeStoreConfig{
		KeyID:    "official-test",
		Settings: settingsSvc,
	})
	payload, contentType, err := runtimeStore.ReadReleaseArtifact(ctx, "plugin_package", "storage-demo", "1.0.0")
	if err != nil {
		t.Fatalf("ReadReleaseArtifact returned error: %v", err)
	}
	if contentType != "application/zip" {
		t.Fatalf("expected application/zip, got %q", contentType)
	}
	if !bytes.Equal(payload, zipPayload) {
		t.Fatalf("downloaded payload mismatch")
	}
}

func buildCatalogTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("Create %s returned error: %v", name, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("Write %s returned error: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close zip writer returned error: %v", err)
	}
	return body.Bytes()
}
