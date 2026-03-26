package service

import (
	"errors"
	"fmt"

	"auralogic/internal/models"
)

type pluginExecutionCatalogEntry struct {
	Plugin           models.Plugin
	Runtime          string
	CapabilityPolicy pluginCapabilityPolicy
	ValidationError  string
}

type pluginExecutionCatalog struct {
	byID            map[uint]pluginExecutionCatalogEntry
	hookIDs         map[string][]uint
	wildcardHookIDs []uint
	jsWorkerIDs     []uint
}

func newPluginExecutionCatalog() pluginExecutionCatalog {
	return pluginExecutionCatalog{
		byID:            make(map[uint]pluginExecutionCatalogEntry),
		hookIDs:         make(map[string][]uint),
		wildcardHookIDs: []uint{},
		jsWorkerIDs:     []uint{},
	}
}

func (s *PluginManagerService) RefreshPluginExecutionCatalog() error {
	if s == nil {
		return nil
	}
	if s.db == nil || !s.isPluginPlatformEnabled() {
		s.replacePluginExecutionCatalog(newPluginExecutionCatalog())
		return nil
	}

	var plugins []models.Plugin
	if err := s.db.Where("enabled = ?", true).Order("id ASC").Find(&plugins).Error; err != nil {
		return err
	}
	s.setPluginExecutionCatalogFromPlugins(plugins)
	return nil
}

func (s *PluginManagerService) setPluginExecutionCatalogFromPlugins(plugins []models.Plugin) {
	if s == nil {
		return
	}
	s.replacePluginExecutionCatalog(s.buildPluginExecutionCatalog(plugins))
}

func (s *PluginManagerService) replacePluginExecutionCatalog(catalog pluginExecutionCatalog) {
	if s == nil {
		return
	}
	s.catalogMu.Lock()
	s.executionCatalog = catalog
	s.catalogMu.Unlock()
}

func (s *PluginManagerService) buildPluginExecutionCatalog(
	plugins []models.Plugin,
) pluginExecutionCatalog {
	catalog := newPluginExecutionCatalog()
	for _, plugin := range plugins {
		if !plugin.Enabled {
			continue
		}

		entry := s.buildPluginExecutionCatalogEntry(plugin)
		catalog.byID[plugin.ID] = entry

		if entry.Runtime == PluginRuntimeJSWorker {
			catalog.jsWorkerIDs = append(catalog.jsWorkerIDs, plugin.ID)
		}
		if entry.ValidationError != "" || !entry.CapabilityPolicy.Valid || !entry.CapabilityPolicy.AllowHookExecute {
			continue
		}
		if len(entry.CapabilityPolicy.Hooks) == 0 {
			catalog.wildcardHookIDs = append(catalog.wildcardHookIDs, plugin.ID)
			continue
		}
		for _, hook := range entry.CapabilityPolicy.Hooks {
			if hook == "" || hookInList(entry.CapabilityPolicy.DisabledHooks, hook) {
				continue
			}
			catalog.hookIDs[hook] = append(catalog.hookIDs[hook], plugin.ID)
		}
	}
	return catalog
}

func (s *PluginManagerService) buildPluginExecutionCatalogEntry(
	plugin models.Plugin,
) pluginExecutionCatalogEntry {
	entry := pluginExecutionCatalogEntry{
		Plugin:           plugin,
		CapabilityPolicy: resolvePluginCapabilityPolicy(&plugin),
	}

	runtime, err := s.ResolveRuntime(plugin.Runtime)
	if err != nil {
		entry.ValidationError = err.Error()
		return entry
	}
	entry.Runtime = runtime
	if err := s.ValidatePluginProfile(runtime, plugin.Type); err != nil {
		entry.ValidationError = err.Error()
		return entry
	}
	if err := ValidatePluginProtocolCompatibility(&plugin); err != nil {
		entry.ValidationError = err.Error()
	}
	return entry
}

func (s *PluginManagerService) clearPluginExecutionCatalog() {
	if s == nil {
		return
	}
	s.replacePluginExecutionCatalog(newPluginExecutionCatalog())
}

func (s *PluginManagerService) getPluginExecutionCatalogEntry(
	pluginID uint,
) (pluginExecutionCatalogEntry, bool) {
	if s == nil || pluginID == 0 {
		return pluginExecutionCatalogEntry{}, false
	}
	s.catalogMu.RLock()
	entry, exists := s.executionCatalog.byID[pluginID]
	s.catalogMu.RUnlock()
	return entry, exists
}

func (s *PluginManagerService) ResolvePluginExecuteAPIAvailabilityFromCatalog(pluginIDs []uint) map[uint]bool {
	if s == nil || len(pluginIDs) == 0 {
		return map[uint]bool{}
	}

	s.catalogMu.RLock()
	defer s.catalogMu.RUnlock()

	availability := make(map[uint]bool, len(pluginIDs))
	for _, pluginID := range pluginIDs {
		if pluginID == 0 {
			continue
		}
		entry, exists := s.executionCatalog.byID[pluginID]
		if !exists {
			continue
		}
		availability[pluginID] = entry.ValidationError == "" &&
			entry.CapabilityPolicy.Valid &&
			entry.CapabilityPolicy.AllowExecuteAPI
	}
	return availability
}

func (s *PluginManagerService) ResolvePluginFrontendHTMLModesFromCatalog(pluginIDs []uint) map[uint]string {
	if s == nil || len(pluginIDs) == 0 {
		return map[uint]string{}
	}

	s.catalogMu.RLock()
	defer s.catalogMu.RUnlock()

	modes := make(map[uint]string, len(pluginIDs))
	for _, pluginID := range pluginIDs {
		if pluginID == 0 {
			continue
		}
		entry, exists := s.executionCatalog.byID[pluginID]
		if !exists {
			continue
		}
		mode := "sanitize"
		if entry.CapabilityPolicy.Valid {
			mode = entry.CapabilityPolicy.FrontendHTMLMode
		}
		modes[pluginID] = mode
	}
	return modes
}

func (s *PluginManagerService) listJSWorkerCatalogPlugins() []models.Plugin {
	if s == nil {
		return []models.Plugin{}
	}

	s.catalogMu.RLock()
	defer s.catalogMu.RUnlock()

	if len(s.executionCatalog.jsWorkerIDs) == 0 {
		return []models.Plugin{}
	}

	plugins := make([]models.Plugin, 0, len(s.executionCatalog.jsWorkerIDs))
	for _, pluginID := range s.executionCatalog.jsWorkerIDs {
		entry, exists := s.executionCatalog.byID[pluginID]
		if !exists {
			continue
		}
		plugins = append(plugins, entry.Plugin)
	}
	return plugins
}

func (s *PluginManagerService) listHookExecutionCatalogEntries(
	hook string,
) []pluginExecutionCatalogEntry {
	if s == nil {
		return []pluginExecutionCatalogEntry{}
	}

	normalizedHook := normalizeHookName(hook)
	if normalizedHook == "" {
		return []pluginExecutionCatalogEntry{}
	}

	s.catalogMu.RLock()
	defer s.catalogMu.RUnlock()

	specificIDs := s.executionCatalog.hookIDs[normalizedHook]
	wildcardIDs := s.executionCatalog.wildcardHookIDs
	if len(specificIDs) == 0 && len(wildcardIDs) == 0 {
		return []pluginExecutionCatalogEntry{}
	}

	out := make([]pluginExecutionCatalogEntry, 0, len(specificIDs)+len(wildcardIDs))
	specificIdx := 0
	wildcardIdx := 0
	var lastID uint
	for specificIdx < len(specificIDs) || wildcardIdx < len(wildcardIDs) {
		useSpecific := false
		switch {
		case wildcardIdx >= len(wildcardIDs):
			useSpecific = true
		case specificIdx >= len(specificIDs):
			useSpecific = false
		default:
			useSpecific = specificIDs[specificIdx] <= wildcardIDs[wildcardIdx]
		}

		var pluginID uint
		if useSpecific {
			pluginID = specificIDs[specificIdx]
			specificIdx++
		} else {
			pluginID = wildcardIDs[wildcardIdx]
			wildcardIdx++
		}

		if pluginID == 0 {
			continue
		}
		if len(out) > 0 && pluginID == lastID {
			continue
		}

		entry, exists := s.executionCatalog.byID[pluginID]
		if !exists {
			continue
		}
		out = append(out, entry)
		lastID = pluginID
	}

	return out
}

func (s *PluginManagerService) getPluginByIDWithCatalog(
	pluginID uint,
) (*models.Plugin, string, pluginCapabilityPolicy, error, bool) {
	if entry, exists := s.getPluginExecutionCatalogEntry(pluginID); exists {
		plugin := entry.Plugin
		if !plugin.Enabled {
			return &plugin, entry.Runtime, entry.CapabilityPolicy, fmt.Errorf("plugin %d is disabled", plugin.ID), true
		}
		if entry.ValidationError != "" {
			return &plugin, entry.Runtime, entry.CapabilityPolicy, errors.New(entry.ValidationError), true
		}
		return &plugin, entry.Runtime, entry.CapabilityPolicy, nil, true
	}

	plugin, err := s.getPluginByID(pluginID)
	if err != nil {
		return nil, "", pluginCapabilityPolicy{}, err, false
	}
	if !plugin.Enabled {
		return plugin, "", pluginCapabilityPolicy{}, fmt.Errorf("plugin %d is disabled", plugin.ID), false
	}
	runtime, err := s.ResolveRuntime(plugin.Runtime)
	if err != nil {
		return plugin, "", pluginCapabilityPolicy{}, err, false
	}
	if err := s.ValidatePluginProfile(runtime, plugin.Type); err != nil {
		return plugin, runtime, pluginCapabilityPolicy{}, err, false
	}
	if err := ValidatePluginProtocolCompatibility(plugin); err != nil {
		return plugin, runtime, pluginCapabilityPolicy{}, err, false
	}
	policy := resolvePluginCapabilityPolicy(plugin)
	return plugin, runtime, policy, nil, false
}

func (s *PluginManagerService) removePluginExecutionCatalogEntry(pluginID uint) {
	if s == nil || pluginID == 0 {
		return
	}
	s.catalogMu.Lock()
	defer s.catalogMu.Unlock()
	if len(s.executionCatalog.byID) == 0 {
		return
	}
	delete(s.executionCatalog.byID, pluginID)
	for hook, ids := range s.executionCatalog.hookIDs {
		filtered := ids[:0]
		for _, id := range ids {
			if id == pluginID {
				continue
			}
			filtered = append(filtered, id)
		}
		if len(filtered) == 0 {
			delete(s.executionCatalog.hookIDs, hook)
			continue
		}
		s.executionCatalog.hookIDs[hook] = filtered
	}
	filteredWildcard := s.executionCatalog.wildcardHookIDs[:0]
	for _, id := range s.executionCatalog.wildcardHookIDs {
		if id == pluginID {
			continue
		}
		filteredWildcard = append(filteredWildcard, id)
	}
	s.executionCatalog.wildcardHookIDs = filteredWildcard

	filteredJSWorker := s.executionCatalog.jsWorkerIDs[:0]
	for _, id := range s.executionCatalog.jsWorkerIDs {
		if id == pluginID {
			continue
		}
		filteredJSWorker = append(filteredJSWorker, id)
	}
	s.executionCatalog.jsWorkerIDs = filteredJSWorker
}
