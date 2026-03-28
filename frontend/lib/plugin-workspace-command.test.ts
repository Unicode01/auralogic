import {
  normalizePluginWorkspaceShellCommandName,
  resolvePluginWorkspaceSubmission,
} from './plugin-workspace-command'

describe('plugin-workspace-command', () => {
  test('normalizes builtin aliases', () => {
    expect(normalizePluginWorkspaceShellCommandName('help')).toBe('builtin/help')
    expect(normalizePluginWorkspaceShellCommandName('kv.list')).toBe('builtin/kv.list')
    expect(normalizePluginWorkspaceShellCommandName('builtin/ls')).toBe('builtin/ls')
  })

  test('routes builtin commands through terminal line execution', () => {
    expect(resolvePluginWorkspaceSubmission('help')).toEqual({
      mode: 'terminal_line',
      line: 'help',
    })
    expect(resolvePluginWorkspaceSubmission('ls | grep "index"')).toEqual({
      mode: 'terminal_line',
      line: 'ls | grep "index"',
    })
  })

  test('routes inspect commands through runtime inspect execution', () => {
    expect(resolvePluginWorkspaceSubmission(':inspect --depth=3 Plugin.host')).toEqual({
      mode: 'runtime_inspect',
      line: 'Plugin.host',
      depth: 3,
    })
  })

  test('keeps normal expressions on runtime eval path', () => {
    expect(resolvePluginWorkspaceSubmission('help()')).toEqual({
      mode: 'runtime_eval',
      line: 'help()',
    })
    expect(resolvePluginWorkspaceSubmission('globalThis.counter += 1')).toEqual({
      mode: 'runtime_eval',
      line: 'globalThis.counter += 1',
    })
  })
})
