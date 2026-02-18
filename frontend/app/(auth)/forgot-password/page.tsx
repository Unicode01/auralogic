'use client'

import { useState, useEffect, useRef } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import { Loader2, Mail, ArrowLeft } from 'lucide-react'
import Link from 'next/link'
import { useQuery } from '@tanstack/react-query'
import { getPublicConfig, getCaptcha, forgotPassword } from '@/lib/api'
import { useTheme } from '@/contexts/theme-context'
import toast from 'react-hot-toast'
import { AuthBrandingPanel } from '@/components/auth-branding-panel'

export default function ForgotPasswordPage() {
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.forgotPassword)
  const { resolvedTheme } = useTheme()

  const [email, setEmail] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [sent, setSent] = useState(false)
  const [countdown, setCountdown] = useState(0)
  const [captchaToken, setCaptchaToken] = useState('')
  const [builtinCode, setBuiltinCode] = useState('')
  const captchaContainerRef = useRef<HTMLDivElement>(null)
  const widgetRendered = useRef(false)
  const widgetIdRef = useRef<any>(null)

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
  })

  const captchaConfig = publicConfig?.data?.captcha
  const needCaptcha = captchaConfig?.provider && captchaConfig.provider !== 'none' && captchaConfig.enable_for_login

  const { data: builtinCaptcha, refetch: refetchCaptcha } = useQuery({
    queryKey: ['captcha', 'forgot'],
    queryFn: getCaptcha,
    enabled: needCaptcha && captchaConfig?.provider === 'builtin',
  })

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

  // Load third-party captcha scripts
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
    } catch (e: any) {
      const msg = e?.code === 42902 ? t.auth.cooldownWait : (e?.message || t.auth.requestFailed)
      toast.error(msg)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="min-h-screen flex">
      <AuthBrandingPanel />

      {/* Right form panel */}
      <div className="flex-1 flex items-center justify-center p-6 sm:p-12 bg-background">
        <div className="w-full max-w-sm space-y-6 sm:space-y-8">
          <div className="lg:hidden text-center">
            <h1 className="text-2xl font-bold text-foreground tracking-tight">AuraLogic</h1>
          </div>

          <div className="space-y-2">
            <h2 className="text-2xl font-semibold tracking-tight text-foreground">
              {t.auth.forgotPasswordTitle}
            </h2>
            <p className="text-sm text-muted-foreground">
              {t.auth.forgotPasswordDesc}
            </p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-5">
            <div className="space-y-2">
              <label className="text-sm font-medium">{t.auth.email}</label>
              <div className="relative">
                <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                <Input
                  type="email"
                  placeholder={t.auth.emailPlaceholder}
                  className="pl-10 h-11"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                />
              </div>
            </div>

            {needCaptcha && !sent && (
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
              disabled={isSubmitting || !email || countdown > 0}
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
          </form>

          {sent && (
            <p className="text-sm text-center text-muted-foreground">
              {t.auth.resetEmailSent}
            </p>
          )}

          <p className="text-center text-xs text-muted-foreground">
            <Link href="/login" className="text-primary hover:underline inline-flex items-center gap-1">
              <ArrowLeft className="h-3 w-3" />
              {t.auth.backToLogin}
            </Link>
          </p>
        </div>
      </div>
    </div>
  )
}
