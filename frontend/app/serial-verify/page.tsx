'use client'

import { Suspense, useState, useEffect, useRef } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { 
  Search, 
  CheckCircle2, 
  XCircle, 
  Package, 
  Eye, 
  Calendar,
  ShieldCheck,
  AlertTriangle,
  Globe
} from 'lucide-react'
import { formatDate } from '@/lib/utils'
import { useTheme } from '@/contexts/theme-context'
import { useLocale } from '@/hooks/use-locale'
import { getTranslations } from '@/lib/i18n'
import { usePageTitle } from '@/hooks/use-page-title'

// 翻译文本
const translations = {
  zh: {
    title: '产品防伪验证',
    subtitle: '输入产品序列号验证真伪',
    serialQuery: '序列号查询',
    inputPlaceholder: '请输入产品序列号（如：ABC001XY2Z）',
    verify: '验证',
    verifying: '查询中...',
    hint: '💡 序列号通常印在产品包装或标签上，也可以扫描二维码获取',
    inputRequired: '请输入序列号',
    serialNotFound: '序列号不存在或无效',
    networkError: '查询失败，请检查网络连接',
    verifiedTitle: '✓ 正品验证通过',
    verifiedDesc: '此序列号有效，产品为正品',
    productInfo: '商品信息',
    sku: 'SKU',
    productCode: '产品码',
    sequenceNumber: '第 {n} 件',
    antiCounterfeitInfo: '防伪信息',
    fullSerialNumber: '完整序列号',
    factoryNumber: '出厂序号',
    antiCounterfeitCode: '防伪码',
    generatedTime: '生成时间',
    queryRecord: '查询记录',
    queryCount: '查询次数',
    queryCountValue: '{n} 次',
    firstQuery: '首次查询',
    lastQuery: '最近查询',
    queryWarning: '此序列号已被查询 {n} 次，请注意辨别真伪',
    queryAnother: '查询其他序列号',
    instructions: '使用说明',
    instruction1: '• 序列号格式：产品码 + 序号 + 防伪码（如：ABC001XY2Z）',
    instruction2: '• 序列号通常印在产品包装或标签上',
    instruction3: '• 每次查询都会被记录，首次查询的序列号更可信',
    instruction4: '• 如果序列号被查询多次，请谨慎辨别',
    instruction5: '• 支持扫描二维码自动填充序列号',
    backToHome: '返回上一页',
    captcha: '验证码',
    captchaPlaceholder: '请输入验证码',
    captchaRefresh: '点击刷新',
    captchaRequired: '请完成验证码',
    captchaFailed: '验证码验证失败',
  },
  en: {
    title: 'Product Anti-Counterfeiting Verification',
    subtitle: 'Enter product serial number to verify authenticity',
    serialQuery: 'Serial Number Query',
    inputPlaceholder: 'Enter product serial number (e.g., ABC001XY2Z)',
    verify: 'Verify',
    verifying: 'Verifying...',
    hint: '💡 Serial number is usually printed on product packaging or label, or scan QR code',
    inputRequired: 'Please enter serial number',
    serialNotFound: 'Serial number not found or invalid',
    networkError: 'Query failed, please check network connection',
    verifiedTitle: '✓ Genuine Product Verified',
    verifiedDesc: 'This serial number is valid, product is genuine',
    productInfo: 'Product Information',
    sku: 'SKU',
    productCode: 'Product Code',
    sequenceNumber: 'No. {n}',
    antiCounterfeitInfo: 'Anti-Counterfeiting Information',
    fullSerialNumber: 'Full Serial Number',
    factoryNumber: 'Factory Number',
    antiCounterfeitCode: 'Anti-Counterfeit Code',
    generatedTime: 'Generated Time',
    queryRecord: 'Query Record',
    queryCount: 'Query Count',
    queryCountValue: '{n} times',
    firstQuery: 'First Query',
    lastQuery: 'Last Query',
    queryWarning: 'This serial number has been queried {n} times, please verify carefully',
    queryAnother: 'Query Another Serial Number',
    instructions: 'Instructions',
    instruction1: '• Serial number format: Product Code + Number + Anti-Counterfeit Code (e.g., ABC001XY2Z)',
    instruction2: '• Serial number is usually printed on product packaging or label',
    instruction3: '• Every query will be recorded, first-time queries are more reliable',
    instruction4: '• If serial number is queried multiple times, please verify carefully',
    instruction5: '• Supports QR code scanning for auto-fill',
    backToHome: 'Go Back',
    captcha: 'Captcha',
    captchaPlaceholder: 'Enter code',
    captchaRefresh: 'Click to refresh',
    captchaRequired: 'Please complete the captcha',
    captchaFailed: 'Captcha verification failed',
  }
}

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
    <Suspense fallback={
      <div className="min-h-screen flex items-center justify-center">
        <div className="animate-spin h-8 w-8 border-4 border-primary border-t-transparent rounded-full" />
      </div>
    }>
      <SerialVerifyContent />
    </Suspense>
  )
}

function SerialVerifyContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const { locale } = useLocale()
  const { resolvedTheme } = useTheme()
  usePageTitle(getTranslations(locale).pageTitle.serialVerify)
  const [lang, setLang] = useState('zh')
  const [serialNumber, setSerialNumber] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [serialInfo, setSerialInfo] = useState<SerialInfo | null>(null)
  const [error, setError] = useState('')
  const [publicConfig, setPublicConfig] = useState<any>(null)
  const [captchaToken, setCaptchaToken] = useState('')
  const [builtinCaptcha, setBuiltinCaptcha] = useState<{ captcha_id: string; image: string } | null>(null)
  const [builtinCode, setBuiltinCode] = useState('')
  const captchaContainerRef = useRef<HTMLDivElement>(null)
  const widgetRendered = useRef(false)
  const widgetIdRef = useRef<any>(null)

  const t = translations[lang as 'zh' | 'en']
  const captchaCfg = publicConfig?.captcha
  const needCaptcha =
    captchaCfg?.provider &&
    captchaCfg.provider !== 'none' &&
    !!captchaCfg.enable_for_serial_verify

  // Load public config (for captcha)
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

  // Load builtin captcha when needed
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

  // 验证码超时自动刷新（后端TTL为5分钟，提前30秒刷新）
  useEffect(() => {
    if (!needCaptcha || captchaCfg?.provider !== 'builtin') return
    const timer = setInterval(() => {
      refreshBuiltinCaptcha()
    }, 270000)
    return () => clearInterval(timer)
  }, [needCaptcha, captchaCfg?.provider])

  // Load Turnstile/reCAPTCHA scripts for serial verify
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
  }, [needCaptcha, captchaCfg])

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
  }, [needCaptcha, captchaCfg])

  // Auto-verify when CF/Google captcha completes
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

  // 初始化语言和自动检测浏览器语言
  useEffect(() => {
    const urlLang = searchParams.get('lang')
    if (urlLang && (urlLang === 'zh' || urlLang === 'en')) {
      setLang(urlLang)
    } else {
      // 优先读取用户已保存的语言偏好，其次检测浏览器语言
      const storedLocale = localStorage.getItem('auralogic_locale')
      let detectedLang: string
      if (storedLocale === 'zh' || storedLocale === 'en') {
        detectedLang = storedLocale
      } else {
        const browserLang = navigator.language.toLowerCase()
        detectedLang = browserLang.startsWith('zh') ? 'zh' : 'en'
      }
      setLang(detectedLang)
      // 更新URL但不刷新页面
      const url = new URL(window.location.href)
      url.searchParams.set('lang', detectedLang)
      window.history.replaceState({}, '', url.toString())
    }
  }, [searchParams])

  // 切换语言
  const toggleLanguage = () => {
    const newLang = lang === 'zh' ? 'en' : 'zh'
    setLang(newLang)
    const url = new URL(window.location.href)
    url.searchParams.set('lang', newLang)
    window.history.replaceState({}, '', url.toString())
  }

  const handleVerify = async () => {
    if (!serialNumber.trim()) {
      setError(t.inputRequired)
      return
    }

    let tokenToSend = ''
    if (needCaptcha) {
      if (captchaCfg?.provider === 'builtin') {
        tokenToSend = builtinCaptcha?.captcha_id ? `${builtinCaptcha.captcha_id}:${builtinCode}` : ''
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
    } catch (err) {
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

  const primaryImage = serialInfo?.product?.images?.find(img => img.is_primary)?.url

  return (
    <div className="min-h-screen bg-gradient-to-b from-blue-50 to-white dark:from-background dark:to-background py-12 px-4 relative">
      {/* 语言切换按钮 */}
      <Button
        variant="outline"
        size="sm"
        onClick={toggleLanguage}
        className="fixed top-4 right-4 z-50 gap-2 shadow-lg hover:shadow-xl transition-shadow bg-background"
      >
        <Globe className="h-4 w-4" />
        {lang === 'zh' ? 'English' : '中文'}
      </Button>

      <div className="max-w-2xl mx-auto space-y-6">
        {/* 标题 */}
        <div className="text-center space-y-2">
          <div className="flex items-center justify-center gap-2 mb-4">
            <ShieldCheck className="w-10 h-10 text-blue-600" />
          </div>
          <h1 className="text-3xl font-bold">{t.title}</h1>
          <p className="text-muted-foreground">{t.subtitle}</p>
        </div>

        {/* 查询卡片 */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Search className="w-5 h-5" />
              {t.serialQuery}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-4">
            <div className="flex gap-2">
              <Input
                placeholder={t.inputPlaceholder}
                value={serialNumber}
                onChange={(e) => setSerialNumber(e.target.value.toUpperCase())}
                onKeyPress={handleKeyPress}
                className="text-lg font-mono"
              />
              <Button onClick={handleVerify} disabled={isLoading || (needCaptcha && !captchaToken && !(captchaCfg?.provider === 'builtin' && builtinCode))} size="lg">
                {isLoading ? t.verifying : t.verify}
              </Button>
            </div>

            {/* Captcha */}
            {needCaptcha && (
              <div className="space-y-2">
                {(captchaCfg?.provider === 'cloudflare' || captchaCfg?.provider === 'google') && (
                  <div ref={captchaContainerRef} />
                )}
                {captchaCfg?.provider === 'builtin' && builtinCaptcha?.image && (
                  <div className="space-y-1">
                    <label className="text-sm font-medium">{t.captcha}</label>
                    <div className="flex items-center gap-2">
                      <Input
                        placeholder={t.captchaPlaceholder}
                        value={builtinCode}
                        onChange={(e) => setBuiltinCode(e.target.value)}
                        maxLength={4}
                      />
                      <img
                        src={builtinCaptcha.image}
                        alt="captcha"
                        className="h-9 w-28 rounded border cursor-pointer flex-shrink-0 dark:brightness-90"
                        onClick={() => refreshBuiltinCaptcha()}
                        title={t.captchaRefresh}
                      />
                    </div>
                  </div>
                )}
              </div>
            )}
            <p className="text-xs text-muted-foreground">
              {t.hint}
            </p>
          </CardContent>
        </Card>

        {/* 错误提示 */}
        {error && (
          <Alert variant="destructive">
            <XCircle className="h-4 w-4" />
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        {/* 验证结果 */}
        {serialInfo && (
          <div className="space-y-4">
            {/* 真伪状态 */}
            <Card className="border-green-500/30 bg-green-500/10">
              <CardContent className="pt-6">
                <div className="flex items-center gap-3">
                  <CheckCircle2 className="w-12 h-12 text-green-600 dark:text-green-400" />
                  <div>
                    <h3 className="text-xl font-bold text-green-700 dark:text-green-300">{t.verifiedTitle}</h3>
                    <p className="text-sm text-green-600 dark:text-green-400">{t.verifiedDesc}</p>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* 商品信息 */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Package className="w-5 h-5" />
                  {t.productInfo}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="flex gap-4">
                  {primaryImage ? (
                    <div className="w-24 h-24 rounded overflow-hidden bg-muted flex-shrink-0">
                      <img
                        src={primaryImage}
                        alt={serialInfo.product?.name}
                        className="w-full h-full object-cover"
                      />
                    </div>
                  ) : (
                    <div className="w-24 h-24 rounded bg-muted flex items-center justify-center flex-shrink-0">
                      <Package className="w-12 h-12 text-muted-foreground" />
                    </div>
                  )}
                  <div className="flex-1">
                    <h4 className="font-bold text-lg">{serialInfo.product?.name}</h4>
                    <p className="text-sm text-muted-foreground mt-1">
                      {t.sku}: {serialInfo.product?.sku}
                    </p>
                    <div className="mt-2">
                      <Badge variant="outline">{t.productCode}: {serialInfo.product_code}</Badge>
                      <Badge variant="outline" className="ml-2">
                        {t.sequenceNumber.replace('{n}', serialInfo.sequence_number.toString())}
                      </Badge>
                    </div>
                  </div>
                </div>
              </CardContent>
            </Card>

            {/* 序列号详情 */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <ShieldCheck className="w-5 h-5" />
                  {t.antiCounterfeitInfo}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex justify-between py-2 border-b">
                  <span className="text-muted-foreground">{t.fullSerialNumber}</span>
                  <span className="font-mono font-bold text-lg">{serialInfo.serial_number}</span>
                </div>
                <div className="flex justify-between py-2 border-b">
                  <span className="text-muted-foreground">{t.productCode}</span>
                  <span className="font-medium">{serialInfo.product_code}</span>
                </div>
                <div className="flex justify-between py-2 border-b">
                  <span className="text-muted-foreground">{t.factoryNumber}</span>
                  <span className="font-medium">#{serialInfo.sequence_number}</span>
                </div>
                <div className="flex justify-between py-2 border-b">
                  <span className="text-muted-foreground">{t.antiCounterfeitCode}</span>
                  <span className="font-mono font-bold">{serialInfo.anti_counterfeit_code}</span>
                </div>
                <div className="flex justify-between py-2 border-b">
                  <span className="text-muted-foreground">{t.generatedTime}</span>
                  <span>{formatDate(serialInfo.created_at)}</span>
                </div>
              </CardContent>
            </Card>

            {/* 查询统计 */}
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <Eye className="w-5 h-5" />
                  {t.queryRecord}
                </CardTitle>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex items-center justify-between py-2 border-b">
                  <span className="text-muted-foreground">{t.queryCount}</span>
                  <span className="font-bold text-lg">{t.queryCountValue.replace('{n}', serialInfo.view_count.toString())}</span>
                </div>
                {serialInfo.first_viewed_at && (
                  <div className="flex items-center justify-between py-2 border-b">
                    <span className="text-muted-foreground">{t.firstQuery}</span>
                    <span>{formatDate(serialInfo.first_viewed_at)}</span>
                  </div>
                )}
                {serialInfo.last_viewed_at && (
                  <div className="flex items-center justify-between py-2">
                    <span className="text-muted-foreground">{t.lastQuery}</span>
                    <span>{formatDate(serialInfo.last_viewed_at)}</span>
                  </div>
                )}
                
                {/* 查询次数警告 */}
                {serialInfo.view_count > 5 && (
                  <Alert>
                    <AlertTriangle className="h-4 w-4" />
                    <AlertDescription className="text-sm">
                      {t.queryWarning.replace('{n}', serialInfo.view_count.toString())}
                    </AlertDescription>
                  </Alert>
                )}
              </CardContent>
            </Card>

            {/* 返回按钮 */}
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
        )}

        {/* 使用说明 */}
        {!serialInfo && !error && (
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
        )}

        {/* 底部链接 */}
        <div className="text-center text-sm text-muted-foreground">
          <button onClick={() => window.history.length > 1 ? router.back() : router.push('/')} className="hover:underline">
            {t.backToHome}
          </button>
        </div>
      </div>
    </div>
  )
}
