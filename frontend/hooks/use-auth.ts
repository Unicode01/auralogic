'use client'

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getCurrentUser, login, loginWithCode, logout, register } from '@/lib/api'
import { setToken, clearToken, setUser } from '@/lib/auth'
import { useRouter } from 'next/navigation'
import toast from 'react-hot-toast'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'

type Locale = 'zh' | 'en'

// Map backend error messages to i18n keys
const errorMessageMap: Record<string, keyof ReturnType<typeof getTranslations>['auth']> = {
  'Invalid email or password': 'invalidEmailOrPassword',
  'User account has been disabled': 'accountDisabled',
  'Password login is disabled, please use quick login or OAuth login': 'passwordLoginDisabled',
  'Please verify your email before logging in': 'emailNotVerified',
  'Captcha is required': 'captchaRequired',
  'Captcha verification failed': 'captchaFailed',
  'Email already in use': 'emailAlreadyInUse',
  'Phone number already in use': 'phoneAlreadyInUse',
  'Registration is disabled': 'registrationDisabled',
  'Invalid request parameters': 'invalidParams',
  'Password must contain at least one uppercase letter': 'passwordNeedUppercase',
  'Password must contain at least one lowercase letter': 'passwordNeedLowercase',
  'Password must contain at least one digit': 'passwordNeedDigit',
  'Password must contain at least one special character': 'passwordNeedSpecial',
  'Verification code expired or invalid': 'codeExpired',
  'Invalid verification code': 'invalidCode',
}

export function useAuth() {
  const router = useRouter()
  const queryClient = useQueryClient()
  const { locale, setLocale } = useLocale()
  const t = getTranslations(locale)

  function getErrorMessage(error: Error, fallback: string): string {
    const msg = error.message
    // Exact match
    const key = errorMessageMap[msg]
    if (key && t.auth[key]) {
      return t.auth[key] as string
    }
    // Dynamic match: "Password must be at least X characters"
    const minLenMatch = msg.match(/^Password must be at least (\d+) characters$/)
    if (minLenMatch) {
      return (t.auth.passwordTooShort as string).replace('{n}', minLenMatch[1])
    }
    return fallback
  }

  // 获取当前用户
  const { data: user, isLoading, error } = useQuery({
    queryKey: ['currentUser'],
    queryFn: getCurrentUser,
    retry: false,
    enabled: typeof window !== 'undefined' && !!localStorage.getItem('auth_token'),
  })

  // 登录
  const loginMutation = useMutation({
    mutationFn: login,
    onSuccess: (data: any) => {
      setToken(data.data.token)
      const user = data.data.user

      // If the frontend initialized locale before login, it will be stored here.
      // Prefer pending-local locale for first-run/default initialization, then server locale, then stored.
      const pending = typeof window !== 'undefined' ? localStorage.getItem('auralogic_locale_pending_sync') : null
      const stored = typeof window !== 'undefined' ? localStorage.getItem('auralogic_locale') : null
      const serverLocale = user?.locale

      let desired: Locale | null = null
      if (pending === 'zh' || pending === 'en') desired = pending
      else if (serverLocale === 'zh' || serverLocale === 'en') desired = serverLocale
      else if (stored === 'zh' || stored === 'en') desired = stored

      const finalUser = desired ? { ...user, locale: desired } : user
      setUser(finalUser)

      if (desired) {
        // Triggers backend sync (and clears pending on success) via LocaleProvider.
        setLocale(desired)
      }
      // 设置正确的缓存数据结构
      queryClient.setQueryData(['currentUser'], { data: finalUser })
      router.push('/orders')
    },
    onError: (error: any) => {
      // 邮箱未验证，跳转到验证页面
      if (error.code === 30003 && error.data?.email) {
        router.push(`/verify-email?email=${encodeURIComponent(error.data.email)}&pending=true`)
        return
      }
      toast.error(getErrorMessage(error, t.auth.loginFailed))
    },
  })

  // 验证码登录
  const loginWithCodeMutation = useMutation({
    mutationFn: loginWithCode,
    onSuccess: (data: any) => {
      setToken(data.data.token)
      const user = data.data.user
      const pending = typeof window !== 'undefined' ? localStorage.getItem('auralogic_locale_pending_sync') : null
      const stored = typeof window !== 'undefined' ? localStorage.getItem('auralogic_locale') : null
      const serverLocale = user?.locale
      let desired: Locale | null = null
      if (pending === 'zh' || pending === 'en') desired = pending
      else if (serverLocale === 'zh' || serverLocale === 'en') desired = serverLocale
      else if (stored === 'zh' || stored === 'en') desired = stored
      const finalUser = desired ? { ...user, locale: desired } : user
      setUser(finalUser)
      if (desired) setLocale(desired)
      queryClient.setQueryData(['currentUser'], { data: finalUser })
      router.push('/orders')
    },
    onError: (error: any) => {
      toast.error(getErrorMessage(error, t.auth.loginFailed))
    },
  })

  // 注册
  const registerMutation = useMutation({
    mutationFn: register,
    onSuccess: (data: any) => {
      // 如果需要邮箱验证
      if (data.data?.require_verification) {
        router.push(`/verify-email?email=${encodeURIComponent(data.data.email)}&pending=true`)
        return
      }
      setToken(data.data.token)
      const user = data.data.user

      const pending = typeof window !== 'undefined' ? localStorage.getItem('auralogic_locale_pending_sync') : null
      const stored = typeof window !== 'undefined' ? localStorage.getItem('auralogic_locale') : null
      const serverLocale = user?.locale

      let desired: Locale | null = null
      if (pending === 'zh' || pending === 'en') desired = pending
      else if (serverLocale === 'zh' || serverLocale === 'en') desired = serverLocale
      else if (stored === 'zh' || stored === 'en') desired = stored

      const finalUser = desired ? { ...user, locale: desired } : user
      setUser(finalUser)

      if (desired) {
        setLocale(desired)
      }

      queryClient.setQueryData(['currentUser'], { data: finalUser })
      router.push('/orders')
    },
    onError: (error: Error) => {
      toast.error(getErrorMessage(error, t.auth.registerFailed))
    },
  })

  // 登出
  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: () => {
      clearToken()
      queryClient.clear()
      router.push('/login')
    },
  })

  // 判断认证状态
  const hasToken = typeof window !== 'undefined' && !!localStorage.getItem('auth_token')
  // 如果有token且正在loading，或者有token且请求成功有用户数据，则认为已认证
  // 如果请求失败（error），说明token无效，返回false
  const isAuthenticated = hasToken && !error && (isLoading || !!user?.data)

  return {
    user: user?.data,
    isLoading,
    isAuthenticated,
    login: loginMutation.mutate,
    loginWithCode: loginWithCodeMutation.mutate,
    logout: logoutMutation.mutate,
    isLoggingIn: loginMutation.isPending,
    isLoggingInWithCode: loginWithCodeMutation.isPending,
    register: registerMutation.mutate,
    isRegistering: registerMutation.isPending,
  }
}
