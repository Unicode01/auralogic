package service

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"auralogic/internal/models"
)

type pluginRuntimeSlot struct {
	PluginID       uint
	Generation     uint
	Runtime        string
	PluginSnapshot models.Plugin
	GRPCClient     *PluginClient
	ActivatedAt    time.Time
	onClose        func()

	inFlight              atomic.Int64
	drainingSinceUnixNano atomic.Int64
	closeOnce             sync.Once
}

type pluginRuntimeSlotSet struct {
	Active   *pluginRuntimeSlot
	Draining []*pluginRuntimeSlot
}

type pluginRuntimeSlotInspection struct {
	ActiveGeneration    uint
	ActiveInFlight      int64
	DrainingSlotCount   int
	DrainingInFlight    int64
	DrainingGenerations []uint
}

func newPluginRuntimeSlot(plugin *models.Plugin, runtime string, client *PluginClient) *pluginRuntimeSlot {
	if plugin == nil {
		return nil
	}

	snapshot := *plugin
	return &pluginRuntimeSlot{
		PluginID:       plugin.ID,
		Generation:     resolvePluginAppliedGeneration(plugin),
		Runtime:        strings.ToLower(strings.TrimSpace(runtime)),
		PluginSnapshot: snapshot,
		GRPCClient:     client,
		ActivatedAt:    time.Now().UTC(),
	}
}

func resolvePluginAppliedGeneration(plugin *models.Plugin) uint {
	if plugin == nil {
		return 1
	}
	if plugin.AppliedGeneration > 0 {
		return plugin.AppliedGeneration
	}
	if plugin.DesiredGeneration > 0 {
		return plugin.DesiredGeneration
	}
	return 1
}

func (s *pluginRuntimeSlot) acquire() {
	if s == nil {
		return
	}
	s.inFlight.Add(1)
}

func (s *pluginRuntimeSlot) release() int64 {
	if s == nil {
		return 0
	}
	remaining := s.inFlight.Add(-1)
	if remaining < 0 {
		s.inFlight.Store(0)
		return 0
	}
	return remaining
}

func (s *pluginRuntimeSlot) InFlight() int64 {
	if s == nil {
		return 0
	}
	return s.inFlight.Load()
}

func (s *pluginRuntimeSlot) markDraining(now time.Time) {
	if s == nil {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	s.drainingSinceUnixNano.CompareAndSwap(0, now.UTC().UnixNano())
}

func (s *pluginRuntimeSlot) IsDraining() bool {
	if s == nil {
		return false
	}
	return s.drainingSinceUnixNano.Load() > 0
}

func (s *pluginRuntimeSlot) DrainingSince() *time.Time {
	if s == nil {
		return nil
	}
	unixNano := s.drainingSinceUnixNano.Load()
	if unixNano <= 0 {
		return nil
	}
	value := time.Unix(0, unixNano).UTC()
	return &value
}

func (s *pluginRuntimeSlot) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		if s.GRPCClient != nil {
			_ = s.GRPCClient.Close()
		}
		if s.onClose != nil {
			s.onClose()
		}
	})
}

func slotMatchesExpectation(slot *pluginRuntimeSlot, runtime string, generation uint) bool {
	if slot == nil {
		return false
	}
	if normalizedRuntime := strings.ToLower(strings.TrimSpace(runtime)); normalizedRuntime != "" && slot.Runtime != normalizedRuntime {
		return false
	}
	if generation > 0 && slot.Generation != generation {
		return false
	}
	return true
}

func selectMatchingRuntimeSlot(
	state *pluginRuntimeSlotSet,
	runtime string,
	generation uint,
) *pluginRuntimeSlot {
	if state == nil {
		return nil
	}

	if slotMatchesExpectation(state.Active, runtime, generation) {
		return state.Active
	}
	for idx := len(state.Draining) - 1; idx >= 0; idx-- {
		if slotMatchesExpectation(state.Draining[idx], runtime, generation) {
			return state.Draining[idx]
		}
	}
	if generation > 0 {
		if slotMatchesExpectation(state.Active, runtime, 0) {
			return state.Active
		}
		for idx := len(state.Draining) - 1; idx >= 0; idx-- {
			if slotMatchesExpectation(state.Draining[idx], runtime, 0) {
				return state.Draining[idx]
			}
		}
	}
	return nil
}

func (s *PluginManagerService) acquirePluginRuntimeSlot(
	pluginID uint,
	runtime string,
	generation uint,
) (*pluginRuntimeSlot, bool) {
	if s == nil || pluginID == 0 {
		return nil, false
	}

	s.mu.RLock()
	state := s.runtimeSlots[pluginID]
	slot := selectMatchingRuntimeSlot(state, runtime, generation)
	if slot != nil {
		slot.acquire()
	}
	s.mu.RUnlock()
	return slot, slot != nil
}

func (s *PluginManagerService) releasePluginRuntimeSlot(slot *pluginRuntimeSlot) {
	if s == nil || slot == nil {
		return
	}
	remaining := slot.release()
	if remaining == 0 && slot.IsDraining() {
		s.finalizeDrainingRuntimeSlot(slot.PluginID, slot)
	}
}

func (s *PluginManagerService) activatePreparedRuntimeSlot(
	pluginID uint,
	slot *pluginRuntimeSlot,
) *pluginRuntimeSlot {
	if s == nil || pluginID == 0 {
		return nil
	}

	var previous *pluginRuntimeSlot
	now := time.Now().UTC()

	s.mu.Lock()
	if s.runtimeSlots == nil {
		s.runtimeSlots = make(map[uint]*pluginRuntimeSlotSet)
	}
	state := s.runtimeSlots[pluginID]
	if state == nil {
		state = &pluginRuntimeSlotSet{}
		s.runtimeSlots[pluginID] = state
	}
	previous = state.Active
	state.Active = slot
	if slot != nil && slot.Runtime == PluginRuntimeGRPC && slot.GRPCClient != nil {
		if s.grpcClients == nil {
			s.grpcClients = make(map[uint]*PluginClient)
		}
		s.grpcClients[pluginID] = slot.GRPCClient
	} else if s.grpcClients != nil {
		delete(s.grpcClients, pluginID)
	}
	if previous != nil {
		previous.markDraining(now)
		state.Draining = append(state.Draining, previous)
	}
	s.mu.Unlock()

	if previous != nil && previous.InFlight() == 0 {
		s.finalizeDrainingRuntimeSlot(pluginID, previous)
	}
	return previous
}

func (s *PluginManagerService) deactivateActiveRuntimeSlot(pluginID uint) *pluginRuntimeSlot {
	if s == nil || pluginID == 0 {
		return nil
	}

	var previous *pluginRuntimeSlot
	now := time.Now().UTC()

	s.mu.Lock()
	if state := s.runtimeSlots[pluginID]; state != nil {
		previous = state.Active
		state.Active = nil
		if previous != nil {
			previous.markDraining(now)
			state.Draining = append(state.Draining, previous)
		}
		if state.Active == nil && len(state.Draining) == 0 {
			delete(s.runtimeSlots, pluginID)
		}
	}
	if s.grpcClients != nil {
		delete(s.grpcClients, pluginID)
	}
	s.mu.Unlock()

	if previous != nil && previous.InFlight() == 0 {
		s.finalizeDrainingRuntimeSlot(pluginID, previous)
	}
	return previous
}

func (s *PluginManagerService) finalizeDrainingRuntimeSlot(pluginID uint, slot *pluginRuntimeSlot) {
	if s == nil || pluginID == 0 || slot == nil {
		return
	}

	removed := false
	s.mu.Lock()
	if state := s.runtimeSlots[pluginID]; state != nil {
		nextDraining := make([]*pluginRuntimeSlot, 0, len(state.Draining))
		for _, item := range state.Draining {
			if item == nil {
				continue
			}
			if item == slot {
				removed = true
				continue
			}
			nextDraining = append(nextDraining, item)
		}
		state.Draining = nextDraining
		if state.Active == nil && len(state.Draining) == 0 {
			delete(s.runtimeSlots, pluginID)
		}
	}
	s.mu.Unlock()

	if removed {
		slot.Close()
	}
}

const drainingSlotMaxIdleAge = 5 * time.Minute

func (s *PluginManagerService) sweepExpiredDrainingSlots() {
	if s == nil {
		return
	}

	now := time.Now().UTC()

	s.mu.RLock()
	var candidates []struct {
		pluginID uint
		slot     *pluginRuntimeSlot
	}
	for pluginID, state := range s.runtimeSlots {
		if state == nil {
			continue
		}
		for _, slot := range state.Draining {
			if slot == nil {
				continue
			}
			since := slot.DrainingSince()
			if since != nil && now.Sub(*since) >= drainingSlotMaxIdleAge {
				candidates = append(candidates, struct {
					pluginID uint
					slot     *pluginRuntimeSlot
				}{pluginID, slot})
			}
		}
	}
	s.mu.RUnlock()

	for _, c := range candidates {
		s.finalizeDrainingRuntimeSlot(c.pluginID, c.slot)
	}
}

func (s *PluginManagerService) inspectPluginRuntimeSlots(pluginID uint) pluginRuntimeSlotInspection {
	inspection := pluginRuntimeSlotInspection{
		DrainingGenerations: []uint{},
	}
	if s == nil || pluginID == 0 {
		return inspection
	}

	s.mu.RLock()
	state := s.runtimeSlots[pluginID]
	if state != nil && state.Active != nil {
		inspection.ActiveGeneration = state.Active.Generation
		inspection.ActiveInFlight = state.Active.InFlight()
	}
	if state != nil && len(state.Draining) > 0 {
		inspection.DrainingSlotCount = len(state.Draining)
		for _, slot := range state.Draining {
			if slot == nil {
				continue
			}
			inspection.DrainingInFlight += slot.InFlight()
			inspection.DrainingGenerations = append(inspection.DrainingGenerations, slot.Generation)
		}
	}
	s.mu.RUnlock()
	return inspection
}

func (s *PluginManagerService) listActiveRuntimeSlots(runtime string) []*pluginRuntimeSlot {
	if s == nil {
		return []*pluginRuntimeSlot{}
	}

	normalizedRuntime := strings.ToLower(strings.TrimSpace(runtime))
	s.mu.RLock()
	slots := make([]*pluginRuntimeSlot, 0, len(s.runtimeSlots))
	for _, state := range s.runtimeSlots {
		if state == nil || state.Active == nil {
			continue
		}
		if normalizedRuntime != "" && state.Active.Runtime != normalizedRuntime {
			continue
		}
		state.Active.acquire()
		slots = append(slots, state.Active)
	}
	s.mu.RUnlock()
	return slots
}

func (s *PluginManagerService) closeAllRuntimeSlots() {
	if s == nil {
		return
	}

	s.mu.Lock()
	allSlots := make([]*pluginRuntimeSlot, 0, len(s.runtimeSlots)*2)
	for _, state := range s.runtimeSlots {
		if state == nil {
			continue
		}
		if state.Active != nil {
			allSlots = append(allSlots, state.Active)
		}
		allSlots = append(allSlots, state.Draining...)
	}
	s.runtimeSlots = make(map[uint]*pluginRuntimeSlotSet)
	s.grpcClients = make(map[uint]*PluginClient)
	s.mu.Unlock()

	for _, slot := range allSlots {
		if slot != nil {
			slot.Close()
		}
	}
}
