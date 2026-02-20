'use client'

import { useRef, useCallback } from 'react'
import CodeMirror from '@uiw/react-codemirror'
import { markdown } from '@codemirror/lang-markdown'
import { EditorView } from '@codemirror/view'
import { useTheme } from '@/contexts/theme-context'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import {
  Bold, Italic, Strikethrough, Heading1, Heading2, Heading3,
  Code, FileCode, Link, Image, List, ListOrdered, Quote,
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

export function MarkdownEditor({ value, onChange, height = '400px', fill, theme, className, placeholder }: MarkdownEditorProps) {
  const viewRef = useRef<EditorView | null>(null)
  const { resolvedTheme } = useTheme()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const resolvedEditorTheme = theme ?? (resolvedTheme === 'dark' ? 'dark' : 'light')

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
