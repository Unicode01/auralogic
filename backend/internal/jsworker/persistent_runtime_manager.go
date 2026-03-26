package jsworker

import (
	"fmt"
	"sync"

	"auralogic/internal/pluginipc"
)

type persistentPluginRuntimeKey struct {
	PluginID   uint
	Generation uint
}

type persistentPluginRuntimeRef struct {
	runtime  *persistentPluginRuntime
	refCount int
	disposed bool
}

type persistentPluginRuntimeManager struct {
	mu       sync.Mutex
	runtimes map[persistentPluginRuntimeKey]*persistentPluginRuntimeRef
}

var globalPersistentPluginRuntimeManager = newPersistentPluginRuntimeManager()

func newPersistentPluginRuntimeManager() *persistentPluginRuntimeManager {
	return &persistentPluginRuntimeManager{
		runtimes: make(map[persistentPluginRuntimeKey]*persistentPluginRuntimeRef),
	}
}

func normalizePersistentPluginGeneration(generation uint) uint {
	if generation == 0 {
		return 1
	}
	return generation
}

func buildPersistentPluginRuntimeKey(pluginID uint, generation uint) persistentPluginRuntimeKey {
	return persistentPluginRuntimeKey{
		PluginID:   pluginID,
		Generation: normalizePersistentPluginGeneration(generation),
	}
}

func (m *persistentPluginRuntimeManager) acquire(req pluginipc.Request, opts workerOptions) (*persistentPluginRuntime, func(), error) {
	if m == nil {
		return nil, nil, fmt.Errorf("persistent runtime manager is nil")
	}
	if req.PluginID == 0 {
		return nil, nil, fmt.Errorf("plugin_id is required for persistent runtime")
	}

	key := buildPersistentPluginRuntimeKey(req.PluginID, req.PluginGeneration)
	scriptPath := normalizeScriptPath(req.ScriptPath)
	if scriptPath == "" {
		return nil, nil, fmt.Errorf("script_path is required")
	}

	m.mu.Lock()
	ref := m.runtimes[key]
	var staleRuntime *persistentPluginRuntime
	if ref != nil && ref.runtime != nil && !ref.runtime.matchesScript(scriptPath) {
		delete(m.runtimes, key)
		ref.disposed = true
		if ref.refCount == 0 {
			staleRuntime = ref.runtime
		}
		ref = nil
	}
	if ref == nil || ref.runtime == nil {
		fsCtx, err := buildPluginFSRuntimeContext(opts, req.PluginID, req.PluginName, scriptPath)
		if err != nil {
			m.mu.Unlock()
			if staleRuntime != nil {
				staleRuntime.close()
			}
			return nil, nil, err
		}
		runtime, err := newPersistentPluginRuntime(key, scriptPath, opts, fsCtx, true, nil)
		if err != nil {
			m.mu.Unlock()
			if staleRuntime != nil {
				staleRuntime.close()
			}
			return nil, nil, err
		}
		ref = &persistentPluginRuntimeRef{runtime: runtime}
		m.runtimes[key] = ref
	}

	ref.refCount++
	released := false
	release := func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if released {
			return
		}
		released = true
		if ref.refCount > 0 {
			ref.refCount--
		}
		if ref.disposed && ref.refCount == 0 && ref.runtime != nil {
			ref.runtime.close()
		}
	}
	m.mu.Unlock()
	if staleRuntime != nil {
		staleRuntime.close()
	}
	return ref.runtime, release, nil
}

func (m *persistentPluginRuntimeManager) dispose(pluginID uint, generation uint) {
	if m == nil || pluginID == 0 {
		return
	}

	key := buildPersistentPluginRuntimeKey(pluginID, generation)
	m.mu.Lock()
	ref := m.runtimes[key]
	delete(m.runtimes, key)
	if ref != nil {
		ref.disposed = true
	}
	shouldClose := ref != nil && ref.refCount == 0 && ref.runtime != nil
	runtime := (*persistentPluginRuntime)(nil)
	if shouldClose {
		runtime = ref.runtime
	}
	m.mu.Unlock()

	if runtime != nil {
		runtime.close()
	}
}
