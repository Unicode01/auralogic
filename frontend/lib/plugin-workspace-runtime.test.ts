import {
  normalizeWorkspaceRuntimeInspectDepth,
  parseWorkspaceRuntimeConsoleLine,
} from './plugin-workspace-runtime'

describe('plugin-workspace-runtime', () => {
  test('parses eval input by default', () => {
    expect(parseWorkspaceRuntimeConsoleLine('globalThis.counter += 1')).toEqual({
      action: 'eval',
      expression: 'globalThis.counter += 1',
      depth: 2,
    })
  })

  test('parses inspect input with default expression', () => {
    expect(parseWorkspaceRuntimeConsoleLine(':inspect')).toEqual({
      action: 'inspect',
      expression: 'globalThis',
      depth: 2,
    })
  })

  test('parses inspect input with explicit depth', () => {
    expect(parseWorkspaceRuntimeConsoleLine(':inspect --depth=3 Plugin.host')).toEqual({
      action: 'inspect',
      expression: 'Plugin.host',
      depth: 3,
    })
  })

  test('clamps inspect depth', () => {
    expect(normalizeWorkspaceRuntimeInspectDepth(0)).toBe(2)
    expect(normalizeWorkspaceRuntimeInspectDepth(9)).toBe(4)
  })
})
