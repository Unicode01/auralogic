import {
  buildWorkspaceTranscriptBlocks,
  buildWorkspaceTerminalPlainText,
  buildWorkspaceTerminalScreen,
  shouldInterruptWorkspaceTerminalInputShortcut,
  type WorkspaceTranscriptBlock,
  type WorkspaceTranscriptSourceEntry,
} from './plugin-workspace-terminal'

function createBlock(
  text: string,
  overrides?: Partial<WorkspaceTranscriptBlock>
): WorkspaceTranscriptBlock {
  return {
    key: overrides?.key || text,
    kind: overrides?.kind || 'stdout',
    level: overrides?.level,
    text,
  }
}

function createEntry(
  message: string,
  overrides?: Partial<WorkspaceTranscriptSourceEntry>
): WorkspaceTranscriptSourceEntry {
  return {
    seq: overrides?.seq ?? 1,
    timestamp: overrides?.timestamp ?? '2026-03-23T00:00:00Z',
    channel: overrides?.channel ?? 'console',
    level: overrides?.level ?? 'info',
    message,
    source: overrides?.source ?? 'console.log',
    metadata: overrides?.metadata,
  }
}

describe('plugin-workspace-terminal', () => {
  it('preserves block boundaries as terminal lines', () => {
    const screen = buildWorkspaceTerminalScreen([
      createBlock('$ help', { kind: 'prompt' }),
      createBlock('available commands'),
    ])

    expect(screen.lines.map((line) => line.text)).toEqual(['$ help', 'available commands'])
    expect(buildWorkspaceTerminalPlainText([createBlock('$ help', { kind: 'prompt' }), createBlock('available commands')])).toBe(
      '$ help\navailable commands'
    )
  })

  it('overwrites the current line when receiving carriage returns', () => {
    const screen = buildWorkspaceTerminalScreen([
      createBlock('progress 10%\rprogress 90%\rprogress 100%\ncompleted'),
    ])

    expect(screen.lines.map((line) => line.text)).toEqual(['progress 100%', 'completed'])
  })

  it('applies ANSI colors and reset codes on the same line', () => {
    const screen = buildWorkspaceTerminalScreen([
      createBlock('plain \u001b[31mred\u001b[0m tail'),
    ])

    expect(screen.lines).toHaveLength(1)
    expect(screen.lines[0]?.segments.map((segment) => segment.text)).toEqual(['plain ', 'red', ' tail'])
    expect(screen.lines[0]?.segments[0]?.style?.color).toBe('#f5f5f5')
    expect(screen.lines[0]?.segments[1]?.style?.color).toBe('#f87171')
    expect(screen.lines[0]?.segments[2]?.style?.color).toBe('#f5f5f5')
  })

  it('supports clear screen and cursor home sequences', () => {
    const screen = buildWorkspaceTerminalScreen([
      createBlock('first line\nsecond line\u001b[2J\u001b[Hready'),
    ])

    expect(screen.lines.map((line) => line.text)).toEqual(['ready'])
  })

  it('supports erase line to end for in-place redraws', () => {
    const screen = buildWorkspaceTerminalScreen([
      createBlock('loading package\rready\u001b[K'),
    ])

    expect(screen.lines.map((line) => line.text)).toEqual(['ready'])
  })

  it('keeps structured console preview blocks separated', () => {
    const blocks = buildWorkspaceTranscriptBlocks([
      createEntry('a', {
        seq: 1,
        metadata: {
          workspace_console_previews_json:
            '[{"type":"string","summary":"a","value":"a","keys":[],"entries":[],"truncated":false}]',
        },
      }),
      createEntry('a', {
        seq: 2,
        metadata: {
          workspace_console_previews_json:
            '[{"type":"string","summary":"a","value":"a","keys":[],"entries":[],"truncated":false}]',
        },
      }),
      createEntry('a', {
        seq: 3,
        metadata: {
          workspace_console_previews_json:
            '[{"type":"string","summary":"a","value":"a","keys":[],"entries":[],"truncated":false}]',
        },
      }),
    ])

    expect(blocks).toHaveLength(3)
    expect(blocks.map((block) => block.text)).toEqual(['a', 'a', 'a'])
  })

  it('still merges plain console blocks without structured preview metadata', () => {
    const blocks = buildWorkspaceTranscriptBlocks([
      createEntry('a', { seq: 1 }),
      createEntry('a', { seq: 2 }),
      createEntry('a', { seq: 3 }),
    ])

    expect(blocks).toHaveLength(1)
    expect(blocks[0]?.text).toBe('a\na\na')
  })

  it('allows Ctrl+C interrupt when a task is active and nothing is selected', () => {
    expect(
      shouldInterruptWorkspaceTerminalInputShortcut({
        hasCancelableTask: true,
        hasSelection: false,
      })
    ).toBe(true)
  })

  it('keeps native copy behavior when terminal input text is selected', () => {
    expect(
      shouldInterruptWorkspaceTerminalInputShortcut({
        hasCancelableTask: true,
        hasSelection: true,
      })
    ).toBe(false)
  })

  it('does not interrupt when there is no active task', () => {
    expect(
      shouldInterruptWorkspaceTerminalInputShortcut({
        hasCancelableTask: false,
        hasSelection: false,
      })
    ).toBe(false)
  })
})
