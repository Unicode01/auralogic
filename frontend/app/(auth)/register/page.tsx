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
import { createRegisterSchema, registerSchema } from '@/lib/validators'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { Loader2, Mail, Lock, ArrowRight, User, Phone, KeyRound } from 'lucide-react'
import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import { getPublicConfig, getCaptcha, sendPhoneRegisterCode } from '@/lib/api'
import { useRouter } from 'next/navigation'
import { useEffect, useState, useRef, useCallback } from 'react'
import toast from 'react-hot-toast'
import { useTheme } from '@/contexts/theme-context'
import { AuthBrandingPanel } from '@/components/auth-branding-panel'
import { PhoneInput } from '@/components/phone-input'

export default function RegisterPage() {
  const { register: registerUser, isRegistering, registerWithPhone, isRegisteringWithPhone } = useAuth()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.register)
  const router = useRouter()
  const [captchaToken, setCaptchaToken] = useState('')
  const [builtinCode, setBuiltinCode] = useState('')
  const { resolvedTheme } = useTheme()
  const captchaContainerRef = useRef<HTMLDivElement>(null)
  const widgetRendered = useRef(false)
  const widgetIdRef = useRef<any>(null)
  const [mode, setMode] = useState<'email' | 'phone'>('email')
  const [phoneNumber, setPhoneNumber] = useState('')
  const [phoneCountryCode, setPhoneCountryCode] = useState('+86')
  const [phoneName, setPhoneName] = useState('')
  const [phonePassword, setPhonePassword] = useState('')
  const [phoneConfirmPassword, setPhoneConfirmPassword] = useState('')
  const [phoneCode, setPhoneCode] = useState('')
  const [countdown, setCountdown] = useState(0)
  const [sendingCode, setSendingCode] = useState(false)

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
  })

  const allowRegistration = publicConfig?.data?.allow_registration
  const allowPhoneRegister = publicConfig?.data?.allow_phone_register
  const captchaConfig = publicConfig?.data?.captcha
  const needCaptcha = captchaConfig?.provider && captchaConfig.provider !== 'none' && captchaConfig.enable_for_register

  const { data: builtinCaptcha, refetch: refetchCaptcha } = useQuery({
    queryKey: ['captcha', 'register'],
    queryFn: getCaptcha,
    enabled: needCaptcha && captchaConfig?.provider === 'builtin',
  })

  // 验证码超时自动刷新（后端TTL为5分钟，提前30秒刷新）
  useEffect(() => {
    if (!needCaptcha || captchaConfig?.provider !== 'builtin') return
    const timer = setInterval(() => {
      refetchCaptcha()
      setBuiltinCode('')
    }, 270000)
    return () => clearInterval(timer)
  }, [needCaptcha, captchaConfig?.provider, refetchCaptcha])

  // If all registration is disabled, redirect to login
  useEffect(() => {
    if (publicConfig && !allowRegistration && !allowPhoneRegister) {
      router.replace('/login')
    }
  }, [publicConfig, allowRegistration, allowPhoneRegister, router])

  // Countdown timer for phone code
  useEffect(() => {
    if (countdown <= 0) return
    const timer = setTimeout(() => setCountdown(countdown - 1), 1000)
    return () => clearTimeout(timer)
  }, [countdown])

  const handleSendPhoneCode = useCallback(async () => {
    if (!phoneNumber || sendingCode || countdown > 0) return
    setSendingCode(true)
    try {
      let token = captchaToken
      if (needCaptcha && captchaConfig?.provider === 'builtin') {
        token = `${builtinCaptcha?.data?.captcha_id}:${builtinCode}`
      }
      await sendPhoneRegisterCode({ phone: phoneNumber, phone_code: phoneCountryCode, captcha_token: token || undefined })
      toast.success(t.auth.phoneCodeSent)
      setCountdown(60)
    } catch {
      toast.error(t.auth.requestFailed)
      resetCaptcha()
    } finally {
      setSendingCode(false)
    }
  }, [phoneNumber, sendingCode, countdown, captchaToken, needCaptcha, captchaConfig, builtinCaptcha, builtinCode])

  function onPhoneSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!phoneNumber || !phoneName || !phonePassword || phoneCode.length !== 6) return
    if (phonePassword !== phoneConfirmPassword) {
      toast.error(t.auth.passwordMismatch)
      return
    }
    let token = captchaToken
    if (needCaptcha && captchaConfig?.provider === 'builtin') {
      token = `${builtinCaptcha?.data?.captcha_id}:${builtinCode}`
    }
    registerWithPhone({
      phone: phoneNumber,
      phone_code: phoneCountryCode,
      name: phoneName,
      password: phonePassword,
      code: phoneCode,
      captcha_token: token || undefined,
    }, {
      onError: () => resetCaptcha(),
    })
  }

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
  }, [needCaptcha, captchaConfig])

  const schema = createRegisterSchema({
    invalidEmail: t.auth.invalidEmail,
    passwordMin8: (t.auth.passwordMinLength as string).replace('{n}', '8'),
    nameMin2: (t.auth.nameMinLength as string).replace('{n}', '2'),
    passwordMismatch: t.auth.passwordMismatch,
  })

  const form = useForm({
    resolver: zodResolver(schema),
    defaultValues: {
      email: '',
      name: '',
      password: '',
      confirm_password: '',
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

  function onSubmit(values: z.infer<typeof registerSchema>) {
    let token = captchaToken
    if (needCaptcha && captchaConfig.provider === 'builtin') {
      token = `${builtinCaptcha?.data?.captcha_id}:${builtinCode}`
    }
    registerUser({
      email: values.email,
      name: values.name,
      password: values.password,
      captcha_token: token || undefined,
    }, {
      onError: () => resetCaptcha(),
    })
  }

  if (publicConfig && !allowRegistration && !allowPhoneRegister) {
    return null
  }

  const showModeSwitcher = allowRegistration && allowPhoneRegister

  return (
    <div className="min-h-screen flex">
      <AuthBrandingPanel />

      {/* Right form panel */}
      <div className="flex-1 flex items-center justify-center p-6 sm:p-12 bg-background">
        <div className="w-full max-w-sm space-y-4 sm:space-y-6">
          {/* Mobile logo */}
          <div className="lg:hidden text-center">
            <h1 className="text-2xl font-bold text-foreground tracking-tight">
              AuraLogic
            </h1>
          </div>

          {/* Header */}
          <div className="space-y-2">
            <h2 className="text-2xl font-semibold tracking-tight text-foreground">
              {mode === 'phone' ? t.auth.phoneRegister : t.auth.welcomeRegister}
            </h2>
            <p className="text-sm text-muted-foreground">
              {mode === 'phone' ? t.auth.phoneRegisterDesc : t.auth.registerDescription}
            </p>
          </div>

          {/* Mode switcher */}
          {showModeSwitcher && (
            <div className="flex gap-2 p-1 bg-muted rounded-lg">
              <button type="button" onClick={() => { setMode('email'); widgetRendered.current = false }}
                className={`flex-1 py-1.5 text-xs font-medium rounded-md transition-colors ${mode === 'email' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}>
                {t.auth.register}
              </button>
              <button type="button" onClick={() => { setMode('phone'); widgetRendered.current = false }}
                className={`flex-1 py-1.5 text-xs font-medium rounded-md transition-colors ${mode === 'phone' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}>
                {t.auth.phoneRegister}
              </button>
            </div>
          )}

          {/* Phone registration form */}
          {mode === 'phone' ? (
            <form onSubmit={onPhoneSubmit} className="space-y-3">
              <div className="space-y-1.5">
                <label className="text-sm font-medium">{t.auth.phone}</label>
                <PhoneInput countryCode={phoneCountryCode} onCountryCodeChange={setPhoneCountryCode}
                  phone={phoneNumber} onPhoneChange={setPhoneNumber} placeholder={t.auth.phonePlaceholder} />
              </div>

              <div className="space-y-1.5">
                <label className="text-sm font-medium">{t.auth.name}</label>
                <div className="relative">
                  <User className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                  <Input placeholder={t.auth.namePlaceholder} className="pl-10 h-10"
                    value={phoneName} onChange={(e) => setPhoneName(e.target.value)} />
                </div>
              </div>

              <div className="space-y-1.5">
                <label className="text-sm font-medium">{t.auth.password}</label>
                <div className="relative">
                  <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                  <Input type="password" placeholder={t.auth.passwordPlaceholder} className="pl-10 h-10"
                    value={phonePassword} onChange={(e) => setPhonePassword(e.target.value)} />
                </div>
              </div>

              <div className="space-y-1.5">
                <label className="text-sm font-medium">{t.auth.confirmPassword}</label>
                <div className="relative">
                  <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                  <Input type="password" placeholder={t.auth.confirmPasswordPlaceholder} className="pl-10 h-10"
                    value={phoneConfirmPassword} onChange={(e) => setPhoneConfirmPassword(e.target.value)} />
                </div>
              </div>

              {/* Captcha - must complete before requesting SMS code */}
              {needCaptcha && (
                <div className="space-y-1.5">
                  {(captchaConfig.provider === 'cloudflare' || captchaConfig.provider === 'google') && (
                    <div ref={captchaContainerRef} />
                  )}
                  {captchaConfig.provider === 'builtin' && builtinCaptcha?.data && (
                    <>
                      <label className="text-sm font-medium">{t.auth.captcha}</label>
                      <div className="flex items-center gap-2">
                        <Input placeholder={t.auth.captchaPlaceholder} value={builtinCode}
                          onChange={(e) => setBuiltinCode(e.target.value)} maxLength={4} className="h-10" />
                        <img src={builtinCaptcha.data.image} alt="captcha"
                          className="border border-border rounded-md h-10 shrink-0 cursor-pointer dark:brightness-90"
                          onClick={() => { refetchCaptcha(); setBuiltinCode('') }} title={t.auth.captchaRefresh} />
                      </div>
                    </>
                  )}
                </div>
              )}

              <div className="space-y-1.5">
                <label className="text-sm font-medium">{t.auth.phoneCode}</label>
                <div className="flex gap-2">
                  <div className="relative flex-1">
                    <KeyRound className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                    <Input placeholder={t.auth.phoneCodePlaceholder} className="pl-10 h-10"
                      value={phoneCode} onChange={(e) => setPhoneCode(e.target.value)} maxLength={6} />
                  </div>
                  <Button type="button" variant="outline" className="h-10 shrink-0 text-xs"
                    disabled={!phoneNumber || sendingCode || countdown > 0 || (needCaptcha && !captchaToken && !(captchaConfig?.provider === 'builtin' && builtinCode))} onClick={handleSendPhoneCode}>
                    {sendingCode ? t.auth.sendingCode : countdown > 0 ? (t.auth.codeResendIn as string).replace('{n}', String(countdown)) : t.auth.sendPhoneCode}
                  </Button>
                </div>
              </div>

              <Button type="submit" className="w-full h-10 text-sm font-medium" disabled={isRegisteringWithPhone || phoneCode.length !== 6}>
                {isRegisteringWithPhone ? (
                  <><Loader2 className="mr-2 h-4 w-4 animate-spin" />{t.auth.registering}</>
                ) : (
                  <>{t.auth.createAccount}<ArrowRight className="ml-2 h-4 w-4" /></>
                )}
              </Button>
            </form>
          ) : (
          /* Email registration form */
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-3">
              <FormField
                control={form.control}
                name="email"
                render={({ field }) => (
                  <FormItem className="space-y-1.5">
                    <FormLabel className="text-sm font-medium">{t.auth.email}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input
                          type="email"
                          placeholder={t.auth.emailPlaceholder}
                          className="pl-10 h-10"
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
                name="name"
                render={({ field }) => (
                  <FormItem className="space-y-1.5">
                    <FormLabel className="text-sm font-medium">{t.auth.name}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <User className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input
                          type="text"
                          placeholder={t.auth.namePlaceholder}
                          className="pl-10 h-10"
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
                  <FormItem className="space-y-1.5">
                    <FormLabel className="text-sm font-medium">{t.auth.password}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input
                          type="password"
                          placeholder={t.auth.passwordPlaceholder}
                          className="pl-10 h-10"
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
                name="confirm_password"
                render={({ field }) => (
                  <FormItem className="space-y-1.5">
                    <FormLabel className="text-sm font-medium">{t.auth.confirmPassword}</FormLabel>
                    <FormControl>
                      <div className="relative">
                        <Lock className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                        <Input
                          type="password"
                          placeholder={t.auth.confirmPasswordPlaceholder}
                          className="pl-10 h-10"
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
                <div className="space-y-1.5">
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
                          className="h-10"
                        />
                        <img
                          src={builtinCaptcha.data.image}
                          alt="captcha"
                          className="border border-border rounded-md h-10 shrink-0 cursor-pointer dark:brightness-90"
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
                className="w-full h-10 text-sm font-medium"
                disabled={isRegistering}
              >
                {isRegistering ? (
                  <>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    {t.auth.registering}
                  </>
                ) : (
                  <>
                    {t.auth.createAccount}
                    <ArrowRight className="ml-2 h-4 w-4" />
                  </>
                )}
              </Button>
            </form>
          </Form>
          )}

          {/* Footer */}
          <p className="text-center text-xs text-muted-foreground">
            {t.auth.hasAccount}{' '}
            <Link href="/login" className="text-primary hover:underline">
              {t.auth.login}
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
