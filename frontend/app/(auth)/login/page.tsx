'use client'

import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import * as z from 'zod'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from '@/components/ui/form'
import { useAuth } from '@/hooks/use-auth'
import { createLoginSchema, loginSchema } from '@/lib/validators'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { Loader2, Mail, Lock, ArrowRight, KeyRound } from 'lucide-react'
import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { useQuery } from '@tanstack/react-query'
import { getPublicConfig, getCaptcha, sendLoginCode } from '@/lib/api'
import { useState, useEffect, useRef } from 'react'
import { useTheme } from '@/contexts/theme-context'
import toast from 'react-hot-toast'
import { AuthBrandingPanel } from '@/components/auth-branding-panel'

export default function LoginPage() {
  const { login, loginWithCode, isLoggingIn, isLoggingInWithCode, isAuthenticated, isLoading } = useAuth()
  const router = useRouter()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.login)

  // 已登录用户自动跳转到商品页
  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      router.replace('/products')
    }
  }, [isLoading, isAuthenticated, router])
  const [captchaToken, setCaptchaToken] = useState('')
  const [builtinCode, setBuiltinCode] = useState('')
  const [loginMode, setLoginMode] = useState<'password' | 'code'>('password')
  const [codeEmail, setCodeEmail] = useState('')
  const [codeValue, setCodeValue] = useState('')
  const [countdown, setCountdown] = useState(0)
  const [isSendingCode, setIsSendingCode] = useState(false)
  const [codeSent, setCodeSent] = useState(false)
  const { resolvedTheme } = useTheme()
  const captchaContainerRef = useRef<HTMLDivElement>(null)
  const widgetRendered = useRef(false)
  const widgetIdRef = useRef<any>(null)

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
  })

  const allowRegistration = publicConfig?.data?.allow_registration
  const smtpEnabled = publicConfig?.data?.smtp_enabled
  const captchaConfig = publicConfig?.data?.captcha
  const needCaptcha = captchaConfig?.provider && captchaConfig.provider !== 'none' && captchaConfig.enable_for_login
  const emailCodeAvailable = smtpEnabled && needCaptcha

  const { data: builtinCaptcha, refetch: refetchCaptcha } = useQuery({
    queryKey: ['captcha', 'login'],
    queryFn: getCaptcha,
    enabled: needCaptcha && captchaConfig?.provider === 'builtin',
  })

  // 60秒倒计时
  useEffect(() => {
    if (countdown <= 0) return
    const timer = setTimeout(() => {
      const next = countdown - 1
      setCountdown(next)
      if (next === 0) {
        setCodeSent(false)
        widgetRendered.current = false
        refetchCaptcha()
        setBuiltinCode('')
      }
    }, 1000)
    return () => clearTimeout(timer)
  }, [countdown, refetchCaptcha])

  // Load Turnstile/reCAPTCHA scripts
  useEffect(() => {
    if (!needCaptcha) return

    if (captchaConfig.provider === 'cloudflare' && !document.getElementById('cf-turnstile-script')) {
      const script = document.createElement('script')
      script.id = 'cf-turnstile-script'
      script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad'
      script.async = true
      ;(window as any).onTurnstileLoad = () => {
        if (captchaContainerRef.current && !widgetRendered.current) {
          widgetRendered.current = true
          widgetIdRef.current = (window as any).turnstile.render(captchaContainerRef.current, {
            sitekey: captchaConfig.site_key,
            theme: resolvedTheme === 'dark' ? 'dark' : 'light',
            callback: (token: string) => setCaptchaToken(token),
            'expired-callback': () => setCaptchaToken(''),
          })
        }
      }
      document.head.appendChild(script)
    } else if (captchaConfig.provider === 'google' && !document.getElementById('recaptcha-script')) {
      const script = document.createElement('script')
      script.id = 'recaptcha-script'
      script.src = 'https://www.google.com/recaptcha/api.js?onload=onRecaptchaLoad&render=explicit'
      script.async = true
      ;(window as any).onRecaptchaLoad = () => {
        if (captchaContainerRef.current && !widgetRendered.current) {
          widgetRendered.current = true
          widgetIdRef.current = (window as any).grecaptcha.render(captchaContainerRef.current, {
            sitekey: captchaConfig.site_key,
            theme: resolvedTheme === 'dark' ? 'dark' : 'light',
            callback: (token: string) => setCaptchaToken(token),
            'expired-callback': () => setCaptchaToken(''),
          })
        }
      }
      document.head.appendChild(script)
    }
  }, [needCaptcha, captchaConfig])

  // Render widget if script already loaded
  useEffect(() => {
    if (!needCaptcha || widgetRendered.current || !captchaContainerRef.current) return

    if (captchaConfig.provider === 'cloudflare' && (window as any).turnstile) {
      widgetRendered.current = true
      widgetIdRef.current = (window as any).turnstile.render(captchaContainerRef.current, {
        sitekey: captchaConfig.site_key,
        theme: resolvedTheme === 'dark' ? 'dark' : 'light',
        callback: (token: string) => setCaptchaToken(token),
        'expired-callback': () => setCaptchaToken(''),
      })
    } else if (captchaConfig.provider === 'google' && (window as any).grecaptcha?.render) {
      widgetRendered.current = true
      widgetIdRef.current = (window as any).grecaptcha.render(captchaContainerRef.current, {
        sitekey: captchaConfig.site_key,
        theme: resolvedTheme === 'dark' ? 'dark' : 'light',
        callback: (token: string) => setCaptchaToken(token),
        'expired-callback': () => setCaptchaToken(''),
      })
    }
  }, [needCaptcha, captchaConfig, loginMode])

  const schema = createLoginSchema({
    invalidEmail: t.auth.invalidEmail,
    passwordMin6: (t.auth.passwordMinLength as string).replace('{n}', '6'),
  })

  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: {
      email: '',
      password: '',
    },
  })

  function resetCaptcha() {
    if (!needCaptcha) return
    if (captchaConfig.provider === 'builtin') {
      refetchCaptcha()
      setBuiltinCode('')
    } else if (captchaConfig.provider === 'cloudflare' && (window as any).turnstile) {
      ;(window as any).turnstile.reset(widgetIdRef.current)
      setCaptchaToken('')
    } else if (captchaConfig.provider === 'google' && (window as any).grecaptcha) {
      ;(window as any).grecaptcha.reset(widgetIdRef.current)
      setCaptchaToken('')
    }
  }

  function onSubmit(values: z.infer<typeof loginSchema>) {
    let token = captchaToken
    if (needCaptcha && captchaConfig.provider === 'builtin') {
      token = `${builtinCaptcha?.data?.captcha_id}:${builtinCode}`
    }
    login({ ...values, captcha_token: token || undefined }, {
      onError: () => resetCaptcha(),
    })
  }

  async function handleSendCode() {
    if (!codeEmail || countdown > 0 || isSendingCode) return
    let token = captchaToken
    if (needCaptcha && captchaConfig.provider === 'builtin') {
      token = `${builtinCaptcha?.data?.captcha_id}:${builtinCode}`
    }
    setIsSendingCode(true)
    try {
      await sendLoginCode({ email: codeEmail, captcha_token: token || undefined })
      toast.success(t.auth.codeSent)
      setCountdown(60)
      setCodeSent(true)
    } catch (e: any) {
      const msg = e?.code === 42902 ? t.auth.cooldownWait : (e?.message || t.auth.requestFailed)
      toast.error(msg)
      if (e?.code !== 42902) resetCaptcha()
    } finally {
      setIsSendingCode(false)
    }
  }

  function onCodeSubmit() {
    if (!codeEmail || !codeValue) return
    loginWithCode({ email: codeEmail, code: codeValue }, {
      onError: () => setCodeValue(''),
    })
  }

  return (
    <div className="min-h-screen flex">
      <AuthBrandingPanel />

      {/* Right form panel */}
      <div className="flex-1 flex items-center justify-center p-6 sm:p-12 bg-background">
        <div className="w-full max-w-sm space-y-6 sm:space-y-8">
          {/* Mobile logo */}
          <div className="lg:hidden text-center">
            <h1 className="text-2xl font-bold text-foreground tracking-tight">
              AuraLogic
            </h1>
          </div>

          {/* Header */}
          <div className="space-y-2">
            <h2 className="text-2xl font-semibold tracking-tight text-foreground">
              {locale === 'zh' ? '欢迎回来' : 'Welcome back'}
            </h2>
            <p className="text-sm text-muted-foreground">
              {locale === 'zh' ? '请登录您的账户' : 'Sign in to your account'}
            </p>
          </div>

          {/* Mode switcher */}
          {emailCodeAvailable && (
            <div className="flex rounded-lg border border-border p-1 bg-muted/50">
              <button
                type="button"
                className={`flex-1 text-xs sm:text-sm py-2 rounded-md transition-colors whitespace-nowrap ${loginMode === 'password' ? 'bg-background text-foreground shadow-sm font-medium' : 'text-muted-foreground hover:text-foreground'}`}
                onClick={() => { setLoginMode('password'); widgetRendered.current = false }}
              >
                <Lock className="inline h-3.5 w-3.5 mr-1 -mt-0.5" />
                {t.auth.passwordLogin}
              </button>
              <button
                type="button"
                className={`flex-1 text-xs sm:text-sm py-2 rounded-md transition-colors whitespace-nowrap ${loginMode === 'code' ? 'bg-background text-foreground shadow-sm font-medium' : 'text-muted-foreground hover:text-foreground'}`}
                onClick={() => { setLoginMode('code'); widgetRendered.current = false }}
              >
                <KeyRound className="inline h-3.5 w-3.5 mr-1 -mt-0.5" />
                {t.auth.emailCodeLogin}
              </button>
            </div>
          )}

          {/* Password login form */}
          {loginMode === 'password' && <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-5">
              <FormField
                control={form.control}
                name="email"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel className="text-sm font-medium">{t.auth.email}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input
                          type="email"
                          placeholder={t.auth.emailPlaceholder}
                          className="pl-10 h-11"
                          {...field}
                        />
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="password"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <div className="flex items-center justify-between">
                      <FormLabel className="text-sm font-medium">{t.auth.password}</FormLabel>
                      {smtpEnabled && (
                        <Link href="/forgot-password" className="text-xs text-muted-foreground hover:text-primary transition-colors">
                          {t.auth.forgotPassword}
                        </Link>
                      )}
                    </div>
                    <FormControl>
                      <div className="relative">
                        <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input
                          type="password"
                          placeholder={t.auth.passwordPlaceholder}
                          className="pl-10 h-11"
                          {...field}
                        />
                      </div>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {/* Captcha */}
              {needCaptcha && (
                <div className="space-y-2">
                  {(captchaConfig.provider === 'cloudflare' || captchaConfig.provider === 'google') && (
                    <div ref={captchaContainerRef} />
                  )}
                  {captchaConfig.provider === 'builtin' && builtinCaptcha?.data && (
                    <>
                      <label className="text-sm font-medium">{t.auth.captcha}</label>
                      <div className="flex items-center gap-2">
                        <Input
                          placeholder={t.auth.captchaPlaceholder}
                          value={builtinCode}
                          onChange={(e) => setBuiltinCode(e.target.value)}
                          maxLength={4}
                          className="h-11"
                        />
                        <img
                          src={builtinCaptcha.data.image}
                          alt="captcha"
                          className="border border-border rounded-md h-11 shrink-0 cursor-pointer dark:brightness-90"
                          onClick={() => { refetchCaptcha(); setBuiltinCode('') }}
                          title={t.auth.captchaRefresh}
                        />
                      </div>
                    </>
                  )}
                </div>
              )}

              <Button
                type="submit"
                className="w-full h-11 text-sm font-medium"
                disabled={isLoggingIn}
              >
                {isLoggingIn ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    {t.auth.loggingIn}
                  </>
                ) : (
                  <>
                    {t.auth.login}
                    <ArrowRight className="ml-2 h-4 w-4" />
                  </>
                )}
              </Button>
            </form>
          </Form>}

          {/* Email code login form */}
          {loginMode === 'code' && (
            <div className="space-y-5">
              <div className="space-y-2">
                <label className="text-sm font-medium">{t.auth.email}</label>
                <div className="relative">
                  <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                  <Input
                    type="email"
                    placeholder={t.auth.emailPlaceholder}
                    className="pl-10 h-11"
                    value={codeEmail}
                    onChange={(e) => setCodeEmail(e.target.value)}
                  />
                </div>
              </div>

              {/* Captcha - hide after code sent */}
              {needCaptcha && !codeSent && (
                <div className="space-y-2">
                  {(captchaConfig.provider === 'cloudflare' || captchaConfig.provider === 'google') && (
                    <div ref={captchaContainerRef} />
                  )}
                  {captchaConfig.provider === 'builtin' && builtinCaptcha?.data && (
                    <>
                      <label className="text-sm font-medium">{t.auth.captcha}</label>
                      <div className="flex items-center gap-2">
                        <Input
                          placeholder={t.auth.captchaPlaceholder}
                          value={builtinCode}
                          onChange={(e) => setBuiltinCode(e.target.value)}
                          maxLength={4}
                          className="h-11"
                        />
                        <img
                          src={builtinCaptcha.data.image}
                          alt="captcha"
                          className="border border-border rounded-md h-11 shrink-0 cursor-pointer dark:brightness-90"
                          onClick={() => { refetchCaptcha(); setBuiltinCode('') }}
                          title={t.auth.captchaRefresh}
                        />
                      </div>
                    </>
                  )}
                </div>
              )}

              <div className="space-y-2">
                <label className="text-sm font-medium">{t.auth.emailCode}</label>
                <div className="flex gap-2">
                  <Input
                    placeholder={t.auth.codePlaceholder}
                    value={codeValue}
                    onChange={(e) => setCodeValue(e.target.value.replace(/\D/g, '').slice(0, 6))}
                    maxLength={6}
                    className="h-11"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    className="h-11 shrink-0 text-sm"
                    disabled={!codeEmail || countdown > 0 || isSendingCode}
                    onClick={handleSendCode}
                  >
                    {isSendingCode ? t.auth.sendingCode
                      : countdown > 0 ? (t.auth.codeResendIn as string).replace('{n}', String(countdown))
                      : t.auth.sendCode}
                  </Button>
                </div>
              </div>

              <Button
                className="w-full h-11 text-sm font-medium"
                disabled={isLoggingInWithCode || codeValue.length !== 6}
                onClick={onCodeSubmit}
              >
                {isLoggingInWithCode ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    {t.auth.loggingIn}
                  </>
                ) : (
                  <>
                    {t.auth.login}
                    <ArrowRight className="ml-2 h-4 w-4" />
                  </>
                )}
              </Button>
            </div>
          )}

          {/* Footer */}
          {allowRegistration && (
            <p className="text-center text-xs text-muted-foreground">
              {t.auth.noAccount}{' '}
              <Link href="/register" className="text-primary hover:underline">
                {t.auth.register}
              </Link>
            </p>
          )}
        </div>
      </div>
    </div>
  )
}
