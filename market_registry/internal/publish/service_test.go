package publish

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"auralogic/market_registry/internal/signing"
	"auralogic/market_registry/internal/storage"
)

func TestPublishUpdatesArtifactIndexWithoutTypeAssertionPanics(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewService(store, signSvc, "official-test")

	buildArtifact := func(version string) []byte {
		payload, buildErr := buildTestZip(map[string]string{
			"manifest.json": `{
  "name": "demo-market",
  "display_name": "Demo Market",
  "version": "` + version + `",
  "description": "demo package"
}`,
			"index.js": `module.exports = {};`,
		})
		if buildErr != nil {
			t.Fatalf("buildZip returned error: %v", buildErr)
		}
		return payload
	}

	if err := svc.Publish(ctx, Request{
		Kind:        "plugin_package",
		Channel:     "stable",
		ArtifactZip: buildArtifact("1.0.0"),
		Metadata: Metadata{
			Title:   "Demo Market",
			Summary: "First release",
		},
	}); err != nil {
		t.Fatalf("first Publish returned error: %v", err)
	}

	if err := svc.Publish(ctx, Request{
		Kind:        "plugin_package",
		Name:        "demo-market",
		Version:     "1.1.0",
		Channel:     "stable",
		ArtifactZip: buildArtifact("1.1.0"),
		Metadata: Metadata{
			Title:   "Demo Market",
			Summary: "Second release",
		},
	}); err != nil {
		t.Fatalf("second Publish returned error: %v", err)
	}

	if err := svc.Publish(ctx, Request{
		Kind:        "plugin_package",
		Name:        "demo-market",
		Version:     "1.1.0",
		Channel:     "stable",
		ArtifactZip: buildArtifact("1.1.0"),
		Metadata: Metadata{
			Title:   "Demo Market",
			Summary: "Second release updated",
		},
	}); err != nil {
		t.Fatalf("third Publish returned error: %v", err)
	}

	indexBody, err := store.Read(ctx, "index/artifacts/plugin_package/demo-market/index.json")
	if err != nil {
		t.Fatalf("Read index returned error: %v", err)
	}

	var index map[string]any
	if err := json.Unmarshal(indexBody, &index); err != nil {
		t.Fatalf("Unmarshal index returned error: %v", err)
	}

	if got := index["latest_version"]; got != "1.1.0" {
		t.Fatalf("expected latest_version 1.1.0, got %#v", got)
	}

	versions, ok := index["versions"].([]any)
	if !ok {
		t.Fatalf("expected versions array, got %#v", index["versions"])
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 unique versions, got %#v", versions)
	}

	releases, ok := index["releases"].([]any)
	if !ok {
		t.Fatalf("expected releases array, got %#v", index["releases"])
	}
	if len(releases) != 2 {
		t.Fatalf("expected releases alias to contain 2 entries, got %#v", releases)
	}
	firstRelease := releases[0].(map[string]any)
	if got := firstRelease["sha256"]; got == "" {
		t.Fatalf("expected indexed release sha256, got %#v", got)
	}
	if got := firstRelease["size"]; got != float64(len(buildArtifact("1.1.0"))) && got != len(buildArtifact("1.1.0")) {
		t.Fatalf("expected indexed release size to be populated, got %#v", got)
	}

	if _, err := store.Read(ctx, "index/source.json"); !os.IsNotExist(err) {
		t.Fatalf("expected source snapshot to remain absent before explicit rebuild, got err=%v", err)
	}
}

func TestPublishRejectsMissingRequiredFields(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewService(store, signSvc, "official-test")

	if err := svc.Publish(ctx, Request{}); err == nil {
		t.Fatal("expected empty publish request to be rejected")
	} else if !IsRequestError(err) {
		t.Fatalf("expected request error for empty publish request, got %T", err)
	}
}

func TestPublishRejectsExplicitIdentityMismatch(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewService(store, signSvc, "official-test")

	payload, err := buildTestZip(map[string]string{
		"manifest.json": `{
  "name": "manifest-demo",
  "display_name": "Manifest Demo",
  "version": "1.0.0",
  "description": "demo package",
  "entry": "index.js",
  "runtime": "js_worker"
}`,
		"index.js": `module.exports = {};`,
	})
	if err != nil {
		t.Fatalf("buildZip returned error: %v", err)
	}

	err = svc.Publish(ctx, Request{
		Kind:        "plugin_package",
		Name:        "manual-demo",
		Version:     "1.0.0",
		Channel:     "stable",
		ArtifactZip: payload,
	})
	if err == nil {
		t.Fatal("expected mismatch publish request to be rejected")
	}
	if !IsRequestError(err) {
		t.Fatalf("expected request error, got %T", err)
	}
	if !strings.Contains(err.Error(), `name "manual-demo" does not match manifest name "manifest-demo"`) {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPublishRejectsUnsupportedKind(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewService(store, signSvc, "official-test")

	payload, err := buildTestZip(map[string]string{
		"manifest.json": `{
  "kind": "unknown_package",
  "name": "manifest-demo",
  "version": "1.0.0"
}`,
	})
	if err != nil {
		t.Fatalf("buildZip returned error: %v", err)
	}

	err = svc.Publish(ctx, Request{
		Channel:     "stable",
		ArtifactZip: payload,
	})
	if err == nil {
		t.Fatal("expected unsupported kind publish request to be rejected")
	}
	if !IsRequestError(err) {
		t.Fatalf("expected request error, got %T", err)
	}
	if !strings.Contains(err.Error(), `unsupported artifact kind "unknown_package"`) {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPublishRejectsUnsafeNameFromManifest(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewService(store, signSvc, "official-test")

	payload, err := buildTestZip(map[string]string{
		"manifest.json": `{
  "kind": "plugin_package",
  "name": "../unsafe-demo",
  "version": "1.0.0",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})
	if err != nil {
		t.Fatalf("buildZip returned error: %v", err)
	}

	err = svc.Publish(ctx, Request{
		Channel:     "stable",
		ArtifactZip: payload,
	})
	if err == nil {
		t.Fatal("expected unsafe name publish request to be rejected")
	}
	if !IsRequestError(err) {
		t.Fatalf("expected request error, got %T", err)
	}
	if !strings.Contains(err.Error(), "name contains forbidden path characters") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPublishInfersPluginPackageKindFromManifestRuntime(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewService(store, signSvc, "official-test")

	payload, err := buildTestZip(map[string]string{
		"manifest.json": `{
  "name": "runtime-kind-demo",
  "display_name": "Runtime Kind Demo",
  "version": "1.0.0",
  "description": "demo package",
  "entry": "index.js",
  "runtime": "js_worker"
}`,
		"index.js": `module.exports = {};`,
	})
	if err != nil {
		t.Fatalf("buildZip returned error: %v", err)
	}

	if err := svc.Publish(ctx, Request{
		Channel:     "stable",
		ArtifactZip: payload,
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	if _, err := store.Read(ctx, "artifacts/plugin_package/runtime-kind-demo/1.0.0/manifest.json"); err != nil {
		t.Fatalf("expected inferred plugin_package artifact to exist, got error: %v", err)
	}
}

func TestPublishBuildsRichTemplateReleaseManifest(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewService(store, signSvc, "official-test")

	payload, err := buildTestZip(map[string]string{
		"manifest.json": `{
  "kind": "email_template",
  "name": "order_paid",
  "title": "Order Paid Email",
  "version": "1.0.0",
  "event": "order_paid",
  "engine": "go_template",
  "content_file": "template.html",
  "subject": "Your order has been paid",
  "docs_url": "https://example.com/docs/order-paid"
}`,
		"template.html": `<html><body>paid {{.OrderNo}}</body></html>`,
	})
	if err != nil {
		t.Fatalf("buildZip returned error: %v", err)
	}

	if err := svc.Publish(ctx, Request{
		Channel:     "stable",
		ArtifactZip: payload,
		Metadata: Metadata{
			Title:       "Order Paid Email",
			Summary:     "Transactional email",
			Description: "Published from tests",
		},
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	manifestBody, err := store.Read(ctx, "artifacts/email_template/order_paid/1.0.0/manifest.json")
	if err != nil {
		t.Fatalf("Read release manifest returned error: %v", err)
	}

	var manifest map[string]any
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		t.Fatalf("Unmarshal release manifest returned error: %v", err)
	}

	templateData := manifest["template"].(map[string]any)
	if got := templateData["content"]; got == "" {
		t.Fatalf("expected template content to be embedded, got %#v", templateData)
	}
	targets := manifest["targets"].(map[string]any)
	if got := targets["event"]; got != "order_paid" {
		t.Fatalf("expected template target event order_paid, got %#v", got)
	}
	install := manifest["install"].(map[string]any)
	if got := install["inline_content"]; got != true {
		t.Fatalf("expected inline_content true for template release, got %#v", got)
	}
	docs := manifest["docs"].(map[string]any)
	if got := docs["docs_url"]; got != "https://example.com/docs/order-paid" {
		t.Fatalf("expected docs_url to be preserved, got %#v", got)
	}
	download := manifest["download"].(map[string]any)
	if got := download["filename"]; got != "order_paid-1.0.0.zip" {
		t.Fatalf("expected download filename order_paid-1.0.0.zip, got %#v", got)
	}
	transport := download["transport"].(map[string]any)
	if got := transport["provider"]; got != "local" {
		t.Fatalf("expected local transport provider, got %#v", got)
	}
	if got := transport["mode"]; got != "mirror" {
		t.Fatalf("expected mirror transport mode, got %#v", got)
	}

	originBody, err := store.Read(ctx, "artifacts/email_template/order_paid/1.0.0/origin.json")
	if err != nil {
		t.Fatalf("Read origin document returned error: %v", err)
	}
	var origin map[string]any
	if err := json.Unmarshal(originBody, &origin); err != nil {
		t.Fatalf("Unmarshal origin document returned error: %v", err)
	}
	if got := origin["provider"]; got != "local" {
		t.Fatalf("expected origin provider local, got %#v", got)
	}
	if got := origin["mode"]; got != "mirror" {
		t.Fatalf("expected origin mode mirror, got %#v", got)
	}
}

func TestRebuildRegistryCreatesSourceAndCatalogSnapshots(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewServiceWithOptions(store, signSvc, "official-test", Options{
		SourceID:   "official",
		SourceName: "AuraLogic Official Source",
		BaseURL:    "http://localhost:18080",
	})

	payload, err := buildTestZip(map[string]string{
		"manifest.json": `{
  "kind": "plugin_package",
  "name": "demo-market",
  "display_name": "Demo Market",
  "version": "1.0.0",
  "description": "demo package",
  "entry": "index.js",
  "runtime": "js_worker"
}`,
		"index.js": `module.exports = {};`,
	})
	if err != nil {
		t.Fatalf("buildZip returned error: %v", err)
	}

	if err := svc.Publish(ctx, Request{
		Kind:        "plugin_package",
		Channel:     "stable",
		ArtifactZip: payload,
		Metadata: Metadata{
			Title:   "Demo Market",
			Summary: "Demo plugin package",
		},
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	result, err := svc.RebuildRegistry(ctx)
	if err != nil {
		t.Fatalf("RebuildRegistry returned error: %v", err)
	}
	if result.TotalArtifacts != 1 {
		t.Fatalf("expected TotalArtifacts 1, got %d", result.TotalArtifacts)
	}

	sourceSnapshot, err := store.Read(ctx, result.SourcePath)
	if err != nil {
		t.Fatalf("expected source snapshot to be generated, got error: %v", err)
	}
	if len(sourceSnapshot) == 0 {
		t.Fatal("expected source snapshot to be non-empty")
	}

	catalogSnapshot, err := store.Read(ctx, result.CatalogPath)
	if err != nil {
		t.Fatalf("expected catalog snapshot to be generated, got error: %v", err)
	}
	if len(catalogSnapshot) == 0 {
		t.Fatal("expected catalog snapshot to be non-empty")
	}
}

func TestDeleteReleaseRebuildsArtifactIndex(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewService(store, signSvc, "official-test")

	buildArtifact := func(version string) []byte {
		payload, buildErr := buildTestZip(map[string]string{
			"manifest.json": `{
  "name": "delete-demo",
  "display_name": "Delete Demo",
  "version": "` + version + `",
  "description": "delete package",
  "entry": "index.js",
  "runtime": "js_worker"
}`,
			"index.js": `module.exports = {};`,
		})
		if buildErr != nil {
			t.Fatalf("buildZip returned error: %v", buildErr)
		}
		return payload
	}

	for _, version := range []string{"1.0.0", "1.1.0"} {
		if err := svc.Publish(ctx, Request{
			Kind:        "plugin_package",
			Name:        "delete-demo",
			Version:     version,
			Channel:     "stable",
			ArtifactZip: buildArtifact(version),
			Metadata: Metadata{
				Title:   "Delete Demo " + version,
				Summary: "Delete demo " + version,
			},
		}); err != nil {
			t.Fatalf("Publish(%s) returned error: %v", version, err)
		}
	}

	result, err := svc.DeleteRelease(ctx, "plugin_package", "delete-demo", "1.1.0")
	if err != nil {
		t.Fatalf("DeleteRelease returned error: %v", err)
	}
	if result.ArtifactDeleted {
		t.Fatalf("expected artifact to remain after deleting a single version, got %+v", result)
	}
	if result.RemainingVersions != 1 || result.LatestVersion != "1.0.0" {
		t.Fatalf("expected remaining version 1.0.0, got %+v", result)
	}

	indexBody, err := store.Read(ctx, "index/artifacts/plugin_package/delete-demo/index.json")
	if err != nil {
		t.Fatalf("Read rebuilt index returned error: %v", err)
	}
	var index map[string]any
	if err := json.Unmarshal(indexBody, &index); err != nil {
		t.Fatalf("Unmarshal rebuilt index returned error: %v", err)
	}
	if got := index["latest_version"]; got != "1.0.0" {
		t.Fatalf("expected latest_version 1.0.0, got %#v", got)
	}
	releases := index["releases"].([]any)
	if len(releases) != 1 {
		t.Fatalf("expected 1 remaining release, got %#v", releases)
	}

	if _, err := store.Read(ctx, "artifacts/plugin_package/delete-demo/1.1.0/manifest.json"); !os.IsNotExist(err) {
		t.Fatalf("expected deleted release manifest to be removed, got err=%v", err)
	}
	if _, err := store.Read(ctx, "artifacts/plugin_package/delete-demo/1.0.0/manifest.json"); err != nil {
		t.Fatalf("expected remaining release manifest to exist, got %v", err)
	}
}

func TestDeleteArtifactRemovesAllFilesAndIndex(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewService(store, signSvc, "official-test")

	payload, err := buildTestZip(map[string]string{
		"manifest.json": `{
  "name": "delete-all-demo",
  "display_name": "Delete All Demo",
  "version": "2.0.0",
  "description": "delete package",
  "entry": "index.js",
  "runtime": "js_worker"
}`,
		"index.js": `module.exports = {};`,
	})
	if err != nil {
		t.Fatalf("buildZip returned error: %v", err)
	}

	if err := svc.Publish(ctx, Request{
		Kind:        "plugin_package",
		Name:        "delete-all-demo",
		Version:     "2.0.0",
		Channel:     "stable",
		ArtifactZip: payload,
		Metadata: Metadata{
			Title:   "Delete All Demo",
			Summary: "Delete all demo",
		},
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	result, err := svc.DeleteArtifact(ctx, "plugin_package", "delete-all-demo")
	if err != nil {
		t.Fatalf("DeleteArtifact returned error: %v", err)
	}
	if !result.ArtifactDeleted || result.RemainingVersions != 0 {
		t.Fatalf("expected artifact to be fully deleted, got %+v", result)
	}

	if _, err := store.Read(ctx, "index/artifacts/plugin_package/delete-all-demo/index.json"); !os.IsNotExist(err) {
		t.Fatalf("expected artifact index to be removed, got err=%v", err)
	}
	if _, err := store.Read(ctx, "artifacts/plugin_package/delete-all-demo/2.0.0/manifest.json"); !os.IsNotExist(err) {
		t.Fatalf("expected artifact manifest to be removed, got err=%v", err)
	}
}

func TestPublishSameVersionAcrossChannelsMergesIndexEntry(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewService(store, signSvc, "official-test")

	payload, err := buildTestZip(map[string]string{
		"manifest.json": `{
  "name": "channel-demo",
  "display_name": "Channel Demo",
  "version": "1.0.0",
  "description": "channel merge package",
  "entry": "index.js",
  "runtime": "js_worker"
}`,
		"index.js": `module.exports = {};`,
	})
	if err != nil {
		t.Fatalf("buildZip returned error: %v", err)
	}

	for _, channel := range []string{"alpha", "stable", "beta"} {
		if err := svc.Publish(ctx, Request{
			Kind:        "plugin_package",
			Name:        "channel-demo",
			Version:     "1.0.0",
			Channel:     channel,
			ArtifactZip: payload,
			Metadata: Metadata{
				Title:   "Channel Demo",
				Summary: "Channel Demo Summary",
			},
		}); err != nil {
			t.Fatalf("Publish(%s) returned error: %v", channel, err)
		}
	}

	indexBody, err := store.Read(ctx, "index/artifacts/plugin_package/channel-demo/index.json")
	if err != nil {
		t.Fatalf("Read index returned error: %v", err)
	}

	var index map[string]any
	if err := json.Unmarshal(indexBody, &index); err != nil {
		t.Fatalf("Unmarshal index returned error: %v", err)
	}

	releases := index["releases"].([]any)
	if len(releases) != 1 {
		t.Fatalf("expected one merged release entry, got %#v", releases)
	}
	release := releases[0].(map[string]any)
	if got := release["version"]; got != "1.0.0" {
		t.Fatalf("expected merged version 1.0.0, got %#v", got)
	}
	channels := release["channels"].([]any)
	if len(channels) != 3 {
		t.Fatalf("expected three merged channels, got %#v", channels)
	}
	channelSet := map[string]bool{}
	for _, item := range channels {
		channelSet[strings.TrimSpace(item.(string))] = true
	}
	for _, channel := range []string{"alpha", "stable", "beta"} {
		if !channelSet[channel] {
			t.Fatalf("expected channel %s to be present, got %#v", channel, channels)
		}
	}
}

func TestRegistryStatusReportsStaleThenHealthyAfterRebuild(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewServiceWithOptions(store, signSvc, "official-test", Options{
		SourceID:   "official",
		SourceName: "AuraLogic Official Source",
		BaseURL:    "http://localhost:18080",
	})

	payload, err := buildTestZip(map[string]string{
		"manifest.json": `{
  "kind": "plugin_package",
  "name": "demo-status",
  "display_name": "Demo Status",
  "version": "1.0.0",
  "description": "demo package",
  "entry": "index.js",
  "runtime": "js_worker"
}`,
		"index.js": `module.exports = {};`,
	})
	if err != nil {
		t.Fatalf("buildZip returned error: %v", err)
	}

	if err := svc.Publish(ctx, Request{
		Kind:        "plugin_package",
		Channel:     "stable",
		ArtifactZip: payload,
		Metadata: Metadata{
			Title:   "Demo Status",
			Summary: "Demo plugin package",
		},
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	status, err := svc.RegistryStatus(ctx)
	if err != nil {
		t.Fatalf("RegistryStatus returned error: %v", err)
	}
	if status.Healthy {
		t.Fatalf("expected status to be unhealthy before rebuild, got %#v", status)
	}
	if status.Catalog.Status != "missing" {
		t.Fatalf("expected catalog status missing before rebuild, got %#v", status.Catalog)
	}

	if _, err := svc.RebuildRegistry(ctx); err != nil {
		t.Fatalf("RebuildRegistry returned error: %v", err)
	}

	status, err = svc.RegistryStatus(ctx)
	if err != nil {
		t.Fatalf("RegistryStatus after rebuild returned error: %v", err)
	}
	if !status.Healthy {
		t.Fatalf("expected status to be healthy after rebuild, got %#v", status)
	}
	if status.Catalog.Status != "healthy" || status.Source.Status != "healthy" {
		t.Fatalf("expected source/catalog to be healthy after rebuild, got source=%#v catalog=%#v", status.Source, status.Catalog)
	}
}

func TestRegistryStatusReportsDetailedDriftIssues(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	svc := NewServiceWithOptions(store, signSvc, "official-test", Options{
		SourceID:   "official",
		SourceName: "AuraLogic Official Source",
		BaseURL:    "http://localhost:18080",
	})

	payload, err := buildTestZip(map[string]string{
		"manifest.json": `{
  "kind": "plugin_package",
  "name": "demo-drift",
  "display_name": "Demo Drift",
  "version": "1.0.0",
  "description": "demo package",
  "entry": "index.js",
  "runtime": "js_worker"
}`,
		"index.js": `module.exports = {};`,
	})
	if err != nil {
		t.Fatalf("buildZip returned error: %v", err)
	}

	if err := svc.Publish(ctx, Request{
		Kind:        "plugin_package",
		Channel:     "stable",
		ArtifactZip: payload,
		Metadata: Metadata{
			Title:   "Demo Drift",
			Summary: "Demo plugin package",
		},
	}); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	if _, err := svc.RebuildRegistry(ctx); err != nil {
		t.Fatalf("RebuildRegistry returned error: %v", err)
	}

	catalogPayload, err := store.Read(ctx, "index/catalog.json")
	if err != nil {
		t.Fatalf("Read catalog snapshot returned error: %v", err)
	}
	var catalogSnapshot map[string]any
	if err := json.Unmarshal(catalogPayload, &catalogSnapshot); err != nil {
		t.Fatalf("Unmarshal catalog snapshot returned error: %v", err)
	}
	catalogSnapshot["total"] = 0
	items := catalogSnapshot["items"].([]any)
	items[0].(map[string]any)["latest_version"] = "0.9.0"
	mutatedCatalogPayload, err := json.MarshalIndent(catalogSnapshot, "", "  ")
	if err != nil {
		t.Fatalf("Marshal mutated catalog snapshot returned error: %v", err)
	}
	if err := store.Write(ctx, "index/catalog.json", mutatedCatalogPayload); err != nil {
		t.Fatalf("Write mutated catalog snapshot returned error: %v", err)
	}

	sourcePayload, err := store.Read(ctx, "index/source.json")
	if err != nil {
		t.Fatalf("Read source snapshot returned error: %v", err)
	}
	var sourceSnapshot map[string]any
	if err := json.Unmarshal(sourcePayload, &sourceSnapshot); err != nil {
		t.Fatalf("Unmarshal source snapshot returned error: %v", err)
	}
	sourceSnapshot["name"] = "Unexpected Source"
	capabilities := sourceSnapshot["capabilities"].(map[string]any)
	capabilities["artifact_kinds"] = []string{"payment_package"}
	sourceSnapshot["capabilities"] = capabilities
	mutatedSourcePayload, err := json.MarshalIndent(sourceSnapshot, "", "  ")
	if err != nil {
		t.Fatalf("Marshal mutated source snapshot returned error: %v", err)
	}
	if err := store.Write(ctx, "index/source.json", mutatedSourcePayload); err != nil {
		t.Fatalf("Write mutated source snapshot returned error: %v", err)
	}

	status, err := svc.RegistryStatus(ctx)
	if err != nil {
		t.Fatalf("RegistryStatus returned error: %v", err)
	}
	if status.Healthy {
		t.Fatalf("expected unhealthy status after drift injection, got %#v", status)
	}
	if status.Catalog.Status != "stale" || status.Source.Status != "stale" {
		t.Fatalf("expected stale source/catalog status, got source=%#v catalog=%#v", status.Source, status.Catalog)
	}
	if !containsIssue(status.Catalog.Issues, "catalog total drifted: current=0 expected=1") {
		t.Fatalf("expected catalog total drift issue, got %#v", status.Catalog.Issues)
	}
	if !containsIssue(status.Catalog.Issues, "catalog artifact plugin_package:demo-drift drifted: latest_version") {
		t.Fatalf("expected catalog artifact drift issue, got %#v", status.Catalog.Issues)
	}
	if !containsIssue(status.Source.Issues, "source fields drifted:") {
		t.Fatalf("expected source drift issue, got %#v", status.Source.Issues)
	}
	if !containsIssue(status.Source.Issues, "capabilities.artifact_kinds") || !containsIssue(status.Source.Issues, "name") {
		t.Fatalf("expected source drift issue to include changed paths, got %#v", status.Source.Issues)
	}
}

func buildTestZip(files map[string]string) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	keys := make([]string, 0, len(files))
	for name := range files {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		entry, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write([]byte(files[name])); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func containsIssue(issues []string, want string) bool {
	for _, issue := range issues {
		if strings.Contains(issue, want) {
			return true
		}
	}
	return false
}
