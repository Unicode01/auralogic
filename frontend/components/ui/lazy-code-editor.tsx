'use client'

import dynamic from 'next/dynamic'
import { useEffect, useState } from 'react'
import type { Extension } from '@codemirror/state'
import type { EditorView } from '@codemirror/view'
import type { ReactCodeMirrorProps } from '@uiw/react-codemirror'

const CodeMirror = dynamic(() => import('@uiw/react-codemirror'), { ssr: false })

export type LazyCodeEditorLanguage = 'css' | 'html' | 'javascript' | 'json' | 'markdown'

type LazyCodeEditorProps = {
  value: string
  onChange: (value: string) => void
  language?: LazyCodeEditorLanguage
  height?: string
  theme?: ReactCodeMirrorProps['theme']
  className?: ReactCodeMirrorProps['className']
  placeholder?: ReactCodeMirrorProps['placeholder']
  basicSetup?: ReactCodeMirrorProps['basicSetup']
  onBlur?: ReactCodeMirrorProps['onBlur']
  onCreateEditor?: (view: EditorView) => void
}

const extensionCache = new Map<LazyCodeEditorLanguage, Promise<Extension>>()

function loadLanguageExtension(language?: LazyCodeEditorLanguage): Promise<Extension[]> {
  if (!language) {
    return Promise.resolve([])
  }

  let cached = extensionCache.get(language)
  if (!cached) {
    switch (language) {
      case 'css':
        cached = import('@codemirror/lang-css').then((mod) => mod.css())
        break
      case 'html':
        cached = import('@codemirror/lang-html').then((mod) => mod.html())
        break
      case 'javascript':
        cached = import('@codemirror/lang-javascript').then((mod) => mod.javascript())
        break
      case 'json':
        cached = import('@codemirror/lang-json').then((mod) => mod.json())
        break
      case 'markdown':
        cached = import('@codemirror/lang-markdown').then((mod) => mod.markdown())
        break
      default:
        cached = Promise.resolve([])
        break
    }
    extensionCache.set(language, cached)
  }

  return cached.then((extension) => (Array.isArray(extension) ? extension : [extension]))
}

export function LazyCodeEditor({
  value,
  onChange,
  language,
  height,
  theme,
  className,
  placeholder,
  basicSetup,
  onBlur,
  onCreateEditor,
}: LazyCodeEditorProps) {
  const [extensions, setExtensions] = useState<Extension[]>([])

  useEffect(() => {
    let active = true
    loadLanguageExtension(language)
      .then((loaded) => {
        if (!active) return
        setExtensions(loaded)
      })
      .catch(() => {
        if (!active) return
        setExtensions([])
      })
    return () => {
      active = false
    }
  }, [language])

  return (
    <CodeMirror
      value={value}
      extensions={extensions}
      onChange={(nextValue) => onChange(nextValue)}
      height={height}
      theme={theme}
      className={className}
      placeholder={placeholder}
      basicSetup={basicSetup}
      onBlur={onBlur}
      onCreateEditor={(view) => onCreateEditor?.(view)}
    />
  )
}
