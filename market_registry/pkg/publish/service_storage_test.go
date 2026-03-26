package publish

import (
	"archive/zip"
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"auralogic/market_registry/pkg/registrysettings"
	"auralogic/market_registry/pkg/signing"
	"auralogic/market_registry/pkg/storage"
)

func TestDeleteReleaseRemovesExternalArtifactPayload(t *testing.T) {
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
			DefaultProfileID: "cleanup-store",
			Profiles: []registrysettings.ArtifactStorageProfile{
				{
					ID:      "cleanup-store",
					Name:    "Cleanup Store",
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

	service := NewServiceWithOptions(primaryStore, signSvc, "official-test", Options{
		Settings: settingsSvc,
	})
	zipPayload := buildPublishTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "cleanup-demo",
  "version": "1.0.0",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})
	if err := service.Publish(ctx, Request{
		Channel:                  "stable",
		ArtifactStorageProfileID: "cleanup-store",
		ArtifactZip:              zipPayload,
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	secondaryStore, err := storage.NewLocalStorage(filepath.Join(root, "secondary"), "")
	if err != nil {
		t.Fatalf("NewLocalStorage secondary returned error: %v", err)
	}
	artifactPath := "artifacts/plugin_package/cleanup-demo/1.0.0/cleanup-demo-1.0.0.zip"
	exists, err := secondaryStore.Exists(ctx, artifactPath)
	if err != nil {
		t.Fatalf("secondary Exists returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected artifact zip to exist before delete")
	}

	if _, err := service.DeleteRelease(ctx, "plugin_package", "cleanup-demo", "1.0.0"); err != nil {
		t.Fatalf("DeleteRelease returned error: %v", err)
	}

	exists, err = secondaryStore.Exists(ctx, artifactPath)
	if err != nil {
		t.Fatalf("secondary Exists after delete returned error: %v", err)
	}
	if exists {
		t.Fatalf("expected external artifact zip to be deleted")
	}
}

func TestDeleteArtifactRemovesExternalPayloadEvenWhenManifestIsMissing(t *testing.T) {
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
			DefaultProfileID: "artifact-delete-store",
			Profiles: []registrysettings.ArtifactStorageProfile{
				{
					ID:      "artifact-delete-store",
					Name:    "Artifact Delete Store",
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

	service := NewServiceWithOptions(primaryStore, signSvc, "official-test", Options{
		Settings: settingsSvc,
	})
	zipPayload := buildPublishTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "artifact-delete-demo",
  "version": "1.0.0",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})
	if err := service.Publish(ctx, Request{
		Channel:                  "stable",
		ArtifactStorageProfileID: "artifact-delete-store",
		ArtifactZip:              zipPayload,
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	if err := primaryStore.Delete(ctx, "artifacts/plugin_package/artifact-delete-demo/1.0.0/manifest.json"); err != nil {
		t.Fatalf("Delete manifest returned error: %v", err)
	}

	secondaryStore, err := storage.NewLocalStorage(filepath.Join(root, "secondary"), "")
	if err != nil {
		t.Fatalf("NewLocalStorage secondary returned error: %v", err)
	}
	artifactPath := "artifacts/plugin_package/artifact-delete-demo/1.0.0/artifact-delete-demo-1.0.0.zip"
	exists, err := secondaryStore.Exists(ctx, artifactPath)
	if err != nil {
		t.Fatalf("secondary Exists returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected external artifact zip to exist before artifact delete")
	}

	result, err := service.DeleteArtifact(ctx, "plugin_package", "artifact-delete-demo")
	if err != nil {
		t.Fatalf("DeleteArtifact returned error: %v", err)
	}
	if len(result.DeletedVersions) != 1 || result.DeletedVersions[0] != "1.0.0" {
		t.Fatalf("expected deleted version 1.0.0, got %#v", result.DeletedVersions)
	}

	exists, err = secondaryStore.Exists(ctx, artifactPath)
	if err != nil {
		t.Fatalf("secondary Exists after artifact delete returned error: %v", err)
	}
	if exists {
		t.Fatalf("expected external artifact zip to be deleted with artifact")
	}
}

func buildPublishTestZip(t *testing.T, files map[string]string) []byte {
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
