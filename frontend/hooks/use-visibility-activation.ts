'use client'

import { useEffect, useRef, useState } from 'react'

type UseVisibilityActivationOptions = {
  enabled?: boolean
  rootMargin?: string
}

export function useVisibilityActivation({
  enabled = true,
  rootMargin = '400px 0px',
}: UseVisibilityActivationOptions) {
  const targetRef = useRef<HTMLDivElement | null>(null)
  const [activated, setActivated] = useState(!enabled)

  useEffect(() => {
    if (!enabled || activated) {
      return
    }

    const target = targetRef.current
    if (!target) {
      return
    }

    if (typeof IntersectionObserver === 'undefined') {
      setActivated(true)
      return
    }

    const observer = new IntersectionObserver(
      (entries) => {
        const [entry] = entries
        if (!entry?.isIntersecting) {
          return
        }
        setActivated(true)
        observer.disconnect()
      },
      {
        root: null,
        rootMargin,
        threshold: 0,
      }
    )

    observer.observe(target)

    return () => {
      observer.disconnect()
    }
  }, [activated, enabled, rootMargin])

  return {
    targetRef,
    activated,
  }
}
