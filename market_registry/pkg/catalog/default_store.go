package catalog

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"auralogic/market_registry/pkg/artifactorigin"
)

const hostBridgeVersion = "1.0.0"

type Publisher struct {
	ID   string
	Name string
}

type Release struct {
	Kind          string
	Name          string
	Version       string
	Channel       string
	Title         string
	Summary       string
	Description   string
	ReleaseNotes  string
	PublishedAt   time.Time
	Governance    map[string]any
	Compatibility map[string]any
	Permissions   map[string]any
	Install       map[string]any
	Targets       map[string]any
	Template      map[string]any
	Docs          map[string]any
	ContentType   string
	Artifact      []byte
}

type Artifact struct {
	Kind        string
	Name        string
	Title       string
	Summary     string
	Description string
	Publisher   Publisher
	Labels      []string
	Versions    []*Release
}

type Store struct {
	SourceID   string
	SourceName string
	Artifacts  map[string]*Artifact
}

type Query struct {
	Kind    string
	Channel string
	Search  string
	Runtime string
	Offset  int
	Limit   int
}

func NewDefaultStore() (*Store, error) {
	now := time.Date(2026, 3, 13, 18, 0, 0, 0, time.UTC)
	pluginZip, err := buildZip(map[string]string{
		"manifest.json": `{
  "name": "hello-market",
  "display_name": "Hello Market",
  "description": "Sample plugin package served by the market registry API.",
  "type": "custom",
  "runtime": "js_worker",
  "entry": "index.js",
  "version": "0.1.0",
  "manifest_version": "1.0.0",
  "protocol_version": "1.0.0",
  "min_host_protocol_version": "1.0.0",
  "max_host_protocol_version": "1.0.0",
  "capabilities": {
    "hooks": [],
    "disabled_hooks": [],
    "requested_permissions": [],
    "granted_permissions": []
  }
}`,
		"index.js": `
module.exports.health = function health() {
  return { healthy: true, version: "hello-market/0.1.0" };
};
module.exports.execute = function execute(action, params) {
  return {
    success: true,
    data: {
      source: "market-registry-api",
      action: String(action || ""),
      params: params || {}
    }
  };
};
`,
	})
	if err != nil {
		return nil, err
	}

	paymentZip, err := buildZip(map[string]string{
		"manifest.json": `{
  "name": "mock-checkout",
  "display_name": "Mock Checkout",
  "description": "Sample PaymentJS package served by the market registry API.",
  "icon": "CreditCard",
  "entry": "index.js",
  "version": "1.0.0",
  "poll_interval": 30,
  "manifest_version": "1.0.0",
  "protocol_version": "1.0.0",
  "min_host_protocol_version": "1.0.0",
  "max_host_protocol_version": "1.0.0",
  "config_schema": {
    "title": "Mock Checkout Config",
    "fields": [
      {
        "key": "checkout_title",
        "label": "Checkout Title",
        "type": "string",
        "default": "Mock Checkout",
        "required": true
      }
    ]
  },
  "webhooks": [
    {
      "key": "payment.notify",
      "method": "POST",
      "auth_mode": "none"
    }
  ]
}`,
		"index.js": `
function onGeneratePaymentCard(order, config) {
  var title = (config && config.checkout_title) || "Mock Checkout";
  return {
    title: title,
    html: '<div style="padding:16px;border:1px solid #dbe2ea;border-radius:12px;font-family:Arial,sans-serif;">' +
      '<h3 style="margin:0 0 8px;">' + title + '</h3>' +
      '<p style="margin:0;color:#475569;">Order ' + (order.order_no || order.id || '-') + ' is waiting for payment.</p>' +
    '</div>'
  };
}

function onCheckPaymentStatus() {
  return { paid: false, message: "Pending manual confirmation" };
}

function onWebhook() {
  return { ack_status: 200, ack_body: "ok" };
}
`,
	})
	if err != nil {
		return nil, err
	}

	emailZip, err := buildZip(map[string]string{
		"manifest.json": `{
  "kind": "email_template",
  "name": "order_paid",
  "title": "Order Paid Email",
  "version": "1.0.0",
  "event": "order_paid",
  "engine": "go_template",
  "content_file": "template.html"
}`,
		"template.html": orderPaidTemplateHTML,
	})
	if err != nil {
		return nil, err
	}

	landingZip, err := buildZip(map[string]string{
		"manifest.json": `{
  "kind": "landing_page_template",
  "name": "home",
  "title": "Storefront Home",
  "version": "1.0.0",
  "engine": "go_template",
  "content_file": "landing.html"
}`,
		"landing.html": landingPageTemplateHTML,
	})
	if err != nil {
		return nil, err
	}

	publisher := Publisher{ID: "auralogic", Name: "AuraLogic"}
	pluginArtifact := &Artifact{
		Kind:        "plugin_package",
		Name:        "hello-market",
		Title:       "Hello Market",
		Summary:     "Minimal JS Worker plugin package published by the source server MVP.",
		Description: "Used to verify host.market.install.execute against the market registry sample source.",
		Publisher:   publisher,
		Labels:      []string{"official", "sample"},
		Versions: []*Release{
			{
				Kind:         "plugin_package",
				Name:         "hello-market",
				Version:      "0.1.0",
				Channel:      "stable",
				Title:        "Hello Market",
				Summary:      "Minimal JS Worker plugin package published by the source server MVP.",
				Description:  "Installs a tiny JS Worker plugin that echoes execute payloads.",
				ReleaseNotes: "Initial sample release published by the market registry API.",
				PublishedAt:  now,
				Governance: map[string]any{
					"mode":              "host_managed",
					"install_strategy":  "host_bridge",
					"supports_rollback": true,
				},
				Compatibility: map[string]any{
					"min_host_version":          "1.0.0",
					"runtime":                   "js_worker",
					"manifest_version":          "1.0.0",
					"protocol_version":          "1.0.0",
					"min_host_protocol_version": "1.0.0",
					"min_host_bridge_version":   hostBridgeVersion,
				},
				Permissions: map[string]any{
					"requested":                     []string{},
					"default_granted":               []string{},
					"requires_reconfirm_on_upgrade": false,
				},
				Install: map[string]any{
					"package_format":         "zip",
					"entry":                  "index.js",
					"requires_host_download": true,
					"auto_activate_default":  true,
					"auto_start_default":     false,
				},
				Docs: map[string]any{
					"docs_url":    "https://d.auralogic.org/docs/hello-market",
					"support_url": "https://d.auralogic.org/support",
				},
				ContentType: "application/zip",
				Artifact:    pluginZip,
			},
		},
	}

	paymentArtifact := &Artifact{
		Kind:        "payment_package",
		Name:        "mock-checkout",
		Title:       "Mock Checkout",
		Summary:     "Sample PaymentJS package published by the source server MVP.",
		Description: "Used to verify market-powered payment package preview and import flows.",
		Publisher:   publisher,
		Labels:      []string{"official", "payment"},
		Versions: []*Release{
			{
				Kind:         "payment_package",
				Name:         "mock-checkout",
				Version:      "1.0.0",
				Channel:      "stable",
				Title:        "Mock Checkout",
				Summary:      "Sample PaymentJS package with a basic payment card and webhook manifest.",
				Description:  "Can be imported by the market flow into native PaymentJS payment methods.",
				ReleaseNotes: "Initial payment package sample release.",
				PublishedAt:  now,
				Governance: map[string]any{
					"mode":              "host_managed",
					"install_strategy":  "host_bridge",
					"supports_rollback": false,
				},
				Compatibility: map[string]any{
					"min_host_version":        "1.0.0",
					"min_host_bridge_version": hostBridgeVersion,
				},
				Install: map[string]any{
					"package_format":         "zip",
					"entry":                  "index.js",
					"requires_host_download": true,
				},
				Targets: map[string]any{
					"target": "payment_method",
				},
				Docs: map[string]any{
					"docs_url":    "https://d.auralogic.org/docs/mock-checkout",
					"support_url": "https://d.auralogic.org/support",
				},
				ContentType: "application/zip",
				Artifact:    paymentZip,
			},
		},
	}

	emailArtifact := &Artifact{
		Kind:        "email_template",
		Name:        "order_paid",
		Title:       "Order Paid Email",
		Summary:     "Responsive order-paid email template with inline content for plugin import.",
		Description: "Publishes a ready-to-import `order_paid` email template.",
		Publisher:   publisher,
		Labels:      []string{"official", "email"},
		Versions: []*Release{
			{
				Kind:         "email_template",
				Name:         "order_paid",
				Version:      "1.0.0",
				Channel:      "stable",
				Title:        "Order Paid Email",
				Summary:      "Responsive order-paid email template.",
				Description:  "Can be imported by the official market plugin through host-managed template install APIs.",
				ReleaseNotes: "Initial email template sample release.",
				PublishedAt:  now,
				Governance: map[string]any{
					"mode":              "host_managed",
					"install_strategy":  "host_template_install",
					"supports_rollback": true,
				},
				Compatibility: map[string]any{
					"min_host_version":        "1.0.0",
					"min_host_bridge_version": hostBridgeVersion,
					"engine":                  "go_template",
				},
				Install: map[string]any{
					"package_format":         "zip",
					"requires_host_download": false,
					"inline_content":         true,
					"save_mode":              "replace",
				},
				Targets: map[string]any{
					"event":  "order_paid",
					"key":    "order_paid",
					"engine": "go_template",
				},
				Template: map[string]any{
					"key":      "order_paid",
					"filename": "order_paid.html",
					"subject":  "Your order {{.OrderNo}} has been paid",
					"content":  orderPaidTemplateHTML,
				},
				ContentType: "application/zip",
				Artifact:    emailZip,
			},
		},
	}

	landingArtifact := &Artifact{
		Kind:        "landing_page_template",
		Name:        "home",
		Title:       "Storefront Home",
		Summary:     "Simple home landing page with hero section and product grid placeholder.",
		Description: "Publishes the default `home` landing page template for plugin-managed import.",
		Publisher:   publisher,
		Labels:      []string{"official", "landing"},
		Versions: []*Release{
			{
				Kind:         "landing_page_template",
				Name:         "home",
				Version:      "1.0.0",
				Channel:      "stable",
				Title:        "Storefront Home",
				Summary:      "Simple home landing page template.",
				Description:  "Can be imported by the official market plugin through host-managed template install APIs.",
				ReleaseNotes: "Initial landing page sample release.",
				PublishedAt:  now,
				Governance: map[string]any{
					"mode":              "host_managed",
					"install_strategy":  "host_template_install",
					"supports_rollback": true,
				},
				Compatibility: map[string]any{
					"min_host_version":        "1.0.0",
					"min_host_bridge_version": hostBridgeVersion,
					"engine":                  "go_template",
				},
				Install: map[string]any{
					"package_format":         "zip",
					"requires_host_download": false,
					"inline_content":         true,
					"save_mode":              "replace",
				},
				Targets: map[string]any{
					"page_key": "home",
					"slug":     "home",
					"engine":   "go_template",
				},
				Template: map[string]any{
					"page_key": "home",
					"slug":     "home",
					"content":  landingPageTemplateHTML,
				},
				ContentType: "application/zip",
				Artifact:    landingZip,
			},
		},
	}

	artifacts := map[string]*Artifact{
		storeKey(pluginArtifact.Kind, pluginArtifact.Name):   pluginArtifact,
		storeKey(paymentArtifact.Kind, paymentArtifact.Name): paymentArtifact,
		storeKey(emailArtifact.Kind, emailArtifact.Name):     emailArtifact,
		storeKey(landingArtifact.Kind, landingArtifact.Name): landingArtifact,
	}
	return &Store{
		SourceID:   "official",
		SourceName: "AuraLogic Official Source",
		Artifacts:  artifacts,
	}, nil
}

func (s *Store) BuildSourceDocument(_ context.Context, baseURL string) (map[string]any, error) {
	return map[string]any{
		"api_version":  "v1",
		"source_id":    s.SourceID,
		"name":         s.SourceName,
		"base_url":     baseURL,
		"generated_at": time.Now().UTC(),
		"capabilities": map[string]any{
			"artifact_kinds": []string{
				"plugin_package",
				"payment_package",
				"email_template",
				"landing_page_template",
			},
			"governance_modes": []string{
				"host_managed",
			},
			"supports_delta":     false,
			"supports_signature": false,
		},
		"compatibility": map[string]any{
			"min_host_version":        "1.0.0",
			"min_host_bridge_version": hostBridgeVersion,
		},
	}, nil
}

func (s *Store) ListCatalog(_ context.Context, query Query, baseURL string) ([]map[string]any, int, error) {
	items := make([]map[string]any, 0, len(s.Artifacts))
	for _, artifact := range s.sortedArtifacts() {
		release := latestRelease(artifact)
		if release == nil {
			continue
		}
		if query.Kind != "" && !strings.EqualFold(artifact.Kind, query.Kind) {
			continue
		}
		versionChannels := collectReleaseChannelsForVersion(artifact.Versions, release.Version)
		if query.Channel != "" && !ArtifactVersionMatchesChannel(map[string]any{
			"channel":  release.Channel,
			"channels": versionChannels,
		}, query.Channel) {
			continue
		}
		if query.Runtime != "" && !strings.EqualFold(stringValue(release.Compatibility["runtime"]), query.Runtime) {
			continue
		}
		searchable := []string{
			artifact.Kind,
			artifact.Name,
			artifact.Title,
			artifact.Summary,
		}
		searchable = append(searchable, versionChannels...)
		if query.Search != "" && !matchesCatalogSearch(searchable, query.Search) {
			continue
		}
		download := releaseDownloadDocument(release, baseURL)
		items = append(items, map[string]any{
			"kind":           artifact.Kind,
			"name":           artifact.Name,
			"title":          artifact.Title,
			"summary":        artifact.Summary,
			"latest_version": release.Version,
			"channel":        release.Channel,
			"channels":       versionChannels,
			"publisher": map[string]any{
				"id":   artifact.Publisher.ID,
				"name": artifact.Publisher.Name,
			},
			"governance":    cloneMap(release.Governance),
			"compatibility": cloneMap(release.Compatibility),
			"permissions":   cloneMap(release.Permissions),
			"download": map[string]any{
				"size":   download["size"],
				"sha256": download["sha256"],
			},
			"labels":       append([]string(nil), artifact.Labels...),
			"published_at": release.PublishedAt.UTC().Format(time.RFC3339),
		})
	}
	total := len(items)
	offset := maxInt(query.Offset, 0)
	limit := query.Limit
	if limit <= 0 {
		limit = 20
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return items[offset:end], total, nil
}

func (s *Store) GetArtifact(kind, name string) (*Artifact, bool) {
	artifact, ok := s.Artifacts[storeKey(kind, name)]
	return artifact, ok
}

func (s *Store) BuildArtifactDocument(_ context.Context, kind, name string) (map[string]any, error) {
	artifact, ok := s.GetArtifact(kind, name)
	if !ok {
		return nil, ErrNotFound
	}
	versions := make([]map[string]any, 0, len(artifact.Versions))
	for _, release := range artifact.Versions {
		channels := collectReleaseChannelsForVersion(artifact.Versions, release.Version)
		versions = append(versions, map[string]any{
			"version":      release.Version,
			"channel":      firstNonEmpty(release.Channel, ArtifactVersionPrimaryChannel(map[string]any{"channels": channels})),
			"channels":     channels,
			"published_at": release.PublishedAt.UTC().Format(time.RFC3339),
		})
	}
	versions = normalizeArtifactVersionList(versions)
	return map[string]any{
		"kind":           artifact.Kind,
		"name":           artifact.Name,
		"title":          artifact.Title,
		"summary":        artifact.Summary,
		"description":    artifact.Description,
		"latest_version": latestRelease(artifact).Version,
		"channels":       collectChannels(artifact.Versions),
		"governance":     cloneMap(latestRelease(artifact).Governance),
		"versions":       versions,
	}, nil
}

func (s *Store) BuildReleaseDocument(_ context.Context, kind, name, version, baseURL string) (map[string]any, error) {
	artifact, ok := s.GetArtifact(kind, name)
	if !ok {
		return nil, ErrNotFound
	}
	for _, release := range artifact.Versions {
		if !strings.EqualFold(release.Version, version) {
			continue
		}
		download := releaseDownloadDocument(release, baseURL)
		channels := collectReleaseChannelsForVersion(artifact.Versions, release.Version)
		document := map[string]any{
			"artifact_id":   fmt.Sprintf("%s:%s", artifact.Kind, artifact.Name),
			"kind":          artifact.Kind,
			"name":          artifact.Name,
			"version":       release.Version,
			"channel":       firstNonEmpty(release.Channel, ArtifactVersionPrimaryChannel(map[string]any{"channels": channels})),
			"channels":      channels,
			"title":         release.Title,
			"summary":       release.Summary,
			"description":   release.Description,
			"release_notes": release.ReleaseNotes,
			"published_at":  release.PublishedAt.UTC().Format(time.RFC3339),
			"governance":    cloneMap(release.Governance),
			"download":      download,
			"compatibility": cloneMap(release.Compatibility),
			"install":       cloneMap(release.Install),
			"permissions":   cloneMap(release.Permissions),
			"targets":       cloneMap(release.Targets),
			"template":      cloneMap(release.Template),
			"docs":          cloneMap(release.Docs),
		}
		return document, nil
	}
	return nil, ErrNotFound
}

func (s *Store) ReadReleaseArtifact(_ context.Context, kind string, name string, version string) ([]byte, string, error) {
	artifact, ok := s.GetArtifact(kind, name)
	if !ok {
		return nil, "", ErrNotFound
	}
	for _, release := range artifact.Versions {
		if !strings.EqualFold(release.Version, version) {
			continue
		}
		return append([]byte(nil), release.Artifact...), release.ContentType, nil
	}
	return nil, "", ErrNotFound
}

func (s *Store) sortedArtifacts() []*Artifact {
	items := make([]*Artifact, 0, len(s.Artifacts))
	for _, artifact := range s.Artifacts {
		items = append(items, artifact)
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left.Kind == right.Kind {
			return left.Name < right.Name
		}
		return left.Kind < right.Kind
	})
	return items
}

func latestRelease(artifact *Artifact) *Release {
	if artifact == nil || len(artifact.Versions) == 0 {
		return nil
	}
	return artifact.Versions[0]
}

func collectChannels(releases []*Release) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(releases))
	for _, release := range releases {
		if release == nil || release.Channel == "" {
			continue
		}
		if _, ok := seen[release.Channel]; ok {
			continue
		}
		seen[release.Channel] = struct{}{}
		out = append(out, release.Channel)
	}
	return out
}

func collectReleaseChannelsForVersion(releases []*Release, version string) []string {
	channels := make([]string, 0, len(releases))
	for _, release := range releases {
		if release == nil || !strings.EqualFold(release.Version, version) {
			continue
		}
		channels = append(channels, strings.TrimSpace(release.Channel))
	}
	return dedupeStringValues(channels...)
}

func releaseDownloadDocument(release *Release, baseURL string) map[string]any {
	size, sha := releaseDigest(release.Artifact)
	return map[string]any{
		"url":          fmt.Sprintf("%s/v1/artifacts/%s/%s/releases/%s/download", strings.TrimRight(baseURL, "/"), release.Kind, release.Name, release.Version),
		"filename":     releaseDownloadFilename(release.Name, release.Version, release.ContentType),
		"size":         size,
		"content_type": release.ContentType,
		"sha256":       sha,
		"transport":    artifactorigin.DefaultLocalMirrorTransport(),
	}
}

func releaseDownloadFilename(name string, version string, contentType string) string {
	filename := strings.TrimSpace(name)
	if filename == "" {
		filename = "artifact"
	}
	if trimmedVersion := strings.TrimSpace(version); trimmedVersion != "" {
		filename += "-" + trimmedVersion
	}
	return filename + releaseDownloadFileExtension(contentType)
}

func releaseDownloadFileExtension(contentType string) string {
	value := strings.ToLower(strings.TrimSpace(contentType))
	switch value {
	case "application/zip":
		return ".zip"
	case "application/gzip":
		return ".gz"
	case "application/json":
		return ".json"
	case "text/plain":
		return ".txt"
	default:
		return ".bin"
	}
}

func releaseDigest(payload []byte) (int, string) {
	sum := sha256.Sum256(payload)
	return len(payload), hex.EncodeToString(sum[:])
}

func buildZip(files map[string]string) ([]byte, error) {
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

func cloneMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(value))
	for key, item := range value {
		out[key] = item
	}
	return out
}

func storeKey(kind, name string) string {
	return strings.ToLower(strings.TrimSpace(kind)) + ":" + strings.ToLower(strings.TrimSpace(name))
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

const orderPaidTemplateHTML = `<!doctype html>
<html>
  <body style="font-family: Arial, sans-serif; background: #f5f7fb; padding: 24px;">
    <table style="max-width: 640px; margin: 0 auto; background: #ffffff; border-radius: 16px; padding: 32px;">
      <tr><td>
        <h1 style="margin: 0 0 16px;">Payment confirmed</h1>
        <p style="color: #475569;">Hi {{.UserName}}, your order <strong>{{.OrderNo}}</strong> has been paid successfully.</p>
        <p style="color: #475569;">Amount: <strong>{{.OrderAmount}}</strong></p>
        <p style="color: #475569;">You can now return to {{.AppName}} to track fulfillment.</p>
      </td></tr>
    </table>
  </body>
</html>`

const landingPageTemplateHTML = `<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <title>{{.AppName}}</title>
  </head>
  <body style="margin:0;font-family:Arial,sans-serif;background:#f8fafc;color:#0f172a;">
    <section style="padding:72px 24px;background:linear-gradient(135deg,#0f172a,#1d4ed8);color:#fff;">
      <div style="max-width:960px;margin:0 auto;">
        <p style="letter-spacing:.18em;text-transform:uppercase;opacity:.8;">AuraLogic Market</p>
        <h1 style="font-size:48px;line-height:1.05;margin:16px 0;">A cleaner storefront home page</h1>
        <p style="max-width:640px;font-size:18px;line-height:1.6;opacity:.9;">Use this landing page as a starting point for campaigns, limited launches, or product-focused storefront experiences.</p>
      </div>
    </section>
    <section style="max-width:960px;margin:0 auto;padding:48px 24px;">
      <h2 style="margin:0 0 20px;">Featured products</h2>
      <p style="color:#475569;">Render your featured catalog here using the existing landing page variables.</p>
    </section>
  </body>
</html>`
