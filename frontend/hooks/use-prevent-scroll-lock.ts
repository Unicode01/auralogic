import { useEffect } from 'react'

/**
 * 防止 Radix UI 组件（Dialog/Select/AlertDialog）锁定页面滚动
 * 
 * Radix UI 会在打开模态组件时自动锁定 body 的滚动，
 * 但这在使用自定义滚动容器的布局中会导致问题。
 * 
 * 这个 hook 会监控 body 元素的样式变化，并移除不需要的滚动锁定。
 */
export function usePreventScrollLock() {
  useEffect(() => {
    // 强制移除任何已存在的锁定样式
    const forceUnlock = () => {
      const body = document.body
      
      // 只有当样式被设置时才移除（避免不必要的重绘）
      if (body.style.pointerEvents) {
        body.style.pointerEvents = ''
      }
      if (body.style.overflow && body.style.overflow === 'hidden') {
        body.style.overflow = ''
      }
      if (body.style.paddingRight) {
        body.style.paddingRight = ''
      }
    }

    // 立即执行一次
    forceUnlock()

    // 创建 MutationObserver 来监控 body 的样式变化
    const observer = new MutationObserver(() => {
      // 使用 requestAnimationFrame 来优化性能
      requestAnimationFrame(forceUnlock)
    })

    // 开始观察 body 元素
    observer.observe(document.body, {
      attributes: true,
      attributeFilter: ['style'],
    })

    // 清理函数
    return () => {
      observer.disconnect()
    }
  }, [])
}
