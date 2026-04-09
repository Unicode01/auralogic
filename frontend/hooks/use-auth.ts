'use client'

import { useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  addToCart,
  getCurrentUser,
  login,
  loginWithCode,
  loginWithPhoneCode,
  register,
  phoneRegister,
} from '@/lib/api'
import { getToken, setToken, clearToken, setUser } from '@/lib/auth'
import { clearGuestCart, getGuestCart, setGuestCart } from '@/lib/guest-cart'
import { clearAuthReturnState, readAuthReturnState } from '@/lib/auth-return-state'
import { resolveAuthApiErrorMessage } from '@/lib/api-error'
import { normalizeAuthUser } from '@/lib/auth-user'
import { useRouter } from 'next/navigation'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

type Locale = 'zh' | 'en'

// Resolve which locale to apply after login/register.
// Priority: pending-sync locale > server locale > stored locale
function resolvePostLoginLocale(serverLocale: string | undefined): Locale | null {
  if (typeof window === 'undefined') return null
  const pending = localStorage.getItem('auralogic_locale_pending_sync')
  const stored = localStorage.getItem('auralogic_locale')
  if (pending === 'zh' || pending === 'en') return pending
  if (serverLocale === 'zh' || serverLocale === 'en') return serverLocale
  if (stored === 'zh' || stored === 'en') return stored
  return null
}

export function useAuth() {
  const router = useRouter()
  const queryClient = useQueryClient()
  const { locale, setLocale } = useLocale()
  const t = getTranslations(locale)

  async function syncGuestCartAfterLogin(): Promise<boolean> {
    const guestItems = getGuestCart()
    if (!guestItems.length) return false

    const failedItems: typeof guestItems = []
    for (const item of guestItems) {
      try {
        await addToCart({
          product_id: item.product_id,
          quantity: item.quantity,
          attributes: item.attributes,
        })
      } catch {
        failedItems.push(item)
      }
    }

    if (failedItems.length > 0) {
      setGuestCart(failedItems)
    } else {
      clearGuestCart()
    }

    await queryClient.invalidateQueries({ queryKey: ['cart'] })
    await queryClient.invalidateQueries({ queryKey: ['cartCount'] })
    return true
  }

  async function handleAuthSuccess(data: any) {
    setToken(data.data.token)
    const user = normalizeAuthUser(data.data.user)
    const desired = resolvePostLoginLocale(user?.locale)
    const finalUser = normalizeAuthUser(desired ? { ...user, locale: desired } : user)
    setUser(finalUser)
    if (desired) setLocale(desired)
    queryClient.setQueryData(['currentUser'], { data: finalUser })

    const hadGuestCart = await syncGuestCartAfterLogin()
    const pendingReturnState = readAuthReturnState()
    router.push(pendingReturnState?.redirectPath || (hadGuestCart ? '/cart' : '/orders'))
  }

  // 获取当前用户
  const { data: user, isLoading, error } = useQuery({
    queryKey: ['currentUser'],
    queryFn: async () => {
      const response: any = await getCurrentUser()
      return response?.data
        ? {
            ...response,
            data: normalizeAuthUser(response.data),
          }
        : response
    },
    retry: false,
    enabled: typeof window !== 'undefined' && !!getToken(),
  })

  // 登录
  const loginMutation = useMutation({
    mutationFn: login,
    onSuccess: async (data: any) => {
      await handleAuthSuccess(data)
    },
    onError: (error: any) => {
      // 邮箱未验证，跳转到验证页面
      if (error.code === 30003 && error.data?.email) {
        router.push(`/verify-email?email=${encodeURIComponent(error.data.email)}&pending=true`)
        return
      }
      toast.error(resolveAuthApiErrorMessage(error, t, t.auth.loginFailed))
    },
  })

  // 验证码登录
  const loginWithCodeMutation = useMutation({
    mutationFn: loginWithCode,
    onSuccess: async (data: any) => {
      await handleAuthSuccess(data)
    },
    onError: (error: any) => {
      toast.error(resolveAuthApiErrorMessage(error, t, t.auth.loginFailed))
    },
  })

  // 手机验证码登录
  const loginWithPhoneCodeMutation = useMutation({
    mutationFn: loginWithPhoneCode,
    onSuccess: async (data: any) => {
      await handleAuthSuccess(data)
    },
    onError: (error: any) => {
      toast.error(resolveAuthApiErrorMessage(error, t, t.auth.loginFailed))
    },
  })

  // 注册
  const registerMutation = useMutation({
    mutationFn: register,
    onSuccess: async (data: any) => {
      // 如果需要邮箱验证
      if (data.data?.require_verification) {
        router.push(`/verify-email?email=${encodeURIComponent(data.data.email)}&pending=true`)
        return
      }
      await handleAuthSuccess(data)
    },
    onError: (error: Error) => {
      toast.error(resolveAuthApiErrorMessage(error, t, t.auth.registerFailed))
    },
  })

  // 手机号注册
  const phoneRegisterMutation = useMutation({
    mutationFn: phoneRegister,
    onSuccess: async (data: any) => {
      await handleAuthSuccess(data)
    },
    onError: (error: Error) => {
      toast.error(resolveAuthApiErrorMessage(error, t, t.auth.registerFailed))
    },
  })

  const logoutUser = () => {
    clearToken()
    clearAuthReturnState()
    queryClient.clear()
    router.replace('/login')
  }

  // 判断认证状态
  const hasToken = typeof window !== 'undefined' && !!getToken()
  // 如果有token且正在loading，或者有token且请求成功有用户数据，则认为已认证
  // 如果请求失败（error），说明token无效，返回false
  const isAuthenticated = hasToken && !error && (isLoading || !!user?.data)
  const normalizedUser = useMemo(
    () => normalizeAuthUser(user?.data),
    [user?.data]
  )

  return {
    user: normalizedUser,
    isLoading,
    isAuthenticated,
    login: loginMutation.mutate,
    loginWithCode: loginWithCodeMutation.mutate,
    loginWithPhoneCode: loginWithPhoneCodeMutation.mutate,
    logout: logoutUser,
    isLoggingIn: loginMutation.isPending,
    isLoggingInWithCode: loginWithCodeMutation.isPending,
    isLoggingInWithPhoneCode: loginWithPhoneCodeMutation.isPending,
    register: registerMutation.mutate,
    isRegistering: registerMutation.isPending,
    registerWithPhone: phoneRegisterMutation.mutate,
    isRegisteringWithPhone: phoneRegisterMutation.isPending,
  }
}
