'use client'

import { useState, useEffect } from 'react'

const PHONE_BREAKPOINT = 768
const TABLET_BREAKPOINT = 1024
const TABLET_DEVICE_MAX_WIDTH = 1400

type ResponsiveLayoutState = {
  width: number
  isPhone: boolean
  isTablet: boolean
  isDesktop: boolean
  isMobile: boolean
  mounted: boolean
}

function isLikelyTabletDevice() {
  if (typeof navigator === 'undefined' || typeof window === 'undefined') {
    return false
  }

  const userAgent = navigator.userAgent || navigator.vendor || ''
  const normalizedUserAgent = userAgent.toLowerCase()
  const isAndroidTablet =
    normalizedUserAgent.includes('android') && !normalizedUserAgent.includes('mobile')
  const hasTabletKeyword = /ipad|tablet|playbook|silk|kindle/.test(normalizedUserAgent)
  const isIpadDesktopMode = navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1

  return isAndroidTablet || hasTabletKeyword || isIpadDesktopMode
}

function getResponsiveLayoutState(): Omit<ResponsiveLayoutState, 'mounted'> {
  const width = window.innerWidth
  const isPhone = width < PHONE_BREAKPOINT
  const isTabletByWidth = width >= PHONE_BREAKPOINT && width < TABLET_BREAKPOINT
  const isTabletByDevice = !isPhone && isLikelyTabletDevice() && width < TABLET_DEVICE_MAX_WIDTH
  const isTablet = isTabletByWidth || isTabletByDevice

  return {
    width,
    isPhone,
    isTablet,
    isDesktop: !isPhone && !isTablet,
    isMobile: isPhone || isTablet,
  }
}

const INITIAL_STATE: ResponsiveLayoutState = {
  width: 0,
  isPhone: false,
  isTablet: false,
  isDesktop: false,
  isMobile: false,
  mounted: false,
}

export function useResponsiveLayout() {
  const [layoutState, setLayoutState] = useState<ResponsiveLayoutState>(INITIAL_STATE)

  useEffect(() => {
    const updateLayoutState = () => {
      setLayoutState({
        ...getResponsiveLayoutState(),
        mounted: true,
      })
    }

    updateLayoutState()
    window.addEventListener('resize', updateLayoutState)

    return () => {
      window.removeEventListener('resize', updateLayoutState)
    }
  }, [])

  return layoutState
}

export function useIsMobile() {
  const { isMobile, mounted } = useResponsiveLayout()
  return { isMobile, mounted }
}
