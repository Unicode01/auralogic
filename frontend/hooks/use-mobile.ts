'use client'

import { useState, useEffect } from 'react'

const MOBILE_BREAKPOINT = 768 // px

export function useIsMobile() {
  const [isMobile, setIsMobile] = useState(false)
  const [mounted, setMounted] = useState(false)

  useEffect(() => {
    setMounted(true)

    // 检测方法1: 通过视口宽度
    const checkWidth = () => {
      setIsMobile(window.innerWidth < MOBILE_BREAKPOINT)
    }

    // 检测方法2: 通过 User Agent
    const checkUA = () => {
      const ua = navigator.userAgent || navigator.vendor || (window as any).opera
      const mobileRegex = /android|webos|iphone|ipad|ipod|blackberry|iemobile|opera mini|mobile|tablet/i
      return mobileRegex.test(ua.toLowerCase())
    }

    // 检测方法3: 通过触摸支持
    const checkTouch = () => {
      return 'ontouchstart' in window || navigator.maxTouchPoints > 0
    }

    // 综合判断: 视口宽度 OR (UA检测 AND 触摸支持)
    const checkMobile = () => {
      const isNarrowScreen = window.innerWidth < MOBILE_BREAKPOINT
      const isMobileUA = checkUA()
      const hasTouch = checkTouch()

      setIsMobile(isNarrowScreen || (isMobileUA && hasTouch))
    }

    checkMobile()

    // 监听窗口大小变化
    window.addEventListener('resize', checkMobile)

    return () => {
      window.removeEventListener('resize', checkMobile)
    }
  }, [])

  return { isMobile, mounted }
}
