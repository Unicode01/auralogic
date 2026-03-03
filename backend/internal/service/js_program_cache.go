package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/dop251/goja"
)

const defaultJSProgramCacheSize = 512

type jsProgramCache struct {
	mu         sync.RWMutex
	maxEntries int
	entries    map[string]*goja.Program
	order      []string
}

func newJSProgramCache(maxEntries int) *jsProgramCache {
	if maxEntries <= 0 {
		maxEntries = defaultJSProgramCacheSize
	}
	return &jsProgramCache{
		maxEntries: maxEntries,
		entries:    make(map[string]*goja.Program, maxEntries),
		order:      make([]string, 0, maxEntries),
	}
}

func (c *jsProgramCache) Get(key string) (*goja.Program, bool) {
	c.mu.RLock()
	program, ok := c.entries[key]
	c.mu.RUnlock()
	return program, ok
}

func (c *jsProgramCache) Set(key string, program *goja.Program) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; exists {
		c.entries[key] = program
		return
	}

	c.entries[key] = program
	c.order = append(c.order, key)

	for len(c.entries) > c.maxEntries && len(c.order) > 0 {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
}

func (c *jsProgramCache) Clear() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	cleared := len(c.entries)
	c.entries = make(map[string]*goja.Program, c.maxEntries)
	c.order = c.order[:0]
	return cleared
}

var globalJSProgramCache = newJSProgramCache(defaultJSProgramCacheSize)

// ClearJSProgramCache clears compiled JS programs cached in memory.
// It returns the number of entries removed.
func ClearJSProgramCache() int {
	return globalJSProgramCache.Clear()
}

func getOrCompileJSProgram(namespace, script string) (*goja.Program, error) {
	if script == "" {
		return nil, fmt.Errorf("script is empty")
	}

	key := buildJSProgramCacheKey(namespace, script)
	if program, ok := globalJSProgramCache.Get(key); ok {
		return program, nil
	}

	program, err := goja.Compile(namespace, script, false)
	if err != nil {
		return nil, err
	}

	globalJSProgramCache.Set(key, program)
	return program, nil
}

func buildJSProgramCacheKey(namespace, script string) string {
	sum := sha256.Sum256([]byte(script))
	return namespace + ":" + hex.EncodeToString(sum[:])
}
