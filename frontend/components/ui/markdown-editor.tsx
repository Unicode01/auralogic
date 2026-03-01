'use client'

import { useRef, useCallback, useState, useEffect } from 'react'
import CodeMirror from '@uiw/react-codemirror'
import { markdown } from '@codemirror/lang-markdown'
import { EditorView } from '@codemirror/view'
import { useTheme } from '@/contexts/theme-context'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import {
  Bold, Italic, Strikethrough, Heading1, Heading2, Heading3,
  Code, FileCode, Link, Image, List, ListOrdered, Quote, Palette, X, Pipette,
} from 'lucide-react'

interface MarkdownEditorProps {
  value: string
  onChange: (value: string) => void
  height?: string
  fill?: boolean
  theme?: 'light' | 'dark'
  className?: string
  placeholder?: string
}

type WrapAction = { type: 'wrap'; before: string; after: string }
type LineAction = { type: 'line'; prefix: string }
type BlockAction = { type: 'block'; before: string; after: string }
type Action = WrapAction | LineAction | BlockAction

const actions: { icon: typeof Bold; action: Action; key: string }[] = [
  { icon: Bold, action: { type: 'wrap', before: '**', after: '**' }, key: 'bold' },
  { icon: Italic, action: { type: 'wrap', before: '*', after: '*' }, key: 'italic' },
  { icon: Strikethrough, action: { type: 'wrap', before: '~~', after: '~~' }, key: 'strikethrough' },
  { icon: null as any, action: null as any, key: 'sep1' },
  { icon: Heading1, action: { type: 'line', prefix: '# ' }, key: 'heading1' },
  { icon: Heading2, action: { type: 'line', prefix: '## ' }, key: 'heading2' },
  { icon: Heading3, action: { type: 'line', prefix: '### ' }, key: 'heading3' },
  { icon: null as any, action: null as any, key: 'sep2' },
  { icon: Code, action: { type: 'wrap', before: '`', after: '`' }, key: 'code' },
  { icon: FileCode, action: { type: 'block', before: '```\n', after: '\n```' }, key: 'codeBlock' },
  { icon: null as any, action: null as any, key: 'sep3' },
  { icon: Link, action: { type: 'wrap', before: '[', after: '](url)' }, key: 'link' },
  { icon: Image, action: { type: 'wrap', before: '![', after: '](url)' }, key: 'image' },
  { icon: null as any, action: null as any, key: 'sep4' },
  { icon: List, action: { type: 'line', prefix: '- ' }, key: 'unorderedList' },
  { icon: ListOrdered, action: { type: 'line', prefix: '1. ' }, key: 'orderedList' },
  { icon: Quote, action: { type: 'line', prefix: '> ' }, key: 'quote' },
]

// 预设颜色色板 - 按色相(列)和明暗(行)组织
// 每列: 红、橙、黄、绿、青、蓝、紫、粉
const COLOR_PALETTE = [
  // 浅色
  ['#fca5a5', '#fdba74', '#fde047', '#86efac', '#67e8f9', '#93c5fd', '#c4b5fd', '#f9a8d4'],
  // 中等
  ['#f87171', '#fb923c', '#facc15', '#4ade80', '#22d3ee', '#60a5fa', '#a78bfa', '#f472b6'],
  // 标准
  ['#ef4444', '#f97316', '#eab308', '#22c55e', '#06b6d4', '#3b82f6', '#8b5cf6', '#ec4899'],
  // 深色
  ['#b91c1c', '#c2410c', '#a16207', '#15803d', '#0e7490', '#1d4ed8', '#6d28d9', '#be185d'],
  // 更深
  ['#7f1d1d', '#7c2d12', '#713f12', '#14532d', '#164e63', '#1e3a5f', '#4c1d95', '#831843'],
]

// 中性色
const NEUTRAL_COLORS = [
  '#000000', '#374151', '#6b7280', '#9ca3af',
  '#d1d5db', '#e5e7eb', '#f3f4f6', '#ffffff',
]

export function MarkdownEditor({ value, onChange, height = '400px', fill, theme, className, placeholder }: MarkdownEditorProps) {
  const viewRef = useRef<EditorView | null>(null)
  const { resolvedTheme } = useTheme()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const resolvedEditorTheme = theme ?? (resolvedTheme === 'dark' ? 'dark' : 'light')

  const [colorPickerOpen, setColorPickerOpen] = useState(false)
  const [customHex, setCustomHex] = useState('#')
  const colorPickerRef = useRef<HTMLDivElement>(null)
  const nativePickerRef = useRef<HTMLInputElement>(null)

  // 点击外部关闭颜色选择器
  useEffect(() => {
    if (!colorPickerOpen) return
    const handler = (e: MouseEvent) => {
      if (colorPickerRef.current && !colorPickerRef.current.contains(e.target as Node)) {
        setColorPickerOpen(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [colorPickerOpen])

  // 检测选中文本是否在颜色 span 内（用于避免嵌套）
  const detectColorSpan = useCallback(() => {
    const view = viewRef.current
    if (!view) return null
    const { from, to } = view.state.selection.main
    const selected = view.state.sliceDoc(from, to)

    // Case 1: 选中的文本本身就是完整的 <span style="color: ...">...</span>
    const fullMatch = selected.match(/^<span style="color:\s*([^"]*)">([\s\S]*)<\/span>$/)
    if (fullMatch) {
      return { color: fullMatch[1].trim(), inner: fullMatch[2], outerFrom: from, outerTo: to }
    }

    // Case 2: 选中的文本在 span 内部（标签在选区外）
    const searchBefore = view.state.sliceDoc(Math.max(0, from - 100), from)
    const beforeMatch = searchBefore.match(/<span style="color:\s*([^"]*)">$/)
    if (beforeMatch) {
      const searchAfter = view.state.sliceDoc(to, Math.min(view.state.doc.length, to + 20))
      const afterMatch = searchAfter.match(/^<\/span>/)
      if (afterMatch) {
        const outerFrom = from - beforeMatch[0].length
        const outerTo = to + afterMatch[0].length
        return { color: beforeMatch[1].trim(), inner: selected, outerFrom, outerTo }
      }
    }

    return null
  }, [])

  const handleAction = useCallback((action: Action) => {
    const view = viewRef.current
    if (!view) return
    const { from, to } = view.state.selection.main
    const selected = view.state.sliceDoc(from, to)

    let insert: string
    let cursorPos: number

    if (action.type === 'wrap') {
      insert = action.before + selected + action.after
      cursorPos = from + action.before.length + selected.length
    } else if (action.type === 'line') {
      insert = action.prefix + selected
      cursorPos = from + insert.length
    } else {
      insert = action.before + selected + action.after
      cursorPos = from + action.before.length + selected.length
    }

    view.dispatch({
      changes: { from, to, insert },
      selection: { anchor: cursorPos },
    })
    view.focus()
  }, [])

  const handleColorSelect = useCallback((color: string) => {
    const view = viewRef.current
    if (!view) return

    const existing = detectColorSpan()
    const before = `<span style="color: ${color}">`
    const after = '</span>'

    if (existing) {
      // 替换已有颜色 span 的颜色值，避免嵌套
      const insert = before + existing.inner + after
      view.dispatch({
        changes: { from: existing.outerFrom, to: existing.outerTo, insert },
        selection: { anchor: existing.outerFrom + before.length, head: existing.outerFrom + before.length + existing.inner.length },
      })
    } else {
      // 新包裹
      const { from, to } = view.state.selection.main
      const selected = view.state.sliceDoc(from, to)
      const text = selected || 'text'
      const insert = before + text + after
      const cursorFrom = from + before.length
      const cursorTo = cursorFrom + text.length
      view.dispatch({
        changes: { from, to, insert },
        selection: { anchor: cursorFrom, head: cursorTo },
      })
    }

    view.focus()
    setColorPickerOpen(false)
  }, [detectColorSpan])

  const handleRemoveColor = useCallback(() => {
    const view = viewRef.current
    if (!view) return

    const existing = detectColorSpan()
    if (existing) {
      view.dispatch({
        changes: { from: existing.outerFrom, to: existing.outerTo, insert: existing.inner },
        selection: { anchor: existing.outerFrom, head: existing.outerFrom + existing.inner.length },
      })
    }

    view.focus()
    setColorPickerOpen(false)
  }, [detectColorSpan])

  const handleCustomHexApply = useCallback(() => {
    const hex = customHex.trim()
    if (/^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(hex)) {
      handleColorSelect(hex)
      setCustomHex('#')
    }
  }, [customHex, handleColorSelect])

  const handleNativePickerChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    handleColorSelect(e.target.value)
  }, [handleColorSelect])

  return (
    <div className={`${fill ? 'flex flex-col' : ''} ${className ?? ''}`}>
      <div className="flex flex-wrap items-center gap-0.5 p-1.5 border border-b-0 rounded-t-md bg-muted/30 shrink-0">
        {actions.map((item) => {
          if (!item.icon) {
            return <div key={item.key} className="w-px h-5 bg-border mx-1" />
          }
          const Icon = item.icon
          return (
            <button
              key={item.key}
              type="button"
              title={t.editor[item.key as keyof typeof t.editor]}
              className="p-1.5 rounded hover:bg-muted transition-colors text-muted-foreground hover:text-foreground"
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => handleAction(item.action)}
            >
              <Icon className="h-4 w-4" />
            </button>
          )
        })}

        <div className="w-px h-5 bg-border mx-1" />

        {/* 字体颜色选择器 */}
        <div className="relative" ref={colorPickerRef}>
          <button
            type="button"
            title={t.editor.textColor}
            className="p-1.5 rounded hover:bg-muted transition-colors text-muted-foreground hover:text-foreground"
            onMouseDown={(e) => e.preventDefault()}
            onClick={() => {
              const opening = !colorPickerOpen
              setColorPickerOpen(opening)
              if (opening) {
                const existing = detectColorSpan()
                setCustomHex(existing?.color || '#')
              }
            }}
          >
            <Palette className="h-4 w-4" />
          </button>

          {colorPickerOpen && (
            <div className="absolute top-full left-0 mt-1 z-50 bg-popover border rounded-lg shadow-lg p-2.5 w-[236px]">
              {/* 色相色板 */}
              <div className="space-y-1">
                {COLOR_PALETTE.map((row, rowIdx) => (
                  <div key={rowIdx} className="flex gap-1">
                    {row.map((color) => (
                      <button
                        key={color}
                        type="button"
                        className="w-6 h-6 rounded border border-black/10 hover:scale-125 hover:z-10 transition-transform cursor-pointer relative"
                        style={{ backgroundColor: color }}
                        title={color}
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => handleColorSelect(color)}
                      />
                    ))}
                  </div>
                ))}
              </div>

              {/* 分隔线 */}
              <div className="h-px bg-border my-2" />

              {/* 中性色 */}
              <div className="flex gap-1">
                {NEUTRAL_COLORS.map((color) => (
                  <button
                    key={color}
                    type="button"
                    className="w-6 h-6 rounded border border-border hover:scale-125 hover:z-10 transition-transform cursor-pointer"
                    style={{ backgroundColor: color }}
                    title={color}
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => handleColorSelect(color)}
                  />
                ))}
              </div>

              {/* 分隔线 */}
              <div className="h-px bg-border my-2" />

              {/* 自定义颜色输入 */}
              <div className="flex items-center gap-1.5">
                <div
                  className="w-6 h-6 rounded border border-border shrink-0"
                  style={{ backgroundColor: /^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(customHex.trim()) ? customHex.trim() : 'transparent' }}
                />
                <input
                  type="text"
                  value={customHex}
                  onChange={(e) => setCustomHex(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      e.preventDefault()
                      handleCustomHexApply()
                    }
                  }}
                  placeholder="#ff0000"
                  maxLength={7}
                  className="flex-1 min-w-0 h-6 px-1.5 text-xs border rounded bg-background text-foreground outline-none focus:ring-1 focus:ring-ring font-mono"
                  onMouseDown={(e) => e.stopPropagation()}
                />
                <button
                  type="button"
                  className="h-6 px-2 text-xs rounded bg-primary text-primary-foreground hover:bg-primary/90 transition-colors shrink-0 disabled:opacity-50"
                  disabled={!/^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(customHex.trim())}
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={handleCustomHexApply}
                >
                  OK
                </button>

                {/* 系统取色器 */}
                <button
                  type="button"
                  title={t.editor.customColor}
                  className="h-6 w-6 flex items-center justify-center rounded border border-border hover:bg-muted transition-colors shrink-0 relative"
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => nativePickerRef.current?.click()}
                >
                  <Pipette className="h-3 w-3" />
                  <input
                    ref={nativePickerRef}
                    type="color"
                    className="absolute inset-0 opacity-0 w-full h-full cursor-pointer"
                    tabIndex={-1}
                    onChange={handleNativePickerChange}
                  />
                </button>
              </div>

              {/* 清除颜色 */}
              <button
                type="button"
                className="mt-2 w-full flex items-center justify-center gap-1.5 text-xs text-muted-foreground hover:text-foreground py-1 rounded hover:bg-muted transition-colors"
                onMouseDown={(e) => e.preventDefault()}
                onClick={handleRemoveColor}
              >
                <X className="h-3 w-3" />
                {t.editor.removeColor}
              </button>
            </div>
          )}
        </div>
      </div>
      <div
        className={`rounded-b-md border overflow-hidden cursor-text ${fill ? 'relative flex-1 min-h-0' : ''}`}
        onClick={(e) => {
          if ((e.target as HTMLElement).closest('.cm-content')) return
          const view = viewRef.current
          if (!view) return
          view.focus()
        }}
      >
        <CodeMirror
          value={value}
          extensions={[markdown()]}
          onChange={onChange}
          theme={resolvedEditorTheme}
          className="[&_.cm-editor]:!rounded-none"
          placeholder={placeholder}
          onCreateEditor={(view) => {
            viewRef.current = view
            const el = view.dom
            const scroller = el.querySelector('.cm-scroller') as HTMLElement
            if (scroller) scroller.style.overflow = 'auto'
            if (fill) {
              const wrapper = el.parentElement as HTMLElement
              wrapper.style.position = 'absolute'
              wrapper.style.inset = '0'
              el.style.height = '100%'
            } else {
              el.style.minHeight = height
              el.style.maxHeight = height
            }
          }}
        />
      </div>
    </div>
  )
}
