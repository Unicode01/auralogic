import type { CSSProperties } from 'react'

import { tryParseWorkspaceJSONOutput } from './plugin-workspace-json'

export type WorkspaceTranscriptKind =
  | 'command'
  | 'prompt'
  | 'stdin'
  | 'stdout'
  | 'stderr'
  | 'system'
  | 'log'

export type WorkspaceTranscriptBlock = {
  key: string
  kind: WorkspaceTranscriptKind
  text: string
  level?: string
  source?: string
  metadata?: Record<string, string>
}

export type WorkspaceTranscriptSourceEntry = {
  seq?: number | string
  timestamp?: string
  channel?: string
  level?: string
  message?: string
  source?: string
  metadata?: Record<string, string>
}

export type WorkspaceTerminalSegment = {
  key: string
  text: string
  style?: CSSProperties
}

export type WorkspaceTerminalLine = {
  key: string
  text: string
  segments: WorkspaceTerminalSegment[]
}

export type WorkspaceTerminalScreen = {
  lines: WorkspaceTerminalLine[]
}

export function shouldInterruptWorkspaceTerminalInputShortcut(params: {
  hasCancelableTask: boolean
  hasSelection: boolean
}): boolean {
  return params.hasCancelableTask && !params.hasSelection
}

type TerminalStyle = {
  color?: string
  backgroundColor?: string
  fontWeight?: CSSProperties['fontWeight']
  textDecorationLine?: CSSProperties['textDecorationLine']
  opacity?: number
}

type TerminalCell = {
  char: string
  style: TerminalStyle
}

type TerminalLineBuffer = TerminalCell[]

type TerminalAnsiState = {
  foreground?: string
  background?: string
  bold?: boolean
  underline?: boolean
  dim?: boolean
  inverse?: boolean
}

const ANSI_FOREGROUND_COLORS: Record<number, string> = {
  30: '#171717',
  31: '#f87171',
  32: '#4ade80',
  33: '#facc15',
  34: '#60a5fa',
  35: '#c084fc',
  36: '#22d3ee',
  37: '#f5f5f5',
  90: '#525252',
  91: '#fca5a5',
  92: '#86efac',
  93: '#fde047',
  94: '#93c5fd',
  95: '#d8b4fe',
  96: '#67e8f9',
  97: '#ffffff',
}

const ANSI_BACKGROUND_COLORS: Record<number, string> = {
  40: '#171717',
  41: '#7f1d1d',
  42: '#14532d',
  43: '#713f12',
  44: '#1d4ed8',
  45: '#701a75',
  46: '#155e75',
  47: '#f5f5f5',
  100: '#404040',
  101: '#b91c1c',
  102: '#15803d',
  103: '#a16207',
  104: '#2563eb',
  105: '#a21caf',
  106: '#0891b2',
  107: '#ffffff',
}

function resolveWorkspaceLevelTextColor(level?: string): string | undefined {
  switch (
    String(level || '')
      .trim()
      .toLowerCase()
  ) {
    case 'error':
      return '#fda4af'
    case 'warn':
    case 'warning':
      return '#fcd34d'
    case 'debug':
      return '#7dd3fc'
    default:
      return '#f5f5f5'
  }
}

function resolveWorkspaceTranscriptBaseStyle(
  kind: WorkspaceTranscriptKind,
  level?: string
): TerminalStyle {
  switch (kind) {
    case 'command':
      return { color: '#bae6fd' }
    case 'prompt':
      return { color: '#a7f3d0', fontWeight: 600 }
    case 'stdin':
      return { color: '#fcd34d' }
    case 'stderr':
      return { color: '#fda4af' }
    case 'system':
      return { color: '#737373' }
    case 'stdout':
      return { color: level === 'warn' ? '#fde68a' : '#f5f5f5' }
    default:
      return { color: resolveWorkspaceLevelTextColor(level) }
  }
}

function cloneStyle(style?: TerminalStyle): TerminalStyle {
  if (!style) {
    return {}
  }
  return {
    color: style.color,
    backgroundColor: style.backgroundColor,
    fontWeight: style.fontWeight,
    textDecorationLine: style.textDecorationLine,
    opacity: style.opacity,
  }
}

function stylesEqual(left?: TerminalStyle, right?: TerminalStyle): boolean {
  return (
    (left?.color || '') === (right?.color || '') &&
    (left?.backgroundColor || '') === (right?.backgroundColor || '') &&
    (left?.fontWeight || '') === (right?.fontWeight || '') &&
    (left?.textDecorationLine || '') === (right?.textDecorationLine || '') &&
    (left?.opacity || '') === (right?.opacity || '')
  )
}

function ensureTerminalLine(lines: TerminalLineBuffer[], row: number): TerminalLineBuffer {
  while (lines.length <= row) {
    lines.push([])
  }
  return lines[row] || []
}

function writeTerminalCharacter(
  lines: TerminalLineBuffer[],
  row: number,
  column: number,
  char: string,
  style: TerminalStyle
): number {
  const line = ensureTerminalLine(lines, row)
  while (line.length < column) {
    line.push({ char: ' ', style: {} })
  }
  line[column] = {
    char,
    style: cloneStyle(style),
  }
  return column + 1
}

function clearTerminalLineToEnd(lines: TerminalLineBuffer[], row: number, column: number): void {
  const line = ensureTerminalLine(lines, row)
  line.splice(column)
}

function clearTerminalLineFromStart(
  lines: TerminalLineBuffer[],
  row: number,
  column: number
): void {
  const line = ensureTerminalLine(lines, row)
  const limit = Math.min(column + 1, line.length)
  for (let index = 0; index < limit; index += 1) {
    line[index] = { char: ' ', style: {} }
  }
}

function clearTerminalLine(lines: TerminalLineBuffer[], row: number): void {
  ensureTerminalLine(lines, row)
  lines[row] = []
}

function resolveAnsiStyle(baseStyle: TerminalStyle, state: TerminalAnsiState): TerminalStyle {
  const style: TerminalStyle = cloneStyle(baseStyle)
  const foreground = state.foreground
  const background = state.background
  if (state.inverse) {
    if (background) {
      style.color = background
    } else {
      delete style.color
    }
    if (foreground) {
      style.backgroundColor = foreground
    } else {
      delete style.backgroundColor
    }
  } else {
    if (foreground) {
      style.color = foreground
    }
    if (background) {
      style.backgroundColor = background
    }
  }
  if (state.bold) {
    style.fontWeight = 700
  }
  if (state.underline) {
    style.textDecorationLine = 'underline'
  }
  if (state.dim) {
    style.opacity = 0.72
  }
  return style
}

function stripUnsupportedPrefix(raw: string): string {
  if (!raw) {
    return raw
  }
  if (raw.startsWith('?') || raw.startsWith('>')) {
    return raw.slice(1)
  }
  return raw
}

function parseAnsiParameters(raw: string): number[] {
  const normalized = stripUnsupportedPrefix(raw)
  if (!normalized.trim()) {
    return []
  }
  return normalized.split(';').map((value) => {
    const trimmed = value.trim()
    if (!trimmed) {
      return 0
    }
    const parsed = Number.parseInt(trimmed, 10)
    return Number.isNaN(parsed) ? 0 : parsed
  })
}

function applyAnsiSgrCodes(state: TerminalAnsiState, rawParams: string): TerminalAnsiState {
  const nextState: TerminalAnsiState = { ...state }
  const params = parseAnsiParameters(rawParams)
  const codes = params.length > 0 ? params : [0]

  for (let index = 0; index < codes.length; index += 1) {
    const code = codes[index] || 0
    switch (code) {
      case 0:
        nextState.foreground = undefined
        nextState.background = undefined
        nextState.bold = false
        nextState.underline = false
        nextState.dim = false
        nextState.inverse = false
        break
      case 1:
        nextState.bold = true
        break
      case 2:
        nextState.dim = true
        break
      case 4:
        nextState.underline = true
        break
      case 7:
        nextState.inverse = true
        break
      case 22:
        nextState.bold = false
        nextState.dim = false
        break
      case 24:
        nextState.underline = false
        break
      case 27:
        nextState.inverse = false
        break
      case 39:
        nextState.foreground = undefined
        break
      case 49:
        nextState.background = undefined
        break
      default:
        if (ANSI_FOREGROUND_COLORS[code]) {
          nextState.foreground = ANSI_FOREGROUND_COLORS[code]
          break
        }
        if (ANSI_BACKGROUND_COLORS[code]) {
          nextState.background = ANSI_BACKGROUND_COLORS[code]
          break
        }
        break
    }
  }

  return nextState
}

function skipOscSequence(text: string, startIndex: number): number {
  let cursor = startIndex + 2
  while (cursor < text.length) {
    const current = text[cursor]
    if (current === '\u0007') {
      return cursor
    }
    if (current === '\u001b' && text[cursor + 1] === '\\') {
      return cursor + 1
    }
    cursor += 1
  }
  return startIndex
}

function shouldInsertBlockSeparator(block: WorkspaceTranscriptBlock, next?: WorkspaceTranscriptBlock): boolean {
  if (!next) {
    return false
  }
  return !block.text.endsWith('\n')
}

export function resolveWorkspaceTranscriptKind(
  entry: WorkspaceTranscriptSourceEntry
): WorkspaceTranscriptKind {
  const source = String(entry.source || '')
    .trim()
    .toLowerCase()
  const channel = String(entry.channel || '')
    .trim()
    .toLowerCase()
  const level = String(entry.level || '')
    .trim()
    .toLowerCase()

  if (source === 'host.workspace.command' || channel === 'command') {
    return 'command'
  }
  if (channel === 'prompt') {
    return 'prompt'
  }
  if (source === 'host.workspace.stdin' || channel === 'stdin' || channel === 'input') {
    return 'stdin'
  }
  if (channel === 'stderr' || level === 'error') {
    return 'stderr'
  }
  if (channel === 'stdout' || channel === 'console' || channel === 'workspace') {
    return 'stdout'
  }
  if (source.startsWith('host.workspace.') || channel === 'system') {
    return 'system'
  }
  return 'log'
}

function workspaceTranscriptText(
  entry: WorkspaceTranscriptSourceEntry,
  kind: WorkspaceTranscriptKind
): string {
  const message = String(entry.message || '')
  if (!message) {
    return ''
  }
  if (kind === 'system') {
    const trimmed = message.trimEnd()
    return trimmed.startsWith('#') ? trimmed : `# ${trimmed}`
  }
  if (kind === 'stdin') {
    const channel = String(entry.channel || '')
      .trim()
      .toLowerCase()
    if (channel === 'input') {
      return message
    }
    return message.trimEnd()
  }
  if (kind === 'command' || kind === 'prompt') {
    return message.trimEnd()
  }
  return message
}

function appendWorkspaceTranscriptText(current: string, next: string): string {
  if (!current) {
    return next
  }
  if (!next) {
    return current
  }
  if (current.endsWith('\n') || next.startsWith('\n')) {
    return current + next
  }
  return `${current}\n${next}`
}

function hasWorkspaceStructuredPreviewMetadata(metadata?: Record<string, string>): boolean {
  const record = metadata || {}
  return Boolean(
    String(record.workspace_console_previews_json || '').trim() ||
      String(record.workspace_runtime_preview_json || '').trim()
  )
}

function canMergeWorkspaceTranscriptBlock(
  current: WorkspaceTranscriptBlock | undefined,
  nextKind: WorkspaceTranscriptKind,
  nextLevel?: string
): boolean {
  if (!current) {
    return false
  }
  switch (nextKind) {
    case 'stdout':
    case 'stderr':
    case 'system':
    case 'log':
      return current.kind === nextKind && (current.level || '') === (nextLevel || '')
    default:
      return false
  }
}

export function buildWorkspaceTranscriptBlocks(
  entries: WorkspaceTranscriptSourceEntry[]
): WorkspaceTranscriptBlock[] {
  const blocks: WorkspaceTranscriptBlock[] = []
  for (let index = 0; index < entries.length; index += 1) {
    const entry = entries[index]
    const kind = resolveWorkspaceTranscriptKind(entry)
    if (kind === 'prompt') {
      const nextEntry = entries[index + 1]
      const nextChannel = String(nextEntry?.channel || '')
        .trim()
        .toLowerCase()
      if (nextEntry && nextChannel === 'input') {
        const promptText = workspaceTranscriptText(entry, kind)
        const inputText = workspaceTranscriptText(nextEntry, 'stdin')
        const combinedText = `${promptText}${inputText}`
        if (combinedText) {
          blocks.push({
            key: `${entry.seq || index}-${nextEntry.seq || index + 1}`,
            kind: 'prompt',
            text: combinedText,
            level: entry.level,
            source: entry.source,
            metadata: entry.metadata,
          })
        }
        index += 1
        continue
      }
    }

    const text = workspaceTranscriptText(entry, kind)
    if (!text) {
      continue
    }
    const current = blocks[blocks.length - 1]
    if (
      canMergeWorkspaceTranscriptBlock(current, kind, entry.level) &&
      !hasWorkspaceStructuredPreviewMetadata(current?.metadata) &&
      !hasWorkspaceStructuredPreviewMetadata(entry.metadata) &&
      !String(current?.source || '')
        .trim()
        .toLowerCase()
        .startsWith('host.workspace.runtime.') &&
      !String(entry.source || '')
        .trim()
        .toLowerCase()
        .startsWith('host.workspace.runtime.') &&
      !tryParseWorkspaceJSONOutput(current?.text || '') &&
      !tryParseWorkspaceJSONOutput(text)
    ) {
      current.text = appendWorkspaceTranscriptText(current.text, text)
      current.level = entry.level || current.level
      continue
    }
    blocks.push({
      key: `${entry.seq || index}-${entry.timestamp || ''}`,
      kind,
      text,
      level: entry.level,
      source: entry.source,
      metadata: entry.metadata,
    })
  }
  return blocks
}

export function buildWorkspaceTerminalScreen(
  blocks: WorkspaceTranscriptBlock[]
): WorkspaceTerminalScreen {
  const lines: TerminalLineBuffer[] = []
  let row = 0
  let column = 0

  const writeText = (text: string, baseStyle: TerminalStyle) => {
    let ansiState: TerminalAnsiState = {}
    let index = 0
    while (index < text.length) {
      const current = text[index]
      if (current === '\u001b') {
        const next = text[index + 1]
        if (next === '[') {
          let cursor = index + 2
          while (cursor < text.length && !/[@-~]/.test(text[cursor] || '')) {
            cursor += 1
          }
          if (cursor >= text.length) {
            index += 1
            continue
          }
          const finalChar = text[cursor] || ''
          const rawParams = text.slice(index + 2, cursor)
          const params = parseAnsiParameters(rawParams)
          switch (finalChar) {
            case 'm':
              ansiState = applyAnsiSgrCodes(ansiState, rawParams)
              break
            case 'A':
              row = Math.max(row - (params[0] || 1), 0)
              ensureTerminalLine(lines, row)
              break
            case 'B':
              row = Math.max(row + (params[0] || 1), 0)
              ensureTerminalLine(lines, row)
              break
            case 'C':
              column = Math.max(column + (params[0] || 1), 0)
              break
            case 'D':
              column = Math.max(column - (params[0] || 1), 0)
              break
            case 'G':
              column = Math.max((params[0] || 1) - 1, 0)
              break
            case 'H':
            case 'f': {
              const targetRow = Math.max((params[0] || 1) - 1, 0)
              const targetColumn = Math.max((params[1] || 1) - 1, 0)
              row = targetRow
              column = targetColumn
              ensureTerminalLine(lines, row)
              break
            }
            case 'J':
              if ((params[0] || 0) === 2) {
                lines.splice(0, lines.length)
                row = 0
                column = 0
              }
              break
            case 'K': {
              const mode = params[0] || 0
              if (mode === 2) {
                clearTerminalLine(lines, row)
              } else if (mode === 1) {
                clearTerminalLineFromStart(lines, row, column)
              } else {
                clearTerminalLineToEnd(lines, row, column)
              }
              break
            }
            default:
              break
          }
          index = cursor + 1
          continue
        }
        if (next === ']') {
          const skippedIndex = skipOscSequence(text, index)
          if (skippedIndex !== index) {
            index = skippedIndex + 1
            continue
          }
        }
        index += 1
        continue
      }

      if (current === '\r') {
        column = 0
        index += 1
        continue
      }
      if (current === '\n') {
        row += 1
        column = 0
        ensureTerminalLine(lines, row)
        index += 1
        continue
      }
      if (current === '\b') {
        column = Math.max(column - 1, 0)
        index += 1
        continue
      }
      if (current === '\t') {
        const tabOffset = column % 4
        const nextTabStop = column + (tabOffset === 0 ? 4 : 4 - tabOffset)
        while (column < nextTabStop) {
          column = writeTerminalCharacter(
            lines,
            row,
            column,
            ' ',
            resolveAnsiStyle(baseStyle, ansiState)
          )
        }
        index += 1
        continue
      }

      column = writeTerminalCharacter(
        lines,
        row,
        column,
        current,
        resolveAnsiStyle(baseStyle, ansiState)
      )
      index += 1
    }
  }

  blocks.forEach((block, index) => {
    const baseStyle = resolveWorkspaceTranscriptBaseStyle(block.kind, block.level)
    writeText(block.text, baseStyle)
    if (shouldInsertBlockSeparator(block, blocks[index + 1])) {
      row += 1
      column = 0
      ensureTerminalLine(lines, row)
    }
  })

  const renderedLines = lines.map((line, lineIndex) => {
    if (!line || line.length === 0) {
      return {
        key: `line-${lineIndex}`,
        text: '',
        segments: [],
      }
    }

    const segments: WorkspaceTerminalSegment[] = []
    let currentText = ''
    let currentStyle: TerminalStyle | undefined

    line.forEach((cell) => {
      if (!currentText) {
        currentText = cell.char
        currentStyle = cell.style
        return
      }
      if (stylesEqual(currentStyle, cell.style)) {
        currentText += cell.char
        return
      }
      segments.push({
        key: `line-${lineIndex}-segment-${segments.length}`,
        text: currentText,
        style: currentStyle,
      })
      currentText = cell.char
      currentStyle = cell.style
    })

    if (currentText) {
      segments.push({
        key: `line-${lineIndex}-segment-${segments.length}`,
        text: currentText,
        style: currentStyle,
      })
    }

    return {
      key: `line-${lineIndex}`,
      text: line.map((cell) => cell.char).join(''),
      segments,
    }
  })

  return {
    lines: renderedLines,
  }
}

export function buildWorkspaceTerminalPlainText(blocks: WorkspaceTranscriptBlock[]): string {
  return buildWorkspaceTerminalScreen(blocks)
    .lines.map((line) => line.text)
    .join('\n')
    .trimEnd()
}
