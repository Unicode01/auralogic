'use client'

import { useMemo } from 'react'

import { PluginExtensionList } from '@/components/plugins/plugin-extension-list'
import { type PluginFrontendExtension } from '@/lib/api'
import { cn } from '@/lib/utils'

type PluginSlotContentProps = {
  extensions: PluginFrontendExtension[]
  className?: string
  display?: 'stack' | 'inline'
  animate?: boolean
  refreshing?: boolean
}

export function buildPluginSlotContentSignature(
  extensions: PluginFrontendExtension[]
): string {
  if (!Array.isArray(extensions) || extensions.length === 0) {
    return ''
  }

  return JSON.stringify(
    extensions.map((extension) => ({
      id: extension.id || '',
      plugin_id: extension.plugin_id || 0,
      type: extension.type || '',
      title: extension.title || '',
      content: extension.content || '',
      link: extension.link || '',
      data: extension.data || null,
      metadata: extension.metadata || null,
    }))
  )
}

export function shouldShowPluginSlotRefreshing(
  hasData: boolean,
  isFetching: boolean,
  extensions: PluginFrontendExtension[]
): boolean {
  return hasData && isFetching && Array.isArray(extensions) && extensions.length > 0
}

export function PluginSlotContent({
  extensions,
  className,
  display = 'stack',
  animate = true,
  refreshing = false,
}: PluginSlotContentProps) {
  const contentSignature = useMemo(
    () => (animate ? buildPluginSlotContentSignature(extensions) : ''),
    [animate, extensions]
  )

  if (!Array.isArray(extensions) || extensions.length === 0) {
    return null
  }

  return (
    <div className={cn('plugin-slot-shell', refreshing && 'plugin-slot-refreshing', className)}>
      <div key={animate ? contentSignature : undefined} className={animate ? 'plugin-slot-fade-in' : undefined}>
        <PluginExtensionList extensions={extensions} display={display} />
      </div>
    </div>
  )
}
