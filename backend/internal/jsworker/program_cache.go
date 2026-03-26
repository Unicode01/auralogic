package jsworker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dop251/goja"
)

const defaultWorkerProgramCacheEntries = 1024

type workerProgramWrapperMode string

const (
	workerProgramWrapperNone     workerProgramWrapperMode = "plain"
	workerProgramWrapperCommonJS workerProgramWrapperMode = "commonjs"
)

type workerProgramCacheKey struct {
	path    string
	wrapper workerProgramWrapperMode
}

type workerProgramCacheEntry struct {
	size            int64
	modTimeUnixNano int64
	program         *goja.Program
}

type workerProgramCache struct {
	mu         sync.RWMutex
	maxEntries int
	entries    map[workerProgramCacheKey]workerProgramCacheEntry
	order      []workerProgramCacheKey
}

var globalWorkerProgramCache = newWorkerProgramCache(defaultWorkerProgramCacheEntries)

func newWorkerProgramCache(maxEntries int) *workerProgramCache {
	if maxEntries <= 0 {
		maxEntries = defaultWorkerProgramCacheEntries
	}
	return &workerProgramCache{
		maxEntries: maxEntries,
		entries:    make(map[workerProgramCacheKey]workerProgramCacheEntry, maxEntries),
		order:      make([]workerProgramCacheKey, 0, maxEntries),
	}
}

func (c *workerProgramCache) Get(
	key workerProgramCacheKey,
	size int64,
	modTimeUnixNano int64,
) (*goja.Program, bool) {
	if c == nil {
		return nil, false
	}

	c.mu.RLock()
	entry, exists := c.entries[key]
	c.mu.RUnlock()
	if !exists {
		return nil, false
	}
	if entry.size != size || entry.modTimeUnixNano != modTimeUnixNano || entry.program == nil {
		return nil, false
	}
	return entry.program, true
}

func (c *workerProgramCache) Set(
	key workerProgramCacheKey,
	size int64,
	modTimeUnixNano int64,
	program *goja.Program,
) {
	if c == nil || program == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; !exists {
		c.order = append(c.order, key)
	}
	c.entries[key] = workerProgramCacheEntry{
		size:            size,
		modTimeUnixNano: modTimeUnixNano,
		program:         program,
	}

	for len(c.entries) > c.maxEntries && len(c.order) > 0 {
		oldest := c.order[0]
		c.order[0] = workerProgramCacheKey{}
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
	if cap(c.order) > c.maxEntries*2 && len(c.order) < c.maxEntries {
		compacted := make([]workerProgramCacheKey, len(c.order), c.maxEntries)
		copy(compacted, c.order)
		c.order = compacted
	}
}

func (c *workerProgramCache) Clear() int {
	if c == nil {
		return 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	cleared := len(c.entries)
	c.entries = make(map[workerProgramCacheKey]workerProgramCacheEntry, c.maxEntries)
	c.order = c.order[:0]
	return cleared
}

func (c *workerProgramCache) DeleteMatching(match func(workerProgramCacheKey) bool) int {
	if c == nil || match == nil {
		return 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) == 0 {
		return 0
	}

	nextOrder := make([]workerProgramCacheKey, 0, len(c.order))
	removed := 0
	for _, key := range c.order {
		entry, exists := c.entries[key]
		if !exists {
			continue
		}
		if match(key) {
			delete(c.entries, key)
			removed++
			continue
		}
		nextOrder = append(nextOrder, key)
		c.entries[key] = entry
	}
	c.order = nextOrder
	return removed
}

func (c *workerProgramCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func InvalidateWorkerProgramCachePath(path string) int {
	normalizedPath := normalizedWorkerProgramCachePath(path)
	if normalizedPath == "" {
		return 0
	}
	return globalWorkerProgramCache.DeleteMatching(func(key workerProgramCacheKey) bool {
		return key.path == normalizedPath
	})
}

func InvalidateWorkerProgramCachePathPrefix(prefix string) int {
	normalizedPrefix := normalizedWorkerProgramCachePath(prefix)
	if normalizedPrefix == "" {
		return 0
	}
	directoryPrefix := normalizedPrefix
	if !strings.HasSuffix(directoryPrefix, "/") {
		directoryPrefix += "/"
	}
	return globalWorkerProgramCache.DeleteMatching(func(key workerProgramCacheKey) bool {
		return key.path == normalizedPrefix || strings.HasPrefix(key.path, directoryPrefix)
	})
}

func WorkerProgramCacheEntryCount() int {
	return globalWorkerProgramCache.Len()
}

func loadWorkerProgram(path string, wrapper workerProgramWrapperMode) (*goja.Program, error) {
	normalizedPath := normalizeScriptPath(path)
	if normalizedPath == "" {
		return nil, fmt.Errorf("script path is required")
	}

	info, err := os.Stat(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("stat script failed: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("script path is a directory: %s", normalizedPath)
	}

	cacheKey := workerProgramCacheKey{
		path:    normalizedWorkerProgramCachePath(normalizedPath),
		wrapper: wrapper,
	}
	size := info.Size()
	modTimeUnixNano := info.ModTime().UTC().UnixNano()
	if program, ok := globalWorkerProgramCache.Get(cacheKey, size, modTimeUnixNano); ok {
		return program, nil
	}

	content, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, fmt.Errorf("read script failed: %w", err)
	}

	source := string(content)
	switch wrapper {
	case workerProgramWrapperCommonJS:
		source = wrapCommonJSModuleSource(source)
	case workerProgramWrapperNone:
	default:
		return nil, fmt.Errorf("unsupported worker program wrapper %q", wrapper)
	}

	program, err := goja.Compile(normalizedPath, source, true)
	if err != nil {
		return nil, fmt.Errorf("compile script failed: %w", err)
	}
	globalWorkerProgramCache.Set(cacheKey, size, modTimeUnixNano, program)
	return program, nil
}

func wrapCommonJSModuleSource(source string) string {
	return "(function(exports, module, require, __filename, __dirname){\n" + source + "\n})"
}

func normalizedWorkerProgramCachePath(path string) string {
	return filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(path))))
}
