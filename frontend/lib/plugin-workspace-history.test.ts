import {
  buildPluginWorkspaceHistoryStorageKey,
  normalizePluginWorkspaceHistoryEntries,
  parsePluginWorkspaceHistoryStorage,
  pushPluginWorkspaceHistoryEntry,
  resolvePluginWorkspaceHistoryNavigation,
  shouldHandlePluginWorkspaceHistoryNavigation,
} from './plugin-workspace-history'

describe('plugin-workspace-history', () => {
  test('builds a scoped storage key per plugin', () => {
    expect(buildPluginWorkspaceHistoryStorageKey(12)).toBe('auralogic.plugin-workspace.history.12')
    expect(buildPluginWorkspaceHistoryStorageKey(0)).toBe('')
  })

  test('normalizes, trims, deduplicates, and caps history entries', () => {
    expect(
      normalizePluginWorkspaceHistoryEntries(['  help()  ', '', 'help()', 'Plugin.host.order.get(1)'], 2)
    ).toEqual(['help()', 'Plugin.host.order.get(1)'])
  })

  test('parses stored history safely', () => {
    expect(parsePluginWorkspaceHistoryStorage('["help()", " :inspect Plugin.host "]')).toEqual([
      'help()',
      ':inspect Plugin.host',
    ])
    expect(parsePluginWorkspaceHistoryStorage('{bad json')).toEqual([])
  })

  test('pushes new entries to the tail and moves duplicates to latest', () => {
    expect(pushPluginWorkspaceHistoryEntry(['help()', 'Plugin.host.user.get(1)'], 'help()')).toEqual([
      'Plugin.host.user.get(1)',
      'help()',
    ])
    expect(pushPluginWorkspaceHistoryEntry(['help()'], '   ')).toEqual(['help()'])
  })

  test('navigates backward through history and restores the draft on the way back down', () => {
    const previous = resolvePluginWorkspaceHistoryNavigation({
      entries: ['help()', ':inspect Plugin.host', 'Plugin.host.order.get({ id: 1 })'],
      currentValue: 'Plugin.host.user.get({ id: 2 })',
      index: -1,
      draft: '',
      direction: 'previous',
    })

    expect(previous).toEqual({
      index: 2,
      value: 'Plugin.host.order.get({ id: 1 })',
      draft: 'Plugin.host.user.get({ id: 2 })',
    })

    const previousAgain = resolvePluginWorkspaceHistoryNavigation({
      entries: ['help()', ':inspect Plugin.host', 'Plugin.host.order.get({ id: 1 })'],
      currentValue: previous.value,
      index: previous.index,
      draft: previous.draft,
      direction: 'previous',
    })

    expect(previousAgain).toEqual({
      index: 1,
      value: ':inspect Plugin.host',
      draft: 'Plugin.host.user.get({ id: 2 })',
    })

    const next = resolvePluginWorkspaceHistoryNavigation({
      entries: ['help()', ':inspect Plugin.host', 'Plugin.host.order.get({ id: 1 })'],
      currentValue: previousAgain.value,
      index: previousAgain.index,
      draft: previousAgain.draft,
      direction: 'next',
    })

    expect(next).toEqual({
      index: 2,
      value: 'Plugin.host.order.get({ id: 1 })',
      draft: 'Plugin.host.user.get({ id: 2 })',
    })

    const backToDraft = resolvePluginWorkspaceHistoryNavigation({
      entries: ['help()', ':inspect Plugin.host', 'Plugin.host.order.get({ id: 1 })'],
      currentValue: next.value,
      index: next.index,
      draft: next.draft,
      direction: 'next',
    })

    expect(backToDraft).toEqual({
      index: -1,
      value: 'Plugin.host.user.get({ id: 2 })',
      draft: 'Plugin.host.user.get({ id: 2 })',
    })
  })

  test('only handles history navigation when the caret is on the first or last line', () => {
    expect(
      shouldHandlePluginWorkspaceHistoryNavigation({
        value: 'help()',
        selectionStart: 6,
        direction: 'previous',
      })
    ).toBe(true)

    expect(
      shouldHandlePluginWorkspaceHistoryNavigation({
        value: 'first line\nsecond line',
        selectionStart: 'first line\nsec'.length,
        direction: 'previous',
      })
    ).toBe(false)

    expect(
      shouldHandlePluginWorkspaceHistoryNavigation({
        value: 'first line\nsecond line',
        selectionStart: 'first line'.length,
        direction: 'previous',
      })
    ).toBe(true)

    expect(
      shouldHandlePluginWorkspaceHistoryNavigation({
        value: 'first line\nsecond line',
        selectionStart: 2,
        direction: 'next',
      })
    ).toBe(false)

    expect(
      shouldHandlePluginWorkspaceHistoryNavigation({
        value: 'first line\nsecond line',
        selectionStart: 'first line\nsecond line'.length,
        direction: 'next',
      })
    ).toBe(true)
  })
})
