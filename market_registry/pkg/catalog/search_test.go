package catalog

import "testing"

func TestMatchesCatalogSearchUsesTokenMatching(t *testing.T) {
	if matchesCatalogSearch([]string{"alpine market plugin"}, "alpha") {
		t.Fatal("expected alpha search not to match alpine")
	}
	if !matchesCatalogSearch([]string{"alpha market plugin"}, "alpha") {
		t.Fatal("expected alpha search to match alpha token")
	}
	if !matchesCatalogSearch([]string{"alpha-market plugin"}, "alpha market") {
		t.Fatal("expected multi-token search to match tokenized values")
	}
}

func TestNormalizeArtifactIndexMergesSameVersionChannels(t *testing.T) {
	index := NormalizeArtifactIndex("plugin_package", "demo", map[string]any{
		"versions": []map[string]any{
			{"version": "1.0.0", "channel": "alpha"},
			{"version": "1.0.0", "channel": "stable"},
			{"version": "1.0.0", "channel": "beta"},
		},
	})

	versions := index["versions"].([]map[string]any)
	if len(versions) != 1 {
		t.Fatalf("expected one merged version entry, got %#v", versions)
	}
	channels := ArtifactVersionChannels(versions[0])
	if len(channels) != 3 {
		t.Fatalf("expected merged channels, got %#v", channels)
	}
	if !ArtifactVersionMatchesChannel(versions[0], "beta") {
		t.Fatalf("expected merged version entry to match beta channel, got %#v", versions[0])
	}
}

func TestCollectChannelsFromIndexUsesMergedVersionChannels(t *testing.T) {
	versions := NormalizeArtifactVersionEntries([]map[string]any{
		{"version": "1.0.0", "channel": "alpha"},
		{"version": "1.0.0", "channel": "stable"},
		{"version": "1.0.0", "channel": "beta"},
		{"version": "1.1.0", "channel": "stable"},
	})

	channels := collectChannelsFromIndex(versions)
	if len(channels) != 3 {
		t.Fatalf("expected merged artifact channels, got %#v", channels)
	}
	for _, channel := range []string{"alpha", "stable", "beta"} {
		if !containsStringFold(channels, channel) {
			t.Fatalf("expected artifact channels to contain %q, got %#v", channel, channels)
		}
	}
}

func TestNormalizeArtifactIndexSetsTopLevelMergedChannels(t *testing.T) {
	index := NormalizeArtifactIndex("plugin_package", "demo", map[string]any{
		"versions": []map[string]any{
			{"version": "1.0.0", "channel": "alpha"},
			{"version": "1.0.0", "channel": "stable"},
			{"version": "1.0.0", "channel": "beta"},
		},
	})

	channels := normalizedArtifactChannels(index["channels"])
	if len(channels) != 3 {
		t.Fatalf("expected top-level merged channels, got %#v", index["channels"])
	}
	for _, channel := range []string{"alpha", "stable", "beta"} {
		if !containsStringFold(channels, channel) {
			t.Fatalf("expected top-level channels to contain %q, got %#v", channel, channels)
		}
	}
}

func TestNormalizeCatalogSnapshotItemsBackfillsChannels(t *testing.T) {
	items := normalizeCatalogSnapshotItems([]map[string]any{
		{"name": "demo", "channel": "stable"},
	})

	if len(items) != 1 {
		t.Fatalf("expected one item, got %#v", items)
	}
	channels := ArtifactVersionChannels(items[0])
	if len(channels) != 1 || !containsStringFold(channels, "stable") {
		t.Fatalf("expected snapshot item to backfill stable channel, got %#v", items[0])
	}
}
