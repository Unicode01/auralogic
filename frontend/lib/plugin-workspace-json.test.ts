import {
  formatWorkspaceJSONOutput,
  tryParseWorkspaceJSONOutput,
} from './plugin-workspace-json'

describe('plugin-workspace-json', () => {
  test('parses plain object output', () => {
    const parsed = tryParseWorkspaceJSONOutput('{"ok":true,"count":2}')

    expect(parsed).toMatchObject({
      raw: '{"ok":true,"count":2}',
      preview: {
        type: 'object',
        keys: ['ok', 'count'],
      },
    })
    expect(parsed?.preview.summary).toContain('ok: true')
    expect(parsed?.preview.summary).toContain('count: 2')
  })

  test('parses runtime-prefixed json output', () => {
    const parsed = tryParseWorkspaceJSONOutput('< {"items":[1,2,3]}')

    expect(parsed).not.toBeNull()
    expect(parsed?.preview.type).toBe('object')
    expect(parsed?.preview.summary).toContain('items: [1, 2, 3]')
  })

  test('parses nested stringified json output', () => {
    const parsed = tryParseWorkspaceJSONOutput('"{\\"user\\":{\\"id\\":1}}"')

    expect(parsed).not.toBeNull()
    expect(parsed?.preview.type).toBe('object')
    expect(parsed?.preview.summary).toContain('user: { id: 1 }')
  })

  test('ignores non-json output', () => {
    expect(tryParseWorkspaceJSONOutput('help()')).toBeNull()
    expect(tryParseWorkspaceJSONOutput('< { topic: "console" }')).toBeNull()
  })

  test('formats parsed json for raw view', () => {
    expect(formatWorkspaceJSONOutput('{"ok":true,"nested":{"count":2}}')).toBe(
      '{\n  "ok": true,\n  "nested": {\n    "count": 2\n  }\n}'
    )
  })
})
