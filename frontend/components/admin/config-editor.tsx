'use client'

import { useState, useEffect, useRef, useCallback } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Plus, X } from 'lucide-react'
import dynamic from 'next/dynamic'

const CodeMirror = dynamic(() => import('@uiw/react-codemirror'), { ssr: false })
const loadJsonLang = () => import('@codemirror/lang-json').then(m => m.json())

type ConfigEntry = { key: string; value: any }

function numToPlain(n: number): string {
  const s = String(n)
  if (!s.includes('e') && !s.includes('E')) return s
  return n.toFixed(20).replace(/\.?0+$/, '')
}

function detectType(v: any): 'string' | 'number' | 'boolean' {
  if (typeof v === 'boolean') return 'boolean'
  if (typeof v === 'number') return 'number'
  return 'string'
}

interface ConfigEditorLabels {
  configJson: string
  configFields: string
  jsonEditor: string
  visualEditor: string
  invalidJson: string
  noFields: string
  addField: string
}

export function ConfigEditor({
  value,
  onChange,
  flushRef,
  labels,
  cmTheme,
}: {
  value: string
  onChange: (v: string) => void
  flushRef?: React.MutableRefObject<(() => string | null) | null>
  labels: ConfigEditorLabels
  cmTheme?: 'light' | 'dark'
}) {
  const [entries, setEntries] = useState<ConfigEntry[]>([])
  const [rawMode, setRawMode] = useState(false)
  const [rawValue, setRawValue] = useState(value)
  const skipSync = useRef(false)
  const [rawNumbers, setRawNumbers] = useState<Record<number, string>>({})
  const debounceRef = useRef<ReturnType<typeof setTimeout>>()
  const onChangeRef = useRef(onChange)
  onChangeRef.current = onChange
  const pendingJsonRef = useRef<string | null>(null)
  const [jsonExtensions, setJsonExtensions] = useState<any[]>([])

  useEffect(() => {
    loadJsonLang().then(ext => setJsonExtensions([ext]))
  }, [])

  useEffect(() => () => { clearTimeout(debounceRef.current) }, [])

  // Expose flush method to parent
  useEffect(() => {
    if (flushRef) {
      flushRef.current = () => {
        clearTimeout(debounceRef.current)
        if (pendingJsonRef.current !== null) {
          const pending = pendingJsonRef.current
          onChangeRef.current(pending)
          pendingJsonRef.current = null
          return pending
        }
        return null
      }
    }
  }, [flushRef])

  // Parse JSON → entries when value changes externally
  useEffect(() => {
    if (skipSync.current) { skipSync.current = false; return }
    try {
      const obj = JSON.parse(value || '{}')
      setEntries(Object.entries(obj).map(([k, v]) => ({ key: k, value: v })))
      setRawValue(value)
    } catch {
      setRawMode(true)
      setRawValue(value)
    }
  }, [value])

  const sync = useCallback((next: ConfigEntry[]) => {
    setEntries(next)
    skipSync.current = true
    const lines = next.filter(e => e.key).map(e => {
      const v = typeof e.value === 'number' ? numToPlain(e.value) : JSON.stringify(e.value)
      return `  ${JSON.stringify(e.key)}: ${v}`
    })
    const json = lines.length ? '{\n' + lines.join(',\n') + '\n}' : '{}'
    setRawValue(json)
    pendingJsonRef.current = json
    clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(() => {
      onChangeRef.current(json)
      pendingJsonRef.current = null
    }, 300)
  }, [])

  const updateEntry = (i: number, field: 'key' | 'value', val: any) => {
    const next = [...entries]
    next[i] = { ...next[i], [field]: val }
    sync(next)
  }

  const removeEntry = (i: number) => sync(entries.filter((_, idx) => idx !== i))

  const addEntry = () => sync([...entries, { key: '', value: '' }])

  const changeType = (i: number, type: string) => {
    const next = [...entries]
    const cur = next[i].value
    if (type === 'boolean') next[i] = { ...next[i], value: cur === 'true' || cur === true }
    else if (type === 'number') next[i] = { ...next[i], value: Number(cur) || 0 }
    else next[i] = { ...next[i], value: String(cur) }
    sync(next)
  }

  // Raw JSON mode
  if (rawMode) {
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <Label>{labels.configJson}</Label>
          <Button variant="ghost" size="sm" onClick={() => {
            try {
              const obj = JSON.parse(rawValue || '{}')
              setEntries(Object.entries(obj).map(([k, v]) => ({ key: k, value: v })))
              skipSync.current = true
              onChange(rawValue)
              setRawMode(false)
            } catch {
              // stay in raw mode on invalid JSON
            }
          }}>
            {labels.visualEditor}
          </Button>
        </div>
        <CodeMirror
          value={rawValue}
          extensions={jsonExtensions}
          onChange={(v) => { setRawValue(v); onChange(v) }}
          height="250px"
          theme={cmTheme}
          className="rounded-md border overflow-hidden text-sm"
        />
      </div>
    )
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <Label>{labels.configFields}</Label>
        <Button variant="ghost" size="sm" onClick={() => setRawMode(true)}>
          {labels.jsonEditor}
        </Button>
      </div>

      {entries.length === 0 && (
        <p className="text-sm text-muted-foreground py-4 text-center">
          {labels.noFields}
        </p>
      )}

      <div className="space-y-2">
        {entries.map((entry, i) => {
          const type = detectType(entry.value)
          return (
            <div key={i} className="flex items-center gap-2 rounded-lg border p-2 bg-muted/30">
              <Input
                value={entry.key}
                onChange={(e) => updateEntry(i, 'key', e.target.value)}
                placeholder="key"
                className="font-mono text-sm w-[35%]"
              />
              <Select value={type} onValueChange={(v) => changeType(i, v)}>
                <SelectTrigger className="w-[90px] text-xs">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="string">String</SelectItem>
                  <SelectItem value="number">Number</SelectItem>
                  <SelectItem value="boolean">Bool</SelectItem>
                </SelectContent>
              </Select>
              {type === 'boolean' ? (
                <div className="flex-1 flex items-center gap-2 pl-2">
                  <Switch
                    checked={!!entry.value}
                    onCheckedChange={(v) => updateEntry(i, 'value', v)}
                  />
                  <span className="text-xs text-muted-foreground">{String(entry.value)}</span>
                </div>
              ) : type === 'number' ? (
                <Input
                  value={rawNumbers[i] ?? numToPlain(entry.value)}
                  onFocus={() => setRawNumbers(p => ({ ...p, [i]: numToPlain(entry.value) }))}
                  onChange={(e) => {
                    const raw = e.target.value
                    if (raw === '' || /^-?\d*\.?\d*$/.test(raw)) {
                      setRawNumbers(p => ({ ...p, [i]: raw }))
                      const n = Number(raw)
                      if (raw !== '' && !isNaN(n)) updateEntry(i, 'value', n)
                    }
                  }}
                  onBlur={() => setRawNumbers(p => { const { [i]: _, ...rest } = p; return rest })}
                  className="flex-1 font-mono text-sm"
                />
              ) : (
                <Input
                  value={String(entry.value ?? '')}
                  onChange={(e) => updateEntry(i, 'value', e.target.value)}
                  className="flex-1 font-mono text-sm"
                  placeholder="value"
                />
              )}
              <Button variant="ghost" size="icon" className="h-8 w-8 shrink-0" onClick={() => removeEntry(i)}>
                <X className="h-4 w-4" />
              </Button>
            </div>
          )
        })}
      </div>

      <Button variant="outline" size="sm" onClick={addEntry} className="w-full">
        <Plus className="h-4 w-4 mr-1" />
        {labels.addField}
      </Button>
    </div>
  )
}
