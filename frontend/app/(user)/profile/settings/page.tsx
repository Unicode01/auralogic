'use client'

import { useState, useEffect, useCallback, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useAuth } from '@/hooks/use-auth'
import { Card, CardHeader, CardTitle, CardContent, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
  FormDescription,
} from '@/components/ui/form'
import { Separator } from '@/components/ui/separator'
import { changePasswordSchema } from '@/lib/validators'
import { changePassword, getPublicConfig, sendBindEmailCode, bindEmail, sendBindPhoneCode, bindPhone, getCaptcha } from '@/lib/api'
import { useToast } from '@/hooks/use-toast'
import { Key, User, ArrowLeft, Mail, Phone } from 'lucide-react'
import * as z from 'zod'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import Link from 'next/link'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTheme } from '@/contexts/theme-context'

export default function SettingsPage() {
  const { user } = useAuth()
  const { locale } = useLocale()
  const t = getTranslations(locale)
  usePageTitle(t.pageTitle.accountSettings)
  const toast = useToast()
  const queryClient = useQueryClient()
  const [isChangingPassword, setIsChangingPassword] = useState(false)

  const { data: publicConfig } = useQuery({
    queryKey: ['publicConfig'],
    queryFn: getPublicConfig,
  })
  const smtpEnabled = publicConfig?.data?.smtp_enabled
  const smsEnabled = publicConfig?.data?.sms_enabled
  const captchaConfig = publicConfig?.data?.captcha
  const needBindCaptcha = captchaConfig?.provider && captchaConfig.provider !== 'none' && captchaConfig.enable_for_bind
  const { resolvedTheme } = useTheme()

  // Captcha state
  const [captchaToken, setCaptchaToken] = useState('')
  const [builtinCode, setBuiltinCode] = useState('')
  const captchaContainerRef = useRef<HTMLDivElement>(null)
  const widgetRendered = useRef(false)
  const widgetIdRef = useRef<any>(null)

  const { data: builtinCaptcha, refetch: refetchCaptcha } = useQuery({
    queryKey: ['captcha', 'bind'],
    queryFn: getCaptcha,
    enabled: !!needBindCaptcha && captchaConfig?.provider === 'builtin',
  })

  // 验证码超时自动刷新（后端TTL为5分钟，提前30秒刷新）
  useEffect(() => {
    if (!needBindCaptcha || captchaConfig?.provider !== 'builtin') return
    const timer = setInterval(() => {
      refetchCaptcha()
      setBuiltinCode('')
    }, 270000)
    return () => clearInterval(timer)
  }, [needBindCaptcha, captchaConfig?.provider, refetchCaptcha])

  // Bind email state
  const [bindEmailAddr, setBindEmailAddr] = useState('')
  const [bindEmailCode, setBindEmailCode] = useState('')
  const [emailCooldown, setEmailCooldown] = useState(0)
  const [emailSending, setEmailSending] = useState(false)
  const [emailBinding, setEmailBinding] = useState(false)

  // Bind phone state
  const [bindPhoneNum, setBindPhoneNum] = useState('')
  const [bindPhoneCode, setBindPhoneCode] = useState('')
  const [phoneCooldown, setPhoneCooldown] = useState(0)
  const [phoneSending, setPhoneSending] = useState(false)
  const [phoneBinding, setPhoneBinding] = useState(false)

  // Load captcha scripts for cloudflare/google
  useEffect(() => {
    if (!needBindCaptcha) return
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
  }, [needBindCaptcha, captchaConfig, resolvedTheme])

  // Render widget if script already loaded
  useEffect(() => {
    if (!needBindCaptcha || widgetRendered.current || !captchaContainerRef.current) return
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
  }, [needBindCaptcha, captchaConfig, resolvedTheme])

  function getBindCaptchaToken(): string | undefined {
    if (!needBindCaptcha) return undefined
    if (captchaConfig.provider === 'builtin') {
      const captchaId = builtinCaptcha?.data?.captcha_id
      if (!captchaId) return undefined
      return `${captchaId}:${builtinCode}`
    }
    return captchaToken || undefined
  }

  function resetBindCaptcha() {
    if (!needBindCaptcha) return
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

  useEffect(() => {
    if (emailCooldown <= 0) return
    const timer = setTimeout(() => setEmailCooldown(emailCooldown - 1), 1000)
    return () => clearTimeout(timer)
  }, [emailCooldown])

  useEffect(() => {
    if (phoneCooldown <= 0) return
    const timer = setTimeout(() => setPhoneCooldown(phoneCooldown - 1), 1000)
    return () => clearTimeout(timer)
  }, [phoneCooldown])

  const handleSendBindEmailCode = useCallback(async () => {
    if (!bindEmailAddr) return
    setEmailSending(true)
    try {
      await sendBindEmailCode(bindEmailAddr, getBindCaptchaToken())
      toast.success(t.profile.codeSentSuccess)
      setEmailCooldown(60)
    } catch (e: any) {
      toast.error(e.message || t.profile.bindFailed)
      resetBindCaptcha()
    } finally {
      setEmailSending(false)
    }
  }, [bindEmailAddr, t, toast, captchaToken, builtinCode, builtinCaptcha, needBindCaptcha, captchaConfig])

  const handleBindEmail = useCallback(async () => {
    if (!bindEmailAddr || !bindEmailCode) return
    setEmailBinding(true)
    try {
      await bindEmail(bindEmailAddr, bindEmailCode)
      toast.success(t.profile.bindSuccess)
      queryClient.invalidateQueries({ queryKey: ['currentUser'] })
    } catch (e: any) {
      toast.error(e.message || t.profile.bindFailed)
    } finally {
      setEmailBinding(false)
    }
  }, [bindEmailAddr, bindEmailCode, t, toast, queryClient])

  const handleSendBindPhoneCode = useCallback(async () => {
    if (!bindPhoneNum) return
    setPhoneSending(true)
    try {
      await sendBindPhoneCode(bindPhoneNum, undefined, getBindCaptchaToken())
      toast.success(t.profile.codeSentSuccess)
      setPhoneCooldown(60)
    } catch (e: any) {
      toast.error(e.message || t.profile.bindFailed)
      resetBindCaptcha()
    } finally {
      setPhoneSending(false)
    }
  }, [bindPhoneNum, t, toast, captchaToken, builtinCode, builtinCaptcha, needBindCaptcha, captchaConfig])

  const handleBindPhone = useCallback(async () => {
    if (!bindPhoneNum || !bindPhoneCode) return
    setPhoneBinding(true)
    try {
      await bindPhone(bindPhoneNum, bindPhoneCode)
      toast.success(t.profile.bindSuccess)
      queryClient.invalidateQueries({ queryKey: ['currentUser'] })
    } catch (e: any) {
      toast.error(e.message || t.profile.bindFailed)
    } finally {
      setPhoneBinding(false)
    }
  }, [bindPhoneNum, bindPhoneCode, t, toast, queryClient])

  const passwordForm = useForm({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: {
      old_password: '',
      new_password: '',
      confirm_password: '',
    },
  })

  async function onPasswordSubmit(values: z.infer<typeof changePasswordSchema>) {
    setIsChangingPassword(true)
    try {
      await changePassword(values.old_password, values.new_password)
      toast.success(t.profile.passwordChangeSuccess)
      passwordForm.reset()
    } catch (error: any) {
      toast.error(error.message || t.profile.passwordChangeFailed)
    } finally {
      setIsChangingPassword(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button asChild variant="ghost" size="icon" className="md:hidden">
          <Link href="/profile">
            <ArrowLeft className="h-5 w-5" />
          </Link>
        </Button>
        <h1 className="text-2xl md:text-3xl font-bold">{t.profile.accountSettings}</h1>
      </div>

      {/* 账户信息 */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <User className="h-5 w-5" />
            {t.profile.accountInfo}
          </CardTitle>
          <CardDescription>{t.profile.accountInfoReadonly}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div>
            <label className="text-sm font-medium">{t.profile.email}</label>
            <Input value={user?.email} disabled className="mt-2" />
          </div>
          <div>
            <label className="text-sm font-medium">{t.profile.name}</label>
            <Input value={user?.name || ''} disabled className="mt-2" />
          </div>
          {user?.phone && (
            <div>
              <label className="text-sm font-medium">{t.profile.phone}</label>
              <Input value={user.phone} disabled className="mt-2" />
            </div>
          )}
        </CardContent>
      </Card>

      {/* Bind Email */}
      {!user?.email && smtpEnabled && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Mail className="h-5 w-5" />
              {t.profile.bindEmail}
            </CardTitle>
            <CardDescription>{t.profile.bindEmailDesc}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <label className="text-sm font-medium">{t.profile.email}</label>
              <Input
                type="email"
                className="mt-2"
                placeholder={t.profile.emailPlaceholder}
                value={bindEmailAddr}
                onChange={(e) => setBindEmailAddr(e.target.value)}
              />
            </div>
            {needBindCaptcha && (
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
                        className="h-11"
                      />
                      <img
                        src={builtinCaptcha.data.image}
                        alt="captcha"
                        className="border border-border rounded-md h-11 shrink-0 cursor-pointer dark:brightness-90"
                        onClick={() => { refetchCaptcha(); setBuiltinCode('') }}
                      />
                    </div>
                  </>
                )}
              </div>
            )}
            <div>
              <label className="text-sm font-medium">{t.profile.verificationCode}</label>
              <div className="flex gap-2 mt-2">
                <Input
                  placeholder={t.profile.codePlaceholder}
                  value={bindEmailCode}
                  onChange={(e) => setBindEmailCode(e.target.value)}
                  maxLength={6}
                />
                <Button
                  type="button"
                  variant="outline"
                  disabled={!bindEmailAddr || emailCooldown > 0 || emailSending}
                  onClick={handleSendBindEmailCode}
                  className="shrink-0"
                >
                  {emailSending ? t.profile.sending : emailCooldown > 0 ? (t.profile.resendIn as string).replace('{n}', String(emailCooldown)) : t.profile.sendCode}
                </Button>
              </div>
            </div>
            <Button disabled={!bindEmailAddr || !bindEmailCode || emailBinding} onClick={handleBindEmail}>
              {emailBinding ? t.profile.binding : t.profile.bind}
            </Button>
          </CardContent>
        </Card>
      )}

      {/* Bind Phone */}
      {!user?.phone && smsEnabled && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Phone className="h-5 w-5" />
              {t.profile.bindPhone}
            </CardTitle>
            <CardDescription>{t.profile.bindPhoneDesc}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <label className="text-sm font-medium">{t.profile.phone}</label>
              <Input
                className="mt-2"
                placeholder={t.profile.phonePlaceholder}
                value={bindPhoneNum}
                onChange={(e) => setBindPhoneNum(e.target.value)}
              />
            </div>
            {needBindCaptcha && !(!user?.email && smtpEnabled) && (
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
                        className="h-11"
                      />
                      <img
                        src={builtinCaptcha.data.image}
                        alt="captcha"
                        className="border border-border rounded-md h-11 shrink-0 cursor-pointer dark:brightness-90"
                        onClick={() => { refetchCaptcha(); setBuiltinCode('') }}
                      />
                    </div>
                  </>
                )}
              </div>
            )}
            <div>
              <label className="text-sm font-medium">{t.profile.verificationCode}</label>
              <div className="flex gap-2 mt-2">
                <Input
                  placeholder={t.profile.codePlaceholder}
                  value={bindPhoneCode}
                  onChange={(e) => setBindPhoneCode(e.target.value)}
                  maxLength={6}
                />
                <Button
                  type="button"
                  variant="outline"
                  disabled={!bindPhoneNum || phoneCooldown > 0 || phoneSending}
                  onClick={handleSendBindPhoneCode}
                  className="shrink-0"
                >
                  {phoneSending ? t.profile.sending : phoneCooldown > 0 ? (t.profile.resendIn as string).replace('{n}', String(phoneCooldown)) : t.profile.sendCode}
                </Button>
              </div>
            </div>
            <Button disabled={!bindPhoneNum || !bindPhoneCode || phoneBinding} onClick={handleBindPhone}>
              {phoneBinding ? t.profile.binding : t.profile.bind}
            </Button>
          </CardContent>
        </Card>
      )}

      <Separator />

      {/* 修改密码 */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Key className="h-5 w-5" />
            {t.profile.changePassword}
          </CardTitle>
          <CardDescription>
            {t.profile.changePasswordDesc}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form {...passwordForm}>
            <form
              onSubmit={passwordForm.handleSubmit(onPasswordSubmit)}
              className="space-y-4"
            >
              <FormField
                control={passwordForm.control}
                name="old_password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t.profile.currentPassword}</FormLabel>
                    <FormControl>
                      <Input 
                        type="password" 
                        placeholder={t.profile.currentPasswordPlaceholder}
                        {...field} 
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={passwordForm.control}
                name="new_password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t.profile.newPassword}</FormLabel>
                    <FormControl>
                      <Input 
                        type="password" 
                        placeholder={t.profile.newPasswordPlaceholder}
                        {...field} 
                      />
                    </FormControl>
                    <FormDescription>{t.profile.passwordRequirement}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={passwordForm.control}
                name="confirm_password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t.profile.confirmPassword}</FormLabel>
                    <FormControl>
                      <Input 
                        type="password" 
                        placeholder={t.profile.confirmNewPasswordPlaceholder}
                        {...field} 
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <Button type="submit" disabled={isChangingPassword}>
                {isChangingPassword ? t.profile.changing : t.profile.changePassword}
              </Button>
            </form>
          </Form>
        </CardContent>
      </Card>

      {/* 账户安全提示 */}
      <Card className="border-yellow-500/30 bg-yellow-500/10">
        <CardHeader>
          <CardTitle className="text-base">{t.profile.securityTips}</CardTitle>
        </CardHeader>
        <CardContent className="text-sm space-y-2">
          <p>{t.profile.securityTip1}</p>
          <p>{t.profile.securityTip2}</p>
          <p>{t.profile.securityTip3}</p>
          <p>{t.profile.securityTip4}</p>
        </CardContent>
      </Card>
    </div>
  )
}

