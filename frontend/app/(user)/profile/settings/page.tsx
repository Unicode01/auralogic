'use client'
/* eslint-disable @next/next/no-img-element */

import { useState, useEffect, useCallback, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useAuth } from '@/hooks/use-auth'
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/card'
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
import {
  changePassword,
  getPublicConfig,
  sendBindEmailCode,
  bindEmail,
  sendBindPhoneCode,
  bindPhone,
  getCaptcha,
} from '@/lib/api'
import { useToast } from '@/hooks/use-toast'
import { Key, User, ArrowLeft, Mail, Phone } from 'lucide-react'
import * as z from 'zod'
import { useLocale } from '@/hooks/use-locale'
import { usePageTitle } from '@/hooks/use-page-title'
import { getTranslations } from '@/lib/i18n'
import Link from 'next/link'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useTheme } from '@/contexts/theme-context'
import { resolveApiErrorMessage } from '@/lib/api-error'
import { PluginSlot } from '@/components/plugins/plugin-slot'

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
  const hasServiceConfig = typeof publicConfig !== 'undefined'
  const captchaConfig = publicConfig?.data?.captcha
  const needBindCaptcha =
    captchaConfig?.provider && captchaConfig.provider !== 'none' && captchaConfig.enable_for_bind
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

  // Reset widget when bind sections change (e.g. after email bind, phone section appears)
  useEffect(() => {
    widgetRendered.current = false
  }, [user?.email, user?.phone])

  // Load captcha scripts for cloudflare/google
  useEffect(() => {
    if (!needBindCaptcha) return
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
  }, [needBindCaptcha, captchaConfig, resolvedTheme, user?.email, user?.phone])

  // Auto-send bind code when CF/Google captcha completes
  useEffect(() => {
    if (!captchaToken || !needBindCaptcha || captchaConfig?.provider === 'builtin') return
    const emailBindVisible = !user?.email && smtpEnabled
    const phoneBindVisible = !user?.phone && smsEnabled
    if (emailBindVisible && bindEmailAddr && !emailSending && emailCooldown <= 0) {
      handleSendBindEmailCode()
    } else if (
      phoneBindVisible &&
      !emailBindVisible &&
      bindPhoneNum &&
      !phoneSending &&
      phoneCooldown <= 0
    ) {
      handleSendBindPhoneCode()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [captchaToken])

  const getBindCaptchaToken = useCallback((): string | undefined => {
    if (!needBindCaptcha) return undefined
    if (captchaConfig.provider === 'builtin') {
      const captchaId = builtinCaptcha?.data?.captcha_id
      if (!captchaId) return undefined
      return `${captchaId}:${builtinCode}`
    }
    return captchaToken || undefined
  }, [
    builtinCaptcha?.data?.captcha_id,
    builtinCode,
    captchaConfig?.provider,
    captchaToken,
    needBindCaptcha,
  ])

  const resetBindCaptcha = useCallback(() => {
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
  }, [captchaConfig?.provider, needBindCaptcha, refetchCaptcha])

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
      resetBindCaptcha()
    } catch (e: any) {
      toast.error(resolveApiErrorMessage(e, t, t.profile.bindFailed))
      resetBindCaptcha()
    } finally {
      setEmailSending(false)
    }
  }, [bindEmailAddr, getBindCaptchaToken, resetBindCaptcha, t, toast])

  const handleBindEmail = useCallback(async () => {
    if (!bindEmailAddr || !bindEmailCode) return
    setEmailBinding(true)
    try {
      await bindEmail(bindEmailAddr, bindEmailCode)
      toast.success(t.profile.bindSuccess)
      queryClient.invalidateQueries({ queryKey: ['currentUser'] })
    } catch (e: any) {
      toast.error(resolveApiErrorMessage(e, t, t.profile.bindFailed))
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
      resetBindCaptcha()
    } catch (e: any) {
      toast.error(resolveApiErrorMessage(e, t, t.profile.bindFailed))
      resetBindCaptcha()
    } finally {
      setPhoneSending(false)
    }
  }, [bindPhoneNum, getBindCaptchaToken, resetBindCaptcha, t, toast])

  const handleBindPhone = useCallback(async () => {
    if (!bindPhoneNum || !bindPhoneCode) return
    setPhoneBinding(true)
    try {
      await bindPhone(bindPhoneNum, bindPhoneCode)
      toast.success(t.profile.bindSuccess)
      queryClient.invalidateQueries({ queryKey: ['currentUser'] })
    } catch (e: any) {
      toast.error(resolveApiErrorMessage(e, t, t.profile.bindFailed))
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
      toast.error(resolveApiErrorMessage(error, t, t.profile.passwordChangeFailed))
    } finally {
      setIsChangingPassword(false)
    }
  }

  const userProfileSettingsPluginContext = {
    view: 'user_profile_settings',
    user: {
      id: user?.id,
      email: user?.email || undefined,
      phone: user?.phone || undefined,
      name: user?.name || undefined,
    },
    summary: {
      smtp_enabled: Boolean(smtpEnabled),
      sms_enabled: Boolean(smsEnabled),
      captcha_required_for_bind: Boolean(needBindCaptcha),
      has_email: Boolean(user?.email),
      has_phone: Boolean(user?.phone),
    },
    state: {
      bind_email_available: !user?.email && Boolean(smtpEnabled),
      bind_email_unavailable: !user?.email && hasServiceConfig && !smtpEnabled,
      bind_phone_available: !user?.phone && Boolean(smsEnabled),
      bind_phone_unavailable: !user?.phone && hasServiceConfig && !smsEnabled,
      bind_email_sending: emailSending,
      bind_email_submitting: emailBinding,
      bind_phone_sending: phoneSending,
      bind_phone_submitting: phoneBinding,
      password_submitting: isChangingPassword,
    },
  }

  return (
    <div className="space-y-6">
      <PluginSlot slot="user.profile.settings.top" context={userProfileSettingsPluginContext} />
      <div className="flex flex-col gap-3">
        <div className="flex items-center gap-4">
          <Button asChild variant="outline" size="icon" className="md:hidden">
            <Link href="/profile">
              <ArrowLeft className="h-5 w-5" />
              <span className="sr-only">{t.profile.profileCenter}</span>
            </Link>
          </Button>
          <h1 className="text-2xl font-bold md:text-3xl">{t.profile.accountSettings}</h1>
        </div>
      </div>

      {/* 账户信息 */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <User className="h-5 w-5" />
            {t.profile.accountInfo}
          </CardTitle>
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
      <PluginSlot
        slot="user.profile.settings.account_info.after"
        context={{ ...userProfileSettingsPluginContext, section: 'account_info' }}
      />

      {/* Bind Email */}
      {!user?.email && smtpEnabled && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Mail className="h-5 w-5" />
              {t.profile.bindEmail}
            </CardTitle>
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
            <div>
              <label className="text-sm font-medium">{t.auth.emailCode}</label>
              <div className="mt-2 flex gap-2">
                <Input
                  placeholder={t.profile.codePlaceholder}
                  value={bindEmailCode}
                  onChange={(e) => setBindEmailCode(e.target.value)}
                  maxLength={6}
                />
                <Button
                  type="button"
                  variant="outline"
                  disabled={
                    !bindEmailAddr ||
                    emailCooldown > 0 ||
                    emailSending ||
                    (needBindCaptcha &&
                      !captchaToken &&
                      !(captchaConfig?.provider === 'builtin' && builtinCode))
                  }
                  onClick={handleSendBindEmailCode}
                  className="shrink-0"
                >
                  {emailSending
                    ? t.profile.sending
                    : emailCooldown > 0
                      ? (t.profile.resendIn as string).replace('{n}', String(emailCooldown))
                      : t.profile.sendCode}
                </Button>
              </div>
            </div>
            <Button
              disabled={!bindEmailAddr || !bindEmailCode || emailBinding}
              onClick={handleBindEmail}
            >
              {emailBinding ? t.profile.binding : t.profile.bind}
            </Button>
          </CardContent>
        </Card>
      )}
      {!user?.email && smtpEnabled ? (
        <PluginSlot
          slot="user.profile.settings.bind_email.after"
          context={{ ...userProfileSettingsPluginContext, section: 'bind_email' }}
        />
      ) : null}

      {!user?.email && hasServiceConfig && !smtpEnabled && (
        <Card className="border-dashed">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Mail className="h-5 w-5" />
              {t.profile.bindEmail}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">{t.profile.emailBindUnavailableHint}</p>
          </CardContent>
        </Card>
      )}
      {!user?.email && hasServiceConfig && !smtpEnabled ? (
        <PluginSlot
          slot="user.profile.settings.bind_email.unavailable"
          context={{ ...userProfileSettingsPluginContext, section: 'bind_email' }}
        />
      ) : null}

      {/* Bind Phone */}
      {!user?.phone && smsEnabled && (
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Phone className="h-5 w-5" />
              {t.profile.bindPhone}
            </CardTitle>
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
            <div>
              <label className="text-sm font-medium">{t.auth.phoneCode}</label>
              <div className="mt-2 flex gap-2">
                <Input
                  placeholder={t.profile.codePlaceholder}
                  value={bindPhoneCode}
                  onChange={(e) => setBindPhoneCode(e.target.value)}
                  maxLength={6}
                />
                <Button
                  type="button"
                  variant="outline"
                  disabled={
                    !bindPhoneNum ||
                    phoneCooldown > 0 ||
                    phoneSending ||
                    (needBindCaptcha &&
                      !captchaToken &&
                      !(captchaConfig?.provider === 'builtin' && builtinCode))
                  }
                  onClick={handleSendBindPhoneCode}
                  className="shrink-0"
                >
                  {phoneSending
                    ? t.profile.sending
                    : phoneCooldown > 0
                      ? (t.profile.resendIn as string).replace('{n}', String(phoneCooldown))
                      : t.profile.sendCode}
                </Button>
              </div>
            </div>
            <Button
              disabled={!bindPhoneNum || !bindPhoneCode || phoneBinding}
              onClick={handleBindPhone}
            >
              {phoneBinding ? t.profile.binding : t.profile.bind}
            </Button>
          </CardContent>
        </Card>
      )}
      {!user?.phone && smsEnabled ? (
        <PluginSlot
          slot="user.profile.settings.bind_phone.after"
          context={{ ...userProfileSettingsPluginContext, section: 'bind_phone' }}
        />
      ) : null}

      {!user?.phone && hasServiceConfig && !smsEnabled && (
        <Card className="border-dashed">
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Phone className="h-5 w-5" />
              {t.profile.bindPhone}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">{t.profile.phoneBindUnavailableHint}</p>
          </CardContent>
        </Card>
      )}
      {!user?.phone && hasServiceConfig && !smsEnabled ? (
        <PluginSlot
          slot="user.profile.settings.bind_phone.unavailable"
          context={{ ...userProfileSettingsPluginContext, section: 'bind_phone' }}
        />
      ) : null}

      <Separator />

      {/* 修改密码 */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Key className="h-5 w-5" />
            {t.profile.changePassword}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <PluginSlot
            slot="user.profile.settings.password.top"
            context={{ ...userProfileSettingsPluginContext, section: 'password' }}
          />
          <Form {...passwordForm}>
            <form onSubmit={passwordForm.handleSubmit(onPasswordSubmit)} className="space-y-4">
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

              <PluginSlot
                slot="user.profile.settings.password.submit.before"
                context={{ ...userProfileSettingsPluginContext, section: 'password' }}
              />
              <Button type="submit" disabled={isChangingPassword}>
                {isChangingPassword ? t.profile.changing : t.profile.changePassword}
              </Button>
            </form>
          </Form>
        </CardContent>
      </Card>

      {/* 账户安全提示 */}
      <Card className="border-yellow-500/30 bg-yellow-500/10 dark:border-yellow-500/40 dark:bg-yellow-950/20">
        <CardHeader>
          <CardTitle className="text-base">{t.profile.securityTips}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2 text-sm">
          <p>{t.profile.securityTip1}</p>
          <p>{t.profile.securityTip2}</p>
          <p>{t.profile.securityTip3}</p>
          <p>{t.profile.securityTip4}</p>
        </CardContent>
      </Card>
      <PluginSlot
        slot="user.profile.settings.bottom"
        context={userProfileSettingsPluginContext}
      />
    </div>
  )
}
