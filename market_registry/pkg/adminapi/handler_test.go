package adminapi

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"testing"

	"auralogic/market_registry/pkg/auth"
	"auralogic/market_registry/pkg/publish"
	"auralogic/market_registry/pkg/registrysettings"
	"auralogic/market_registry/pkg/signing"
	"auralogic/market_registry/pkg/storage"
)

func TestSettingsEndpointsMaskSecretsAndPreserveStoredValues(t *testing.T) {
	handler, token, settingsSvc, store := newTestHandlerWithSettings(t)

	_, err := settingsSvc.Update(context.Background(), registrysettings.Document{
		ArtifactStorage: registrysettings.ArtifactStorageSettings{
			DefaultProfileID: "s3-demo",
			Profiles: []registrysettings.ArtifactStorageProfile{
				{
					ID:                "s3-demo",
					Name:              "S3 Demo",
					Type:              "s3",
					S3Endpoint:        "https://s3.example.com",
					S3Region:          "us-east-1",
					S3Bucket:          "bucket-a",
					S3AccessKeyID:     "access-key",
					S3SecretAccessKey: "secret-v1",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Update settings returned error: %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRecorder := httptest.NewRecorder()
	handler.GetSettings(getRecorder, getReq)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected get settings 200, got %d with body %s", getRecorder.Code, getRecorder.Body.String())
	}

	var getPayload map[string]any
	if err := json.Unmarshal(getRecorder.Body.Bytes(), &getPayload); err != nil {
		t.Fatalf("Unmarshal get settings returned error: %v", err)
	}
	data := getPayload["data"].(map[string]any)
	artifactStorage := data["artifact_storage"].(map[string]any)
	profiles := artifactStorage["profiles"].([]any)
	var customProfile map[string]any
	for _, item := range profiles {
		profile := item.(map[string]any)
		if profile["id"] == "s3-demo" {
			customProfile = profile
			break
		}
	}
	if customProfile == nil {
		t.Fatalf("expected custom profile in response, got %#v", profiles)
	}
	if got := customProfile["s3_secret_access_key"]; got != nil && got != "" {
		t.Fatalf("expected masked secret in get response, got %#v", got)
	}
	if got := customProfile["has_s3_secret_access_key"]; got != true {
		t.Fatalf("expected has_s3_secret_access_key true, got %#v", got)
	}

	updateBody := bytes.NewBufferString(`{
  "artifact_storage": {
    "default_profile_id": "s3-demo",
    "profiles": [
      {
        "id": "s3-demo",
        "original_id": "s3-demo",
        "name": "S3 Demo",
        "type": "s3",
        "s3_endpoint": "https://s3.example.com",
        "s3_region": "us-east-1",
        "s3_bucket": "bucket-a",
        "s3_access_key_id": "access-key"
      }
    ]
  }
}`)
	updateReq := httptest.NewRequest(http.MethodPut, "/admin/settings", updateBody)
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRecorder := httptest.NewRecorder()
	handler.UpdateSettings(updateRecorder, updateReq)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("expected update settings 200, got %d with body %s", updateRecorder.Code, updateRecorder.Body.String())
	}

	raw, err := store.Read(context.Background(), registrysettings.SettingsPath)
	if err != nil {
		t.Fatalf("Read settings file returned error: %v", err)
	}
	var persisted registrysettings.Document
	if err := json.Unmarshal(raw, &persisted); err != nil {
		t.Fatalf("Unmarshal persisted settings returned error: %v", err)
	}
	if len(persisted.ArtifactStorage.Profiles) != 1 {
		t.Fatalf("expected one persisted profile, got %#v", persisted.ArtifactStorage.Profiles)
	}
	if got := persisted.ArtifactStorage.Profiles[0].S3SecretAccessKey; got != "secret-v1" {
		t.Fatalf("expected secret to be preserved, got %#v", got)
	}
}

func TestPublishRejectsOversizedArtifact(t *testing.T) {
	handler, token := newTestHandler(t)

	previousLimit := maxArtifactUploadSize
	maxArtifactUploadSize = 64
	t.Cleanup(func() {
		maxArtifactUploadSize = previousLimit
	})

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("kind", "plugin_package"); err != nil {
		t.Fatalf("WriteField kind returned error: %v", err)
	}
	fileWriter, err := writer.CreateFormFile("artifact", "too-large.zip")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write(bytes.Repeat([]byte("a"), 80)); err != nil {
		t.Fatalf("Write artifact payload returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/publish", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	handler.Publish(recorder, req)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal error payload returned error: %v", err)
	}
	errorPayload := payload["error"].(map[string]any)
	if got := errorPayload["code"]; got != "artifact_too_large" {
		t.Fatalf("expected artifact_too_large code, got %#v", got)
	}
}

func TestPublishAllowsEmptyMetadata(t *testing.T) {
	handler, token := newTestHandler(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("kind", "plugin_package"); err != nil {
		t.Fatalf("WriteField kind returned error: %v", err)
	}
	if err := writer.WriteField("channel", "stable"); err != nil {
		t.Fatalf("WriteField channel returned error: %v", err)
	}
	fileWriter, err := writer.CreateFormFile("artifact", "demo-plugin.zip")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write(buildTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "empty-metadata-demo",
  "display_name": "Empty Metadata Demo",
  "version": "1.0.0",
  "description": "demo plugin",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})); err != nil {
		t.Fatalf("Write artifact payload returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/publish", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	handler.Publish(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestPublishRejectsUnsafeManifestIdentity(t *testing.T) {
	handler, token := newTestHandler(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("channel", "stable"); err != nil {
		t.Fatalf("WriteField channel returned error: %v", err)
	}
	fileWriter, err := writer.CreateFormFile("artifact", "unsafe-plugin.zip")
	if err != nil {
		t.Fatalf("CreateFormFile returned error: %v", err)
	}
	if _, err := fileWriter.Write(buildTestZip(t, map[string]string{
		"manifest.json": `{
  "kind": "plugin_package",
  "name": "../unsafe-demo",
  "version": "1.0.0",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})); err != nil {
		t.Fatalf("Write artifact payload returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close multipart writer returned error: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/publish", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	handler.Publish(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal error payload returned error: %v", err)
	}
	errorPayload := payload["error"].(map[string]any)
	if got := errorPayload["code"]; got != "invalid_publish_request" {
		t.Fatalf("expected invalid_publish_request code, got %#v", got)
	}
	if got := errorPayload["message"]; got != "name contains forbidden path characters" {
		t.Fatalf("expected unsafe name error, got %#v", got)
	}
}

func TestArtifactEndpointsExposeMergedChannelsForSameVersion(t *testing.T) {
	handler, token := newTestHandler(t)
	payload := buildTestZip(t, map[string]string{
		"manifest.json": `{
  "name": "channel-demo",
  "display_name": "Channel Demo",
  "version": "1.0.0",
  "description": "demo plugin",
  "runtime": "js_worker",
  "entry": "index.js"
}`,
		"index.js": `module.exports = {};`,
	})

	for _, channel := range []string{"alpha", "stable", "beta"} {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		if err := writer.WriteField("kind", "plugin_package"); err != nil {
			t.Fatalf("WriteField kind returned error: %v", err)
		}
		if err := writer.WriteField("channel", channel); err != nil {
			t.Fatalf("WriteField channel returned error: %v", err)
		}
		fileWriter, err := writer.CreateFormFile("artifact", "channel-demo.zip")
		if err != nil {
			t.Fatalf("CreateFormFile returned error: %v", err)
		}
		if _, err := fileWriter.Write(payload); err != nil {
			t.Fatalf("Write artifact payload returned error: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("Close multipart writer returned error: %v", err)
		}

		req := httptest.NewRequest(http.MethodPost, "/admin/publish", body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		recorder := httptest.NewRecorder()
		handler.Publish(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("expected publish %s to succeed, got %d with body %s", channel, recorder.Code, recorder.Body.String())
		}
	}

	listReq := httptest.NewRequest(http.MethodGet, "/admin/artifacts", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRecorder := httptest.NewRecorder()
	handler.ListArtifacts(listRecorder, listReq)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected list artifacts to succeed, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}

	var listPayload map[string]any
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listPayload); err != nil {
		t.Fatalf("Unmarshal list response returned error: %v", err)
	}
	data := listPayload["data"].(map[string]any)
	kindData := data["plugin_package"].(map[string]any)
	artifact := kindData["channel-demo"].(map[string]any)
	artifactChannels := artifact["channels"].([]any)
	if len(artifactChannels) != 3 {
		t.Fatalf("expected artifact summary to expose three channels, got %#v", artifact["channels"])
	}

	releaseReq := httptest.NewRequest(http.MethodGet, "/admin/artifacts/plugin_package/channel-demo/1.0.0", nil)
	releaseReq.Header.Set("Authorization", "Bearer "+token)
	releaseRecorder := httptest.NewRecorder()
	handler.GetArtifactRelease(releaseRecorder, releaseReq)
	if releaseRecorder.Code != http.StatusOK {
		t.Fatalf("expected get artifact release to succeed, got %d with body %s", releaseRecorder.Code, releaseRecorder.Body.String())
	}

	var releasePayload map[string]any
	if err := json.Unmarshal(releaseRecorder.Body.Bytes(), &releasePayload); err != nil {
		t.Fatalf("Unmarshal release response returned error: %v", err)
	}
	releaseData := releasePayload["data"].(map[string]any)
	releaseChannels := releaseData["channels"].([]any)
	if len(releaseChannels) != 3 {
		t.Fatalf("expected release detail to expose three channels, got %#v", releaseData["channels"])
	}
}

func newTestHandler(t *testing.T) (*Handler, string) {
	handler, token, _, _ := newTestHandlerWithSettings(t)
	return handler, token
}

func newTestHandlerWithSettings(t *testing.T) (*Handler, string, *registrysettings.Service, storage.Storage) {
	t.Helper()

	root := t.TempDir()
	store, err := storage.NewLocalStorage(filepath.Join(root, "data"), "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}
	signSvc := signing.NewService(filepath.Join(root, "keys"))
	if _, err := signSvc.GenerateKeyPair("official-test"); err != nil {
		t.Fatalf("GenerateKeyPair returned error: %v", err)
	}
	authSvc := auth.NewServiceWithConfig(auth.Config{
		AdminUsername: "admin",
		AdminPassword: "password",
	})
	settingsSvc := registrysettings.NewService(store, registrysettings.ArtifactStorageProfile{
		ID:      registrysettings.CanonicalArtifactStorageProfileID,
		Name:    "Canonical Storage",
		Type:    "local",
		BaseDir: filepath.Join(root, "data"),
		BaseURL: "http://localhost:18080",
	})
	pubSvc := publish.NewServiceWithOptions(store, signSvc, "official-test", publish.Options{
		Settings: settingsSvc,
	})
	token, err := authSvc.Login("admin", "password")
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	return NewHandlerWithOptions(authSvc, pubSvc, store, signSvc, Options{
		Settings: settingsSvc,
	}), token, settingsSvc, store
}

func buildTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	keys := make([]string, 0, len(files))
	for name := range files {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	for _, name := range keys {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("Create zip entry %s returned error: %v", name, err)
		}
		if _, err := entry.Write([]byte(files[name])); err != nil {
			t.Fatalf("Write zip entry %s returned error: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close zip writer returned error: %v", err)
	}
	return body.Bytes()
}
