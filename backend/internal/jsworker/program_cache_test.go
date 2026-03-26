package jsworker

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadWorkerProgramCachesByFileMetadata(t *testing.T) {
	originalCache := globalWorkerProgramCache
	globalWorkerProgramCache = newWorkerProgramCache(16)
	t.Cleanup(func() {
		globalWorkerProgramCache = originalCache
	})

	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "index.js")
	mustWriteFile(t, scriptPath, []byte(`module.exports = { value: 1 };`))

	programA, err := loadWorkerProgram(scriptPath, workerProgramWrapperNone)
	if err != nil {
		t.Fatalf("loadWorkerProgram first load failed: %v", err)
	}
	programB, err := loadWorkerProgram(scriptPath, workerProgramWrapperNone)
	if err != nil {
		t.Fatalf("loadWorkerProgram second load failed: %v", err)
	}
	if programA != programB {
		t.Fatalf("expected cached compiled program pointer to be reused")
	}

	info, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("stat script failed: %v", err)
	}
	nextTime := info.ModTime().Add(2 * time.Second)
	mustWriteFile(t, scriptPath, []byte(`module.exports = { value: 2, extra: true };`))
	if err := os.Chtimes(scriptPath, nextTime, nextTime); err != nil {
		t.Fatalf("chtimes script failed: %v", err)
	}

	programC, err := loadWorkerProgram(scriptPath, workerProgramWrapperNone)
	if err != nil {
		t.Fatalf("loadWorkerProgram after file change failed: %v", err)
	}
	if programC == programA {
		t.Fatalf("expected cache invalidation after file metadata change")
	}
}

func TestLoadWorkerProgramSeparatesWrapperModes(t *testing.T) {
	originalCache := globalWorkerProgramCache
	globalWorkerProgramCache = newWorkerProgramCache(16)
	t.Cleanup(func() {
		globalWorkerProgramCache = originalCache
	})

	rootDir := t.TempDir()
	scriptPath := filepath.Join(rootDir, "module.js")
	mustWriteFile(t, scriptPath, []byte(`module.exports = { ok: true };`))

	plainProgram, err := loadWorkerProgram(scriptPath, workerProgramWrapperNone)
	if err != nil {
		t.Fatalf("loadWorkerProgram plain failed: %v", err)
	}
	wrappedProgram, err := loadWorkerProgram(scriptPath, workerProgramWrapperCommonJS)
	if err != nil {
		t.Fatalf("loadWorkerProgram commonjs failed: %v", err)
	}
	if plainProgram == wrappedProgram {
		t.Fatalf("expected wrapper mode to use an isolated cache entry")
	}
}

func TestInvalidateWorkerProgramCachePathPrefixRemovesOnlyMatchingEntries(t *testing.T) {
	originalCache := globalWorkerProgramCache
	globalWorkerProgramCache = newWorkerProgramCache(16)
	t.Cleanup(func() {
		globalWorkerProgramCache = originalCache
	})

	rootDir := t.TempDir()
	matchDir := filepath.Join(rootDir, "match")
	otherDir := filepath.Join(rootDir, "other")
	if err := os.MkdirAll(matchDir, 0755); err != nil {
		t.Fatalf("mkdir match dir failed: %v", err)
	}
	if err := os.MkdirAll(otherDir, 0755); err != nil {
		t.Fatalf("mkdir other dir failed: %v", err)
	}

	matchPath := filepath.Join(matchDir, "index.js")
	otherPath := filepath.Join(otherDir, "index.js")
	mustWriteFile(t, matchPath, []byte(`module.exports = { value: 1 };`))
	mustWriteFile(t, otherPath, []byte(`module.exports = { value: 2 };`))

	if _, err := loadWorkerProgram(matchPath, workerProgramWrapperNone); err != nil {
		t.Fatalf("loadWorkerProgram match failed: %v", err)
	}
	if _, err := loadWorkerProgram(otherPath, workerProgramWrapperNone); err != nil {
		t.Fatalf("loadWorkerProgram other failed: %v", err)
	}

	if count := WorkerProgramCacheEntryCount(); count != 2 {
		t.Fatalf("expected 2 cache entries, got %d", count)
	}

	removed := InvalidateWorkerProgramCachePathPrefix(matchDir)
	if removed != 1 {
		t.Fatalf("expected 1 matching cache entry to be removed, got %d", removed)
	}
	if count := WorkerProgramCacheEntryCount(); count != 1 {
		t.Fatalf("expected 1 cache entry remaining, got %d", count)
	}
}
