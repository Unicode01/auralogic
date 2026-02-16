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
}

const defaultTranslations: ToolbarTranslations = {
  messagePlaceholder: '输入消息... (Enter 发送, Shift+Enter 换行)',
  uploadImage: '上传图片',
  recordVoice: '录制语音',
  recording: '录制中',
  recordingTip: '点击停止',
  voiceMessage: '语音消息',
  bold: '粗体',
  italic: '斜体',
  code: '代码',
  list: '列表',
  link: '链接',
  preview: '预览',
  editMode: '编辑',
  send: '发送',
  noPreviewContent: '无内容可预览',
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
  translations = defaultTranslations,
}: MessageToolbarProps) {
  const tt = translations
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
        stream.getTracks().forEach(track => track.stop())
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
        setRecordingDuration(d => d + 1)
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

  const ToolbarButton = ({ icon: Icon, label, onClick, active, btnDisabled, className: extraClassName }: {
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
    >
      <Icon className="h-3.5 w-3.5" />
    </Button>
  )

  return (
    <div className="border rounded-md bg-background overflow-hidden">
      {/* 工具栏 */}
      <div className="flex items-center gap-0.5 px-1.5 py-1 border-b bg-muted/30">
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
        >
          {isUploading
            ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
            : <ImageIcon className="h-3.5 w-3.5" />
          }
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
          title={isRecording ? `${tt.recording} ${recordingDuration}s，${tt.recordingTip}` : tt.recordVoice}
        >
          {isRecording
            ? <Square className="h-3 w-3 fill-current" />
            : <Mic className="h-3.5 w-3.5" />
          }
        </Button>
        )}

        {isRecording && (
          <span className="text-xs text-red-500 animate-pulse ml-0.5 tabular-nums">
            {recordingDuration}s
          </span>
        )}

        {(enableImage || enableVoice) && (
          <div className="w-px h-4 bg-border mx-0.5 hidden md:block" />
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

        <div className="w-px h-4 bg-border mx-0.5 hidden md:block" />

        {/* 预览切换 - 移动端隐藏 */}
        <ToolbarButton
          icon={isPreview ? EyeOff : Eye}
          label={isPreview ? tt.editMode : tt.preview}
          onClick={() => setIsPreview(!isPreview)}
          active={isPreview}
          className="hidden md:inline-flex"
        />

        {/* 发送按钮 */}
        <div className="ml-auto">
          <Button
            type="button"
            size="icon"
            className="h-7 w-7"
            onClick={onSend}
            disabled={disabled || !value.trim() || isSending || isPreview}
            title={tt.send}
          >
            <Send className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      {/* 内容区 */}
      {isPreview ? (
        <div className="min-h-[100px] max-h-[150px] overflow-y-auto px-3 py-2 scrollbar-hide">
          {value.trim() ? (
            <MarkdownMessage content={value} />
          ) : (
            <p className="text-sm text-muted-foreground">{tt.noPreviewContent}</p>
          )}
        </div>
      ) : (
        <div className="relative">
          <Textarea
            ref={textareaRef}
            value={value}
            onChange={handleChange}
            onKeyDown={handleKeyDown}
            placeholder={placeholder || tt.messagePlaceholder}
            className="min-h-[100px] max-h-[150px] resize-none border-0 rounded-none focus-visible:ring-0 focus-visible:ring-offset-0 scrollbar-hide text-sm"
            rows={3}
            disabled={disabled}
          />
          {maxLength && maxLength > 0 && (
            <span className={cn(
              "absolute bottom-1.5 right-2 text-xs tabular-nums",
              value.length > maxLength ? "text-destructive" : "text-muted-foreground"
            )}>
              {value.length}/{maxLength}
            </span>
          )}
        </div>
      )}

      {/* 隐藏的文件输入 */}
      <input
        ref={fileInputRef}
        type="file"
        accept={fileAccept}
        className="hidden"
        onChange={handleFileChange}
      />
    </div>
  )
}
