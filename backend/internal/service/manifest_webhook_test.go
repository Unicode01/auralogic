package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestParseDeclaredWebhookManifestsNormalizesDefaults(t *testing.T) {
	items, err := ParseDeclaredWebhookManifests(`{
		"webhooks": [
			{"key":"payment.notify","auth_mode":"query","secret_key":"token_secret"},
			{"key":"payment.return","method":"any","auth_mode":"header","secret_key":"header_secret"},
			{"key":"payment.sign","auth_mode":"hmac_sha256","secret_key":"signature_secret"}
		]
	}`)
	if err != nil {
		t.Fatalf("ParseDeclaredWebhookManifests returned error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 webhook items, got %d", len(items))
	}

	if items[0].Method != "POST" {
		t.Fatalf("expected default method POST, got %q", items[0].Method)
	}
	if items[0].Action != "webhook.payment.notify" {
		t.Fatalf("expected default action webhook.payment.notify, got %q", items[0].Action)
	}
	if items[0].QueryParam != "token" {
		t.Fatalf("expected default query param token, got %q", items[0].QueryParam)
	}
	if items[1].Method != "*" {
		t.Fatalf("expected any method to normalize to *, got %q", items[1].Method)
	}
	if items[1].Header != "X-Plugin-Webhook-Token" {
		t.Fatalf("expected default header token name, got %q", items[1].Header)
	}
	if items[2].SignatureHeader != "X-Plugin-Webhook-Signature" {
		t.Fatalf("expected default signature header, got %q", items[2].SignatureHeader)
	}
}

func TestAuthenticateDeclaredWebhookRequestQueryAndHeader(t *testing.T) {
	queryWebhook, err := NormalizeDeclaredWebhookManifest(DeclaredWebhookManifest{
		Key:        "payment.query",
		AuthMode:   "query",
		SecretKey:  "token_secret",
		QueryParam: "token",
	})
	if err != nil {
		t.Fatalf("NormalizeDeclaredWebhookManifest query returned error: %v", err)
	}
	headerWebhook, err := NormalizeDeclaredWebhookManifest(DeclaredWebhookManifest{
		Key:       "payment.header",
		AuthMode:  "header",
		SecretKey: "header_secret",
		Header:    "X-Test-Token",
	})
	if err != nil {
		t.Fatalf("NormalizeDeclaredWebhookManifest header returned error: %v", err)
	}

	secrets := map[string]string{
		"token_secret":  "query-demo",
		"header_secret": "header-demo",
	}
	if err := AuthenticateDeclaredWebhookRequest(
		queryWebhook,
		map[string]string{"token": "query-demo"},
		map[string]string{},
		nil,
		secrets,
	); err != nil {
		t.Fatalf("expected query auth success, got %v", err)
	}
	if err := AuthenticateDeclaredWebhookRequest(
		headerWebhook,
		map[string]string{},
		map[string]string{"x-test-token": "header-demo"},
		nil,
		secrets,
	); err != nil {
		t.Fatalf("expected header auth success, got %v", err)
	}
}

func TestAuthenticateDeclaredWebhookRequestHMACSHA256(t *testing.T) {
	webhook, err := NormalizeDeclaredWebhookManifest(DeclaredWebhookManifest{
		Key:             "payment.sign",
		AuthMode:        "hmac_sha256",
		SecretKey:       "signature_secret",
		SignatureHeader: "X-Test-Signature",
	})
	if err != nil {
		t.Fatalf("NormalizeDeclaredWebhookManifest returned error: %v", err)
	}

	body := []byte(`{"status":"paid"}`)
	mac := hmac.New(sha256.New, []byte("sign-demo"))
	_, _ = mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	err = AuthenticateDeclaredWebhookRequest(
		webhook,
		map[string]string{},
		map[string]string{"x-test-signature": "sha256=" + signature},
		body,
		map[string]string{"signature_secret": "sign-demo"},
	)
	if err != nil {
		t.Fatalf("expected hmac auth success, got %v", err)
	}
}

func TestBuildWebhookSecretsFromConfig(t *testing.T) {
	secrets, err := BuildWebhookSecretsFromConfig(`{"token_secret":"abc","retry":3,"enabled":true}`)
	if err != nil {
		t.Fatalf("BuildWebhookSecretsFromConfig returned error: %v", err)
	}
	if secrets["token_secret"] != "abc" {
		t.Fatalf("expected token_secret=abc, got %q", secrets["token_secret"])
	}
	if secrets["retry"] != "3" {
		t.Fatalf("expected retry=3, got %q", secrets["retry"])
	}
	if secrets["enabled"] != "true" {
		t.Fatalf("expected enabled=true, got %q", secrets["enabled"])
	}
}
