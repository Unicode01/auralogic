'use client'

import { useState, useEffect, useRef, useCallback } from 'react'
import { TicketMessage } from '@/lib/api'
import { getToken } from '@/lib/auth'

interface WebSocketMessage {
  type: 'new_message' | 'read_status'
  ticket_id: number
  data: any
  timestamp: number
}

interface UseTicketWebSocketResult {
  messages: TicketMessage[]
  isConnected: boolean
  error: string | null
}

export function useTicketWebSocket(
  ticketId: number,
  userType: 'user' | 'admin'
): UseTicketWebSocketResult {
  const [messages, setMessages] = useState<TicketMessage[]>([])
  const [isConnected, setIsConnected] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<NodeJS.Timeout | null>(null)
  const reconnectAttemptsRef = useRef(0)
  const maxReconnectAttempts = 5

  const connect = useCallback(() => {
    if (!ticketId || wsRef.current?.readyState === WebSocket.OPEN) {
      return
    }

    const token = getToken()
    if (!token) {
      setError('未登录')
      return
    }

    // 构建WebSocket URL
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || window.location.origin
    const wsHost = apiUrl.replace(/^https?:\/\//, '')
    const basePath = userType === 'admin' ? '/api/admin/tickets' : '/api/user/tickets'
    const wsUrl = `${protocol}//${wsHost}${basePath}/${ticketId}/ws?token=${token}`

    try {
      const ws = new WebSocket(wsUrl)
      wsRef.current = ws

      ws.onopen = () => {
        setIsConnected(true)
        setError(null)
        reconnectAttemptsRef.current = 0
      }

      ws.onmessage = (event) => {
        try {
          const data: WebSocketMessage = JSON.parse(event.data)

          if (data.type === 'new_message') {
            const newMessage: TicketMessage = {
              id: data.data.id,
              ticket_id: data.data.ticket_id,
              sender_type: data.data.sender_type,
              sender_id: data.data.sender_id,
              sender_name: data.data.sender_name,
              content: data.data.content,
              content_type: data.data.content_type,
              is_read_by_user: data.data.is_read_by_user,
              is_read_by_admin: data.data.is_read_by_admin,
              created_at: data.data.created_at,
            }

            setMessages((prev) => {
              // 避免重复添加
              if (prev.some((msg) => msg.id === newMessage.id)) {
                return prev
              }
              return [...prev, newMessage]
            })
          }
        } catch (e) {
          console.error('Failed to parse WebSocket message:', e)
        }
      }

      ws.onerror = () => {
        setError('连接错误')
      }

      ws.onclose = () => {
        setIsConnected(false)
        wsRef.current = null

        // 尝试重连
        if (reconnectAttemptsRef.current < maxReconnectAttempts) {
          reconnectAttemptsRef.current++
          const delay = Math.min(1000 * Math.pow(2, reconnectAttemptsRef.current), 30000)
          reconnectTimeoutRef.current = setTimeout(() => {
            connect()
          }, delay)
        }
      }
    } catch (e) {
      console.error('WebSocket connection failed:', e)
      setError('连接失败')
    }
  }, [ticketId, userType])

  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = null
    }
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
    setIsConnected(false)
  }, [])

  useEffect(() => {
    if (ticketId) {
      connect()
    }

    return () => {
      disconnect()
    }
  }, [ticketId, connect, disconnect])

  // 当ticketId变化时，清空之前的消息
  useEffect(() => {
    setMessages([])
  }, [ticketId])

  return {
    messages,
    isConnected,
    error,
  }
}
