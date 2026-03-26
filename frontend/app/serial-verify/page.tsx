'use client'
/* eslint-disable @next/next/no-img-element */

import { Suspense, useEffect, useRef, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Search,
  CheckCircle2,
  XCircle,
  Package,
  Eye,
  ShieldCheck,
  AlertTriangle,
  Globe,
  Copy,
  RotateCcw,
  X,
} from 'lucide-react'
import { formatDate } from '@/lib/utils'
import { useTheme } from '@/contexts/theme-context'
import { useLocale } from '@/hooks/use-locale'
import { useAuth } from '@/hooks/use-auth'
import { useIsMobile } from '@/hooks/use-mobile'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'
import { UserSidebar } from '@/components/layout/user-sidebar'
import { MobileBottomNav } from '@/components/layout/mobile-bottom-nav'
import { FullPageLoading } from '@/components/ui/page-loading'
import { PluginSlot } from '@/components/plugins/plugin-slot'
import toast from 'react-hot-toast'

interface SerialInfo {
  id: number
  serial_number: string
  product_id: number
  product_code: string
  sequence_number: number
  anti_counterfeit_code: string
  view_count: number
  first_viewed_at?: string
  last_viewed_at?: string
  created_at: string
  product?: {
    id: number
    name: string
    sku: string
    images?: Array<{ url: string; is_primary: boolean }>
  }
}

export default function SerialVerifyPage() {
  return (
    <Suspense fallback={<FullPageLoading />}>
      <SerialVerifyContent />
    </Suspense>
  )
}

function SerialVerifyContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { locale } = useLocale()
  const { resolvedTheme } = useTheme()
  const { isAuthenticated, isLoading: authLoading } = useAuth()
  const { isMobile, mounted: mobileMounted } = useIsMobile()
  usePageTitle(getTranslations(locale).pageTitle.serialVerify)
  const [lang, setLang] = useState('zh')
  const [serialNumber, setSerialNumber] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [serialInfo, setSerialInfo] = useState<SerialInfo | null>(null)
  const [error, setError] = useState('')
  const [publicConfig, setPublicConfig] = useState<any>(null)
  const [captchaToken, setCaptchaToken] = useState('')
  const [builtinCaptcha, setBuiltinCaptcha] = useState<{
    captcha_id: string
    image: string
  } | null>(null)
  const [builtinCode, setBuiltinCode] = useState('')
  const captchaContainerRef = useRef<HTMLDivElement>(null)
  const widgetRendered = useRef(false)
  const widgetIdRef = useRef<any>(null)

  const resolvedLocale = lang === 'en' ? 'en' : 'zh'
  const i18n = getTranslations(resolvedLocale)
  const t = i18n.serialVerify
  const captchaCfg = publicConfig?.captcha
  const needCaptcha =
    captchaCfg?.provider && captchaCfg.provider !== 'none' && !!captchaCfg.enable_for_serial_verify

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const resp = await fetch('/api/config/public')
        const json = await resp.json()
        if (!cancelled && resp.ok && json?.code === 0) {
          setPublicConfig(json.data)
        }
      } catch {
        // ignore
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (!needCaptcha || captchaCfg?.provider !== 'builtin') return
    let cancelled = false
    ;(async () => {
      try {
        const resp = await fetch('/api/user/auth/captcha')
        const json = await resp.json()
        if (!cancelled && resp.ok && json?.code === 0) {
          setBuiltinCaptcha(json.data)
        }
      } catch {
        // ignore
      }
    })()
    return () => {
      cancelled = true
    }
  }, [needCaptcha, captchaCfg?.provider])

  useEffect(() => {
    if (!needCaptcha || captchaCfg?.provider !== 'builtin') return
    const timer = setInterval(() => {
      refreshBuiltinCaptcha()
    }, 270000)
    return () => clearInterval(timer)
  }, [needCaptcha, captchaCfg?.provider])

  useEffect(() => {
    if (!needCaptcha) return
    if (captchaCfg?.provider === 'cloudflare' && !document.getElementById('cf-turnstile-script')) {
      const script = document.createElement('script')
      script.id = 'cf-turnstile-script'
      script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?onload=onTurnstileLoad'
      script.async = true
      ;(window as any).onTurnstileLoad = () => {
        if (captchaContainerRef.current && !widgetRendered.current) {
          widgetRendered.current = true
          widgetIdRef.current = (window as any).turnstile.render(captchaContainerRef.current, {
            sitekey: captchaCfg?.site_key,
            theme: resolvedTheme === 'dark' ? 'dark' : 'light',
            callback: (token: string) => setCaptchaToken(token),
            'expired-callback': () => setCaptchaToken(''),
          })
        }
      }
      document.body.appendChild(script)
    } else if (captchaCfg?.provider === 'google' && !document.getElementById('recaptcha-script')) {
      const script = document.createElement('script')
      script.id = 'recaptcha-script'
      script.src = 'https://www.google.com/recaptcha/api.js?onload=onRecaptchaLoad&render=explicit'
      script.async = true
      ;(window as any).onRecaptchaLoad = () => {
        if (captchaContainerRef.current && !widgetRendered.current) {
          widgetRendered.current = true
          widgetIdRef.current = (window as any).grecaptcha.render(captchaContainerRef.current, {
            sitekey: captchaCfg?.site_key,
            theme: resolvedTheme === 'dark' ? 'dark' : 'light',
            callback: (token: string) => setCaptchaToken(token),
            'expired-callback': () => setCaptchaToken(''),
          })
        }
      }
      document.body.appendChild(script)
    }
  }, [needCaptcha, captchaCfg, resolvedTheme])

  useEffect(() => {
    if (!needCaptcha || widgetRendered.current || !captchaContainerRef.current) return
    if (captchaCfg?.provider === 'cloudflare' && (window as any).turnstile) {
      widgetRendered.current = true
      widgetIdRef.current = (window as any).turnstile.render(captchaContainerRef.current, {
        sitekey: captchaCfg?.site_key,
        theme: resolvedTheme === 'dark' ? 'dark' : 'light',
        callback: (token: string) => setCaptchaToken(token),
        'expired-callback': () => setCaptchaToken(''),
      })
    } else if (captchaCfg?.provider === 'google' && (window as any).grecaptcha?.render) {
      widgetRendered.current = true
      widgetIdRef.current = (window as any).grecaptcha.render(captchaContainerRef.current, {
        sitekey: captchaCfg?.site_key,
        theme: resolvedTheme === 'dark' ? 'dark' : 'light',
        callback: (token: string) => setCaptchaToken(token),
        'expired-callback': () => setCaptchaToken(''),
      })
    }
  }, [needCaptcha, captchaCfg, resolvedTheme])

  useEffect(() => {
    if (!captchaToken || !needCaptcha || captchaCfg?.provider === 'builtin') return
    if (serialNumber.trim() && !isLoading) {
      handleVerify()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [captchaToken])

  async function refreshBuiltinCaptcha() {
    try {
      const resp = await fetch('/api/user/auth/captcha')
      const json = await resp.json()
      if (resp.ok && json?.code === 0) {
        setBuiltinCaptcha(json.data)
      }
    } catch {
      // ignore
    } finally {
      setBuiltinCode('')
    }
  }

  function resetCaptcha() {
    if (!needCaptcha) return
    if (captchaCfg?.provider === 'builtin') {
      refreshBuiltinCaptcha()
    } else if (captchaCfg?.provider === 'cloudflare' && (window as any).turnstile) {
      try {
        ;(window as any).turnstile.reset(widgetIdRef.current)
      } catch {
        // ignore
      }
      setCaptchaToken('')
    } else if (captchaCfg?.provider === 'google' && (window as any).grecaptcha) {
      try {
        ;(window as any).grecaptcha.reset(widgetIdRef.current)
      } catch {
        // ignore
      }
      setCaptchaToken('')
    }
  }

  useEffect(() => {
    const urlLang = searchParams.get('lang')
    if (urlLang && (urlLang === 'zh' || urlLang === 'en')) {
      setLang(urlLang)
    } else {
      const storedLocale = localStorage.getItem('auralogic_locale')
      let detectedLang: string
      if (storedLocale === 'zh' || storedLocale === 'en') {
        detectedLang = storedLocale
      } else {
        const browserLang = navigator.language.toLowerCase()
        detectedLang = browserLang.startsWith('zh') ? 'zh' : 'en'
      }
      setLang(detectedLang)
      const url = new URL(window.location.href)
      url.searchParams.set('lang', detectedLang)
      window.history.replaceState({}, '', url.toString())
    }
  }, [searchParams])

  const toggleLanguage = () => {
    const newLang = lang === 'zh' ? 'en' : 'zh'
    setLang(newLang)
    const url = new URL(window.location.href)
    url.searchParams.set('lang', newLang)
    window.history.replaceState({}, '', url.toString())
  }

  const handleVerify = async () => {
    if (isLoading) {
      return
    }
    if (!serialNumber.trim()) {
      setError(t.inputRequired)
      return
    }

    let tokenToSend = ''
    if (needCaptcha) {
      if (captchaCfg?.provider === 'builtin') {
        tokenToSend = builtinCaptcha?.captcha_id
          ? `${builtinCaptcha.captcha_id}:${builtinCode}`
          : ''
      } else {
        tokenToSend = captchaToken
      }
      if (!tokenToSend || (captchaCfg?.provider === 'builtin' && !builtinCode.trim())) {
        setError(t.captchaRequired)
        return
      }
    }

    setIsLoading(true)
    setError('')
    setSerialInfo(null)

    try {
      const response = await fetch('/api/serial/verify', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          serial_number: serialNumber.trim().toUpperCase(),
          captcha_token: tokenToSend || undefined,
        }),
      })

      const result = await response.json()

      if (response.ok && result.code === 0) {
        setSerialInfo(result.data)
        resetCaptcha()
      } else {
        const msg = result?.message || ''
        if (msg === 'Captcha is required') {
          setError(t.captchaRequired)
        } else if (msg === 'Captcha verification failed') {
          setError(t.captchaFailed)
          resetCaptcha()
        } else {
          setError(msg || t.serialNotFound)
        }
      }
    } catch {
      setError(t.networkError)
    } finally {
      setIsLoading(false)
    }
  }

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleVerify()
    }
  }

  const primaryImage = serialInfo?.product?.images?.find((img) => img.is_primary)?.url
  const normalizedSerialNumber = serialNumber.trim().toUpperCase()
  const canVerify =
    !isLoading &&
    !!normalizedSerialNumber &&
    (!needCaptcha || (captchaCfg?.provider === 'builtin' ? !!builtinCode.trim() : !!captchaToken))

  const copyResultValue = async (value: string) => {
    try {
      await navigator.clipboard.writeText(value)
      toast.success(i18n.common.copiedToClipboard)
    } catch {
      toast.error(i18n.common.failed)
    }
  }

  if (!mobileMounted || authLoading) {
    return <FullPageLoading text={i18n.common.loading} />
  }

  const showLanguageToggle = !isAuthenticated || isMobile
  const publicSerialVerifyPluginContext = {
    view: 'public_serial_verify',
    locale: resolvedLocale,
    query: {
      serial_number: normalizedSerialNumber || undefined,
      has_input: Boolean(normalizedSerialNumber),
    },
    capabilities: {
      captcha_required: Boolean(needCaptcha),
      authenticated: isAuthenticated,
      mobile: isMobile,
    },
    summary: {
      is_loading: isLoading,
      has_result: Boolean(serialInfo),
      has_error: Boolean(error),
    },
    state: {
      loading: isLoading,
      error: Boolean(error),
      idle: !isLoading && !serialInfo && !error,
      verified: Boolean(serialInfo),
      query_present: Boolean(normalizedSerialNumber),
      captcha_required: Boolean(needCaptcha),
      can_verify: canVerify,
      view_warning: Boolean(serialInfo && serialInfo.view_count > 5),
    },
    result: serialInfo
      ? {
          id: serialInfo.id,
          serial_number: serialInfo.serial_number,
          product_id: serialInfo.product_id,
          product_name: serialInfo.product?.name || undefined,
          product_sku: serialInfo.product?.sku || undefined,
          product_code: serialInfo.product_code,
          sequence_number: serialInfo.sequence_number,
          view_count: serialInfo.view_count,
          created_at: serialInfo.created_at,
          first_viewed_at: serialInfo.first_viewed_at,
          last_viewed_at: serialInfo.last_viewed_at,
        }
      : undefined,
  }
  const content = (
    <div className="mx-auto max-w-2xl space-y-6">
      {showLanguageToggle ? (
        <div className="flex justify-end">
          <Button
            variant="outline"
            size="sm"
            onClick={toggleLanguage}
            className="gap-2"
            aria-label={`${i18n.profile.languagePreference}: ${lang === 'zh' ? i18n.language.en : i18n.language.zh}`}
            title={`${i18n.profile.languagePreference}: ${lang === 'zh' ? i18n.language.en : i18n.language.zh}`}
          >
            <Globe className="h-4 w-4" />
            {lang === 'zh' ? i18n.language.en : i18n.language.zh}
          </Button>
        </div>
      ) : null}

      <PluginSlot slot="public.serial_verify.top" context={publicSerialVerifyPluginContext} />

      <div className="space-y-2 text-center">
        <div className="flex items-center justify-center gap-2">
          <ShieldCheck className="h-9 w-9 text-primary" />
        </div>
        <h1 className="text-3xl font-bold">{t.title}</h1>
        <p className="text-muted-foreground">{t.subtitle}</p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Search className="h-5 w-5" />
            {t.serialQuery}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col gap-2 sm:flex-row">
            <div className="relative flex-1">
              <Input
                placeholder={t.inputPlaceholder}
                value={serialNumber}
                onChange={(e) => {
                  setSerialNumber(e.target.value.toUpperCase())
                  if (error) setError('')
                  if (serialInfo) setSerialInfo(null)
                }}
                onKeyDown={handleKeyPress}
                className="pr-9 font-mono text-base sm:text-lg"
                aria-label={t.currentSerial}
              />
              {serialNumber ? (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="absolute right-1 top-1/2 h-7 w-7 -translate-y-1/2 rounded-full text-muted-foreground hover:text-foreground"
                  onClick={() => {
                    setSerialNumber('')
                    setSerialInfo(null)
                    setError('')
                  }}
                  aria-label={i18n.common.clear}
                  title={i18n.common.clear}
                >
                  <X className="h-4 w-4" />
                  <span className="sr-only">{i18n.common.clear}</span>
                </Button>
              ) : null}
            </div>
            <Button onClick={handleVerify} disabled={!canVerify} size="lg" className="sm:min-w-28">
              {isLoading ? t.verifying : t.verify}
            </Button>
          </div>

          {needCaptcha ? (
            <div className="space-y-2">
              {(captchaCfg?.provider === 'cloudflare' || captchaCfg?.provider === 'google') && (
                <div ref={captchaContainerRef} />
              )}
              {captchaCfg?.provider === 'builtin' && builtinCaptcha?.image ? (
                <div className="space-y-1">
                  <label className="text-sm font-medium">{t.captcha}</label>
                  <div className="flex items-center gap-2">
                    <Input
                      placeholder={t.captchaPlaceholder}
                      value={builtinCode}
                      onChange={(e) => setBuiltinCode(e.target.value)}
                      maxLength={4}
                      aria-label={t.captcha}
                    />
                    <img
                      src={builtinCaptcha.image}
                      alt={t.captchaAlt}
                      className="h-9 w-28 flex-shrink-0 cursor-pointer rounded border dark:brightness-90"
                      onClick={() => refreshBuiltinCaptcha()}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault()
                          refreshBuiltinCaptcha()
                        }
                      }}
                      role="button"
                      tabIndex={0}
                      aria-label={t.captchaRefresh}
                      title={t.captchaRefresh}
                    />
                  </div>
                </div>
              ) : null}
            </div>
          ) : null}

          <p className="text-xs text-muted-foreground">{t.hint}</p>
        </CardContent>
      </Card>
      <PluginSlot
        slot="public.serial_verify.form.after"
        context={{ ...publicSerialVerifyPluginContext, section: 'query_form' }}
      />

      {error ? (
        <Alert variant="destructive">
          <XCircle className="h-4 w-4" />
          <AlertDescription className="space-y-3">
            <p>{error}</p>
            <p className="text-xs opacity-90">{t.retryHint}</p>
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={handleVerify}
                disabled={!normalizedSerialNumber || isLoading}
              >
                <RotateCcw className="mr-2 h-4 w-4" />
                {t.retryVerify}
              </Button>
              {needCaptcha ? (
                <Button type="button" size="sm" variant="ghost" onClick={resetCaptcha}>
                  {i18n.common.refresh}
                </Button>
              ) : null}
            </div>
          </AlertDescription>
        </Alert>
      ) : null}
      {error ? (
        <PluginSlot
          slot="public.serial_verify.error"
          context={{ ...publicSerialVerifyPluginContext, section: 'verify_state' }}
        />
      ) : null}

      {serialInfo ? (
        <div className="space-y-4">
          <Card className="border-green-500/30 bg-green-500/10 dark:border-green-500/40 dark:bg-green-950/20">
            <CardContent className="pt-6">
              <div className="flex items-center gap-3">
                <CheckCircle2 className="h-12 w-12 text-green-600 dark:text-green-400" />
                <div>
                  <h3 className="text-xl font-bold text-green-700 dark:text-green-300">
                    {t.verifiedTitle}
                  </h3>
                  <p className="text-sm text-green-600 dark:text-green-400">{t.verifiedDesc}</p>
                </div>
              </div>
            </CardContent>
          </Card>
          <PluginSlot
            slot="public.serial_verify.result.summary.after"
            context={{ ...publicSerialVerifyPluginContext, section: 'result_summary' }}
          />

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Package className="h-5 w-5" />
                {t.productInfo}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex gap-4">
                {primaryImage ? (
                  <div className="h-24 w-24 flex-shrink-0 overflow-hidden rounded bg-muted">
                    <img
                      src={primaryImage}
                      alt={serialInfo.product?.name}
                      className="h-full w-full object-cover"
                    />
                  </div>
                ) : (
                  <div className="flex h-24 w-24 flex-shrink-0 items-center justify-center rounded bg-muted">
                    <Package className="h-12 w-12 text-muted-foreground" />
                  </div>
                )}
                <div className="flex-1">
                  <h4 className="text-lg font-bold">{serialInfo.product?.name}</h4>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {t.sku}: {serialInfo.product?.sku}
                  </p>
                  <p className="mt-2 text-sm text-muted-foreground">
                    {[
                      `${t.productCode}: ${serialInfo.product_code}`,
                      t.sequenceNumber.replace('{n}', serialInfo.sequence_number.toString()),
                    ].join(' · ')}
                  </p>
                </div>
              </div>
            </CardContent>
          </Card>
          <PluginSlot
            slot="public.serial_verify.result.product.after"
            context={{ ...publicSerialVerifyPluginContext, section: 'result_product' }}
          />

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <ShieldCheck className="h-5 w-5" />
                {t.antiCounterfeitInfo}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => void copyResultValue(serialInfo.serial_number)}
                >
                  <Copy className="mr-2 h-4 w-4" />
                  {t.copySerialNumber}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => void copyResultValue(serialInfo.anti_counterfeit_code)}
                >
                  <Copy className="mr-2 h-4 w-4" />
                  {t.copyAntiCounterfeitCode}
                </Button>
              </div>
              <div className="flex justify-between gap-3 border-b py-2">
                <span className="text-muted-foreground">{t.fullSerialNumber}</span>
                <span className="text-right font-mono text-base font-bold sm:text-lg">
                  {serialInfo.serial_number}
                </span>
              </div>
              <div className="flex justify-between gap-3 border-b py-2">
                <span className="text-muted-foreground">{t.productCode}</span>
                <span className="text-right font-medium">{serialInfo.product_code}</span>
              </div>
              <div className="flex justify-between gap-3 border-b py-2">
                <span className="text-muted-foreground">{t.factoryNumber}</span>
                <span className="text-right font-medium">#{serialInfo.sequence_number}</span>
              </div>
              <div className="flex justify-between gap-3 border-b py-2">
                <span className="text-muted-foreground">{t.antiCounterfeitCode}</span>
                <span className="text-right font-mono font-bold">
                  {serialInfo.anti_counterfeit_code}
                </span>
              </div>
              <div className="flex justify-between gap-3 border-b py-2">
                <span className="text-muted-foreground">{t.generatedTime}</span>
                <span className="text-right">{formatDate(serialInfo.created_at)}</span>
              </div>
            </CardContent>
          </Card>
          <PluginSlot
            slot="public.serial_verify.result.security.after"
            context={{ ...publicSerialVerifyPluginContext, section: 'result_security' }}
          />

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <Eye className="h-5 w-5" />
                {t.queryRecord}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center justify-between border-b py-2">
                <span className="text-muted-foreground">{t.queryCount}</span>
                <span className="text-lg font-bold">
                  {t.queryCountValue.replace('{n}', serialInfo.view_count.toString())}
                </span>
              </div>
              {serialInfo.first_viewed_at ? (
                <div className="flex items-center justify-between border-b py-2">
                  <span className="text-muted-foreground">{t.firstQuery}</span>
                  <span>{formatDate(serialInfo.first_viewed_at)}</span>
                </div>
              ) : null}
              {serialInfo.last_viewed_at ? (
                <div className="flex items-center justify-between py-2">
                  <span className="text-muted-foreground">{t.lastQuery}</span>
                  <span>{formatDate(serialInfo.last_viewed_at)}</span>
                </div>
              ) : null}

              {serialInfo.view_count > 5 ? (
                <Alert>
                  <AlertTriangle className="h-4 w-4" />
                  <AlertDescription className="text-sm">
                    {t.queryWarning.replace('{n}', serialInfo.view_count.toString())}
                  </AlertDescription>
                </Alert>
              ) : null}
            </CardContent>
          </Card>
          <PluginSlot
            slot="public.serial_verify.result.history.after"
            context={{ ...publicSerialVerifyPluginContext, section: 'result_history' }}
          />

          <div className="text-center">
            <Button
              variant="outline"
              onClick={() => {
                setSerialNumber('')
                setSerialInfo(null)
                setError('')
              }}
            >
              {t.queryAnother}
            </Button>
          </div>
        </div>
      ) : null}

      {!serialInfo && !error ? (
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t.instructions}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 text-sm text-muted-foreground">
            <p>{t.instruction1}</p>
            <p>{t.instruction2}</p>
            <p>{t.instruction3}</p>
            <p>{t.instruction4}</p>
            <p>{t.instruction5}</p>
          </CardContent>
        </Card>
      ) : null}
      {!serialInfo && !error ? (
        <PluginSlot
          slot="public.serial_verify.idle"
          context={{ ...publicSerialVerifyPluginContext, section: 'verify_state' }}
        />
      ) : null}

      <PluginSlot
        slot="public.serial_verify.bottom"
        context={publicSerialVerifyPluginContext}
      />

      <div className="text-center text-sm text-muted-foreground">
        <button
          type="button"
          onClick={() => (window.history.length > 1 ? router.back() : router.push('/'))}
          className="hover:underline"
          title={t.backToHome}
        >
          {t.backToHome}
        </button>
      </div>
    </div>
  )

  if (!isAuthenticated) {
    return <div className="px-4 py-10 md:px-6 md:py-12">{content}</div>
  }

  return (
    <div className="sidebar-layout flex h-screen">
      {!isMobile ? <UserSidebar /> : null}
      <main className={`flex-1 overflow-y-auto p-4 md:p-8 ${isMobile ? 'pb-20' : ''}`}>
        {content}
      </main>
      {isMobile ? <MobileBottomNav /> : null}
    </div>
  )
}
