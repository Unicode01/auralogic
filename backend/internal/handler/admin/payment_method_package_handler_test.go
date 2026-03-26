package admin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"auralogic/internal/config"
	"auralogic/internal/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestPreviewPaymentMethodPackageRejectsInvalidWebhookManifest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPaymentMethodPackageTestDB(t)
	handler := NewPaymentMethodHandler(db, &config.Config{}, nil)

	manifest := map[string]interface{}{
		"name":         "invalid-payment-preview",
		"display_name": "Invalid Payment Preview",
		"entry":        "index.js",
		"webhooks": []map[string]interface{}{
			{
				"key":       "payment.notify",
				"auth_mode": "header",
			},
		},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest failed: %v", err)
	}

	zipPath := filepath.Join(t.TempDir(), "invalid-payment-preview.zip")
	if err := writeZipFile(zipPath, map[string]string{
		"manifest.json": string(manifestRaw),
		"index.js":      "function onGeneratePaymentCard() { return { html: '<div>ok</div>' } }",
	}); err != nil {
		t.Fatalf("write payment package zip failed: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = newPaymentMethodPackageRequest(t, http.MethodPost, "/api/admin/payment-methods/preview-package", zipPath, nil)

	handler.PreviewPackage(ctx)

	assertPaymentPackageManifestValidationError(t, rec, "webhooks[0].secret_key", `is required when auth_mode is "header"`)
}

func TestUploadPaymentMethodPackageRejectsInvalidWebhookManifest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPaymentMethodPackageTestDB(t)
	handler := NewPaymentMethodHandler(db, &config.Config{}, nil)

	manifest := map[string]interface{}{
		"name":         "invalid-payment-upload",
		"display_name": "Invalid Payment Upload",
		"entry":        "index.js",
		"webhooks": []map[string]interface{}{
			{
				"key":       "payment.notify",
				"auth_mode": "header",
			},
		},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest failed: %v", err)
	}

	zipPath := filepath.Join(t.TempDir(), "invalid-payment-upload.zip")
	if err := writeZipFile(zipPath, map[string]string{
		"manifest.json": string(manifestRaw),
		"index.js":      "function onGeneratePaymentCard() { return { html: '<div>ok</div>' } }",
	}); err != nil {
		t.Fatalf("write payment package zip failed: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = newPaymentMethodPackageRequest(t, http.MethodPost, "/api/admin/payment-methods/upload-package", zipPath, map[string]string{
		"name": "Invalid Upload",
	})

	handler.UploadPackage(ctx)

	assertPaymentPackageManifestValidationError(t, rec, "webhooks[0].secret_key", `is required when auth_mode is "header"`)

	var count int64
	if err := db.Model(&models.PaymentMethod{}).Count(&count).Error; err != nil {
		t.Fatalf("count payment methods failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no payment method to be created, got %d", count)
	}
}

func TestUploadPaymentMethodPackageCreatesVersionSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)

	db := openPaymentMethodPackageTestDB(t)
	handler := NewPaymentMethodHandler(db, &config.Config{}, nil)

	manifest := map[string]interface{}{
		"name":                      "mock-upload-payment",
		"display_name":              "Mock Upload Payment",
		"description":               "Uploaded payment package",
		"icon":                      "CreditCard",
		"runtime":                   "payment_js",
		"entry":                     "index.js",
		"version":                   "1.0.0",
		"manifest_version":          "1.0.0",
		"protocol_version":          "1.0.0",
		"min_host_protocol_version": "1.0.0",
		"max_host_protocol_version": "1.0.0",
		"config_schema": map[string]interface{}{
			"fields": []map[string]interface{}{
				{
					"key":      "checkout_title",
					"type":     "string",
					"default":  "Mock Upload Payment",
					"required": true,
				},
			},
		},
	}
	manifestRaw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest failed: %v", err)
	}

	zipPath := filepath.Join(t.TempDir(), "mock-upload-payment.zip")
	if err := writeZipFile(zipPath, map[string]string{
		"manifest.json": string(manifestRaw),
		"index.js": `
function onGeneratePaymentCard(order, config) {
  return { html: "<div>" + ((config && config.checkout_title) || "Mock Upload Payment") + "</div>" };
}
function onCheckPaymentStatus() {
  return { paid: false };
}
`,
	}); err != nil {
		t.Fatalf("write payment package zip failed: %v", err)
	}

	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = newPaymentMethodPackageRequest(t, http.MethodPost, "/api/admin/payment-methods/upload-package", zipPath, map[string]string{
		"name":   "Mock Upload Payment",
		"config": `{"checkout_title":"Uploaded Checkout"}`,
	})

	handler.UploadPackage(ctx)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var method models.PaymentMethod
	if err := db.Where("name = ?", "Mock Upload Payment").First(&method).Error; err != nil {
		t.Fatalf("query payment method failed: %v", err)
	}
	if strings.TrimSpace(method.PackageName) != "mock-upload-payment.zip" {
		t.Fatalf("expected package_name mock-upload-payment.zip, got %q", method.PackageName)
	}
	if strings.TrimSpace(method.PackageEntry) != "index.js" {
		t.Fatalf("expected package_entry index.js, got %q", method.PackageEntry)
	}
	if strings.TrimSpace(method.PackageChecksum) == "" {
		t.Fatalf("expected package checksum to be populated")
	}
	if strings.TrimSpace(method.Manifest) == "" {
		t.Fatalf("expected manifest to be persisted")
	}

	var versions []models.PaymentMethodVersion
	if err := db.Where("payment_method_id = ?", method.ID).Order("id ASC").Find(&versions).Error; err != nil {
		t.Fatalf("query payment method versions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 payment method version, got %d", len(versions))
	}

	version := versions[0]
	if !version.IsActive {
		t.Fatalf("expected uploaded payment method version to be active")
	}
	if version.PackageName != method.PackageName || version.PackageEntry != method.PackageEntry || version.PackageChecksum != method.PackageChecksum {
		t.Fatalf("expected version snapshot package metadata to match method, method=%+v version=%+v", method, version)
	}
	if version.MarketSourceID != "" || version.MarketArtifactKind != "" || version.MarketArtifactName != "" || version.MarketArtifactVersion != "" {
		t.Fatalf("expected local upload snapshot to have empty market coordinates, got %+v", version)
	}
	if !strings.Contains(version.ConfigSnapshot, "Uploaded Checkout") {
		t.Fatalf("expected version config snapshot to include uploaded config, got %q", version.ConfigSnapshot)
	}
}

func openPaymentMethodPackageTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := openPluginUploadTestDB(t)
	if err := db.AutoMigrate(&models.PaymentMethod{}, &models.PaymentMethodVersion{}, &models.PaymentMethodStorageEntry{}); err != nil {
		t.Fatalf("auto migrate payment method models failed: %v", err)
	}
	return db
}

func newPaymentMethodPackageRequest(
	t *testing.T,
	method string,
	targetURL string,
	filePath string,
	fields map[string]string,
) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}

	src := mustOpenFileForPaymentPackageTest(t, filePath)
	defer func() {
		_ = src.Close()
	}()
	if _, err := io.Copy(part, src); err != nil {
		t.Fatalf("copy file part failed: %v", err)
	}

	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field %s failed: %v", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer failed: %v", err)
	}

	req := httptest.NewRequest(method, targetURL, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func mustOpenFileForPaymentPackageTest(t *testing.T, filePath string) io.ReadCloser {
	t.Helper()

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("open file failed: %v", err)
	}
	return file
}

func assertPaymentPackageManifestValidationError(
	t *testing.T,
	rec *httptest.ResponseRecorder,
	expectedPath string,
	expectedReason string,
) {
	t.Helper()

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d, body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v, body=%s", err, rec.Body.String())
	}
	message := strings.TrimSpace(fmt.Sprint(payload["message"]))
	if !strings.Contains(message, expectedPath) {
		t.Fatalf("expected message to contain %q, got %q", expectedPath, message)
	}
	if !strings.Contains(message, expectedReason) {
		t.Fatalf("expected message to contain %q, got %q", expectedReason, message)
	}

	data, ok := payload["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected response data object, body=%s", rec.Body.String())
	}
	if got := strings.TrimSpace(fmt.Sprint(data["error_key"])); got != "plugin.admin.http_400.invalid_package_manifest_schema" {
		t.Fatalf("expected error_key invalid_package_manifest_schema, got %q", got)
	}
	params, ok := data["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected response data.params object, body=%s", rec.Body.String())
	}
	if got := strings.TrimSpace(fmt.Sprint(params["path"])); got != expectedPath {
		t.Fatalf("expected params.path=%q, got %q", expectedPath, got)
	}
	if got := strings.TrimSpace(fmt.Sprint(params["reason"])); got != expectedReason {
		t.Fatalf("expected params.reason=%q, got %q", expectedReason, got)
	}
}
