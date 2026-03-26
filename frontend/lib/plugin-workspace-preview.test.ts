import {
  extractWorkspaceConsolePreviewsFromMetadata,
  extractWorkspaceRuntimePreviewFromMetadata,
  parseWorkspaceRuntimePreview,
} from './plugin-workspace-preview'

describe('plugin-workspace-preview', () => {
  test('parses runtime preview objects with nested entries', () => {
    const preview = parseWorkspaceRuntimePreview({
      type: 'object',
      summary: '{ a: "x" }',
      entries: [
        {
          key: 'a',
          value: {
            type: 'string',
            summary: '"x"',
            value: 'x',
          },
        },
      ],
    })

    expect(preview?.type).toBe('object')
    expect(preview?.entries).toHaveLength(1)
    expect(preview?.entries[0]?.key).toBe('a')
    expect(preview?.entries[0]?.value.value).toBe('x')
  })

  test('extracts runtime preview from metadata json', () => {
    const preview = extractWorkspaceRuntimePreviewFromMetadata({
      workspace_runtime_preview_json:
        '{"type":"number","summary":"5","value":5,"keys":[],"entries":[],"truncated":false}',
    })

    expect(preview).toMatchObject({
      type: 'number',
      summary: '5',
      value: 5,
    })
  })

  test('extracts console previews array from metadata json', () => {
    const previews = extractWorkspaceConsolePreviewsFromMetadata({
      workspace_console_previews_json:
        '[{"type":"string","summary":"a","value":"a","keys":[],"entries":[],"truncated":false},{"type":"number","summary":"2","value":2,"keys":[],"entries":[],"truncated":false}]',
    })

    expect(previews).toHaveLength(2)
    expect(previews[0]?.value).toBe('a')
    expect(previews[1]?.value).toBe(2)
  })

  test('returns empty results for invalid metadata payloads', () => {
    expect(
      extractWorkspaceRuntimePreviewFromMetadata({
        workspace_runtime_preview_json: '{bad json}',
      })
    ).toBeNull()
    expect(
      extractWorkspaceConsolePreviewsFromMetadata({
        workspace_console_previews_json: '{bad json}',
      })
    ).toEqual([])
  })
})
