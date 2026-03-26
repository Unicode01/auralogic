import type { Renderable } from 'react-hot-toast'

import { cn } from '@/lib/utils'

type ToastMessageProps = {
  message: string
  className?: string
}

export function ToastMessage({ message, className }: ToastMessageProps) {
  return (
    <div className={cn('max-w-md whitespace-pre-line break-words text-sm', className)}>
      {message}
    </div>
  )
}

export function renderToastMessage(message: string, className?: string): Renderable {
  return <ToastMessage message={message} className={className} />
}
