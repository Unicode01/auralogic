'use client'

import { useState, useRef, useCallback, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { MarkdownMessage } from '@/components/ui/markdown-message'
import {
  Image as ImageIcon,
  Mic,
  Bold,
  Italic,
  Code,
  List,
  Link2,
  Eye,
  EyeOff,
  Send,
  Loader2,
  Square,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { PluginSlot } from '@/components/plugins/plugin-slot'

interface ToolbarTranslations {
  messagePlaceholder: string
  uploadImage: string
  recordVoice: string
  recording: string
  recordingTip: string
  voiceMessage: string
  bold: string
  italic: string
  code: string
  list: string
  link: string
  preview: string
  editMode: string
  send: string
  noPreviewContent: string
}

interface MessageToolbarProps {
  value: string
  onChange: (value: string) => void
  onSend: () => void
  onUploadFile: (file: File) => Promise<string>
  isSending?: boolean
  disabled?: boolean
  placeholder?: string
  enableImage?: boolean
  enableVoice?: boolean
  acceptImageTypes?: string[]
  maxLength?: number
  translations?: ToolbarTranslations
  pluginSlotNamespace?: string
  pluginSlotContext?: Record<string, any>
  pluginSlotPath?: string
}

export function MessageToolbar({
  value,
  onChange,
  onSend,
  onUploadFile,
  isSending = false,
  disabled = false,
  placeholder,
  enableImage = true,
  enableVoice = true,
  acceptImageTypes,
  maxLength,
  translations,
  pluginSlotNamespace,
  pluginSlotContext,
  pluginSlotPath,
}: MessageToolbarProps) {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  const tt: ToolbarTranslations = translations || {
    messagePlaceholder: t.ticket.messagePlaceholder,
    uploadImage: t.ticket.uploadImage,
    recordVoice: t.ticket.recordVoice,
    recording: t.ticket.recording,
    recordingTip: t.ticket.recordingTip,
    voiceMessage: t.ticket.voiceMessage,
    bold: t.ticket.bold,
    italic: t.ticket.italic,
    code: t.ticket.code,
    list: t.ticket.list,
    link: t.ticket.link,
    preview: t.ticket.preview,
    editMode: t.ticket.editMode,
    send: t.ticket.send,
    noPreviewContent: t.ticket.noPreviewContent,
  }
  const [isPreview, setIsPreview] = useState(false)
  const [isUploading, setIsUploading] = useState(false)
  const [isRecording, setIsRecording] = useState(false)
  const [recordingDuration, setRecordingDuration] = useState(0)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const audioChunksRef = useRef<Blob[]>([])
  const recordingTimerRef = useRef<NodeJS.Timeout | null>(null)

  // 根据后台配置构造 file input 的 accept 属性
  const fileAccept = (() => {
    if (acceptImageTypes && acceptImageTypes.length > 0) {
      return acceptImageTypes.join(',')
    }
    return 'image/*'
  })()
  const isOverLimit = Boolean(maxLength && value.length > maxLength)
  const buildPluginComposerContext = (section: string, extra?: Record<string, any>) => ({
    ...(pluginSlotContext || {}),
    section,
    composer: {
      draft_length: value.length,
      has_content: value.trim().length > 0,
      is_preview: isPreview,
      is_uploading: isUploading,
      is_recording: isRecording,
      recording_duration: recordingDuration,
      is_over_limit: isOverLimit,
      max_length: maxLength || undefined,
      enable_image: enableImage,
      enable_voice: enableVoice,
    },
    ...(extra || {}),
  })
  const renderPluginSlot = (
    slotSuffix: string,
    section: string,
    extra?: Record<string, any>,
    display: 'stack' | 'inline' = 'stack'
  ) =>
    pluginSlotNamespace ? (
      <PluginSlot
        slot={`${pluginSlotNamespace}.${slotSuffix}`}
        path={pluginSlotPath}
        context={buildPluginComposerContext(section, extra)}
        display={display}
      />
    ) : null

  const adjustTextareaHeight = useCallback(() => {
    const textarea = textareaRef.current
    if (textarea) {
      textarea.style.height = 'auto'
      textarea.style.height = Math.min(textarea.scrollHeight, 120) + 'px'
    }
  }, [])

  useEffect(() => {
    adjustTextareaHeight()
  }, [value, adjustTextareaHeight])

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (!isPreview) onSend()
    }
  }

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    onChange(e.target.value)
  }

  // 在光标位置插入文本
  const insertAtCursor = (before: string, after: string = '') => {
    const textarea = textareaRef.current
    if (!textarea) return
    const start = textarea.selectionStart
    const end = textarea.selectionEnd
    const selected = value.substring(start, end)
    const newText = value.substring(0, start) + before + selected + after + value.substring(end)
    onChange(newText)

    requestAnimationFrame(() => {
      textarea.focus()
      const cursorPos = selected
        ? start + before.length + selected.length + after.length
        : start + before.length
      textarea.setSelectionRange(cursorPos, cursorPos)
    })
  }

  // 图片上传
  const handleImageClick = () => {
    fileInputRef.current?.click()
  }

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    e.target.value = ''

    setIsUploading(true)
    try {
      const url = await onUploadFile(file)
      if (file.type.startsWith('audio/')) {
        insertAtCursor(`[${tt.voiceMessage}](${url})`)
      } else {
        insertAtCursor(`![${file.name}](${url})`)
      }
    } catch {
      // 错误由上层处理
    } finally {
      setIsUploading(false)
    }
  }

  // 语音录制
  const startRecording = async () => {
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
      const mediaRecorder = new MediaRecorder(stream)
      mediaRecorderRef.current = mediaRecorder
      audioChunksRef.current = []

      mediaRecorder.ondataavailable = (e) => {
        if (e.data.size > 0) audioChunksRef.current.push(e.data)
      }

      mediaRecorder.onstop = async () => {
        stream.getTracks().forEach((track) => track.stop())
        const audioBlob = new Blob(audioChunksRef.current, { type: 'audio/webm' })
        const file = new File([audioBlob], `voice_${Date.now()}.webm`, { type: 'audio/webm' })

        setIsUploading(true)
        try {
          const url = await onUploadFile(file)
          insertAtCursor(`[${tt.voiceMessage} ${recordingDuration}s](${url})`)
        } catch {
          // 错误由上层处理
        } finally {
          setIsUploading(false)
          setRecordingDuration(0)
        }
      }

      mediaRecorder.start()
      setIsRecording(true)
      setRecordingDuration(0)
      recordingTimerRef.current = setInterval(() => {
        setRecordingDuration((d) => d + 1)
      }, 1000)
    } catch {
      // 麦克风权限被拒绝等
    }
  }

  const stopRecording = () => {
    if (mediaRecorderRef.current && mediaRecorderRef.current.state === 'recording') {
      mediaRecorderRef.current.stop()
    }
    if (recordingTimerRef.current) {
      clearInterval(recordingTimerRef.current)
      recordingTimerRef.current = null
    }
    setIsRecording(false)
  }

  // 清理录制计时器
  useEffect(() => {
    return () => {
      if (recordingTimerRef.current) {
        clearInterval(recordingTimerRef.current)
      }
    }
  }, [])

  const mdActions = [
    { icon: Bold, label: tt.bold, action: () => insertAtCursor('**', '**') },
    { icon: Italic, label: tt.italic, action: () => insertAtCursor('*', '*') },
    { icon: Code, label: tt.code, action: () => insertAtCursor('`', '`') },
    { icon: List, label: tt.list, action: () => insertAtCursor('\n- ') },
    { icon: Link2, label: tt.link, action: () => insertAtCursor('[', '](url)') },
  ]

  const ToolbarButton = ({
    icon: Icon,
    label,
    onClick,
    active,
    btnDisabled,
    className: extraClassName,
  }: {
    icon: typeof Bold
    label: string
    onClick: () => void
    active?: boolean
    btnDisabled?: boolean
    className?: string
  }) => (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className={cn('h-7 w-7', active && 'bg-accent', extraClassName)}
      onClick={onClick}
      disabled={disabled || btnDisabled}
      title={label}
      aria-label={label}
      aria-pressed={active}
    >
      <Icon className="h-3.5 w-3.5" />
      <span className="sr-only">{label}</span>
    </Button>
  )

  return (
    <div className="overflow-hidden rounded-md border bg-background">
      {/* 工具栏 */}
      <div className="flex items-center gap-0.5 border-b bg-muted/30 px-1.5 py-1">
        {/* 图片上传 */}
        {enableImage && (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            onClick={handleImageClick}
            disabled={disabled || isUploading}
            title={tt.uploadImage}
            aria-label={tt.uploadImage}
          >
            {isUploading ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <ImageIcon className="h-3.5 w-3.5" />
            )}
            <span className="sr-only">{tt.uploadImage}</span>
          </Button>
        )}

        {/* 语音录制 */}
        {enableVoice && (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={cn('h-7 w-7', isRecording && 'text-red-500 hover:text-red-600')}
            onClick={isRecording ? stopRecording : startRecording}
            disabled={disabled || isUploading}
            title={
              isRecording
                ? `${tt.recording} ${recordingDuration}s, ${tt.recordingTip}`
                : tt.recordVoice
            }
            aria-label={isRecording ? `${tt.recording} ${recordingDuration}s` : tt.recordVoice}
            aria-pressed={isRecording}
          >
            {isRecording ? (
              <Square className="h-3 w-3 fill-current" />
            ) : (
              <Mic className="h-3.5 w-3.5" />
            )}
            <span className="sr-only">
              {isRecording ? `${tt.recording} ${recordingDuration}s` : tt.recordVoice}
            </span>
          </Button>
        )}

        {isRecording && (
          <span className="ml-0.5 animate-pulse text-xs tabular-nums text-red-500">
            {recordingDuration}s
          </span>
        )}

        {(enableImage || enableVoice) && (
          <div className="mx-0.5 hidden h-4 w-px bg-border md:block" />
        )}

        {/* Markdown 快捷操作 - 移动端隐藏 */}
        {mdActions.map((action) => (
          <ToolbarButton
            key={action.label}
            icon={action.icon}
            label={action.label}
            onClick={action.action}
            btnDisabled={isPreview}
            className="hidden md:inline-flex"
          />
        ))}

        <div className="mx-0.5 hidden h-4 w-px bg-border md:block" />

        {/* 预览切换 - 移动端隐藏 */}
        <ToolbarButton
          icon={isPreview ? EyeOff : Eye}
          label={isPreview ? tt.editMode : tt.preview}
          onClick={() => setIsPreview(!isPreview)}
          active={isPreview}
          className="hidden md:inline-flex"
        />
        {renderPluginSlot('toolbar.after', 'toolbar', undefined, 'inline')}

        {/* 发送按钮 */}
        <div className="ml-auto">
          <Button
            type="button"
            size="icon"
            className="h-7 w-7"
            onClick={onSend}
            disabled={disabled || !value.trim() || isSending || isPreview || isOverLimit}
            title={tt.send}
            aria-label={tt.send}
          >
            <Send className="h-3.5 w-3.5" />
            <span className="sr-only">{tt.send}</span>
          </Button>
        </div>
      </div>

      {/* 内容区 */}
      {isPreview ? (
        <div className="scrollbar-hide max-h-[150px] min-h-[100px] overflow-y-auto px-3 py-2">
          {value.trim() ? (
            <MarkdownMessage content={value} />
          ) : (
            <p className="text-sm text-muted-foreground">{tt.noPreviewContent}</p>
          )}
          {renderPluginSlot('preview.after', 'preview', {
            has_content: value.trim().length > 0,
          })}
        </div>
      ) : (
        <div>
          <div className="relative">
            <Textarea
              ref={textareaRef}
              value={value}
              onChange={handleChange}
              onKeyDown={handleKeyDown}
              placeholder={placeholder || tt.messagePlaceholder}
              className="scrollbar-hide max-h-[150px] min-h-[100px] resize-none rounded-none border-0 text-sm focus-visible:ring-0 focus-visible:ring-offset-0"
              rows={3}
              disabled={disabled}
              aria-label={placeholder || tt.messagePlaceholder}
            />
            {maxLength && maxLength > 0 && (
              <span
                className={cn(
                  'absolute bottom-1.5 right-2 text-xs tabular-nums',
                  value.length > maxLength ? 'text-destructive' : 'text-muted-foreground'
                )}
              >
                {value.length}/{maxLength}
              </span>
            )}
          </div>
          {renderPluginSlot('editor.after', 'editor', {
            has_content: value.trim().length > 0,
          })}
        </div>
      )}

      {renderPluginSlot('bottom', 'bottom')}

      {/* 隐藏的文件输入 */}
      <input
        ref={fileInputRef}
        type="file"
        accept={fileAccept}
        className="hidden"
        onChange={handleFileChange}
        aria-label={tt.uploadImage}
      />
    </div>
  )
}
