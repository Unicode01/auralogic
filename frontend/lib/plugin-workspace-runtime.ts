export type WorkspaceRuntimeConsoleCommand = {
  action: 'eval' | 'inspect'
  expression: string
  depth: number
}

const DEFAULT_WORKSPACE_RUNTIME_INSPECT_DEPTH = 2
const MAX_WORKSPACE_RUNTIME_INSPECT_DEPTH = 4

export function normalizeWorkspaceRuntimeInspectDepth(value?: number): number {
  if (!value || !Number.isFinite(value)) {
    return DEFAULT_WORKSPACE_RUNTIME_INSPECT_DEPTH
  }
  const normalized = Math.trunc(value)
  if (normalized <= 0) {
    return DEFAULT_WORKSPACE_RUNTIME_INSPECT_DEPTH
  }
  if (normalized > MAX_WORKSPACE_RUNTIME_INSPECT_DEPTH) {
    return MAX_WORKSPACE_RUNTIME_INSPECT_DEPTH
  }
  return normalized
}

export function parseWorkspaceRuntimeConsoleLine(line: string): WorkspaceRuntimeConsoleCommand {
  const raw = String(line || '').replace(/[\r\n]+$/, '')
  const trimmed = raw.trim()
  if (!trimmed) {
    return {
      action: 'eval',
      expression: '',
      depth: DEFAULT_WORKSPACE_RUNTIME_INSPECT_DEPTH,
    }
  }

  const inspectMatch = trimmed.match(
    /^:inspect(?:\s+--depth(?:=|\s+)(\d+))?(?:\s+(.*))?$/i
  )
  if (!inspectMatch) {
    return {
      action: 'eval',
      expression: raw,
      depth: DEFAULT_WORKSPACE_RUNTIME_INSPECT_DEPTH,
    }
  }

  return {
    action: 'inspect',
    expression: String(inspectMatch[2] || '').trim() || 'globalThis',
    depth: normalizeWorkspaceRuntimeInspectDepth(
      inspectMatch[1] ? Number.parseInt(inspectMatch[1], 10) : undefined
    ),
  }
}
