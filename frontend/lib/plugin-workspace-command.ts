import type { AdminPluginWorkspaceCommand } from '@/lib/api'
import { parseWorkspaceRuntimeConsoleLine } from '@/lib/plugin-workspace-runtime'

export type PluginWorkspaceResolvedSubmission =
  | {
      mode: 'noop'
    }
  | {
      mode: 'terminal_line'
      line: string
    }
  | {
      mode: 'runtime_eval'
      line: string
    }
  | {
      mode: 'runtime_inspect'
      line: string
      depth: number
    }

const WORKSPACE_SHELL_COMMAND_ALIAS_MAP: Record<string, string> = {
  '?': 'builtin/help',
  help: 'builtin/help',
  clear: 'builtin/clear',
  'log.tail': 'builtin/log.tail',
  pwd: 'builtin/pwd',
  ls: 'builtin/ls',
  stat: 'builtin/stat',
  cat: 'builtin/cat',
  mkdir: 'builtin/mkdir',
  find: 'builtin/find',
  grep: 'builtin/grep',
  'kv.get': 'builtin/kv.get',
  'kv.set': 'builtin/kv.set',
  'kv.list': 'builtin/kv.list',
  'kv.del': 'builtin/kv.del',
}

function extractWorkspaceCommandToken(line: string): string {
  const trimmed = String(line || '').trim()
  if (!trimmed) {
    return ''
  }
  const match = trimmed.match(/^[^\s;|&]+/)
  return String(match?.[0] || '').trim()
}

export function normalizePluginWorkspaceShellCommandName(raw: string): string {
  const trimmed = String(raw || '').trim()
  if (!trimmed) {
    return ''
  }
  const lowered = trimmed.toLowerCase()
  return WORKSPACE_SHELL_COMMAND_ALIAS_MAP[lowered] || trimmed
}

function buildWorkspaceShellCommandNameSet(
  workspaceCommands?: AdminPluginWorkspaceCommand[] | null
): Set<string> {
  const out = new Set<string>(Object.values(WORKSPACE_SHELL_COMMAND_ALIAS_MAP))
  ;(workspaceCommands || []).forEach((command) => {
    const normalized = normalizePluginWorkspaceShellCommandName(command?.name || '')
    if (normalized) {
      out.add(normalized)
    }
  })
  return out
}

export function resolvePluginWorkspaceSubmission(
  line: string,
  workspaceCommands?: AdminPluginWorkspaceCommand[] | null
): PluginWorkspaceResolvedSubmission {
  const raw = String(line || '').replace(/[\r\n]+$/, '')
  const trimmed = raw.trim()
  if (!trimmed) {
    return { mode: 'noop' }
  }

  const parsedRuntimeCommand = parseWorkspaceRuntimeConsoleLine(raw)
  if (parsedRuntimeCommand.action === 'inspect') {
    return {
      mode: 'runtime_inspect',
      line: parsedRuntimeCommand.expression,
      depth: parsedRuntimeCommand.depth,
    }
  }

  const commandToken = extractWorkspaceCommandToken(trimmed)
  const normalizedCommand = normalizePluginWorkspaceShellCommandName(commandToken)
  const knownWorkspaceCommands = buildWorkspaceShellCommandNameSet(workspaceCommands)
  if (normalizedCommand && knownWorkspaceCommands.has(normalizedCommand)) {
    return {
      mode: 'terminal_line',
      line: raw,
    }
  }

  return {
    mode: 'runtime_eval',
    line: raw,
  }
}
