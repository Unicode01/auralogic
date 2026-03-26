import {
  applyWorkspaceStreamEvent,
  extractWorkspaceRuntimeState,
  extractWorkspaceSnapshot,
  preferNewerWorkspaceSnapshot,
} from './plugin-workspace-stream'

describe('plugin-workspace-stream', () => {
  test('extracts workspace snapshot from wrapped and direct payloads', () => {
    const snapshot = {
      plugin_id: 7,
      enabled: true,
      buffer_capacity: 128,
      entry_count: 1,
      last_seq: 3,
      entries: [],
    }

    expect(extractWorkspaceSnapshot({ workspace: snapshot })).toEqual(snapshot)
    expect(extractWorkspaceSnapshot(snapshot)).toEqual(snapshot)
  })

  test('extracts runtime state from wrapped and direct payloads', () => {
    const runtimeState = {
      available: true,
      exists: true,
      plugin_id: 7,
      eval_count: 4,
    }

    expect(extractWorkspaceRuntimeState({ runtime_state: runtimeState })).toEqual(runtimeState)
    expect(extractWorkspaceRuntimeState(runtimeState)).toEqual(runtimeState)
  })

  test('applies delta entries and updates counters', () => {
    const current = {
      plugin_id: 7,
      enabled: true,
      buffer_capacity: 3,
      entry_count: 1,
      last_seq: 1,
      entries: [{ seq: 1, message: 'first' }],
    }

    const next = applyWorkspaceStreamEvent(current, {
      type: 'delta',
      last_seq: 2,
      entry_count: 2,
      entries: [{ seq: 2, message: 'second' }],
    })

    expect(next?.entries.map((entry) => entry.message)).toEqual(['first', 'second'])
    expect(next?.last_seq).toBe(2)
    expect(next?.entry_count).toBe(2)
  })

  test('prefers newer workspace snapshots', () => {
    const current = {
      plugin_id: 7,
      enabled: true,
      buffer_capacity: 128,
      entry_count: 1,
      last_seq: 2,
      entries: [],
    }
    const next = {
      ...current,
      last_seq: 3,
    }

    expect(preferNewerWorkspaceSnapshot(current, next)).toEqual(next)
    expect(preferNewerWorkspaceSnapshot(next, current)).toEqual(next)
  })
})
