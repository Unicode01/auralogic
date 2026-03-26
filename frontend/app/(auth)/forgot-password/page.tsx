'use client'
/* eslint-disable @next/next/no-img-element */

import { Suspense, useState, useEffect, useMemo, useRef } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { Loader2, Mail, ArrowLeft, Phone, Lock, KeyRound, Eye, EyeOff } from 'lucide-react'
import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import {
  getPublicConfig,
  getCaptcha,
  forgotPassword,
  phoneForgotPassword,
  phoneResetPassword,
} from '@/lib/api'
import { resolveAuthApiErrorMessage } from '@/lib/api-error'
import { useTheme } from '@/contexts/theme-context'
import toast from 'react-hot-toast'
import { AuthBrandingPanel } from '@/components/auth-branding-panel'
import { PhoneInput } from '@/components/phone-input'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import { PluginSlotBatchBoundary } from '@/lib/plugin-slot-batch'
import { useRouter } from 'next/navigation'

export default function ForgotPasswordPage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.forgotPassword)
  const { resolvedTheme } = useTheme()
  const router = useRouter()

  const [resetMode, setResetMode] = useState<'email' | 'phone'>('email')
  const [email, setEmail] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [sent, setSent] = useState(false)
  const [countdown, setCountdown] = useState(0)
  const [captchaToken, setCaptchaToken] = useState('')
  const [builtinCode, setBuiltinCode] = useState('')
  const captchaContainerRef = useRef<HTMLDivElement>(null)
  const widgetRendered = useRef(false)
  const widgetIdRef = useRef<any>(null)
  // Phone reset state
  const [phoneNumber, setPhoneNumber] = useState('')
  const [phoneCountryCode, setPhoneCountryCode] = useState('+86')
  const [phoneCode, setPhoneCode] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [phoneCountdown, setPhoneCountdown] = useState(0)
  const [isSendingPhoneCode, setIsSendingPhoneCode] = useState(false)
  const [phoneCodeSent, setPhoneCodeSent] = useState(false)
  const [isResettingPhone, setIsResettingPhone] = useState(false)
  const [showNewPassword, setShowNewPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
  })

  const allowPasswordReset = publicConfig?.data?.allow_password_reset
  const smsEnabled = publicConfig?.data?.sms_enabled
  const allowPhonePasswordReset = publicConfig?.data?.allow_phone_password_reset
  const phoneResetAvailable = smsEnabled && allowPhonePasswordReset
  // 密码重置被禁用时跳转回登录页
  useEffect(() => {
    if (publicConfig && !allowPasswordReset && !phoneResetAvailable) {
      router.replace('/login')
    }
  }, [publicConfig, allowPasswordReset, phoneResetAvailable, router])

  // Auto-switch to phone mode when email reset disabled
  useEffect(() => {
    if (publicConfig && !allowPasswordReset && phoneResetAvailable) {
      setResetMode('phone')
    }
  }, [publicConfig, allowPasswordReset, phoneResetAvailable])

  const captchaConfig = publicConfig?.data?.captcha
  const needCaptcha =
    captchaConfig?.provider && captchaConfig.provider !== 'none' && captchaConfig.enable_for_login

  const { data: builtinCaptcha, refetch: refetchCaptcha } = useQuery({
    queryKey: ['captcha', 'forgot'],
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

  useEffect(() => {
    if (countdown <= 0) return
    const timer = setTimeout(() => {
      const next = countdown - 1
      setCountdown(next)
      if (next === 0) {
        setSent(false)
        widgetRendered.current = false
        refetchCaptcha()
        setBuiltinCode('')
      }
    }, 1000)
    return () => clearTimeout(timer)
  }, [countdown, refetchCaptcha])

  // Phone countdown
  useEffect(() => {
    if (phoneCountdown <= 0) return
    const timer = setTimeout(() => {
      const next = phoneCountdown - 1
      setPhoneCountdown(next)
      if (next === 0) setPhoneCodeSent(false)
    }, 1000)
    return () => clearTimeout(timer)
  }, [phoneCountdown])

  // Load third-party captcha scripts
  useEffect(() => {
    if (!needCaptcha) return
    if (
      captchaConfig.provider === 'cloudflare' &&
      !document.getElementById('cf-turnstile-script')
    ) {
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
    } else if (
      captchaConfig.provider === 'google' &&
      !document.getElementById('recaptcha-script')
    ) {
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
  }, [needCaptcha, captchaConfig, resolvedTheme])

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
  }, [needCaptcha, captchaConfig, resetMode, resolvedTheme])

  // Auto-submit/send when CF/Google captcha completes
  useEffect(() => {
    if (!captchaToken || !needCaptcha || captchaConfig?.provider === 'builtin') return
    if (resetMode === 'email' && email && !sent && !isSubmitting && countdown <= 0) {
      handleSubmit({ preventDefault: () => {} } as React.FormEvent)
    } else if (
      resetMode === 'phone' &&
      phoneNumber &&
      !phoneCodeSent &&
      !isSendingPhoneCode &&
      phoneCountdown <= 0
    ) {
      handleSendPhoneCode()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [captchaToken])

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

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!email || isSubmitting) return

    let token = captchaToken
    if (needCaptcha && captchaConfig.provider === 'builtin') {
      token = `${builtinCaptcha?.data?.captcha_id}:${builtinCode}`
    }

    setIsSubmitting(true)
    try {
      await forgotPassword({ email, captcha_token: token || undefined })
      toast.success(t.auth.resetEmailSent)
      setSent(true)
      setCountdown(60)
      resetCaptcha()
    } catch (e: any) {
      toast.error(resolveAuthApiErrorMessage(e, t, t.auth.requestFailed))
      resetCaptcha()
    } finally {
      setIsSubmitting(false)
    }
  }

  async function handleSendPhoneCode() {
    if (!phoneNumber || phoneCountdown > 0 || isSendingPhoneCode) return
    let token = captchaToken
    if (needCaptcha && captchaConfig.provider === 'builtin') {
      token = `${builtinCaptcha?.data?.captcha_id}:${builtinCode}`
    }
    setIsSendingPhoneCode(true)
    try {
      await phoneForgotPassword({
        phone: phoneNumber,
        phone_code: phoneCountryCode,
        captcha_token: token || undefined,
      })
      toast.success(t.auth.phoneResetCodeSent)
      setPhoneCountdown(60)
      setPhoneCodeSent(true)
      resetCaptcha()
    } catch (e: any) {
      toast.error(resolveAuthApiErrorMessage(e, t, t.auth.requestFailed))
      resetCaptcha()
    } finally {
      setIsSendingPhoneCode(false)
    }
  }

  async function handlePhoneReset(e: React.FormEvent) {
    e.preventDefault()
    if (!phoneNumber || !phoneCode || !newPassword || isResettingPhone) return
    if (newPassword !== confirmPassword) {
      toast.error(t.auth.passwordMismatch)
      return
    }
    setIsResettingPhone(true)
    try {
      await phoneResetPassword({
        phone: phoneNumber,
        phone_code: phoneCountryCode,
        code: phoneCode,
        new_password: newPassword,
      })
      toast.success(t.auth.passwordResetSuccess)
      router.push('/login')
    } catch (e: any) {
      toast.error(resolveAuthApiErrorMessage(e, t, t.auth.requestFailed))
    } finally {
      setIsResettingPhone(false)
    }
  }

  const authForgotPasswordPluginContext = {
    view: 'auth_forgot_password',
    auth: {
      mode: resetMode,
    },
    capabilities: {
      email_reset_enabled: Boolean(allowPasswordReset),
      phone_reset_enabled: Boolean(phoneResetAvailable),
      captcha_required: Boolean(needCaptcha),
    },
    state: {
      mode: resetMode,
      email_sent: sent,
      email_countdown: countdown,
      email_countdown_active: countdown > 0,
      phone_code_sent: phoneCodeSent,
      phone_code_countdown: phoneCountdown,
      phone_countdown_active: phoneCountdown > 0,
      is_submitting: isSubmitting,
      is_resetting_phone: isResettingPhone,
      sending_phone_code: isSendingPhoneCode,
      mode_switcher_visible: Boolean(allowPasswordReset && phoneResetAvailable),
    },
  }
  const forgotPasswordBatchItems = useMemo(
    () => [
      {
        slot: 'auth.forgot_password.top',
        hostContext: authForgotPasswordPluginContext,
      },
      {
        slot: 'auth.forgot_password.mode.after',
        hostContext: { ...authForgotPasswordPluginContext, section: 'mode_switcher' },
      },
      {
        slot: 'auth.forgot_password.email.alert.after',
        hostContext: { ...authForgotPasswordPluginContext, section: 'email_form' },
      },
      {
        slot: 'auth.forgot_password.email.submit.before',
        hostContext: { ...authForgotPasswordPluginContext, section: 'email_form' },
      },
      {
        slot: 'auth.forgot_password.email.form.after',
        hostContext: { ...authForgotPasswordPluginContext, section: 'email_form' },
      },
      {
        slot: 'auth.forgot_password.phone.alert.after',
        hostContext: { ...authForgotPasswordPluginContext, section: 'phone_form' },
      },
      {
        slot: 'auth.forgot_password.phone.code.after',
        hostContext: { ...authForgotPasswordPluginContext, section: 'phone_form' },
      },
      {
        slot: 'auth.forgot_password.phone.submit.before',
        hostContext: { ...authForgotPasswordPluginContext, section: 'phone_form' },
      },
      {
        slot: 'auth.forgot_password.phone.form.after',
        hostContext: { ...authForgotPasswordPluginContext, section: 'phone_form' },
      },
    ],
    [authForgotPasswordPluginContext]
  )

  return (
    <div className="flex min-h-screen">
      <AuthBrandingPanel />

      {/* Right form panel */}
      <div className="flex flex-1 items-center justify-center bg-background p-6 sm:p-12">
        <PluginSlotBatchBoundary
          scope="public"
          path="/forgot-password"
          items={forgotPasswordBatchItems}
        >
          <div className="w-full max-w-sm space-y-6 sm:space-y-8">
            <div className="text-center lg:hidden">
              <h1 className="text-2xl font-bold tracking-tight text-foreground">AuraLogic</h1>
            </div>

            <Suspense fallback={null}>
              <PluginSlot
                slot="auth.forgot_password.top"
                context={authForgotPasswordPluginContext}
              />
            </Suspense>

            <div className="space-y-2">
              <h2 className="text-2xl font-semibold tracking-tight text-foreground">
                {t.auth.forgotPasswordTitle}
              </h2>
              <p className="text-sm text-muted-foreground">
                {resetMode === 'phone' ? t.auth.phoneResetDesc : t.auth.forgotPasswordDesc}
              </p>
            </div>
            {/* Mode switcher */}
            {allowPasswordReset && phoneResetAvailable && (
              <div className="flex rounded-lg border border-border bg-muted/50 p-1">
                <button
                  type="button"
                  aria-pressed={resetMode === 'email'}
                  className={`flex-1 whitespace-nowrap rounded-md py-2 text-xs transition-colors sm:text-sm ${resetMode === 'email' ? 'bg-background font-medium text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                  onClick={() => {
                    setResetMode('email')
                    widgetRendered.current = false
                  }}
                >
                  <Mail className="-mt-0.5 mr-1 inline h-3.5 w-3.5" />
                  {t.auth.emailResetTab}
                </button>
                <button
                  type="button"
                  aria-pressed={resetMode === 'phone'}
                  className={`flex-1 whitespace-nowrap rounded-md py-2 text-xs transition-colors sm:text-sm ${resetMode === 'phone' ? 'bg-background font-medium text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                  onClick={() => {
                    setResetMode('phone')
                    widgetRendered.current = false
                  }}
                >
                  <Phone className="-mt-0.5 mr-1 inline h-3.5 w-3.5" />
                  {t.auth.phoneResetTab}
                </button>
              </div>
            )}
            <Suspense fallback={null}>
              <PluginSlot
                slot="auth.forgot_password.mode.after"
                context={{ ...authForgotPasswordPluginContext, section: 'mode_switcher' }}
              />
            </Suspense>

            {/* Email reset form */}
            {resetMode === 'email' && (
              <form onSubmit={handleSubmit} className="space-y-5">
                {sent && email && (
                  <>
                    <Alert>
                      <Mail className="h-4 w-4" />
                      <AlertDescription className="space-y-1">
                        <p>{t.auth.resetEmailSent}</p>
                        <p className="break-all">
                          {(t.auth.sentTo as string).replace('{target}', email)}
                        </p>
                      </AlertDescription>
                    </Alert>
                    <Suspense fallback={null}>
                      <PluginSlot
                        slot="auth.forgot_password.email.alert.after"
                        context={{ ...authForgotPasswordPluginContext, section: 'email_form' }}
                      />
                    </Suspense>
                  </>
                )}
                <div className="space-y-2">
                  <label className="text-sm font-medium">{t.auth.email}</label>
                  <div className="relative">
                    <Mail className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                      type="email"
                      placeholder={t.auth.emailPlaceholder}
                      className="h-11 pl-10"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                    />
                  </div>
                </div>

                {needCaptcha && !sent && (
                  <div className="space-y-2">
                    {(captchaConfig.provider === 'cloudflare' ||
                      captchaConfig.provider === 'google') && <div ref={captchaContainerRef} />}
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
                            aria-label={t.auth.captcha}
                          />
                          <img
                            src={builtinCaptcha.data.image}
                            alt={t.auth.captcha}
                            className="h-11 shrink-0 cursor-pointer rounded-md border border-border dark:brightness-90"
                            onClick={() => {
                              refetchCaptcha()
                              setBuiltinCode('')
                            }}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter' || e.key === ' ') {
                                e.preventDefault()
                                refetchCaptcha()
                                setBuiltinCode('')
                              }
                            }}
                            role="button"
                            tabIndex={0}
                            aria-label={t.auth.captchaRefresh}
                            title={t.auth.captchaRefresh}
                          />
                        </div>
                      </>
                    )}
                  </div>
                )}

                <Suspense fallback={null}>
                  <PluginSlot
                    slot="auth.forgot_password.email.submit.before"
                    context={{ ...authForgotPasswordPluginContext, section: 'email_form' }}
                  />
                </Suspense>
                <Button
                  type="submit"
                  className="h-11 w-full text-sm font-medium"
                  disabled={
                    isSubmitting ||
                    !email ||
                    countdown > 0 ||
                    (needCaptcha &&
                      !captchaToken &&
                      !(captchaConfig?.provider === 'builtin' && builtinCode))
                  }
                >
                  {isSubmitting ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {t.auth.sending}
                    </>
                  ) : countdown > 0 ? (
                    (t.auth.codeResendIn as string).replace('{n}', String(countdown))
                  ) : (
                    t.auth.sendResetLink
                  )}
                </Button>
                <Suspense fallback={null}>
                  <PluginSlot
                    slot="auth.forgot_password.email.form.after"
                    context={{ ...authForgotPasswordPluginContext, section: 'email_form' }}
                  />
                </Suspense>
              </form>
            )}

            {/* Phone reset form */}
            {resetMode === 'phone' && (
              <form onSubmit={handlePhoneReset} className="space-y-5">
                {phoneCodeSent && phoneNumber && (
                  <>
                    <Alert>
                      <Phone className="h-4 w-4" />
                      <AlertDescription className="space-y-1">
                        <p>{t.auth.phoneResetCodeSent}</p>
                        <p className="break-all">
                          {(t.auth.sentTo as string).replace(
                            '{target}',
                            `${phoneCountryCode} ${phoneNumber}`
                          )}
                        </p>
                      </AlertDescription>
                    </Alert>
                    <Suspense fallback={null}>
                      <PluginSlot
                        slot="auth.forgot_password.phone.alert.after"
                        context={{ ...authForgotPasswordPluginContext, section: 'phone_form' }}
                      />
                    </Suspense>
                  </>
                )}
                <div className="space-y-2">
                  <label className="text-sm font-medium">{t.auth.phone}</label>
                  <PhoneInput
                    countryCode={phoneCountryCode}
                    onCountryCodeChange={setPhoneCountryCode}
                    phone={phoneNumber}
                    onPhoneChange={setPhoneNumber}
                    placeholder={t.auth.phonePlaceholder}
                    className="h-11"
                  />
                </div>

                {needCaptcha && !phoneCodeSent && (
                  <div className="space-y-2">
                    {(captchaConfig.provider === 'cloudflare' ||
                      captchaConfig.provider === 'google') && <div ref={captchaContainerRef} />}
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
                            aria-label={t.auth.captcha}
                          />
                          <img
                            src={builtinCaptcha.data.image}
                            alt={t.auth.captcha}
                            className="h-11 shrink-0 cursor-pointer rounded-md border border-border dark:brightness-90"
                            onClick={() => {
                              refetchCaptcha()
                              setBuiltinCode('')
                            }}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter' || e.key === ' ') {
                                e.preventDefault()
                                refetchCaptcha()
                                setBuiltinCode('')
                              }
                            }}
                            role="button"
                            tabIndex={0}
                            aria-label={t.auth.captchaRefresh}
                            title={t.auth.captchaRefresh}
                          />
                        </div>
                      </>
                    )}
                  </div>
                )}

                <div className="space-y-2">
                  <label className="text-sm font-medium">{t.auth.phoneCode}</label>
                  <div className="flex gap-2">
                    <Input
                      placeholder={t.auth.phoneCodePlaceholder}
                      value={phoneCode}
                      onChange={(e) => setPhoneCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                      maxLength={6}
                      className="h-11"
                    />
                    <Button
                      type="button"
                      variant="outline"
                      className="h-11 shrink-0 text-sm"
                      disabled={
                        !phoneNumber ||
                        phoneCountdown > 0 ||
                        isSendingPhoneCode ||
                        (needCaptcha &&
                          !captchaToken &&
                          !(captchaConfig?.provider === 'builtin' && builtinCode))
                      }
                      onClick={handleSendPhoneCode}
                    >
                      {isSendingPhoneCode
                        ? t.auth.sendingCode
                        : phoneCountdown > 0
                          ? (t.auth.codeResendIn as string).replace('{n}', String(phoneCountdown))
                          : t.auth.sendPhoneCode}
                    </Button>
                  </div>
                </div>
                <Suspense fallback={null}>
                  <PluginSlot
                    slot="auth.forgot_password.phone.code.after"
                    context={{ ...authForgotPasswordPluginContext, section: 'phone_form' }}
                  />
                </Suspense>

                <div className="space-y-2">
                  <label className="text-sm font-medium">{t.auth.newPassword}</label>
                  <div className="relative">
                    <Lock className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                      type={showNewPassword ? 'text' : 'password'}
                      placeholder={t.auth.passwordPlaceholder}
                      className="h-11 pl-10 pr-10"
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                    />
                    <button
                      type="button"
                      onClick={() => setShowNewPassword(!showNewPassword)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                      aria-label={showNewPassword ? t.auth.hidePassword : t.auth.showPassword}
                      title={showNewPassword ? t.auth.hidePassword : t.auth.showPassword}
                    >
                      {showNewPassword ? (
                        <EyeOff className="h-4 w-4" />
                      ) : (
                        <Eye className="h-4 w-4" />
                      )}
                    </button>
                  </div>
                  <p className="text-xs text-muted-foreground">{t.profile.passwordRequirement}</p>
                </div>

                <div className="space-y-2">
                  <label className="text-sm font-medium">{t.auth.confirmNewPassword}</label>
                  <div className="relative">
                    <Lock className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                      type={showConfirmPassword ? 'text' : 'password'}
                      placeholder={t.auth.confirmPasswordPlaceholder}
                      className="h-11 pl-10 pr-10"
                      value={confirmPassword}
                      onChange={(e) => setConfirmPassword(e.target.value)}
                    />
                    <button
                      type="button"
                      onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
                      aria-label={showConfirmPassword ? t.auth.hidePassword : t.auth.showPassword}
                      title={showConfirmPassword ? t.auth.hidePassword : t.auth.showPassword}
                    >
                      {showConfirmPassword ? (
                        <EyeOff className="h-4 w-4" />
                      ) : (
                        <Eye className="h-4 w-4" />
                      )}
                    </button>
                  </div>
                </div>

                <Suspense fallback={null}>
                  <PluginSlot
                    slot="auth.forgot_password.phone.submit.before"
                    context={{ ...authForgotPasswordPluginContext, section: 'phone_form' }}
                  />
                </Suspense>
                <Button
                  type="submit"
                  className="h-11 w-full text-sm font-medium"
                  disabled={
                    isResettingPhone ||
                    !phoneNumber ||
                    phoneCode.length !== 6 ||
                    !newPassword ||
                    !confirmPassword
                  }
                >
                  {isResettingPhone ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      {t.auth.sending}
                    </>
                  ) : (
                    t.auth.resetPassword
                  )}
                </Button>
                <Suspense fallback={null}>
                  <PluginSlot
                    slot="auth.forgot_password.phone.form.after"
                    context={{ ...authForgotPasswordPluginContext, section: 'phone_form' }}
                  />
                </Suspense>
              </form>
            )}

            <Suspense fallback={null}>
              <PluginSlot
                slot="auth.forgot_password.bottom"
                context={authForgotPasswordPluginContext}
              />
            </Suspense>

            <Suspense fallback={null}>
              <PluginSlot
                slot="auth.forgot_password.footer.before"
                context={{ ...authForgotPasswordPluginContext, section: 'footer' }}
              />
            </Suspense>
            <p className="text-center text-xs text-muted-foreground">
              <Link
                href="/login"
                className="inline-flex items-center gap-1 text-primary hover:underline"
              >
                <ArrowLeft className="h-3 w-3" />
                {t.auth.backToLogin}
              </Link>
            </p>
          </div>
        </PluginSlotBatchBoundary>
      </div>
    </div>
  )
}
