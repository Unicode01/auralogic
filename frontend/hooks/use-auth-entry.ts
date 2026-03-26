'use client'

import { useMemo } from 'react'
import { usePathname, useRouter, useSearchParams } from 'next/navigation'

import { setAuthReturnState } from '@/lib/auth-return-state'

type AuthEntryTarget = '/login' | '/register'

export function useAuthEntry() {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const searchKey = searchParams?.toString() || ''

  const currentPathWithSearch = useMemo(() => {
    const currentPath = pathname || '/'
    return searchKey ? `${currentPath}?${searchKey}` : currentPath
  }, [pathname, searchKey])

  const goToAuth = (target: AuthEntryTarget) => {
    setAuthReturnState({
      redirectPath: currentPathWithSearch,
    })
    router.push(target)
  }

  return {
    currentPathWithSearch,
    goToAuth,
  }
}
