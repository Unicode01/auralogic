import { resolvePluginWorkspaceCompletion } from './plugin-workspace-completion'

describe('plugin-workspace-completion', () => {
  test('completes helper globals', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: 'hel',
        selectionStart: 3,
      })
    ).toMatchObject({
      kind: 'completed',
      nextValue: 'help',
      suggestions: ['help'],
    })
  })

  test('completes newly added runtime helpers', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: 'per',
        selectionStart: 3,
      })
    ).toMatchObject({
      kind: 'completed',
      nextValue: 'permissions',
      suggestions: ['permissions'],
    })
  })

  test('completes browser-compatible encoding globals', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: 'TextEnc',
        selectionStart: 7,
      })
    ).toMatchObject({
      kind: 'completed',
      nextValue: 'TextEncoder',
      suggestions: ['TextEncoder'],
    })
  })

  test('completes Worker global', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: 'Wor',
        selectionStart: 3,
      })
    ).toMatchObject({
      kind: 'completed',
      nextValue: 'Worker',
      suggestions: ['Worker'],
    })
  })

  test('completes runtime async globals', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: 'struct',
        selectionStart: 6,
      })
    ).toMatchObject({
      kind: 'completed',
      nextValue: 'structuredClone',
      suggestions: ['structuredClone'],
    })
  })

  test('completes runtime timer globals', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: 'queueM',
        selectionStart: 6,
      })
    ).toMatchObject({
      kind: 'completed',
      nextValue: 'queueMicrotask',
      suggestions: ['queueMicrotask'],
    })

    expect(
      resolvePluginWorkspaceCompletion({
        value: 'clearT',
        selectionStart: 6,
      })
    ).toMatchObject({
      kind: 'completed',
      nextValue: 'clearTimeout',
      suggestions: ['clearTimeout'],
    })
  })

  test('completes plugin host paths', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: 'Plugin.host.o',
        selectionStart: 'Plugin.host.o'.length,
      })
    ).toMatchObject({
      kind: 'completed',
      nextValue: 'Plugin.host.order',
      suggestions: ['Plugin.host.order'],
    })
  })

  test('extends to the common prefix when multiple matches remain', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: 'Plugin.i',
        selectionStart: 'Plugin.i'.length,
      })
    ).toMatchObject({
      kind: 'extended',
      nextValue: 'Plugin.inventory',
      resolvedToken: 'Plugin.inventory',
      suggestions: ['Plugin.inventory', 'Plugin.inventoryBinding'],
    })
  })

  test('returns suggestions when the token already matches the common prefix', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: 'Plugin.inventory',
        selectionStart: 'Plugin.inventory'.length,
      })
    ).toMatchObject({
      kind: 'suggestions',
      nextValue: 'Plugin.inventory',
      suggestions: ['Plugin.inventory', 'Plugin.inventoryBinding'],
    })
  })

  test('completes tokens inside expressions', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: 'keys(Plu',
        selectionStart: 'keys(Plu'.length,
      })
    ).toMatchObject({
      kind: 'completed',
      nextValue: 'keys(Plugin',
      suggestions: ['Plugin'],
    })
  })

  test('completes inspect command prefix', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: ':ins',
        selectionStart: 4,
      })
    ).toMatchObject({
      kind: 'completed',
      nextValue: ':inspect',
      suggestions: [':inspect'],
    })
  })

  test('completes plugin-defined globals from dynamic runtime paths', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: 'deb',
        selectionStart: 3,
        dynamicPaths: ['debug', 'globalThis.debug', 'module.exports.debug', 'exports.debug'],
      })
    ).toMatchObject({
      kind: 'completed',
      nextValue: 'debug',
      suggestions: ['debug'],
    })
  })

  test('completes exported workspace handlers from dynamic runtime paths', () => {
    expect(
      resolvePluginWorkspaceCompletion({
        value: 'module.exports.workspace.deb',
        selectionStart: 'module.exports.workspace.deb'.length,
        dynamicPaths: [
          'module',
          'module.exports',
          'module.exports.workspace',
          'module.exports.workspace.debug',
          'exports.workspace.debug',
        ],
      })
    ).toMatchObject({
      kind: 'completed',
      nextValue: 'module.exports.workspace.debug',
      suggestions: ['module.exports.workspace.debug'],
    })
  })
})
