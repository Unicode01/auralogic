package storage

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorageRejectsPathTraversal(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStorage(root, "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}

	if err := store.Write(context.Background(), "../../escape.txt", []byte("bad")); err == nil {
		t.Fatal("expected traversal write to be rejected")
	}
	if _, err := store.Read(context.Background(), "../escape.txt"); err == nil {
		t.Fatal("expected traversal read to be rejected")
	}
	if _, err := store.Exists(context.Background(), "../escape.txt"); err == nil {
		t.Fatal("expected traversal exists to be rejected")
	}
	if err := store.Delete(context.Background(), "../escape.txt"); err == nil {
		t.Fatal("expected traversal delete to be rejected")
	}
}

func TestLocalStorageListHandlesMissingPrefixAndSingleFile(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStorage(root, "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}

	files, err := store.List(context.Background(), "missing")
	if err != nil {
		t.Fatalf("List returned error for missing prefix: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected empty list for missing prefix, got %#v", files)
	}

	if err := store.Write(context.Background(), "artifacts/plugin_package/demo/index.json", []byte("{}")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if err := store.Write(context.Background(), "artifacts/plugin_package/demo/1.0.0/manifest.json", []byte("{}")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	singleFile, err := store.List(context.Background(), "artifacts/plugin_package/demo/index.json")
	if err != nil {
		t.Fatalf("List single file returned error: %v", err)
	}
	if len(singleFile) != 1 || singleFile[0] != filepath.ToSlash("artifacts/plugin_package/demo/index.json") {
		t.Fatalf("unexpected single file list: %#v", singleFile)
	}

	allFiles, err := store.List(context.Background(), "artifacts")
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(allFiles) != 2 {
		t.Fatalf("expected 2 files, got %#v", allFiles)
	}
}

func TestLocalStorageWriteStaysInsideRoot(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStorage(root, "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}

	if err := store.Write(context.Background(), "index/catalog.json", []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	fullPath := filepath.Join(root, "index", "catalog.json")
	if _, err := os.Stat(fullPath); err != nil {
		t.Fatalf("expected file to exist inside root: %v", err)
	}
}

func TestLocalStorageWriteReplacesExistingFileWithoutTempLeak(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalStorage(root, "http://localhost:18080")
	if err != nil {
		t.Fatalf("NewLocalStorage returned error: %v", err)
	}

	ctx := context.Background()
	targetPath := "index/catalog.json"
	if err := store.Write(ctx, targetPath, []byte(`{"version":1}`)); err != nil {
		t.Fatalf("first Write returned error: %v", err)
	}
	if err := store.Write(ctx, targetPath, []byte(`{"version":2}`)); err != nil {
		t.Fatalf("second Write returned error: %v", err)
	}

	body, err := store.Read(ctx, targetPath)
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if string(body) != `{"version":2}` {
		t.Fatalf("expected overwritten file content, got %q", string(body))
	}

	entries, err := os.ReadDir(filepath.Join(root, "index"))
	if err != nil {
		t.Fatalf("ReadDir returned error: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("unexpected temp file left behind: %s", entry.Name())
		}
	}
}
